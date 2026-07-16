<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { type ArchiveContract, type ArchiveDashboardSummary, archiveService } from '@/services/archive-service'
import { contractWorkflowService } from '@/services/contract-workflow-service'
import { useAuthStore } from '@/stores/auth-store'
import { ContractState } from '@/types/contract-state'
import { UserRole } from '@/types/user-role'

const authStore = useAuthStore()
const route = useRoute()
const router = useRouter()
const canManage = computed(() => authStore.user?.roles.includes(UserRole.archiveManager) ?? false)
const archiveStates = [ContractState.signed, ContractState.terminated]
const query = ref('')
const state = ref('')
const tag = ref('')
const results = ref<ArchiveContract[]>([])
const selected = ref<ArchiveContract | null>(null)
const summary = ref('')
const tagsText = ref('')
const integrityChecks = ref<{ result: string; ruleId: string; reason: string }[]>([])
const terminationReason = ref('')
const deleteJustification = ref('')
const deletedEntryCount = ref<number | null>(null)
const terminationResult = ref<{ reason: string; actor: string; timestamp: string } | null>(null)
const error = ref('')
const dashboard = ref<ArchiveDashboardSummary | null>(null)
const searching = ref(false)
let resultRequest = 0
const auditText = (value: unknown) =>
  typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' ? String(value) : ''

async function search() {
  const request = ++resultRequest
  error.value = ''
  searching.value = true
  try {
    const found = await archiveService.search({
      name: query.value.trim() || undefined,
      state: state.value || undefined,
      tag: tag.value.trim() || undefined,
    })
    if (request === resultRequest) results.value = found
  } catch (cause: unknown) {
    if (request === resultRequest) {
      error.value = cause instanceof Error ? cause.message : 'Archive search failed.'
    }
  } finally {
    if (request === resultRequest) searching.value = false
  }
}

async function open(contract: ArchiveContract) {
  selected.value = contract
  summary.value = contract.archive_summary ?? ''
  tagsText.value = contract.archive_tags?.join(',') ?? ''
  await router.replace({ query: { ...route.query, archiveDid: contract.did } })
}

async function annotate() {
  if (!selected.value) return
  const annotation = await archiveService.annotate(
    selected.value.did,
    summary.value.trim(),
    tagsText.value
      .split(',')
      .map((tag) => tag.trim())
      .filter(Boolean),
  )
  selected.value.archive_summary = annotation.summary
  selected.value.archive_tags = annotation.tags ?? []
  await reloadSelected()
}

async function auditIntegrity() {
  if (!selected.value) return
  const entries = await archiveService.integrity(selected.value.did)
  integrityChecks.value = entries.flatMap((entry) =>
    entry.audit_trail
      .filter((audit) => audit.kind === 'CHECK' && audit.result && audit.rule_id)
      .map((audit) => ({ result: audit.result!, ruleId: audit.rule_id!, reason: audit.reason ?? '' })),
  )
}

async function reloadSelected() {
  if (!selected.value) return
  const did = selected.value.did
  results.value = await archiveService.search({ did })
  selected.value = results.value.find((contract) => contract.did === did) ?? null
  if (selected.value) {
    summary.value = selected.value.archive_summary ?? ''
    tagsText.value = selected.value.archive_tags?.join(',') ?? ''
  }
}

async function terminate() {
  if (!selected.value || !terminationReason.value.trim()) return
  const terminated = await contractWorkflowService.terminate({
    did: selected.value.did,
    reason: terminationReason.value.trim(),
    updated_at: selected.value.updated_at,
  })
  // The lifecycle command response is authoritative. Reflect it immediately;
  // the immutable archive read model may be updated asynchronously.
  selected.value = { ...selected.value, state: terminated.state }
  const refreshed = await archiveService.retrieve()
  results.value = refreshed
  const archivedContract = refreshed.find((contract) => contract.did === terminated.did)
  selected.value = archivedContract
    ? {
        ...archivedContract,
        state: terminated.state,
      }
    : null
  const entries = await archiveService.audit(terminated.did, 'Review contract termination')
  const termination = entries
    .flatMap((entry) => entry.audit_trail)
    .reverse()
    .find((entry) => entry.event_type === 'TERMINATE_CONTRACT')
  if (termination) {
    terminationResult.value = {
      reason: auditText(termination.event_data.reason),
      actor: auditText(termination.event_data.terminated_by),
      timestamp: termination.created_at,
    }
  }
}

function exportResults() {
  const payload = results.value.map((contract) => ({
    did: contract.did,
    contract_version: contract.contract_version,
    state: contract.state,
    name: contract.name,
    archive_summary: contract.archive_summary,
    archive_tags: contract.archive_tags,
    evidence: contract.evidence,
  }))
  const url = URL.createObjectURL(new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' }))
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = 'archive-search-results.json'
  anchor.click()
  URL.revokeObjectURL(url)
}

async function remove() {
  if (!selected.value || !deleteJustification.value.trim()) return
  deletedEntryCount.value = await archiveService.remove(selected.value.did, deleteJustification.value.trim())
  results.value = await archiveService.retrieve()
  selected.value = null
  await router.replace({ query: { ...route.query, archiveDid: undefined } })
}

onMounted(async () => {
  const request = ++resultRequest
  try {
    const [initialResults, summaryData] = await Promise.all([archiveService.retrieve(), archiveService.dashboard()])
    dashboard.value = summaryData
    if (request !== resultRequest) return
    results.value = initialResults
    const selectedDID = typeof route.query.archiveDid === 'string' ? route.query.archiveDid : ''
    if (selectedDID) {
      const contract = initialResults.find((item) => item.did === selectedDID)
      if (contract) await open(contract)
    }
  } catch (cause: unknown) {
    error.value = cause instanceof Error ? cause.message : 'Archive dashboard failed.'
  }
})
</script>

<template>
  <main class="p-4">
    <h1 class="text-2xl font-bold">Contract Archive</h1>
    <div class="my-4 grid gap-3 md:grid-cols-3">
      <section data-test-id="archive-recent-actions" class="rounded border p-3">
        <h2 class="font-bold">Recent archive actions</h2>
        <p v-if="dashboard?.recent_actions.length === 0">No archive actions recorded.</p>
        <p v-for="action in dashboard?.recent_actions ?? []" :key="action.id" :data-test-key="String(action.id)">
          {{ action.event_type }} · {{ action.did }} · {{ action.occurred_at }}
        </p>
      </section>
      <section data-test-id="archive-compliance-summary" class="rounded border p-3">
        <h2 class="font-bold">Archive proof coverage</h2>
        <template v-if="dashboard">
          <p>{{ dashboard.compliance.entries_with_proof }} / {{ dashboard.compliance.archive_entries }} with proof</p>
          <p>{{ dashboard.compliance.entries_without_proof }} without proof</p>
        </template>
      </section>
      <section class="rounded border p-3">
        <h2 class="font-bold">Expiring contracts</h2>
        <p
          v-for="contract in dashboard?.expiring_contracts ?? []"
          :key="contract.did"
          data-test-id="archive-expiring-contract"
          :data-test-key="contract.did"
        >
          {{ contract.name }} · {{ contract.exp_date }}
        </p>
        <p v-if="dashboard" data-test-id="archive-statistics-source">
          {{ dashboard.source }} · {{ dashboard.generated_at }}
        </p>
      </section>
    </div>
    <div class="my-4 flex gap-2">
      <input
        v-model="query"
        class="input-bordered input"
        data-test-id="archive-search-query"
        aria-label="Archive query"
      />
      <select
        v-model="state"
        class="select-bordered select"
        data-test-id="archive-state-filter"
        aria-label="Archive state"
      >
        <option value="">All states</option>
        <option v-for="archiveState in archiveStates" :key="archiveState" :value="archiveState">
          {{ archiveState }}
        </option>
      </select>
      <input v-model="tag" class="input-bordered input" data-test-id="archive-tag-filter" aria-label="Archive tag" />
      <button class="btn btn-primary" data-test-id="archive-search-submit" :disabled="searching" @click="search">
        <span v-if="searching" class="loading loading-sm loading-spinner" data-test-id="archive-search-loading"></span>
        <span v-else>Search</span>
      </button>
      <button class="btn" data-test-id="archive-search-export" :disabled="results.length === 0" @click="exportResults">
        Export results
      </button>
    </div>
    <p v-if="error" class="alert alert-error">{{ error }}</p>
    <section
      v-for="contract in results"
      :key="contract.did"
      data-test-id="archive-search-result"
      :data-test-key="contract.did"
      class="my-2 flex items-center gap-3 rounded border p-3"
    >
      <span>{{ contract.name }} · {{ contract.state }}</span>
      <button
        class="btn btn-sm"
        data-test-id="archive-result-open"
        :data-test-key="contract.did"
        :disabled="searching"
        @click="open(contract)"
      >
        Open
      </button>
    </section>
    <section v-if="selected" class="mt-6 rounded border p-4" data-test-id="archive-contract-details">
      <h2 class="font-bold">{{ selected.name }}</h2>
      <p data-test-id="archive-contract-id">{{ selected.did }}</p>
      <p data-test-id="archive-state">{{ selected.state }}</p>
      <pre v-if="selected.evidence" data-test-id="archive-signature-evidence" class="mt-2 overflow-auto text-xs">{{
        JSON.stringify(selected.evidence, null, 2)
      }}</pre>
      <p v-if="selected.evidence?.snapshot_cid || selected.evidence?.content_hash" data-test-id="archive-server-proof">
        {{ selected.evidence.snapshot_cid ?? selected.evidence.content_hash }}
      </p>
      <p data-test-id="archive-annotation-summary-result">{{ selected.archive_summary }}</p>
      <span
        v-for="archiveTag in selected.archive_tags"
        :key="archiveTag"
        class="mr-1 badge"
        data-test-id="archive-tag"
        :data-test-key="archiveTag"
      >
        {{ archiveTag }}
      </span>
      <template v-if="canManage">
        <div class="mt-3 flex flex-col gap-2">
          <input
            v-model="summary"
            class="input-bordered input"
            data-test-id="archive-annotation-summary"
            placeholder="Summary"
          />
          <input
            v-model="tagsText"
            class="input-bordered input"
            data-test-id="archive-annotation-tags"
            placeholder="Tags"
          />
          <button class="btn" data-test-id="archive-annotation-save" @click="annotate">Save annotation</button>
          <button class="btn" data-test-id="archive-integrity-audit" @click="auditIntegrity">Audit integrity</button>
          <div v-if="integrityChecks.length" data-test-id="archive-integrity-result">
            <p v-for="check in integrityChecks" :key="check.ruleId">
              {{ check.result }} ·
              <span data-test-id="archive-integrity-rule-reference" :data-test-key="check.ruleId">
                {{ check.ruleId }}
              </span>
              ·
              {{ check.reason }}
            </p>
          </div>
          <input
            v-model="terminationReason"
            class="input-bordered input"
            data-test-id="archive-termination-reason"
            placeholder="Termination reason"
          />
          <button class="btn" data-test-id="archive-terminate" @click="terminate">Terminate</button>
          <div v-if="terminationResult" data-test-id="archive-termination-result">
            {{ terminationResult.reason }}
            <span data-test-id="archive-termination-actor">{{ terminationResult.actor }}</span>
            <time data-test-id="archive-termination-timestamp">{{ terminationResult.timestamp }}</time>
          </div>
          <input
            v-model="deleteJustification"
            class="input-bordered input"
            data-test-id="archive-delete-justification"
            placeholder="Deletion justification"
          />
          <button class="btn btn-error" data-test-id="archive-delete" @click="remove">Delete</button>
        </div>
      </template>
    </section>
    <p v-if="deletedEntryCount !== null" class="mt-3" data-test-id="archive-delete-result">
      Deleted {{ deletedEntryCount }} archive entries
    </p>
  </main>
</template>
