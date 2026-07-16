package qry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/base"
	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/base/event"
	event2 "digital-contracting-service/internal/processauditandcompliance/event"
)

type GetAuditLogQry struct {
	Scope         componenttype.ComponentType
	AuditedBy     string
	HolderDID     string
	UserRoles     userrole.UserRoles
	DID           string
	Justification string
}

type Auditor struct {
	DB           *sqlx.DB
	ATrailReader base.AuditTrailReader
}

// ReadScopedAuditEntries combines the asynchronously materialized audit chain
// with canonical transactional outbox events for one resource.
func ReadScopedAuditEntries(ctx context.Context, tx *sqlx.Tx, reader base.AuditTrailReader, scope componenttype.ComponentType, did string) ([]datatype.AuditLogEntry, error) {
	entries, err := reader.ReadAuditLogEntriesByComponentAndDID(ctx, tx, scope, did)
	if err != nil {
		return nil, fmt.Errorf("could not read audit log entries: %w", err)
	}
	outboxEntries := make([]datatype.AuditLogEntry, 0)
	if err := tx.SelectContext(ctx, &outboxEntries, `
		SELECT id, component, event_type, event_data, did, created_at,
		       NULL::text AS res_log_pred_cid, NULL::text AS global_log_pred_cid
		FROM outbox_events
		WHERE component = $1 AND did = $2
		ORDER BY created_at DESC, id DESC
	`, scope.String(), did); err != nil {
		return nil, fmt.Errorf("could not read transactional audit entries: %w", err)
	}
	seen := make(map[int64]struct{}, len(entries)+len(outboxEntries))
	for _, entry := range entries {
		seen[entry.ID] = struct{}{}
	}
	for _, entry := range outboxEntries {
		if _, exists := seen[entry.ID]; exists {
			continue
		}
		entries = append(entries, entry)
		seen[entry.ID] = struct{}{}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].CreatedAt.Equal(entries[j].CreatedAt) {
			return entries[i].ID > entries[j].ID
		}
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})
	return entries, nil
}

func (h *Auditor) Handle(ctx context.Context, query GetAuditLogQry) ([][]datatype.AuditLogEntry, error) {

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("could not start transaction: %w", err)
	}
	defer func(tx *sqlx.Tx) {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("could not rollback transaction: %v", err)
		}
	}(tx)

	var result [][]datatype.AuditLogEntry
	if query.DID != "" {
		entries, readErr := ReadScopedAuditEntries(ctx, tx, h.ATrailReader, query.Scope, query.DID)
		if readErr != nil {
			return nil, readErr
		}
		result = [][]datatype.AuditLogEntry{entries}
	} else {
		result, err = h.ATrailReader.ReadAuditLogEntriesByComponent(ctx, tx, query.Scope)
		if err != nil {
			return nil, fmt.Errorf("could not read audit log entries: %w", err)
		}
	}

	evt := event2.AuditEvent{
		Scope:         query.Scope,
		ComponentType: componenttype.ProcessAuditAndCompliance,
		AuditedBy:     query.AuditedBy,
		OccurredAt:    time.Now().UTC(),
		HolderDID:     query.HolderDID,
		UserRoles:     query.UserRoles,
		DID:           query.DID,
		Justification: query.Justification,
	}
	err = event.Create(ctx, tx, evt, componenttype.ProcessAuditAndCompliance)
	if err != nil {
		return nil, fmt.Errorf("could not create event: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("could not commit transaction: %w", err)
	}

	return result, nil
}
