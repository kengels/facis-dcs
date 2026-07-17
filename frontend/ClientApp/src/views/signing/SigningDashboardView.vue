<script setup lang="ts">
import { onMounted, ref, useTemplateRef } from 'vue'
import { useRoute } from 'vue-router'
import SigningCeremonyDialog from '@/components/signing/SigningCeremonyDialog.vue'
import { useContractPermissions } from '@/modules/contract-workflow-engine/composables/useContractPermissions'
import { contractWorkflowService } from '@/services/contract-workflow-service'
import {
  type SignatureComplianceResult,
  type SignatureContract,
  type SignatureEnvelope,
  signatureManagementService,
  type SignatureValidateResult,
  type SignatureVerifyResult,
  type SigningTask,
} from '@/services/signature-management-service'

const ceremonyDialog = useTemplateRef<InstanceType<typeof SigningCeremonyDialog>>('ceremony-dialog')
const route = useRoute()

const contracts = ref<SignatureContract[]>([])
const signingTasks = ref<SigningTask[]>([])
const selectedContractDid = ref<string | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

const { isManager, isSigner } = useContractPermissions()

// Per-contract state: signing in progress, result envelope, verify result.
const signing = ref<Record<string, boolean>>({})
const envelopes = ref<Record<string, SignatureEnvelope | undefined>>({})
const verifyResults = ref<Record<string, SignatureVerifyResult | undefined>>({})
const validateResults = ref<Record<string, SignatureValidateResult | undefined>>({})
const complianceResults = ref<Record<string, SignatureComplianceResult | undefined>>({})
const verifiedSigners = ref<Record<string, string | undefined>>({})
const completionTimestamps = ref<Record<string, string | undefined>>({})
const pdfVerifyResults = ref<
  Record<
    string,
    | {
        lifecycle_status?: string
        status_list_status?: string
      }
    | undefined
  >
>({})

// DCS-OR-C2PA-006: derive human-readable C2PA lifecycle banner label and CSS class
// from the contract state. Returns one of: Active, Draft, Suspended, Terminated,
// Replaced, Expired, or the raw state as fallback.
interface C2PAStatus {
  label: string
  cls: string
}
function c2paStatus(contract: SignatureContract): C2PAStatus {
  const pdfVerify = pdfVerifyResults.value[contract.did]
  const lifecycle = (pdfVerify?.lifecycle_status ?? '').toLowerCase()
  const statusList = (pdfVerify?.status_list_status ?? '').toLowerCase()
  if (statusList === 'revoked') {
    return { label: 'Suspended', cls: 'badge-warning' }
  }

  const lifecycleMap: Record<string, C2PAStatus> = {
    active: { label: 'Active', cls: 'badge-success' },
    draft: { label: 'Draft', cls: 'badge-ghost' },
    suspended: { label: 'Suspended', cls: 'badge-warning' },
    terminated: { label: 'Terminated', cls: 'badge-error' },
    replaced: { label: 'Replaced', cls: 'badge-neutral' },
    expired: { label: 'Expired', cls: 'badge-neutral' },
  }
  if (lifecycleMap[lifecycle]) {
    return lifecycleMap[lifecycle]
  }

  const state = (contract.state ?? '').toLowerCase()
  const map: Record<string, C2PAStatus> = {
    active: { label: 'Active', cls: 'badge-success' },
    approved: { label: 'Draft', cls: 'badge-ghost' },
    draft: { label: 'Draft', cls: 'badge-ghost' },
    signed: { label: 'Active', cls: 'badge-success' },
    suspended: { label: 'Suspended', cls: 'badge-warning' },
    revoked: { label: 'Suspended', cls: 'badge-warning' },
    terminated: { label: 'Terminated', cls: 'badge-error' },
    replaced: { label: 'Replaced', cls: 'badge-neutral' },
    expired: { label: 'Expired', cls: 'badge-neutral' },
    amended: { label: 'Active', cls: 'badge-success' },
  }
  return map[state] ?? { label: contract.state ?? 'Unknown', cls: 'badge-ghost' }
}

onMounted(async () => {
  loading.value = true
  try {
    const dashboard = await signatureManagementService.retrieveContracts()
    contracts.value = dashboard.contracts
    signingTasks.value = dashboard.signingTasks
    if (route.params.did) selectedContractDid.value = String(route.params.did)
  } catch {
    error.value = 'Failed to load contracts for signing.'
  } finally {
    loading.value = false
  }
})

function tasksFor(contractDid: string): SigningTask[] {
  return signingTasks.value.filter((task) => task.did === contractDid).sort((a, b) => a.order - b.order)
}

function dependencySatisfied(task: SigningTask): boolean {
  if (!task.dependency) return true
  return tasksFor(task.did).some(
    (candidate) => candidate.field_name === task.dependency && candidate.state === 'SIGNED',
  )
}

function ceremonyDependencySatisfied(task: SigningTask): boolean {
  if (!task.dependency) return true
  return dependencySatisfied(task) || Boolean(verifiedSigners.value[`${task.did}:${task.dependency}`])
}

function openTask(contract: SignatureContract) {
  selectedContractDid.value = contract.did
}

async function sign(contract: SignatureContract, task: SigningTask) {
  signing.value[contract.did] = true
  try {
    const outcome = await ceremonyDialog.value?.reveal({
      contractDid: contract.did,
      fieldName: task.field_name,
    })
    if (!outcome || outcome.isCanceled || !outcome.data) {
      return
    }
    verifiedSigners.value[`${contract.did}:${task.field_name}`] = outcome.data.signerDid
  } catch (e: unknown) {
    error.value = `Failed to sign contract ${contract.did}: ${e instanceof Error ? e.message : String(e)}`
  } finally {
    signing.value[contract.did] = false
  }
}

async function applyVerifiedSignature(contract: SignatureContract, task: SigningTask) {
  const signerDid = verifiedSigners.value[`${contract.did}:${task.field_name}`]
  if (!signerDid) return
  signing.value[contract.did] = true
  try {
    const env = await signatureManagementService.applySignature(contract.did, signerDid, task.field_name, 'AES')
    envelopes.value[contract.did] = env
    task.state = env?.status ?? task.state
    completionTimestamps.value[`${contract.did}:${task.field_name}`] = env?.signed_at
  } catch (e: unknown) {
    error.value = `Failed to sign contract ${contract.did}: ${e instanceof Error ? e.message : String(e)}`
  } finally {
    signing.value[contract.did] = false
  }
}

async function verify(contract: SignatureContract) {
  try {
    verifyResults.value[contract.did] = await signatureManagementService.verifySignature(contract.did)
    pdfVerifyResults.value[contract.did] = await contractWorkflowService.verifyPdf(contract.did)
  } catch (e: unknown) {
    error.value = `Failed to verify contract ${contract.did}: ${e instanceof Error ? e.message : String(e)}`
  }
}

async function validate(contract: SignatureContract) {
  try {
    validateResults.value[contract.did] = await signatureManagementService.validateSignature(contract.did)
  } catch (e: unknown) {
    error.value = `Failed to validate contract ${contract.did}: ${e instanceof Error ? e.message : String(e)}`
  }
}

async function compliance(contract: SignatureContract) {
  try {
    complianceResults.value[contract.did] = await signatureManagementService.complianceCheck(contract.did)
  } catch (e: unknown) {
    error.value = `Failed to run compliance check for ${contract.did}: ${e instanceof Error ? e.message : String(e)}`
  }
}
</script>

<template>
  <div class="mb-4 flex justify-between border-b border-base-content/10 bg-base-100 p-4">
    <h2 class="text-2xl/7 font-bold sm:truncate sm:text-3xl sm:tracking-tight">Signing Dashboard</h2>
  </div>

  <div class="p-4">
    <div v-if="loading" class="text-base-content/60">Loading approved contracts…</div>
    <div v-else-if="error" class="mb-4 alert alert-error">{{ error }}</div>
    <div v-else-if="contracts.length === 0" class="text-base-content/60">
      Nothing awaits your signature. Contracts appear here once they are approved for signing.
    </div>

    <div v-else class="overflow-x-auto">
      <table class="table w-full table-zebra">
        <thead>
          <tr>
            <th>DID</th>
            <th>Name</th>
            <th>Version</th>
            <th>Updated</th>
            <th>C2PA Status</th>
            <th>Signature</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="contract in contracts"
            :key="contract.did"
            data-test-id="signing-task-row"
            :data-test-key="contract.did"
          >
            <td class="max-w-xs truncate font-mono text-xs">{{ contract.did }}</td>
            <td>{{ contract.name ?? '—' }}</td>
            <td>{{ contract.contract_version ?? 1 }}</td>
            <td>{{ new Date(contract.updated_at).toLocaleDateString() }}</td>
            <!-- DCS-OR-C2PA-006: C2PA lifecycle status banner -->
            <td>
              <span :class="['badge', 'badge-sm', c2paStatus(contract).cls]">
                {{ c2paStatus(contract).label }}
              </span>
            </td>
            <td>
              <span
                v-if="envelopes[contract.did]"
                :class="['badge', envelopes[contract.did]?.status === 'SIGNED' ? 'badge-success' : 'badge-warning']"
              >
                {{ envelopes[contract.did]?.status }}
              </span>
              <span v-else class="badge badge-ghost">UNSIGNED</span>

              <div
                v-if="verifyResults[contract.did]"
                data-test-id="signature-integrity-result"
                class="mt-1 text-xs"
                :class="verifyResults[contract.did]?.integrity_status === 'VALID' ? 'text-success' : 'text-error'"
              >
                {{ verifyResults[contract.did]?.integrity_status.toLowerCase() }} · MR/HR:
                {{ verifyResults[contract.did]?.match ? 'match ✓' : 'mismatch ✗' }} ({{
                  verifyResults[contract.did]?.sig_count
                }}
                sig(s))
              </div>
              <div v-if="verifyResults[contract.did]" data-test-id="signature-envelope-result" class="mt-1 text-xs">
                {{ verifyResults[contract.did]?.envelope_status.toLowerCase() }} ·
                {{ verifyResults[contract.did]?.sig_count }} signature(s)
              </div>
              <div v-if="verifyResults[contract.did]?.findings?.length" class="mt-1 text-xs">
                Verify: {{ verifyResults[contract.did]?.findings?.[0] }}
              </div>

              <div v-if="validateResults[contract.did]" data-test-id="signature-validation-result" class="mt-1 text-xs">
                {{ validateResults[contract.did]?.status.toLowerCase() }}
              </div>
              <div
                v-if="(validateResults[contract.did]?.findings?.length ?? 0) > 0"
                data-test-id="signature-validation-errors"
                class="mt-1 text-xs text-error"
              >
                {{ validateResults[contract.did]?.findings?.join(', ') }}
              </div>
              <div v-if="complianceResults[contract.did]?.findings?.length" class="mt-1 text-xs">
                Compliance: {{ complianceResults[contract.did]?.findings?.[0] }}
              </div>
              <div v-for="task in tasksFor(contract.did)" :key="task.field_name" class="mt-2">
                <span data-test-id="signing-declared-field" :data-test-key="task.field_name">
                  {{ task.field_name }}
                </span>
                <span class="ml-2 badge badge-sm" data-test-id="signing-task-status" :data-test-key="task.field_name">
                  {{ task.state }}
                </span>
                <span class="ml-2" data-test-id="signing-task-order" :data-test-key="task.field_name">
                  Order {{ task.order }}
                </span>
                <span class="ml-2" data-test-id="signing-task-dependency" :data-test-key="task.field_name">
                  Depends on {{ task.dependency ?? 'none' }}
                </span>
                <span class="ml-2" data-test-id="signing-task-deadline" :data-test-key="task.field_name">
                  Deadline {{ task.deadline ? new Date(task.deadline).toLocaleString() : 'not declared' }}
                </span>
                <time
                  v-if="completionTimestamps[`${contract.did}:${task.field_name}`] || task.signed_at"
                  data-test-id="signing-completion-timestamp"
                  :data-test-key="task.field_name"
                >
                  {{ completionTimestamps[`${contract.did}:${task.field_name}`] || task.signed_at }}
                </time>
              </div>
            </td>
            <td class="flex gap-2">
              <button
                class="btn btn-sm btn-primary"
                data-test-id="signing-task-open"
                :data-test-key="contract.did"
                :disabled="!isSigner && !isManager"
                @click="openTask(contract)"
              >
                Open
              </button>
              <button
                class="btn btn-outline btn-sm"
                data-test-id="signature-integrity-verify"
                :disabled="!isManager"
                @click="verify(contract)"
              >
                Verify
              </button>
              <button
                class="btn btn-outline btn-sm"
                data-test-id="signature-applied-validate"
                :disabled="!isManager"
                @click="validate(contract)"
              >
                Validate
              </button>
              <button class="btn btn-outline btn-sm" :disabled="!isSigner" @click="compliance(contract)">
                Compliance
              </button>
            </td>
          </tr>
          <tr v-if="selectedContractDid" data-test-id="signing-contract-viewer">
            <td colspan="7">
              <div class="flex flex-wrap gap-2">
                <button
                  v-for="task in tasksFor(selectedContractDid)"
                  :key="task.field_name"
                  class="btn btn-sm btn-primary"
                  data-test-id="signing-ceremony-start"
                  :data-test-key="task.field_name"
                  :disabled="
                    !isSigner ||
                    signing[selectedContractDid] ||
                    task.state === 'SIGNED' ||
                    !ceremonyDependencySatisfied(task)
                  "
                  @click="sign(contracts.find((item) => item.did === selectedContractDid)!, task)"
                >
                  Sign {{ task.field_name }}
                </button>
                <button
                  v-for="task in tasksFor(selectedContractDid)"
                  :key="`apply-${task.field_name}`"
                  class="btn btn-sm btn-success"
                  data-test-id="signing-apply-signature"
                  :data-test-key="task.field_name"
                  :disabled="
                    !verifiedSigners[`${selectedContractDid}:${task.field_name}`] ||
                    signing[selectedContractDid] ||
                    !dependencySatisfied(task)
                  "
                  @click="applyVerifiedSignature(contracts.find((item) => item.did === selectedContractDid)!, task)"
                >
                  Apply {{ task.field_name }}
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>

  <SigningCeremonyDialog ref="ceremony-dialog" />
</template>
