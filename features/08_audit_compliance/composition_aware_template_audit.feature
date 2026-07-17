@DCS-FR-TR-07 @DCS-FR-TR-20 @UC-02 @UC-08-01
Feature: Composition-aware policy audit of persisted template snapshots

  @clean_db @REQ-template-composition-policy-audit-AC1
  Scenario: Immediate component data satisfies the composition-wide data requirement
    Given contract template "Composed Data Template" has no root contract data and embeds an immediate component with valid contract data
    When the Auditor audits policies of template "Composed Data Template"
    Then template policy rule "FACIS-TPL-DATA-001" has no finding

  @clean_db @REQ-template-composition-policy-audit-AC2
  Scenario: An immediate component clause satisfies composition-wide clause binding
    Given contract template "Composed Clause Template" has no bound root clause and embeds an immediate component with a clause bound to its contract data
    When the Auditor audits policies of template "Composed Clause Template"
    Then template policy rule "FACIS-TPL-CLAUSE-001" has no finding

  @clean_db @REQ-template-composition-policy-audit-AC3
  Scenario: An immediate component policy may reference a field declared by the root
    Given contract template "Shared Policy Field Scope" has an immediate component policy that references a field declared only by the root
    When the Auditor audits policies of template "Shared Policy Field Scope"
    Then template policy rule "FACIS-TPL-POLICY-001" has no finding

  @clean_db @REQ-template-composition-policy-audit-AC4
  Scenario: An unknown root domain field is reported at the root source
    Given contract template "Unknown Root Domain" has an unknown root domain field and a valid immediate component
    When the Auditor audits policies of template "Unknown Root Domain"
    Then template policy rule "FACIS-TPL-DOMAIN-001" has an error finding below "semanticConditions."

  @clean_db @REQ-template-composition-policy-audit-AC4
  Scenario: An unknown component domain field is reported at the persisted snapshot source
    Given contract template "Unknown Component Domain" has valid root content and an immediate component with an unknown domain field
    When the Auditor audits policies of template "Unknown Component Domain"
    Then template policy rule "FACIS-TPL-DOMAIN-001" has an error finding below "dcs:metadata.dcs:subTemplates[0].dcs:template."

  @clean_db @REQ-template-composition-policy-audit-AC5
  Scenario: A malformed root policy constraint is reported at the root source
    Given contract template "Missing Root Constraint" has a root policy constraint without an operator and a valid immediate component
    When the Auditor audits policies of template "Missing Root Constraint"
    Then template policy rule "FACIS-TPL-POLICY-001" has an error finding below "dcs:policies"

  @clean_db @REQ-template-composition-policy-audit-AC5
  Scenario: A malformed component policy constraint is reported at the persisted snapshot source
    Given contract template "Missing Component Constraint" has valid root content and an immediate component policy constraint without an operator
    When the Auditor audits policies of template "Missing Component Constraint"
    Then template policy rule "FACIS-TPL-POLICY-001" has an error finding below "dcs:metadata.dcs:subTemplates[0].dcs:template.dcs:policies"

  @clean_db @REQ-template-composition-policy-audit-AC6
  Scenario: Required fields are satisfied by the union of root and immediate component fields
    Given contract template "Required Field Union" distributes required jurisdiction, country, and signature fields across root and immediate component
    When the Auditor audits policies of template "Required Field Union"
    Then template policy rules have no findings for
      | rule_id              |
      | FACIS-TPL-LEGAL-001  |
      | FACIS-TPL-PARTY-001  |
      | FACIS-TPL-SIGN-001   |

  @clean_db @REQ-template-composition-policy-audit-AC7
  Scenario: Malformed root content remains visible despite valid component content
    Given contract template "Malformed Root Data Template" has a malformed root data requirement and a valid immediate component requirement
    When the Auditor audits policies of template "Malformed Root Data Template"
    Then template policy rule "FACIS-TPL-DATA-001" has an error finding below "dcs:contractData"

  @clean_db @REQ-template-composition-policy-audit-AC8
  Scenario: Malformed component content identifies its persisted snapshot location
    Given contract template "Malformed Component Data Template" has valid root data and a malformed immediate component requirement
    When the Auditor audits policies of template "Malformed Component Data Template"
    Then template policy rule "FACIS-TPL-DATA-001" has an error finding below "dcs:metadata.dcs:subTemplates[0].dcs:template.dcs:contractData"

  @clean_db @REQ-template-composition-policy-audit-AC9
  Scenario: Component structure metadata and lifecycle do not replace root-only checks
    Given registered contract template "Root-only Policy Scope" has valid root structure metadata and lifecycle and embeds a component snapshot invalid in those areas
    When the Auditor audits policies of template "Root-only Policy Scope"
    Then template policy rules have no findings for
      | rule_id                    |
      | FACIS-TPL-STRUCT-001       |
      | FACIS-TPL-AUDIT-001        |
      | FACIS-TPL-LIFECYCLE-001    |

  # Registered component content is immutable through the public API. The fixture therefore
  # simulates a later repository version at the persistence boundary, while the audit itself
  # remains a public API call and must continue to use the parent's persisted snapshot.
  @clean_db @REQ-template-composition-policy-audit-AC10
  Scenario: A later repository component change does not alter the parent audit
    Given contract template "Persisted Snapshot Audit" embeds a valid immediate component snapshot
    And the authoritative component content changes after the parent snapshot was persisted
    When the Auditor audits policies of template "Persisted Snapshot Audit"
    Then template policy rule "FACIS-TPL-DOMAIN-001" has no finding
    And the persisted immediate component snapshot is unchanged

  @clean_db @REQ-template-composition-policy-audit-AC11
  Scenario: Audit is read-only non-recursive and does not resolve the immediate component DID
    Given contract template "Snapshot Boundary Template" embeds a valid immediate component with invalid nested snapshot content
    And the authoritative component content changes after the parent snapshot was persisted
    When the Auditor audits policies of template "Snapshot Boundary Template"
    Then template policy rule "FACIS-TPL-DOMAIN-001" has no finding
    And the persisted immediate component snapshot is unchanged

  @clean_db @REQ-template-composition-policy-audit-AC12
  Scenario: A standalone component keeps component-specific policy rules
    Given standalone component template "Standalone Invalid Component" has an incomplete contract data field
    When the Auditor audits policies of template "Standalone Invalid Component"
    Then template policy rule "FACIS-COMP-DATA-001" has an error finding
    And template policy rule "FACIS-TPL-DATA-001" has no finding
