package command

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/base/event"
	pacdb "digital-contracting-service/internal/processauditandcompliance/db"
	pacevent "digital-contracting-service/internal/processauditandcompliance/event"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type ReportIncidentCmd struct {
	AffectedContractDIDs []string
	AffectedTemplateDIDs []string
	FindingRefs          []string
	Reason               string
	ReportedBy           string
	HolderDID            string
	UserRoles            userrole.UserRoles
}

var ErrInvalidIncident = errors.New("invalid incident")

type IncidentReporter struct {
	DB   *sqlx.DB
	Repo pacdb.IncidentRepo
}

func (h *IncidentReporter) Handle(ctx context.Context, cmd ReportIncidentCmd) (*pacdb.Incident, error) {
	resources := append(append([]string{}, cmd.AffectedContractDIDs...), cmd.AffectedTemplateDIDs...)
	if len(resources) == 0 {
		return nil, fmt.Errorf("%w: at least one affected contract or template DID is required", ErrInvalidIncident)
	}
	for _, resourceDID := range resources {
		if strings.TrimSpace(resourceDID) == "" {
			return nil, fmt.Errorf("%w: affected DIDs must not be empty", ErrInvalidIncident)
		}
	}

	incident := pacdb.Incident{
		IncidentID: uuid.NewString(), AffectedContractDIDs: cmd.AffectedContractDIDs,
		AffectedTemplateDIDs: cmd.AffectedTemplateDIDs, FindingRefs: cmd.FindingRefs,
		Reason: strings.TrimSpace(cmd.Reason), ReportedBy: cmd.ReportedBy, ReportedAt: time.Now().UTC(),
	}
	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("could not start transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("could not rollback transaction: %v", err)
		}
	}()
	if err := h.Repo.Create(ctx, tx, incident); err != nil {
		return nil, fmt.Errorf("could not persist incident: %w", err)
	}
	seen := make(map[string]struct{}, len(resources))
	for _, resourceDID := range resources {
		resourceDID = strings.TrimSpace(resourceDID)
		if _, exists := seen[resourceDID]; exists {
			continue
		}
		seen[resourceDID] = struct{}{}
		evt := pacevent.IncidentReportedEvent{
			IncidentID: incident.IncidentID, ResourceDID: resourceDID,
			AffectedContractDIDs: incident.AffectedContractDIDs, AffectedTemplateDIDs: incident.AffectedTemplateDIDs,
			FindingRefs: incident.FindingRefs, Reason: incident.Reason, ReportedBy: incident.ReportedBy,
			ReportedAt: incident.ReportedAt, HolderDID: cmd.HolderDID, UserRoles: cmd.UserRoles,
		}
		if err := event.Create(ctx, tx, evt, componenttype.ProcessAuditAndCompliance); err != nil {
			return nil, fmt.Errorf("could not create incident event: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("could not commit incident: %w", err)
	}
	return &incident, nil
}
