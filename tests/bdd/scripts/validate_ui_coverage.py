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
STEP_LINE = re.compile(r"^\s+(?:Given|When|Then|And|But)\b")


def fail(message: str) -> None:
    print(f"UI coverage gate: {message}", file=sys.stderr)
    raise SystemExit(1)


def require_passing_scenario(entry: dict[str, object], scenario: dict[str, object]) -> None:
    evidence = ROOT / str(scenario["evidence"])
    scenario_name = scenario.get("scenario")
    if not isinstance(scenario_name, str) or not scenario_name.strip():
        fail(f"{entry['id']} claims ui-covered without an exact evidence scenario")
    if not evidence.is_file():
        fail(f"{entry['id']} claims ui-covered without generated evidence at {evidence}")
    try:
        root = ET.parse(evidence).getroot()
    except (ET.ParseError, OSError) as exc:
        fail(f"{entry['id']} references unreadable JUnit evidence: {exc}")

    cases = [case for case in root.iter("testcase") if case.get("name") == scenario_name]
    if len(cases) != 1:
        fail(f"{entry['id']} evidence must contain exactly one scenario named {scenario_name!r}")
    case = cases[0]
    failed = any(case.find(result) is not None for result in ("error", "failure", "skipped"))
    if failed or case.get("status") != "passed":
        fail(f"{entry['id']} evidence scenario {scenario_name!r} did not pass")


def require_atomic_evidence(catalog: dict[str, object]) -> None:
    evidence_run = catalog.get("evidence_run")
    if not isinstance(evidence_run, dict):
        fail("catalog has no evidence_run metadata")

    junit_root_value = evidence_run.get("junit_root")
    if not isinstance(junit_root_value, str):
        fail("evidence_run has no junit_root")
    junit_root = ROOT / junit_root_value
    run_start = junit_root.parent / "run-start"
    if not run_start.is_file():
        fail(f"atomic evidence marker is missing: {run_start}")
    run_started_at = run_start.stat().st_mtime_ns

    feature_files = sorted(FEATURE_ROOT.glob("*.feature"))
    expected_files = {f"TESTS-{feature.stem}.xml" for feature in feature_files}
    actual_files = {path.relative_to(junit_root).as_posix() for path in junit_root.rglob("*.xml")}
    if actual_files != expected_files:
        missing = sorted(expected_files - actual_files)
        extra = sorted(actual_files - expected_files)
        fail(f"evidence is not one exact suite run; missing={missing}, extra={extra}")

    evidence_inputs = [
        CATALOG,
        STEPS,
        ROOT / "environment.py",
        ROOT / "tests/bdd/scripts/run_bdd_helm.sh",
        ROOT / "tests/bdd/scripts/run_playwright_container.sh",
        *feature_files,
    ]
    evidence_inputs.extend((ROOT / "frontend/ClientApp/src").rglob("*.ts"))
    evidence_inputs.extend((ROOT / "frontend/ClientApp/src").rglob("*.vue"))
    evidence_inputs.extend((ROOT / "backend/design").rglob("*.go"))
    evidence_inputs.extend((ROOT / "backend/internal").rglob("*.go"))
    evidence_inputs.extend((ROOT / "backend/migrations/sql").rglob("*.sql"))
    evidence_inputs.extend((ROOT / "testWallet").rglob("*.py"))
    newer_inputs = [path.relative_to(ROOT).as_posix() for path in evidence_inputs if path.stat().st_mtime_ns > run_started_at]
    if newer_inputs:
        fail(f"evidence predates browser-test inputs: {', '.join(newer_inputs)}")

    scenario_count = 0
    step_count = 0
    for relative_name in sorted(actual_files):
        evidence = junit_root / relative_name
        if evidence.stat().st_mtime_ns < run_started_at:
            fail(f"stale JUnit file predates the atomic run: {evidence}")
        try:
            suite = ET.parse(evidence).getroot()
        except (ET.ParseError, OSError) as exc:
            fail(f"unreadable JUnit evidence {evidence}: {exc}")
        if any(int(suite.get(field, "0")) != 0 for field in ("errors", "failures", "skipped")):
            fail(f"JUnit suite is not green: {evidence}")
        cases = list(suite.iter("testcase"))
        scenario_count += len(cases)
        for case in cases:
            if case.get("status") != "passed" or any(
                case.find(result) is not None for result in ("error", "failure", "skipped")
            ):
                fail(f"JUnit scenario is not green: {case.get('name')!r} in {evidence}")
            step_count += sum(
                bool(STEP_LINE.match(line))
                for output in case.findall("system-out")
                for line in (output.text or "").splitlines()
            )

    expected_scenarios = evidence_run.get("expected_scenarios")
    expected_steps = evidence_run.get("expected_steps")
    if scenario_count != expected_scenarios:
        fail(f"atomic evidence contains {scenario_count} scenarios, expected {expected_scenarios}")
    if step_count != expected_steps:
        fail(f"atomic evidence contains {step_count} executed steps, expected {expected_steps}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--require-evidence", action="store_true")
    args = parser.parse_args()
    catalog = json.loads(CATALOG.read_text(encoding="utf-8"))
    entries = catalog.get("requirements", [])
    scenarios = catalog.get("scenarios", {})
    ids = [entry.get("id") for entry in entries]
    if len(ids) != len(set(ids)):
        fail("catalog contains duplicate ids")
    if args.require_evidence:
        require_atomic_evidence(catalog)

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
        if disposition == "ui-covered":
            scenario_ref = entry.get("scenario_ref")
            scenario = scenarios.get(scenario_ref) if isinstance(scenario_ref, str) else None
            if not isinstance(scenario, dict):
                fail(f"{entry['id']} claims ui-covered without a valid scenario_ref")
            feature = scenario.get("feature")
            if not isinstance(feature, str) or not (ROOT / feature).is_file():
                fail(f"{entry['id']} references missing scenario feature {feature}")
            requirement_id = str(entry.get("requirement_id") or entry["id"])
            if f"@{requirement_id}" not in (ROOT / feature).read_text(encoding="utf-8"):
                fail(f"{entry['id']} is not tagged in its browser feature {feature}")
            if not scenario.get("evidence") or not scenario.get("scenario"):
                fail(f"{entry['id']} claims ui-covered without exact JUnit evidence")
            if args.require_evidence:
                require_passing_scenario(entry, scenario)

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
