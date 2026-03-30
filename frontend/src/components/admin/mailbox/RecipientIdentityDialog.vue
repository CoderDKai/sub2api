<template>
  <section
    v-if="open"
    class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-600 dark:bg-dark-800"
  >
    <div class="space-y-4">
      <div>
        <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-200">
          Identity name
        </label>
        <input
          data-testid="recipient-name"
          v-model="name"
          type="text"
          class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
        />
      </div>

      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
        <input v-model="enabled" type="checkbox" />
        Enabled
      </label>

      <div class="rounded-lg border border-dashed border-gray-300 p-3 dark:border-dark-600">
        <div class="flex flex-wrap items-center gap-2">
          <button
            data-testid="add-exact-address"
            type="button"
            class="rounded-lg border border-gray-300 px-2 py-1 text-sm dark:border-dark-600"
            @click="addMatchValue('exact_address')"
          >
            Add exact address
          </button>
          <button
            data-testid="add-domain-suffix"
            type="button"
            class="rounded-lg border border-gray-300 px-2 py-1 text-sm dark:border-dark-600"
            @click="addMatchValue('domain_suffix')"
          >
            Add domain suffix
          </button>
        </div>

        <div class="mt-3 space-y-3">
          <div
            v-for="(value, index) in values"
            :key="`${value.match_type}-${index}`"
            data-testid="match-value-row"
            class="grid gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-700 md:grid-cols-[140px_1fr_90px_80px_96px]"
          >
            <select
              v-model="value.match_type"
              class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
            >
              <option value="exact_address">exact_address</option>
              <option value="domain_suffix">domain_suffix</option>
            </select>

            <input
              :data-testid="`match-value-${index}`"
              v-model="value.match_value"
              type="text"
              class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
            />

            <input
              v-model.number="value.priority"
              type="number"
              class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
            />

            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
              <input v-model="value.active" type="checkbox" />
              Active
            </label>

            <button
              :data-testid="`remove-match-value-${index}`"
              type="button"
              class="rounded-lg border border-gray-300 px-2 py-2 text-sm dark:border-dark-600"
              @click="removeMatchValue(index)"
            >
              Remove
            </button>
          </div>
        </div>
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
          data-testid="recipient-save-button"
          type="button"
          class="rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white dark:bg-white dark:text-dark-950"
          :disabled="submitting"
          @click="submit"
        >
          Save recipient
        </button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

import type { CreateRecipientPayload, RecipientIdentity, RecipientMatchType, UpdateRecipientPayload } from '@/types'

type EditableMatchValue = {
  match_type: RecipientMatchType
  match_value: string
  priority: number
  active: boolean
  source_kind: string
  source_metadata: Record<string, unknown>
}

const props = withDefaults(defineProps<{
  open: boolean
  submitting?: boolean
  initialRecipient?: RecipientIdentity | null
}>(), {
  submitting: false,
  initialRecipient: null
})

const emit = defineEmits<{
  save: [payload: CreateRecipientPayload | UpdateRecipientPayload]
  cancel: []
}>()

const name = ref('')
const enabled = ref(true)
const values = ref<EditableMatchValue[]>([])

watch(
  () => [props.open, props.initialRecipient] as const,
  ([open, recipient]) => {
    if (!open) {
      return
    }
    name.value = recipient?.name ?? ''
    enabled.value = recipient?.enabled ?? true
    values.value = recipient?.match_values.map((value) => ({
      match_type: value.match_type,
      match_value: value.match_value,
      priority: value.priority,
      active: value.active,
      source_kind: value.source_kind || 'manual',
      source_metadata: { ...value.source_metadata }
    })) ?? []
  },
  { immediate: true }
)

function addMatchValue(matchType: RecipientMatchType) {
    values.value.push({
      match_type: matchType,
      match_value: '',
      priority: 100,
      active: true,
      source_kind: 'manual',
      source_metadata: {}
    })
}

function removeMatchValue(index: number) {
  values.value.splice(index, 1)
}

function submit() {
  emit('save', {
    name: name.value.trim(),
    enabled: enabled.value,
    match_values: values.value.map((value) => ({
      match_type: value.match_type,
      match_value: value.match_value.trim(),
      priority: value.priority,
      active: value.active,
      source_kind: value.source_kind || 'manual',
      source_metadata: value.source_metadata
    }))
  })
}
</script>
