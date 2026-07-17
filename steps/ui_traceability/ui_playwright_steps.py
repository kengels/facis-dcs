"""Thin Playwright bindings for the UI traceability red specifications.

The BDD harness owns browser/context lifecycle and exposes ``context.page``.
These steps intentionally use only semantic ``data-test-id`` selectors and,
for repeated records, ``data-test-key``. Product setup remains in the existing
API-backed Given steps; business actions and assertions below stay in-browser.
"""

from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import sys
import time
from datetime import datetime, timedelta, timezone
from pathlib import Path

from behave import given, then, when

from steps.support.api_client import (
    archive_retrieve_url,
    contract_update_url,
    get_with_headers,
    origin_url,
    put_json,
)
from steps.support.services.auth_service import AuthService
from steps.support.services.contract_service import ContractService
from steps.support.services.template_service import TemplateService
from steps.template_management.contract_state_machine_steps import _advance_to_approved, _apply_signature


_TEST_ID = re.compile(r"^[a-z0-9-]+$")
_UI_EXPECT_TIMEOUT_MS = 30_000


def _pid_credential_dir(credential: str) -> Path:
    root = Path(__file__).resolve().parents[2]
    source_dir = root / "testWallet" / "credentials"
    target_dir = root / "tests" / "bdd" / ".tmp" / "pid-credentials"
    target_dir.mkdir(parents=True, exist_ok=True)
    template = source_dir / f"{credential}.pid.template.json"
    target_template = target_dir / template.name
    shutil.copy2(template, target_template)
    shutil.copy2(source_dir / f"{credential}.template.json", target_dir / f"{credential}.template.json")

    issuer = root / "testWallet" / "scripts" / "issue_credentials.py"
    statuslist_base = os.environ.get("BDD_CREDENTIAL_STATUSLIST_SERVICE_URL", "").strip()
    assert statuslist_base, "BDD_CREDENTIAL_STATUSLIST_SERVICE_URL is required for local PID issuance"
    result = subprocess.run(
        [
            sys.executable,
            str(issuer),
            "--credentials-dir",
            str(target_dir),
            "--credential",
            credential,
            "--keys-dir",
            str(root / "testWallet" / "keys"),
            "--statuslist-service-base",
            statuslist_base,
            "--statuslist-tenant",
            "credential",
        ],
        cwd=root,
        capture_output=True,
        text=True,
        timeout=60,
        check=False,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    return target_dir


def _page(context):
    page = getattr(context, "page", None)
    assert page is not None, "Playwright harness must expose context.page for @ui scenarios"
    return page


def _selector(test_id: str, key: str | None = None) -> str:
    assert _TEST_ID.fullmatch(test_id), f"Invalid semantic data-test-id: {test_id!r}"
    selector = f'[data-test-id="{test_id}"]'
    if key is not None:
        selector += f'[data-test-key="{key}"]'
    return selector


def _element(context, test_id: str, key: str | None = None):
    return _page(context).locator(_selector(test_id, key))


def _expect_contains(node, expected: str, *, timeout: float | None = None) -> None:
    from playwright.sync_api import expect

    timeout = _UI_EXPECT_TIMEOUT_MS if timeout is None else timeout
    pattern = re.compile(re.escape(expected), re.IGNORECASE)
    expect(node).to_be_visible(timeout=timeout)
    if node.evaluate("element => element.matches('input, textarea, select')"):
        expect(node).to_have_value(pattern, timeout=timeout)
    else:
        expect(node).to_contain_text(pattern, timeout=timeout)


def _ui_base(context) -> str:
    configured = os.getenv("BDD_DCS_UI_URL", "").strip()
    if configured:
        return configured.rstrip("/")
    return f"{origin_url(context.base_url)}/digital-contracting-service/ui"


def _goto(page, url: str) -> None:
    page.goto(url, wait_until="domcontentloaded")


def _template_did(context, name: str) -> str:
    return str(TemplateService.named(context, name)["did"])


def _contract_did(context, name: str) -> str:
    return str(ContractService._contract_data(context, name)[0])


def _source_key(context, source_test_id: str) -> str:
    from playwright.sync_api import expect

    node = _element(context, source_test_id)
    expect(node).to_have_attribute("data-test-key", re.compile(r".+"))
    key = node.get_attribute("data-test-key")
    assert key, f"UI element {source_test_id!r} has no data-test-key"
    return key


def _credential_for_role(role: str) -> str:
    env_name = "BDD_UI_WALLET_CREDENTIAL_" + re.sub(r"[^A-Z0-9]+", "_", role.upper()).strip("_")
    override = os.getenv(env_name, "").strip()
    if override:
        return override
    credentials = Path(__file__).resolve().parents[2] / "testWallet" / "credentials"
    candidates: list[tuple[int, int, str]] = []
    for template in sorted(credentials.glob("*.template.json")):
        body = json.loads(template.read_text(encoding="utf-8"))
        roles = set(body.get("roles") or [])
        if role in roles:
            credential_name = template.name.removesuffix(".template.json")
            candidates.append((len(roles - {role}), len(roles), credential_name))
    if candidates:
        return min(candidates)[2]
    raise AssertionError(f"No testWallet credential declares role {role!r}")


def _present_wallet(context, role: str, presentation_test_id: str) -> None:
    from playwright.sync_api import expect

    node = _element(context, presentation_test_id)
    expect(node).to_be_visible()
    if presentation_test_id == "auth-oid4vp-presentation":
        expect(node).to_have_attribute("data-login-challenge-bound", "true", timeout=30_000)

    browser_context = _page(context).context
    if getattr(context, "ui_presentation_context", None) is not browser_context:
        context.ui_presentation_context = browser_context
        context.ui_used_presentation_urls = set()
    used_urls = context.ui_used_presentation_urls
    expect(node).to_have_attribute(
        "data-presentation-url",
        re.compile(r"^(?:openid4vp|https?)://.+"),
        timeout=30_000,
    )
    for used_url in sorted(used_urls):
        expect(node).not_to_have_attribute("data-presentation-url", used_url, timeout=30_000)
    presentation_url = node.get_attribute("data-presentation-url")
    assert presentation_url, f"{presentation_test_id!r} must expose data-presentation-url"
    assert presentation_url not in used_urls, (
        f"{presentation_test_id!r} did not deliver a fresh OID4VP challenge URL; "
        f"the verifier URL {presentation_url!r} was already used in this browser context"
    )
    used_urls.add(presentation_url)
    wallet = Path(__file__).resolve().parents[2] / "testWallet" / "demo_wallet.py"
    credential = _credential_for_role(role)
    wallet_credential_dir = ""
    if presentation_test_id == "signing-oid4vp-presentation":
        wallet_credential_dir = str(_pid_credential_dir(credential))
        credential = f"{credential}.pid"
    result = subprocess.run(
        [sys.executable, str(wallet), "--presentation-url", presentation_url, "--credential", credential],
        cwd=wallet.parent.parent,
        env={
            **os.environ,
            "DCS_WALLET_CREDENTIALS_DIR": wallet_credential_dir,
            "DCS_WALLET_STATUSLIST_SERVICE_BASE": (
                ""
                if presentation_test_id == "signing-oid4vp-presentation"
                else os.environ.get("BDD_CREDENTIAL_STATUSLIST_SERVICE_URL", "")
            ),
            "DCS_WALLET_STATUSLIST_TENANT": "credential",
        },
        capture_output=True,
        text=True,
        timeout=60,
        check=False,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    context.ui_auth_established_via = "oid4vp-test-wallet"


def _login(context, role: str) -> None:
    _goto(_page(context), f"{_ui_base(context)}/")
    _present_wallet(context, role, "auth-oid4vp-presentation")
    _element(context, "app-authenticated-shell").wait_for(state="visible")


@given("I have a fresh unauthenticated Playwright browser session")
def fresh_browser(context):
    page = _page(context)
    page.context.clear_cookies()
    context.ui_auth_established_via = None


@given('contract "{name}" is archived and expires within {days:d} days')
def archived_contract_nearing_expiry(context, name, days):
    ContractService._create_contract_in_draft(context, name)
    did, updated_at = ContractService._contract_data(context, name)
    start = datetime.now(timezone.utc) + timedelta(days=1)
    response = put_json(
        context,
        contract_update_url(context),
        {
            "did": did,
            "updated_at": updated_at,
            "start_date": start.strftime("%Y-%m-%dT%H:%M:%SZ"),
            "exp_date": (datetime.now(timezone.utc) + timedelta(days=days)).strftime(
                "%Y-%m-%dT%H:%M:%SZ"
            ),
        },
        headers=context.contract_seed_headers[name],
    )
    assert response.status_code == 200, response.text
    ContractService._refresh_contract(context, name)
    _advance_to_approved(context, name)
    _apply_signature(context, name)
    ContractService._refresh_contract(context, name)


def _create_named_contract_in_draft(context, name: str) -> None:
    """Create the normal fixture, then authoritatively persist its scenario name.

    Contract creation deliberately inherits the source template metadata. UI
    search scenarios need a distinct contract name, so set it through the real
    update API while the contract is still editable instead of teaching the UI
    test to accept the inherited template name.
    """
    ContractService._create_contract_in_draft(context, name)
    did, updated_at = ContractService._contract_data(context, name)
    response = put_json(
        context,
        contract_update_url(context),
        {"did": did, "updated_at": updated_at, "name": name},
        headers=context.contract_seed_headers[name],
    )
    assert response.status_code == 200, response.text
    contract = ContractService._refresh_contract(context, name)
    assert contract.get("name") == name, (
        f"Expected the persisted contract name {name!r}, got {contract.get('name')!r}"
    )


@given('named contract "{name}" has reached contract state "{state}"')
def named_contract_reached_state(context, name, state):
    _create_named_contract_in_draft(context, name)
    normalized = state.strip().upper()
    if normalized == "DRAFT":
        return
    assert normalized == "SIGNED", (
        f"The named UI-search fixture only supports DRAFT and SIGNED, got {state!r}"
    )
    _advance_to_approved(context, name)
    _apply_signature(context, name)
    ContractService._refresh_contract(context, name)


@given('contract "{name}" is present in the Archive Store')
def contract_present_in_archive_store(context, name):
    did, _ = ContractService._contract_data(context, name)
    headers = AuthService.get_headers_for_roles(["Archive Manager"])
    deadline = time.monotonic() + 90
    entries = []
    while time.monotonic() < deadline:
        response = get_with_headers(context, archive_retrieve_url(context), headers=headers)
        assert response.status_code == 200, response.text
        body = response.json()
        entries = body.get("contracts", []) if isinstance(body, dict) else body
        if any(isinstance(entry, dict) and entry.get("did") == did for entry in entries):
            return
        time.sleep(2)
    raise AssertionError(
        f"Expected signed contract {name!r} ({did}) to be stored through the Archive Store, got {entries!r}"
    )


@given('I am signed in through the DCS login page and test wallet as "{role}"')
@when('I sign in through the DCS login page and test wallet as "{role}"')
def signed_in(context, role):
    _login(context, role)


@when("I open the DCS login page in the browser")
def open_login(context):
    _goto(_page(context), f"{_ui_base(context)}/")


@when('I present the test wallet credential for role "{role}" from the browser login challenge')
def present_login_wallet(context, role):
    _present_wallet(context, role, "auth-oid4vp-presentation")


@when('I present the test wallet credential for role "{role}" from UI element "{test_id}"')
def present_wallet_from_element(context, role, test_id):
    _present_wallet(context, role, test_id)


@then("the browser session was not established by token or storage injection")
def no_injection(context):
    assert context.ui_auth_established_via == "oid4vp-test-wallet"


@when('I open UI route "{route}"')
def open_route(context, route):
    _goto(_page(context), f"{_ui_base(context)}{route}")


@when('I navigate using remembered key "{alias}" to UI route "{route}"')
def open_route_with_remembered_key(context, alias, route):
    keys = getattr(context, "ui_test_keys", {})
    assert alias in keys, f"No remembered UI business key named {alias!r}"
    open_route(context, f"{route.rstrip('/')}/{keys[alias]}")


@when('I open the edit UI for template "{name}"')
def open_template_edit(context, name):
    open_route(context, f"/templates/edit/{_template_did(context, name)}")


@when('I open the details UI for template "{name}"')
def open_template_details(context, name):
    open_route(context, f"/templates/view/{_template_did(context, name)}")


@when('I open the {view:w} UI for contract "{name}"')
def open_contract_view(context, view, name):
    route = {
        "edit": "edit",
        "negotiation": "negotiate",
        "review": "review",
        "approval": "approve",
        "management": "view",
        "signing": "signing",
        "compliance": "signature/compliance",
    }[view]
    open_route(context, f"/{route}/{_contract_did(context, name)}" if view in {"signing", "compliance"} else f"/contracts/{route}/{_contract_did(context, name)}")


@when('I reopen the {view:w} UI for contract "{name}"')
def reopen_contract_view(context, view, name):
    open_contract_view(context, view, name)


@when("I reload the current UI route")
def reload_current_route(context):
    _page(context).reload(wait_until="domcontentloaded")


@then('current UI route is "{route}"')
def current_ui_route(context, route):
    from playwright.sync_api import expect

    expect(_page(context)).to_have_url(
        re.compile(rf"{re.escape(route)}(?:[?#].*)?$"), timeout=30_000
    )


@when('I open the contract from UI element "{source_test_id}"')
def open_contract_from_source(context, source_test_id):
    open_route(context, f"/contracts/view/{_source_key(context, source_test_id)}")


@when("I fill these UI controls")
def fill_controls(context):
    for row in context.table:
        _element(context, row["test_id"]).fill(row["value"])


@when('I fill UI control "{test_id}" with "{value}"')
def fill_control(context, test_id, value):
    _element(context, test_id).fill(value)


@when('I fill UI control "{test_id}" with the DID of contract "{name}"')
def fill_contract_did(context, test_id, name):
    _element(context, test_id).fill(_contract_did(context, name))


@when('I fill UI control "{test_id}" with the DID of template "{name}"')
def fill_template_did(context, test_id, name):
    _element(context, test_id).fill(_template_did(context, name))


@when('I click UI control "{test_id}"')
def click_control(context, test_id):
    _element(context, test_id).click()


@when('I click keyed UI control "{test_id}" with key "{key}"')
def click_key(context, test_id, key):
    _element(context, test_id, key).click()


@when('I click keyed UI control "{test_id}" from "{source_test_id}"')
def click_source_key(context, test_id, source_test_id):
    _element(context, test_id, _source_key(context, source_test_id)).click()


@when('I remember data-test-key from UI element "{source_test_id}" as "{alias}"')
def remember_source_key(context, source_test_id, alias):
    if not hasattr(context, "ui_test_keys"):
        context.ui_test_keys = {}
    context.ui_test_keys[alias] = _source_key(context, source_test_id)


@when('I click keyed UI control "{test_id}" with remembered key "{alias}"')
def click_remembered_key(context, test_id, alias):
    keys = getattr(context, "ui_test_keys", {})
    assert alias in keys, f"No remembered UI business key named {alias!r}"
    _element(context, test_id, keys[alias]).click()


@then('keyed UI element "{test_id}" with remembered key "{alias}" is visible')
def remembered_key_visible(context, test_id, alias):
    from playwright.sync_api import expect

    keys = getattr(context, "ui_test_keys", {})
    assert alias in keys, f"No remembered UI business key named {alias!r}"
    expect(_element(context, test_id, keys[alias])).to_be_visible()


@then('keyed UI element "{test_id}" with remembered key "{alias}" contains "{expected}"')
def remembered_key_contains(context, test_id, alias, expected):
    keys = getattr(context, "ui_test_keys", {})
    assert alias in keys, f"No remembered UI business key named {alias!r}"
    _expect_contains(_element(context, test_id, keys[alias]), expected)


@when('I click template UI control "{test_id}" for "{name}"')
def click_template_key(context, test_id, name):
    _element(context, test_id, _template_did(context, name)).click()


@when('I click contract UI control "{test_id}" for "{name}"')
def click_contract_key(context, test_id, name):
    _element(context, test_id, _contract_did(context, name)).click()


@when('I select UI option "{value}" in control "{test_id}"')
def select_option(context, value, test_id):
    _element(context, test_id).select_option(value=value)


@when('I select template "{name}" in UI control "{test_id}"')
def select_template(context, name, test_id):
    _element(context, test_id).select_option(value=_template_did(context, name))


@when('I select finding from contract "{name}" in UI control "{test_id}"')
def select_contract_finding(context, name, test_id):
    _element(context, test_id).select_option(value=_contract_did(context, name))


@when('I complete the template review for "{name}" through the browser')
def complete_template_review(context, name):
    open_route(context, "/tasks/reviews")
    click_template_key(context, "template-review-task-open", name)
    click_control(context, "template-review-verify")
    click_control(context, "template-verification-close")
    click_control(context, "template-review-forward-approval")
    fill_control(context, "template-review-comment", "Verified for approval through browser")
    click_control(context, "template-review-comment-submit")


@when('I complete contract review for "{name}" through the browser')
def complete_contract_review(context, name):
    open_contract_view(context, "review", name)
    click_control(context, "contract-review-verify")
    click_control(context, "contract-review-forward-approval")
    fill_control(context, "contract-review-finding", "Verified for approval through browser")
    click_control(context, "contract-review-finding-submit")


@when("I sign out through the UI")
def sign_out(context):
    _element(context, "auth-logout").click()
    _element(context, "auth-oid4vp-presentation").wait_for(state="visible")


@then('UI element "{test_id}" is visible')
def visible(context, test_id):
    from playwright.sync_api import expect

    expect(_element(context, test_id)).to_be_visible(timeout=_UI_EXPECT_TIMEOUT_MS)


@then('UI element "{test_id}" is absent')
def absent(context, test_id):
    from playwright.sync_api import expect

    expect(_element(context, test_id)).to_have_count(0)


@then('keyed UI element "{test_id}" with key "{key}" is absent')
def keyed_absent(context, test_id, key):
    from playwright.sync_api import expect

    expect(_element(context, test_id, key)).to_have_count(0)


@then('keyed UI control "{test_id}" with key "{key}" is disabled')
def keyed_disabled(context, test_id, key):
    from playwright.sync_api import expect

    expect(_element(context, test_id, key)).to_be_disabled(timeout=_UI_EXPECT_TIMEOUT_MS)


@then('keyed UI control "{test_id}" with key "{key}" is enabled')
def keyed_enabled(context, test_id, key):
    from playwright.sync_api import expect

    expect(_element(context, test_id, key)).to_be_enabled(timeout=_UI_EXPECT_TIMEOUT_MS)


@then('UI element "{test_id}" contains "{expected}"')
def contains(context, test_id, expected):
    _expect_contains(_element(context, test_id), expected)


@then('keyed UI element "{test_id}" with key "{key}" is visible')
def key_visible(context, test_id, key):
    from playwright.sync_api import expect

    expect(_element(context, test_id, key)).to_be_visible(timeout=_UI_EXPECT_TIMEOUT_MS)


@then('keyed UI element "{test_id}" with key "{key}" contains "{expected}"')
def key_contains(context, test_id, key, expected):
    _expect_contains(_element(context, test_id, key), expected)


@then('keyed UI element "{test_id}" from "{source_test_id}" is visible')
def source_key_visible(context, test_id, source_test_id):
    from playwright.sync_api import expect

    expect(_element(context, test_id, _source_key(context, source_test_id))).to_be_visible(
        timeout=_UI_EXPECT_TIMEOUT_MS
    )


@then('keyed UI element "{test_id}" from "{source_test_id}" contains "{expected}"')
def source_key_contains(context, test_id, source_test_id, expected):
    _expect_contains(_element(context, test_id, _source_key(context, source_test_id)), expected)


@then('template UI element "{test_id}" for "{name}" is visible')
def template_key_visible(context, test_id, name):
    from playwright.sync_api import expect

    expect(_element(context, test_id, _template_did(context, name))).to_be_visible()


@then('template UI element "{test_id}" for "{name}" is absent')
def template_key_absent(context, test_id, name):
    from playwright.sync_api import expect

    expect(_element(context, test_id, _template_did(context, name))).to_have_count(0)


@then('template UI element "{test_id}" for "{name}" contains "{expected}"')
def template_key_contains(context, test_id, name, expected):
    timeout = 90_000 if test_id == "template-search-result" else None
    _expect_contains(
        _element(context, test_id, _template_did(context, name)),
        expected,
        timeout=timeout,
    )


@then('the component picker shows template "{name}" with its name and DID')
def component_picker_shows_name_and_did(context, name):
    from playwright.sync_api import expect

    did = _template_did(context, name)
    picker = _element(context, "template-component-reference")
    option = _element(context, "template-component-option", did)
    expect(picker).to_be_visible(timeout=_UI_EXPECT_TIMEOUT_MS)
    expect(picker).to_have_attribute(
        "list",
        "template-component-reference-options",
        timeout=_UI_EXPECT_TIMEOUT_MS,
    )
    expect(option).to_have_count(1)
    expect(option).to_contain_text(name, timeout=_UI_EXPECT_TIMEOUT_MS)
    expect(option).to_have_attribute("value", did, timeout=_UI_EXPECT_TIMEOUT_MS)


@then(
    'persisted template with remembered key "{alias}" contains a complete snapshot of component "{name}"'
)
def persisted_template_contains_complete_component_snapshot(context, alias, name):
    keys = getattr(context, "ui_test_keys", {})
    assert alias in keys, f"No remembered UI business key named {alias!r}"
    creator_headers = AuthService.get_headers_for_roles(["Template Creator"])
    persisted = TemplateService.fetch_template(context, keys[alias], headers=creator_headers)
    metadata = (persisted.get("template_data") or {}).get("dcs:metadata") or {}
    snapshots = metadata.get("dcs:subTemplates") or []
    component_did = _template_did(context, name)
    source_component = TemplateService.fetch_template(
        context,
        component_did,
        headers=creator_headers,
    )
    snapshot = next(
        (
            item
            for item in snapshots
            if isinstance(item, dict) and item.get("@id") == component_did
        ),
        None,
    )
    assert snapshot is not None, (
        f"Persisted template {keys[alias]!r} has no snapshot for component {component_did!r}: "
        f"{snapshots!r}"
    )
    assert snapshot.get("@id") == source_component.get("did"), (
        f"Component snapshot DID differs from authoritative source: {snapshot!r}"
    )
    assert snapshot.get("dcs:version") == source_component.get("version"), (
        f"Component snapshot version differs from authoritative source: "
        f"snapshot={snapshot!r}, source={source_component!r}"
    )
    assert snapshot.get("dcs:template") == source_component.get("template_data"), (
        f"Component snapshot template_data differs from authoritative source: "
        f"snapshot={snapshot!r}, source={source_component!r}"
    )


@then('contract UI element "{test_id}" for "{name}" is visible')
def contract_key_visible(context, test_id, name):
    from playwright.sync_api import expect

    timeout = 90_000 if test_id == "audit-result" else None
    expect(_element(context, test_id, _contract_did(context, name))).to_be_visible(timeout=timeout)


@then('contract UI element "{test_id}" for "{name}" contains "{expected}"')
def contract_key_contains(context, test_id, name, expected):
    timeout = 90_000 if test_id == "audit-result" else None
    _expect_contains(
        _element(context, test_id, _contract_did(context, name)),
        expected,
        timeout=timeout,
    )


@then('contract UI element "{test_id}" for "{name}" contains an authoritative timestamp')
def contract_key_contains_timestamp(context, test_id, name):
    from playwright.sync_api import expect

    node = _element(context, test_id, _contract_did(context, name))
    timeout = 90_000 if test_id == "audit-result" else None
    expect(node).to_be_visible(timeout=timeout)
    if test_id == "audit-result":
        timestamp = node.locator(_selector("audit-event-timestamp")).first
        expect(timestamp).to_be_visible(timeout=timeout)
        value = timestamp.get_attribute("datetime") or ""
        assert re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})", value), (
            f"Expected authoritative RFC3339 timestamp, got {value!r}"
        )
        return
    expect(node).to_contain_text(
        re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}"),
        timeout=timeout,
    )


@then('UI element "{test_id}" contains the DID of template "{name}"')
def contains_template_did(context, test_id, name):
    contains(context, test_id, _template_did(context, name))


@then('UI element "{test_id}" contains the DID of contract "{name}"')
def contains_contract_did(context, test_id, name):
    contains(context, test_id, _contract_did(context, name))


@then('template option for "{name}" in UI control "{test_id}" is absent')
def template_option_absent(context, name, test_id):
    from playwright.sync_api import expect

    expect(_element(context, f"{test_id}-option", _template_did(context, name))).to_have_count(0)


@then('exactly {expected_count:d} UI element "{test_id}" is visible')
def exact_visible_count(context, expected_count, test_id):
    from playwright.sync_api import expect

    nodes = _element(context, test_id)
    expect(nodes).to_have_count(expected_count)
    for node in nodes.all():
        expect(node).to_be_visible()


@then('a browser download from UI control "{test_id}" is produced')
def download_produced(context, test_id):
    with _page(context).expect_download() as download_info:
        _element(context, test_id).click()
    assert download_info.value.suggested_filename


@then('a non-empty browser download from UI control "{test_id}" is produced')
def non_empty_download_produced(context, test_id):
    with _page(context).expect_download() as download_info:
        _element(context, test_id).click()
    download = download_info.value
    assert download.suggested_filename
    path = download.path()
    assert path is not None and path.stat().st_size > 0


@then('a browser download from UI control "{test_id}" contains the DID of contract "{name}"')
def download_contains_contract_did(context, test_id, name):
    with _page(context).expect_download() as download_info:
        _element(context, test_id).click()
    path = download_info.value.path()
    assert path is not None
    payload = path.read_bytes()
    assert _contract_did(context, name).encode("utf-8") in payload


@given("the UI gap resolution acceptance features are loaded")
def ui_gap_features_loaded(context):
    feature_root = Path(__file__).resolve().parents[2] / "features" / "24_ui_traceability"
    context.ui_gap_feature_sources = {
        path: path.read_text(encoding="utf-8") for path in sorted(feature_root.glob("*.feature"))
    }
    assert context.ui_gap_feature_sources


@then("every UI gap resolution AC from 1 through 22 is tagged for UI execution")
def every_gap_ac_is_ui_tagged(context):
    sources = getattr(context, "ui_gap_feature_sources", {})
    for ac_number in range(1, 23):
        tag = f"@REQ-ui-gap-resolution-AC{ac_number}"
        matching = [
            (path, body)
            for path, body in sources.items()
            if re.search(rf"{re.escape(tag)}(?!\d)", body)
        ]
        assert matching, f"Missing acceptance tag {tag}"
        for path, body in matching:
            feature_preamble = body.split("Feature:", 1)[0]
            assert "@ui" in feature_preamble, f"{path} is not tagged @ui"


@then("the current UI scenario has an isolated Playwright browser context")
def isolated_playwright_context(context):
    assert _page(context).context is context.browser_context
    assert context.ui_artifact_dir.name


@then("UI selectors are restricted to semantic test IDs and business keys")
def semantic_selectors_only(context):
    assert _selector("domain-capability-control") == '[data-test-id="domain-capability-control"]'
    assert _selector("domain-record", "did:web:example") == (
        '[data-test-id="domain-record"][data-test-key="did:web:example"]'
    )
    try:
        _selector("div.card")
    except AssertionError:
        pass
    else:
        raise AssertionError("CSS-shaped selector was accepted as a semantic data-test-id")


@then("UI failure evidence is configured for trace, screenshot, video, and isolated JUnit")
def ui_failure_evidence_configured(context):
    repository = Path(__file__).resolve().parents[2]
    environment_source = (repository / "environment.py").read_text(encoding="utf-8")
    make_source = (repository / "tests" / "bdd" / "Makefile").read_text(encoding="utf-8")
    workflow_source = (repository / ".github" / "workflows" / "bdd-kind.yml").read_text(encoding="utf-8")
    for token in ("record_video_dir", "tracing.start", "failure.png", "trace.zip"):
        assert token in environment_source
    assert "$(REPORTS_ABS)/ui/junit" in make_source
    assert "tests/bdd/.reports/ui/junit/**/*.xml" in workflow_source
