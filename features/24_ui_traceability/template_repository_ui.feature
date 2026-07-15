@ui @UC-02
Feature: Template Repository browser evidence
  Template lifecycle actions are performed and observed through the product UI.

  @clean_db @REQ-ui-traceability-playwright-evidence-AC4 @DCS-IR-TR-01 @DCS-IR-TR-02 @DCS-FR-TR-13 @DCS-FR-TR-14
  Scenario: A creator creates, edits, submits, searches, and retrieves a template in the UI
    Given I am signed in through the DCS login page and test wallet as "Template Creator"
    When I open UI route "/templates/new"
    And I click UI control "template-type-contract"
    And I fill these UI controls:
      | test_id                       | value                         |
      | template-editor-name          | Browser Evidence Template     |
      | template-editor-description   | Created through Playwright    |
    And I click UI control "template-editor-save"
    Then UI element "template-save-result" contains "Draft"
    When I click keyed UI control "template-search-result-edit" from "template-save-result"
    And I fill UI control "template-editor-description" with "Edited through Playwright"
    And I click UI control "template-editor-save"
    And I click keyed UI control "template-search-result-open" from "template-save-result"
    And I click UI control "template-submit-review"
    Then keyed UI element "template-search-result" from "template-save-result" contains "Submitted"
    When I click UI control "template-search-filter-open"
    And I click keyed UI control "template-search-filter-option" with key "name"
    And I fill UI control "template-search-query" with "Browser Evidence Template"
    And I click UI control "template-search-submit"
    Then keyed UI element "template-search-result" from "template-save-result" is visible
    When I click keyed UI control "template-search-result-open" from "template-save-result"
    Then UI element "template-details-name" contains "Browser Evidence Template"
    And UI element "template-details-description" contains "Edited through Playwright"

  @clean_db @REQ-ui-traceability-playwright-evidence-AC5 @DCS-IR-TR-03 @DCS-IR-TR-04 @DCS-IR-TR-05 @DCS-IR-TR-06 @DCS-IR-TR-07 @DCS-IR-TR-08 @DCS-FR-TR-08 @DCS-FR-TR-15 @DCS-FR-TR-16 @DCS-FR-TR-17 @DCS-FR-TR-18 @DCS-FR-TR-19 @DCS-FR-TR-20 @DCS-FR-TR-21 @DCS-FR-TR-28
  Scenario: Reviewer, approver, and manager complete the governed template lifecycle in the UI
    Given template "Governed Browser Template" is in "Submitted" status
    And I am signed in through the DCS login page and test wallet as "Template Reviewer"
    When I open UI route "/tasks/reviews"
    And I click template UI control "template-review-task-open" for "Governed Browser Template"
    And I click UI control "template-review-verify"
    Then UI element "template-verification-result" contains "valid"
    When I fill UI control "template-review-comment" with "Verified in browser review"
    And I click UI control "template-review-return-draft"
    Then UI element "template-lifecycle-status" contains "Draft"
    When I click UI control "template-resubmit-review"
    And I click UI control "template-review-forward-approval"
    And I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Template Approver"
    And I open UI route "/tasks/approvals"
    And I click template UI control "template-approval-task-open" for "Governed Browser Template"
    And I click UI control "template-approval-approve"
    Then UI element "template-lifecycle-status" contains "Approved"
    And UI element "template-contract-ready-indicator" contains "ready"
    When I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Template Manager"
    And I open UI route "/templates"
    And I click template UI control "template-manager-register" for "Governed Browser Template"
    Then UI element "template-lifecycle-status" contains "Registered"
    When I click template UI control "template-manager-audit" for "Governed Browser Template"
    Then UI element "template-audit-timeline" is visible
    When I click template UI control "template-manager-archive" for "Governed Browser Template"
    Then UI element "template-lifecycle-status" contains "Deprecated"

  @clean_db @REQ-ui-traceability-playwright-evidence-AC6 @DCS-FR-TR-25 @DCS-FR-TR-26
  Scenario: Nested template components and dependency failures are visible in the builder
    Given template "Nested Browser Template" is approved and available
    And I am signed in through the DCS login page and test wallet as "Template Creator"
    When I open the edit UI for template "Nested Browser Template"
    Then template UI element "template-hierarchy-node" for "Nested Browser Template" is visible
    When I click UI control "template-component-add"
    And I fill UI control "template-component-reference" with "did:web:invalid.example:missing"
    And I click UI control "template-component-save"
    Then UI element "template-dependency-error" contains "dependency"
    And UI element "template-save-result" contains "blocked"
