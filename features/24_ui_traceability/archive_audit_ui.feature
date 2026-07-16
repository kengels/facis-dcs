@ui @UC-07 @UC-08
Feature: Archive and audit browser evidence
  Archive and compliance actions use role-protected product dashboards and
  expose authoritative server evidence suitable for browser acceptance.

  @clean_db @REQ-ui-traceability-playwright-evidence-AC14 @REQ-ui-gap-resolution-AC17 @DCS-IR-CSA-01 @DCS-IR-CSA-05 @DCS-IR-CSA-06 @DCS-FR-CSA-02 @DCS-FR-CSA-22
  Scenario: Archive search, annotation, drill-down, and export respect roles
    Given named contract "Archive Search Contract" has reached contract state "SIGNED"
    And contract "Archive Search Contract" is present in the Archive Store
    And the Archive Manager annotates the archived contract "Archive Search Contract" with summary "Initial server summary" and tags "browser-evidence"
    And I am signed in through the DCS login page and test wallet as "Archive Manager"
    When I open UI route "/archive"
    And I fill UI control "archive-search-query" with "Archive Search Contract"
    And I select UI option "SIGNED" in control "archive-state-filter"
    And I fill UI control "archive-tag-filter" with "browser-evidence"
    And I click UI control "archive-search-submit"
    Then contract UI element "archive-search-result" for "Archive Search Contract" is visible
    When I click contract UI control "archive-result-open" for "Archive Search Contract"
    Then UI element "archive-contract-details" is visible
    When I fill UI control "archive-annotation-summary" with "Browser archive summary"
    And I fill UI control "archive-annotation-tags" with "browser-evidence,legal-review"
    And I click UI control "archive-annotation-save"
    And I reload the current UI route
    Then keyed UI element "archive-tag" with key "legal-review" is visible
    And UI element "archive-annotation-summary-result" contains "Browser archive summary"
    When I click UI control "archive-search-submit"
    Then a browser download from UI control "archive-search-export" contains the DID of contract "Archive Search Contract"
    When I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Contract Observer"
    And I open UI route "/archive"
    And I fill UI control "archive-search-query" with "Archive Search Contract"
    And I click UI control "archive-search-submit"
    And I click contract UI control "archive-result-open" for "Archive Search Contract"
    Then UI element "archive-contract-details" is visible
    And UI element "archive-delete" is absent
    And UI element "archive-annotation-save" is absent
    And UI element "archive-terminate" is absent

  @clean_db @REQ-ui-gap-resolution-AC18 @DCS-IR-CSA-02
  Scenario: A signed contract exposes its server-generated archive evidence and identifier
    Given named contract "Archived Evidence Contract" has reached contract state "SIGNED"
    And contract "Archived Evidence Contract" is present in the Archive Store
    And I am signed in through the DCS login page and test wallet as "Archive Manager"
    When I open UI route "/archive"
    And I fill UI control "archive-search-query" with "Archived Evidence Contract"
    And I click UI control "archive-search-submit"
    And I click contract UI control "archive-result-open" for "Archived Evidence Contract"
    Then UI element "archive-contract-id" is visible
    And UI element "archive-contract-id" contains the DID of contract "Archived Evidence Contract"
    And UI element "archive-signature-evidence" is visible
    And UI element "archive-server-proof" is visible

  @clean_db @REQ-ui-gap-resolution-AC19 @DCS-IR-CSA-03 @DCS-IR-CSA-04
  Scenario: Integrity, termination, deletion, and their audit use server outcomes
    Given named contract "Archive Lifecycle Contract" has reached contract state "SIGNED"
    And contract "Archive Lifecycle Contract" is present in the Archive Store
    And I am signed in through the DCS login page and test wallet as "Archive Manager"
    When I open UI route "/archive"
    And I fill UI control "archive-search-query" with "Archive Lifecycle Contract"
    And I click UI control "archive-search-submit"
    And I click contract UI control "archive-result-open" for "Archive Lifecycle Contract"
    And I click UI control "archive-integrity-audit"
    Then UI element "archive-integrity-result" contains "Passed"
    And keyed UI element "archive-integrity-rule-reference" with key "ARCHIVE_DB_SNAPSHOT" is visible
    When I fill UI control "archive-termination-reason" with "Archive policy termination"
    And I click UI control "archive-terminate"
    Then UI element "archive-state" contains "Terminated"
    And UI element "archive-termination-result" contains "Archive policy termination"
    When I fill UI control "archive-delete-justification" with "Approved retention disposal"
    And I click UI control "archive-delete"
    Then UI element "archive-delete-result" contains "Deleted"
    When I open UI route "/audit"
    And I select UI option "archive" in control "audit-scope-filter"
    And I fill UI control "audit-did-filter" with the DID of contract "Archive Lifecycle Contract"
    And I fill UI control "audit-justification" with "Verify archived contract termination and deletion evidence"
    And I click UI control "audit-run"
    And I click UI control "audit-tab-timeline"
    Then contract UI element "audit-result" for "Archive Lifecycle Contract" contains "terminate"
    And contract UI element "audit-result" for "Archive Lifecycle Contract" contains "delete"
    When I click keyed UI control "audit-event" with key "TERMINATE_CONTRACT"
    Then UI element "audit-selected-details" contains "Archive policy termination"
    When I click keyed UI control "audit-event" with key "DELETE_ARCHIVED_CONTRACT"
    Then UI element "audit-selected-details" contains "Approved retention disposal"

  @clean_db @REQ-ui-gap-resolution-AC20 @DCS-FR-CSA-21
  Scenario: The archive dashboard shows real recent, expiring, and compliance data
    Given contract "Expiring Archive Contract" is archived and expires within 7 days
    And I am signed in through the DCS login page and test wallet as "Archive Manager"
    When I open UI route "/archive"
    Then UI element "archive-recent-actions" is visible
    And contract UI element "archive-expiring-contract" for "Expiring Archive Contract" is visible
    And UI element "archive-compliance-summary" is visible
    And UI element "archive-statistics-source" contains "server"

  @clean_db @REQ-ui-traceability-playwright-evidence-AC15 @REQ-ui-gap-resolution-AC21 @DCS-IR-PACM-01 @DCS-IR-PACM-02 @DCS-FR-CSA-24
  Scenario: An auditor receives a scoped server report for a contract
    Given contract "Audit Browser Contract" has reached contract state "SIGNED"
    And I am signed in through the DCS login page and test wallet as "Auditor"
    When I open UI route "/audit"
    And I select UI option "contracts" in control "audit-scope-filter"
    And I fill UI control "audit-did-filter" with the DID of contract "Audit Browser Contract"
    And I fill UI control "audit-justification" with "Scoped contract audit browser evidence"
    And I click UI control "audit-run"
    Then contract UI element "audit-result" for "Audit Browser Contract" is visible
    And UI element "audit-report-server-reference" is visible
    And a non-empty browser download from UI control "audit-export" is produced

  @clean_db @REQ-ui-gap-resolution-AC22 @DCS-IR-PACM-03 @DCS-IR-PACM-04 @DCS-FR-PACM-02 @DCS-FR-PACM-05
  Scenario: A compliance officer persists, retrieves, and exports a linked incident case
    Given contract "Incident Browser Contract" is in approval workflow
    And contract "Incident Browser Contract" still has an open required approval task
    And template "Incident Browser Template" is approved and available
    And I am signed in through the DCS login page and test wallet as "Compliance Officer"
    When I open UI route "/compliance"
    And I click UI control "compliance-monitor-run"
    Then contract UI element "compliance-monitor-finding" for "Incident Browser Contract" is visible
    When I select finding from contract "Incident Browser Contract" in UI control "incident-finding-selection"
    And I fill UI control "incident-contract-dids" with the DID of contract "Incident Browser Contract"
    And I fill UI control "incident-template-dids" with the DID of template "Incident Browser Template"
    And I fill UI control "incident-reason" with "Browser-linked compliance incident"
    And I click UI control "incident-submit"
    Then UI element "incident-result" contains "Browser-linked compliance incident"
    And UI element "incident-affected-contract" contains the DID of contract "Incident Browser Contract"
    And UI element "incident-affected-template" contains the DID of template "Incident Browser Template"
    When I remember data-test-key from UI element "incident-result" as "reported-incident"
    And I open UI route "/compliance/incidents"
    And I click keyed UI control "incident-list-open" with remembered key "reported-incident"
    Then UI element "incident-details" contains "Browser-linked compliance incident"
    And a non-empty browser download from UI control "incident-export" is produced
