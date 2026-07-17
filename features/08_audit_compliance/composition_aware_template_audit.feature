@DCS-FR-TR-07 @DCS-FR-TR-20 @DCS-FR-TR-23 @DCS-FR-TR-25 @DCS-FR-TR-26 @UC-02 @UC-08-01
Feature: Composition-aware policy audit of contract templates

  @clean_db @REQ-composition-aware-template-audit-AC1
  Scenario: Immediate component data satisfies the contract-template data rule
    Given contract template "Composed Data Template" has no root contract data and embeds an immediate component with valid contract data
    When the Auditor audits policies of template "Composed Data Template"
    Then template policy rule "FACIS-TPL-DATA-001" has no finding

  @clean_db @REQ-composition-aware-template-audit-AC2
  Scenario: Missing data in the entire composition is reported
    Given contract template "Data-Free Composition" and its immediate component declare no contract data
    When the Auditor audits policies of template "Data-Free Composition"
    Then template policy rule "FACIS-TPL-DATA-001" has an error finding

  @clean_db @REQ-composition-aware-template-audit-AC3
  Scenario: Valid component data does not hide a malformed root requirement
    Given contract template "Malformed Root Data Template" has a malformed root data requirement and a valid immediate component requirement
    When the Auditor audits policies of template "Malformed Root Data Template"
    Then template policy rule "FACIS-TPL-DATA-001" has an error finding

  @clean_db @REQ-composition-aware-template-audit-AC3
  Scenario: Valid root data does not hide a malformed component requirement
    Given contract template "Malformed Component Data Template" has valid root data and a malformed immediate component requirement
    When the Auditor audits policies of template "Malformed Component Data Template"
    Then template policy rule "FACIS-TPL-DATA-001" has an error finding

  @clean_db @REQ-composition-aware-template-audit-AC4
  Scenario: Immediate component clause binding satisfies the clause-semantics rule
    Given contract template "Composed Clause Template" has no bound root clause and embeds an immediate component with a clause bound to its contract data
    When the Auditor audits policies of template "Composed Clause Template"
    Then template policy rule "FACIS-TPL-CLAUSE-001" has no finding

  @clean_db @REQ-composition-aware-template-audit-AC5
  Scenario: Missing clause binding in the entire composition is reported
    Given contract template "Unbound Clause Composition" and its immediate component have no clause bound to contract data
    When the Auditor audits policies of template "Unbound Clause Composition"
    Then template policy rule "FACIS-TPL-CLAUSE-001" has an error finding

  @clean_db @REQ-composition-aware-template-audit-AC6
  Scenario: Invalid component policy and domain content is not hidden by valid root content
    Given contract template "Invalid Effective Content Template" has valid root content and an immediate component with an invalid policy operand and domain field
    When the Auditor audits policies of template "Invalid Effective Content Template"
    Then template policy rule "FACIS-TPL-POLICY-001" has an error finding
    And template policy rule "FACIS-TPL-DOMAIN-001" has an error finding

  @clean_db @REQ-composition-aware-template-audit-AC6
  Scenario: Component fields and policy satisfy effective content rules
    Given contract template "Required Component Fields Template" has no root contract data and embeds a canonical policy with constrained jurisdiction, country, and signature fields
    When the Auditor audits policies of template "Required Component Fields Template"
    Then template policy rules have no findings for
      | rule_id                     |
      | FACIS-TPL-POLICY-001        |
      | FACIS-TPL-CONSTRAINT-001    |
      | FACIS-TPL-LEGAL-001         |
      | FACIS-TPL-PARTY-001         |
      | FACIS-TPL-SIGN-001          |

  @clean_db @REQ-composition-aware-template-audit-AC7
  Scenario: Component layout does not replace root document structure validation
    Given contract template "Root Structure Template" has a valid root structure and embeds a snapshot with an invalid component layout
    When the Auditor audits policies of template "Root Structure Template"
    Then template policy rule "FACIS-TPL-STRUCT-001" has no finding

  @clean_db @REQ-composition-aware-template-audit-AC7
  Scenario: Metadata and lifecycle rules remain scoped to the root template
    Given registered contract template "Root Lifecycle Template" embeds a component snapshot with missing metadata and a draft state marker
    When the Auditor audits policies of template "Root Lifecycle Template"
    Then template policy rule "FACIS-TPL-AUDIT-001" has no finding
    And template policy rule "FACIS-TPL-LIFECYCLE-001" has no finding

  @clean_db @REQ-composition-aware-template-audit-AC7
  Scenario: Audit uses immediate snapshots without mutation or repository resolution
    Given contract template "Snapshot Boundary Template" embeds a valid immediate component with invalid nested snapshot content
    And the authoritative component content changes after the parent snapshot was persisted
    When the Auditor audits policies of template "Snapshot Boundary Template"
    Then template policy rules have no findings for
      | rule_id                 |
      | FACIS-TPL-DATA-001      |
      | FACIS-TPL-CLAUSE-001    |
      | FACIS-TPL-DOMAIN-001    |
    And the persisted immediate component snapshot is unchanged

  @clean_db @REQ-composition-aware-template-audit-AC8
  Scenario: Standalone component audit keeps component-specific rules
    Given standalone component template "Standalone Invalid Component" has an incomplete contract data field
    When the Auditor audits policies of template "Standalone Invalid Component"
    Then template policy rule "FACIS-COMP-DATA-001" has an error finding
    And template policy rule "FACIS-TPL-DATA-001" has no finding
