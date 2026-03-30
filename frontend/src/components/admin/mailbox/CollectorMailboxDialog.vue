<template>
  <section
    v-if="open"
    class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-600 dark:bg-dark-800"
  >
    <div class="space-y-4">
      <div class="grid gap-4 md:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-200">
            Email address
          </label>
          <input
            data-testid="collector-email-address"
            v-model="emailAddress"
            type="email"
            class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
          />
        </div>

        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-200">
            Display name
          </label>
          <input
            data-testid="collector-display-name"
            v-model="displayName"
            type="text"
            class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
          />
        </div>
      </div>

      <div class="grid gap-4 md:grid-cols-2">
        <div v-if="!isEditing">
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-200">
            Provider account ID
          </label>
          <input
            data-testid="collector-provider-account-id"
            v-model="providerAccountId"
            type="number"
            min="1"
            class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
          />
        </div>

        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-200">
            Business tags
          </label>
          <input
            v-model="businessTags"
            type="text"
            placeholder="vip, apac"
            class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
          />
        </div>
      </div>

      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
        <input v-model="enabled" type="checkbox" />
        Enabled
      </label>

      <div v-if="!isEditing" class="rounded-lg border border-dashed border-gray-300 p-3 dark:border-dark-600">
        <div class="flex items-center justify-between gap-2">
          <p class="text-sm font-medium text-gray-700 dark:text-gray-200">
            Initial capabilities
          </p>
          <button
            data-testid="collector-add-initial-capability"
            type="button"
            class="rounded-lg border border-gray-300 px-2 py-1 text-sm dark:border-dark-600"
            @click="addInitialCapability"
          >
            Add IMAP capability
          </button>
        </div>

        <ul class="mt-3 space-y-2 text-sm text-gray-600 dark:text-gray-300">
          <li v-for="(capability, index) in capabilities" :key="`${capability.capability_kind}-${index}`">
            {{ capability.capability_kind }} / {{ capability.connection_config?.folder ?? 'INBOX' }}
          </li>
          <li v-if="!capabilities.length" class="text-gray-400 dark:text-gray-500">
            No initial capability
          </li>
        </ul>
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
          data-testid="collector-save-button"
          type="button"
          class="rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white dark:bg-white dark:text-dark-950"
          :disabled="submitting"
          @click="submit"
        >
          Save collector
        </button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import type { CollectorCapabilityInput, CollectorMailbox, CreateCollectorPayload } from '@/types'

const props = withDefaults(defineProps<{
  open: boolean
  submitting?: boolean
  initialCollector?: CollectorMailbox | null
}>(), {
  submitting: false,
  initialCollector: null
})

const emit = defineEmits<{
  save: [payload: CreateCollectorPayload]
  cancel: []
}>()

const emailAddress = ref('')
const displayName = ref('')
const enabled = ref(true)
const providerAccountId = ref('')
const businessTags = ref('')
const capabilities = ref<CollectorCapabilityInput[]>([])
const isEditing = computed(() => props.initialCollector !== null)

watch(
  () => [props.open, props.initialCollector] as const,
  ([open, collector]) => {
    if (!open) {
      return
    }
    emailAddress.value = collector?.email_address ?? ''
    displayName.value = collector?.display_name ?? ''
    enabled.value = collector?.enabled ?? true
    providerAccountId.value = ''
    businessTags.value = collector?.business_tags.join(', ') ?? ''
    capabilities.value = []
  },
  { immediate: true }
)

function addInitialCapability() {
  capabilities.value.push({
    capability_kind: 'imap-primary',
    connection_config: { folder: 'INBOX' },
    sync_enabled: true,
    sync_interval_seconds: 300
  })
}

function submit() {
  emit('save', {
    email_address: emailAddress.value.trim(),
    display_name: displayName.value.trim(),
    enabled: enabled.value,
    business_tags: businessTags.value
      .split(',')
      .map((value) => value.trim())
      .filter(Boolean),
    provider_account_id: isEditing.value
      ? undefined
      : providerAccountId.value
        ? Number(providerAccountId.value)
        : undefined,
    capabilities: isEditing.value ? undefined : capabilities.value.map((capability) => ({ ...capability }))
  })
}
</script>
