<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { type ArchiveContract, archiveService } from '@/services/archive-service'
import { contractWorkflowService } from '@/services/contract-workflow-service'
import { useAuthStore } from '@/stores/auth-store'

const authStore = useAuthStore()
const canManage = computed(() => authStore.user?.roles.includes('ARCHIVE_MANAGER') ?? false)
const query = ref('')
const state = ref('')
const results = ref<ArchiveContract[]>([])
const selected = ref<ArchiveContract | null>(null)
const summary = ref('')
const tagsText = ref('')
const integrityAuditEntryCount = ref<number | null>(null)
const terminationReason = ref('')
const deleteJustification = ref('')
const deletedEntryCount = ref<number | null>(null)
const error = ref('')

async function search() {
  error.value = ''
  try {
    results.value = await archiveService.search({
      name: query.value.trim() || undefined,
      state: state.value || undefined,
    })
  } catch (cause: unknown) {
    error.value = cause instanceof Error ? cause.message : 'Archive search failed.'
  }
}

function open(contract: ArchiveContract) {
  selected.value = contract
  summary.value = contract.archive_summary ?? ''
  tagsText.value = contract.archive_tags?.join(',') ?? ''
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
}

async function auditIntegrity() {
  if (!selected.value) return
  const entries = await archiveService.audit(selected.value.did, 'Archive integrity review')
  integrityAuditEntryCount.value = entries.length
}

async function terminate() {
  if (!selected.value || !terminationReason.value.trim()) return
  const terminated = await contractWorkflowService.terminate({
    did: selected.value.did,
    reason: terminationReason.value.trim(),
    updated_at: selected.value.updated_at,
  })
  const refreshed = await archiveService.retrieve()
  results.value = refreshed
  selected.value = refreshed.find((contract) => contract.did === terminated.did) ?? null
}

async function remove() {
  if (!selected.value || !deleteJustification.value.trim()) return
  deletedEntryCount.value = await archiveService.remove(selected.value.did, deleteJustification.value.trim())
  results.value = await archiveService.retrieve()
  selected.value = null
}

onMounted(async () => {
  results.value = await archiveService.retrieve()
})
</script>

<template>
  <main class="p-4">
    <h1 class="text-2xl font-bold">Contract Archive</h1>
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
        <option value="SIGNED">SIGNED</option>
        <option value="TERMINATED">TERMINATED</option>
      </select>
      <button class="btn btn-primary" data-test-id="archive-search-submit" @click="search">Search</button>
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
        @click="open(contract)"
      >
        Open
      </button>
    </section>
    <section v-if="selected" class="mt-6 rounded border p-4" data-test-id="archive-contract-details">
      <h2 class="font-bold">{{ selected.name }}</h2>
      <p data-test-id="archive-state">{{ selected.state }}</p>
      <span
        v-for="tag in selected.archive_tags"
        :key="tag"
        class="mr-1 badge"
        data-test-id="archive-tag"
        :data-test-key="tag"
      >
        {{ tag }}
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
          <p v-if="integrityAuditEntryCount !== null" data-test-id="archive-integrity-result">
            {{ integrityAuditEntryCount }} authoritative audit entries returned
          </p>
          <input
            v-model="terminationReason"
            class="input-bordered input"
            data-test-id="archive-termination-reason"
            placeholder="Termination reason"
          />
          <button class="btn" data-test-id="archive-terminate" @click="terminate">Terminate</button>
          <input
            v-model="deleteJustification"
            class="input-bordered input"
            data-test-id="archive-delete-justification"
            placeholder="Deletion justification"
          />
          <button class="btn btn-error" data-test-id="archive-delete" @click="remove">Delete</button>
          <p v-if="deletedEntryCount !== null" data-test-id="archive-delete-result">
            {{ deletedEntryCount }} archive entries deleted
          </p>
        </div>
      </template>
    </section>
  </main>
</template>
