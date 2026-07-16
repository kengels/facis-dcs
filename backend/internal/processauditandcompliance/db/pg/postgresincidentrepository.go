package pg

import (
	"context"
	"database/sql"
	"errors"

	pacdb "digital-contracting-service/internal/processauditandcompliance/db"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type PostgresIncidentRepo struct{}

const incidentSelect = `
	SELECT incident_id, affected_contract_dids, affected_template_dids,
	       finding_refs, reason, reported_by, reported_at
	FROM pac_incidents`

type incidentRow struct {
	IncidentID           string         `db:"incident_id"`
	AffectedContractDIDs pq.StringArray `db:"affected_contract_dids"`
	AffectedTemplateDIDs pq.StringArray `db:"affected_template_dids"`
	FindingRefs          pq.StringArray `db:"finding_refs"`
	Reason               string         `db:"reason"`
	ReportedBy           string         `db:"reported_by"`
	ReportedAt           sql.NullTime   `db:"reported_at"`
}

func (r *PostgresIncidentRepo) Create(ctx context.Context, tx *sqlx.Tx, incident pacdb.Incident) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO pac_incidents (
			incident_id, affected_contract_dids, affected_template_dids,
			finding_refs, reason, reported_by, reported_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, incident.IncidentID, pq.Array(incident.AffectedContractDIDs), pq.Array(incident.AffectedTemplateDIDs),
		pq.Array(incident.FindingRefs), incident.Reason, incident.ReportedBy, incident.ReportedAt)
	return err
}

func (r *PostgresIncidentRepo) ReadAll(ctx context.Context, tx *sqlx.Tx) ([]pacdb.Incident, error) {
	rows := make([]incidentRow, 0)
	if err := tx.SelectContext(ctx, &rows, incidentSelect+` ORDER BY reported_at DESC, incident_id`); err != nil {
		return nil, err
	}
	result := make([]pacdb.Incident, 0, len(rows))
	for _, row := range rows {
		result = append(result, toIncident(row))
	}
	return result, nil
}

func (r *PostgresIncidentRepo) ReadByID(ctx context.Context, tx *sqlx.Tx, incidentID string) (*pacdb.Incident, error) {
	var row incidentRow
	if err := tx.GetContext(ctx, &row, incidentSelect+` WHERE incident_id = $1`, incidentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pacdb.ErrIncidentNotFound
		}
		return nil, err
	}
	incident := toIncident(row)
	return &incident, nil
}

func toIncident(row incidentRow) pacdb.Incident {
	return pacdb.Incident{
		IncidentID: row.IncidentID, AffectedContractDIDs: []string(row.AffectedContractDIDs),
		AffectedTemplateDIDs: []string(row.AffectedTemplateDIDs), FindingRefs: []string(row.FindingRefs),
		Reason: row.Reason, ReportedBy: row.ReportedBy, ReportedAt: row.ReportedAt.Time,
	}
}
