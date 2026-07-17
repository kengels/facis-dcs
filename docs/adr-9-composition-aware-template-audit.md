# ADR-9: Template policy audits evaluate persisted immediate component snapshots

## Context

A contract template can persist complete component snapshots below
`dcs:metadata.dcs:subTemplates`. Auditing only the root document produces false
findings when required contract data or a bound clause is intentionally supplied
by a component. Resolving the component DID again during an audit would instead
evaluate current repository state rather than the composition that was actually
saved. Flattening component content into the root would mutate or duplicate the
authoritative document.

The policy evaluator therefore needs a composition view that is broad enough to
represent effective contract content but preserves the stored document and its
temporal evidence.

## Decision

- A `CONTRACT_TEMPLATE` policy audit builds a read-only evaluation view from the
  root document and each immediate persisted component snapshot
  (`backend/internal/base/validation/templatepolicy.go:90-123`). It neither
  rewrites the JSON-LD document nor resolves component DIDs from the repository.
- Contract-data presence and validity, policy operands, clause bindings, domain
  fields, constraints, and required domain fields use this effective content
  view (`backend/internal/base/validation/templatepolicy.go:179-204`). Field IDs
  are combined across the same sources so a clause or policy can refer to data
  supplied by the persisted composition
  (`backend/internal/base/validation/templatepolicy.go:726-734`).
- Canonical root structure, audit metadata, and lifecycle state remain root-only.
  Component-specific rules also remain scoped to a standalone `COMPONENT`
  template (`backend/internal/base/validation/templatepolicy.go:179-204`).
- Evaluation stops after immediate snapshots. A snapshot's own nested snapshots
  are not recursively added to the view. This boundary matches the persisted
  parent composition and avoids silently expanding its audit scope.
- Findings originating in a component retain a path prefixed with the persisted
  snapshot location (`backend/internal/base/validation/templatepolicy.go:736-748`).
  The event evidence preserves that path for reports and forensic analysis
  (`backend/internal/processauditandcompliance/policy_audit.go:32-43`).

## Consequences

- A valid component can satisfy composition-wide presence rules, while malformed
  root or component content still produces its own finding.
- Re-running an audit evaluates the same saved composition even if the reusable
  component has since changed in the repository.
- Audit execution is side-effect free for the template and its snapshots.
- Transitive nested snapshots do not contribute content. If transitive
  composition becomes a product requirement, its ownership and cycle semantics
  require a separate decision rather than an implicit recursive lookup.
- Technical snapshot paths are available in audit evidence and JSON reports;
  compact UI cards may continue to emphasize the user-facing rule, severity,
  and message.
