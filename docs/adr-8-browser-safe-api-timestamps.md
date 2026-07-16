# ADR-8: API timestamps are normalized to UTC RFC3339 at the service boundary

## Context

Several hand-written service mappings exposed Go's default `time.Time.String()`
representation, for example `2026-07-16 10:12:13 +0000 UTC`. That representation
is intended for diagnostics, not as a wire format, and is not required to be
accepted by JavaScript's `Date` parser. Template lifecycle evidence could
therefore render `Invalid Date` even though the stored audit timestamp was valid.

The affected response mappings were template audit, contract audit, signature
audit, and retrieved contract negotiations. Audit persistence and its hash chain
already use their own canonical payload and must not be rewritten to solve a
presentation-boundary problem.

## Decision

- Hand-written API mappings serialize `time.Time` through the shared
  `formatAPITimestamp` helper in `backend/internal/service/api_timestamp.go`.
- The wire representation is UTC RFC3339 (`time.RFC3339`), for example
  `2026-07-16T10:12:13Z`.
- The normalization applies to the four affected mappings: template audit,
  contract audit, signature audit, and negotiations returned with a contract.
- Persisted audit entries, canonical audit payloads, and hash-chain inputs remain
  unchanged. No database migration is required.

## Consequences

- Browser clients receive an unambiguous, timezone-independent timestamp and can
  render template lifecycle evidence without `Invalid Date`.
- Timestamp formatting is owned by the API boundary instead of being repeated in
  Vue components or relying on browser-specific parsing behavior.
- Regression evidence is provided by
  `features/08_audit_compliance/browser_safe_api_timestamps.feature` (four API
  scenarios) and
  `features/24_ui_traceability/browser_safe_template_timestamps.feature` (one
  Playwright scenario).
- Changing the timestamp wire format does not alter historical audit evidence or
  its integrity guarantees.
