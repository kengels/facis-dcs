package template

import (
	"strings"
	"testing"
)

func TestCatalogueQueriesProjectTemplateType(t *testing.T) {
	for name, statement := range map[string]string{
		"retrieve": retrieveTemplatesStatementTemplate,
		"detail":   retrieveTemplateByIDStatement,
		"search":   searchTemplatesStatementTemplate,
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(statement, "templateType: ct.templateType") {
				t.Fatalf("query does not project ct.templateType: %s", statement)
			}
		})
	}
}

func TestCatalogueMappersPreserveTemplateType(t *testing.T) {
	projection := map[string]interface{}{
		"did":          "did:web:templates.example:component-1",
		"version":      "2",
		"name":         "Reusable terms",
		"description":  "Reusable component",
		"templateType": "COMPONENT",
	}

	item := mapCatalogueItem(projection)
	if item == nil || item.TemplateType == nil || *item.TemplateType != "COMPONENT" {
		t.Fatalf("mapCatalogueItem() template type = %#v, want COMPONENT", item)
	}

	detail := mapCatalogueDetail(projection)
	if detail == nil || detail.TemplateType == nil || *detail.TemplateType != "COMPONENT" {
		t.Fatalf("mapCatalogueDetail() template type = %#v, want COMPONENT", detail)
	}
}
