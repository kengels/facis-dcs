package query

import (
	"testing"
	"time"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/signingmanagement/datatype/signingstatus"
	"digital-contracting-service/internal/signingmanagement/db"
)

func TestDeriveSigningTaskItemsUsesDeclarationsAndExistingFieldStatus(t *testing.T) {
	contractData := datatype.JSON(`{"signatureFields":[{"signatoryName":"First"},{"signatoryName":"Second"}]}`)
	deadline := time.Date(2027, time.January, 2, 3, 4, 5, 0, time.UTC)
	signedAt := time.Date(2026, time.July, 15, 10, 30, 0, 0, time.UTC)
	second := "Second"
	items, err := deriveSigningTaskItems(db.ContractMetadata{
		DID: "did:web:example.test:contract", ContractVersion: 4,
		ContractData: &contractData, ExpDate: &deadline,
	}, []db.SignatureRecord{{FieldName: &second, SignerDID: "did:web:signer.example", Status: "SIGNED", SignedAt: &signedAt}})
	if err != nil {
		t.Fatalf("deriveSigningTaskItems returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("task count = %d, want 2", len(items))
	}
	if items[0].FieldName != "First" || items[0].Order != 1 || items[0].Dependency != nil || items[0].State != signingstatus.Pending {
		t.Fatalf("unexpected first task: %#v", items[0])
	}
	if items[1].FieldName != "Second" || items[1].Order != 2 || items[1].Dependency == nil || *items[1].Dependency != "First" {
		t.Fatalf("unexpected second task ordering: %#v", items[1])
	}
	if items[1].State != signingstatus.Signed || items[1].SignerDID != "did:web:signer.example" || items[1].Deadline != &deadline || items[1].SignedAt != &signedAt {
		t.Fatalf("existing signature/deadline not merged: %#v", items[1])
	}
}
