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
  }
  source: string
  generated_at: string
}

export const archiveService = {
  async retrieve(): Promise<ArchiveContract[]> {
    return http.get<{ contracts?: ArchiveContract[] }>('/archive/retrieve').then((res) => res.data.contracts ?? [])
  },
  async search(params: { did?: string; name?: string; state?: string; tag?: string }): Promise<ArchiveContract[]> {
    return http.get<ArchiveContract[]>('/archive/search', { params }).then((res) => res.data)
  },
  async dashboard(): Promise<ArchiveDashboardSummary> {
    return http.get<ArchiveDashboardSummary>('/archive/dashboard').then((res) => res.data)
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
