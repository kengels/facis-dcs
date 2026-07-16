"""Playwright assertion for browser-rendered lifecycle timestamps."""

from behave import then

from .ui_playwright_steps import _element


@then("the template lifecycle evidence contains entries without an invalid date")
def step_lifecycle_evidence_has_valid_dates(context):
    from playwright.sync_api import expect

    evidence = _element(context, "template-review-history")
    first_entry = evidence.locator("li").first
    expect(first_entry).to_be_visible(timeout=90_000)
    text = evidence.inner_text()
    assert "Invalid Date" not in text, (
        f"Expected browser-rendered lifecycle timestamps, found 'Invalid Date' in: {text!r}"
    )
