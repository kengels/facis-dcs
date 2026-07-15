import http from '@/api/http'
import type { Contract } from '@/models/contract/contract'

export interface ArchiveContract extends Contract {
  archive_summary?: string
  archive_tags?: string[]
  evidence?: unknown
}

export interface ArchiveAuditEntry {
  did: string
  component: string
  created_at: string
  audit_trail: unknown[]
}

export const archiveService = {
  async retrieve(): Promise<ArchiveContract[]> {
    return http.get<{ contracts?: ArchiveContract[] }>('/archive/retrieve').then((res) => res.data.contracts ?? [])
  },
  async search(params: { name?: string; state?: string; tag?: string }): Promise<ArchiveContract[]> {
    return http.get<ArchiveContract[]>('/archive/search', { params }).then((res) => res.data)
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
}
