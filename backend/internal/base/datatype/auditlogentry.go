// Package datatype holds base-level, domain-agnostic value types shared by
// every domain (audit log entries, outbox events, JSON wrappers, pagination).
package datatype

import (
	"encoding/json"
	"time"
)

// AuditLogEntry is one entry of the tamper-evident audit trail. ResLogPredCID
// and GlobalLogPredCID chain this entry to the previous IPFS-anchored entry
// for the same resource and globally, respectively — retroactively editing
// an entry breaks the chain and is therefore detectable. See
// base/event.OutboxProcessor, which builds and anchors these entries.
type AuditLogEntry struct {
	ID               int64           `db:"id" json:"id"`
	Component        string          `db:"component" json:"component"`
	EventType        string          `db:"event_type" json:"event_type"`
	EventData        json.RawMessage `db:"event_data" json:"event_data"`
	DID              *string         `db:"did" json:"did"`
	CreatedAt        time.Time       `db:"created_at" json:"created_at"`
	ResLogPredCID    *string         `db:"res_log_pred_cid" json:"res_log_pred_cid"`
	GlobalLogPredCID *string         `db:"global_log_pred_cid" json:"global_log_pred_cid"`
}
