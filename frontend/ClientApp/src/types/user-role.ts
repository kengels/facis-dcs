export type UserRole = (typeof UserRole)[keyof typeof UserRole]

export const UserRole = {
  templateCreator: 'TEMPLATE_CREATOR',
  templateReviewer: 'TEMPLATE_REVIEWER',
  templateApprover: 'TEMPLATE_APPROVER',
  templateManager: 'TEMPLATE_MANAGER',
  contractCreator: 'CONTRACT_CREATOR',
  contractReviewer: 'CONTRACT_REVIEWER',
  contractApprover: 'CONTRACT_APPROVER',
  contractManager: 'CONTRACT_MANAGER',
  contractNegotiator: 'CONTRACT_NEGOTIATOR',
  contractSigner: 'CONTRACT_SIGNER',
  contractObserver: 'CONTRACT_OBSERVER',
  archiveManager: 'ARCHIVE_MANAGER',
  auditor: 'AUDITOR',
  systemAdministrator: 'SYSTEM_ADMINISTRATOR',
  complianceOfficer: 'COMPLIANCE_OFFICER',
  integrationManager: 'INTEGRATION_MANAGER',
  processOrchestrator: 'PROCESS_ORCHESTRATOR',
  validator: 'VALIDATOR',
} as const

/** Maps access-token role claim labels to UserRole ids. */
const ROLE_LABEL_TO_USER_ROLE: Record<string, UserRole> = {
  'Template Creator': UserRole.templateCreator,
  'Template Reviewer': UserRole.templateReviewer,
  'Template Approver': UserRole.templateApprover,
  'Template Manager': UserRole.templateManager,
  'Contract Creator': UserRole.contractCreator,
  'Contract Reviewer': UserRole.contractReviewer,
  'Contract Approver': UserRole.contractApprover,
  'Contract Manager': UserRole.contractManager,
  'Contract Negotiator': UserRole.contractNegotiator,
  'Contract Signer': UserRole.contractSigner,
  'Contract Observer': UserRole.contractObserver,
  'Archive Manager': UserRole.archiveManager,
  Auditor: UserRole.auditor,
  'Sys. Administrator': UserRole.systemAdministrator,
  'Compliance Officer': UserRole.complianceOfficer,
  'Integration Manager': UserRole.integrationManager,
  'Process Orchestrator': UserRole.processOrchestrator,
  Validator: UserRole.validator,
}

const USER_ROLE_TO_ROLE_LABEL = Object.fromEntries(
  Object.entries(ROLE_LABEL_TO_USER_ROLE).map(([label, role]) => [role, label]),
) as Record<UserRole, string>

export function userRoleLabel(role: UserRole): string {
  return USER_ROLE_TO_ROLE_LABEL[role]
}

/** Reads roles from a JWT payload. */
export function rolesFromJwtPayload(payload: Record<string, unknown> | null | undefined): unknown {
  if (!payload) return []
  if (Array.isArray(payload.roles)) return payload.roles
  const ext = payload.ext
  if (ext && typeof ext === 'object' && Array.isArray((ext as Record<string, unknown>).roles)) {
    return (ext as Record<string, unknown>).roles
  }
  return []
}

export function mapRoleLabelsToUserRoles(roles: unknown): UserRole[] {
  if (!Array.isArray(roles)) return []
  const mapped: UserRole[] = []
  for (const r of roles) {
    if (typeof r !== 'string') continue
    const role = ROLE_LABEL_TO_USER_ROLE[r]
    if (role) mapped.push(role)
  }
  return mapped
}
