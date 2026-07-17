<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  type ArchiveAlert,
  type ArchiveComponent,
  type ArchiveContract,
  type ArchiveDashboardSummary,
  type ArchiveMonitoringPreferences,
  type ArchiveSavedQuery,
  type ArchiveSearchFilters,
  archiveService,
} from '@/services/archive-service'
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
const party = ref('')
const contractType = ref('')
const jurisdiction = ref('')
const parentDid = ref('')
const validFrom = ref('')
const validTo = ref('')
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
const alerts = ref<ArchiveAlert[]>([])
const monitoringPreferences = ref<ArchiveMonitoringPreferences>({ enabled: true, notice_days: 30, updated_at: '' })
const savedQueries = ref<ArchiveSavedQuery[]>([])
const savedQueryName = ref('')
const components = ref<ArchiveComponent[]>([])
const retentionUntil = ref('')
const retentionJustification = ref('')
const selectedForRenewal = ref<string[]>([])
const bulkRenewalResult = ref('')
let resultRequest = 0
const auditText = (value: unknown) =>
  typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' ? String(value) : ''

async function search() {
  const request = ++resultRequest
  error.value = ''
  searching.value = true
  try {
    const found = await archiveService.search(currentFilters())
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
  components.value = await archiveService.components(contract.did)
  await router.replace({ query: { ...route.query, archiveDid: contract.did } })
}

function toRFC3339Date(value: string, endOfDay = false) {
  if (!value) return undefined
  return `${value}T${endOfDay ? '23:59:59' : '00:00:00'}Z`
}

function currentFilters(): ArchiveSearchFilters {
  return {
    name: query.value.trim() || undefined,
    state: state.value || undefined,
    tag: tag.value.trim() || undefined,
    party: party.value.trim() || undefined,
    contract_type: contractType.value.trim() || undefined,
    jurisdiction: jurisdiction.value.trim() || undefined,
    parent_did: parentDid.value.trim() || undefined,
    valid_from: toRFC3339Date(validFrom.value),
    valid_to: toRFC3339Date(validTo.value, true),
  }
}

function applyFilters(filters: ArchiveSearchFilters) {
  query.value = filters.name ?? ''
  state.value = filters.state ?? ''
  tag.value = filters.tag ?? ''
  party.value = filters.party ?? ''
  contractType.value = filters.contract_type ?? ''
  jurisdiction.value = filters.jurisdiction ?? ''
  parentDid.value = filters.parent_did ?? ''
  validFrom.value = filters.valid_from?.slice(0, 10) ?? ''
  validTo.value = filters.valid_to?.slice(0, 10) ?? ''
  void search()
}

async function saveCurrentQuery() {
  if (!savedQueryName.value.trim()) return
  const saved = await archiveService.saveQuery(savedQueryName.value.trim(), currentFilters())
  savedQueries.value = [
    ...savedQueries.value.filter((item) => item.id !== saved.id && item.name !== saved.name),
    saved,
  ].sort((a, b) => a.name.localeCompare(b.name))
  savedQueryName.value = ''
}

async function deleteSavedQuery(saved: ArchiveSavedQuery) {
  await archiveService.deleteSavedQuery(saved.id)
  savedQueries.value = savedQueries.value.filter((item) => item.id !== saved.id)
}

async function acknowledgeAlert(alert: ArchiveAlert) {
  await archiveService.acknowledgeAlert(alert.id)
  alerts.value = alerts.value.filter((item) => item.id !== alert.id)
  dashboard.value = await archiveService.dashboard()
}

async function saveMonitoringPreferences() {
  monitoringPreferences.value = await archiveService.setMonitoringPreferences(
    monitoringPreferences.value.enabled,
    monitoringPreferences.value.notice_days,
  )
  alerts.value = await archiveService.alerts()
}

async function setRetention() {
  if (!selected.value || !retentionUntil.value || !retentionJustification.value.trim()) return
  await archiveService.setRetention(
    selected.value.did,
    `${retentionUntil.value}T23:59:59Z`,
    retentionJustification.value.trim(),
  )
  retentionJustification.value = ''
  alerts.value = await archiveService.alerts()
}

function toggleRenewalSelection(did: string) {
  selectedForRenewal.value = selectedForRenewal.value.includes(did)
    ? selectedForRenewal.value.filter((item) => item !== did)
    : [...selectedForRenewal.value, did]
}

async function renewSelected() {
  const targets = results.value.filter((contract) => selectedForRenewal.value.includes(contract.did))
  const failures: string[] = []
  let completed = 0
  for (const contract of targets) {
    try {
      await contractWorkflowService.renew({ did: contract.did, updated_at: contract.updated_at })
      completed++
    } catch {
      failures.push(contract.did)
    }
  }
  bulkRenewalResult.value = `${completed} renewed${failures.length ? `; failed: ${failures.join(', ')}` : ''}`
  selectedForRenewal.value = []
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

function exportResults(format: 'json' | 'csv' = 'json') {
  const payload = results.value.map((contract) => ({
    did: contract.did,
    contract_version: contract.contract_version,
    state: contract.state,
    name: contract.name,
    archive_summary: contract.archive_summary,
    archive_tags: contract.archive_tags,
    evidence: contract.evidence,
  }))
  const body =
    format === 'json'
      ? JSON.stringify(payload, null, 2)
      : [
          'did,contract_version,state,name,summary,tags',
          ...payload.map((item) =>
            [
              item.did,
              item.contract_version,
              item.state,
              item.name ?? '',
              item.archive_summary ?? '',
              (item.archive_tags ?? []).join('|'),
            ]
              .map((value) => `"${String(value).replace(/"/g, '""')}"`)
              .join(','),
          ),
        ].join('\n')
  const url = URL.createObjectURL(new Blob([body], { type: format === 'json' ? 'application/json' : 'text/csv' }))
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = `archive-search-results.${format}`
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
    const [initialResults, summaryData, alertData, savedData, preferenceData] = await Promise.all([
      archiveService.retrieve(),
      archiveService.dashboard(),
      archiveService.alerts(),
      archiveService.savedQueries(),
      archiveService.monitoringPreferences(),
    ])
    dashboard.value = summaryData
    alerts.value = alertData
    savedQueries.value = savedData
    monitoringPreferences.value = preferenceData
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
          <p>{{ dashboard.compliance.non_compliant_entries }} non-compliant</p>
          <p>
            {{ dashboard.storage_bytes }} bytes · {{ dashboard.open_alerts }} open alerts ·
            {{ dashboard.renewal_due }} renewals due
          </p>
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
    <section class="my-4 rounded border p-3" data-test-id="archive-alerts">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <h2 class="font-bold">Archive alerts</h2>
        <div class="flex items-center gap-2" data-test-id="archive-monitoring-preferences">
          <label class="flex items-center gap-1">
            <input v-model="monitoringPreferences.enabled" type="checkbox" data-test-id="archive-alerts-enabled" />
            UI alerts
          </label>
          <label>
            Notice days
            <input
              v-model.number="monitoringPreferences.notice_days"
              type="number"
              min="0"
              max="3650"
              class="input-bordered input input-sm w-24"
              data-test-id="archive-alert-notice-days"
            />
          </label>
          <button class="btn btn-sm" data-test-id="archive-alert-preferences-save" @click="saveMonitoringPreferences">
            Save
          </button>
        </div>
      </div>
      <p v-if="alerts.length === 0">No open archive alerts.</p>
      <div
        v-for="alert in alerts"
        :key="alert.id"
        class="flex items-center gap-2"
        data-test-id="archive-alert"
        :data-test-key="alert.did"
      >
        <span>
          {{ alert.alert_type }} · {{ alert.did }} · {{ alert.due_at ?? alert.created_at }} · {{ alert.message }}
        </span>
        <button
          v-if="canManage"
          class="btn btn-xs"
          data-test-id="archive-alert-acknowledge"
          :data-test-key="alert.id"
          @click="acknowledgeAlert(alert)"
        >
          Acknowledge
        </button>
      </div>
    </section>
    <div class="my-4 grid gap-2 md:grid-cols-4">
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
      <input v-model="party" class="input-bordered input" data-test-id="archive-party-filter" placeholder="Party DID" />
      <input
        v-model="contractType"
        class="input-bordered input"
        data-test-id="archive-contract-type-filter"
        placeholder="Contract type"
      />
      <input
        v-model="jurisdiction"
        class="input-bordered input"
        data-test-id="archive-jurisdiction-filter"
        placeholder="Jurisdiction"
      />
      <input
        v-model="parentDid"
        class="input-bordered input"
        data-test-id="archive-parent-filter"
        placeholder="Parent DID"
      />
      <input v-model="validFrom" type="date" class="input-bordered input" data-test-id="archive-valid-from-filter" />
      <input v-model="validTo" type="date" class="input-bordered input" data-test-id="archive-valid-to-filter" />
      <button class="btn btn-primary" data-test-id="archive-search-submit" :disabled="searching" @click="search">
        <span v-if="searching" class="loading loading-sm loading-spinner" data-test-id="archive-search-loading"></span>
        <span v-else>Search</span>
      </button>
      <button
        class="btn"
        data-test-id="archive-search-export"
        :disabled="results.length === 0"
        @click="exportResults('json')"
      >
        Export JSON
      </button>
      <button
        class="btn"
        data-test-id="archive-search-export-csv"
        :disabled="results.length === 0"
        @click="exportResults('csv')"
      >
        Export CSV
      </button>
    </div>
    <section class="my-3 flex flex-wrap items-center gap-2" data-test-id="archive-saved-queries">
      <input
        v-model="savedQueryName"
        class="input-bordered input"
        placeholder="Saved query name"
        data-test-id="archive-saved-query-name"
      />
      <button class="btn" data-test-id="archive-saved-query-save" @click="saveCurrentQuery">Save query</button>
      <span v-for="saved in savedQueries" :key="saved.id" class="join">
        <button
          class="btn join-item btn-sm"
          data-test-id="archive-saved-query-load"
          :data-test-key="saved.id"
          @click="applyFilters(saved.filters)"
        >
          {{ saved.name }}
        </button>
        <button
          class="btn join-item btn-sm"
          data-test-id="archive-saved-query-delete"
          :data-test-key="saved.id"
          @click="deleteSavedQuery(saved)"
        >
          ×
        </button>
      </span>
    </section>
    <section v-if="canManage" class="my-3 flex items-center gap-2">
      <button
        class="btn"
        data-test-id="archive-bulk-renew"
        :disabled="selectedForRenewal.length === 0"
        @click="renewSelected"
      >
        Renew selected
      </button>
      <span data-test-id="archive-bulk-renew-result">{{ bulkRenewalResult }}</span>
    </section>
    <p v-if="error" class="alert alert-error">{{ error }}</p>
    <section
      v-for="contract in results"
      :key="contract.did"
      data-test-id="archive-search-result"
      :data-test-key="contract.did"
      class="my-2 flex items-center gap-3 rounded border p-3"
    >
      <input
        v-if="canManage"
        type="checkbox"
        :checked="selectedForRenewal.includes(contract.did)"
        data-test-id="archive-renew-select"
        :data-test-key="contract.did"
        @change="toggleRenewalSelection(contract.did)"
      />
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
      <section class="mt-3" data-test-id="archive-components">
        <h3 class="font-bold">Archived components</h3>
        <p
          v-for="component in components"
          :key="component.id"
          data-test-id="archive-component"
          :data-test-key="component.component_iri"
        >
          {{ component.component_iri }} · {{ component.party_ids.join(', ') || 'shared' }} ·
          {{ component.content_hash }}
        </p>
      </section>
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
          <input
            v-model="retentionUntil"
            type="date"
            class="input-bordered input"
            data-test-id="archive-retention-until"
          />
          <input
            v-model="retentionJustification"
            class="input-bordered input"
            data-test-id="archive-retention-justification"
            placeholder="Retention justification"
          />
          <button class="btn" data-test-id="archive-retention-save" @click="setRetention">Set retention</button>
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
