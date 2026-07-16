@ui
Feature: UI acceptance evidence policy
  A capability is promoted from a UI gap only when its browser scenario uses
  isolated Playwright execution, semantic selectors, and durable evidence.

  @REQ-ui-gap-resolution-AC23
  Scenario: UI gap resolution is guarded by stable browser evidence
    Given the UI gap resolution acceptance features are loaded
    Then every UI gap resolution AC from 1 through 22 is tagged for UI execution
    And the current UI scenario has an isolated Playwright browser context
    And UI selectors are restricted to semantic test IDs and business keys
    And UI failure evidence is configured for trace, screenshot, video, and isolated JUnit
