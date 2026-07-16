<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  type SignatureAuditEntry,
  type SignatureComplianceResult,
  signatureManagementService,
  type SignatureView,
} from '@/services/signature-management-service'

const route = useRoute()
const did = computed(() => String(route.params.did ?? route.query.did ?? ''))
const canManage = computed(() => view.value?.can_revoke ?? false)
const view = ref<SignatureView | null>(null)
const compliance = ref<SignatureComplianceResult | null>(null)
const audit = ref<SignatureAuditEntry[]>([])
const revocationReason = ref('')
const error = ref('')
const revocationResult = ref<{ reason: string } | null>(null)
const revoking = ref(false)
const signatureStatus = computed(() => {
  if (
    revocationResult.value ||
    view.value?.contract_state.toUpperCase() === 'REVOKED' ||
    view.value?.signatures.some((signature) => signature.status.toUpperCase() === 'REVOKED')
  )
    return 'Revoked'
  return view.value?.contract_state ?? ''
})
const complianceResult = computed(() => {
  if (!compliance.value) return 'Compliance check not run'
  const details = compliance.value.findings?.join(', ')
  const status = compliance.value.status.toLowerCase()
  return details ? `${status}: ${details}` : status
})

async function load() {
  error.value = ''
  try {
    view.value = await signatureManagementService.viewSignatures(did.value)
    audit.value = await signatureManagementService.getAudit(did.value)
    const persistedReason = view.value.signatures.find(
      (signature) => signature.status.toUpperCase() === 'REVOKED' && signature.revocation_reason,
    )?.revocation_reason
    if (persistedReason) revocationResult.value = { reason: persistedReason }
    const revocation = [...audit.value].reverse().find((entry) => entry.event_type === 'REVOKE_SIGNATURE')
    if (revocation && typeof revocation.event_data === 'object' && revocation.event_data !== null) {
      const reason = (revocation.event_data as Record<string, unknown>).reason
      if (typeof reason === 'string') revocationResult.value = { reason }
    }
  } catch (cause: unknown) {
    error.value = cause instanceof Error ? cause.message : 'Could not load signature compliance data.'
  }
}

async function runCompliance() {
  compliance.value = await signatureManagementService.complianceCheck(did.value)
}

async function revoke() {
  const signature = view.value?.signatures.find((item) => item.status === 'SIGNED')
  if (!signature || !revocationReason.value.trim()) return
  error.value = ''
  revoking.value = true
  const reason = revocationReason.value.trim()
  try {
    await signatureManagementService.revokeSignature(did.value, signature.signer_did, reason)
    revocationResult.value = { reason }
    await load()
  } catch (cause: unknown) {
    error.value = cause instanceof Error ? cause.message : 'Could not revoke signature.'
  } finally {
    revoking.value = false
  }
}

function downloadReport() {
  const report = { signature_view: view.value, compliance: compliance.value, audit: audit.value }
  const url = URL.createObjectURL(new Blob([JSON.stringify(report, null, 2)], { type: 'application/json' }))
  const link = document.createElement('a')
  link.href = url
  link.download = `signature-compliance-${encodeURIComponent(did.value)}.json`
  link.click()
  URL.revokeObjectURL(url)
}

onMounted(load)
</script>

<template>
  <main class="p-4">
    <h1 class="mb-4 text-2xl font-bold">Signature Compliance</h1>
    <div v-if="error" class="alert alert-error">{{ error }}</div>
    <template v-if="view">
      <p data-test-id="signature-status">{{ signatureStatus }}</p>
      <section data-test-id="signature-compliance-trust-anchor" class="card my-3 bg-base-200 p-4">
        <p
          v-for="signature in view.signatures"
          :key="`trust:${signature.signer_did}:${signature.field_name}`"
          :data-test-key="signature.field_name ?? signature.signer_did"
        >
          {{ signature.signer_did }} · {{ signature.credential_type }}
        </p>
      </section>
      <section data-test-id="signature-compliance-proof" class="card my-3 bg-base-200 p-4">
        <p
          v-for="signature in view.signatures"
          :key="`proof:${signature.signer_did}:${signature.field_name}`"
          :data-test-key="signature.field_name ?? signature.signer_did"
        >
          {{ signature.format }} · {{ view.integrity_status }}
        </p>
        <p v-for="finding in view.integrity_findings" :key="finding">{{ finding }}</p>
      </section>
      <section data-test-id="signature-compliance-timestamp" class="card my-3 bg-base-200 p-4">
        <time
          v-for="signature in view.signatures"
          :key="`time:${signature.signer_did}:${signature.field_name}`"
          :data-test-key="signature.field_name ?? signature.signer_did"
        >
          {{ signature.signed_at ?? 'Not signed' }}
        </time>
      </section>
      <article
        v-for="signature in view.signatures"
        :key="`${signature.signer_did}:${signature.field_name}`"
        class="card my-3 bg-base-200 p-4"
        :data-test-key="signature.field_name ?? signature.signer_did"
      >
        <p>{{ signature.field_name ?? 'Declared signature' }} · {{ signature.status }}</p>
      </article>
      <p data-test-id="signature-compliance-result">{{ complianceResult }}</p>
      <p v-if="revocationResult" data-test-id="signature-revocation-reason-result">
        {{ revocationResult.reason }}
      </p>
      <p v-if="revocationResult" data-test-id="signature-revocation-success">Revoked</p>
      <div v-if="canManage" class="mt-4 flex flex-wrap gap-2">
        <button class="btn btn-primary" data-test-id="signature-compliance-run" @click="runCompliance">
          Run compliance
        </button>
        <button class="btn" data-test-id="signature-compliance-report" @click="downloadReport">Download report</button>
        <input
          v-model="revocationReason"
          class="input-bordered input"
          data-test-id="signature-revocation-reason"
          placeholder="Revocation reason"
        />
        <button
          class="btn btn-error"
          data-test-id="signature-revoke"
          :disabled="!revocationReason.trim() || revoking"
          @click="revoke"
        >
          {{ revoking ? 'Revoking…' : 'Revoke' }}
        </button>
      </div>
    </template>
  </main>
</template>
