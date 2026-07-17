package command

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/base/identity"
	"digital-contracting-service/internal/base/validation"
	"digital-contracting-service/internal/contractworkflowengine/datatype/actionflag"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/datatype/negotiationtaskstate"
	"digital-contracting-service/internal/contractworkflowengine/datatype/reviewtaskstate"
	"digital-contracting-service/internal/contractworkflowengine/db"
	contractevents "digital-contracting-service/internal/contractworkflowengine/event"
	"digital-contracting-service/internal/contractworkflowengine/negotiationmerging"
	"digital-contracting-service/internal/contractworkflowengine/remotesync/remoteaction"
	db2 "digital-contracting-service/internal/dcstodcs/db"

	"github.com/jmoiron/sqlx"
)

type SubmitCmd struct {
	DID          string                 `json:"did"`
	UpdatedAt    time.Time              `json:"updated_at"`
	SubmittedBy  string                 `json:"submitted_by"`
	Reviewers    []string               `json:"reviewers"`
	Approvers    []string               `json:"approvers"`
	Negotiators  []string               `json:"negotiators"`
	ActionFlag   *actionflag.ActionFlag `json:"action_flag"`
	Comments     []string               `json:"comments"`
	ContractData *datatype.JSON         `json:"contract_data"`
	HolderDID    string                 `json:"holder_did"`
	UserRoles    userrole.UserRoles     `json:"user_roles"`
	CauserDID    string                 `json:"causer_did"`
}

type Submitter struct {
	DB          *sqlx.DB
	CRepo       db.ContractRepo
	RTRepo      db.ReviewTaskRepo
	ATRepo      db.ApprovalTaskRepo
	NRepo       db.NegotiationRepo
	NTRepo      db.NegotiationTaskRepo
	SRepo       db2.SyncRepository
	DIDDocument identity.DIDDocument
}

func (h *Submitter) Handle(ctx context.Context, cmd SubmitCmd) error {

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer func(tx *sqlx.Tx) {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("could not rollback transaction: %v", err)
		}
	}(tx)

	processData, err := h.CRepo.ReadProcessDataByDID(ctx, tx, cmd.DID)
	if err != nil {
		return fmt.Errorf("could not read process data: %w", err)
	}

	localPeer, err := h.DIDDocument.GetID()
	if err != nil {
		return err
	}

	if processData.Origin != localPeer && cmd.CauserDID != processData.Origin {
		/*
			Not the Origin peer for this contract: forward unchanged instead of
			mutating locally (single-writer-per-aggregate, see package doc).
		*/

		err := tx.Commit()
		if err != nil {
			return fmt.Errorf("could not commit transaction: %w", err)
		}

		err = remoteaction.Submit.Execute(ctx, h.DB, h.DIDDocument, processData.Origin, processData.DID, cmd)
		if err != nil {
			return err
		}

		return nil
	}

	// Optimistic concurrency: reject if the caller's view of the contract is
	// older than what's stored (see package doc / ADR-0007).
	if cmd.UpdatedAt.Unix() < processData.UpdatedAt.Unix() {
		if localPeer != cmd.CauserDID {
			return errors.New("contract was updated elsewhere, please force synchronisation and reload")
		}
		return errors.New("contract was updated elsewhere, please reload")
	}

	hasSubmittedContractData := cmd.ContractData != nil && cmd.ContractData.IsNotNullValue()
	if hasSubmittedContractData && !canSubmitUpdatedContractData(processData.State) {
		return errors.New("contract data can only be submitted in draft or rejected state")
	}

	// The transition table (contractstate.Transitions) is the single source
	// of truth for which states Submit may be called from at all. It does
	// NOT replace the imperative branching below — that logic stays,
	// unchanged, to decide (given the current state and payload) exactly
	// which of the table's allowed outcomes applies; see command package doc.
	currentState := contractstate.ContractState(processData.State)
	if err := contractstate.ValidateTransition(currentState, contractstate.EventSubmit); err != nil {
		return err
	}

	// Submit is intentionally overloaded: its effect depends entirely on the
	// contract's current state (state pattern via if/else, not polymorphism).
	// See docs/backend architecture doc, section "Contract Workflow Engine".
	var nextState contractstate.ContractState
	// Draft and Offered share assignment/task preparation. Negotiation is an
	// optional stage: contracts with negotiators enter NEGOTIATION, while
	// contracts without negotiators proceed directly to SUBMITTED review.
	if processData.State == contractstate.Draft.String() || processData.State == contractstate.Offered.String() {

		if !cmd.UserRoles.HasRoles(userrole.ContractCreator) {
			return errors.New("invalid user permission")
		}

		// This avoids that state changes on different DCS are possible
		if cmd.CauserDID == localPeer && cmd.SubmittedBy != processData.CreatedBy {
			return errors.New("invalid participant")
		}

		contractData, err := h.contractDataForSemanticValidation(ctx, tx, cmd)
		if err != nil {
			return err
		}
		if err := validation.ValidateContractSemantics(contractData); err != nil {
			return fmt.Errorf("contract semantic validation failed: %w", err)
		}
		if err := validation.RequireHubConformance(ctx, contractData); err != nil {
			return fmt.Errorf("contract submission blocked: %w", err)
		}

		resp, targetState, err := h.prepareInitialSubmitTasks(ctx, tx, processData, cmd)
		if err != nil {
			return err
		}
		updateData := db.ContractUpdateData{
			DID:         cmd.DID,
			Responsible: resp,
		}
		err = h.CRepo.Update(ctx, tx, updateData)
		if err != nil {
			return fmt.Errorf("could not update contract: %w", err)
		}

		nextState = targetState

	} else if processData.State == contractstate.Rejected.String() {

		if !cmd.UserRoles.HasRoles(userrole.ContractCreator) {
			return errors.New("invalid user permission")
		}

		// This avoids that state changes on different DCS are possible
		if cmd.CauserDID == localPeer && cmd.SubmittedBy != processData.CreatedBy {
			return errors.New("invalid participant")
		}

		contractData, err := h.contractDataForSemanticValidation(ctx, tx, cmd)
		if err != nil {
			return err
		}
		if err := validation.ValidateContractSemantics(contractData); err != nil {
			return fmt.Errorf("contract semantic validation failed: %w", err)
		}
		if err := validation.RequireHubConformance(ctx, contractData); err != nil {
			return fmt.Errorf("contract submission blocked: %w", err)
		}

		err = h.RTRepo.ReopenTasks(ctx, tx, cmd.DID)
		if err != nil {
			return errors.New("could not reopen review tasks")
		}

		err = h.NTRepo.ReopenTasks(ctx, tx, cmd.DID)
		if err != nil {
			return errors.New("could not reopen negotiation tasks")
		}

		err = h.ATRepo.ReopenTasks(ctx, tx, cmd.DID)
		if err != nil {
			return errors.New("could not reopen approval tasks")
		}

		nextState = contractstate.Negotiation

	} else if processData.State == contractstate.Negotiation.String() {

		if !cmd.UserRoles.HasRoles(userrole.ContractCreator, userrole.ContractNegotiator, userrole.ContractReviewer) {
			return errors.New("invalid user permission")
		}

		isValidNegotiator, err := h.NTRepo.IsValidNegotiator(ctx, tx, cmd.DID, cmd.CauserDID)
		if err != nil {
			return fmt.Errorf("could not validate negotiator: %w", err)
		}

		if !isValidNegotiator {
			return errors.New("this peer is not a valid negotiator")
		}

		hasOpenNegotiations, err := h.NRepo.HasOpenNegotiationDecisions(ctx, tx, cmd.DID, processData.ContractVersion, cmd.CauserDID)
		if err != nil {
			return fmt.Errorf("could not check open negotiations: %w", err)
		}

		if hasOpenNegotiations {
			return errors.New("not all negotiations are processed")
		}

		err = h.NTRepo.UpdateState(ctx, tx, processData.DID, cmd.CauserDID, negotiationtaskstate.Accepted.String())
		if err != nil {
			return fmt.Errorf("could not update negotiation task: %w", err)
		}

		existOpenTasks, err := h.NTRepo.AnyTasksInState(ctx, tx, processData.DID, negotiationtaskstate.Open.String())
		if err != nil {
			return fmt.Errorf("could not check if review task exists: %w", err)
		}

		if !existOpenTasks {

			hasNegotiations, err := h.NRepo.HasNegotiationForContractVersion(ctx, tx, cmd.DID, processData.ContractVersion)
			if err != nil {
				return fmt.Errorf("could not check if negotiation exists: %w", err)
			}

			if hasNegotiations {
				// All negotiators have responded and there are accepted change
				// requests to fold in: snapshot the current row to contract_history,
				// merge the changes, and bump contract_version. The contract stays in
				// NEGOTIATION (nextState is left unset) rather than advancing to
				// SUBMITTED, since the merged result itself starts a new round.
				err = h.CRepo.CreateHistoryEntryForDID(ctx, tx, processData.DID)
				if err != nil {
					return fmt.Errorf("could not create history entry for did %s: %w", cmd.DID, err)
				}

				updatedData, err := negotiationmerging.MergeChangeRequests(ctx, tx, h.CRepo, h.NRepo, cmd.DID, processData.ContractVersion)
				if err != nil {
					return fmt.Errorf("could not merge change requests: %w", err)
				}

				updatedData.ContractVersion = processData.ContractVersion + 1
				err = h.CRepo.Update(ctx, tx, *updatedData)
				if err != nil {
					return fmt.Errorf("could not update contract version: %w", err)
				}

				evt := contractevents.IncreaseContractVersionEvent{
					DID:                cmd.DID,
					OldContractVersion: processData.ContractVersion,
					NewContractVersion: processData.ContractVersion + 1,
					SubmittedBy:        cmd.SubmittedBy,
					OccurredAt:         time.Now().UTC(),
					HolderDID:          cmd.HolderDID,
					UserRoles:          cmd.UserRoles,
				}
				err = event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine)
				if err != nil {
					return fmt.Errorf("could not create event: %w", err)
				}

			} else {
				nextState = contractstate.Submitted
			}
		}

	} else if processData.State == contractstate.Submitted.String() {

		if !cmd.UserRoles.HasRoles(userrole.ContractReviewer) {
			return errors.New("invalid user permission")
		}

		isValid, err := h.RTRepo.IsValidReviewer(ctx, tx, processData.DID, cmd.CauserDID)
		if err != nil {
			return err
		}

		if !isValid {
			return errors.New("invalid user")
		}

		if cmd.ActionFlag != nil {
			switch *cmd.ActionFlag {
			case actionflag.Approval:
				err = h.RTRepo.UpdateState(ctx, tx, processData.DID, cmd.CauserDID, contractstate.Approved.String())
				if err != nil {
					return fmt.Errorf("could not update approval task: %w", err)
				}
				existOpenTasks, err := h.RTRepo.AnyTasksInState(ctx, tx, processData.DID, reviewtaskstate.Open.String())
				if err != nil {
					return fmt.Errorf("could not check if review task exists: %w", err)
				}
				if !existOpenTasks {
					nextState = contractstate.Reviewed
				}
			case actionflag.Reject:
				err = h.RTRepo.ReopenTasks(ctx, tx, cmd.DID)
				if err != nil {
					return err
				}
				err = h.NTRepo.ReopenTasks(ctx, tx, cmd.DID)
				if err != nil {
					return err
				}
				err = h.ATRepo.ReopenTasks(ctx, tx, cmd.DID)
				if err != nil {
					return err
				}
				nextState = contractstate.Negotiation
			}

		} else {
			return errors.New("action flags is missing")
		}

	} else if processData.State == contractstate.Reviewed.String() {

		if !cmd.UserRoles.HasRoles(userrole.ContractApprover) {
			return errors.New("invalid user permission")
		}

		isValid, err := h.ATRepo.IsValidApprover(ctx, tx, processData.DID, cmd.CauserDID)
		if err != nil {
			return err
		}

		if !isValid {
			return errors.New("invalid user")
		}

		err = h.RTRepo.ReopenTasks(ctx, tx, cmd.DID)
		if err != nil {
			return err
		}

		err = h.ATRepo.ReopenTasks(ctx, tx, cmd.DID)
		if err != nil {
			return err
		}

		nextState = contractstate.Submitted

	} else {
		// Unreachable: the ValidateTransition guard above already rejects any
		// state without a declared EventSubmit outcome. Kept as a
		// defense-in-depth fallback.
		return fmt.Errorf("%w: submit is not allowed from state %s", contractstate.ErrInvalidTransition, processData.State)
	}

	if len(nextState) > 0 && processData.State != nextState.String() {
		if err := contractstate.ValidateOutcome(currentState, contractstate.EventSubmit, nextState); err != nil {
			return err
		}
		err = h.CRepo.UpdateState(ctx, tx, cmd.DID, nextState.String())
		if err != nil {
			return fmt.Errorf("could not update contract state: %w", err)
		}
	}

	evt := contractevents.SubmitEvent{
		DID:             cmd.DID,
		ContractVersion: processData.ContractVersion,
		SubmittedBy:     cmd.SubmittedBy,
		PreviousState:   processData.State,
		NewState:        nextState.String(),
		ActionFlag:      cmd.ActionFlag,
		Comments:        cmd.Comments,
		OccurredAt:      time.Now().UTC(),
		HolderDID:       cmd.HolderDID,
		UserRoles:       cmd.UserRoles,
	}
	err = event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine)
	if err != nil {
		return fmt.Errorf("could not create event: %w", err)
	}

	return tx.Commit()
}

func uniqueAssignees(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		assignee := strings.TrimSpace(value)
		if assignee == "" {
			continue
		}
		if _, exists := seen[assignee]; exists {
			continue
		}
		seen[assignee] = struct{}{}
		result = append(result, assignee)
	}
	return result
}

func (h *Submitter) prepareInitialSubmitTasks(
	ctx context.Context,
	tx *sqlx.Tx,
	processData *db.ContractProcessData,
	cmd SubmitCmd,
) (*db.Responsible, contractstate.ContractState, error) {
	reviewers := uniqueAssignees(cmd.Reviewers)
	approvers := uniqueAssignees(cmd.Approvers)
	negotiators := uniqueAssignees(cmd.Negotiators)
	if len(reviewers) == 0 {
		return nil, "", errors.New("no reviewers provided")
	}
	if len(approvers) == 0 {
		return nil, "", errors.New("no approvers provided")
	}

	if err := h.ensureSubmitTasks(ctx, tx, cmd.DID, cmd.SubmittedBy, processData.ContractVersion, reviewers, approvers, negotiators); err != nil {
		return nil, "", err
	}

	responsible := &db.Responsible{
		// Responsible.Creator is the ORIGIN PEER DID, not the human actor.
		Creator:     processData.Origin,
		Reviewers:   reviewers,
		Approvers:   approvers,
		Negotiators: negotiators,
	}
	if len(negotiators) == 0 {
		return responsible, contractstate.Submitted, nil
	}
	return responsible, contractstate.Negotiation, nil
}

func (h *Submitter) ensureSubmitTasks(
	ctx context.Context,
	tx *sqlx.Tx,
	did string,
	createdBy string,
	contractVersion int,
	reviewers []string,
	approvers []string,
	negotiators []string,
) error {
	existingReviewTasks, err := h.RTRepo.ReadAllByDID(ctx, tx, did)
	if err != nil {
		return fmt.Errorf("could not read review tasks: %w", err)
	}
	existingReviewers := make(map[string]struct{}, len(existingReviewTasks))
	for _, task := range existingReviewTasks {
		existingReviewers[task.Reviewer] = struct{}{}
	}
	for _, reviewer := range reviewers {
		if _, exists := existingReviewers[reviewer]; exists {
			continue
		}
		if _, err := h.RTRepo.Create(ctx, tx, db.ReviewTaskData{
			DID: did, Reviewer: reviewer, State: reviewtaskstate.Open.String(), CreatedBy: createdBy, ContractVersion: contractVersion,
		}); err != nil {
			return fmt.Errorf("could not create review task: %w", err)
		}
		existingReviewers[reviewer] = struct{}{}
	}

	existingApprovalTasks, err := h.ATRepo.ReadAllByDID(ctx, tx, did)
	if err != nil {
		return fmt.Errorf("could not read approval tasks: %w", err)
	}
	existingApprovers := make(map[string]struct{}, len(existingApprovalTasks))
	for _, task := range existingApprovalTasks {
		existingApprovers[task.Approver] = struct{}{}
	}
	for _, approver := range approvers {
		if _, exists := existingApprovers[approver]; exists {
			continue
		}
		if _, err := h.ATRepo.Create(ctx, tx, db.ApprovalTaskData{
			DID: did, Approver: approver, State: reviewtaskstate.Open.String(), CreatedBy: createdBy,
		}); err != nil {
			return fmt.Errorf("could not create approval task: %w", err)
		}
		existingApprovers[approver] = struct{}{}
	}

	existingNegotiationTasks, err := h.NTRepo.ReadAllByDID(ctx, tx, did)
	if err != nil {
		return fmt.Errorf("could not read negotiation tasks: %w", err)
	}
	existingNegotiators := make(map[string]struct{}, len(existingNegotiationTasks))
	for _, task := range existingNegotiationTasks {
		existingNegotiators[task.Negotiator] = struct{}{}
	}
	for _, negotiator := range negotiators {
		if _, exists := existingNegotiators[negotiator]; exists {
			continue
		}
		if _, err := h.NTRepo.Create(ctx, tx, db.NegotiationTaskData{
			DID: did, Negotiator: negotiator, State: negotiationtaskstate.Open.String(), CreatedBy: createdBy,
		}); err != nil {
			return fmt.Errorf("could not create negotiation task: %w", err)
		}
		existingNegotiators[negotiator] = struct{}{}
	}
	return nil
}

func (h *Submitter) contractDataForSemanticValidation(ctx context.Context, tx *sqlx.Tx, cmd SubmitCmd) (*datatype.JSON, error) {
	if cmd.ContractData != nil && cmd.ContractData.IsNotNullValue() {
		normalizedContractData, err := validation.NormalizeContractDataForPersistence(cmd.ContractData, cmd.DID, false)
		if err != nil {
			return nil, fmt.Errorf("contract data validation failed: %w", err)
		}
		updateData := db.ContractUpdateData{
			DID:          cmd.DID,
			ContractData: normalizedContractData,
		}
		if err := h.CRepo.Update(ctx, tx, updateData); err != nil {
			return nil, fmt.Errorf("could not update submitted contract data: %w", err)
		}
		return normalizedContractData, nil
	}

	contractData, err := h.CRepo.ReadDataByDID(ctx, tx, cmd.DID)
	if err != nil {
		return nil, fmt.Errorf("could not read contract data: %w", err)
	}
	return contractData.ContractData, nil
}

func canSubmitUpdatedContractData(state string) bool {
	return state == contractstate.Draft.String() || state == contractstate.Rejected.String()
}
