import http from '@/api/http'

export interface SignatureContract {
  did: string
  contract_version?: number
  state: string
  name?: string
  description?: string
  created_at: string
  updated_at: string
}

export interface SigningTask {
  did: string
  contract_version: number
  state: string
  signer: string
  field_name: string
  created_at: string
}

export interface SigningDashboardData {
  contracts: SignatureContract[]
  signingTasks: SigningTask[]
}

export interface SignatureEnvelope {
  contract_did: string
  signer_did: string
  credential_type: string
  status: string
  signed_at?: string
  revoked_at?: string
  ipfs_cid?: string
}

export interface SignatureVerifyResult {
  did: string
  match: boolean
  jsonld_hash?: string
  base_pdf_hash?: string
  sig_count: number
  findings?: string[]
}

export interface SignatureValidateResult {
  did: string
  findings?: string[]
}

export interface SignatureComplianceResult {
  did: string
  findings?: string[]
}

export interface SignatureAuditEntry {
  id: number
  component: string
  event_type: string
  event_data: unknown
  did?: string
  created_at: string
  res_log_pred_cid?: string
  global_log_pred_cid?: string
}

export interface SignatureViewItem {
  signer_did: string
  field_name?: string
  credential_type: string
  status: string
  signed_at?: string
  revoked_at?: string
  format: string
}

export interface SignatureView {
  did: string
  contract_state: string
  signatures: SignatureViewItem[]
  integrity_findings: string[]
}

export type CeremonyStatus = 'pending' | 'verified' | 'expired' | 'failed'

export interface CeremonyStartResult {
  ceremony_id: string
  wallet_uri: string
  expires_at: string
  status: CeremonyStatus
}

export interface CeremonyStatusResult {
  ceremony_id: string
  contract_did: string
  field_name?: string
  status: CeremonyStatus
  signer_did?: string
  expires_at?: string
}

export const signatureManagementService = {
  async retrieveContracts(): Promise<SigningDashboardData> {
    return http
      .get<{ contracts: SignatureContract[]; signing_tasks: SigningTask[] }>('/signature/retrieve')
      .then((res) => ({ contracts: res.data.contracts, signingTasks: res.data.signing_tasks }))
  },

  async startCeremony(contractDid: string, fieldName: string): Promise<CeremonyStartResult> {
    return http
      .post<CeremonyStartResult>('/signature/request', { contract_did: contractDid, field_name: fieldName })
      .then((res) => res.data)
  },

  async getCeremonyStatus(ceremonyId: string): Promise<CeremonyStatusResult> {
    return http.get<CeremonyStatusResult>(`/signature/request/${ceremonyId}`).then((res) => res.data)
  },

  async applySignature(
    did: string,
    signerDid: string,
    fieldName: string,
    credentialType: string,
  ): Promise<SignatureEnvelope | undefined> {
    return http
      .post<{ did: string; signature_envelope?: SignatureEnvelope }>('/signature/apply', {
        did,
        signer_did: signerDid,
        field_name: fieldName,
        credential_type: credentialType,
        updated_at: new Date().toISOString(),
      })
      .then((res) => res.data.signature_envelope)
  },

  async verifySignature(did: string): Promise<SignatureVerifyResult> {
    return http.post<SignatureVerifyResult>('/signature/verify', { did }).then((res) => res.data)
  },

  async validateSignature(did: string): Promise<SignatureValidateResult> {
    return http.post<SignatureValidateResult>('/signature/validate', { did }).then((res) => res.data)
  },

  async complianceCheck(did: string): Promise<SignatureComplianceResult> {
    return http.post<SignatureComplianceResult>('/signature/compliance', { did }).then((res) => res.data)
  },

  async revokeSignature(did: string, signerDid: string, reason: string): Promise<void> {
    await http.post('/signature/revoke', { did, signer_did: signerDid, reason })
  },

  async viewSignatures(did: string): Promise<SignatureView> {
    return http.get<SignatureView>('/signature/view', { params: { did } }).then((res) => res.data)
  },

  async getAudit(did: string): Promise<SignatureAuditEntry[]> {
    return http.get<SignatureAuditEntry[]>('/signature/audit', { params: { did } }).then((res) => res.data ?? [])
  },
}
