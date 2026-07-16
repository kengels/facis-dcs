@ui @UC-02
Feature: Template Repository browser evidence
  Template governance and structural validation are performed through the
  delivered product UI and remain visible after a browser reload.

  @clean_db @REQ-ui-traceability-playwright-evidence-AC4 @DCS-IR-TR-01 @DCS-IR-TR-02 @DCS-FR-TR-13 @DCS-FR-TR-14
  Scenario: A creator creates, edits, submits, searches, and retrieves a template in the UI
    Given I am signed in through the DCS login page and test wallet as "Template Creator"
    When I open UI route "/templates/new"
    And I click UI control "template-type-contract"
    And I fill these UI controls:
      | test_id                     | value                     |
      | template-editor-name        | Browser Evidence Template |
      | template-editor-description | Created through Playwright |
    And I click UI control "template-editor-save"
    Then UI element "template-save-result" contains "Draft"
    When I remember data-test-key from UI element "template-save-result" as "created-template"
    And I click keyed UI control "template-search-result-edit" with remembered key "created-template"
    And I fill UI control "template-editor-description" with "Edited through Playwright"
    And I click UI control "template-editor-save"
    And I click keyed UI control "template-search-result-open" with remembered key "created-template"
    And I click UI control "template-submit-review"
    Then keyed UI element "template-search-result" with remembered key "created-template" contains "Submitted"
    When I click UI control "template-search-filter-open"
    And I click keyed UI control "template-search-filter-option" with key "name"
    And I fill UI control "template-search-query" with "Browser Evidence Template"
    And I click UI control "template-search-submit"
    Then keyed UI element "template-search-result" with remembered key "created-template" is visible
    When I click keyed UI control "template-search-result-open" with remembered key "created-template"
    Then UI element "template-details-name" contains "Browser Evidence Template"
    And UI element "template-details-description" contains "Edited through Playwright"

  @clean_db @REQ-ui-traceability-playwright-evidence-AC5 @REQ-ui-gap-resolution-AC1 @DCS-IR-TR-03 @DCS-IR-TR-04 @DCS-FR-TR-20
  Scenario Outline: A reviewer records a verified template decision
    Given template "Review Browser Template" is in "Submitted" status
    And I am signed in through the DCS login page and test wallet as "Template Reviewer"
    When I open UI route "/tasks/reviews"
    And I click template UI control "template-review-task-open" for "Review Browser Template"
    And I click UI control "template-review-verify"
    Then UI element "template-verification-result" contains "valid"
    When I click UI control "template-verification-close"
    And I click UI control "<decision_control>"
    Then UI element "template-review-comment" is visible
    When I fill UI control "template-review-comment" with "<comment>"
    And I click UI control "template-review-comment-submit"
    Then current UI route is "/tasks/reviews"
    When I open the details UI for template "Review Browser Template"
    And I reload the current UI route
    Then UI element "template-lifecycle-status" contains "<state>"
    And UI element "template-review-history" contains "<comment>"

    Examples:
      | decision_control                 | comment                       | state    |
      | template-review-return-draft     | Returned after browser review | Draft    |
      | template-review-forward-approval | Verified in browser review    | Reviewed |

  @clean_db @REQ-ui-gap-resolution-AC2 @DCS-IR-TR-05 @DCS-IR-TR-06 @DCS-FR-TR-15
  Scenario: An approver rejects with a reason and later approves a resubmission
    Given template "Approval Browser Template" is in "Reviewed" status
    And I am signed in through the DCS login page and test wallet as "Template Approver"
    When I open UI route "/tasks/approvals"
    And I click template UI control "template-approval-task-open" for "Approval Browser Template"
    And I click UI control "template-approval-reject"
    Then UI element "template-approval-decision-note" is visible
    When I fill UI control "template-approval-decision-note" with "Approval evidence is incomplete"
    And I click UI control "template-approval-decision-submit"
    Then current UI route is "/tasks/approvals"
    When I open the details UI for template "Approval Browser Template"
    Then UI element "template-lifecycle-status" contains "Draft"
    And UI element "template-approval-history" contains "Approval evidence is incomplete"
    When I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Template Creator"
    And I open the details UI for template "Approval Browser Template"
    And I click UI control "template-resubmit-review"
    And I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Template Reviewer"
    And I complete the template review for "Approval Browser Template" through the browser
    And I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Template Approver"
    And I open UI route "/tasks/approvals"
    And I click template UI control "template-approval-task-open" for "Approval Browser Template"
    And I click UI control "template-approval-approve"
    Then UI element "template-approval-decision-note" is visible
    When I fill UI control "template-approval-decision-note" with "Approved after browser review"
    And I click UI control "template-approval-decision-submit"
    Then current UI route is "/tasks/approvals"
    When I open the details UI for template "Approval Browser Template"
    And I reload the current UI route
    Then UI element "template-lifecycle-status" contains "Approved"
    And UI element "template-contract-ready-indicator" contains "ready"

  @clean_db @REQ-ui-gap-resolution-AC3 @DCS-IR-TR-07 @DCS-IR-TR-08 @DCS-FR-TR-08 @DCS-FR-TR-16 @DCS-FR-TR-17 @DCS-FR-TR-19 @DCS-FR-TR-21 @DCS-FR-TR-28
  Scenario: A manager registers, updates, audits, and deprecates a template
    Given template "Managed Browser Template" is in "Approved" status
    And I am signed in through the DCS login page and test wallet as "Template Manager"
    When I open UI route "/templates"
    And I click template UI control "template-search-result-edit" for "Managed Browser Template"
    And I fill UI control "template-editor-description" with "Manager update through browser"
    And I click UI control "template-editor-save"
    Then UI element "template-save-result" contains "Draft"
    When I open the details UI for template "Managed Browser Template"
    Then UI element "template-details-description" contains "Manager update through browser"
    And UI element "template-version-history" contains "Current version"
    And UI element "template-version-history" contains "Historical version"
    When I click UI control "template-manager-audit"
    Then UI element "template-audit-timeline" is visible
    And UI element "template-audit-timeline" contains "Manager update through browser"
    When I open UI route "/templates"
    And I click template UI control "template-manager-register" for "Managed Browser Template"
    Then template UI element "template-search-result" for "Managed Browser Template" contains "Registered"
    When I open the details UI for template "Managed Browser Template"
    Then UI element "template-provenance-history" contains "Version"
    When I open UI route "/templates"
    And I click template UI control "template-manager-archive" for "Managed Browser Template"
    And I click template UI control "template-manager-confirm" for "Managed Browser Template"
    Then template UI element "template-search-result" for "Managed Browser Template" contains "Deprecated"
    When I open the details UI for template "Managed Browser Template"
    Then UI element "template-lifecycle-status" contains "Deprecated"

  @clean_db @REQ-ui-gap-resolution-AC4 @DCS-FR-TR-18
  Scenario: A deprecated template can be soft-deleted and is excluded from reuse
    Given template "Deleted Browser Template" is in "Approved" status
    And I am signed in through the DCS login page and test wallet as "Template Manager"
    When I open UI route "/templates"
    And I click template UI control "template-manager-register" for "Deleted Browser Template"
    Then template UI element "template-search-result" for "Deleted Browser Template" contains "Registered"
    When I click template UI control "template-manager-archive" for "Deleted Browser Template"
    And I click template UI control "template-manager-confirm" for "Deleted Browser Template"
    Then template UI element "template-search-result" for "Deleted Browser Template" contains "Deprecated"
    When I open the details UI for template "Deleted Browser Template"
    Then UI element "template-lifecycle-status" contains "Deprecated"
    When I click UI control "template-manager-delete"
    Then UI element "template-delete-confirmation" is visible
    When I click UI control "template-delete-confirm"
    Then current UI route is "/templates"
    When I fill UI control "template-search-query" with "Deleted Browser Template"
    And I click UI control "template-search-submit"
    Then template UI element "template-search-result" for "Deleted Browser Template" is absent
    When I open UI route "/templates/deleted"
    Then template UI element "template-deletion-tombstone" for "Deleted Browser Template" is visible
    When I open UI route "/contracts/new"
    Then template option for "Deleted Browser Template" in UI control "contract-create-template" is absent

  @clean_db @REQ-ui-traceability-playwright-evidence-AC6 @REQ-ui-gap-resolution-AC5 @DCS-FR-TR-25
  Scenario: A nested template hierarchy is visible in the builder
    Given template "Nested Browser Template" is approved and available
    And I am signed in through the DCS login page and test wallet as "Template Creator"
    When I open the edit UI for template "Nested Browser Template"
    Then template UI element "template-hierarchy-node" for "Nested Browser Template" is visible
    And UI element "template-hierarchy-tree" is visible
    And UI element "template-hierarchy-dependencies" is visible

  @clean_db @REQ-ui-gap-resolution-AC6 @DCS-FR-TR-26
  Scenario Outline: Authoritative dependency validation blocks missing and malformed references
    Given template "Dependency Browser Template" is in "Draft" status
    And I am signed in through the DCS login page and test wallet as "Template Creator"
    When I open the edit UI for template "Dependency Browser Template"
    And I click UI control "template-component-add"
    And I fill UI control "template-component-reference" with "<reference>"
    And I click UI control "template-component-save"
    Then UI element "template-dependency-error" contains "<error>"
    And UI element "template-save-result" contains "blocked"
    When I reload the current UI route
    Then keyed UI element "template-hierarchy-node" with key "<reference>" is absent

    Examples:
      | reference                         | error   |
      | did:web:missing.example:template  | missing |
      | urn:not-a-valid-template-reference | invalid |

  @clean_db @REQ-ui-gap-resolution-AC6 @DCS-FR-TR-26
  Scenario: Authoritative dependency validation blocks a real self-cycle
    Given template "Cyclic Browser Template" is in "Draft" status
    And I am signed in through the DCS login page and test wallet as "Template Creator"
    When I open the edit UI for template "Cyclic Browser Template"
    And I click UI control "template-component-add"
    And I fill UI control "template-component-reference" with the DID of template "Cyclic Browser Template"
    And I click UI control "template-component-save"
    Then UI element "template-dependency-error" contains "cycle"
    And UI element "template-save-result" contains "blocked"
    When I reload the current UI route
    Then template UI element "template-dependency-edge" for "Cyclic Browser Template" is absent
