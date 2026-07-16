<script setup lang="ts">
import { useDcsDraftStore } from '@template-repository/store/dcsDraftStore'
import { useTemplateEditorUiStore } from '@template-repository/store/templateEditorUiStore'
import axios from 'axios'
import { storeToRefs } from 'pinia'
import { computed, onMounted, ref } from 'vue'
import { TemplateType } from '@/modules/template-repository/models/contract-template'
import { contractTemplateService } from '@/services/contract-template-service'
import { useContractTemplatesStore } from '@/stores/contract-templates-store'
import { TemplateState } from '@/types/contract-template-state'
import { useTemplatePermissions } from '../composables/useTemplatePermissions'

interface ComponentTemplateKey {
  did: string
  version: number
  document_number?: string
}

const store = useDcsDraftStore()
const uiStore = useTemplateEditorUiStore()
const templatesStore = useContractTemplatesStore()
const { templateType, blocks, subTemplateSnapshots, state, version } = storeToRefs(store)
const { contractTemplates: allTemplates } = storeToRefs(templatesStore)

const { isManager } = useTemplatePermissions()

onMounted(templatesStore.loadTemplates)

const document_number = computed({
  get: () => store.document_number,
  set: (value: string) => store.updateDocumentNumber(value),
})

const name = computed({
  get: () => store.name,
  set: (value: string) => store.updateName(value.trim()),
})

const description = computed({
  get: () => store.description,
  set: (value: string) => store.updateDescription(value),
})

const selectedComponents = computed<ComponentTemplateKey[]>(() =>
  subTemplateSnapshots.value.map((item) => ({
    did: item.did,
    version: item.version,
    document_number: item.document_number,
  })),
)
const showComponentPicker = ref(false)
const dependencyReference = ref('')
const dependencyError = ref<string | null>(null)
const dependencySaveResult = ref<string | null>(null)
const dependencySaving = ref(false)

const isSameTemplate = (a: ComponentTemplateKey, b: ComponentTemplateKey) =>
  a.did === b.did && a.version === b.version && a.document_number === b.document_number

const getComponentTemplateName = (item: ComponentTemplateKey) =>
  subTemplateSnapshots.value.find((t) => isSameTemplate(t, item))?.name ??
  allTemplates.value.find((t) => isSameTemplate(t, item))?.name ??
  item.did

const addComponentTemplate = async () => {
  if (!store.did || dependencySaving.value) return
  dependencyError.value = null
  dependencySaveResult.value = null
  dependencySaving.value = true
  try {
    const snapshot = await contractTemplateService.validateDependency({
      template_did: store.did,
      reference_did: dependencyReference.value,
    })
    store.addSubTemplateSnapshot(snapshot)
    dependencySaveResult.value = 'saved'
    dependencyReference.value = ''
  } catch (error: unknown) {
    dependencySaveResult.value = 'blocked'
    if (axios.isAxiosError(error)) {
      const payload = error.response?.data as { message?: string; name?: string } | undefined
      dependencyError.value = payload?.message ?? payload?.name ?? 'Dependency validation failed'
    } else {
      dependencyError.value = error instanceof Error ? error.message : 'Dependency validation failed'
    }
  } finally {
    dependencySaving.value = false
  }
}

const isComponentReferenced = (item: ComponentTemplateKey): boolean => {
  const inOutline = store.blockIdsInOutline
  return blocks.value.some(
    (b) => b['@type'] === 'dcs:ApprovedTemplate' && inOutline.has(b['@id']) && b['dcs:templateDid'] === item.did,
  )
}

const removeComponentTemplate = (item: ComponentTemplateKey) => {
  if (isComponentReferenced(item)) return
  store.removeSubTemplateSnapshot(item)
}
</script>

<template>
  <div class="grid grid-cols-1 gap-4">
    <!-- Contract Kind -->
    <fieldset class="fieldset border-none p-0">
      <legend class="fieldset-legend">Version: {{ version }}</legend>
    </fieldset>
    <fieldset class="fieldset border-none p-0">
      <legend class="fieldset-legend">Contract Type</legend>
      <div class="mt-1 grid grid-cols-2 gap-3">
        <div
          class="pointer-events-none card border-2 transition-all"
          :class="templateType === TemplateType.contractTemplate ? 'border-primary bg-primary/5' : 'border-base-300'"
        >
          <div class="card-body gap-1 p-4">
            <span class="card-title text-sm">Contract</span>
            <p class="text-xs font-normal text-base-content/60">Top-level contract template that can serve as parent</p>
          </div>
        </div>
        <div
          class="pointer-events-none card border-2 transition-all"
          :class="templateType === TemplateType.component ? 'border-primary bg-primary/5' : 'border-base-300'"
        >
          <div class="card-body gap-1 p-4">
            <span class="card-title text-sm">Component</span>
            <p class="text-xs font-normal text-base-content/60">
              Reusable partial contract, embeddable in other templates
            </p>
          </div>
        </div>
      </div>
    </fieldset>

    <fieldset v-if="isManager" class="fieldset border-none p-0">
      <legend class="fieldset-legend">Template State</legend>
      <select v-model="state" class="input-bordered select w-full" required :disabled="!uiStore.isTemplateEditable">
        <option>DRAFT</option>
        <option>REJECTED</option>
        <option>SUBMITTED</option>
        <option>REVIEWED</option>
        <option>APPROVED</option>
        <option>DELETED</option>
        <option v-if="state == TemplateState.registered">REGISTERED</option>
        <option v-if="state == TemplateState.published">PUBLISHED</option>
      </select>
    </fieldset>

    <fieldset class="fieldset border-none p-0">
      <legend class="fieldset-legend">Document number</legend>
      <input
        v-model="document_number"
        class="input-bordered input w-full"
        type="text"
        required
        :disabled="!uiStore.isTemplateEditable"
      />
    </fieldset>

    <fieldset class="fieldset border-none p-0">
      <legend class="fieldset-legend">Global Name</legend>
      <input
        v-model="name"
        :data-test-id="uiStore.isTemplateEditable ? 'template-editor-name' : 'template-details-name'"
        class="input-bordered input w-full"
        type="text"
        required
        :disabled="!uiStore.isTemplateEditable"
      />
    </fieldset>

    <fieldset class="fieldset border-none p-0">
      <legend class="fieldset-legend">Base Description</legend>
      <textarea
        v-model="description"
        :data-test-id="uiStore.isTemplateEditable ? 'template-editor-description' : 'template-details-description'"
        class="textarea-bordered textarea h-24 w-full"
        required
        :disabled="!uiStore.isTemplateEditable"
      ></textarea>
    </fieldset>

    <!-- Component templates (only for Contract type) -->
    <fieldset v-if="templateType === TemplateType.contractTemplate" class="fieldset border-none p-0">
      <legend
        data-test-id="template-component-add"
        class="fieldset-legend inline-flex cursor-pointer items-center gap-1.5 select-none"
        @click="showComponentPicker = !showComponentPicker"
      >
        Component Templates
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-3 w-3 opacity-60 transition-transform duration-200"
          :class="{ 'rotate-180': showComponentPicker }"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
        </svg>
      </legend>

      <!-- Collapsible picker -->
      <div v-show="showComponentPicker" class="mt-1">
        <input
          v-model="dependencyReference"
          data-test-id="template-component-reference"
          list="template-component-reference-options"
          class="input-bordered input input-sm w-full"
          placeholder="Template DID"
        />
        <datalist id="template-component-reference-options">
          <option
            v-for="template in allTemplates.filter(
              (item) =>
                item.template_type === TemplateType.component &&
                (item.state === TemplateState.approved || item.state === TemplateState.published),
            )"
            :key="template.did"
            :value="template.did"
          >
            {{ template.name }}
          </option>
        </datalist>
        <button
          type="button"
          class="btn mt-2 btn-sm btn-primary"
          data-test-id="template-component-save"
          :disabled="!store.did || dependencySaving"
          @click="addComponentTemplate"
        >
          {{ dependencySaving ? 'Validating…' : 'Save dependency' }}
        </button>
        <p v-if="dependencyError" data-test-id="template-dependency-error" class="mt-2 text-sm text-error">
          {{ dependencyError }}
        </p>
        <p
          v-if="dependencySaveResult"
          data-test-id="template-save-result"
          class="mt-2 text-sm"
          :class="dependencySaveResult === 'blocked' ? 'text-error' : 'text-success'"
        >
          {{ dependencySaveResult }}
        </p>
      </div>

      <!-- Selected templates (always visible) -->
      <div data-test-id="template-hierarchy-tree" class="mt-3">
        <div data-test-id="template-hierarchy-dependencies" class="flex flex-wrap gap-2">
          <div
            v-if="store.did"
            data-test-id="template-hierarchy-node"
            :data-test-key="store.did"
            class="badge gap-1 badge-outline py-3 badge-secondary"
          >
            <span>{{ store.name || store.did }}</span>
          </div>
          <div
            v-for="item in selectedComponents"
            :key="`${item.did}-${item.version}-${item.document_number}`"
            data-test-id="template-hierarchy-node"
            :data-test-key="item.did"
            class="badge gap-1 badge-outline py-3 badge-primary"
          >
            <span>{{ getComponentTemplateName(item) }}</span>
            <button
              type="button"
              :disabled="isComponentReferenced(item) || !uiStore.isTemplateEditable"
              :title="isComponentReferenced(item) ? 'Cannot remove: used in document' : undefined"
              class="text-error transition-opacity hover:opacity-70 disabled:cursor-not-allowed disabled:opacity-40"
              @click="removeComponentTemplate(item)"
            >
              ✕
            </button>
          </div>
          <p v-if="!selectedComponents.length" class="mt-2 fieldset-label">No component templates selected yet.</p>
        </div>
      </div>
    </fieldset>
  </div>
</template>
