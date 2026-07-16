CREATE TABLE contract_renewal_relations (
    renewal_did VARCHAR(255) PRIMARY KEY REFERENCES contracts(did) ON DELETE CASCADE,
    original_did VARCHAR(255) NOT NULL REFERENCES contracts(did) ON DELETE CASCADE,
    original_contract_version INT NOT NULL CHECK (original_contract_version > 0),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_contract_renewal_not_self CHECK (renewal_did <> original_did)
);

CREATE INDEX idx_contract_renewal_relations_original
    ON contract_renewal_relations (original_did, created_at, renewal_did);
