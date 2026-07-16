package event

import (
	"testing"
	"time"
)

func TestRecordEvidenceEventCarriesTypedReference(t *testing.T) {
	event := RecordEvidenceEvent{
		DID:          "did:web:example:contract",
		EvidenceType: "supporting-document",
		Reference:    "urn:evidence:document-1",
		OccurredAt:   time.Now().UTC(),
	}
	if event.GetDID() != event.DID {
		t.Fatalf("GetDID() = %q, want %q", event.GetDID(), event.DID)
	}
	if event.EvidenceType == "" || event.Reference == "" {
		t.Fatal("typed evidence fields must be retained by the audit event")
	}
}
