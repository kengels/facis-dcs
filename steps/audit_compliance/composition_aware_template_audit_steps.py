"""BDD fixtures and assertions for composition-aware template policy audits."""

from __future__ import annotations

import copy
import json

from behave import given, then, when

from steps.support.api_client import (
    pac_audit_url,
    post_json,
    template_approve_url,
    template_create_url,
    template_register_url,
)
from steps.support.services.auth_service import AuthService
from steps.support.services.template_service import TemplateService


KNOWN_DOMAIN_FIELD = "https://w3id.org/facis/dcs/taxonomy/v1#field-contract-jurisdiction"
COUNTRY_DOMAIN_FIELD = "https://w3id.org/facis/dcs/taxonomy/v1#field-company-location-country"
SIGNATURE_DOMAIN_FIELD = "https://w3id.org/facis/dcs/taxonomy/v1#field-signature-requiredLevel"
UNKNOWN_DOMAIN_FIELD = "https://example.invalid/ontology#unknown-field"


def _field(field_id: str, parameter_name: str, domain_field: str, *, complete: bool = True) -> dict:
    field = {
        "@id": field_id,
        "@type": "dcs:RequirementField",
    }
    if complete:
        field.update(
            {
                "dcs:parameterName": parameter_name,
                "dcs:domainField": {"@id": domain_field},
            }
        )
    return field


def _requirement(domain_field: str = KNOWN_DOMAIN_FIELD, *, complete: bool = True) -> dict:
    return {
        "@id": "urn:uuid:requirement-jurisdiction",
        "@type": "dcs:DataRequirement",
        "dcs:conditionId": "legal",
        "dcs:fields": [
            _field(
                "urn:uuid:field-jurisdiction",
                "jurisdiction",
                domain_field,
                complete=complete,
            )
        ],
    }


def _required_domain_requirement() -> dict:
    return {
        "@id": "urn:uuid:requirement-required-domains",
        "@type": "dcs:DataRequirement",
        "dcs:conditionId": "required-domains",
        "dcs:fields": [
            _field("urn:uuid:field-jurisdiction", "jurisdiction", KNOWN_DOMAIN_FIELD),
            _field("urn:uuid:field-country", "country", COUNTRY_DOMAIN_FIELD),
            _field("urn:uuid:field-signature-level", "signature-level", SIGNATURE_DOMAIN_FIELD),
        ],
    }


def _clause(*, bound: bool) -> dict:
    content: list[object] = ["This agreement is governed by "]
    if bound:
        content.append(
            {
                "@type": "dcs:Placeholder",
                "dcs:bindsTo": {"@id": "urn:uuid:field-jurisdiction"},
            }
        )
    return {
        "@id": "urn:uuid:clause-jurisdiction",
        "@type": "dcs:Clause",
        "dcs:content": {"@list": content},
    }


def _policy(field_id: str = "urn:uuid:field-jurisdiction") -> dict:
    return {
        "@id": "urn:uuid:policy-set",
        "@type": "odrl:Set",
        "uid": "urn:uuid:policy-set",
        "odrl:profile": {"@id": "https://w3id.org/facis/dcs/odrl-profile/v1"},
        "odrl:duty": [
            {
                "@id": "urn:uuid:policy-jurisdiction",
                "@type": "odrl:Duty",
                "odrl:action": {"@id": "odrl:use"},
                "odrl:assigner": {"@id": "urn:facis:party:assigner"},
                "odrl:assignee": {"@id": "urn:facis:party:assignee"},
                "odrl:target": {"@id": "urn:facis:asset:contract"},
                "odrl:constraint": {
                    "@type": "odrl:Constraint",
                    "odrl:leftOperand": {"@id": field_id},
                    "odrl:operator": {"@id": "odrl:eq"},
                    "odrl:rightOperand": {"@type": "xsd:string", "@value": "DEU"},
                },
            }
        ],
    }


def _document(
    title: str,
    *,
    contract_data: list[dict] | None = None,
    bound_clause: bool = False,
    policies: dict | list | None = None,
) -> dict:
    clause = _clause(bound=bound_clause)
    return {
        "@context": {
            "dcs": "https://w3id.org/facis/dcs/ontology/v1#",
            "odrl": "http://www.w3.org/ns/odrl/2/",
            "xsd": "http://www.w3.org/2001/XMLSchema#",
        },
        "@type": "dcs:ContractTemplate",
        "dcs:metadata": {
            "@type": "dcs:TemplateMetadata",
            "dcs:title": title,
            "dcs:templateType": "dcs:ContractTemplate",
        },
        "dcs:documentStructure": {
            "@type": "dcs:DocumentStructure",
            "dcs:blocks": {"@list": [clause]},
            "dcs:layout": [
                {
                    "@id": "urn:uuid:layout-root",
                    "@type": "dcs:LayoutNode",
                    "dcs:isRoot": True,
                    "dcs:children": {"@list": [{"@id": clause["@id"]}]},
                }
            ],
        },
        "dcs:contractData": contract_data or [],
        "dcs:policies": [] if policies is None else policies,
    }


def _create_template(context, name: str, template_type: str, template_data: dict) -> dict:
    creator_headers = AuthService.get_headers_for_roles(["Template Creator"])
    response = post_json(
        context,
        template_create_url(context),
        {
            "template_type": template_type,
            "name": name,
            "description": "Composition-aware audit BDD fixture",
            "template_data": template_data,
        },
        headers=creator_headers,
    )
    assert response.status_code == 200, response.text
    did = response.json().get("did")
    assert did, response.text
    body = TemplateService.fetch_template(context, did, headers=creator_headers)
    TemplateService.store_named(context, name, did, body.get("updated_at"))
    return body


def _make_reusable(context, name: str, component_data: dict) -> dict:
    component = _create_template(context, name, TemplateService.COMPONENT_TEMPLATE_TYPE, component_data)
    did = component["did"]
    updated_at = TemplateService.do_submit(context, did, component["updated_at"])
    updated_at = TemplateService.do_recommend_for_approval(context, did, updated_at)
    approval = post_json(
        context,
        template_approve_url(context),
        {"did": did, "updated_at": updated_at},
        headers=AuthService.get_headers_for_roles(["Template Approver"]),
    )
    assert approval.status_code == 200, approval.text
    approver_headers = AuthService.get_headers_for_roles(["Template Approver"])
    approved = TemplateService.fetch_template(context, did, headers=approver_headers)
    manager_headers = AuthService.get_headers_for_roles(["Template Manager"])
    registration = post_json(
        context,
        template_register_url(context),
        {"did": did, "updated_at": approved["updated_at"]},
        headers=manager_headers,
    )
    assert registration.status_code == 200, registration.text
    registered = TemplateService.fetch_template(context, did, headers=manager_headers)
    TemplateService.store_named(context, name, did, registered["updated_at"])
    return registered


def _create_composed_parent(
    context,
    name: str,
    component_data: dict,
    *,
    root_data: dict | None = None,
    snapshot_transform=None,
    register_parent: bool = False,
) -> tuple[dict, dict]:
    component_name = f"{name} Component"
    component = _make_reusable(context, component_name, component_data)
    snapshot_data = copy.deepcopy(component["template_data"])
    if snapshot_transform is not None:
        snapshot_transform(snapshot_data)
    root = copy.deepcopy(root_data or _document(name))
    root["dcs:metadata"]["dcs:subTemplates"] = [
        {
            "@id": component["did"],
            "dcs:version": component["version"],
            "dcs:name": component_name,
            "dcs:template": snapshot_data,
        }
    ]
    parent = _create_template(context, name, TemplateService.CONTRACT_TEMPLATE_TYPE, root)
    if register_parent:
        did = parent["did"]
        updated_at = TemplateService.do_submit(context, did, parent["updated_at"])
        updated_at = TemplateService.do_recommend_for_approval(context, did, updated_at)
        approval = post_json(
            context,
            template_approve_url(context),
            {"did": did, "updated_at": updated_at},
            headers=AuthService.get_headers_for_roles(["Template Approver"]),
        )
        assert approval.status_code == 200, approval.text
        approver_headers = AuthService.get_headers_for_roles(["Template Approver"])
        approved = TemplateService.fetch_template(context, did, headers=approver_headers)
        manager_headers = AuthService.get_headers_for_roles(["Template Manager"])
        registration = post_json(
            context,
            template_register_url(context),
            {"did": did, "updated_at": approved["updated_at"]},
            headers=manager_headers,
        )
        assert registration.status_code == 200, registration.text
        parent = TemplateService.fetch_template(context, did, headers=manager_headers)
        TemplateService.store_named(context, name, did, parent["updated_at"])
    context.composition_component_name = component_name
    return parent, component


def _valid_component(
    title: str,
    *,
    policies: bool = False,
    policy_field_id: str = "urn:uuid:field-jurisdiction",
    domain_field: str = KNOWN_DOMAIN_FIELD,
) -> dict:
    return _document(
        title,
        contract_data=[_requirement(domain_field)],
        bound_clause=True,
        policies=_policy(policy_field_id) if policies else [],
    )


@given('contract template "{name}" has no root contract data and embeds an immediate component with valid contract data')
def step_component_supplies_data(context, name):
    _create_composed_parent(context, name, _valid_component(f"{name} Component"))


@given('contract template "{name}" and its immediate component declare no contract data')
def step_composition_has_no_data(context, name):
    _create_composed_parent(context, name, _document(f"{name} Component"))


@given('contract template "{name}" has a malformed root data requirement and a valid immediate component requirement')
def step_malformed_root_requirement(context, name):
    _create_composed_parent(
        context,
        name,
        _valid_component(f"{name} Component"),
        root_data=_document(name, contract_data=[_requirement(complete=False)]),
    )


@given('contract template "{name}" has valid root data and a malformed immediate component requirement')
def step_malformed_component_requirement(context, name):
    def invalidate_requirement(snapshot: dict):
        field = snapshot["dcs:contractData"][0]["dcs:fields"][0]
        field.pop("dcs:parameterName", None)
        field.pop("dcs:domainField", None)

    _create_composed_parent(
        context,
        name,
        _valid_component(f"{name} Component"),
        root_data=_document(name, contract_data=[_requirement()], bound_clause=True),
        snapshot_transform=invalidate_requirement,
    )


@given('contract template "{name}" has no bound root clause and embeds an immediate component with a clause bound to its contract data')
def step_component_supplies_clause(context, name):
    _create_composed_parent(context, name, _valid_component(f"{name} Component"))


@given('contract template "{name}" and its immediate component have no clause bound to contract data')
def step_composition_has_no_bound_clause(context, name):
    _create_composed_parent(
        context,
        name,
        _document(f"{name} Component", contract_data=[_requirement()]),
        root_data=_document(name, contract_data=[_requirement()]),
    )


@given('contract template "{name}" has valid root content and an immediate component with an invalid policy operand and domain field')
def step_invalid_effective_component_content(context, name):
    def invalidate_content(snapshot: dict):
        snapshot["dcs:contractData"][0]["dcs:fields"][0]["dcs:domainField"] = {
            "@id": UNKNOWN_DOMAIN_FIELD
        }
        snapshot["dcs:policies"]["odrl:duty"][0]["odrl:constraint"]["odrl:leftOperand"] = {
            "@id": "urn:uuid:field-not-declared"
        }

    _create_composed_parent(
        context,
        name,
        _valid_component(f"{name} Component", policies=True),
        root_data=_document(
            name,
            contract_data=[_requirement()],
            bound_clause=True,
            policies=_policy(),
        ),
        snapshot_transform=invalidate_content,
    )


@given('contract template "{name}" has no root contract data and embeds a canonical policy with constrained jurisdiction, country, and signature fields')
def step_required_component_fields(context, name):
    component = _document(
        f"{name} Component",
        contract_data=[_required_domain_requirement()],
        bound_clause=True,
        policies=_policy("urn:uuid:field-country"),
    )
    _create_composed_parent(context, name, component)


@given('contract template "{name}" has a valid root structure and embeds a snapshot with an invalid component layout')
def step_invalid_component_layout(context, name):
    def invalidate_layout(snapshot: dict):
        snapshot["dcs:documentStructure"]["dcs:layout"] = []

    _create_composed_parent(
        context,
        name,
        _valid_component(f"{name} Component"),
        root_data=_document(name, contract_data=[_requirement()], bound_clause=True),
        snapshot_transform=invalidate_layout,
    )


@given('registered contract template "{name}" embeds a component snapshot with missing metadata and a draft state marker')
def step_component_metadata_lifecycle_ignored(context, name):
    def invalidate_non_content(snapshot: dict):
        snapshot.pop("dcs:metadata", None)
        snapshot["state"] = "DRAFT"

    _create_composed_parent(
        context,
        name,
        _valid_component(f"{name} Component"),
        root_data=_document(name, contract_data=[_requirement()], bound_clause=True),
        snapshot_transform=invalidate_non_content,
        register_parent=True,
    )


@given('standalone component template "{name}" has an incomplete contract data field')
def step_standalone_component_incomplete_data(context, name):
    data = _document(name, contract_data=[_requirement(complete=False)])
    _create_template(context, name, TemplateService.COMPONENT_TEMPLATE_TYPE, data)


@given('contract template "{name}" embeds a valid immediate component with invalid nested snapshot content')
def step_snapshot_boundary(context, name):
    component_data = _valid_component(f"{name} Component")
    nested_invalid = _valid_component("Nested Invalid Component", domain_field=UNKNOWN_DOMAIN_FIELD)

    def add_nested_snapshot(snapshot: dict):
        snapshot["dcs:metadata"]["dcs:subTemplates"] = [
            {
                "@id": "did:example:nested-component-not-resolved",
                "dcs:version": 1,
                "dcs:template": nested_invalid,
            }
        ]

    parent, _ = _create_composed_parent(
        context,
        name,
        component_data,
        snapshot_transform=add_nested_snapshot,
    )
    context.composition_parent_snapshot_before = copy.deepcopy(
        parent["template_data"]["dcs:metadata"]["dcs:subTemplates"][0]
    )


@given("the authoritative component content changes after the parent snapshot was persisted")
def step_authoritative_component_changes(context):
    component = TemplateService.named(context, context.composition_component_name)
    changed = _valid_component("Changed Authoritative Component", domain_field=UNKNOWN_DOMAIN_FIELD)
    cursor = context.db.cursor()
    try:
        cursor.execute(
            "UPDATE contract_templates SET template_data = %s::jsonb WHERE did = %s",
            (json.dumps(changed), component["did"]),
        )
        assert cursor.rowcount == 1, "Expected exactly one authoritative component to change"
        context.db.commit()
    finally:
        cursor.close()


@when('the Auditor audits policies of template "{name}"')
def step_auditor_audits_template(context, name):
    template = TemplateService.named(context, name)
    assert template, f"Unknown template fixture {name!r}"
    context.requests_response = post_json(
        context,
        pac_audit_url(context),
        {
            "scope": "templates",
            "did": template["did"],
            "justification": "composition-aware template policy BDD audit",
        },
        headers=AuthService.get_headers_for_roles(["Auditor"]),
    )
    assert context.requests_response.status_code == 200, context.requests_response.text


def _policy_findings(context) -> list[dict]:
    findings = []
    for group in context.requests_response.json():
        for entry in group.get("audit_trail") or []:
            data = entry.get("event_data")
            if isinstance(data, str):
                data = json.loads(data)
            if isinstance(data, dict) and data.get("ruleId"):
                findings.append(data)
    return findings


@then('template policy rule "{rule_id}" has no finding')
def step_rule_has_no_finding(context, rule_id):
    matches = [finding for finding in _policy_findings(context) if finding.get("ruleId") == rule_id]
    assert not matches, f"Unexpected {rule_id} findings: {matches!r}"


@then('template policy rule "{rule_id}" has an error finding')
def step_rule_has_error(context, rule_id):
    matches = [
        finding
        for finding in _policy_findings(context)
        if finding.get("ruleId") == rule_id and finding.get("severity") == "error"
    ]
    assert matches, f"Expected error finding {rule_id}; got {_policy_findings(context)!r}"


@then("template policy rules have no findings for")
def step_rules_have_no_findings(context):
    found = {finding.get("ruleId") for finding in _policy_findings(context)}
    expected_absent = {row["rule_id"] for row in context.table}
    assert found.isdisjoint(expected_absent), f"Unexpected findings: {found & expected_absent}"


@then("the persisted immediate component snapshot is unchanged")
def step_snapshot_unchanged(context):
    parent_names = [name for name in context.named_templates if name != context.composition_component_name]
    assert len(parent_names) == 1, f"Expected one parent template, got {parent_names!r}"
    parent = TemplateService.named(context, parent_names[0])
    persisted = TemplateService.fetch_template(
        context,
        parent["did"],
        headers=AuthService.get_headers_for_roles(["Template Creator"]),
    )
    snapshot = persisted["template_data"]["dcs:metadata"]["dcs:subTemplates"][0]
    assert snapshot == context.composition_parent_snapshot_before, (
        "Policy audit mutated or replaced the persisted component snapshot"
    )
