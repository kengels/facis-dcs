@ui @UC-03 @UC-06
Feature: Contract Workflow browser evidence
  Contract creation, governance, monitoring, and lifecycle actions are
  performed and verified through the product UI.

  @clean_db @REQ-ui-traceability-playwright-evidence-AC7 @DCS-IR-CWE-01 @DCS-IR-CWE-02 @DCS-FR-CWE-03 @DCS-FR-CWE-13 @DCS-FR-CWE-14
  Scenario: A creator builds and submits a populated contract from an approved template
    Given template "Contract Browser Template" is approved and available
    And I am signed in through the DCS login page and test wallet as "Contract Creator"
    When I open UI route "/contracts/new"
    And I select template "Contract Browser Template" in UI control "contract-create-template"
    And I fill these UI controls:
      | test_id                         | value                                      |
      | contract-create-name             | Browser Evidence Contract                  |
      | contract-create-party             | did:web:buyer.example                      |
      | contract-create-asset             | urn:asset:browser-evidence                  |
      | contract-create-policy            | dcs:ServiceLevelAgreement                   |
      | contract-create-evidence          | urn:evidence:browser-evidence               |
    And I click UI control "contract-create-save-draft"
    Then UI element "contract-save-result" contains "Draft"
    When I fill UI control "contract-create-evidence" with "urn:evidence:browser-evidence-v2"
    And I click UI control "contract-create-save-draft"
    And I click UI control "contract-submit-review"
    Then UI element "contract-lifecycle-status" contains "Review"
    And UI element "contract-template-reference" contains the DID of template "Contract Browser Template"

  @clean_db @REQ-ui-traceability-playwright-evidence-AC8 @DCS-IR-CWE-03 @DCS-IR-CWE-04 @DCS-IR-CWE-05 @DCS-IR-CWE-06 @DCS-IR-CWE-07 @DCS-IR-CWE-08 @DCS-IR-CWE-09 @DCS-IR-CWE-10 @DCS-FR-CWE-15 @DCS-FR-CWE-17 @DCS-FR-CWE-18 @DCS-FR-CWE-25
  Scenario: Negotiation, review, and approval evidence remains visible through the UI workflow
    Given contract "Governed Browser Contract" is open for negotiation
    And I am signed in through the DCS login page and test wallet as "Contract Negotiator"
    When I open the negotiation UI for contract "Governed Browser Contract"
    And I fill UI control "contract-negotiation-comment" with "Clarify the browser delivery term"
    And I fill UI control "contract-negotiation-redline" with "Delivery within five browser days"
    And I click UI control "contract-negotiation-submit"
    Then UI element "contract-negotiation-thread" contains "Clarify the browser delivery term"
    And UI element "contract-version-diff" contains "Delivery within five browser days"
    When I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Contract Reviewer"
    And I open the review UI for contract "Governed Browser Contract"
    And I fill UI control "contract-review-finding" with "Terms verified in browser"
    And I click UI control "contract-review-request-change"
    Then UI element "contract-lifecycle-status" contains "Negotiation"
    When I click UI control "contract-review-forward-approval"
    And I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Contract Approver"
    And I open the approval UI for contract "Governed Browser Contract"
    And I fill UI control "contract-approval-rejection-reason" with "Reason is mandatory evidence"
    And I click UI control "contract-approval-reject"
    Then UI element "contract-approval-result" contains "Reason is mandatory evidence"
    When I click UI control "contract-approval-resubmit"
    And I click UI control "contract-approval-approve"
    Then UI element "contract-lifecycle-status" contains "Signing"

  @clean_db @REQ-ui-traceability-playwright-evidence-AC9 @DCS-IR-CWE-07 @DCS-IR-CWE-11 @DCS-IR-CWE-13 @DCS-FR-CWE-24 @DCS-FR-CWE-27 @DCS-FR-CWE-29 @DCS-FR-CWE-31
  Scenario: The contract dashboard exposes defined lifecycle monitoring without log-token semantics
    Given contract "Dashboard Browser Contract" is in "Draft" status
    And I am signed in through the DCS login page and test wallet as "Contract Manager"
    When I open UI route "/contracts"
    And I fill UI control "contract-search-query" with "Dashboard Browser Contract"
    And I select UI option "Draft" in control "contract-state-filter"
    And I click UI control "contract-search-submit"
    Then contract UI element "contract-dashboard-row" for "Dashboard Browser Contract" is visible
    And UI element "contract-dashboard-lifecycle" contains "Draft"
    And UI element "contract-dashboard-responsibility" is visible
    And UI element "contract-dashboard-deadline" is visible
    And UI element "contract-dashboard-history" is visible
    And UI element "contract-dashboard-kpi" is visible
    And UI element "contract-dashboard-hierarchy" is visible
    And UI element "contract-dashboard-dependencies" is visible

  @clean_db @REQ-ui-traceability-playwright-evidence-AC10 @DCS-IR-CWE-12 @DCS-FR-CWE-11 @DCS-FR-CWE-12 @DCS-FR-CWE-22 @DCS-FR-CWE-23
  Scenario: A manager stores evidence, audits, terminates with a reason, and renews from the UI
    Given contract "Lifecycle Browser Contract" is in "Draft" status
    And I am signed in through the DCS login page and test wallet as "Contract Manager"
    When I open the management UI for contract "Lifecycle Browser Contract"
    And I fill UI control "contract-evidence-reference" with "urn:evidence:lifecycle-browser"
    And I click UI control "contract-evidence-store"
    Then UI element "contract-evidence-result" contains "urn:evidence:lifecycle-browser"
    When I click UI control "contract-manager-audit"
    Then UI element "contract-audit-timeline" is visible
    When I click UI control "contract-manager-terminate"
    Then UI element "contract-termination-error" contains "reason"
    When I fill UI control "contract-termination-reason" with "Terminated by browser acceptance"
    And I click UI control "contract-manager-terminate"
    Then UI element "contract-lifecycle-status" contains "Terminated"
    When I click UI control "contract-manager-renew"
    Then UI element "contract-renewal-result" contains "Renewed"
    And UI element "contract-renewal-source-reference" contains the DID of contract "Lifecycle Browser Contract"
