"""Behave environment hooks for DCS BDD tests."""

import os
import re
import sys
from pathlib import Path
import psycopg2


SKIP_TAGS = {"skip", "skipped"}
UI_TAG = "ui"


def _has_tag(node, wanted):
	tags = []
	tags.extend(getattr(node, "effective_tags", ()) or ())
	tags.extend(getattr(node, "tags", ()) or ())
	feature = getattr(node, "feature", None)
	if feature is not None:
		tags.extend(getattr(feature, "tags", ()) or ())
	return any(_normalize_tag(tag) == wanted for tag in tags)


def _safe_artifact_name(name):
	value = re.sub(r"[^a-zA-Z0-9._-]+", "-", str(name)).strip("-.")
	return value[:120] or "scenario"


def _ensure_browser(context):
	# Behave removes attributes written in before_scenario when it pops the
	# scenario context layer. Keep the run-wide Playwright driver in private
	# attributes, which bypass that scoped stack, so a failed outline example
	# cannot orphan a live sync driver and start a second one inside its loop.
	if getattr(context, "_ui_browser", None) is not None:
		return
	try:
		from playwright.sync_api import sync_playwright
	except ImportError as exc:
		raise RuntimeError(
			"@ui scenarios require the pinned browser runner; run "
			"'make -C tests/bdd run_bdd_ui_kind_once'"
		) from exc
	context._ui_playwright = sync_playwright().start()
	context._ui_browser = context._ui_playwright.chromium.launch(
		headless=True,
	)


def _ui_scenario_failed(scenario):
	status = getattr(scenario, "status", None)
	return getattr(status, "name", str(status)).lower() == "failed"


def _normalize_tag(tag):
	value = str(tag).strip().lower()
	if value.startswith("@"):
		value = value[1:]
	return value


def _has_skip_tag(tags):
	return any(_normalize_tag(tag) in SKIP_TAGS for tag in (tags or ()))


def _iter_feature_scenarios(feature):
	# Includes scenarios generated from outlines when supported by Behave.
	if hasattr(feature, "walk_scenarios"):
		yield from feature.walk_scenarios()
	else:
		yield from feature.scenarios


def before_feature(context, feature):
	if not _has_skip_tag(getattr(feature, "tags", ())):
		return
	for scenario in _iter_feature_scenarios(feature):
		scenario.skip('Skipped by feature tag "@skip"')


def _scenario_has_skip_tag(scenario):
	tags = []
	tags.extend(getattr(scenario, "effective_tags", ()) or ())
	tags.extend(getattr(scenario, "tags", ()) or ())
	feature = getattr(scenario, "feature", None)
	if feature is not None:
		tags.extend(getattr(feature, "tags", ()) or ())
	return _has_skip_tag(tags)


def cleanup_database(context):
	cursor = context.db.cursor()

	try:
		_cleanup_database(cursor)
	except Exception:
		# Leave the shared connection usable for the rest of the suite; the
		# scenario itself still fails with the original error.
		context.db.rollback()
		cursor.close()
		raise

	context.db.commit()
	cursor.close()


def _cleanup_database(cursor):
	cursor.execute("DELETE FROM access_attempts")
	cursor.execute("DELETE FROM ip_lockouts")
	cursor.execute("DELETE FROM pac_incidents")

	cursor.execute("DELETE FROM contract_negotiation_task")
	cursor.execute("DELETE FROM contract_approval_task")
	cursor.execute("DELETE FROM contract_review_task")
	cursor.execute("DELETE FROM contract_negotiations")
	cursor.execute("TRUNCATE contract_archive_entry_events, contract_archive_entries")
	cursor.execute("DELETE FROM contract_kpis")
	cursor.execute("DELETE FROM contract_deployments")
	cursor.execute("DELETE FROM contract_signatures")
	cursor.execute("DELETE FROM signature_ceremonies")
	cursor.execute("DELETE FROM contracts")

	cursor.execute("DELETE FROM contract_templates_approval_task")
	cursor.execute("DELETE FROM contract_templates_review_task")
	cursor.execute("DELETE FROM template_provenance_credentials")
	cursor.execute("DELETE FROM contract_templates")



def before_scenario(context, scenario):
	if _scenario_has_skip_tag(scenario):
		scenario.skip('Skipped by scenario tag "@skip"')
		return

	if "clean_db" in scenario.tags:
		cleanup_database(context)

	if _has_tag(scenario, UI_TAG):
		_ensure_browser(context)
		artifact_dir = (
			Path(os.getenv("BDD_UI_REPORT_DIR", "tests/bdd/.reports/ui"))
			/ _safe_artifact_name(scenario.feature.name)
			/ _safe_artifact_name(scenario.name)
		)
		artifact_dir.mkdir(parents=True, exist_ok=True)
		context.ui_artifact_dir = artifact_dir
		context.browser_context = context._ui_browser.new_context(
			accept_downloads=True,
			record_video_dir=str(artifact_dir / "video"),
			viewport={"width": 1440, "height": 1000},
		)
		context.browser_context.tracing.start(screenshots=True, snapshots=True, sources=True)
		context.page = context.browser_context.new_page()


def after_scenario(context, scenario):
	if not _has_tag(scenario, UI_TAG) or getattr(context, "browser_context", None) is None:
		return

	failed = _ui_scenario_failed(scenario)
	artifact_dir = context.ui_artifact_dir
	try:
		if failed and getattr(context, "page", None) is not None:
			context.page.screenshot(path=str(artifact_dir / "failure.png"), full_page=True)
		if failed:
			context.browser_context.tracing.stop(path=str(artifact_dir / "trace.zip"))
		else:
			context.browser_context.tracing.stop()
	finally:
		context.browser_context.close()
		context.page = None
		context.browser_context = None


def before_all(context):

	steps_dir = Path(__file__).resolve().parent / "steps"
	steps_dir_str = str(steps_dir)
	if steps_dir_str not in sys.path:
		sys.path.insert(0, steps_dir_str)

	# Shared request defaults for step definitions.
	# Default to the Vite dev-server proxy (:5173), not the backend port
	# directly (:8991): Hydra has no fixed URLS_SELF_PUBLIC configured, so it
	# derives its OAuth redirect target dynamically from the Host header of
	# whichever caller reaches it first in the login chain. Requests that hit
	# the backend port directly leak that host into Hydra's redirect_to
	# response, which the backend then can't serve (404 on /oauth2/auth) —
	# the whole login flow only works end-to-end when everything consistently
	# goes through the same origin the dev stack's Hydra client is registered
	# against (localhost:5173, see deployment/helm/values.dev.yml).
	context.base_url = os.getenv("BDD_DCS_BASE_URL", "http://localhost:5173/api").rstrip("/")
	# 60s: component-wide audit reads (POST /pac/audit) walk every per-DID
	# hash chain over IPFS; mid-suite that legitimately exceeds 20s on slower
	# runners without being wrong.
	context.http_timeout_seconds = float(os.getenv("BDD_HTTP_TIMEOUT_SECONDS", "60"))
	context.aliases = {}
	context._ui_playwright = None
	context._ui_browser = None
	context.browser_context = None
	context.page = None

	try:
		context.db = psycopg2.connect(
			os.getenv("DATABASE_URL", "host=localhost port=30432 user=dcs password=dcs dbname=dcs sslmode=disable")
		)
		context.db.cursor().execute("SELECT 1")
		print("DB connection successful")
	except psycopg2.OperationalError as e:
		raise RuntimeError(f"Could not connect to database: {e}")


def after_all(context):
	if getattr(context, "_ui_browser", None) is not None:
		context._ui_browser.close()
	if getattr(context, "_ui_playwright", None) is not None:
		context._ui_playwright.stop()
	context.db.close()
