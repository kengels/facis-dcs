package qry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	pacdb "digital-contracting-service/internal/processauditandcompliance/db"

	"github.com/jmoiron/sqlx"
)

type IncidentReader struct {
	DB   *sqlx.DB
	Repo pacdb.IncidentRepo
}

func (h *IncidentReader) List(ctx context.Context) ([]pacdb.Incident, error) {
	tx, err := h.DB.BeginTxx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("could not start transaction: %w", err)
	}
	defer rollbackIncidentRead(tx)
	result, err := h.Repo.ReadAll(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("could not read incidents: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("could not commit incident read: %w", err)
	}
	return result, nil
}

func (h *IncidentReader) Get(ctx context.Context, incidentID string) (*pacdb.Incident, error) {
	tx, err := h.DB.BeginTxx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("could not start transaction: %w", err)
	}
	defer rollbackIncidentRead(tx)
	result, err := h.Repo.ReadByID(ctx, tx, incidentID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("could not commit incident read: %w", err)
	}
	return result, nil
}

func rollbackIncidentRead(tx *sqlx.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		log.Printf("could not rollback transaction: %v", err)
	}
}
