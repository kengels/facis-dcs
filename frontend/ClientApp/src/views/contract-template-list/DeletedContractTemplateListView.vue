<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { contractTemplateService } from '@/services/contract-template-service'
import { TemplateState } from '@/types/contract-template-state'
import type { ContractTemplateSearchResponseItem } from '@/models/responses/template-response'

const templates = ref<ContractTemplateSearchResponseItem[]>([])

onMounted(async () => {
  templates.value = await contractTemplateService.search({ state: TemplateState.deleted })
})
</script>

<template>
  <main class="p-4">
    <h1 class="text-2xl font-bold">Deleted templates</h1>
    <ul class="my-4 space-y-2">
      <li
        v-for="template in templates"
        :key="template.did"
        data-test-id="template-deletion-tombstone"
        :data-test-key="template.did"
        class="rounded border p-3"
      >
        {{ template.name }} · {{ template.did }} · {{ template.state }}
      </li>
    </ul>
  </main>
</template>
