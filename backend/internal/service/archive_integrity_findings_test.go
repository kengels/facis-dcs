package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/contractworkflowengine/db"
)

func TestArchiveIntegrityRuleForError(t *testing.T) {
	tests := map[string]string{
		"archive retention deadline violated":          "ARCHIVE_RETENTION_POLICY",
		"archive signature_metadata is missing":        "ARCHIVE_SIGNATURE_EVIDENCE",
		"archive metadata is incomplete: jurisdiction": "ARCHIVE_METADATA_COMPLETENESS",
		"content_hash mismatch":                        "ARCHIVE_CONTENT_HASH",
		"fetch archive snapshot from IPFS":             "ARCHIVE_IPFS_SNAPSHOT",
		"stored notary receipt missing":                "ARCHIVE_ORCE_RECEIPT",
		"ORCE previousHash invalid":                    "ARCHIVE_ORCE_CHAIN",
		"archive TSA receipt verification failed":      "ARCHIVE_TSA_RFC3161",
		"contract_snapshot is empty":                   "ARCHIVE_DB_SNAPSHOT",
	}
	for message, want := range tests {
		if got := archiveIntegrityRuleForError(errors.New(message)); got != want {
			t.Errorf("%q: got %s, want %s", message, got, want)
		}
	}
}

func TestArchiveIntegrityFindingsNeverPassUnevaluatedChecks(t *testing.T) {
	invalidSnapshot := datatype.JSON(`not-json`)
	entry := db.ContractArchiveEntry{DID: "did:web:damaged", ContractVersion: 1, ContractSnapshot: invalidSnapshot, ContentHash: "broken"}
	service := &processAuditAndCompliancesrvc{}
	findings := service.archiveIntegrityTrailEntries(context.Background(), entry, 0, nil, nil, errors.New("ORCE chain unavailable"))
	if len(findings) != len(archiveIntegrityRules) {
		t.Fatalf("got %d findings", len(findings))
	}
	failed := map[string]bool{}
	for _, finding := range findings {
		if finding.Reason == nil || *finding.Reason == "" {
			t.Fatalf("finding has no reason: %+v", finding)
		}
		if finding.RuleID != nil && finding.Result != nil && *finding.Result == "FAILED" {
			failed[*finding.RuleID] = true
		}
	}
	for _, rule := range []string{"ARCHIVE_DB_SNAPSHOT", "ARCHIVE_METADATA_COMPLETENESS", "ARCHIVE_SIGNATURE_EVIDENCE", "ARCHIVE_CONTENT_HASH", "ARCHIVE_IPFS_SNAPSHOT", "ARCHIVE_ORCE_RECEIPT", "ARCHIVE_ORCE_CHAIN", "ARCHIVE_TSA_RFC3161"} {
		if !failed[rule] {
			t.Errorf("expected failed finding for %s", rule)
		}
	}
}

func TestVerifyArchiveMetadataRequiresSearchFacets(t *testing.T) {
	parties := datatype.JSON(`[]`)
	contractType := "ServiceAgreement"
	jurisdiction := "DE"
	entry := db.ContractArchiveEntry{
		DID: "did:web:example:contract", ContractVersion: 1, StoredBy: "did:web:example:archiver",
		StoredAt: time.Now(), ArchiveStatus: "STORED", ComplianceStatus: "COMPLIANT",
		Parties: &parties, ContractType: &contractType, Jurisdiction: &jurisdiction,
	}
	if err := verifyArchiveMetadata(entry); err != nil {
		t.Fatalf("complete metadata rejected: %v", err)
	}
	entry.Jurisdiction = nil
	if err := verifyArchiveMetadata(entry); err == nil {
		t.Fatal("missing jurisdiction was accepted")
	}
}

func TestArchiveRetentionPolicyDetectsEarlyDeletion(t *testing.T) {
	now := time.Now().UTC()
	deadline := now.Add(24 * time.Hour)
	entry := db.ContractArchiveEntry{DeletedAt: &now, RetentionUntil: &deadline}
	service := &processAuditAndCompliancesrvc{}
	checks := service.evaluateArchiveIntegrityChecks(context.Background(), entry, nil, nil, errors.New("ORCE unavailable"))
	if checks["ARCHIVE_RETENTION_POLICY"] == nil {
		t.Fatal("early deletion was not detected")
	}
}
