@ui @UC-04
Feature: Signature Management browser evidence
  Signers and compliance users operate on declared signing tasks and inspect
  validation evidence through role-protected product views.

  @clean_db @REQ-ui-traceability-playwright-evidence-AC11 @DCS-FR-CWE-19 @DCS-FR-CWE-26 @DCS-FR-SM-09 @DCS-FR-SM-13 @DCS-FR-SM-14 @DCS-FR-SM-22 @DCS-FR-SM-23 @DCS-FR-SM-24 @DCS-IR-SM-01 @DCS-IR-SM-03
  Scenario: A signer completes a declared task from the immutable signing dashboard
    Given contract "Signing Browser Contract" is a fresh draft declaring signature fields "SignerOne" and "SignerTwo"
    And contract "Signing Browser Contract" is submitted, reviewed, and approved via the standard workflow
    And I am signed in through the DCS login page and test wallet as "Contract Signer"
    When I open UI route "/signing"
    Then contract UI element "signing-task-row" for "Signing Browser Contract" is visible
    And keyed UI element "signing-declared-field" with key "SignerOne" is visible
    And keyed UI element "signing-declared-field" with key "SignerTwo" is visible
    When I click contract UI control "signing-task-open" for "Signing Browser Contract"
    Then UI element "signing-contract-viewer" is visible
    And UI element "signing-contract-editor" is absent
    When I click keyed UI control "signing-ceremony-start" with key "SignerOne"
    Then UI element "signing-oid4vp-presentation" is visible
    When I present the test wallet credential for role "Contract Signer" from UI element "signing-oid4vp-presentation"
    And I click keyed UI control "signing-apply-signature" with key "SignerOne"
    Then keyed UI element "signing-task-status" with key "SignerOne" contains "Signed"
    And keyed UI element "signing-completion-timestamp" with key "SignerOne" is visible

  @clean_db @REQ-ui-traceability-playwright-evidence-AC12 @DCS-IR-SM-02 @DCS-IR-SM-04 @DCS-FR-SM-09
  Scenario: Integrity, envelope, and applied-signature validation results are visible in the secure viewer
    Given contract "Validation Browser Contract" has reached contract state "SIGNED"
    And I am signed in through the DCS login page and test wallet as "Contract Manager"
    When I open the signing UI for contract "Validation Browser Contract"
    And I click UI control "signature-integrity-verify"
    Then UI element "signature-integrity-result" contains "valid"
    And UI element "signature-envelope-result" contains "valid"
    When I click UI control "signature-applied-validate"
    Then UI element "signature-validation-result" contains "valid"
    And UI element "signature-validation-errors" is absent

  @clean_db @REQ-ui-traceability-playwright-evidence-AC13 @DCS-IR-SM-05 @DCS-IR-SM-06 @DCS-IR-SM-07 @DCS-IR-SM-08 @DCS-FR-SM-26
  Scenario: Compliance users inspect, check, revoke, and report a signature while other roles remain read-only
    Given contract "Compliance Browser Contract" has reached contract state "SIGNED"
    And I am signed in through the DCS login page and test wallet as "Compliance Officer"
    When I open the compliance UI for contract "Compliance Browser Contract"
    Then UI element "signature-compliance-trust-anchor" is visible
    And UI element "signature-compliance-proof" is visible
    And UI element "signature-compliance-timestamp" is visible
    When I click UI control "signature-compliance-run"
    Then UI element "signature-compliance-result" contains "compliant"
    Then a browser download from UI control "signature-compliance-report" is produced
    When I fill UI control "signature-revocation-reason" with "Credential revoked for browser evidence"
    And I click UI control "signature-revoke"
    Then UI element "signature-status" contains "Revoked"
    When I sign out through the UI
    And I sign in through the DCS login page and test wallet as "Contract Observer"
    And I open the compliance UI for contract "Compliance Browser Contract"
    Then UI element "signature-compliance-result" is visible
    And UI element "signature-revoke" is absent
