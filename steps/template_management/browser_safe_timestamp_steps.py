"""BDD assertions for browser-safe timestamps at public API boundaries."""

import re
import time
from datetime import datetime

import requests
from behave import then, when

from steps.support.api_client import (
    contract_audit_url,
    contract_retrieve_by_id_url,
    get_with_headers,
    post_json,
    signature_audit_url,
    template_audit_url,
)
from steps.support.services.auth_service import AuthService
from steps.support.services.contract_service import ContractService
from steps.support.services.template_service import TemplateService


_RFC3339 = re.compile(
    r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$"
)


def _assert_browser_safe_rfc3339(value, source):
    assert isinstance(value, str) and _RFC3339.fullmatch(value), (
        f"Expected {source} to be an RFC3339 browser-safe timestamp, got {value!r}"
    )
    parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    assert parsed.tzinfo is not None, f"Expected {source} to include an explicit timezone: {value!r}"


def _poll_audit_entries(context, request_entries, expected_event_type, source):
    deadline = time.monotonic() + 90
    entries = []
    while time.monotonic() < deadline:
        response = request_entries()
        assert response.status_code == 200, (
            f"{source} query failed: {response.status_code} {response.text}"
        )
        entries = response.json()
        assert isinstance(entries, list), f"Expected {source} entries to be a list, got: {entries!r}"
        if any(str(entry.get("event_type", "")).upper() == expected_event_type for entry in entries):
            return entries
        time.sleep(1)
    raise AssertionError(
        f"Expected {expected_event_type} in {source} before timeout, got: "
        f"{[entry.get('event_type') for entry in entries]}"
    )


@then('every template audit entry for "{name}" has an RFC3339 browser-safe created_at timestamp')
def step_template_audit_timestamps(context, name):
    template = TemplateService.named(context, name)
    headers = AuthService.get_headers_for_roles(["Auditor"])
    entries = _poll_audit_entries(
        context,
        lambda: requests.get(
            template_audit_url(context),
            params={"did": template["did"]},
            headers=headers,
            timeout=context.http_timeout_seconds,
        ),
        "CREATE_CONTRACT_TEMPLATE",
        "template audit",
    )
    for index, entry in enumerate(entries):
        _assert_browser_safe_rfc3339(entry.get("created_at"), f"template audit entry {index} created_at")


@then('every contract audit entry for "{name}" has an RFC3339 browser-safe created_at timestamp')
def step_contract_audit_timestamps(context, name):
    did, _ = ContractService._contract_data(context, name)
    headers = AuthService.get_headers_for_roles(["Auditor"])
    entries = _poll_audit_entries(
        context,
        lambda: post_json(context, contract_audit_url(context), {"did": did}, headers=headers),
        "CREATE_CONTRACT",
        "contract audit",
    )
    for index, entry in enumerate(entries):
        _assert_browser_safe_rfc3339(entry.get("created_at"), f"contract audit entry {index} created_at")


@then('every signature audit entry for "{name}" has an RFC3339 browser-safe created_at timestamp')
def step_signature_audit_timestamps(context, name):
    did, _ = ContractService._contract_data(context, name)
    headers = AuthService.get_headers_for_roles(["Auditor"])
    entries = _poll_audit_entries(
        context,
        lambda: requests.get(
            signature_audit_url(context),
            params={"did": did},
            headers=headers,
            timeout=context.http_timeout_seconds,
        ),
        "APPLIED_SIGNATURE",
        "signature audit",
    )
    for index, entry in enumerate(entries):
        _assert_browser_safe_rfc3339(entry.get("created_at"), f"signature audit entry {index} created_at")


@when('I retrieve contract "{name}" for timestamp verification')
def step_retrieve_contract_for_timestamp_verification(context, name):
    did, _ = ContractService._contract_data(context, name)
    headers = AuthService.get_headers_for_roles(["Contract Manager"])
    context.requests_response = get_with_headers(
        context,
        contract_retrieve_by_id_url(context, did),
        headers=headers,
    )


@then("every returned negotiation has an RFC3339 browser-safe created_at timestamp")
def step_negotiation_timestamps(context):
    assert context.requests_response.status_code == 200, (
        f"Contract retrieve failed: {context.requests_response.status_code} "
        f"{context.requests_response.text}"
    )
    negotiations = context.requests_response.json().get("negotiations") or []
    assert negotiations, "Expected at least one negotiation in the retrieved contract"
    for index, negotiation in enumerate(negotiations):
        _assert_browser_safe_rfc3339(
            negotiation.get("created_at"),
            f"retrieved negotiation {index} created_at",
        )
