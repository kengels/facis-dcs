<script setup lang="ts">
import { computed, normalizeClass, ref, useAttrs, useTemplateRef, watch } from 'vue'
import { useRouter } from 'vue-router'
import ConfirmationModal from '@/components/ConfirmationModal.vue'
import { useContractPermissions } from '@/modules/contract-workflow-engine/composables/useContractPermissions'
import { ROUTES } from '@/router/router'
import { contractWorkflowService } from '@/services/contract-workflow-service'
import { ContractState } from '@/types/contract-state'
import type { Contract } from '@/models/contract/contract'

defineOptions({
  inheritAttrs: false,
})

const attrs = useAttrs()

const filteredClass = computed(() => {
  return normalizeClass(attrs.class)
    .split(' ')
    .filter(
      (cls) =>
        !['btn-primary', 'btn-secondary', 'btn-accent', 'btn-success', 'btn-warning', 'btn-error', 'btn-info'].includes(
          cls,
        ),
    )
    .join(' ')
})

const props = defineProps<{
  contract: Contract
}>()

const confirmationModal = useTemplateRef<InstanceType<typeof ConfirmationModal>>('confirmation-modal')

const router = useRouter()
const { isManager } = useContractPermissions()

const canTerminate = computed(() => {
  return isManager.value && props.contract.state !== ContractState.terminated
})

const canDeploy = computed(() => {
  return isManager.value && props.contract.state === ContractState.signed
})

const deploying = ref(false)
const renewals = ref<{ did: string; renews_did: string; renews_contract_version: number }[]>([])
const renewing = ref(false)

const canRenew = computed(
  () =>
    isManager.value &&
    new Set<ContractState>([
      ContractState.approved,
      ContractState.signed,
      ContractState.active,
      ContractState.terminated,
      ContractState.expired,
    ]).has(props.contract.state),
)

const loadRenewals = async () => {
  if (!canRenew.value) {
    renewals.value = []
    return
  }
  renewals.value = await contractWorkflowService.retrieveRenewals(props.contract.did)
}

const renew = async () => {
  if (!canRenew.value || renewing.value || renewals.value.length > 0) return
  renewing.value = true
  try {
    await contractWorkflowService.renew({
      did: props.contract.did,
      updated_at: props.contract.updated_at,
    })
    await loadRenewals()
  } catch (err) {
    console.error('Renewal failed:', err)
  } finally {
    renewing.value = false
  }
}

watch([() => props.contract.did, canRenew], loadRenewals, { immediate: true })

const deploy = async () => {
  if (!isManager.value || props.contract.state !== ContractState.signed) return
  deploying.value = true
  try {
    await contractWorkflowService.deploy({
      did: props.contract.did,
      updated_at: props.contract.updated_at,
    })
    router.go(0)
  } catch (err) {
    console.error('Deployment failed:', err)
  } finally {
    deploying.value = false
  }
}

const terminate = async () => {
  try {
    if (!confirmationModal.value) return
    const { isCanceled, data: reason } = await confirmationModal.value.reveal({
      message: 'Proceed with terminating?',
      editor: { requiredText: true, placeholder: 'Reason' },
    })
    if (!reason) {
      console.error('Reason is required for termination')
      return
    }
    if (!isCanceled) {
      const response = await contractWorkflowService.terminate({
        did: props.contract.did,
        updated_at: props.contract.updated_at,
        reason: reason,
      })
      if (response.did) {
        await router.push({ name: ROUTES.CONTRACTS.LIST })
      }
    }
  } catch (err) {
    console.error('Termination failed:', err)
  }
}
</script>

<template>
  <button v-if="canDeploy" :class="[filteredClass, 'btn-primary']" :disabled="deploying" @click="deploy">
    {{ deploying ? 'Deploying…' : 'Deploy' }}
  </button>
  <button
    v-if="canRenew"
    data-test-id="contract-manager-renew"
    :class="[filteredClass, 'btn-primary']"
    :disabled="renewing || renewals.length > 0"
    @click="renew"
  >
    {{ renewing ? 'Renewing…' : 'Renew' }}
  </button>
  <div
    v-for="renewal in renewals"
    :key="renewal.did"
    data-test-id="contract-renewal-result"
    :data-test-key="renewal.did"
    class="alert alert-success"
  >
    <RouterLink :to="{ name: ROUTES.CONTRACTS.VIEW, params: { did: renewal.did } }">{{ renewal.did }}</RouterLink>
    <span data-test-id="contract-renewal-source-reference">{{ renewal.renews_did }}</span>
  </div>
  <button
    v-if="canTerminate"
    data-test-id="contract-manager-terminate"
    :class="[filteredClass, 'btn-error']"
    @click="terminate"
  >
    Terminate
  </button>
  <ConfirmationModal
    ref="confirmation-modal"
    editor-test-id="contract-termination-reason"
    confirm-test-id="contract-termination-submit"
  />
</template>
