// Package contractworkflowengine implements the core contract lifecycle:
// creation, negotiation, review, approval, termination, and expiry (this
// file's CronJob). It follows the repository-wide CQRS layout (command/,
// query/, db/, datatype/, event/), plus two contract-specific additions:
// negotiationmerging (folds accepted change requests into a new contract
// version) and remotesync (the command/RPC side of DCS-to-DCS federation;
// the peer-transport side lives in the separate dcstodcs package).
//
// Ownership model: every contract has a single writer, its Origin peer (see
// command package doc). Expiry is additionally exposed instantly to readers
// via the contracts_effective DB view (EXPIRED computed at query time),
// while this file's cron job lags behind to persist the state and emit the
// corresponding event/audit-trail entry.
package contractworkflowengine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"

	baseconf "digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/contractworkflowengine/conf"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/datatype/expirationpolicy"
	database "digital-contracting-service/internal/contractworkflowengine/db"
	contractevents "digital-contracting-service/internal/contractworkflowengine/event"
)

type CronJob struct {
	DB    *sqlx.DB
	CRepo database.ContractRepo
}

func (j CronJob) Start(ctx context.Context, db *sqlx.DB) {
	go startExpiryScheduler(ctx, db, j.CRepo, conf.ExpirationCronJobTimeOut())
}

func startExpiryScheduler(ctx context.Context, db *sqlx.DB, repo database.ContractRepo, interval time.Duration) {

	readExpiredContracts := func() ([]database.ContractMetadata, error) {
		tx, err := db.BeginTxx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("could not start transaction: %w", err)
		}
		defer func(tx *sqlx.Tx) {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				log.Printf("could not rollback transaction: %v", err)
			}
		}(tx)

		expiredContracts, err := repo.ReadExpiredContracts(ctx, tx)
		if err != nil {
			return nil, fmt.Errorf("could not read expired contracts: %w", err)
		}

		err = tx.Commit()
		if err != nil {
			return nil, fmt.Errorf("could not commit transaction: %w", err)
		}

		return expiredContracts, nil
	}

	callExpirationLogic := func(expiredContract database.ContractMetadata) error {

		tx, err := db.BeginTxx(ctx, nil)
		if err != nil {
			return fmt.Errorf("could not start transaction: %w", err)
		}
		defer func(tx *sqlx.Tx) {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				log.Printf("could not rollback transaction: %v", err)
			}
		}(tx)

		var policy *expirationpolicy.ExpirationPolicy
		if expiredContract.ExpPolicy != nil {
			p, err := expirationpolicy.NewExpirationPolicy(*expiredContract.ExpPolicy)
			if err != nil {
				return fmt.Errorf("could not create expiration policy: %w", err)
			}
			policy = &p
		} else {
			return fmt.Errorf("unknown expiration policy for expired contract with DID %s", expiredContract.DID)
		}

		// Readers already see EXPIRED instantly via the contracts_effective view once
		// exp_date has passed; this persists that state physically and emits the
		// ContractExpired event, so it necessarily lags the view by up to the poll interval.
		err = repo.UpdateState(ctx, tx, expiredContract.DID, contractstate.Expired.String())
		if err != nil {
			return fmt.Errorf("could not update expired contract with DID %s: %w", expiredContract.DID, err)
		}

		state, err := contractstate.NewContractState(expiredContract.State)
		if err != nil {
			return fmt.Errorf("could not create state for expired contract with DID %s: %w", expiredContract.DID, err)
		}

		evt := contractevents.ContractExpired{
			DID:             expiredContract.DID,
			ContractVersion: expiredContract.ContractVersion,
			State:           state,
			ExpPolicy:       policy,
			OccurredAt:      time.Now().UTC(),
		}
		err = event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine)
		if err != nil {
			return fmt.Errorf("could not create event: %w", err)
		}

		switch *policy {
		case expirationpolicy.Renewal:
			if err := insertArchiveAlert(ctx, tx, expiredContract, "RENEWAL_DUE", "Contract renewal is due"); err != nil {
				return err
			}
		case expirationpolicy.Archiving:
			if _, err := tx.ExecContext(ctx, `UPDATE contract_archive_entries SET archive_status='RETAINED' WHERE did=$1 AND archive_status='STORED'`, expiredContract.DID); err != nil {
				return fmt.Errorf("retain expired archive entry %s: %w", expiredContract.DID, err)
			}
		case expirationpolicy.Termination:
			// EXPIRED is the terminal, read-only state required by CSA-14. The
			// policy is preserved on the contract and in ContractExpired rather
			// than silently rewriting legal termination metadata as a user action.
		}
		if err := insertArchiveAlert(ctx, tx, expiredContract, "EXPIRED", "Contract has expired and is no longer active"); err != nil {
			return err
		}

		return tx.Commit()
	}

	ticker := time.NewTicker(interval)
	for range ticker.C {
		if err := materializeUpcomingArchiveAlerts(ctx, db); err != nil {
			log.Printf("could not refresh archive alerts: %v", err)
		}

		expiredContracts, err := readExpiredContracts()
		if err != nil {
			log.Printf("could not read expired contracts: %v", err)
			continue
		}

		if len(expiredContracts) > 0 {
			log.Printf("%d contracts expired", len(expiredContracts))
		}

		for _, expiredContract := range expiredContracts {
			err = callExpirationLogic(expiredContract)
			if err != nil {
				log.Printf("could not call expiration logic for expired contract: %v", err)
			}
		}
	}
}

func insertArchiveAlert(ctx context.Context, tx *sqlx.Tx, contract database.ContractMetadata, alertType, message string) error {
	if contract.ExpDate == nil {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		WITH inserted AS (
			INSERT INTO archive_alerts (did,contract_version,alert_type,due_at,message)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (did,contract_version,alert_type,due_at) DO NOTHING
			RETURNING id,did,contract_version,alert_type,due_at,message
		)
		INSERT INTO outbox_events (component,event_type,event_data,did)
		SELECT $6,'ARCHIVE_ALERT_RAISED',jsonb_build_object(
			'alert_id',id,'did',did,'contract_version',contract_version,
			'alert_type',alert_type,'due_at',due_at,'message',message
		),did FROM inserted`,
		contract.DID, contract.ContractVersion, alertType, contract.ExpDate.UTC(), message,
		componenttype.ContractStorageArchive.String())
	if err != nil {
		return fmt.Errorf("store archive alert for %s: %w", contract.DID, err)
	}
	return nil
}

func materializeUpcomingArchiveAlerts(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `
		WITH inserted AS (
			INSERT INTO archive_alerts (did,contract_version,alert_type,due_at,message)
			SELECT did,contract_version,
				CASE WHEN exp_policy='RENEWAL' THEN 'RENEWAL_DUE' ELSE 'EXPIRY_DUE' END,
				exp_date,
				CASE WHEN exp_policy='RENEWAL' THEN 'Contract renewal is due' ELSE 'Contract approaches its configured expiration date' END
			FROM contracts_archive_metadata
			WHERE exp_date > NOW() AND exp_date <= NOW() + make_interval(days => COALESCE(exp_notice_period,$1))
			ON CONFLICT (did,contract_version,alert_type,due_at) DO NOTHING
			RETURNING id,did,contract_version,alert_type,due_at,message
		)
		INSERT INTO outbox_events (component,event_type,event_data,did)
		SELECT $2,'ARCHIVE_ALERT_RAISED',jsonb_build_object('alert_id',id,'did',did,'contract_version',contract_version,'alert_type',alert_type,'due_at',due_at,'message',message),did
		FROM inserted
	`, baseconf.ArchiveDefaultNoticeDays(), componenttype.ContractStorageArchive.String())
	return err
}
