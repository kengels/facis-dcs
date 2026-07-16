CREATE TABLE pac_incidents (
    incident_id           TEXT PRIMARY KEY,
    affected_contract_dids TEXT[] NOT NULL,
    affected_template_dids TEXT[] NOT NULL,
    finding_refs          TEXT[] NOT NULL,
    reason                TEXT NOT NULL CHECK (btrim(reason) <> ''),
    reported_by           TEXT NOT NULL,
    reported_at           TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_pac_incidents_reported_at ON pac_incidents (reported_at DESC);
CREATE INDEX idx_pac_incidents_contract_dids ON pac_incidents USING GIN (affected_contract_dids);
CREATE INDEX idx_pac_incidents_template_dids ON pac_incidents USING GIN (affected_template_dids);
