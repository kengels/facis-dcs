@ui @UC-07 @UC-08
Feature: Archive and audit browser evidence
  Archive and compliance actions use role-protected product dashboards and
  expose evidence suitable for browser-based acceptance.

  @clean_db @REQ-ui-traceability-playwright-evidence-AC14 @DCS-IR-CSA-01 @DCS-IR-CSA-02 @DCS-IR-CSA-03 @DCS-IR-CSA-04 @DCS-IR-CSA-05 @DCS-IR-CSA-06 @DCS-FR-CSA-22 @DCS-FR-CSA-24
  Scenario: Archive managers search, annotate, inspect, terminate, delete, and audit while observers remain read-only
    Given contract "Archive Browser Contract" has reached contract state "SIGNED"
    And I am signed in through the DCS login page and test wallet as "Archive Manager"
    When I open UI route "/archive"
    And I fill UI control "archive-search-query" with "Archive Browser Contract"
    And I select UI option "SIGNED" in control "archive-state-filter"
    And I click UI control "archive-search-submit"
    Then contract UI element "archive-search-result" for "Archive Browser Contract" is visible
    When I click contract UI control "archive-result-open" for "Archive Browser Contract"
    Then UI element "archive-contract-details" is visible
    When I fill UI control "archive-annotation-summary" with "Browser archive summary"
    And I fill UI control "archive-annotation-tags" with "browser-evidence,legal-review"
    And I click UI control "archive-annotation-save"
    Then keyed UI element "archive-tag" with key "browser-evidence" is visible
    When I click UI control "archive-integrity-audit"
    Then UI element "archive-integrity-result" contains "Passed"
    When I fill UI control "archive-termination-reason" with "Archive policy termination"
    And I click UI control "archive-terminate"
    Then UI element "archive-state" contains "Terminated"
    When I fill UI control "archive-delete-justification" with "Approved retention disposal"
    And I click UI control "archive-delete"
    Then UI element "archive-delete-result" contains "Deleted"
    When I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Contract Observer"
    And I open UI route "/archive"
    Then UI element "archive-search-submit" is visible
    And UI element "archive-delete" is absent
    And UI element "archive-annotation-save" is absent

  @clean_db @REQ-ui-traceability-playwright-evidence-AC15 @DCS-IR-PACM-01 @DCS-IR-PACM-02 @DCS-IR-PACM-03 @DCS-IR-PACM-04 @DCS-FR-CSA-24 @DCS-FR-PACM-02 @DCS-FR-PACM-05
  Scenario: Auditors export scoped findings and compliance officers report linked incidents
    Given contract "Audit Browser Contract" has reached contract state "SIGNED"
    And template "Audit Browser Template" is approved and available
    And I am signed in through the DCS login page and test wallet as "Auditor"
    When I open UI route "/audit"
    And I select UI option "contracts" in control "audit-scope-filter"
    And I fill UI control "audit-did-filter" with the DID of contract "Audit Browser Contract"
    And I click UI control "audit-run"
    Then contract UI element "audit-result" for "Audit Browser Contract" is visible
    Then a browser download from UI control "audit-export" is produced
    When I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Compliance Officer"
    And I open UI route "/compliance"
    Then UI element "compliance-monitor-findings" is visible
    When I fill UI control "incident-contract-dids" with the DID of contract "Audit Browser Contract"
    And I fill UI control "incident-template-dids" with the DID of template "Audit Browser Template"
    And I select UI option "missing-approval" in control "incident-finding-selection"
    And I fill UI control "incident-reason" with "Browser-linked compliance incident"
    And I click UI control "incident-submit"
    Then UI element "incident-result" contains "Browser-linked compliance incident"
    And UI element "incident-affected-contract" contains the DID of contract "Audit Browser Contract"
    And UI element "incident-affected-template" contains the DID of template "Audit Browser Template"
