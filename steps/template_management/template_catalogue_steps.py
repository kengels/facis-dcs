"""BDD steps for the template catalogue integration endpoints (DCS-IR-SI-01,
UC-02): POST /template/publish (backend/design/template_repository.go) and
GET /catalogue/template/retrieve, /catalogue/template/retrieve/{did},
/catalogue/template/search (backend/design/template_catalogue_integration.go)
- the XFSC Federated Catalogue integration, deployed in-cluster for the BDD
harness (deployment/helm/charts/federated-catalogue), already exercised
indirectly by the passing "register" scenarios in template_workflow.feature.
"""

from behave import given, then, when

from steps.support.api_client import (
    catalogue_template_retrieve_by_id_url,
    catalogue_template_retrieve_url,
    catalogue_template_search_url,
    post_json,
    template_publish_url,
)
from steps.support.services.template_service import TemplateService


@when('I publish template "{name}"')
def step_when_publish_template(context, name):
    t = TemplateService.named(context, name)
    context.requests_response = post_json(
        context, template_publish_url(context), {"did": t["did"], "updated_at": t["updated_at"]}
    )
    if context.requests_response.status_code == 200:
        updated_at = TemplateService.fetch_template(context, t["did"]).get("updated_at")
        TemplateService.store_named(context, name, t["did"], updated_at)


@given('I publish template "{name}"')
def step_given_publish_template(context, name):
    # Given-position variant (used as setup by the retrieve/search scenarios):
    # unlike the When form, a failure here is a broken precondition, so assert
    # the publish succeeded instead of leaving that to a later Then.
    step_when_publish_template(context, name)
    assert context.requests_response.status_code == 200, (
        f"Publishing template '{name}' as a scenario precondition failed: "
        f"{context.requests_response.status_code} {context.requests_response.text}"
    )


@when('I attempt to publish template "{name}" with my current role')
def step_when_attempt_publish_template(context, name):
    t = TemplateService.named(context, name)
    headers = getattr(context, "headers", {})
    context.requests_response = post_json(
        context, template_publish_url(context), {"did": t["did"], "updated_at": t["updated_at"]}, headers=headers
    )


@when("I retrieve the template catalogue")
def step_when_retrieve_catalogue(context):
    import requests as _requests  # noqa: PLC0415

    context.requests_response = _requests.get(
        catalogue_template_retrieve_url(context),
        params={"offset": 0, "limit": 100},
        headers=getattr(context, "headers", {}),
        timeout=context.http_timeout_seconds,
    )


@when('I search the template catalogue by name "{name}"')
def step_when_search_catalogue(context, name):
    import requests as _requests  # noqa: PLC0415

    context.requests_response = _requests.get(
        catalogue_template_search_url(context),
        params={"name": name, "offset": 0, "limit": 100},
        headers=getattr(context, "headers", {}),
        timeout=context.http_timeout_seconds,
    )


@when('I retrieve the catalogue detail for template "{name}"')
def step_when_retrieve_catalogue_detail(context, name):
    import requests as _requests  # noqa: PLC0415

    versions = getattr(context, "catalogue_detail_versions", None)
    if versions is None:
        versions = {}
        context.catalogue_detail_versions = versions
    if name not in versions:
        template = TemplateService.named(context, name)
        matching_item = next(
            (
                item
                for item in _catalogue_items(context)
                if isinstance(item, dict) and item.get("did") == template["did"]
            ),
            None,
        )
        assert matching_item is not None, (
            f"Catalogue list/search result has no entry for detail retrieval of {name!r}"
        )
        version = matching_item.get("version")
        assert isinstance(version, int), (
            f"Catalogue entry for {name!r} has no integer version: {matching_item!r}"
        )
        versions[name] = version

    template = TemplateService.named(context, name)
    context.requests_response = _requests.get(
        catalogue_template_retrieve_by_id_url(context, template["did"]),
        params={"version": versions[name]},
        headers=getattr(context, "headers", {}),
        timeout=context.http_timeout_seconds,
    )


def _catalogue_items(context):
    body = context.requests_response.json()
    if isinstance(body, dict):
        items = body.get("items")
        assert isinstance(items, list), f"Expected catalogue response to carry an 'items' list, got: {body}"
        return items
    assert isinstance(body, list), f"Expected a catalogue items list, got: {body}"
    return body


def _poll_catalogue_for_template(context, name, refetch):
    """The Federated Catalogue ingests published self-descriptions
    asynchronously (verification + Neo4j claims-graph load), so a search or
    retrieval issued right after /template/publish can legitimately miss the
    template for a few seconds — poll by re-issuing the same request."""
    import time  # noqa: PLC0415

    t = TemplateService.named(context, name)
    dids = []
    deadline = time.monotonic() + 60
    while time.monotonic() < deadline:
        items = _catalogue_items(context)
        dids = [i.get("did") for i in items if isinstance(i, dict)]
        if t["did"] in dids:
            return next(
                item
                for item in items
                if isinstance(item, dict) and item.get("did") == t["did"]
            )
        time.sleep(2)
        refetch(context)
        assert context.requests_response.status_code == 200, (
            f"catalogue re-query failed: {context.requests_response.status_code} "
            f"{context.requests_response.text}"
        )
    raise AssertionError(
        f"Expected the template catalogue to include template '{name}' (did={t['did']}) "
        f"within 60s of publish, got dids: {dids}"
    )


@then('the catalogue result includes template "{name}"')
def step_then_catalogue_includes(context, name):
    _poll_catalogue_for_template(context, name, step_when_retrieve_catalogue)


@then('the catalogue search result includes template "{name}"')
def step_then_catalogue_search_includes(context, name):
    _poll_catalogue_for_template(
        context, name, lambda ctx: step_when_search_catalogue(ctx, name)
    )


def _assert_catalogue_component(item, name):
    actual_type = str(item.get("template_type", "")).upper()
    assert actual_type == TemplateService.COMPONENT_TEMPLATE_TYPE, (
        f"Expected catalogue projection for {name!r} to preserve "
        f"template_type={TemplateService.COMPONENT_TEMPLATE_TYPE!r}, got {item!r}"
    )


@then('the catalogue result includes component template "{name}"')
def step_then_catalogue_includes_component(context, name):
    item = _poll_catalogue_for_template(context, name, step_when_retrieve_catalogue)
    _assert_catalogue_component(item, name)


@then('the catalogue search result includes component template "{name}"')
def step_then_catalogue_search_includes_component(context, name):
    item = _poll_catalogue_for_template(
        context,
        name,
        lambda ctx: step_when_search_catalogue(ctx, name),
    )
    _assert_catalogue_component(item, name)


@then('the catalogue detail roundtrip returns component template "{name}"')
def step_then_catalogue_detail_returns_component(context, name):
    import time  # noqa: PLC0415

    deadline = time.monotonic() + 60
    while context.requests_response.status_code != 200 and time.monotonic() < deadline:
        time.sleep(2)
        step_when_retrieve_catalogue_detail(context, name)

    assert context.requests_response.status_code == 200, (
        f"Catalogue detail retrieval for {name!r} failed: "
        f"{context.requests_response.status_code} {context.requests_response.text}"
    )
    item = context.requests_response.json()
    assert isinstance(item, dict), f"Expected catalogue detail object, got {item!r}"
    expected_did = TemplateService.named(context, name)["did"]
    assert item.get("did") == expected_did, (
        f"Catalogue detail DID mismatch for {name!r}: expected {expected_did!r}, got {item!r}"
    )
    _assert_catalogue_component(item, name)
