import type { AuditReportRequest, AuditRequest, AuditRunListRequest } from '@/models/requests/auditing-request'
import type { AuditReportResponse, AuditResponse, AuditRun } from '@/models/responses/auditing-response'

export interface AuditingService {
  audit: (request: AuditRequest) => Promise<AuditResponse>
  listRuns: (request?: AuditRunListRequest) => Promise<AuditRun[]>
  report: (request: AuditReportRequest) => Promise<AuditReportResponse>
}
