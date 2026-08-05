CREATE TABLE IF NOT EXISTS audit_trail_log (

    id BIGSERIAL PRIMARY KEY,

    component VARCHAR(64) NOT NULL,

    did VARCHAR(255),

    last_log_cid VARCHAR(60),

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);