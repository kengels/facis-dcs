<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { exportIncident, getIncident, type IncidentReportResult, listIncidents } from '@/services/auditing-service'

const incidents = ref<IncidentReportResult[]>([])
const selected = ref<IncidentReportResult | null>(null)

async function open(incidentId: string) {
  selected.value = await getIncident(incidentId)
}

async function download() {
  if (!selected.value) return
  const blob = await exportIncident(selected.value.incident_id)
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = `${selected.value.incident_id}.json`
  anchor.click()
  URL.revokeObjectURL(url)
}

onMounted(async () => {
  incidents.value = await listIncidents()
})
</script>

<template>
  <main class="p-4">
    <h1 class="text-2xl font-bold">Compliance incidents</h1>
    <ul class="my-4 space-y-2">
      <li v-for="incident in incidents" :key="incident.incident_id" class="flex items-center gap-3 rounded border p-3">
        <span>{{ incident.reason }} · {{ incident.reported_at }}</span>
        <button
          class="btn btn-sm"
          data-test-id="incident-list-open"
          :data-test-key="incident.incident_id"
          @click="open(incident.incident_id)"
        >
          Open
        </button>
      </li>
    </ul>
    <section v-if="selected" data-test-id="incident-details" class="rounded border p-4">
      <h2 class="font-bold">{{ selected.incident_id }}</h2>
      <p>{{ selected.reason }}</p>
      <p>{{ selected.affected_contract_dids.join(', ') }}</p>
      <p>{{ selected.affected_template_dids.join(', ') }}</p>
      <p>{{ selected.finding_refs.join(', ') }}</p>
      <button class="btn mt-3" data-test-id="incident-export" @click="download">Export incident</button>
    </section>
  </main>
</template>
