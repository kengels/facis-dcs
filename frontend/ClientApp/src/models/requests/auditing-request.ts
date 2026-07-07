export type AuditScope = 'templates' | 'contracts' | 'signatures' | 'archive'
export type AuditReportFormat = 'json' | 'csv' | 'pdf'

export interface AuditRequest {
  scope: AuditScope
}

export interface AuditReportRequest extends AuditRequest {
  auditRunId?: string
  format?: AuditReportFormat
  did?: string
}

export interface AuditRunListRequest {
  scope?: AuditScope
  status?: string
  from?: string
  to?: string
  auditRunId?: string
  includeEvents?: boolean
}
