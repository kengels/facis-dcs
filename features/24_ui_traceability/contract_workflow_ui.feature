@ui @UC-03 @UC-06
Feature: Contract Workflow browser evidence
  Contract creation, governance, monitoring, and lifecycle actions are
  performed and verified through the delivered product UI.

  @clean_db @REQ-ui-traceability-playwright-evidence-AC7 @REQ-ui-gap-resolution-AC7 @DCS-IR-CWE-01 @DCS-IR-CWE-02 @DCS-FR-CWE-03 @DCS-FR-CWE-13 @DCS-FR-CWE-14
  Scenario: A creator persists a populated contract draft and submits it
    Given template "Contract Browser Template" is approved and available
    And I am signed in through the DCS login page and test wallet as "Contract Creator"
    When I open UI route "/contracts/new"
    And I select template "Contract Browser Template" in UI control "contract-create-template"
    And I fill these UI controls:
      | test_id                 | value                         |
      | contract-create-name    | Browser Evidence Contract     |
      | contract-create-party   | did:web:buyer.example         |
      | contract-create-asset   | urn:asset:browser-evidence    |
      | contract-create-policy  | dcs:ServiceLevelAgreement     |
      | contract-create-evidence | urn:evidence:browser-evidence |
    And I click UI control "contract-create-save-draft"
    Then UI element "contract-save-result" contains "Draft"
    When I open the contract from UI element "contract-save-result"
    And I reload the current UI route
    Then UI element "contract-party-list" contains "did:web:buyer.example"
    And UI element "contract-asset-list" contains "urn:asset:browser-evidence"
    And UI element "contract-policy-list" contains "dcs:ServiceLevelAgreement"
    And UI element "contract-evidence-list" contains "urn:evidence:browser-evidence"
    When I click UI control "contract-submit-review"
    And I fill these UI controls:
      | test_id                         | value                        |
      | contract-participants-reviewer  | did:web:reviewer.example     |
      | contract-participants-approver  | did:web:approver.example     |
    And I click UI control "contract-participants-submit"
    Then UI element "contract-lifecycle-status" contains "Submitted"
    And UI element "contract-template-reference" contains the DID of template "Contract Browser Template"

  @clean_db @REQ-ui-traceability-playwright-evidence-AC8 @REQ-ui-gap-resolution-AC8 @DCS-IR-CWE-03 @DCS-IR-CWE-04 @DCS-FR-CWE-17 @DCS-FR-CWE-18
  Scenario: Negotiation comments and redlines persist with their version diff
    Given contract "Negotiation Browser Contract" is open for negotiation
    And I am signed in through the DCS login page and test wallet as "Contract Negotiator"
    When I open the negotiation UI for contract "Negotiation Browser Contract"
    And I fill UI control "contract-negotiation-comment" with "Clarify the browser delivery term"
    And I fill UI control "contract-negotiation-redline" with "Delivery within five browser days"
    And I click UI control "contract-negotiation-submit"
    Then UI element "contract-negotiation-thread" contains "Clarify the browser delivery term"
    When I reload the current UI route
    Then UI element "contract-negotiation-thread" contains "Clarify the browser delivery term"
    And UI element "contract-version-diff" contains "Delivery within five browser days"
    And UI element "contract-version-history" is visible

  @clean_db @REQ-ui-gap-resolution-AC9 @DCS-IR-CWE-05 @DCS-IR-CWE-06 @DCS-IR-CWE-07 @DCS-IR-CWE-08 @DCS-IR-CWE-09 @DCS-IR-CWE-10 @DCS-FR-CWE-15 @DCS-FR-CWE-25
  Scenario: Review findings and reasoned approval decisions govern the contract
    Given contract "Governed Browser Contract" is in "Under Review" status
    And I am signed in through the DCS login page and test wallet as "Contract Reviewer"
    When I open the review UI for contract "Governed Browser Contract"
    And I click UI control "contract-review-request-change"
    Then UI element "contract-review-finding" is visible
    When I fill UI control "contract-review-finding" with "Terms require browser correction"
    And I click UI control "contract-review-finding-submit"
    And I reopen the review UI for contract "Governed Browser Contract"
    Then UI element "contract-review-history" contains "Terms require browser correction"
    When I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Contract Creator"
    And I open the negotiation UI for contract "Governed Browser Contract"
    And I click UI control "contract-submit-review"
    And I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Contract Reviewer"
    And I reopen the review UI for contract "Governed Browser Contract"
    And I click UI control "contract-review-verify"
    And I click UI control "contract-review-forward-approval"
    Then UI element "contract-review-finding" is visible
    When I fill UI control "contract-review-finding" with "Terms verified in browser"
    And I click UI control "contract-review-finding-submit"
    And I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Contract Approver"
    And I open the approval UI for contract "Governed Browser Contract"
    And I click UI control "contract-approval-reject"
    Then UI element "contract-approval-rejection-reason" is visible
    When I fill UI control "contract-approval-rejection-reason" with "Approval evidence is incomplete"
    And I click UI control "contract-approval-decision-submit"
    And I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Contract Creator"
    And I open the edit UI for contract "Governed Browser Contract"
    And I click UI control "contract-submit-review"
    Then current UI route is "/contracts"
    When I open the negotiation UI for contract "Governed Browser Contract"
    And I click UI control "contract-submit-review"
    And I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Contract Reviewer"
    And I complete contract review for "Governed Browser Contract" through the browser
    And I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Contract Approver"
    And I open the approval UI for contract "Governed Browser Contract"
    And I click UI control "contract-approval-approve"
    And I click UI control "contract-approval-decision-submit"
    And I open the management UI for contract "Governed Browser Contract"
    Then UI element "contract-lifecycle-status" contains "Approved"

  @clean_db @REQ-ui-traceability-playwright-evidence-AC9 @REQ-ui-gap-resolution-AC10 @DCS-IR-CWE-07 @DCS-IR-CWE-11 @DCS-FR-CWE-24 @DCS-FR-CWE-27 @DCS-FR-CWE-29
  Scenario: The contract dashboard links filtered results to complete lifecycle details
    Given named contract "Dashboard Browser Contract" has reached contract state "DRAFT"
    And I am signed in through the DCS login page and test wallet as "Contract Manager"
    When I open UI route "/contracts"
    And I click UI control "contract-search-filter-open"
    And I click keyed UI control "contract-search-filter-option" with key "name"
    And I fill UI control "contract-search-query" with "Dashboard Browser Contract"
    And I click UI control "contract-state-filter"
    And I click keyed UI control "contract-state-filter-option" with key "DRAFT"
    And I click UI control "contract-search-submit"
    Then contract UI element "contract-dashboard-row" for "Dashboard Browser Contract" is visible
    When I open the management UI for contract "Dashboard Browser Contract"
    Then UI element "contract-dashboard-lifecycle" contains "Draft"
    And UI element "contract-dashboard-responsibility" is visible
    And UI element "contract-dashboard-deadline" is visible
    And UI element "contract-dashboard-history" is visible
    And UI element "contract-dashboard-hierarchy" is visible
    And UI element "contract-dashboard-dependencies" is visible

  @clean_db @REQ-ui-gap-resolution-AC11 @DCS-IR-CWE-13 @DCS-FR-CWE-31
  Scenario: Target-reported KPIs, milestones, and violations are visible with timestamps
    Given contract "KPI Browser Contract" is a fresh draft whose ODRL policy constrains field "coverage" using operator "gteq" against "95" while the actual value is "95"
    And contract "KPI Browser Contract" is submitted, reviewed, approved, and signed via the standard workflow
    And an authorized user deploys contract "KPI Browser Contract" to the configured contract target
    And I am signed in through the DCS login page and test wallet as "Contract Manager"
    When the target reports a KPI value "coverage" = "80" for contract "KPI Browser Contract"
    And I open the management UI for contract "KPI Browser Contract"
    Then keyed UI element "contract-kpi-value" with key "coverage" contains "80"
    And keyed UI element "contract-kpi-milestone" with key "coverage" is visible
    And keyed UI element "contract-kpi-timestamp" with key "coverage" is visible
    And keyed UI element "contract-kpi-violation" with key "coverage" is visible
    And keyed UI element "contract-kpi-alert" with key "coverage" is visible

  @clean_db @REQ-ui-traceability-playwright-evidence-AC10 @REQ-ui-gap-resolution-AC12 @DCS-IR-CWE-12 @DCS-FR-CWE-12 @DCS-FR-CWE-23
  Scenario: Typed evidence and a reasoned termination remain auditable
    Given contract "Lifecycle Browser Contract" is in "Draft" status
    And I am signed in through the DCS login page and test wallet as "Contract Manager"
    When I open the management UI for contract "Lifecycle Browser Contract"
    And I fill UI control "contract-evidence-type" with "supporting-document"
    And I fill UI control "contract-evidence-reference" with "urn:evidence:lifecycle-browser"
    And I click UI control "contract-evidence-store"
    Then keyed UI element "contract-evidence-result" with key "urn:evidence:lifecycle-browser" contains "supporting-document"
    When I click UI control "contract-manager-terminate"
    Then UI element "contract-termination-reason" is visible
    When I fill UI control "contract-termination-reason" with "Terminated by browser acceptance"
    And I click UI control "contract-termination-submit"
    Then current UI route is "/contracts"
    When I open UI route "/audit"
    And I select UI option "contracts" in control "audit-scope-filter"
    And I fill UI control "audit-did-filter" with the DID of contract "Lifecycle Browser Contract"
    And I fill UI control "audit-justification" with "Termination lifecycle browser evidence"
    And I click UI control "audit-run"
    And I click UI control "audit-tab-timeline"
    Then contract UI element "audit-result" for "Lifecycle Browser Contract" contains an authoritative timestamp
    When I click keyed UI control "audit-event" with key "TERMINATE_CONTRACT"
    Then UI element "audit-selected-details" contains "Terminated by browser acceptance"
    And UI element "audit-selected-details" contains "Contract Manager"

  @clean_db @REQ-ui-gap-resolution-AC13 @DCS-FR-CWE-11 @DCS-FR-CWE-22 @DCS-FR-CSA-23
  Scenario: A manager creates exactly one renewal linked to its original contract
    Given contract "Renewal Browser Contract" with a contract term has reached contract state "SIGNED"
    And I am signed in through the DCS login page and test wallet as "Contract Manager"
    When I open the management UI for contract "Renewal Browser Contract"
    And I click UI control "contract-manager-renew"
    Then exactly 1 UI element "contract-renewal-result" is visible
    And UI element "contract-renewal-source-reference" contains the DID of contract "Renewal Browser Contract"
    When I reload the current UI route
    Then exactly 1 UI element "contract-renewal-result" is visible
