package command

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/base/validation"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/db"

	"github.com/jmoiron/sqlx"
)

func TestPrepareInitialSubmitTasksSelectsDirectReviewOrNegotiation(t *testing.T) {
	tests := []struct {
		name        string
		negotiators []string
		wantState   contractstate.ContractState
		wantTasks   int
	}{
		{name: "direct review", wantState: contractstate.Submitted},
		{name: "negotiation", negotiators: []string{"did:web:peer.example:negotiator"}, wantState: contractstate.Negotiation, wantTasks: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reviews := &submitReviewTaskRepoFake{}
			approvals := &submitApprovalTaskRepoFake{}
			negotiations := &submitNegotiationTaskRepoFake{}
			submitter := Submitter{RTRepo: reviews, ATRepo: approvals, NTRepo: negotiations}
			processData := &db.ContractProcessData{
				DID:             "did:web:facis.example:contract:submit",
				Origin:          "did:web:origin.example",
				ContractVersion: 3,
			}

			responsible, state, err := submitter.prepareInitialSubmitTasks(context.Background(), nil, processData, SubmitCmd{
				DID:         processData.DID,
				SubmittedBy: "did:web:origin.example:user:creator",
				Reviewers:   []string{"did:web:peer.example:reviewer"},
				Approvers:   []string{"did:web:peer.example:approver"},
				Negotiators: tt.negotiators,
			})
			if err != nil {
				t.Fatalf("prepareInitialSubmitTasks returned error: %v", err)
			}
			if state != tt.wantState {
				t.Fatalf("target state = %s, want %s", state, tt.wantState)
			}
			if responsible.Creator != processData.Origin {
				t.Fatalf("responsible creator = %s, want %s", responsible.Creator, processData.Origin)
			}
			if len(reviews.created) != 1 || len(approvals.created) != 1 || len(negotiations.created) != tt.wantTasks {
				t.Fatalf("created task counts review/approval/negotiation = %d/%d/%d, want 1/1/%d", len(reviews.created), len(approvals.created), len(negotiations.created), tt.wantTasks)
			}
			if reviews.created[0].ContractVersion != processData.ContractVersion {
				t.Fatalf("review task contract version = %d, want %d", reviews.created[0].ContractVersion, processData.ContractVersion)
			}
		})
	}
}

func TestPrepareInitialSubmitTasksDoesNotDuplicateExistingAssignments(t *testing.T) {
	const did = "did:web:facis.example:contract:submit"
	reviews := &submitReviewTaskRepoFake{existing: []db.ReviewTaskData{{DID: did, Reviewer: "did:web:peer.example:reviewer"}}}
	approvals := &submitApprovalTaskRepoFake{existing: []db.ApprovalTaskData{{DID: did, Approver: "did:web:peer.example:approver"}}}
	negotiations := &submitNegotiationTaskRepoFake{existing: []db.NegotiationTaskData{{DID: did, Negotiator: "did:web:peer.example:negotiator"}}}
	submitter := Submitter{RTRepo: reviews, ATRepo: approvals, NTRepo: negotiations}

	responsible, state, err := submitter.prepareInitialSubmitTasks(context.Background(), nil, &db.ContractProcessData{
		DID: did, Origin: "did:web:origin.example", ContractVersion: 1,
	}, SubmitCmd{
		DID:         did,
		SubmittedBy: "did:web:origin.example:user:creator",
		Reviewers:   []string{" did:web:peer.example:reviewer ", "did:web:peer.example:reviewer"},
		Approvers:   []string{"did:web:peer.example:approver", "did:web:peer.example:approver"},
		Negotiators: []string{"did:web:peer.example:negotiator", " did:web:peer.example:negotiator "},
	})
	if err != nil {
		t.Fatalf("prepareInitialSubmitTasks returned error: %v", err)
	}
	if state != contractstate.Negotiation {
		t.Fatalf("target state = %s, want %s", state, contractstate.Negotiation)
	}
	if len(reviews.created) != 0 || len(approvals.created) != 0 || len(negotiations.created) != 0 {
		t.Fatalf("existing assignments were duplicated: review/approval/negotiation = %d/%d/%d", len(reviews.created), len(approvals.created), len(negotiations.created))
	}
	if !reflect.DeepEqual(responsible.Reviewers, []string{"did:web:peer.example:reviewer"}) ||
		!reflect.DeepEqual(responsible.Approvers, []string{"did:web:peer.example:approver"}) ||
		!reflect.DeepEqual(responsible.Negotiators, []string{"did:web:peer.example:negotiator"}) {
		t.Fatalf("responsible assignments were not normalized: %#v", responsible)
	}
}

func TestSubmitterContractDataForSemanticValidationPersistsSubmittedData(t *testing.T) {
	ctx := context.Background()
	did := "did:web:facis.example:contract:submit"
	submitted := minimalCanonicalContractData(t, "did:web:facis.example:contract:stale")
	storedInvalid := datatype.JSON(`{"semanticConditionValues":[{"parameterName":"provider.country","parameterValue":"USA"}]}`)

	repo := &submitContractRepoFake{
		stored: &db.Contract{
			DID:          did,
			ContractData: &storedInvalid,
		},
	}
	submitter := Submitter{CRepo: repo}

	contractData, err := submitter.contractDataForSemanticValidation(ctx, nil, SubmitCmd{
		DID:          did,
		ContractData: &submitted,
	})
	if err != nil {
		t.Fatalf("contractDataForSemanticValidation returned error: %v", err)
	}
	if repo.readDataCalled {
		t.Fatalf("stored contract data was read even though submitted data was provided")
	}
	if repo.updated == nil || repo.updated.ContractData == nil {
		t.Fatalf("submitted contract data was not persisted")
	}
	if contractData != repo.updated.ContractData {
		t.Fatalf("returned contract data is not the persisted normalized contract data")
	}
	if err := validation.ValidateContractSemantics(contractData); err != nil {
		t.Fatalf("persisted submitted contract data is not semantically valid: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(*repo.updated.ContractData, &decoded); err != nil {
		t.Fatalf("could not decode persisted contract data: %v", err)
	}
	if decoded["@id"] != did {
		t.Fatalf("persisted contract data @id = %v, want %s", decoded["@id"], did)
	}
}

func TestCanSubmitUpdatedContractDataOnlyAllowsCreatorSubmitStates(t *testing.T) {
	allowed := []string{
		contractstate.Draft.String(),
		contractstate.Rejected.String(),
	}
	for _, state := range allowed {
		if !canSubmitUpdatedContractData(state) {
			t.Fatalf("state %s should allow submitted contract data", state)
		}
	}

	rejected := []string{
		contractstate.Negotiation.String(),
		contractstate.Submitted.String(),
		contractstate.Reviewed.String(),
		contractstate.Approved.String(),
	}
	for _, state := range rejected {
		if canSubmitUpdatedContractData(state) {
			t.Fatalf("state %s should reject submitted contract data", state)
		}
	}
}

func minimalCanonicalContractData(t *testing.T, id string) datatype.JSON {
	t.Helper()
	data := map[string]any{
		"@context": map[string]any{
			"dcs": "https://w3id.org/facis/dcs/ontology/v1#",
			"xsd": "http://www.w3.org/2001/XMLSchema#",
		},
		"@id":   id,
		"@type": "dcs:Contract",
		"dcs:metadata": map[string]any{
			"@id":   id + "#metadata",
			"@type": "dcs:ContractMetadata",
		},
		"dcs:documentStructure": map[string]any{
			"@id":   id + "#document-structure",
			"@type": "dcs:DocumentStructure",
			"dcs:blocks": map[string]any{"@list": []any{
				map[string]any{
					"@id":   id + "#clause-1",
					"@type": "dcs:Clause",
					"dcs:content": map[string]any{
						"@list": []any{"Contract content."},
					},
				},
			}},
			"dcs:layout": []any{
				map[string]any{
					"@id":        id + "#root",
					"dcs:isRoot": true,
					"dcs:children": map[string]any{
						"@list": []any{
							map[string]any{"@id": id + "#clause-1"},
						},
					},
				},
				map[string]any{
					"@id": id + "#clause-1",
					"dcs:children": map[string]any{
						"@list": []any{},
					},
				},
			},
		},
	}
	result, err := datatype.NewJSON(data)
	if err != nil {
		t.Fatalf("could not create contract data: %v", err)
	}
	return result
}

type submitContractRepoFake struct {
	db.ContractRepo
	stored         *db.Contract
	updated        *db.ContractUpdateData
	readDataCalled bool
}

type submitReviewTaskRepoFake struct {
	db.ReviewTaskRepo
	existing []db.ReviewTaskData
	created  []db.ReviewTaskData
}

func (r *submitReviewTaskRepoFake) ReadAllByDID(context.Context, *sqlx.Tx, string) ([]db.ReviewTaskData, error) {
	return r.existing, nil
}

func (r *submitReviewTaskRepoFake) Create(_ context.Context, _ *sqlx.Tx, data db.ReviewTaskData) (*time.Time, error) {
	r.created = append(r.created, data)
	now := time.Now()
	return &now, nil
}

type submitApprovalTaskRepoFake struct {
	db.ApprovalTaskRepo
	existing []db.ApprovalTaskData
	created  []db.ApprovalTaskData
}

func (r *submitApprovalTaskRepoFake) ReadAllByDID(context.Context, *sqlx.Tx, string) ([]db.ApprovalTaskData, error) {
	return r.existing, nil
}

func (r *submitApprovalTaskRepoFake) Create(_ context.Context, _ *sqlx.Tx, data db.ApprovalTaskData) (*time.Time, error) {
	r.created = append(r.created, data)
	now := time.Now()
	return &now, nil
}

type submitNegotiationTaskRepoFake struct {
	db.NegotiationTaskRepo
	existing []db.NegotiationTaskData
	created  []db.NegotiationTaskData
}

func (r *submitNegotiationTaskRepoFake) ReadAllByDID(context.Context, *sqlx.Tx, string) ([]db.NegotiationTaskData, error) {
	return r.existing, nil
}

func (r *submitNegotiationTaskRepoFake) Create(_ context.Context, _ *sqlx.Tx, data db.NegotiationTaskData) (*time.Time, error) {
	r.created = append(r.created, data)
	now := time.Now()
	return &now, nil
}

func (r *submitContractRepoFake) ReadDataByDID(context.Context, *sqlx.Tx, string) (*db.Contract, error) {
	r.readDataCalled = true
	return r.stored, nil
}

func (r *submitContractRepoFake) Update(_ context.Context, _ *sqlx.Tx, data db.ContractUpdateData) error {
	r.updated = &data
	return nil
}

func (r *submitContractRepoFake) Create(context.Context, *sqlx.Tx, db.Contract) error {
	panic("not implemented")
}
