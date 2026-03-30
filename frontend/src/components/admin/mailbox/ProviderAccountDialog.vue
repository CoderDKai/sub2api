<template>
  <section
    v-if="open"
    class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-600 dark:bg-dark-800"
  >
    <div class="space-y-4">
      <div>
        <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-200">
          Display name
        </label>
        <input
          data-testid="provider-display-name"
          v-model="displayName"
          type="text"
          class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
        />
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-200">
          Outlook import bundle
        </label>
        <textarea
          data-testid="provider-import-payload"
          v-model="importPayload"
          rows="4"
          class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
        />
      </div>

      <div class="flex items-center justify-end gap-2">
        <button
          type="button"
          class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600"
          @click="$emit('cancel')"
        >
          Cancel
        </button>
        <button
          data-testid="provider-save-button"
          type="button"
          class="rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white dark:bg-white dark:text-dark-950"
          :disabled="submitting"
          @click="submit"
        >
          Save provider
        </button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

import type { CreateProviderPayload } from '@/types'

const props = withDefaults(defineProps<{
  open: boolean
  submitting?: boolean
}>(), {
  submitting: false
})

const emit = defineEmits<{
  save: [payload: CreateProviderPayload]
  cancel: []
}>()

const displayName = ref('')
const importPayload = ref('')

watch(
  () => props.open,
  (open) => {
    if (!open) {
      return
    }
    displayName.value = ''
    importPayload.value = ''
  }
)

function submit() {
  emit('save', {
    display_name: displayName.value.trim(),
    provider_kind: 'microsoft',
    auth_kind: 'import_bundle',
    encrypted_payload: importPayload.value.trim()
  })
}
</script>
