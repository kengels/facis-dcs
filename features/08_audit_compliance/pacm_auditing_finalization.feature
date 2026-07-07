Feature: PACM auditing finalization
  The PACM audit API stores synchronous AuditRuns, exposes their findings and reports,
  and keeps SIGNATURE visible without pretending that signing validation exists.

  @REQ-pacm-auditing-finalisierung-AC1 @DCS-IR-PACM-01 @DCS-FR-SM-21 @DCS-FR-SM-26 @DCS-IR-SM-05
  Scenario: SIGNATURE audit scope stays selectable and returns only skipped findings
    Given I am authenticated with roles: "Auditor"
    When an Auditor starts a PACM audit for scope "SIGNATURE"
    Then the PACM audit request succeeds
    And the returned AuditRun has scope "SIGNATURE"
    And every returned AuditFinding has status "SKIPPED"
    And every returned AuditFinding explains that signature validation is unavailable because signing is not implemented

  @REQ-pacm-auditing-finalisierung-AC2 @DCS-IR-PACM-01 @DCS-IR-CI-10
  Scenario Outline: Unsupported audit scopes are rejected
    Given I am authenticated with roles: "Auditor"
    When an Auditor starts a PACM audit for scope "<scope>"
    Then the PACM audit request is rejected as bad request

    Examples:
      | scope       |
      | USER        |
      | STORAGE     |
      | COMPLIANCE  |
      | CONTRACTING |

  @REQ-pacm-auditing-finalisierung-AC3 @UC-08-02 @DCS-FR-PACM-03 @DCS-IR-CI-10
  Scenario: POST audit runs synchronously and stores a result status
    Given I am authenticated with roles: "Auditor"
    When an Auditor starts a PACM audit for scope "CONTRACT"
    Then the PACM audit request succeeds
    And the synchronous PACM audit response contains a stored AuditRun with result status
    And the stored AuditRun can be retrieved from PACM queries

  @REQ-pacm-auditing-finalisierung-AC4 @UC-08-02 @DCS-FR-PACM-03
  Scenario: Stored AuditRun statuses use the allowed vocabulary
    Given I am authenticated with roles: "Auditor"
    When the stored PACM AuditRun list is queried
    Then every stored AuditRun has one of statuses "PENDING,RUNNING,COMPLETED,FAILED"

  @REQ-pacm-auditing-finalisierung-AC5 @UC-08-02 @DCS-FR-PACM-03
  Scenario: AuditFinding statuses use the allowed vocabulary
    Given I am authenticated with roles: "Auditor"
    When an Auditor starts a PACM audit for scope "CONTRACT"
    Then the PACM audit request succeeds
    And every returned AuditFinding has one of statuses "PASS,WARNING,FAIL,SKIPPED"

  @REQ-pacm-auditing-finalisierung-AC6 @UC-08-02 @DCS-FR-PACM-03 @DCS-FR-CWE-09
  Scenario: CONTRACT audit reports the required technical checks
    Given I am authenticated with roles: "Auditor"
    When an Auditor starts a PACM audit for scope "CONTRACT"
    Then the PACM audit request succeeds
    And the returned AuditRun contains findings for:
      | check                         |
      | contract existence            |
      | contract version              |
      | lifecycle status              |
      | machine-readable contract data |
      | required metadata             |
      | content hash                  |
      | modeled approvals             |

  @REQ-pacm-auditing-finalisierung-AC7 @DCS-FR-TR-07 @DCS-FR-TR-20 @DCS-FR-TR-21 @DCS-FR-PACM-03
  Scenario: TEMPLATE audit reports the required technical checks
    Given I am authenticated with roles: "Auditor"
    When an Auditor starts a PACM audit for scope "TEMPLATE"
    Then the PACM audit request succeeds
    And the returned AuditRun contains findings for:
      | check                    |
      | template existence       |
      | template status          |
      | template version         |
      | JSON-LD context schema   |
      | modeled provenance       |

  @REQ-pacm-auditing-finalisierung-AC8 @DCS-FR-CSA-18 @DCS-FR-CSA-19 @DCS-FR-CSA-24 @UC-08-02
  Scenario: ARCHIVE audit reports the required technical checks
    Given I am authenticated with roles: "Auditor"
    When an Auditor starts a PACM audit for scope "ARCHIVE"
    Then the PACM audit request succeeds
    And the returned AuditRun contains findings for:
      | check                            |
      | archive entry                    |
      | snapshot                         |
      | content hash                     |
      | snapshot CID                     |
      | evidence                         |
      | storage retrieval audit events   |

  @REQ-pacm-auditing-finalisierung-AC9 @DCS-FR-PACM-01 @DCS-IR-CI-10
  Scenario: PACM audit and report generation emit audit events
    Given I am authenticated with roles: "Auditor"
    When an Auditor starts a PACM audit for scope "CONTRACT"
    And PACM reports are generated for the current AuditRun in formats "json"
    Then PACM audit events include "AuditRunStarted,AuditCheckCompleted,AuditRunCompleted,AuditReportGenerated"

  @REQ-pacm-auditing-finalisierung-AC10 @DCS-FR-PACM-02 @DCS-IR-PACM-03
  Scenario: Stored AuditRuns can be filtered by scope, status and date
    Given I am authenticated with roles: "Auditor"
    And an Auditor has started PACM audits for scopes "CONTRACT,TEMPLATE"
    When the stored PACM AuditRun list is queried with scope "CONTRACT", status "COMPLETED", and today's date
    Then every returned AuditRun matches scope "CONTRACT"
    And every returned AuditRun matches status "COMPLETED"
    And every returned AuditRun was created today

  @REQ-pacm-auditing-finalisierung-AC11 @DCS-FR-PACM-05 @DCS-FR-CSA-24 @UC-08-02
  Scenario: Stored AuditRun detail exposes findings with evidence
    Given I am authenticated with roles: "Auditor"
    And an Auditor has started a PACM audit for scope "ARCHIVE"
    When the current stored PACM AuditRun detail is queried
    Then the stored AuditRun detail contains all findings from the AuditRun
    And every finding in the stored AuditRun detail includes evidence

  @REQ-pacm-auditing-finalisierung-AC12 @UC-08-01 @DCS-IR-PACM-02 @DCS-FR-PACM-05 @DCS-FR-CSA-24
  Scenario: JSON CSV and PDF reports use the same stored audit summary
    Given I am authenticated with roles: "Auditor"
    And an Auditor has started a PACM audit for scope "CONTRACT"
    When PACM reports are generated for the current AuditRun in formats "json,csv,pdf"
    Then the generated PACM reports share the same summary
    And the generated PACM reports identify the same AuditRun

  @REQ-pacm-auditing-finalisierung-AC13 @DCS-IR-CI-10
  Scenario: PACM commands use POST and PACM queries use GET without starting audit runs
    Given I am authenticated with roles: "Auditor"
    When the current stored PACM AuditRun count is recorded
    And PACM query endpoints are requested via GET
    Then the stored PACM AuditRun count is unchanged
    And PACM command endpoints reject GET for creating audit or report data

  @REQ-pacm-auditing-finalisierung-AC14 @DCS-IR-PACM-01
  Scenario: Auditing UI start flow is backed by the synchronous audit command
    Given the Auditing UI entrypoint is reachable
    And I am authenticated with roles: "Auditor"
    When an Auditor starts a "CONTRACT" audit through the UI-backed PACM command
    Then the PACM audit request succeeds
    And the synchronous PACM audit response contains a stored AuditRun with result status
    And the Auditing UI exposes a progress indicator for running audits

  @REQ-pacm-auditing-finalisierung-AC15 @DCS-IR-PACM-01 @DCS-IR-PACM-03
  Scenario: Auditing UI list is backed by filterable stored AuditRun queries
    Given the Auditing UI entrypoint is reachable
    And I am authenticated with roles: "Auditor"
    When the stored PACM AuditRun list is queried with scope "CONTRACT", status "COMPLETED", and today's date
    Then every returned AuditRun matches scope "CONTRACT"
    And every returned AuditRun matches status "COMPLETED"
    And every returned AuditRun was created today

  @REQ-pacm-auditing-finalisierung-AC16 @DCS-IR-PACM-02 @DCS-FR-CSA-24
  Scenario: Auditing UI detail is backed by stored findings and evidence
    Given the Auditing UI entrypoint is reachable
    And I am authenticated with roles: "Auditor"
    And an Auditor has started a PACM audit for scope "ARCHIVE"
    When the current stored PACM AuditRun detail is queried
    Then the stored AuditRun detail contains all findings from the AuditRun
    And every finding in the stored AuditRun detail includes evidence

  @REQ-pacm-auditing-finalisierung-AC17 @DCS-IR-PACM-02 @UC-08-01
  Scenario: Auditing UI export actions are backed by JSON CSV and PDF report APIs
    Given the Auditing UI entrypoint is reachable
    And I am authenticated with roles: "Auditor"
    And an Auditor has started a PACM audit for scope "CONTRACT"
    When PACM reports are generated for the current AuditRun in formats "json,csv,pdf"
    Then the generated PACM reports share the same summary
    And the generated PACM reports identify the same AuditRun


  @REQ-pacm-contract-content-policy-audits-AC1 @DCS-FR-PACM-03 @UC-08 @UC-08-02
  Scenario: CONTRACT PACM audit flags stored contract SLA availability below policy minimum
    Given I am authenticated with roles: "Contract Creator"
    And a stored contract "Policy SLA breach" has SLA availability below the contract policy minimum
    And I am authenticated with roles: "Auditor"
    When an Auditor starts a CONTRACT-PACM audit for stored contract "Policy SLA breach"
    Then the PACM audit request succeeds
    And the CONTRACT-PACM audit contains a Contract-Content-Policy finding with rule reference "FACIS-CONTRACT-POLICY-003"

  @REQ-pacm-contract-content-policy-audits-AC2 @DCS-FR-PACM-03 @UC-08 @UC-08-02
  Scenario: CONTRACT PACM audit flags stored contract content structure violations from configured SHACL shapes
    Given I am authenticated with roles: "Contract Creator"
    And a stored contract "Policy SHACL breach" has a Contract-Content structure violation
    And I am authenticated with roles: "Auditor"
    When an Auditor starts a CONTRACT-PACM audit for stored contract "Policy SHACL breach"
    Then the PACM audit request succeeds
    And the CONTRACT-PACM audit contains a failing Contract-Content SHACL finding from the configured shape set

  @REQ-pacm-contract-content-policy-audits-AC3 @DCS-FR-PACM-03 @UC-08-02
  Scenario: CONTRACT PACM audit exposes rule references on every contract content policy finding
    Given I am authenticated with roles: "Contract Creator"
    And a stored contract "Policy rule references" has SLA availability below the contract policy minimum
    And I am authenticated with roles: "Auditor"
    When an Auditor starts a CONTRACT-PACM audit for stored contract "Policy rule references"
    Then the PACM audit request succeeds
    And every Contract-Content-Policy finding in the CONTRACT-PACM audit has a non-empty rule reference

  @REQ-pacm-contract-content-policy-audits-AC4 @UC-08 @UC-08-02
  Scenario: CONTRACT PACM report summary counts passed and failed contract content policy checks
    Given I am authenticated with roles: "Contract Creator"
    And a stored contract "Policy report summary" has SLA availability below the contract policy minimum
    And I am authenticated with roles: "Auditor"
    And an Auditor has started a CONTRACT-PACM audit for stored contract "Policy report summary"
    When a PACM JSON report is generated for the current AuditRun and stored contract "Policy report summary"
    Then the PACM report summary counts passed and failed Contract-Content-Policy results

  @REQ-pacm-contract-content-policy-audits-AC5 @UC-08 @UC-08-02
  Scenario: CONTRACT PACM report export includes contract content policy findings with rule references
    Given I am authenticated with roles: "Contract Creator"
    And a stored contract "Policy report export" has SLA availability below the contract policy minimum
    And I am authenticated with roles: "Auditor"
    And an Auditor has started a CONTRACT-PACM audit for stored contract "Policy report export"
    When a PACM JSON report is generated for the current AuditRun and stored contract "Policy report export"
    Then the PACM report export contains Contract-Content-Policy findings with rule references
