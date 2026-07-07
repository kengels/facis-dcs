import http from '@/api/http'
import type {
  AuditReportRequest,
  AuditRequest,
  AuditRunListRequest,
  AuditScope,
} from '@/models/requests/auditing-request'
import type {
  AuditFinding,
  AuditReportResponse,
  AuditResponse,
  AuditRun,
  AuditRunResource,
} from '@/models/responses/auditing-response'
import type { AuditingService } from '@/models/services/auditing-service'
import { contractAuditEventDisplayText } from '@/utils/contract-audit-event-display'

interface RawAuditTrailEntry {
  id?: number | string
  component?: string
  event_type?: string
  eventType?: string
  event_data?: unknown
  eventData?: unknown
  did?: string
  created_at?: string
  createdAt?: string
}

interface RawPACAuditRun {
  id?: string
  auditRunId?: string
  runId?: string
  scope?: string
  status?: string
  resultStatus?: string
  createdAt?: string
  startedAt?: string
  completedAt?: string
  auditedBy?: string
  findings?: AuditFinding[]
  events?: { eventType: string; message: string; createdAt: string }[]
  resources?: RawPACAuditResource[]
}

interface RawPACAuditRunList {
  auditRuns?: RawPACAuditRun[]
  audit_runs?: RawPACAuditRun[]
  runs?: RawPACAuditRun[]
  items?: RawPACAuditRun[]
}

interface RawPACAuditResource extends AuditRunResource {
  id?: number | string
  component?: string
  event_type?: string
  eventType?: string
  did?: string
  created_at?: string
  createdAt?: string
  audit_trail?: RawAuditTrailEntry[]
  auditTrail?: RawAuditTrailEntry[]
}

const latestAuditRunIds: Partial<Record<AuditScope, string>> = {}

function normalizeScopeForRequest(scope?: string): AuditScope | undefined {
  switch (scope?.trim().toUpperCase()) {
    case 'TEMPLATE':
    case 'TEMPLATES':
      return 'templates'
    case 'CONTRACT':
    case 'CONTRACTS':
      return 'contracts'
    case 'SIGNATURE':
    case 'SIGNATURES':
      return 'signatures'
    case 'ARCHIVE':
    case 'ARCHIVES':
      return 'archive'
    default:
      return undefined
  }
}

function normalizeRun(data: RawPACAuditRun): AuditRun {
  const scope = normalizeScopeForRequest(data.scope) ?? 'contracts'
  const id = data.id ?? data.auditRunId ?? data.runId ?? ''
  if (id) latestAuditRunIds[scope] = id
  return {
    id,
    scope: data.scope ?? scope,
    status: data.status ?? '',
    resultStatus: data.resultStatus ?? data.status ?? '',
    createdAt: data.createdAt ?? new Date().toISOString(),
    startedAt: data.startedAt ?? data.createdAt ?? new Date().toISOString(),
    completedAt: data.completedAt,
    auditedBy: data.auditedBy ?? '',
    findings: Array.isArray(data.findings)
      ? data.findings.map((finding) => normalizeStoredFinding(finding, id, scope))
      : [],
    events: Array.isArray(data.events) ? data.events : [],
    resources: Array.isArray(data.resources) ? data.resources : [],
  }
}

function extractAuditRuns(data: unknown): AuditRun[] {
  if (Array.isArray(data)) {
    return data.filter(isObjectRecord).map((item) => normalizeRun(item as RawPACAuditRun))
  }
  if (!isObjectRecord(data)) {
    return []
  }
  const list = data as RawPACAuditRunList
  const runs = list.auditRuns ?? list.audit_runs ?? list.runs ?? list.items
  if (Array.isArray(runs)) {
    return runs.map((run) => normalizeRun(run))
  }
  if (data.id || data.auditRunId || data.runId) {
    return [normalizeRun(data)]
  }
  return []
}

const normalizeAuditResponse = (data: AuditResponse | AuditRun | string, scope: AuditScope): AuditResponse => {
  if (isObjectRecord(data)) {
    const run = data as RawPACAuditRun
    const runId = run.id ?? run.auditRunId ?? run.runId
    if (runId) latestAuditRunIds[scope] = runId
    const findings = Array.isArray(run.findings) ? run.findings : []
    const resources = Array.isArray(run.resources)
      ? run.resources.flatMap((resource, index) =>
          normalizeAuditItem(resource as AuditFinding & RawPACAuditResource, index, scope),
        )
      : []
    const events = auditRunEventsToFindings(run.events, runId, scope)
    return [...findings.map((finding) => normalizeStoredFinding(finding, runId, scope)), ...resources, ...events]
  }
  if (!Array.isArray(data)) {
    return []
  }

  return data.flatMap((item, index) => normalizeAuditItem(item as AuditFinding & RawPACAuditResource, index, scope))
}

function normalizeStoredFinding(finding: AuditFinding, runId: string | undefined, scope: AuditScope): AuditFinding {
  const evidence = finding.evidence
  return {
    ...finding,
    id: finding.id,
    auditRunId: finding.auditRunId ?? runId,
    category: finding.category ?? 'compliance_check',
    title: finding.title ?? finding.check ?? 'Audit check',
    description: finding.description ?? finding.message ?? '',
    component: finding.component ?? auditComponentLabel(scope),
    status: finding.status,
    created_at:
      finding.created_at ?? (finding as unknown as { createdAt?: string }).createdAt ?? new Date().toISOString(),
    details: finding.details ?? {
      eventType: 'AuditCheckCompleted',
      eventData: {
        ruleId: finding.check,
        message: finding.message,
        severity: finding.status,
        evidence,
        auditRunId: finding.auditRunId ?? runId,
      },
    },
  }
}

function auditRunEventsToFindings(
  events: RawPACAuditRun['events'] | undefined,
  runId: string | undefined,
  scope: AuditScope,
): AuditFinding[] {
  return Array.isArray(events)
    ? events.map(
        (event, index): AuditFinding => ({
          id: `${runId ?? scope}-audit-run-event-${index}`,
          auditRunId: runId,
          category: 'compliance_check',
          title: event.eventType,
          description: event.message,
          component: 'PROCESS_AUDIT_AND_COMPLIANCE',
          status: 'PASS',
          created_at: event.createdAt,
          details: {
            eventType: event.eventType,
            eventData: { message: event.message, auditRunId: runId, auditRunEvent: true },
          },
        }),
      )
    : []
}

function normalizeAuditItem(
  item: AuditFinding & RawPACAuditResource,
  index: number,
  scope: AuditScope,
): AuditFinding[] {
  const trail = item.audit_trail ?? item.auditTrail
  if (!Array.isArray(trail)) {
    if (!hasAuditPayload(item)) {
      return []
    }
    if (!isVisibleAuditEvent(item.event_type ?? item.eventType)) {
      return []
    }
    return [normalizeFinding(item, index, scope, item.did, item.created_at ?? item.createdAt)]
  }
  if (trail.length === 0) {
    return []
  }
  return trail
    .filter(hasAuditPayload)
    .filter((entry) => isVisibleAuditEvent(entry.event_type ?? entry.eventType))
    .map((entry, entryIndex) =>
      normalizeFinding(
        entry as AuditFinding & RawAuditTrailEntry,
        `${index}-${entry.id ?? entryIndex}`,
        scope,
        entry.did ?? item.did,
        entry.created_at ?? entry.createdAt ?? item.created_at ?? item.createdAt,
        true,
      ),
    )
}

function normalizeFinding(
  item: AuditFinding & RawAuditTrailEntry,
  fallbackId: number | string,
  scope: AuditScope,
  fallbackDid?: string,
  fallbackCreatedAt?: string,
  useFallbackId = false,
): AuditFinding {
  const eventType = item.event_type ?? item.eventType
  const eventData = item.event_data ?? item.eventData
  const policyData = isObjectRecord(eventData) ? eventData : null
  const severity = stringValue(policyData?.severity)
  const status = item.status ?? severity
  const category = item.category ?? categoryFromEvent(eventType, status)
  const objectDid = stringValue(policyData?.objectDid)
  return {
    id: useFallbackId ? fallbackId : (item.id ?? fallbackId),
    category,
    title: item.title ?? stringValue(policyData?.title) ?? contractAuditEventDisplayText(eventType, eventData),
    description: item.description ?? descriptionFromEventData(eventData),
    component: item.component ?? auditComponentLabel(scope),
    status,
    did: item.did ?? objectDid ?? fallbackDid,
    object_name: stringValue(policyData?.objectName),
    object_type: stringValue(policyData?.objectType),
    created_at: item.created_at ?? item.createdAt ?? fallbackCreatedAt ?? new Date().toISOString(),
    details: item.details ?? item,
  }
}

type RawAuditPayload = RawAuditTrailEntry & Pick<Partial<AuditFinding>, 'description' | 'status' | 'title'>

function hasAuditPayload(item: RawAuditPayload): boolean {
  return (
    Boolean(stringValue(item.event_type ?? item.eventType)) ||
    item.event_data != null ||
    item.eventData != null ||
    Boolean(stringValue(item.title)) ||
    Boolean(stringValue(item.description)) ||
    Boolean(stringValue(item.status))
  )
}

function categoryFromEvent(eventType?: string, severity?: string): AuditFinding['category'] {
  const normalizedSeverity = severity?.trim().toLowerCase()
  if (
    normalizedSeverity === 'error' ||
    normalizedSeverity === 'critical' ||
    normalizedSeverity === 'blocking' ||
    normalizedSeverity === 'failed'
  ) {
    return 'violation'
  }
  if (normalizedSeverity === 'warning' || normalizedSeverity === 'warn') {
    return 'inconsistency'
  }
  if (eventType === 'TEMPLATE_POLICY_AUDIT_FINDING') {
    return 'compliance_check'
  }
  return 'compliance_check'
}

function isVisibleAuditEvent(eventType?: string): boolean {
  const normalized = eventType?.trim().toUpperCase()
  if (!normalized) return true
  return !normalized.startsWith('RETRIEVE_') && !normalized.startsWith('SEARCH_')
}

function descriptionFromEventData(eventData: unknown): string {
  if (typeof eventData === 'string') return eventData
  if (!isObjectRecord(eventData)) return ''
  const message = stringValue(eventData.message)
  const ruleId = stringValue(eventData.ruleId)
  const semanticPath = stringValue(eventData.semanticPath)
  const requirement = stringValue(eventData.requirement)
  const actualValue = detailValue(eventData.actualValue)
  const expectedValue = detailValue(eventData.expectedValue)
  const expectedValues = detailValue(eventData.expectedValues)
  const operator = stringValue(eventData.operator)
  const objectName = stringValue(eventData.objectName)
  const objectDid = stringValue(eventData.objectDid)
  const state = stringValue(eventData.state)
  const templateType = stringValue(eventData.templateType)
  const documentNumber = stringValue(eventData.documentNumber)
  const version = typeof eventData.version === 'number' ? String(eventData.version) : stringValue(eventData.version)
  const parts = [
    objectName
      ? `Object: ${objectName}${objectDid ? ` (${objectDid})` : ''}`
      : objectDid
        ? `Object DID: ${objectDid}`
        : '',
    [templateType ? `Type: ${templateType}` : '', state ? `State: ${state}` : ''].filter(Boolean).join(' · '),
    [documentNumber ? `Document: ${documentNumber}` : '', version ? `Version: ${version}` : '']
      .filter(Boolean)
      .join(' · '),
    message,
    requirement ? `Requirement: ${requirement}` : '',
    actualValue ? `Actual value: ${actualValue}` : '',
    expectedValue ? `Expected value: ${expectedValue}` : '',
    expectedValues ? `Expected values: ${expectedValues}` : '',
    operator ? `Operator: ${operator}` : '',
    ruleId ? `Rule: ${ruleId}` : '',
    semanticPath ? `Semantic path: ${semanticPath}` : '',
  ].filter(Boolean)
  if (parts.length) return parts.join('\n')
  return JSON.stringify(eventData, null, 2)
}

function isObjectRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function stringValue(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value : undefined
}

function detailValue(value: unknown): string | undefined {
  if (typeof value === 'string') return value.trim() ? value : undefined
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  if (Array.isArray(value)) {
    const values = value.map((item) => detailValue(item)).filter(Boolean)
    return values.length ? values.join(', ') : undefined
  }
  if (isObjectRecord(value)) return JSON.stringify(value)
  return undefined
}

function auditComponentLabel(scope: AuditScope): string {
  switch (scope) {
    case 'templates':
      return 'Templates'
    case 'contracts':
      return 'Contracts'
    case 'archive':
      return 'Archive'
    case 'signatures':
      return 'Signatures'
  }
}

export const auditingService: AuditingService = {
  async audit(request: AuditRequest) {
    return http
      .post<AuditResponse | AuditRun | string>('/pac/audit', request)
      .then((res) => normalizeAuditResponse(res.data, request.scope))
  },

  async listRuns(request: AuditRunListRequest = {}) {
    return http
      .get<unknown>('/pac/monitor', {
        params: {
          ...request,
          includeEvents: request.includeEvents ? 'true' : undefined,
        },
      })
      .then((res) => extractAuditRuns(res.data))
  },

  async report(request: AuditReportRequest) {
    const auditRunId = request.auditRunId ?? latestAuditRunIds[request.scope]
    if (auditRunId) {
      return http
        .post<AuditReportResponse>('/pac/report', { auditRunId, format: request.format ?? 'json' })
        .then((res) => res.data)
    }
    return http.get<AuditReportResponse>('/pac/report', { params: request }).then((res) => res.data)
  },
}
