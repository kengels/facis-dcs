-- Complete the archive read model required by DCS-FR-CSA-04/-05/-10/-12,
-- -19/-20/-21/-22/-23/-26.  The immutable contract/evidence snapshot stays
-- in contract_archive_entries; all mutable operational state lives in
-- dedicated tables.

ALTER TABLE contract_archive_entries
    ADD COLUMN parent_contract_did VARCHAR(255),
    ADD COLUMN parties JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN contract_type TEXT,
    ADD COLUMN jurisdiction TEXT,
    ADD COLUMN compliance_status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK (compliance_status IN ('PENDING', 'COMPLIANT', 'NON_COMPLIANT'));

CREATE INDEX idx_contract_archive_parent ON contract_archive_entries (parent_contract_did);
CREATE INDEX idx_contract_archive_parties ON contract_archive_entries USING GIN (parties);
CREATE INDEX idx_contract_archive_contract_type ON contract_archive_entries (contract_type);
CREATE INDEX idx_contract_archive_jurisdiction ON contract_archive_entries (jurisdiction);
CREATE INDEX idx_contract_archive_validity ON contract_archive_entries (did, contract_version, stored_at);

-- Recover index data for archive entries created before this migration.  The
-- snapshot is authoritative, so this does not depend on the mutable live row.
UPDATE contract_archive_entries
SET parent_contract_did = NULLIF(contract_snapshot->'contract_data'->'dcs:parentContract'->>'@id', ''),
    parties = COALESCE(contract_snapshot->'contract_data'->'dcs:parties', '[]'::jsonb),
    contract_type = COALESCE(
        contract_snapshot->'contract_data'->>'dcs:contractType',
        contract_snapshot->'contract_data'->>'@type'
    ),
    jurisdiction = COALESCE(
        contract_snapshot->'contract_data'->'dcs:metadata'->>'dcs:jurisdiction',
        contract_snapshot->'contract_data'->>'jurisdiction'
    ),
    compliance_status = CASE
        WHEN content_hash ~ '^sha256:[a-f0-9]{64}$' AND snapshot_cid <> ''
            THEN 'COMPLIANT'
        ELSE 'NON_COMPLIANT'
    END;

CREATE TABLE archive_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    did VARCHAR(255) NOT NULL,
    contract_version INT NOT NULL,
    alert_type TEXT NOT NULL CHECK (alert_type IN (
        'EXPIRY_DUE', 'EXPIRED', 'RENEWAL_DUE', 'RETENTION_DUE',
        'COMPLIANCE_FAILED', 'EVIDENCE_MISSING'
    )),
    due_at TIMESTAMP,
    message TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    acknowledged_at TIMESTAMP,
    acknowledged_by VARCHAR(255),
    CONSTRAINT fk_archive_alert_entry FOREIGN KEY (did, contract_version)
        REFERENCES contract_archive_entries (did, contract_version),
    CONSTRAINT uq_archive_alert UNIQUE (did, contract_version, alert_type, due_at),
    CONSTRAINT chk_archive_alert_ack CHECK (
        (acknowledged_at IS NULL AND acknowledged_by IS NULL)
        OR (acknowledged_at IS NOT NULL AND acknowledged_by IS NOT NULL AND acknowledged_by <> '')
    )
);

CREATE INDEX idx_archive_alerts_open ON archive_alerts (created_at DESC)
    WHERE acknowledged_at IS NULL;
CREATE INDEX idx_archive_alerts_did ON archive_alerts (did, contract_version);

CREATE TABLE archive_saved_queries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id VARCHAR(255) NOT NULL,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    filters JSONB NOT NULL CHECK (jsonb_typeof(filters) = 'object'),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_archive_saved_query_name UNIQUE (owner_id, name)
);

CREATE INDEX idx_archive_saved_queries_owner ON archive_saved_queries (owner_id, name);

CREATE TABLE archive_monitoring_preferences (
    owner_id VARCHAR(255) PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    default_notice_days INT NOT NULL DEFAULT 30 CHECK (default_notice_days BETWEEN 0 AND 3650),
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE archive_components (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    did VARCHAR(255) NOT NULL,
    contract_version INT NOT NULL,
    component_iri TEXT NOT NULL,
    party_ids JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(party_ids) = 'array'),
    component_snapshot JSONB NOT NULL,
    content_hash TEXT NOT NULL CHECK (content_hash ~ '^sha256:[a-f0-9]{64}$'),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_archive_component_entry FOREIGN KEY (did, contract_version)
        REFERENCES contract_archive_entries (did, contract_version),
    CONSTRAINT uq_archive_component UNIQUE (did, contract_version, component_iri)
);

CREATE INDEX idx_archive_components_contract ON archive_components (did, contract_version);
CREATE INDEX idx_archive_components_parties ON archive_components USING GIN (party_ids);

-- Archive metadata must come from the frozen snapshot. Recreate the read view
-- without consulting mutable JSON-LD for hierarchy or indexed facets.
DROP VIEW IF EXISTS contracts_archive_metadata;
CREATE VIEW contracts_archive_metadata AS
SELECT
    c.did,
    c.created_by,
    c.created_at,
    c.updated_at,
    c.start_date,
    c.exp_date,
    c.exp_policy,
    c.exp_notice_period,
    c.state,
    c.contract_version,
    c.name,
    c.description,
    c.search_vector,
    c.responsible,
    c.template_did,
    c.template_version,
    a.parent_contract_did,
    a.parties AS archive_parties,
    a.contract_type AS archive_contract_type,
    a.jurisdiction AS archive_jurisdiction,
    a.compliance_status AS archive_compliance_status,
    a.retention_until,
    a.summary AS archive_summary,
    a.tags AS archive_tags,
    jsonb_strip_nulls(
        COALESCE(a.evidence, '{}'::jsonb) || jsonb_build_object(
            'content_hash', a.content_hash,
            'snapshot_cid', a.snapshot_cid,
            'signature_metadata', a.signature_metadata,
            'credential_hashes', a.credential_hashes,
            'tsa_receipt', a.tsa_receipt
        ) || CASE
            WHEN d.correlation_id IS NULL THEN '{}'::jsonb
            ELSE jsonb_build_object('deployment', jsonb_strip_nulls(jsonb_build_object(
                'correlation_id', d.correlation_id,
                'payload_hash', d.content_hash,
                'status', d.status,
                'target_url', d.target_url,
                'dispatched_at', to_char(d.requested_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
                'receipt_hash', d.receipt_hash,
                'tsa_token', d.tsa_token,
                'activated_at', to_char(d.acknowledged_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
            )))
        END
    ) AS evidence
FROM contracts_effective c
INNER JOIN contract_archive_entries a
    ON a.did = c.did AND a.contract_version = c.contract_version
LEFT JOIN LATERAL (
    SELECT cd.correlation_id, cd.content_hash, cd.status, cd.target_url, cd.requested_at,
           cd.receipt_hash, cd.tsa_token, cd.acknowledged_at
    FROM contract_deployments cd
    WHERE cd.did = c.did AND cd.contract_version = c.contract_version
    ORDER BY (cd.acknowledged_at IS NOT NULL) DESC, cd.requested_at DESC
    LIMIT 1
) d ON true
WHERE a.archive_status <> 'DELETED';
