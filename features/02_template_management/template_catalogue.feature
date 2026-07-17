# Template catalogue integration (DCS-IR-SI-01, UC-02): POST /template/publish
# (backend/design/template_repository.go) pushes a REGISTERED template to the
# XFSC Federated Catalogue; GET /catalogue/template/retrieve and
# /catalogue/template/search (backend/design/template_catalogue_integration.go)
# read it back. The Federated Catalogue integration itself is already
# exercised indirectly (register-only path) by template_workflow.feature's
# "Register approved template" scenario; this file adds the publish/retrieve/
# search round-trip and its own RBAC scope, which that file does not cover.

@DCS-IR-SI-01 @DCS-NFR-BR-09 @UC-02
Feature: Template catalogue integration

  @clean_db @REQ-component-template-lifecycle-AC2 @DCS-FR-TR-08 @DCS-IR-TR-07 @DCS-IR-SI-01 @UC-02
  Scenario: Template Manager publishes a registered component template to the catalogue
    Given I am authenticated with roles: "Template Manager"
    And component template "Catalogue Publish Component" is available in "REGISTERED" status
    When I publish template "Catalogue Publish Component"
    Then get http 200:Success code
    And the template status is "Published"
    And template "Catalogue Publish Component" has template type "COMPONENT"

  @clean_db @REQ-component-template-lifecycle-AC3 @DCS-FR-TR-03 @DCS-IR-SI-01 @DCS-PC-06 @UC-02-02
  Scenario: A published component template can be retrieved via the catalogue
    Given I am authenticated with roles: "Template Manager"
    And component template "Catalogue Retrieve Component" is available in "REGISTERED" status
    And I publish template "Catalogue Retrieve Component"
    And I am authenticated with roles: "Contract Creator"
    When I retrieve the template catalogue
    Then get http 200:Success code
    And the catalogue result includes component template "Catalogue Retrieve Component"
    When I retrieve the catalogue detail for template "Catalogue Retrieve Component"
    Then the catalogue detail roundtrip returns component template "Catalogue Retrieve Component"

  @clean_db @REQ-component-template-lifecycle-AC3 @DCS-FR-TR-03 @DCS-IR-SI-01 @DCS-PC-06 @UC-02-02
  Scenario: A published component template can be found via catalogue search
    Given I am authenticated with roles: "Template Manager"
    And component template "Catalogue Search Component" is available in "REGISTERED" status
    And I publish template "Catalogue Search Component"
    And I am authenticated with roles: "Contract Creator"
    When I search the template catalogue by name "Catalogue Search Component"
    Then get http 200:Success code
    And the catalogue search result includes component template "Catalogue Search Component"

  @clean_db
  Scenario: A role outside the catalogue scope cannot publish a template
    Given I am authenticated with roles: "Template Manager"
    And template "Unauthorized Catalogue Publish Template" is in "Registered" status
    And I am authenticated with roles: "Template Creator"
    When I attempt to publish template "Unauthorized Catalogue Publish Template" with my current role
    Then the request is denied with a client error
