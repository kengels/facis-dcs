package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	contractstoragearchive "digital-contracting-service/gen/contract_storage_archive"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/middleware"
)

type archiveAlertRow struct {
	ID              string         `db:"id"`
	DID             string         `db:"did"`
	ContractVersion int            `db:"contract_version"`
	AlertType       string         `db:"alert_type"`
	DueAt           sql.NullTime   `db:"due_at"`
	Message         string         `db:"message"`
	CreatedAt       time.Time      `db:"created_at"`
	AcknowledgedAt  sql.NullTime   `db:"acknowledged_at"`
	AcknowledgedBy  sql.NullString `db:"acknowledged_by"`
}

type archiveSavedQueryRow struct {
	ID        string        `db:"id"`
	Name      string        `db:"name"`
	Filters   datatype.JSON `db:"filters"`
	CreatedAt time.Time     `db:"created_at"`
	UpdatedAt time.Time     `db:"updated_at"`
}

type archiveComponentRow struct {
	ID                string        `db:"id"`
	DID               string        `db:"did"`
	ContractVersion   int           `db:"contract_version"`
	ComponentIRI      string        `db:"component_iri"`
	PartyIDs          datatype.JSON `db:"party_ids"`
	ComponentSnapshot datatype.JSON `db:"component_snapshot"`
	ContentHash       string        `db:"content_hash"`
}

type archiveMonitoringPreferencesRow struct {
	Enabled    bool      `db:"enabled"`
	NoticeDays int       `db:"default_notice_days"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// refreshArchiveAlerts materializes deterministic alert rows. The uniqueness
// constraint makes this safe to call from both the dashboard and the expiry
// scheduler after restarts.
func (s *contractStorageArchivesrvc) refreshArchiveAlerts(ctx context.Context) error {
	defaultDays := conf.ArchiveDefaultNoticeDays()
	statements := []string{
		`INSERT INTO archive_alerts (did, contract_version, alert_type, due_at, message)
		 SELECT did, contract_version, 'EXPIRY_DUE', exp_date, 'Contract approaches its configured expiration date'
		 FROM contracts_archive_metadata
		 WHERE exp_date > NOW() AND exp_date <= NOW() + make_interval(days => COALESCE(exp_notice_period, $1))
		 ON CONFLICT (did, contract_version, alert_type, due_at) DO NOTHING`,
		`INSERT INTO archive_alerts (did, contract_version, alert_type, due_at, message)
		 SELECT did, contract_version, 'RENEWAL_DUE', exp_date, 'Contract renewal is due'
		 FROM contracts_archive_metadata
		 WHERE exp_policy = 'RENEWAL' AND exp_date <= NOW() + make_interval(days => COALESCE(exp_notice_period, $1))
		 ON CONFLICT (did, contract_version, alert_type, due_at) DO NOTHING`,
		`INSERT INTO archive_alerts (did, contract_version, alert_type, due_at, message)
		 SELECT did, contract_version, 'EXPIRED', exp_date, 'Contract has expired and is no longer active'
		 FROM contracts_archive_metadata WHERE exp_date <= NOW()
		 ON CONFLICT (did, contract_version, alert_type, due_at) DO NOTHING`,
		`INSERT INTO archive_alerts (did, contract_version, alert_type, due_at, message)
		 SELECT did, contract_version, 'EVIDENCE_MISSING', stored_at, 'Archive evidence is incomplete'
		 FROM contract_archive_entries
		 WHERE archive_status <> 'DELETED' AND (content_hash !~ '^sha256:[a-f0-9]{64}$' OR snapshot_cid = '')
		 ON CONFLICT (did, contract_version, alert_type, due_at) DO NOTHING`,
		`INSERT INTO archive_alerts (did, contract_version, alert_type, due_at, message)
		 SELECT did, contract_version, 'COMPLIANCE_FAILED', stored_at, 'Archived contract failed a compliance check'
		 FROM contract_archive_entries
		 WHERE archive_status <> 'DELETED' AND compliance_status = 'NON_COMPLIANT'
		 ON CONFLICT (did, contract_version, alert_type, due_at) DO NOTHING`,
		`INSERT INTO archive_alerts (did, contract_version, alert_type, due_at, message)
		 SELECT did, contract_version, 'RETENTION_DUE', retention_until, 'Archive retention period has ended'
		 FROM contract_archive_entries
		 WHERE archive_status = 'RETAINED' AND retention_until IS NOT NULL AND retention_until <= NOW()
		 ON CONFLICT (did, contract_version, alert_type, due_at) DO NOTHING`,
	}
	for _, statement := range statements {
		if _, err := s.DB.ExecContext(ctx, statement, defaultDays); err != nil {
			return fmt.Errorf("refresh archive alerts: %w", err)
		}
	}
	if _, err := s.DB.ExecContext(ctx, `
		INSERT INTO outbox_events (component,event_type,event_data,did)
		SELECT $1,'ARCHIVE_ALERT_RAISED',jsonb_build_object(
			'alert_id',a.id,'did',a.did,'contract_version',a.contract_version,
			'alert_type',a.alert_type,'due_at',a.due_at,'message',a.message
		),a.did
		FROM archive_alerts a
		WHERE NOT EXISTS (
			SELECT 1 FROM outbox_events e
			WHERE e.event_type='ARCHIVE_ALERT_RAISED' AND e.event_data->>'alert_id'=a.id::text
		)
	`, componenttype.ContractStorageArchive.String()); err != nil {
		return fmt.Errorf("publish archive alerts: %w", err)
	}
	return nil
}

func (s *contractStorageArchivesrvc) Alerts(ctx context.Context, p *contractstoragearchive.AlertsPayload) ([]*contractstoragearchive.ArchiveAlert, error) {
	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()
	if err := s.refreshArchiveAlerts(ctx); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	preference, err := s.archiveMonitoringPreferences(ctx, middleware.GetParticipantID(ctx))
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	if !preference.Enabled {
		return []*contractstoragearchive.ArchiveAlert{}, nil
	}
	query := `SELECT id, did, contract_version, alert_type, due_at, message, created_at, acknowledged_at, acknowledged_by FROM archive_alerts`
	conditions := []string{`(due_at IS NULL OR due_at <= NOW() + make_interval(days => $1))`}
	if p.IncludeAcknowledged == nil || !*p.IncludeAcknowledged {
		conditions = append(conditions, `acknowledged_at IS NULL`)
	}
	query += ` WHERE ` + strings.Join(conditions, ` AND `)
	query += ` ORDER BY COALESCE(due_at, created_at), created_at, did`
	rows := make([]archiveAlertRow, 0)
	if err := s.DB.SelectContext(ctx, &rows, query, preference.NoticeDays); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	result := make([]*contractstoragearchive.ArchiveAlert, 0, len(rows))
	for _, row := range rows {
		result = append(result, archiveAlertResult(row))
	}
	return result, nil
}

func (s *contractStorageArchivesrvc) archiveMonitoringPreferences(ctx context.Context, owner string) (archiveMonitoringPreferencesRow, error) {
	row := archiveMonitoringPreferencesRow{Enabled: true, NoticeDays: conf.ArchiveDefaultNoticeDays(), UpdatedAt: time.Now().UTC()}
	err := s.DB.GetContext(ctx, &row, `SELECT enabled, default_notice_days, updated_at
		FROM archive_monitoring_preferences WHERE owner_id = $1`, owner)
	if errors.Is(err, sql.ErrNoRows) {
		return row, nil
	}
	return row, err
}

func (s *contractStorageArchivesrvc) MonitoringPreferences(ctx context.Context, _ *contractstoragearchive.MonitoringPreferencesPayload) (*contractstoragearchive.ArchiveMonitoringPreferences, error) {
	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()
	row, err := s.archiveMonitoringPreferences(ctx, middleware.GetParticipantID(ctx))
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	return archiveMonitoringPreferencesResult(row), nil
}

func (s *contractStorageArchivesrvc) SetMonitoringPreferences(ctx context.Context, p *contractstoragearchive.SetMonitoringPreferencesPayload) (*contractstoragearchive.ArchiveMonitoringPreferences, error) {
	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()
	owner := middleware.GetParticipantID(ctx)
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	defer func() { _ = tx.Rollback() }()
	var row archiveMonitoringPreferencesRow
	err = tx.GetContext(ctx, &row, `INSERT INTO archive_monitoring_preferences
		(owner_id, enabled, default_notice_days, updated_at) VALUES ($1,$2,$3,NOW())
		ON CONFLICT (owner_id) DO UPDATE SET enabled=EXCLUDED.enabled,
		default_notice_days=EXCLUDED.default_notice_days, updated_at=NOW()
		RETURNING enabled, default_notice_days, updated_at`, owner, p.Enabled, p.NoticeDays)
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO outbox_events (event_type,event_data,component)
		VALUES ('ARCHIVE_MONITORING_PREFERENCES_SET',jsonb_build_object('owner_id',$1,'enabled',$2,'notice_days',$3),$4)`,
		owner, p.Enabled, p.NoticeDays, componenttype.ContractStorageArchive.String()); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	if err = tx.Commit(); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	return archiveMonitoringPreferencesResult(row), nil
}

func archiveMonitoringPreferencesResult(row archiveMonitoringPreferencesRow) *contractstoragearchive.ArchiveMonitoringPreferences {
	return &contractstoragearchive.ArchiveMonitoringPreferences{
		Enabled: row.Enabled, NoticeDays: row.NoticeDays, UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (s *contractStorageArchivesrvc) AcknowledgeAlert(ctx context.Context, p *contractstoragearchive.AcknowledgeAlertPayload) (*contractstoragearchive.ArchiveAlert, error) {
	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()
	var row archiveAlertRow
	err := s.DB.GetContext(ctx, &row, `
		UPDATE archive_alerts SET acknowledged_at = COALESCE(acknowledged_at, NOW()),
		acknowledged_by = COALESCE(acknowledged_by, $1)
		WHERE id = $2
		RETURNING id, did, contract_version, alert_type, due_at, message, created_at, acknowledged_at, acknowledged_by
	`, middleware.GetParticipantID(ctx), p.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, contractstoragearchive.MakeBadRequest(fmt.Errorf("archive alert %q not found", p.ID))
	}
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	return archiveAlertResult(row), nil
}

func (s *contractStorageArchivesrvc) SetRetention(ctx context.Context, p *contractstoragearchive.SetRetentionPayload) (*contractstoragearchive.ArchiveRetention, error) {
	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()
	if p.RetentionUntil == nil || *p.RetentionUntil == "" {
		return nil, contractstoragearchive.MakeBadRequest(errors.New("retention_until is required"))
	}
	deadline, err := time.Parse(time.RFC3339, *p.RetentionUntil)
	if err != nil || !deadline.After(time.Now()) {
		return nil, contractstoragearchive.MakeBadRequest(errors.New("retention_until must be a future RFC3339 timestamp"))
	}
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	err = tx.GetContext(ctx, &status, `UPDATE contract_archive_entries
		SET retention_until = $1, archive_status = 'RETAINED'
		WHERE did = $2
		  AND archive_status IN ('STORED', 'RETAINED')
		  AND (retention_until IS NULL OR retention_until <= $1)
		RETURNING archive_status`, deadline.UTC(), p.Did)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, contractstoragearchive.MakeBadRequest(fmt.Errorf("active archive entry %q was not found or its retention deadline would be shortened", p.Did))
	}
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO outbox_events (event_type, event_data, component, did)
		VALUES ('ARCHIVE_RETENTION_SET', jsonb_build_object('did',$1,'retention_until',$2,'justification',$3,'set_by',$4), $5, $1)`,
		p.Did, deadline.UTC(), p.Justification, middleware.GetParticipantID(ctx), componenttype.ContractStorageArchive.String()); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	value := deadline.UTC().Format(time.RFC3339)
	return &contractstoragearchive.ArchiveRetention{Did: p.Did, RetentionUntil: &value, ArchiveStatus: status}, nil
}

func (s *contractStorageArchivesrvc) SavedQueries(ctx context.Context, _ *contractstoragearchive.SavedQueriesPayload) ([]*contractstoragearchive.ArchiveSavedQuery, error) {
	rows := make([]archiveSavedQueryRow, 0)
	if err := s.DB.SelectContext(ctx, &rows, `SELECT id, name, filters, created_at, updated_at FROM archive_saved_queries WHERE owner_id=$1 ORDER BY name`, middleware.GetHolderDID(ctx)); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	return archiveSavedQueryResults(rows), nil
}

func (s *contractStorageArchivesrvc) SaveQuery(ctx context.Context, p *contractstoragearchive.SaveQueryPayload) (*contractstoragearchive.ArchiveSavedQuery, error) {
	encoded, err := json.Marshal(p.Filters)
	if err != nil {
		return nil, contractstoragearchive.MakeBadRequest(err)
	}
	var row archiveSavedQueryRow
	err = s.DB.GetContext(ctx, &row, `INSERT INTO archive_saved_queries (owner_id,name,filters) VALUES ($1,$2,$3)
		ON CONFLICT (owner_id,name) DO UPDATE SET filters=EXCLUDED.filters, updated_at=NOW()
		RETURNING id,name,filters,created_at,updated_at`, middleware.GetHolderDID(ctx), p.Name, encoded)
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	return archiveSavedQueryResult(row), nil
}

func (s *contractStorageArchivesrvc) DeleteSavedQuery(ctx context.Context, p *contractstoragearchive.DeleteSavedQueryPayload) (bool, error) {
	result, err := s.DB.ExecContext(ctx, `DELETE FROM archive_saved_queries WHERE id=$1 AND owner_id=$2`, p.ID, middleware.GetHolderDID(ctx))
	if err != nil {
		return false, contractstoragearchive.MakeInternalError(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, contractstoragearchive.MakeInternalError(err)
	}
	if count == 0 {
		return false, contractstoragearchive.MakeBadRequest(fmt.Errorf("saved query %q not found", p.ID))
	}
	return true, nil
}

func (s *contractStorageArchivesrvc) Components(ctx context.Context, p *contractstoragearchive.ComponentsPayload) ([]*contractstoragearchive.ArchiveComponent, error) {
	rows := make([]archiveComponentRow, 0)
	query := `SELECT id,did,contract_version,component_iri,party_ids,component_snapshot,content_hash FROM archive_components WHERE did=$1`
	params := []any{p.Did}
	roles := userrole.UserRoles(middleware.GetUserRoles(ctx))
	if !roles.HasRoles(userrole.ArchiveManager, userrole.Auditor) {
		query += ` AND (party_ids = '[]'::jsonb OR party_ids ? $2 OR party_ids ? $3)`
		params = append(params, middleware.GetHolderDID(ctx), middleware.GetParticipantID(ctx))
	}
	query += ` ORDER BY component_iri`
	if err := s.DB.SelectContext(ctx, &rows, query, params...); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	result := make([]*contractstoragearchive.ArchiveComponent, 0, len(rows))
	for _, row := range rows {
		var partyIDs []string
		var snapshot any
		_ = json.Unmarshal(row.PartyIDs, &partyIDs)
		_ = json.Unmarshal(row.ComponentSnapshot, &snapshot)
		result = append(result, &contractstoragearchive.ArchiveComponent{ID: row.ID, Did: row.DID, ContractVersion: row.ContractVersion, ComponentIri: row.ComponentIRI, PartyIds: partyIDs, ComponentSnapshot: snapshot, ContentHash: row.ContentHash})
	}
	return result, nil
}

func archiveAlertResult(row archiveAlertRow) *contractstoragearchive.ArchiveAlert {
	result := &contractstoragearchive.ArchiveAlert{ID: row.ID, Did: row.DID, ContractVersion: row.ContractVersion, AlertType: row.AlertType, Message: row.Message, CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339)}
	if row.DueAt.Valid {
		value := row.DueAt.Time.UTC().Format(time.RFC3339)
		result.DueAt = &value
	}
	if row.AcknowledgedAt.Valid {
		value := row.AcknowledgedAt.Time.UTC().Format(time.RFC3339)
		result.AcknowledgedAt = &value
	}
	if row.AcknowledgedBy.Valid {
		result.AcknowledgedBy = &row.AcknowledgedBy.String
	}
	return result
}

func archiveSavedQueryResults(rows []archiveSavedQueryRow) []*contractstoragearchive.ArchiveSavedQuery {
	result := make([]*contractstoragearchive.ArchiveSavedQuery, 0, len(rows))
	for _, row := range rows {
		result = append(result, archiveSavedQueryResult(row))
	}
	return result
}

func archiveSavedQueryResult(row archiveSavedQueryRow) *contractstoragearchive.ArchiveSavedQuery {
	var filters any
	_ = json.Unmarshal(row.Filters, &filters)
	return &contractstoragearchive.ArchiveSavedQuery{ID: row.ID, Name: row.Name, Filters: filters, CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339)}
}
