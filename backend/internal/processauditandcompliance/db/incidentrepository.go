package db

import (
	"context"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

var ErrIncidentNotFound = errors.New("incident not found")

type Incident struct {
	IncidentID           string    `db:"incident_id"`
	AffectedContractDIDs []string  `db:"affected_contract_dids"`
	AffectedTemplateDIDs []string  `db:"affected_template_dids"`
	FindingRefs          []string  `db:"finding_refs"`
	Reason               string    `db:"reason"`
	ReportedBy           string    `db:"reported_by"`
	ReportedAt           time.Time `db:"reported_at"`
}

type IncidentRepo interface {
	Create(ctx context.Context, tx *sqlx.Tx, incident Incident) error
	ReadAll(ctx context.Context, tx *sqlx.Tx) ([]Incident, error)
	ReadByID(ctx context.Context, tx *sqlx.Tx, incidentID string) (*Incident, error)
}
