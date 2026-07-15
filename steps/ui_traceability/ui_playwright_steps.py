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
import subprocess
import sys
import time
from pathlib import Path

from behave import given, then, when

from steps.support.api_client import origin_url
from steps.support.services.contract_service import ContractService
from steps.support.services.template_service import TemplateService


_TEST_ID = re.compile(r"^[a-z0-9-]+$")


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


def _ui_base(context) -> str:
    configured = os.getenv("BDD_DCS_UI_URL", "").strip()
    if configured:
        return configured.rstrip("/")
    return f"{origin_url(context.base_url)}/digital-contracting-service/ui"


def _goto(page, url: str) -> None:
    from playwright.sync_api import Error as PlaywrightError

    try:
        page.goto(url, wait_until="domcontentloaded")
    except PlaywrightError as exc:
        if "ERR_NETWORK_CHANGED" not in str(exc):
            raise
        time.sleep(0.25)
        page.goto(url, wait_until="domcontentloaded")


def _template_did(context, name: str) -> str:
    return str(TemplateService.named(context, name)["did"])


def _contract_did(context, name: str) -> str:
    return str(ContractService._contract_data(context, name)[0])


def _source_key(context, source_test_id: str) -> str:
    key = _element(context, source_test_id).get_attribute("data-test-key")
    assert key, f"UI element {source_test_id!r} has no data-test-key"
    return key


def _credential_for_role(role: str) -> str:
    env_name = "BDD_UI_WALLET_CREDENTIAL_" + re.sub(r"[^A-Z0-9]+", "_", role.upper()).strip("_")
    override = os.getenv(env_name, "").strip()
    if override:
        return override
    credentials = Path(__file__).resolve().parents[2] / "testWallet" / "credentials"
    for template in sorted(credentials.glob("*.template.json")):
        body = json.loads(template.read_text(encoding="utf-8"))
        if role in (body.get("roles") or []):
            return template.name.removesuffix(".template.json")
    raise AssertionError(f"No testWallet credential declares role {role!r}")


def _present_wallet(context, role: str, presentation_test_id: str) -> None:
    node = _element(context, presentation_test_id)
    node.wait_for(state="visible")
    if presentation_test_id == "auth-oid4vp-presentation":
        from playwright.sync_api import expect

        expect(node).to_have_attribute("data-login-challenge-bound", "true", timeout=30_000)
    presentation_url = node.get_attribute("data-presentation-url")
    assert presentation_url, f"{presentation_test_id!r} must expose data-presentation-url"
    wallet = Path(__file__).resolve().parents[2] / "testWallet" / "demo_wallet.py"
    result = subprocess.run(
        [sys.executable, str(wallet), "--presentation-url", presentation_url, "--credential", _credential_for_role(role)],
        cwd=wallet.parent.parent,
        env={
            **os.environ,
            "DCS_WALLET_STATUSLIST_SERVICE_BASE": os.environ.get(
                "BDD_CREDENTIAL_STATUSLIST_SERVICE_URL", ""
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


@when('I open the edit UI for template "{name}"')
def open_template_edit(context, name):
    open_route(context, f"/templates/edit/{_template_did(context, name)}")


@when('I open the {view:w} UI for contract "{name}"')
def open_contract_view(context, view, name):
    route = {"negotiation": "negotiate", "review": "review", "approval": "approve", "management": "view", "signing": "signing", "compliance": "signature/compliance"}[view]
    open_route(context, f"/{route}/{_contract_did(context, name)}" if view in {"signing", "compliance"} else f"/contracts/{route}/{_contract_did(context, name)}")


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


@when("I sign out through the UI")
def sign_out(context):
    _element(context, "auth-logout").click()
    _element(context, "auth-oid4vp-presentation").wait_for(state="visible")


@then('UI element "{test_id}" is visible')
def visible(context, test_id):
    _element(context, test_id).wait_for(state="visible")


@then('UI element "{test_id}" is absent')
def absent(context, test_id):
    assert _element(context, test_id).count() == 0


@then('UI element "{test_id}" contains "{expected}"')
def contains(context, test_id, expected):
    node = _element(context, test_id)
    node.wait_for(state="visible")
    actual = node.input_value() if node.evaluate("element => element.matches('input, textarea, select')") else node.inner_text()
    assert expected.casefold() in actual.casefold()


@then('keyed UI element "{test_id}" with key "{key}" is visible')
def key_visible(context, test_id, key):
    _element(context, test_id, key).wait_for(state="visible")


@then('keyed UI element "{test_id}" with key "{key}" contains "{expected}"')
def key_contains(context, test_id, key, expected):
    node = _element(context, test_id, key)
    node.wait_for(state="visible")
    assert expected.casefold() in node.inner_text().casefold()


@then('keyed UI element "{test_id}" from "{source_test_id}" is visible')
def source_key_visible(context, test_id, source_test_id):
    _element(context, test_id, _source_key(context, source_test_id)).wait_for(state="visible")


@then('keyed UI element "{test_id}" from "{source_test_id}" contains "{expected}"')
def source_key_contains(context, test_id, source_test_id, expected):
    node = _element(context, test_id, _source_key(context, source_test_id))
    node.wait_for(state="visible")
    assert expected.casefold() in node.inner_text().casefold()


@then('template UI element "{test_id}" for "{name}" is visible')
def template_key_visible(context, test_id, name):
    _element(context, test_id, _template_did(context, name)).wait_for(state="visible")


@then('contract UI element "{test_id}" for "{name}" is visible')
def contract_key_visible(context, test_id, name):
    _element(context, test_id, _contract_did(context, name)).wait_for(state="visible")


@then('UI element "{test_id}" contains the DID of template "{name}"')
def contains_template_did(context, test_id, name):
    contains(context, test_id, _template_did(context, name))


@then('UI element "{test_id}" contains the DID of contract "{name}"')
def contains_contract_did(context, test_id, name):
    contains(context, test_id, _contract_did(context, name))


@then('a browser download from UI control "{test_id}" is produced')
def download_produced(context, test_id):
    with _page(context).expect_download() as download_info:
        _element(context, test_id).click()
    assert download_info.value.suggested_filename
