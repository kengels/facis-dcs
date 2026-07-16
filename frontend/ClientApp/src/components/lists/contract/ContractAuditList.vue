<script setup lang="ts">
import { useContractEventType } from '@/composables/useContractEventType'
import { contractAuditEventDisplayText } from '@/utils/contract-audit-event-display'
import { toProperCase } from '@/utils/string'
import type { ContractAuditResponse } from '@/models/responses/contract-response'

withDefaults(
  defineProps<{
    audits: ContractAuditResponse
    reviewOnly?: boolean
    showReviewHistory?: boolean
  }>(),
  { reviewOnly: false, showReviewHistory: true },
)

const eventType = useContractEventType()
</script>

<template>
  <div v-if="showReviewHistory" data-test-id="contract-review-history" class="mb-3 space-y-1">
    <template v-for="audit in audits" :key="`review-${audit.id}`">
      <p v-if="eventType.isSubmitEvent(audit) && audit.event_data.comments?.length">
        {{ audit.event_data.comments.join(', ') }}
      </p>
      <p v-else-if="eventType.isReviewEvent(audit)">Reviewed by: {{ audit.event_data.reviewed_by }}</p>
      <p v-else-if="eventType.isRejectEvent(audit)">Reason: {{ audit.event_data.reason }}</p>
    </template>
  </div>
  <ul v-if="!reviewOnly" class="list">
    <li
      v-for="audit in audits"
      :key="audit.id"
      class="list-row grid-cols-1"
      data-test-id="contract-decision-history"
      :data-test-key="audit.did"
    >
      <div class="flex justify-between">
        <div>{{ new Date(audit.event_data.occurred_at).toLocaleString() }}</div>
        <div class="badge badge-outline badge-sm badge-secondary">
          {{ contractAuditEventDisplayText(audit.event_type, audit.event_data) }}
        </div>
        <div class="text-xs">{{ toProperCase(audit.component) }}</div>
      </div>
      <div class="list-col-wrap">
        <div v-if="eventType.isCreateEvent(audit)">
          <div>Created by: {{ audit.event_data.created_by }}</div>
        </div>
        <div v-else-if="eventType.isUpdateEvent(audit)">
          <div>Updated by: {{ audit.event_data.updated_by }}</div>
        </div>
        <div v-else-if="eventType.isSubmitEvent(audit)" class="flex justify-between">
          <div>Submitted by: {{ audit.event_data.submitted_by }}</div>
          <div>
            Transition:
            <span class="relative -top-0.5 badge badge-outline badge-xs badge-secondary">
              {{ toProperCase(audit.event_data.previous_state) }}
            </span>
            →
            <span class="relative -top-0.5 badge badge-outline badge-xs badge-secondary">
              {{ toProperCase(audit.event_data.new_state) }}
            </span>
          </div>
          <div v-if="audit.event_data.comments?.length">
            {{ audit.event_data.comments.join(', ') }}
          </div>
        </div>
        <div v-else-if="eventType.isRetrieveByIDEvent(audit)">
          <div>Retrieved by: {{ audit.event_data.retrieved_by }}</div>
        </div>
        <div v-else-if="eventType.isRetrieveHistoryByDIDEvent(audit)">
          <div>Retrieved by: {{ audit.event_data.retrieved_by }}</div>
        </div>
        <div v-else-if="eventType.isRetrieveAllEvent(audit)">
          <div>Retrieved by: {{ audit.event_data.retrieved_by }}</div>
        </div>
        <div v-else-if="eventType.isVerifyEvent(audit)">
          <div>Verified by: {{ audit.event_data.verified_by }}</div>
        </div>
        <div v-else-if="eventType.isNegotiationEvent(audit)">
          <div>Negotiated by: {{ audit.event_data.negotiated_by }}</div>
          <div v-if="audit.event_data.change_request?.comment">{{ audit.event_data.change_request.comment }}</div>
          <div v-if="audit.event_data.change_request?.redline">{{ audit.event_data.change_request.redline }}</div>
        </div>
        <div v-else-if="eventType.isAcceptNegotiationEvent(audit)">
          <div>Accepted by: {{ audit.event_data.accepted_by }}</div>
        </div>
        <div v-else-if="eventType.isRejectNegotiationEvent(audit)">
          <div>Rejected by: {{ audit.event_data.rejected_by }}</div>
        </div>
        <div v-else-if="eventType.isApproveEvent(audit)">
          <div>Approved by: {{ audit.event_data.approved_by }}</div>
        </div>
        <div v-else-if="eventType.isRejectEvent(audit)" class="flex justify-between">
          <div>Rejected by: {{ audit.event_data.rejected_by }}</div>
          <div>Reason: {{ audit.event_data.reason }}</div>
        </div>
        <div v-else-if="eventType.isTerminateEvent(audit)">
          <div data-test-id="contract-termination-actor">Terminated by: {{ audit.event_data.terminated_by }}</div>
          <div>Reason: {{ audit.event_data.reason }}</div>
          <time data-test-id="contract-termination-timestamp">{{ audit.created_at }}</time>
        </div>
        <div v-else-if="eventType.isRecordEvidenceEvent(audit)">
          <div>Recorded by: {{ audit.event_data.recorded_by }}</div>
          <div>{{ audit.event_data.evidence_type }} · {{ audit.event_data.reference }}</div>
        </div>
        <div v-else-if="eventType.isAuditEvent(audit)">
          <div>Audited by: {{ audit.event_data.audited_by }}</div>
        </div>
        <div v-else-if="eventType.isReviewEvent(audit)">
          <div>Reviewed by: {{ audit.event_data.reviewed_by }}</div>
        </div>
        <div v-else-if="eventType.isIncreaseContractVersionEvent(audit)">
          <div>Submitted by: {{ audit.event_data.submitted_by }}</div>
        </div>
        <div v-else>{{ audit.event_data }}</div>
      </div>
    </li>
  </ul>
</template>
