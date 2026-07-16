#!/usr/bin/env bash
set -euo pipefail

: "${PROJECT_ROOT:?PROJECT_ROOT is required}"
: "${FEATURES_PATH:?FEATURES_PATH is required}"
: "${STATUSLIST_SERVICE_URL:?STATUSLIST_SERVICE_URL is required}"
: "${BDD_UI_REPORT_DIR:?BDD_UI_REPORT_DIR is required}"
: "${K8S_NAMESPACE:?K8S_NAMESPACE is required}"
: "${DCS_SERVICE:?DCS_SERVICE is required}"
: "${LOCAL_FORWARD_PORT:?LOCAL_FORWARD_PORT is required}"
: "${SERVICE_PORT:?SERVICE_PORT is required}"
: "${ORCE_SERVICE:?ORCE_SERVICE is required}"
: "${ORCE_LOCAL_FORWARD_PORT:?ORCE_LOCAL_FORWARD_PORT is required}"
: "${BDD_KIND_CONTROL_PLANE:?BDD_KIND_CONTROL_PLANE is required}"
: "${BDD_TRAEFIK_NODE_PORT:?BDD_TRAEFIK_NODE_PORT is required}"

cd "$PROJECT_ROOT"

JUNIT_DIR="$BDD_UI_REPORT_DIR/junit"
rm -rf "$BDD_UI_REPORT_DIR"
mkdir -p "$JUNIT_DIR"

RUNTIME_KUBECONFIG=/tmp/kubeconfig-runtime
sed -E \
  "s#server: https://(127\\.0\\.0\\.1|localhost):[0-9]+#server: https://${BDD_KIND_CONTROL_PLANE}:6443#" \
  "$KUBECONFIG" > "$RUNTIME_KUBECONFIG"
export KUBECONFIG="$RUNTIME_KUBECONFIG"

FORWARD_PIDS=()
cleanup() {
  for pid in "${FORWARD_PIDS[@]}"; do
    kill "$pid" >/dev/null 2>&1 || true
  done
}
trap cleanup EXIT

socat "TCP-LISTEN:18080,reuseaddr,fork" "TCP:${BDD_KIND_CONTROL_PLANE}:${BDD_TRAEFIK_NODE_PORT}" \
  > "$BDD_UI_REPORT_DIR/ingress-forward.log" 2>&1 &
FORWARD_PIDS+=("$!")
kubectl -n "$K8S_NAMESPACE" port-forward service/dcs-postgresql 5432:5432 \
  > "$BDD_UI_REPORT_DIR/postgresql-forward.log" 2>&1 &
FORWARD_PIDS+=("$!")
kubectl -n "$K8S_NAMESPACE" port-forward "service/$DCS_SERVICE" "$LOCAL_FORWARD_PORT:$SERVICE_PORT" \
  > "$BDD_UI_REPORT_DIR/dcs-forward.log" 2>&1 &
FORWARD_PIDS+=("$!")
kubectl -n "$K8S_NAMESPACE" port-forward "service/$ORCE_SERVICE" "$ORCE_LOCAL_FORWARD_PORT:1880" \
  > "$BDD_UI_REPORT_DIR/orce-forward.log" 2>&1 &
FORWARD_PIDS+=("$!")

python - 18080 5432 "$LOCAL_FORWARD_PORT" "$ORCE_LOCAL_FORWARD_PORT" <<'PY'
import socket
import sys
import time

for value in sys.argv[1:]:
    port = int(value)
    deadline = time.monotonic() + 30
    while True:
        try:
            with socket.create_connection(("127.0.0.1", port), timeout=1):
                break
        except OSError:
            if time.monotonic() >= deadline:
                raise SystemExit(f"timed out waiting for local runner port {port}")
            time.sleep(0.25)
PY

date -u +%Y-%m-%dT%H:%M:%SZ > "$BDD_UI_REPORT_DIR/run-id"
touch "$BDD_UI_REPORT_DIR/run-start"

echo "Checking statuslist for BDD at $STATUSLIST_SERVICE_URL"
python tests/bdd/scripts/ensure_statuslist_for_bdd.py
python -c 'import eu.xfsc.bdd.core, playwright' >/dev/null

EXTRA_ARGS=()
if [[ -n "${ARG_BDD:-}" ]]; then
  # shellcheck disable=SC2206
  EXTRA_ARGS=(${ARG_BDD})
fi

JUNIT_ARGS=(--junit --junit-directory "$JUNIT_DIR")
if [[ -n "${ARG_BDD_JUNIT:-}" ]]; then
  # shellcheck disable=SC2206
  JUNIT_ARGS=(${ARG_BDD_JUNIT})
fi

coverage run --append -m behave "${JUNIT_ARGS[@]}" "$FEATURES_PATH" "${EXTRA_ARGS[@]}"
