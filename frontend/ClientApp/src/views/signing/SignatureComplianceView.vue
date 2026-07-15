<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  type SignatureAuditEntry,
  type SignatureComplianceResult,
  signatureManagementService,
  type SignatureView,
} from '@/services/signature-management-service'
import { useAuthStore } from '@/stores/auth-store'

const route = useRoute()
const authStore = useAuthStore()
const did = computed(() => String(route.params.did ?? route.query.did ?? ''))
const canManage = computed(() => authStore.user?.roles.includes('COMPLIANCE_OFFICER') ?? false)
const view = ref<SignatureView | null>(null)
const compliance = ref<SignatureComplianceResult | null>(null)
const audit = ref<SignatureAuditEntry[]>([])
const revocationReason = ref('')
const error = ref('')

async function load() {
  error.value = ''
  try {
    view.value = await signatureManagementService.viewSignatures(did.value)
    audit.value = await signatureManagementService.getAudit(did.value)
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
  await signatureManagementService.revokeSignature(did.value, signature.signer_did, revocationReason.value.trim())
  await load()
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
      <p data-test-id="signature-status">{{ view.contract_state }}</p>
      <article
        v-for="signature in view.signatures"
        :key="`${signature.signer_did}:${signature.field_name}`"
        class="card my-3 bg-base-200 p-4"
        :data-test-key="signature.field_name ?? signature.signer_did"
      >
        <p data-test-id="signature-compliance-trust-anchor">
          {{ signature.signer_did }} · {{ signature.credential_type }}
        </p>
        <p data-test-id="signature-compliance-proof">
          {{ signature.format }} · {{ view.integrity_findings.join(', ') || 'cryptographic integrity valid' }}
        </p>
        <time data-test-id="signature-compliance-timestamp">{{ signature.signed_at }}</time>
      </article>
      <p v-if="compliance" data-test-id="signature-compliance-result">
        {{ compliance.findings?.length ? compliance.findings.join(', ') : 'compliant' }}
      </p>
      <p v-else data-test-id="signature-compliance-result">Compliance check not run</p>
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
          :disabled="!revocationReason.trim()"
          @click="revoke"
        >
          Revoke
        </button>
      </div>
    </template>
  </main>
</template>
