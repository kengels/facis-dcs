-- Expose the immutable archive proof envelope through the existing archive
-- read model. The archived snapshot remains unchanged; this view only combines
-- already persisted proof columns into the API's evidence object.
CREATE OR REPLACE VIEW contracts_archive_metadata AS
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
            ELSE jsonb_build_object(
                'deployment', jsonb_strip_nulls(jsonb_build_object(
                    'correlation_id', d.correlation_id,
                    'payload_hash', d.content_hash,
                    'status', d.status,
                    'target_url', d.target_url,
                    'dispatched_at', to_char(d.requested_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
                    'receipt_hash', d.receipt_hash,
                    'tsa_token', d.tsa_token,
                    'activated_at', to_char(d.acknowledged_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
                ))
            )
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
