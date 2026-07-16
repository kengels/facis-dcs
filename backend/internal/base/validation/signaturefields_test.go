package validation

import (
	"reflect"
	"testing"
)

func TestRequiredSignatureFields(t *testing.T) {
	doc := []byte(`{
		"@type": "dcs:Contract",
		"signatureFields": [
			{"@type": "SignatureField", "@id": "urn:doc:x#SignerOne", "signatoryName": "SignerOne"},
			{"@type": "SignatureField", "@id": "urn:doc:x#SignerTwo", "signatoryName": "SignerTwo"},
			{"@type": "SignatureField", "@id": "urn:doc:x#dup", "signatoryName": "SignerOne"},
			{"@type": "SignatureField", "@id": "urn:doc:x#blank", "signatoryName": "  "}
		]
	}`)
	got := RequiredSignatureFields(doc)
	want := []string{"SignerOne", "SignerTwo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	if got := RequiredSignatureFields([]byte(`{"@type":"dcs:Contract"}`)); len(got) != 0 {
		t.Fatalf("expected no fields for a contract without signatureFields, got %v", got)
	}
	if got := RequiredSignatureFields([]byte(`not json`)); got != nil {
		t.Fatalf("expected nil for unparseable data, got %v", got)
	}
}

func TestDeclaredSignatureFieldsPreservesOrderAndDependencies(t *testing.T) {
	doc := []byte(`{"signatureFields":[{"signatoryName":"First"},{"signatoryName":"Second"},{"signatoryName":"Third"}]}`)
	got := DeclaredSignatureFields(doc)
	if len(got) != 3 {
		t.Fatalf("expected 3 declarations, got %d", len(got))
	}
	if got[0].Order != 1 || got[0].Dependency != nil {
		t.Fatalf("unexpected first declaration: %#v", got[0])
	}
	if got[1].Order != 2 || got[1].Dependency == nil || *got[1].Dependency != "First" {
		t.Fatalf("unexpected second declaration: %#v", got[1])
	}
	if got[2].Order != 3 || got[2].Dependency == nil || *got[2].Dependency != "Second" {
		t.Fatalf("unexpected third declaration: %#v", got[2])
	}
}
