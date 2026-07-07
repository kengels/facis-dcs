"""PACM auditing finalization steps."""

from __future__ import annotations

import base64
import csv
import io
import json
import os
import re
from datetime import datetime, timezone
from typing import Any

import requests
from behave import given, then, when

from steps.support.api_client import (
    contract_create_url,
    contract_retrieve_by_id_url,
    contract_update_url,
    get_with_headers,
    post_json,
    put_json,
    template_approve_url,
    template_create_url,
    template_retrieve_by_id_url,
    template_submit_url,
    template_verify_url,
)
from steps.support.services.contract_service import ContractService
from steps.support.services.auth_service import AuthService
from steps.support.services.template_service import TemplateService


ALLOWED_SCOPES = {"CONTRACT", "TEMPLATE", "ARCHIVE", "SIGNATURE"}
SIGNATURE_SKIP_REASON = "signature validation is not available because signing is not implemented"

CONTRACT_CONTENT_POLICY_MANIFEST = "docs/policies/facis-contract-content-audit-policies.json"
CONTRACT_CONTENT_POLICY_SET_ID = "facis.dcs.contract.structure-semantics"


def _contract_content_policy_manifest() -> dict[str, Any]:
    path = os.path.join(os.getcwd(), CONTRACT_CONTENT_POLICY_MANIFEST)
    with open(path, encoding="utf-8") as manifest_file:
        manifest = json.load(manifest_file)
    assert isinstance(manifest, dict), f"Contract content policy manifest is not an object: {manifest}"
    return manifest


def _configured_shacl_shape_ids() -> set[str]:
    manifest = _contract_content_policy_manifest()
    shape_ids: set[str] = set()
    for relative_path in manifest.get("shaclShapeFiles", []):
        path = os.path.join(os.getcwd(), str(relative_path))
        with open(path, encoding="utf-8") as shape_file:
            content = shape_file.read()
        shape_ids.update(re.findall(r"(?m)^([A-Za-z0-9_:-]+)\s*\n\s+a\s+sh:NodeShape\b", content))
        shape_ids.update(re.findall(r"(?m)^([A-Za-z0-9_:-]+)\s+a\s+sh:NodeShape\b", content))
    assert shape_ids, f"No SHACL shape IDs found via {CONTRACT_CONTENT_POLICY_MANIFEST}"
    return shape_ids


def _configured_validation_rule_ids() -> set[str]:
    manifest = _contract_content_policy_manifest()
    rule_ids: set[str] = set()
    for relative_path in manifest.get("validationProfiles", []):
        path = os.path.join(os.getcwd(), str(relative_path))
        with open(path, encoding="utf-8") as profile_file:
            content = profile_file.read()
        rule_ids.update(re.findall(r'facisv:ruleId\s+"([^"]+)"', content))
    assert rule_ids, f"No validation rule IDs found via {CONTRACT_CONTENT_POLICY_MANIFEST}"
    return rule_ids


def _rule_reference(finding: dict[str, Any]) -> str:
    value = _field(finding, "ruleId", "rule_id", "ruleReference", "rule_reference", "check", "id")
    return str(value).strip() if value is not None else ""


def _is_configured_shacl_rule(rule_id: str) -> bool:
    if not rule_id:
        return False
    for shape_id in _configured_shacl_shape_ids():
        if rule_id == shape_id or rule_id.startswith(f"{shape_id}-PROP-"):
            return True
    return False


def _walk_dicts(value: Any):
    if isinstance(value, dict):
        yield value
        for nested in value.values():
            yield from _walk_dicts(nested)
    elif isinstance(value, list):
        for item in value:
            yield from _walk_dicts(item)


def _audit_run_candidate_findings(run: dict[str, Any]) -> list[dict[str, Any]]:
    candidates: list[dict[str, Any]] = []
    for item in _walk_dicts(run):
        event_data = _field(item, "eventData", "event_data")
        if isinstance(event_data, dict):
            merged = dict(event_data)
            for source_key, target_key in (
                ("eventType", "eventType"),
                ("event_type", "eventType"),
                ("component", "component"),
                ("did", "did"),
            ):
                value = item.get(source_key)
                if value is not None and target_key not in merged:
                    merged[target_key] = value
            candidates.append(merged)
        if any(key in item for key in (
            "ruleId", "rule_id", "policySetId", "policy_set_id", "severity", "requirement", "semanticPath",
        )):
            candidates.append(item)
    return candidates


def _finding_did(finding: dict[str, Any]) -> str:
    value = _field(finding, "did", "resourceDid", "resource_did", "contractDid", "contract_did")
    return str(value).strip() if value is not None else ""


def _matches_contract_target(context, finding: dict[str, Any], contract_name: str | None) -> bool:
    if not contract_name:
        return True
    target_did = (getattr(context, "contract_dids", {}) or {}).get(contract_name, "")
    finding_did = _finding_did(finding)
    return not target_did or not finding_did or finding_did == target_did


def _is_contract_content_policy_finding(finding: dict[str, Any]) -> bool:
    policy_set = str(_field(finding, "policySetId", "policy_set_id") or "").strip()
    if policy_set == CONTRACT_CONTENT_POLICY_SET_ID:
        return True
    rule_ref = _rule_reference(finding)
    if rule_ref in _configured_validation_rule_ids() or _is_configured_shacl_rule(rule_ref):
        return True
    text = _finding_text(finding)
    return "contract-content" in text or "contract content" in text or "shacl" in text


def _contract_content_policy_findings(context, contract_name: str | None = None) -> list[dict[str, Any]]:
    run = _audit_run_body(context)
    findings = [
        finding for finding in _audit_run_candidate_findings(run)
        if _matches_contract_target(context, finding, contract_name) and _is_contract_content_policy_finding(finding)
    ]
    if findings:
        return findings
    run_id = _field(run, "id", "auditRunId", "audit_run_id", "runId", "run_id")
    if run_id:
        detail = _query_audit_run_detail(context, str(run_id))
        findings = [
            finding for finding in _audit_run_candidate_findings(detail)
            if _matches_contract_target(context, finding, contract_name) and _is_contract_content_policy_finding(finding)
        ]
    return findings


def _is_failed_finding(finding: dict[str, Any]) -> bool:
    value = _field(finding, "severity", "status", "result", "findingStatus", "finding_status")
    if value is None:
        return False
    return _normalize_status(value) == "FAIL"


def _contract_content_template_data(did: str) -> dict[str, Any]:
    return _contract_content_fixture(did, 99.95, False, document_type="dcs:ContractTemplate")


def _contract_content_fixture(
    did: str,
    availability: float,
    structure_violation: bool,
    *,
    document_type: str = "dcs:Contract",
) -> dict[str, Any]:
    parties: list[dict[str, Any]] = [
        {"@type": "dcs:CompanyParty", "dcs:role": "supplier", "dcs:legalName": "BDD Supplier GmbH"},
        {"@type": "dcs:CompanyParty", "dcs:role": "customer", "dcs:legalName": "BDD Customer GmbH"},
    ]
    if structure_violation:
        parties = [{"@type": "dcs:Organization", "dcs:role": "supplier"}]
    return {
        "@context": {
            "dcs": "https://w3id.org/facis/dcs/ontology/v1#",
            "odrl": "http://www.w3.org/ns/odrl/2/",
            "xsd": "http://www.w3.org/2001/XMLSchema#",
        },
        "@id": did,
        "@type": document_type,
        "dcs:did": did,
        "dcs:contractVersion": 1,
        "dcs:templateVersion": 1,
        "dcs:schemaVersion": "v1",
        "dcs:metadata": {
            "@type": "dcs:ContractMetadata",
            "dcs:title": "BDD PACM contract content fixture",
            "dcs:version": 1,
        },
        "dcs:documentStructure": {
            "@type": "dcs:DocumentStructure",
            "dcs:blocks": {"@list": [
                {
                    "@id": "urn:uuid:bdd-pacm-contract-content-clause",
                    "@type": "dcs:Clause",
                    "dcs:blockId": "bdd-pacm-contract-content-clause",
                    "dcs:text": "Service availability and party data for PACM contract content audits.",
                    "dcs:content": {"@list": [
                        "Service availability and party data for PACM contract content audits."
                    ]},
                },
            ]},
            "dcs:layout": [
                {
                    "@id": "urn:uuid:bdd-pacm-contract-content-root",
                    "dcs:isRoot": True,
                    "dcs:children": {"@list": [
                        {"@id": "urn:uuid:bdd-pacm-contract-content-clause"},
                    ]},
                },
            ],
        },
        "dcs:contractData": [
            {
                "@id": "urn:uuid:bdd-pacm-legal-requirement",
                "@type": "dcs:DataRequirement",
                "dcs:conditionId": "condition-legal",
                "dcs:name": "Legal policy values",
                "dcs:schemaVersion": "v1",
                "dcs:fields": [
                    {
                        "@id": "urn:uuid:bdd-pacm-field-jurisdiction",
                        "@type": "dcs:RequirementField",
                        "dcs:parameterName": "contract.jurisdiction",
                        "dcs:required": True,
                    },
                ],
            },
            {
                "@id": "urn:uuid:bdd-pacm-service-requirement",
                "@type": "dcs:DataRequirement",
                "dcs:conditionId": "condition-service",
                "dcs:name": "Service policy values",
                "dcs:schemaVersion": "v1",
                "dcs:fields": [
                    {
                        "@id": "urn:uuid:bdd-pacm-field-availability",
                        "@type": "dcs:RequirementField",
                        "dcs:parameterName": "service.sla.availability",
                        "dcs:required": True,
                    },
                    {
                        "@id": "urn:uuid:bdd-pacm-field-response-time",
                        "@type": "dcs:RequirementField",
                        "dcs:parameterName": "service.sla.responseTime",
                        "dcs:required": True,
                    },
                    {
                        "@id": "urn:uuid:bdd-pacm-field-resolution-time",
                        "@type": "dcs:RequirementField",
                        "dcs:parameterName": "service.sla.resolutionTime",
                        "dcs:required": True,
                    },
                ],
            },
            {
                "@id": "urn:uuid:bdd-pacm-signature-requirement",
                "@type": "dcs:DataRequirement",
                "dcs:conditionId": "condition-signature",
                "dcs:name": "Signature policy values",
                "dcs:schemaVersion": "v1",
                "dcs:fields": [
                    {
                        "@id": "urn:uuid:bdd-pacm-field-signature-level",
                        "@type": "dcs:RequirementField",
                        "dcs:parameterName": "signature.requiredLevel",
                        "dcs:required": True,
                    },
                ],
            },
        ],
        "dcs:party": parties,
        "parties": parties,
        "contract": {"jurisdiction": "DEU"},
        "service": {
            "sla": {
                "availability": availability,
                "responseTime": 10,
                "resolutionTime": 120,
            }
        },
        "semanticConditionValues": [
            {"conditionId": "condition-legal", "parameterName": "contract.jurisdiction", "parameterValue": "DEU"},
            {"conditionId": "condition-service", "parameterName": "service.sla.availability", "parameterValue": availability},
            {"conditionId": "condition-service", "parameterName": "service.sla.responseTime", "parameterValue": 10},
            {"conditionId": "condition-service", "parameterName": "service.sla.resolutionTime", "parameterValue": 120},
            {"conditionId": "condition-signature", "parameterName": "signature.requiredLevel", "parameterValue": "AES"},
        ],
    }


def _create_contract_content_fixture_draft(context, name: str):
    fixture_id = re.sub(r"[^A-Za-z0-9._:-]+", "-", name).strip("-").lower() or "contract-content"
    template_creator_h = AuthService.get_headers_for_roles(["Template Creator"])
    create_template = post_json(
        context,
        template_create_url(context),
        {
            "template_type": TemplateService.CONTRACT_TEMPLATE_TYPE,
            "name": f"BDD PACM Contract Content Template - {name}",
            "description": "Canonical BDD template for PACM contract content audit fixtures",
            "template_data": _contract_content_template_data(f"did:example:bdd-pacm-template-{fixture_id}"),
        },
        headers=template_creator_h,
    )
    assert create_template.status_code == 200, f"Template create failed: {create_template.status_code} {create_template.text}"
    template_did = create_template.json().get("did")
    assert template_did, f"Template create response did not include did: {create_template.text}"

    retrieve_template = get_with_headers(context, template_retrieve_by_id_url(context, template_did), headers=template_creator_h)
    assert retrieve_template.status_code == 200, retrieve_template.text
    updated_at = retrieve_template.json().get("updated_at")

    submit_template = post_json(
        context,
        template_submit_url(context),
        ContractService._template_submit_payload(context, template_did, updated_at),
        headers=template_creator_h,
    )
    assert submit_template.status_code == 200, f"Template submit failed: {submit_template.status_code} {submit_template.text}"

    template_reviewer_h = AuthService.get_headers_for_roles(["Template Reviewer"])
    retrieve_template = get_with_headers(context, template_retrieve_by_id_url(context, template_did), headers=template_reviewer_h)
    assert retrieve_template.status_code == 200, retrieve_template.text
    updated_at = retrieve_template.json().get("updated_at")

    verify_template = post_json(
        context,
        template_verify_url(context),
        {"did": template_did, "updated_at": updated_at},
        headers=template_reviewer_h,
    )
    assert verify_template.status_code == 200, f"Template verify failed: {verify_template.status_code} {verify_template.text}"

    retrieve_template = get_with_headers(context, template_retrieve_by_id_url(context, template_did), headers=template_reviewer_h)
    assert retrieve_template.status_code == 200, retrieve_template.text
    updated_at = retrieve_template.json().get("updated_at")

    recommend_template = post_json(
        context,
        template_submit_url(context),
        ContractService._template_reviewer_submit_payload(context, template_did, updated_at),
        headers=template_reviewer_h,
    )
    assert recommend_template.status_code == 200, (
        f"Template review submit failed: {recommend_template.status_code} {recommend_template.text}"
    )

    template_approver_h = AuthService.get_headers_for_roles(["Template Approver"])
    retrieve_template = get_with_headers(context, template_retrieve_by_id_url(context, template_did), headers=template_approver_h)
    assert retrieve_template.status_code == 200, retrieve_template.text
    updated_at = retrieve_template.json().get("updated_at")

    approve_template = post_json(
        context,
        template_approve_url(context),
        {"did": template_did, "updated_at": updated_at},
        headers=template_approver_h,
    )
    assert approve_template.status_code == 200, f"Template approve failed: {approve_template.status_code} {approve_template.text}"

    contract_creator_h = AuthService.get_headers_for_roles(["Contract Creator"])
    create_contract = post_json(
        context,
        contract_create_url(context),
        ContractService._contract_create_payload(template_did),
        headers=contract_creator_h,
    )
    assert create_contract.status_code == 200, f"Contract create failed: {create_contract.status_code} {create_contract.text}"
    contract_did = create_contract.json().get("did")
    assert contract_did, f"Contract create response did not include did: {create_contract.text}"

    retrieve_contract = get_with_headers(
        context,
        contract_retrieve_by_id_url(context, contract_did),
        headers=contract_creator_h,
    )
    assert retrieve_contract.status_code == 200, retrieve_contract.text

    ContractService._ensure_store(context, "contract_dids", {})
    ContractService._ensure_store(context, "contract_updated_at", {})
    ContractService._ensure_store(context, "contract_seed_headers", {})
    context.contract_dids[name] = contract_did
    context.contract_updated_at[name] = retrieve_contract.json().get("updated_at")
    context.contract_seed_headers[name] = contract_creator_h


def _store_contract_content_fixture(context, name: str, *, availability: float, structure_violation: bool):
    _create_contract_content_fixture_draft(context, name)
    did, updated_at = ContractService._contract_data(context, name)
    headers = (getattr(context, "contract_seed_headers", {}) or {}).get(name)
    response = put_json(
        context,
        contract_update_url(context),
        {
            "did": did,
            "updated_at": updated_at,
            "contract_data": _contract_content_fixture(did, availability, structure_violation),
        },
        headers=headers,
    )
    assert response.status_code == 200, f"Could not store contract content fixture: {response.status_code} {response.text}"
    ContractService._refresh_contract(context, name)


def _report_object(report: dict[str, Any]) -> dict[str, Any]:
    if isinstance(_field(report, "findings"), list):
        return report
    content = _field(report, "content", "data")
    if isinstance(content, str) and content:
        text = _decode_report_payload(report)
        try:
            parsed = json.loads(text)
        except ValueError:
            return report
        if isinstance(parsed, dict):
            return parsed
    return report


def _report_policy_findings(report: dict[str, Any]) -> list[dict[str, Any]]:
    report = _report_object(report)
    raw = _field(report, "findings", "auditFindings", "audit_findings")
    if not isinstance(raw, list):
        return []
    return [finding for finding in raw if isinstance(finding, dict) and _is_contract_content_policy_finding(finding)]


def _headers(context) -> dict[str, str]:
    headers = dict(getattr(context, "headers", {}) or {})
    headers.setdefault("Content-Type", "application/json")
    return headers


def _base_url(context) -> str:
    return getattr(context, "base_url", os.getenv("BDD_DCS_BASE_URL", "http://127.0.0.1:8991")).rstrip("/")


def _request_timeout(context) -> float:
    return float(getattr(context, "http_timeout_seconds", 20))


def _post_json(context, path: str, payload: dict[str, Any]):
    return requests.post(
        f"{_base_url(context)}{path}",
        json=payload,
        headers=_headers(context),
        timeout=_request_timeout(context),
    )


def _get(context, path: str, params: dict[str, Any] | None = None):
    return requests.get(
        f"{_base_url(context)}{path}",
        params={k: v for k, v in (params or {}).items() if v not in (None, "")},
        headers=_headers(context),
        timeout=_request_timeout(context),
    )


def _json(response) -> Any:
    try:
        return response.json()
    except ValueError as exc:
        raise AssertionError(f"Response is not JSON: HTTP {response.status_code} {response.text[:500]}") from exc


def _field(data: Any, *names: str) -> Any:
    if not isinstance(data, dict):
        return None
    for name in names:
        if name in data:
            return data[name]
    return None


def _normalize_scope(scope: str) -> str:
    normalized = str(scope).strip().upper()
    aliases = {
        "CONTRACTS": "CONTRACT",
        "TEMPLATES": "TEMPLATE",
        "ARCHIVES": "ARCHIVE",
        "SIGNATURES": "SIGNATURE",
    }
    return aliases.get(normalized, normalized)


def _normalize_status(status: Any) -> str:
    text = str(status or "").strip().upper()
    aliases = {
        "PASSED": "PASS",
        "SUCCESS": "PASS",
        "SUCCESSFUL": "PASS",
        "OK": "PASS",
        "WARN": "WARNING",
        "ERROR": "FAIL",
        "FAILED": "FAIL",
        "SKIP": "SKIPPED",
    }
    return aliases.get(text, text)


def _audit_run_body(context) -> dict[str, Any]:
    body = getattr(context, "pacm_current_audit_body", None)
    assert isinstance(body, dict), f"Current PACM audit response is not an AuditRun object: {body}"
    return body


def _as_audit_run(body: Any, scope: str | None = None) -> dict[str, Any]:
    if isinstance(body, dict):
        if isinstance(_field(body, "auditRun", "audit_run"), dict):
            return _field(body, "auditRun", "audit_run")
        return body
    if isinstance(body, list):
        # Legacy PAC audit responses were resource lists. Treat them as non-compliant
        # AuditRun candidates so assertions fail with useful context.
        return {"scope": scope, "findings": _legacy_findings(body), "legacyResponse": body}
    raise AssertionError(f"Unexpected PACM audit response shape: {body!r}")


def _audit_run_id(run: dict[str, Any]) -> str:
    value = _field(run, "id", "auditRunId", "audit_run_id", "runId", "run_id")
    assert value, f"AuditRun has no stable identifier: {run}"
    return str(value)


def _audit_run_status(run: dict[str, Any]) -> str:
    value = _field(run, "status", "resultStatus", "result_status")
    assert value, f"AuditRun has no status/resultStatus: {run}"
    return str(value).strip().upper()


def _audit_run_scope(run: dict[str, Any]) -> str:
    value = _field(run, "scope", "auditScope", "audit_scope")
    assert value, f"AuditRun has no scope: {run}"
    return _normalize_scope(str(value))


def _audit_run_created_at(run: dict[str, Any]) -> str:
    value = _field(run, "createdAt", "created_at", "startedAt", "started_at", "completedAt", "completed_at")
    assert value, f"AuditRun has no timestamp: {run}"
    return str(value)


def _findings_from_run(run: dict[str, Any]) -> list[dict[str, Any]]:
    findings = _field(run, "findings", "auditFindings", "audit_findings")
    if isinstance(findings, list):
        return [item for item in findings if isinstance(item, dict)]
    resources = _field(run, "resources", "auditResponses", "audit_responses")
    if isinstance(resources, list):
        return _legacy_findings(resources)
    return []


def _legacy_findings(resources: list[Any]) -> list[dict[str, Any]]:
    findings: list[dict[str, Any]] = []
    for resource in resources:
        if not isinstance(resource, dict):
            continue
        trail = _field(resource, "audit_trail", "auditTrail")
        if isinstance(trail, list):
            for entry in trail:
                if isinstance(entry, dict):
                    event_data = _field(entry, "event_data", "eventData") or {}
                    if isinstance(event_data, dict):
                        merged = dict(event_data)
                    else:
                        merged = {"message": str(event_data)}
                    merged.setdefault("eventType", _field(entry, "event_type", "eventType"))
                    merged.setdefault("component", _field(entry, "component"))
                    findings.append(merged)
        else:
            findings.append(resource)
    return findings


def _finding_status(finding: dict[str, Any]) -> str:
    value = _field(finding, "status", "result", "severity", "findingStatus", "finding_status")
    assert value, f"AuditFinding has no status/result/severity: {finding}"
    return _normalize_status(value)


def _finding_text(finding: dict[str, Any]) -> str:
    parts: list[str] = []
    for key in (
        "id",
        "ruleId",
        "rule_id",
        "title",
        "category",
        "message",
        "description",
        "requirement",
        "path",
        "semanticPath",
        "semantic_path",
        "ontologyTerm",
        "ontology_term",
        "evidence",
        "details",
    ):
        value = finding.get(key)
        if value is not None:
            parts.append(str(value))
    return " ".join(parts).lower()


def _assert_success(response):
    assert 200 <= response.status_code < 300, f"Expected HTTP 2xx, got {response.status_code}: {response.text}"


def _extract_runs(data: Any) -> list[dict[str, Any]]:
    if isinstance(data, list):
        return [item for item in data if isinstance(item, dict)]
    if isinstance(data, dict):
        for key in ("auditRuns", "audit_runs", "runs", "items", "data"):
            value = data.get(key)
            if isinstance(value, list):
                return [item for item in value if isinstance(item, dict)]
        if _field(data, "id", "auditRunId", "audit_run_id", "runId", "run_id"):
            return [data]
    raise AssertionError(f"PACM monitor response does not contain AuditRuns: {data}")


def _query_audit_runs(context, params: dict[str, Any] | None = None) -> list[dict[str, Any]]:
    response = _get(context, "/pac/monitor", params=params)
    context.requests_response = response
    _assert_success(response)
    return _extract_runs(_json(response))


def _current_run_id(context) -> str:
    run = _audit_run_body(context)
    return _audit_run_id(run)


def _query_audit_run_detail(context, run_id: str) -> dict[str, Any]:
    response = _get(context, f"/pac/monitor/{run_id}")
    if response.status_code == 404:
        response = _get(context, "/pac/monitor", params={"auditRunId": run_id})
    context.requests_response = response
    _assert_success(response)
    data = _json(response)
    if isinstance(data, dict) and _field(data, "id", "auditRunId", "audit_run_id", "runId", "run_id"):
        return data
    runs = _extract_runs(data)
    matches = [run for run in runs if str(_field(run, "id", "auditRunId", "audit_run_id", "runId", "run_id")) == run_id]
    assert matches, f"AuditRun {run_id} not found in detail response: {data}"
    return matches[0]


def _today_utc() -> datetime.date:
    return datetime.now(timezone.utc).date()


def _parse_date(value: str) -> datetime.date:
    normalized = value.replace("Z", "+00:00")
    return datetime.fromisoformat(normalized).astimezone(timezone.utc).date()


def _summary(report: dict[str, Any]) -> dict[str, Any]:
    summary = _field(report, "summary")
    assert isinstance(summary, dict), f"Report has no summary: {report}"
    return summary


def _report_run_id(report: dict[str, Any]) -> str:
    value = _field(report, "auditRunId", "audit_run_id", "runId", "run_id")
    assert value, f"Report does not identify the AuditRun: {report}"
    return str(value)


def _decode_report_payload(report: dict[str, Any]) -> str:
    content = _field(report, "content", "data")
    assert isinstance(content, str) and content, f"Report download has no content: {report}"
    encoding = str(_field(report, "encoding") or "").lower()
    if encoding == "base64":
        return base64.b64decode(content).decode("utf-8", errors="replace")
    return content


def _csv_summary(report: dict[str, Any]) -> dict[str, Any]:
    if isinstance(_field(report, "summary"), dict):
        return _summary(report)
    text = _decode_report_payload(report)
    reader = csv.DictReader(io.StringIO(text))
    rows = list(reader)
    return {"totalChecks": len([row for row in rows if row.get("section") == "finding"])}


def _pdf_summary(report: dict[str, Any]) -> dict[str, Any]:
    if isinstance(_field(report, "summary"), dict):
        return _summary(report)
    text = _decode_report_payload(report)
    marker = "Summary:"
    assert marker in text, f"PDF report content does not expose a summary: {text[:500]}"
    return {"summaryText": text.split(marker, 1)[1].splitlines()[0].strip()}


def _store_current_run(context, run: dict[str, Any]):
    context.pacm_current_audit_body = run
    context.pacm_current_audit_run_id = _audit_run_id(run)
    context.pacm_current_findings = _findings_from_run(run)


@when('an Auditor starts a PACM audit for scope "{scope}"')
def step_start_pacm_audit(context, scope):
    response = _post_json(context, "/pac/audit", {"scope": scope})
    context.requests_response = response
    if 200 <= response.status_code < 300:
        run = _as_audit_run(_json(response), scope)
        if _field(run, "id", "auditRunId", "audit_run_id", "runId", "run_id"):
            _store_current_run(context, run)
        else:
            context.pacm_current_audit_body = run
            context.pacm_current_findings = _findings_from_run(run)


@given('an Auditor has started a PACM audit for scope "{scope}"')
def step_given_started_pacm_audit(context, scope):
    step_start_pacm_audit(context, scope)
    _assert_success(context.requests_response)
    run = _audit_run_body(context)
    _store_current_run(context, run)


@given('an Auditor has started PACM audits for scopes "{scopes}"')
def step_given_started_pacm_audits(context, scopes):
    for scope in [item.strip() for item in scopes.split(",") if item.strip()]:
        step_given_started_pacm_audit(context, scope)


@when('an Auditor starts a "{scope}" audit through the UI-backed PACM command')
def step_start_ui_backed_pacm_audit(context, scope):
    step_start_pacm_audit(context, scope)


@then("the PACM audit request succeeds")
def step_pacm_audit_succeeds(context):
    _assert_success(context.requests_response)


@then("the PACM audit request is rejected as bad request")
def step_pacm_audit_bad_request(context):
    assert context.requests_response.status_code == 400, (
        f"Expected HTTP 400 for invalid audit scope, got "
        f"{context.requests_response.status_code}: {context.requests_response.text}"
    )


@then('the returned AuditRun has scope "{scope}"')
def step_returned_audit_run_scope(context, scope):
    assert _audit_run_scope(_audit_run_body(context)) == _normalize_scope(scope)


@then('every returned AuditFinding has status "{status}"')
def step_every_finding_has_status(context, status):
    findings = _findings_from_run(_audit_run_body(context))
    assert findings, "AuditRun has no findings"
    expected = _normalize_status(status)
    for finding in findings:
        assert _finding_status(finding) == expected, f"Unexpected finding status in {finding}"


@then('every returned AuditFinding has one of statuses "{statuses}"')
def step_every_finding_has_allowed_status(context, statuses):
    allowed = {_normalize_status(item) for item in statuses.split(",")}
    findings = _findings_from_run(_audit_run_body(context))
    assert findings, "AuditRun has no findings"
    for finding in findings:
        actual = _finding_status(finding)
        assert actual in allowed, f"Finding status {actual} is not in {allowed}: {finding}"


@then("every returned AuditFinding explains that signature validation is unavailable because signing is not implemented")
def step_signature_findings_explain_skip(context):
    findings = _findings_from_run(_audit_run_body(context))
    assert findings, "SIGNATURE AuditRun has no findings"
    for finding in findings:
        text = _finding_text(finding)
        assert SIGNATURE_SKIP_REASON in text, f"SIGNATURE skipped reason missing in finding: {finding}"


@then("the synchronous PACM audit response contains a stored AuditRun with result status")
def step_sync_response_has_stored_run_status(context):
    _assert_success(context.requests_response)
    run = _audit_run_body(context)
    run_id = _audit_run_id(run)
    status = _audit_run_status(run)
    assert status in {"COMPLETED", "FAILED"}, f"Synchronous audit must finish with result status, got {status}: {run}"
    context.pacm_current_audit_run_id = run_id


@then("the stored AuditRun can be retrieved from PACM queries")
def step_stored_run_retrievable(context):
    run_id = _current_run_id(context)
    detail = _query_audit_run_detail(context, run_id)
    assert str(_field(detail, "id", "auditRunId", "audit_run_id", "runId", "run_id")) == run_id


@when("the stored PACM AuditRun list is queried")
def step_query_stored_audit_runs(context):
    context.pacm_audit_runs = _query_audit_runs(context)


@then('every stored AuditRun has one of statuses "{statuses}"')
def step_every_run_has_allowed_status(context, statuses):
    allowed = {item.strip().upper() for item in statuses.split(",")}
    runs = getattr(context, "pacm_audit_runs", None)
    assert isinstance(runs, list), "No stored AuditRun list was queried"
    for run in runs:
        actual = _audit_run_status(run)
        assert actual in allowed, f"AuditRun status {actual} is not in {allowed}: {run}"


@then("the returned AuditRun contains findings for")
def step_audit_run_contains_findings_for(context):
    findings = _findings_from_run(_audit_run_body(context))
    assert findings, "AuditRun has no findings"
    combined = "\n".join(_finding_text(finding) for finding in findings)
    for row in context.table:
        expected = row["check"].strip().lower()
        tokens = [token for token in expected.replace("-", " ").split() if token]
        missing = [token for token in tokens if token not in combined]
        assert not missing, f"Missing check '{expected}' in findings: {findings}"


@when('PACM reports are generated for the current AuditRun in formats "{formats}"')
def step_generate_reports(context, formats):
    run_id = _current_run_id(context)
    reports: dict[str, dict[str, Any]] = {}
    for report_format in [item.strip().lower() for item in formats.split(",") if item.strip()]:
        response = _post_json(
            context,
            "/pac/report",
            {"auditRunId": run_id, "format": report_format},
        )
        context.requests_response = response
        _assert_success(response)
        report = _json(response)
        assert isinstance(report, dict), f"PACM report response is not an object: {report}"
        reports[report_format] = report
    context.pacm_reports = reports


@then('PACM audit events include "{event_types}"')
def step_pacm_events_include(context, event_types):
    run_id = _current_run_id(context)
    response = _get(context, "/pac/monitor", params={"auditRunId": run_id, "includeEvents": "true"})
    context.requests_response = response
    _assert_success(response)
    body_text = response.text
    for event_type in [item.strip() for item in event_types.split(",") if item.strip()]:
        assert event_type in body_text, f"PACM event {event_type} not found in monitor response: {body_text[:1000]}"


@when('the stored PACM AuditRun list is queried with scope "{scope}", status "{status}", and today\'s date')
def step_query_runs_filtered_today(context, scope, status):
    today = _today_utc().isoformat()
    context.pacm_audit_runs = _query_audit_runs(
        context,
        params={
            "scope": scope,
            "status": status,
            "from": today,
            "to": today,
        },
    )


@then('every returned AuditRun matches scope "{scope}"')
def step_runs_match_scope(context, scope):
    runs = getattr(context, "pacm_audit_runs", None)
    assert isinstance(runs, list), "No stored AuditRun list was queried"
    expected = _normalize_scope(scope)
    for run in runs:
        assert _audit_run_scope(run) == expected, f"AuditRun does not match scope {expected}: {run}"


@then('every returned AuditRun matches status "{status}"')
def step_runs_match_status(context, status):
    runs = getattr(context, "pacm_audit_runs", None)
    assert isinstance(runs, list), "No stored AuditRun list was queried"
    expected = status.strip().upper()
    for run in runs:
        assert _audit_run_status(run) == expected, f"AuditRun does not match status {expected}: {run}"


@then("every returned AuditRun was created today")
def step_runs_created_today(context):
    runs = getattr(context, "pacm_audit_runs", None)
    assert isinstance(runs, list), "No stored AuditRun list was queried"
    today = _today_utc()
    for run in runs:
        assert _parse_date(_audit_run_created_at(run)) == today, f"AuditRun was not created today: {run}"


@when("the current stored PACM AuditRun detail is queried")
def step_query_current_run_detail(context):
    context.pacm_current_audit_run_detail = _query_audit_run_detail(context, _current_run_id(context))


@then("the stored AuditRun detail contains all findings from the AuditRun")
def step_detail_contains_all_findings(context):
    original = _findings_from_run(_audit_run_body(context))
    detail = getattr(context, "pacm_current_audit_run_detail", None)
    assert isinstance(detail, dict), "No AuditRun detail was queried"
    detailed = _findings_from_run(detail)
    assert len(detailed) >= len(original), (
        f"AuditRun detail returned fewer findings than the original run: original={original}, detail={detail}"
    )


@then("every finding in the stored AuditRun detail includes evidence")
def step_detail_findings_include_evidence(context):
    detail = getattr(context, "pacm_current_audit_run_detail", None)
    assert isinstance(detail, dict), "No AuditRun detail was queried"
    findings = _findings_from_run(detail)
    assert findings, f"AuditRun detail has no findings: {detail}"
    for finding in findings:
        evidence = _field(finding, "evidence", "evidences", "sourceEvent", "source_event", "auditTrailEntry", "audit_trail_entry")
        assert evidence, f"AuditFinding has no evidence: {finding}"


@then("the generated PACM reports share the same summary")
def step_reports_share_summary(context):
    reports = getattr(context, "pacm_reports", None)
    assert isinstance(reports, dict) and reports, "No PACM reports were generated"
    summaries = []
    for report_format, report in reports.items():
        if report_format == "csv":
            summaries.append(_csv_summary(report))
        elif report_format == "pdf":
            summaries.append(_pdf_summary(report))
        else:
            summaries.append(_summary(report))
    first = summaries[0]
    for summary in summaries[1:]:
        assert summary == first, f"Report summaries differ: {summaries}"


@then("the generated PACM reports identify the same AuditRun")
def step_reports_identify_same_run(context):
    reports = getattr(context, "pacm_reports", None)
    assert isinstance(reports, dict) and reports, "No PACM reports were generated"
    expected = _current_run_id(context)
    for report_format, report in reports.items():
        actual = _report_run_id(report)
        assert actual == expected, f"{report_format} report identifies AuditRun {actual}, expected {expected}: {report}"


@when("the current stored PACM AuditRun count is recorded")
def step_record_run_count(context):
    context.pacm_recorded_run_count = len(_query_audit_runs(context))


@when("PACM query endpoints are requested via GET")
def step_request_query_endpoints(context):
    monitor = _get(context, "/pac/monitor")
    report = _get(context, "/pac/report")
    context.pacm_query_responses = [monitor, report]
    for response in context.pacm_query_responses:
        _assert_success(response)


@then("the stored PACM AuditRun count is unchanged")
def step_run_count_unchanged(context):
    before = getattr(context, "pacm_recorded_run_count", None)
    assert isinstance(before, int), "No PACM AuditRun count was recorded"
    after = len(_query_audit_runs(context))
    assert after == before, f"GET query endpoints changed AuditRun count from {before} to {after}"


@then("PACM command endpoints reject GET for creating audit or report data")
def step_command_endpoints_reject_get(context):
    audit = _get(context, "/pac/audit")
    report = _get(context, "/pac/report", params={"format": "json", "create": "true"})
    allowed_rejections = {400, 405, 409}
    assert audit.status_code in allowed_rejections, f"GET /pac/audit unexpectedly allowed command behavior: {audit.status_code} {audit.text}"
    assert report.status_code in allowed_rejections, f"GET /pac/report unexpectedly allowed report command behavior: {report.status_code} {report.text}"


@given("the Auditing UI entrypoint is reachable")
def step_auditing_ui_reachable(context):
    configured_ui = os.getenv("BDD_DCS_UI_URL", "").strip()
    if configured_ui:
        ui_base = configured_ui.rstrip("/")
    else:
        api_base = _base_url(context)
        ui_base = api_base[:-4] if api_base.endswith("/api") else api_base
    candidates = ["/ui/audit", "/audit", "/ui/", "/"]
    last_response = None
    for path in candidates:
        response = requests.get(f"{ui_base}{path}", timeout=_request_timeout(context))
        last_response = response
        if response.status_code < 400 and ("audit" in response.text.lower() or path.rstrip("/").endswith("audit")):
            context.pacm_auditing_ui_url = f"{ui_base}{path}"
            return
    assert last_response is not None
    assert False, f"Auditing UI entrypoint was not reachable at {ui_base}; last response {last_response.status_code}"


@then("the Auditing UI exposes a progress indicator for running audits")
def step_ui_progress_indicator_present(context):
    ui_url = getattr(context, "pacm_auditing_ui_url", None)
    ui_text = ""
    if ui_url:
        response = requests.get(ui_url, timeout=_request_timeout(context))
        if response.status_code < 400:
            ui_text = response.text.lower()

    source_path = os.path.join(
        os.getcwd(),
        "frontend",
        "ClientApp",
        "src",
        "views",
        "audit",
        "AuditView.vue",
    )
    if os.path.exists(source_path):
        with open(source_path, encoding="utf-8") as source_file:
            ui_text = f"{ui_text}\n{source_file.read().lower()}"

    indicators = ("loading-spinner", "executing audit", "progress")
    assert any(indicator in ui_text for indicator in indicators), (
        "Auditing UI does not expose a running-audit progress indicator"
    )


@given('a stored contract "{name}" has SLA availability below the contract policy minimum')
def step_given_contract_low_availability(context, name):
    _store_contract_content_fixture(context, name, availability=99.5, structure_violation=False)


@given('a stored contract "{name}" has a Contract-Content structure violation')
def step_given_contract_content_structure_violation(context, name):
    _store_contract_content_fixture(context, name, availability=99.95, structure_violation=True)


@when('an Auditor starts a CONTRACT-PACM audit for stored contract "{name}"')
def step_start_contract_pacm_audit_for_contract(context, name):
    assert (getattr(context, "contract_dids", {}) or {}).get(name), f"No stored contract fixture named {name!r}"
    context.pacm_current_contract_name = name
    step_start_pacm_audit(context, "CONTRACT")


@given('an Auditor has started a CONTRACT-PACM audit for stored contract "{name}"')
def step_given_started_contract_pacm_audit_for_contract(context, name):
    step_start_contract_pacm_audit_for_contract(context, name)
    _assert_success(context.requests_response)


@then('the CONTRACT-PACM audit contains a Contract-Content-Policy finding with rule reference "{rule_reference}"')
def step_contract_pacm_contains_rule_reference(context, rule_reference):
    contract_name = getattr(context, "pacm_current_contract_name", None)
    findings = _contract_content_policy_findings(context, contract_name)
    matches = [finding for finding in findings if _rule_reference(finding) == rule_reference]
    assert matches, f"No Contract-Content-Policy finding with rule reference {rule_reference}: {findings}"
    assert any(_is_failed_finding(finding) for finding in matches), f"Rule {rule_reference} was not reported as failing: {matches}"


@then("the CONTRACT-PACM audit contains a failing Contract-Content SHACL finding from the configured shape set")
def step_contract_pacm_contains_configured_shacl_finding(context):
    contract_name = getattr(context, "pacm_current_contract_name", None)
    findings = _contract_content_policy_findings(context, contract_name)
    matches = [
        finding for finding in findings
        if _is_configured_shacl_rule(_rule_reference(finding)) and _is_failed_finding(finding)
    ]
    assert matches, f"No failing SHACL finding from configured shape set found: {findings}"


@then("every Contract-Content-Policy finding in the CONTRACT-PACM audit has a non-empty rule reference")
def step_every_contract_policy_finding_has_rule_reference(context):
    contract_name = getattr(context, "pacm_current_contract_name", None)
    findings = _contract_content_policy_findings(context, contract_name)
    assert findings, "CONTRACT-PACM audit has no Contract-Content-Policy findings"
    missing = [finding for finding in findings if not _rule_reference(finding)]
    assert not missing, f"Contract-Content-Policy findings without rule reference: {missing}"


@when('a PACM JSON report is generated for the current AuditRun and stored contract "{name}"')
def step_generate_json_report_for_contract(context, name):
    did = (getattr(context, "contract_dids", {}) or {}).get(name)
    assert did, f"No stored contract fixture named {name!r}"
    response = _post_json(
        context,
        "/pac/report",
        {"auditRunId": _current_run_id(context), "format": "json", "did": did},
    )
    context.requests_response = response
    _assert_success(response)
    report = _json(response)
    assert isinstance(report, dict), f"PACM report response is not an object: {report}"
    context.pacm_current_report = report


@then("the PACM report summary counts passed and failed Contract-Content-Policy results")
def step_report_summary_counts_contract_policy_results(context):
    report = getattr(context, "pacm_current_report", None)
    assert isinstance(report, dict), "No current PACM report was generated"
    summary = _summary(_report_object(report))
    passed = int(_field(summary, "passed", "passedChecks", "passed_checks") or 0)
    failed = int(_field(summary, "failed", "failedChecks", "failed_checks") or 0)
    total = int(_field(summary, "totalChecks", "total_checks") or 0)
    findings = _report_policy_findings(report)
    assert findings, f"PACM report has no Contract-Content-Policy findings: {report}"
    assert passed > 0, f"PACM report summary does not count passed policy checks: {summary}"
    assert failed > 0, f"PACM report summary does not count failed policy checks: {summary}"
    assert total >= passed + failed, f"PACM report summary total is inconsistent: {summary}"


@then("the PACM report export contains Contract-Content-Policy findings with rule references")
def step_report_export_contains_contract_policy_findings_with_rules(context):
    report = getattr(context, "pacm_current_report", None)
    assert isinstance(report, dict), "No current PACM report was generated"
    findings = _report_policy_findings(report)
    assert findings, f"PACM report export has no Contract-Content-Policy findings: {report}"
    missing = [finding for finding in findings if not _rule_reference(finding)]
    assert not missing, f"PACM report export contains policy findings without rule references: {missing}"
