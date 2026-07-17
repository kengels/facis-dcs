// Package conf centralizes timing/topic configuration shared across domains
// (transaction timeouts, outbox/cron polling intervals, the event-bus topic
// name), so these values are changed in one place rather than duplicated
// per domain.
package conf

import (
	"os"
	"strconv"
	"time"
)

func TransactionTimeout() time.Duration {
	return 1 * time.Minute
}

func HTTPClientTimeout() time.Duration {
	return 1 * time.Minute
}

func OutboxProcessorTimeOut() time.Duration {
	return 1 * time.Second
}

// OutboxPublishTimeOut is the poll interval for republishing outbox events
// on NATS (see event.OutboxProcessor.startPublishingJob): much tighter than
// OutboxProcessorTimeOut because publishing is a cheap, single NATS call per
// event, unlike the TSA/IPFS round-trips the (slower) anchoring loop does.
func OutboxPublishTimeOut() time.Duration {
	return 100 * time.Millisecond
}

func EventBusTopic() string {
	return "dcs"
}

func GlobalAuditTrailName() string {
	return "GLOBAL_AUDIT_TRAIL"
}

func ArchiveDashboardRecentActionsLimit() int {
	return 50
}

// ArchiveDefaultNoticeDays is used only when a contract does not provide its
// own exp_notice_period. Invalid configuration falls back to the documented
// development default instead of disabling expiration monitoring.
func ArchiveDefaultNoticeDays() int {
	const fallback = 30
	value := os.Getenv("DCS_ARCHIVE_DEFAULT_NOTICE_DAYS")
	if value == "" {
		return fallback
	}
	days, err := strconv.Atoi(value)
	if err != nil || days < 0 || days > 3650 {
		return fallback
	}
	return days
}

func LoginAttemptsThresholdInDuration() int {
	return 5
}

func LoginLockoutDuration() time.Duration {
	return 15 * time.Minute
}

func SyncFailCronJobTimeOut() time.Duration {
	return 24 * time.Hour
}
