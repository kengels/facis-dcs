<script setup lang="ts">
import AddBlockModal from '@template-repository/components/builder-editor/AddBlockModal.vue'
import BuilderPreviewDialog from '@template-repository/components/builder-editor/BuilderPreviewDialog.vue'
import BuilderEditor from '@template-repository/components/BuilderEditor.vue'
import ClauseEditor from '@template-repository/components/clauses-editor/ClauseEditor.vue'
import ClausesEditor from '@template-repository/components/ClausesEditor.vue'
import DetailsEditor from '@template-repository/components/DetailsEditor.vue'
import MetaDataEditor from '@template-repository/components/MetaDataEditor.vue'
import { useTemplatePermissions } from '@template-repository/composables/useTemplatePermissions'
import { useDcsDraftStore } from '@template-repository/store/dcsDraftStore'
import { useTemplateEditorUiStore } from '@template-repository/store/templateEditorUiStore.ts'
import { storeToRefs } from 'pinia'
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import TemplateAuditList from '@/components/lists/template/TemplateAuditList.vue'
import { useContractTemplateEventType } from '@/composables/useContractTemplateEventType'
import { contractTemplateService } from '@/services/contract-template-service'
import { TemplateState } from '@/types/contract-template-state'
import { toProperCase } from '@/utils/string'
import AuditView from './AuditView.vue'
import type {
  ContractTemplateAuditResponse,
  ContractTemplateHistoryResponse,
  TemplateProvenanceResponse,
} from '@/models/responses/template-response'

withDefaults(
  defineProps<{
    title: string
  }>(),
  {},
)

const route = useRoute()

const templateEditorUiStore = useTemplateEditorUiStore()
const draftStore = useDcsDraftStore()
const { activeTab } = storeToRefs(templateEditorUiStore)
const { state, templateType } = storeToRefs(draftStore)
const tabs = computed(() => {
  return templateEditorUiStore.availableTabs(templateType.value).filter((tab) => {
    return tab.id !== 'audit' || !!route.params.did
  })
})
const currentTabNumber = computed(() => 1 + tabs.value.map((tab) => tab.id).indexOf(activeTab.value))
const { isManager } = useTemplatePermissions()
const { isApproveEvent, isRejectEvent } = useContractTemplateEventType()
const lifecycleAudit = ref<ContractTemplateAuditResponse>([])
const versionHistory = ref<ContractTemplateHistoryResponse>([])
const provenance = ref<TemplateProvenanceResponse>([])
const versionHistoryError = ref('')
const provenanceError = ref('')
const currentVersion = computed(() => {
  if (!draftStore.did || draftStore.version == null || !draftStore.state || !draftStore.updated_at) return null
  return {
    did: draftStore.did,
    version: draftStore.version,
    state: draftStore.state,
    updatedAt: draftStore.updated_at,
  }
})
const approvalAudit = computed(() =>
  lifecycleAudit.value.filter((entry) => isApproveEvent(entry) || isRejectEvent(entry)),
)

watch(
  () => draftStore.did,
  async (did) => {
    lifecycleAudit.value = []
    versionHistory.value = []
    provenance.value = []
    versionHistoryError.value = ''
    provenanceError.value = ''
    if (!did) return

    // Audit evidence is independent of the optional version/provenance
    // projections. A failure in either projection must not hide committed
    // lifecycle decisions and their comments.
    lifecycleAudit.value = await contractTemplateService.audit({ did })
    const [historyResult, provenanceResult] = await Promise.allSettled([
      contractTemplateService.history(did),
      contractTemplateService.provenance(did),
    ])
    if (draftStore.did !== did) return
    if (historyResult.status === 'fulfilled') {
      versionHistory.value = historyResult.value
    } else {
      versionHistoryError.value = 'Version history could not be loaded.'
    }
    if (provenanceResult.status === 'fulfilled') {
      provenance.value = provenanceResult.value
    } else {
      provenanceError.value = 'Provenance history could not be loaded.'
    }
  },
  { immediate: true },
)
</script>

<template>
  <div class="sticky top-0 z-10 shrink-0 border-b border-base-300 bg-base-100">
    <div class="mx-auto max-w-5xl px-6 pt-3">
      <p class="mb-2 text-xs font-black tracking-widest text-base-content/40 uppercase">
        {{ title }}
      </p>
      <div role="tablist" class="tabs-border tabs tabs-lg">
        <a
          v-for="(tab, _index) in tabs"
          :key="tab.id"
          :data-test-id="tab.id === 'audit' ? 'template-manager-audit' : undefined"
          :data-test-key="tab.id === 'audit' ? draftStore.did : undefined"
          role="tab"
          class="tab"
          :class="{ 'tab-active text-primary': activeTab === tab.id }"
          @click="templateEditorUiStore.setActiveTab(tab.id)"
        >
          {{ tab.label }}
        </a>
      </div>
    </div>
  </div>

  <!-- Tab content -->
  <div class="mt-5 grow">
    <div class="mx-auto max-w-5xl p-6">
      <div class="grid grid-cols-1 gap-4">
        <slot name="before-tabs" />
        <!-- DETAILS TAB -->
        <div v-show="activeTab === 'details'">
          <div class="card border border-base-300 bg-base-100 shadow-sm">
            <div class="card-body gap-5">
              <h2 class="card-title justify-between text-sm">
                <div class="flex gap-2">
                  <span class="badge w-8 badge-sm badge-primary">0{{ currentTabNumber }}</span>
                  Template Details
                </div>
                <div v-if="state" data-test-id="template-lifecycle-status" class="badge badge-sm badge-secondary">
                  {{ toProperCase(state) }}
                </div>
              </h2>
              <DetailsEditor />
              <section class="mt-4 border-t border-base-300 pt-4">
                <h3 class="mb-2 text-sm font-semibold">Lifecycle evidence</h3>
                <div data-test-id="template-review-history">
                  <TemplateAuditList v-if="lifecycleAudit.length" :audits="lifecycleAudit" />
                  <p v-else class="text-sm text-base-content/60">No lifecycle decisions recorded.</p>
                </div>
                <div data-test-id="template-approval-history" class="mt-2 text-sm text-base-content/70">
                  <TemplateAuditList v-if="approvalAudit.length" :audits="approvalAudit" />
                  <p v-else>No approval decisions recorded.</p>
                </div>
                <div data-test-id="template-version-history" class="mt-3 text-sm">
                  <p
                    v-if="versionHistoryError"
                    data-test-id="template-version-history-error"
                    class="alert alert-error"
                    role="alert"
                  >
                    {{ versionHistoryError }}
                  </p>
                  <ol v-if="currentVersion || versionHistory.length">
                    <li v-if="currentVersion" :data-test-key="`${currentVersion.did}-${currentVersion.version}`">
                      Current version {{ currentVersion.version }} · {{ currentVersion.state }} ·
                      {{ currentVersion.updatedAt }}
                    </li>
                    <li
                      v-for="item in versionHistory"
                      :key="`${item.did}-${item.version}-${item.updated_at}`"
                      :data-test-key="`${item.did}-${item.version}`"
                    >
                      Historical version {{ item.version }} · {{ item.state }} · {{ item.updated_at }}
                    </li>
                  </ol>
                  <p v-else-if="!versionHistoryError" class="text-base-content/60">No version records available.</p>
                </div>
                <div data-test-id="template-provenance-history" class="mt-3 text-sm">
                  <p
                    v-if="provenanceError"
                    data-test-id="template-provenance-history-error"
                    class="alert alert-error"
                    role="alert"
                  >
                    {{ provenanceError }}
                  </p>
                  <ol v-if="provenance.length">
                    <li v-for="item in provenance" :key="item.vc_id" :data-test-key="item.vc_id">
                      Version {{ item.version }} · {{ item.vc_id }}
                    </li>
                  </ol>
                  <p v-else-if="!provenanceError" class="text-base-content/60">
                    No registered provenance credentials recorded.
                  </p>
                </div>
                <p
                  v-if="state === TemplateState.approved"
                  data-test-id="template-contract-ready-indicator"
                  class="mt-3 badge badge-success"
                >
                  Contract ready
                </p>
              </section>
            </div>
          </div>
        </div>

        <!-- CLAUSES TAB -->
        <div v-show="activeTab === 'clauses'">
          <div class="card border border-base-300 bg-base-100 shadow-sm">
            <div class="card-body gap-5">
              <h2 class="card-title text-sm">
                <span class="badge w-8 badge-sm badge-primary">0{{ currentTabNumber }}</span>
                Clauses
              </h2>
              <ClauseEditor />
              <div class="divider text-xs text-base-content/40">existing clauses</div>
              <ClausesEditor />
            </div>
          </div>
        </div>

        <!-- BUILDER TAB -->
        <div v-show="activeTab === 'builder'">
          <div class="card border border-base-300 bg-base-100 shadow-sm">
            <div class="card-body">
              <div class="mb-2 flex items-start justify-between">
                <h2 class="card-title text-sm">
                  <span class="badge w-8 badge-sm badge-primary">0{{ currentTabNumber }}</span>
                  Builder
                </h2>
                <button
                  type="button"
                  class="btn btn-sm btn-secondary"
                  @click="templateEditorUiStore.togglePreviewDialog"
                >
                  Preview
                </button>
              </div>
              <BuilderEditor />
            </div>
          </div>
          <AddBlockModal />
          <BuilderPreviewDialog />
        </div>

        <!-- META TAB -->
        <div v-show="activeTab === 'meta'">
          <div class="card border border-base-300 bg-base-100 shadow-sm">
            <div class="card-body">
              <h2 class="card-title text-sm">
                <span class="badge w-8 badge-sm badge-primary">0{{ currentTabNumber }}</span>
                Meta Data
              </h2>
              <MetaDataEditor />
            </div>
          </div>
        </div>

        <template v-if="isManager">
          <div v-show="activeTab === 'audit'">
            <div class="card border border-base-300 bg-base-100 shadow-sm">
              <div data-test-id="template-audit-timeline" class="card-body">
                <h2 class="card-title text-sm">
                  <span class="badge w-8 badge-sm badge-primary">0{{ currentTabNumber }}</span>
                  Audit History
                </h2>
                <AuditView />
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>
