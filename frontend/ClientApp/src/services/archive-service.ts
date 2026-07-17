import http from '@/api/http'
import type { Contract } from '@/models/contract/contract'

export interface ArchiveContract extends Contract {
  archive_summary?: string
  archive_tags?: string[]
  evidence?: ArchiveEvidence
}

export interface ArchiveEvidence {
  content_hash?: string
  snapshot_cid?: string
  signature_metadata?: unknown
  credential_hashes?: unknown
  tsa_receipt?: unknown
  deployment?: {
    correlation_id?: string
    payload_hash?: string
    receipt_hash?: string
    tsa_token?: string
    activated_at?: string
  }
}

export interface ArchiveAuditEntry {
  did: string
  component: string
  created_at: string
  audit_trail: ArchiveAuditTrailEntry[]
}

export interface ArchiveAuditTrailEntry {
  event_type: string
  event_data: Record<string, unknown>
  created_at: string
  kind?: string
  result?: string
  rule_id?: string
  reason?: string
}

export interface ArchiveDashboardAction {
  id: number
  did: string
  event_type: string
  occurred_at: string
}

export interface ArchiveDashboardSummary {
  recent_actions: ArchiveDashboardAction[]
  expiring_contracts: ArchiveContract[]
  compliance: {
    archive_entries: number
    entries_with_proof: number
    entries_without_proof: number
    non_compliant_entries: number
  }
  storage_bytes: number
  open_alerts: number
  renewal_due: number
  source: string
  generated_at: string
}

export interface ArchiveSearchFilters {
  did?: string
  name?: string
  state?: string
  tag?: string
  party?: string
  contract_type?: string
  jurisdiction?: string
  parent_did?: string
  valid_from?: string
  valid_to?: string
}

export interface ArchiveAlert {
  id: string
  did: string
  contract_version: number
  alert_type: string
  due_at?: string
  message: string
  created_at: string
  acknowledged_at?: string
  acknowledged_by?: string
}

export interface ArchiveMonitoringPreferences {
  enabled: boolean
  notice_days: number
  updated_at: string
}

export interface ArchiveSavedQuery {
  id: string
  name: string
  filters: ArchiveSearchFilters
  created_at: string
  updated_at: string
}

export interface ArchiveComponent {
  id: string
  did: string
  contract_version: number
  component_iri: string
  party_ids: string[]
  component_snapshot: unknown
  content_hash: string
}

export const archiveService = {
  async retrieve(): Promise<ArchiveContract[]> {
    return http.get<{ contracts?: ArchiveContract[] }>('/archive/retrieve').then((res) => res.data.contracts ?? [])
  },
  async search(params: ArchiveSearchFilters): Promise<ArchiveContract[]> {
    return http.get<ArchiveContract[]>('/archive/search', { params }).then((res) => res.data)
  },
  async dashboard(): Promise<ArchiveDashboardSummary> {
    return http.get<ArchiveDashboardSummary>('/archive/dashboard').then((res) => res.data)
  },
  async alerts(includeAcknowledged = false): Promise<ArchiveAlert[]> {
    return http
      .get<ArchiveAlert[]>('/archive/alerts', { params: { include_acknowledged: includeAcknowledged } })
      .then((res) => res.data)
  },
  async acknowledgeAlert(id: string): Promise<ArchiveAlert> {
    return http.post<ArchiveAlert>(`/archive/alerts/${encodeURIComponent(id)}/acknowledge`).then((res) => res.data)
  },
  async monitoringPreferences(): Promise<ArchiveMonitoringPreferences> {
    return http.get<ArchiveMonitoringPreferences>('/archive/monitoring-preferences').then((res) => res.data)
  },
  async setMonitoringPreferences(enabled: boolean, noticeDays: number): Promise<ArchiveMonitoringPreferences> {
    return http
      .put<ArchiveMonitoringPreferences>('/archive/monitoring-preferences', {
        enabled,
        notice_days: noticeDays,
      })
      .then((res) => res.data)
  },
  async setRetention(did: string, retentionUntil: string, justification: string) {
    return http
      .put<{ did: string; retention_until: string; archive_status: string }>('/archive/retention', {
        did,
        retention_until: retentionUntil,
        justification,
      })
      .then((res) => res.data)
  },
  async savedQueries(): Promise<ArchiveSavedQuery[]> {
    return http.get<ArchiveSavedQuery[]>('/archive/saved-queries').then((res) => res.data)
  },
  async saveQuery(name: string, filters: ArchiveSearchFilters): Promise<ArchiveSavedQuery> {
    return http.post<ArchiveSavedQuery>('/archive/saved-queries', { name, filters }).then((res) => res.data)
  },
  async deleteSavedQuery(id: string): Promise<boolean> {
    return http.delete<boolean>(`/archive/saved-queries/${encodeURIComponent(id)}`).then((res) => res.data)
  },
  async components(did: string): Promise<ArchiveComponent[]> {
    return http.get<ArchiveComponent[]>(`/archive/components/${encodeURIComponent(did)}`).then((res) => res.data)
  },
  async annotate(did: string, summary: string, tags: string[]) {
    return http
      .post<{ did: string; summary: string; tags?: string[] }>('/archive/annotate', { did, summary, tags })
      .then((res) => res.data)
  },
  async remove(did: string, justification: string): Promise<number> {
    return http.delete<number>('/archive/delete', { params: { did, justification } }).then((res) => res.data)
  },
  async audit(did: string, justification: string): Promise<ArchiveAuditEntry[]> {
    return http.get<ArchiveAuditEntry[]>('/archive/audit', { params: { did, justification } }).then((res) => res.data)
  },
  async integrity(did: string): Promise<ArchiveAuditEntry[]> {
    return http
      .post<ArchiveAuditEntry[]>('/pac/audit', {
        scope: 'archive',
        did,
        justification: 'Archive integrity review',
      })
      .then((res) => res.data)
  },
}
