package catalogueasset

import (
	"testing"

	"digital-contracting-service/internal/templaterepository/db"
)

func TestBuildTemplatePayloadRejectsMissingOrInvalidTemplateType(t *testing.T) {
	for _, templateType := range []string{"", "UNKNOWN"} {
		t.Run(templateType, func(t *testing.T) {
			_, err := BuildTemplatePayload(
				"did:web:templates.example:template-1",
				"did:web:publisher.example",
				&db.ContractTemplateProcessData{Version: 1, State: "REGISTERED"},
				&db.ContractTemplate{TemplateType: templateType},
			)
			if err == nil {
				t.Fatalf("BuildTemplatePayload() accepted template type %q", templateType)
			}
		})
	}
}

func TestBuildTemplatePayloadNormalizesTemplateType(t *testing.T) {
	payload, err := BuildTemplatePayload(
		"did:web:templates.example:template-1",
		"did:web:publisher.example",
		&db.ContractTemplateProcessData{Version: 1, State: "REGISTERED"},
		&db.ContractTemplate{TemplateType: "component"},
	)
	if err != nil {
		t.Fatalf("BuildTemplatePayload() error = %v", err)
	}

	subject := payload["credentialSubject"].(map[string]any)
	if got := subject["dcs:templateType"]; got != "COMPONENT" {
		t.Fatalf("dcs:templateType = %#v, want COMPONENT", got)
	}
}
