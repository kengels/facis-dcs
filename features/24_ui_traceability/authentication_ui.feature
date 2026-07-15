@ui @UC-01
Feature: Browser authentication evidence
  The product UI authenticates users through Hydra and the existing OID4VP
  test wallet before exposing role-bound application capabilities.

  @REQ-ui-traceability-playwright-evidence-AC3 @DCS-IR-SI-08 @DCS-NFR-BR-01 @DCS-FR-TR-06 @DCS-FR-CWE-07
  Scenario: A wallet presentation creates an authenticated role-bound browser session
    Given I have a fresh unauthenticated Playwright browser session
    When I open the DCS login page in the browser
    Then UI element "auth-oid4vp-presentation" is visible
    When I present the test wallet credential for role "Template Creator" from the browser login challenge
    Then UI element "app-authenticated-shell" is visible
    And UI element "app-authenticated-role" contains "Template Creator"
    And UI element "template-create-navigation" is visible
    And the browser session was not established by token or storage injection
