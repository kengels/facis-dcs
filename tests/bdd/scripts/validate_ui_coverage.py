#!/usr/bin/env python3
"""Fail when UI traceability claims or browser selectors are not evidence-safe."""

from __future__ import annotations

import argparse
import json
import re
import sys
import xml.etree.ElementTree as ET
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
CATALOG = ROOT / "tests/bdd/ui_coverage.json"
FEATURE_ROOT = ROOT / "features/24_ui_traceability"
STEPS = ROOT / "steps/ui_traceability/ui_playwright_steps.py"
ALLOWED = {"ui-covered", "ui-gap", "not-ui-applicable", "decision-blocked"}


def fail(message: str) -> None:
    print(f"UI coverage gate: {message}", file=sys.stderr)
    raise SystemExit(1)


def require_passing_scenario(entry: dict[str, object]) -> None:
    evidence = ROOT / str(entry["evidence"])
    scenario = entry.get("scenario")
    if not isinstance(scenario, str) or not scenario.strip():
        fail(f"{entry['id']} claims ui-covered without an exact evidence scenario")
    if not evidence.is_file():
        fail(f"{entry['id']} claims ui-covered without generated evidence at {evidence}")
    try:
        root = ET.parse(evidence).getroot()
    except (ET.ParseError, OSError) as exc:
        fail(f"{entry['id']} references unreadable JUnit evidence: {exc}")

    cases = [case for case in root.iter("testcase") if case.get("name") == scenario]
    if len(cases) != 1:
        fail(f"{entry['id']} evidence must contain exactly one scenario named {scenario!r}")
    case = cases[0]
    failed = any(case.find(result) is not None for result in ("error", "failure", "skipped"))
    if failed or case.get("status") != "passed":
        fail(f"{entry['id']} evidence scenario {scenario!r} did not pass")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--require-evidence", action="store_true")
    args = parser.parse_args()
    catalog = json.loads(CATALOG.read_text(encoding="utf-8"))
    entries = catalog.get("requirements", [])
    ids = [entry.get("id") for entry in entries]
    if len(ids) != len(set(ids)):
        fail("catalog contains duplicate ids")

    feature_text = "\n".join(path.read_text(encoding="utf-8") for path in FEATURE_ROOT.glob("*.feature"))
    catalog_requirement_ids = {str(entry.get("requirement_id") or entry.get("id")) for entry in entries}
    tagged_requirement_ids = set(re.findall(r"@(DCS-(?:IR|FR|NFR)-[A-Z]+-\d+)", feature_text))
    missing_catalog_entries = sorted(tagged_requirement_ids - catalog_requirement_ids)
    if missing_catalog_entries:
        fail(f"UI scenarios reference requirements absent from catalog: {', '.join(missing_catalog_entries)}")

    for entry in entries:
        disposition = entry.get("disposition")
        if disposition not in ALLOWED:
            fail(f"{entry.get('id')} has invalid disposition {disposition!r}")
        feature = entry.get("feature")
        if feature and not (ROOT / feature).is_file():
            fail(f"{entry['id']} references missing feature {feature}")
        ac = entry.get("ac")
        if ac:
            scenario_tag = f"@REQ-ui-traceability-playwright-evidence-{ac}"
            owning_feature = (ROOT / str(feature)).read_text(encoding="utf-8")
            if owning_feature.count(scenario_tag) != 1:
                fail(f"{entry['id']} must map to exactly one browser scenario tag {ac} in {feature}")
        if disposition == "ui-covered":
            evidence = entry.get("evidence")
            if not evidence:
                fail(f"{entry['id']} claims ui-covered without a declared evidence artifact")
            scenario = entry.get("scenario")
            if not scenario:
                fail(f"{entry['id']} claims ui-covered without an exact evidence scenario")
            if args.require_evidence:
                require_passing_scenario(entry)

    for feature in FEATURE_ROOT.glob("*.feature"):
        text = feature.read_text(encoding="utf-8")
        if "@ui" not in text:
            fail(f"{feature.relative_to(ROOT)} has no @ui tag")
        if re.search(r"@skip(?:ped)?\b", text, re.IGNORECASE):
            fail(f"{feature.relative_to(ROOT)} introduces a skipped UI scenario")

    steps = STEPS.read_text(encoding="utf-8")
    forbidden = ["get_by_text(", "get_by_role(", ".nth(", "locator(\".", "locator('#"]
    for token in forbidden:
        if token in steps:
            fail(f"unstable Playwright selector token found: {token}")
    if 'f\'[data-test-id="{test_id}"]\'' not in steps:
        fail("Playwright steps do not centralize data-test-id selectors")

    print(f"UI coverage gate passed for {len(entries)} catalog entries")


if __name__ == "__main__":
    main()
