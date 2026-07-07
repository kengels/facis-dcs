package command

import (
	"encoding/json"
	"testing"
	"time"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/db"
)

func TestBuildArchiveEntryIncludesSnapshotProvenance(t *testing.T) {
	contractData, err := datatype.NewJSON(map[string]any{"dcs:metadata": map[string]any{"dcs:title": "Service Contract"}})
	if err != nil {
		t.Fatalf("create contract data JSON: %v", err)
	}
	createdAt := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	name := "Service Contract"

	entry, err := BuildArchiveEntry(&db.Contract{
		DID:             "did:dcs:contract:1",
		Origin:          "did:dcs:origin",
		ContractVersion: 3,
		State:           contractstate.Approved.String(),
		Name:            &name,
		CreatedBy:       "did:participant:creator",
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		TemplateDID:     "did:dcs:template:base",
		TemplateVersion: 7,
		ContractData:    &contractData,
	}, "did:participant:approver")
	if err != nil {
		t.Fatalf("build archive entry: %v", err)
	}

	var snapshot map[string]any
	if err := json.Unmarshal(entry.ContractSnapshot, &snapshot); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}

	if got := snapshot["origin"]; got != "did:dcs:origin" {
		t.Fatalf("origin = %v, want did:dcs:origin", got)
	}
	if got := snapshot["template_did"]; got != "did:dcs:template:base" {
		t.Fatalf("template_did = %v, want did:dcs:template:base", got)
	}
	if got := int(snapshot["template_version"].(float64)); got != 7 {
		t.Fatalf("template_version = %v, want 7", got)
	}
	if entry.ContentHash == "" {
		t.Fatal("content hash must be persisted")
	}
}

func TestBuildArchiveEntryRejectsUnapprovedContract(t *testing.T) {
	_, err := BuildArchiveEntry(&db.Contract{
		DID:             "did:dcs:contract:1",
		ContractVersion: 1,
		State:           contractstate.Draft.String(),
	}, "did:participant:approver")
	if err == nil {
		t.Fatal("expected unapproved contracts to be rejected")
	}
}
