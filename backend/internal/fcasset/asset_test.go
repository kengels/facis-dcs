package fcasset

import (
	"testing"
	"time"
)

func TestBuildPayloadPreservesTemplateTypeAndCommonRDFType(t *testing.T) {
	for _, templateType := range []string{"COMPONENT", "CONTRACT_TEMPLATE"} {
		t.Run(templateType, func(t *testing.T) {
			payload, err := BuildPayload(BuildInput{
				Issuer:    "did:web:publisher.example",
				ValidFrom: time.Date(2026, time.July, 16, 12, 0, 0, 0, time.UTC),
				Subject: CatalogueSubjectFromRepository(
					"did:web:templates.example:template-1",
					1,
					"REGISTERED",
					templateType,
					"Reusable terms",
					"Reusable component",
				),
			})
			if err != nil {
				t.Fatalf("BuildPayload() error = %v", err)
			}

			types, ok := payload["type"].([]string)
			if !ok || len(types) != 2 || types[1] != "dcs:ContractTemplate" {
				t.Fatalf("payload type = %#v, want common dcs:ContractTemplate RDF type", payload["type"])
			}

			subject, ok := payload["credentialSubject"].(map[string]any)
			if !ok {
				t.Fatalf("credentialSubject = %#v, want map", payload["credentialSubject"])
			}
			if got := subject["type"]; got != "dcs:ContractTemplate" {
				t.Errorf("credentialSubject type = %#v, want dcs:ContractTemplate", got)
			}
			if got := subject["dcs:templateType"]; got != templateType {
				t.Errorf("dcs:templateType = %#v, want %q", got, templateType)
			}
		})
	}
}
