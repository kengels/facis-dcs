<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  type ComplianceMonitorResult,
  type IncidentReportResult,
  monitorCompliance,
  reportIncident,
} from '@/services/auditing-service'

const monitor = ref<ComplianceMonitorResult | null>(null)
const contractDids = ref('')
const templateDids = ref('')
const findingRefs = ref<string[]>([])
const reason = ref('')
const result = ref<IncidentReportResult | null>(null)
const error = ref('')

const splitDids = (value: string) =>
  value
    .split(',')
    .map((did) => did.trim())
    .filter(Boolean)
const findingValue = (riskType: string) => riskType.toLowerCase().split('_').join('-')

async function submit() {
  error.value = ''
  try {
    result.value = await reportIncident({
      affected_contract_dids: splitDids(contractDids.value),
      affected_template_dids: splitDids(templateDids.value),
      finding_refs: findingRefs.value,
      reason: reason.value.trim(),
    })
  } catch (cause: unknown) {
    error.value = cause instanceof Error ? cause.message : 'Incident report failed.'
  }
}

onMounted(async () => {
  monitor.value = await monitorCompliance()
})
</script>

<template>
  <main class="p-4">
    <h1 class="text-2xl font-bold">Non-Compliance Investigation</h1>
    <section class="my-4" data-test-id="compliance-monitor-findings">
      <p v-if="!monitor">Monitoring compliance…</p>
      <p v-else-if="monitor.risks.length === 0">No current compliance risks.</p>
      <ul v-else>
        <li v-for="risk in monitor.risks" :key="`${risk.did}:${risk.risk_type}`">{{ risk.did }}: {{ risk.detail }}</li>
      </ul>
    </section>
    <div class="flex flex-col gap-3">
      <input
        v-model="contractDids"
        class="input-bordered input"
        data-test-id="incident-contract-dids"
        placeholder="Affected contract DIDs"
      />
      <input
        v-model="templateDids"
        class="input-bordered input"
        data-test-id="incident-template-dids"
        placeholder="Affected template DIDs"
      />
      <select v-model="findingRefs" multiple class="select-bordered select" data-test-id="incident-finding-selection">
        <option
          v-for="risk in monitor?.risks ?? []"
          :key="`${risk.did}:${risk.risk_type}`"
          :value="findingValue(risk.risk_type)"
        >
          {{ risk.detail }}
        </option>
      </select>
      <textarea
        v-model="reason"
        class="textarea-bordered textarea"
        data-test-id="incident-reason"
        placeholder="Investigation reason"
      ></textarea>
      <button
        class="btn btn-primary"
        data-test-id="incident-submit"
        :disabled="!reason.trim() || findingRefs.length === 0"
        @click="submit"
      >
        Report incident
      </button>
    </div>
    <div v-if="result" class="mt-4 alert alert-success" data-test-id="incident-result">
      <p>{{ result.reason }}</p>
      <p data-test-id="incident-affected-contract">{{ result.affected_contract_dids.join(', ') }}</p>
      <p data-test-id="incident-affected-template">{{ result.affected_template_dids.join(', ') }}</p>
    </div>
    <p v-if="error" class="mt-4 alert alert-error">{{ error }}</p>
  </main>
</template>
