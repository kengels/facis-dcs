CREATE TABLE IF NOT EXISTS pacm_audit_runs (
    id VARCHAR(96) PRIMARY KEY,
    scope VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    result_status VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ,
    audited_by VARCHAR(255),
    run_data JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_pacm_audit_runs_scope_status_created
    ON pacm_audit_runs(scope, status, created_at);

CREATE TABLE IF NOT EXISTS pacm_audit_findings (
    id VARCHAR(128) PRIMARY KEY,
    audit_run_id VARCHAR(96) NOT NULL REFERENCES pacm_audit_runs(id) ON DELETE CASCADE,
    scope VARCHAR(32) NOT NULL,
    check_name VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    component VARCHAR(64) NOT NULL,
    did VARCHAR(255),
    evidence JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pacm_audit_findings_run
    ON pacm_audit_findings(audit_run_id);

CREATE TABLE IF NOT EXISTS pacm_audit_events (
    id BIGSERIAL PRIMARY KEY,
    audit_run_id VARCHAR(96) NOT NULL REFERENCES pacm_audit_runs(id) ON DELETE CASCADE,
    event_type VARCHAR(64) NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pacm_audit_events_run
    ON pacm_audit_events(audit_run_id);

CREATE TABLE IF NOT EXISTS pacm_audit_reports (
    id VARCHAR(96) PRIMARY KEY,
    audit_run_id VARCHAR(96) NOT NULL REFERENCES pacm_audit_runs(id) ON DELETE CASCADE,
    format VARCHAR(16) NOT NULL,
    content_hash VARCHAR(128) NOT NULL,
    report_data JSONB NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pacm_audit_reports_run
    ON pacm_audit_reports(audit_run_id, generated_at DESC);
