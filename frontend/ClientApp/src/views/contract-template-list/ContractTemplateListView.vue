<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import TemplateList from '@/components/lists/template/TemplateList.vue'
import { useTemplatePermissions } from '@/modules/template-repository/composables/useTemplatePermissions'
import { ROUTES } from '@/router/router'

const { isCreator } = useTemplatePermissions()
const route = useRoute()
const savedDid = computed(() => (typeof route.query.saved_did === 'string' ? route.query.saved_did : ''))
</script>

<template>
  <div v-if="savedDid" class="mb-4 alert alert-success" data-test-id="template-save-result" :data-test-key="savedDid">
    Draft saved
  </div>
  <div class="mb-4 flex justify-between border-b border-base-content/10 bg-base-100 p-4">
    <h2 class="text-2xl/7 font-bold sm:truncate sm:text-3xl sm:tracking-tight">
      {{ $route.meta.name }}
    </h2>

    <RouterLink
      v-if="isCreator"
      v-slot="{ route: linkRoute }"
      :to="{ name: ROUTES.TEMPLATES.NEW }"
      class="btn gap-2 self-end btn-primary"
    >
      {{ linkRoute.meta.name }}
    </RouterLink>
    <div v-else></div>
  </div>

  <TemplateList />
</template>
