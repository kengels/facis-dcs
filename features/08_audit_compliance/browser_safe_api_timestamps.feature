@UC-08-01
Feature: Browser-safe API timestamps
  Audit and negotiation timestamps exposed at the API boundary use an
  unambiguous RFC3339 representation that browser clients can parse.

  @clean_db @REQ-browser-safe-api-timestamps-AC1 @DCS-FR-TR-08 @DCS-FR-TR-21 @DCS-IR-TR-07 @DCS-IR-TR-08 @UC-02-09
  Scenario: Template audit timestamps are browser-safe
    Given I am authenticated with roles: "Template Manager"
    And template "Timestamp Template" is in "Approved" status
    Then every template audit entry for "Timestamp Template" has an RFC3339 browser-safe created_at timestamp

  @clean_db @REQ-browser-safe-api-timestamps-AC2 @DCS-FR-CWE-27 @DCS-IR-CWE-12 @DCS-IR-CWE-13
  Scenario: Contract audit timestamps are browser-safe
    Given contract "Timestamp Audit Contract" is in "Draft" status
    Then every contract audit entry for "Timestamp Audit Contract" has an RFC3339 browser-safe created_at timestamp

  @clean_db @REQ-browser-safe-api-timestamps-AC3 @DCS-FR-SM-19 @DCS-IR-SM-08
  Scenario: Signature audit timestamps are browser-safe
    Given contract "Timestamp Signature Contract" has reached contract state "SIGNED"
    When the contract manager validates the signature for contract "Timestamp Signature Contract"
    Then get http 200:Success code
    And every signature audit entry for "Timestamp Signature Contract" has an RFC3339 browser-safe created_at timestamp

  @clean_db @REQ-browser-safe-api-timestamps-AC4 @DCS-FR-CWE-18 @DCS-FR-CWE-27 @UC-03-02
  Scenario: Retrieved negotiation timestamps are browser-safe
    Given I am authenticated with roles: "Contract Manager"
    And contract "Timestamp Negotiation Contract" has multiple negotiation edits
    When I retrieve contract "Timestamp Negotiation Contract" for timestamp verification
    Then every returned negotiation has an RFC3339 browser-safe created_at timestamp
