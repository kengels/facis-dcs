@ui @UC-02
Feature: Browser-safe template lifecycle timestamps
  Template lifecycle evidence renders every available server timestamp as a
  date that a browser can understand.

  @clean_db @REQ-browser-safe-api-timestamps-AC5 @DCS-FR-TR-08 @DCS-FR-TR-21 @DCS-FR-TR-28 @DCS-IR-TR-07 @DCS-IR-TR-08 @UC-02-09
  Scenario: Lifecycle evidence renders every available timestamp as a date
    Given template "Lifecycle Timestamp Template" is in "Approved" status
    And I am signed in through the DCS login page and test wallet as "Template Manager"
    When I open the details UI for template "Lifecycle Timestamp Template"
    Then the template lifecycle evidence contains entries without an invalid date
