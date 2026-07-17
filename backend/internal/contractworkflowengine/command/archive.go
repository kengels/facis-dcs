package command

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/db"
)

const archiveSnapshotHashAlgorithm = "SHA-256"

type ArchiveSigningEvidence struct {
	Signer, CredentialType, CeremonyID, Field, PDFCID, PDFHash string
	SignedAt                                                   time.Time
	CredentialHashes                                           map[string]string
}

type ArchiveComponent struct {
	IRI         string
	PartyIDs    datatype.JSON
	Snapshot    datatype.JSON
	ContentHash string
}

// BuildArchiveEntry freezes the signed contract state for archive
// persistence (DCS-FR-CWE-20): the archive entry is created once the
// signature workflow completes (SIGNED), not at APPROVED.
func BuildArchiveEntry(contract *db.Contract, storedBy string, signing ArchiveSigningEvidence) (db.ContractArchiveEntry, error) {
	if contract == nil {
		return db.ContractArchiveEntry{}, fmt.Errorf("contract is required")
	}
	if contract.State != contractstate.Signed.String() {
		return db.ContractArchiveEntry{}, fmt.Errorf("contract %s must be signed before archive storage", contract.DID)
	}

	snapshotJSON, err := buildContractSnapshot(contract)
	if err != nil {
		return db.ContractArchiveEntry{}, err
	}
	contentHash, err := HashArchiveSnapshot(snapshotJSON)
	if err != nil {
		return db.ContractArchiveEntry{}, err
	}

	signatureMetadata, err := datatype.NewJSON(map[string]any{
		"status": "SIGNED", "signer": signing.Signer, "credential_type": signing.CredentialType,
		"ceremony_id": signing.CeremonyID, "field": signing.Field, "signed_at": signing.SignedAt.UTC().Format(time.RFC3339Nano),
		"pdf_cid": signing.PDFCID, "pdf_hash": "sha256:" + strings.TrimPrefix(signing.PDFHash, "sha256:"),
	})
	if err != nil {
		return db.ContractArchiveEntry{}, err
	}
	credentialHashes, err := datatype.NewJSON(signing.CredentialHashes)
	if err != nil {
		return db.ContractArchiveEntry{}, err
	}
	evidence, err := datatype.NewJSON(map[string]any{
		"source":                  "SIGNING_WORKFLOW_COMPLETION",
		"stored_by":               storedBy,
		"stored_state":            contractstate.Signed.String(),
		"snapshot_hash_algorithm": archiveSnapshotHashAlgorithm,
	})
	if err != nil {
		return db.ContractArchiveEntry{}, err
	}
	index, err := archiveIndex(contract.ContractData)
	if err != nil {
		return db.ContractArchiveEntry{}, err
	}

	return db.ContractArchiveEntry{
		DID:               contract.DID,
		ContractVersion:   contract.ContractVersion,
		StoredBy:          storedBy,
		StoredAt:          time.Now().UTC(),
		ContractSnapshot:  snapshotJSON,
		ContentHash:       contentHash,
		SignatureMeta:     &signatureMetadata,
		CredentialHashes:  &credentialHashes,
		Evidence:          &evidence,
		ParentContractDID: index.parentDID,
		Parties:           index.parties,
		ContractType:      index.contractType,
		Jurisdiction:      index.jurisdiction,
		ComplianceStatus:  "COMPLIANT",
	}, nil
}

type archiveIndexValues struct {
	parentDID, contractType, jurisdiction *string
	parties                               *datatype.JSON
}

// archiveIndex derives searchable facets from the frozen machine-readable
// contract. It reads the existing JSON-LD vocabulary instead of introducing
// archive-only business values.
func archiveIndex(raw *datatype.JSON) (archiveIndexValues, error) {
	empty, err := datatype.NewJSON([]any{})
	if err != nil {
		return archiveIndexValues{}, err
	}
	result := archiveIndexValues{parties: &empty}
	if raw == nil || !raw.IsNotNullValue() {
		return result, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(*raw, &doc); err != nil {
		return result, fmt.Errorf("decode contract JSON-LD for archive index: %w", err)
	}
	if id := jsonLDID(doc["dcs:parentContract"]); id != "" {
		result.parentDID = &id
	}
	if parties, ok := doc["dcs:parties"]; ok {
		encoded, err := datatype.NewJSON(parties)
		if err != nil {
			return result, fmt.Errorf("encode archive parties: %w", err)
		}
		result.parties = &encoded
	}
	if value := stringJSONLDValue(doc["dcs:contractType"]); value != "" {
		result.contractType = &value
	} else if value := stringJSONLDValue(doc["@type"]); value != "" {
		result.contractType = &value
	}
	if value := findSemanticParameterValue(doc, "contract.jurisdiction"); value != "" {
		result.jurisdiction = &value
	}
	return result, nil
}

func jsonLDID(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		id, _ := typed["@id"].(string)
		return id
	case []any:
		if len(typed) > 0 {
			return jsonLDID(typed[0])
		}
	}
	return ""
}

func stringJSONLDValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		if len(typed) > 0 {
			return stringJSONLDValue(typed[0])
		}
	case map[string]any:
		if text, _ := typed["@value"].(string); text != "" {
			return text
		}
		if id, _ := typed["@id"].(string); id != "" {
			return id
		}
	}
	return ""
}

func findSemanticParameterValue(value any, parameter string) string {
	switch typed := value.(type) {
	case map[string]any:
		if name, _ := typed["dcs:parameterName"].(string); name == parameter {
			return stringJSONLDValue(typed["dcs:parameterValue"])
		}
		for _, nested := range typed {
			if found := findSemanticParameterValue(nested, parameter); found != "" {
				return found
			}
		}
	case []any:
		for _, nested := range typed {
			if found := findSemanticParameterValue(nested, parameter); found != "" {
				return found
			}
		}
	}
	return ""
}

func buildContractSnapshot(contract *db.Contract) (datatype.JSON, error) {
	contractData := json.RawMessage(`{}`)
	if contract.ContractData != nil && contract.ContractData.IsNotNullValue() {
		contractData = json.RawMessage(*contract.ContractData)
	}

	snapshot := map[string]any{
		"did":               contract.DID,
		"contract_version":  contract.ContractVersion,
		"state":             contract.State,
		"name":              stringPtrValue(contract.Name),
		"description":       stringPtrValue(contract.Description),
		"created_by":        contract.CreatedBy,
		"created_at":        formatArchiveTime(&contract.CreatedAt),
		"updated_at":        formatArchiveTime(&contract.UpdatedAt),
		"start_date":        formatArchiveTime(contract.StartDate),
		"exp_date":          formatArchiveTime(contract.ExpDate),
		"exp_policy":        stringPtrValue(contract.ExpPolicy),
		"exp_notice_period": intPtrValue(contract.ExpNoticePeriod),
		"responsible":       contract.Responsible,
		"contract_data":     contractData,
	}

	return datatype.NewJSON(snapshot)
}

func HashArchiveSnapshot(snapshot datatype.JSON) (string, error) {
	canonicalSnapshot, err := CanonicalizeArchiveSnapshot(snapshot)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonicalSnapshot)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// BuildArchiveComponents freezes the ordered dcs:sections members separately
// so a multi-party archive can enforce component-level visibility without
// losing the linkage to the complete contract snapshot.
func BuildArchiveComponents(raw *datatype.JSON) ([]ArchiveComponent, error) {
	if raw == nil || !raw.IsNotNullValue() {
		return []ArchiveComponent{}, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(*raw, &doc); err != nil {
		return nil, fmt.Errorf("decode contract JSON-LD for archive components: %w", err)
	}
	sections := jsonLDList(doc["dcs:sections"])
	components := make([]ArchiveComponent, 0, len(sections))
	for index, rawSection := range sections {
		section, ok := rawSection.(map[string]any)
		if !ok {
			continue
		}
		iri := jsonLDID(section)
		if iri == "" {
			iri = fmt.Sprintf("%s#section-%d", stringJSONLDValue(doc["@id"]), index+1)
		}
		snapshot, err := datatype.NewJSON(section)
		if err != nil {
			return nil, fmt.Errorf("encode archive component %s: %w", iri, err)
		}
		hash, err := HashArchiveSnapshot(snapshot)
		if err != nil {
			return nil, fmt.Errorf("hash archive component %s: %w", iri, err)
		}
		partyIDs, err := datatype.NewJSON(collectPartyIDs(section))
		if err != nil {
			return nil, fmt.Errorf("encode archive component parties %s: %w", iri, err)
		}
		components = append(components, ArchiveComponent{IRI: iri, PartyIDs: partyIDs, Snapshot: snapshot, ContentHash: hash})
	}
	return components, nil
}

func jsonLDList(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case map[string]any:
		if list, ok := typed["@list"].([]any); ok {
			return list
		}
	}
	return []any{}
}

func collectPartyIDs(value any) []string {
	ids := map[string]struct{}{}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, nested := range typed {
				switch key {
				case "dcs:assignedParty", "dcs:party", "odrl:assigner", "odrl:assignee":
					if id := jsonLDID(nested); id != "" {
						ids[id] = struct{}{}
					}
				}
				walk(nested)
			}
		case []any:
			for _, nested := range typed {
				walk(nested)
			}
		}
	}
	walk(value)
	result := make([]string, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	slices.Sort(result)
	return result
}

func CanonicalizeArchiveSnapshot(snapshot datatype.JSON) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(snapshot))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode archive snapshot JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode archive snapshot JSON: multiple JSON values")
		}
		return nil, fmt.Errorf("decode archive snapshot JSON: %w", err)
	}

	canonicalSnapshot, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("canonicalize archive snapshot JSON: %w", err)
	}
	return canonicalSnapshot, nil
}

func formatArchiveTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func intPtrValue(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
