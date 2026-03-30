<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-50 flex justify-end bg-black/30">
      <div class="h-full w-full max-w-2xl overflow-y-auto bg-white shadow-2xl dark:bg-dark-900">
        <div class="flex items-center justify-between border-b border-gray-200 px-6 py-4 dark:border-dark-700">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Inbox detail</h2>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ displayRecord?.subject || 'No subject' }}
            </p>
          </div>
          <button
            type="button"
            class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600"
            @click="emit('close')"
          >
            Close
          </button>
        </div>

        <div class="space-y-6 px-6 py-5">
          <p v-if="loading" class="text-sm text-gray-500 dark:text-gray-400">Loading detail...</p>
          <p v-if="requestError" class="text-sm text-red-600 dark:text-red-400">{{ requestError }}</p>
          <div
            v-if="detailFetchFailed"
            class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-900/20 dark:text-red-300"
          >
            Detail fetch failed. Showing backend header detail state only.
          </div>

          <div v-if="displayRecord" class="space-y-6">
            <section class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
              <h3 class="text-sm font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                Summary
              </h3>
              <dl class="mt-4 grid gap-4 md:grid-cols-2">
                <div>
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">subject</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ displayRecord.subject || '-' }}</dd>
                </div>
                <div>
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">folder</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ displayRecord.folder }}</dd>
                </div>
                <div>
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">sender</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ displayRecord.sender || '-' }}</dd>
                </div>
                <div>
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">recipients</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ formatList(displayRecord.recipients) }}</dd>
                </div>
                <div>
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">received_at</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ displayRecord.received_at }}</dd>
                </div>
                <div>
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">flags</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ formatList(displayRecord.flags) }}</dd>
                </div>
                <div>
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">resolution_state</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ displayRecord.resolution_state }}</dd>
                </div>
                <div>
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">detail_fetch_state</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ displayRecord.detail_fetch_state }}</dd>
                </div>
              </dl>
            </section>

            <section class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
              <h3 class="text-sm font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                Snippet
              </h3>
              <p class="mt-4 whitespace-pre-wrap text-sm text-gray-800 dark:text-gray-200">
                {{ displayRecord.snippet || '-' }}
              </p>
            </section>

            <section class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
              <h3 class="text-sm font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                Delivery
              </h3>
              <dl class="mt-4 grid gap-4 md:grid-cols-2">
                <div>
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">envelope_recipients</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ formatList(displayRecord.envelope_recipients) }}</dd>
                </div>
                <div>
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">delivered_to</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ formatList(displayRecord.delivered_to) }}</dd>
                </div>
                <div class="md:col-span-2">
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">original_to</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ formatList(displayRecord.original_to) }}</dd>
                </div>
              </dl>
            </section>

            <section class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
              <h3 class="text-sm font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                Resolution
              </h3>
              <dl class="mt-4 grid gap-4 md:grid-cols-2">
                <div>
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">resolved_recipient_identity_id</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ formatValue(displayRecord.resolved_recipient_identity_id) }}</dd>
                </div>
                <div>
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">resolved_address</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ formatValue(displayRecord.resolved_address) }}</dd>
                </div>
                <div>
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">match_type</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ formatValue(displayRecord.match_type) }}</dd>
                </div>
                <div>
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">matched_value_id</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ formatValue(displayRecord.matched_value_id) }}</dd>
                </div>
                <div class="md:col-span-2">
                  <dt class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">resolution_source_field</dt>
                  <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ formatValue(displayRecord.resolution_source_field) }}</dd>
                </div>
              </dl>
            </section>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { adminAPI } from '@/api/admin'
import type { MailboxHeaderRecord } from '@/types'

interface Props {
  open: boolean
  headerId: number | null
  header: MailboxHeaderRecord | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const loading = ref(false)
const requestError = ref('')
const detailRecord = ref<MailboxHeaderRecord | null>(null)
const requestSequence = ref(0)

const displayRecord = computed(() => detailRecord.value ?? props.header ?? null)
const detailFetchFailed = computed(() => displayRecord.value?.detail_fetch_state === 'failed')

watch(
  () => [props.open, props.headerId] as const,
  async ([open, headerId]) => {
    if (!open || headerId === null) {
      resetState()
      return
    }

    detailRecord.value = props.header ? { ...props.header } : null
    loading.value = true
    requestError.value = ''
    const currentSequence = ++requestSequence.value

    try {
      const record = await adminAPI.mailbox.getInboxDetail(headerId)
      if (currentSequence !== requestSequence.value) {
        return
      }
      detailRecord.value = record
    } catch (error) {
      if (currentSequence !== requestSequence.value) {
        return
      }
      requestError.value = error instanceof Error ? error.message : 'Failed to load inbox detail'
    } finally {
      if (currentSequence === requestSequence.value) {
        loading.value = false
      }
    }
  },
  { immediate: true }
)

function resetState() {
  requestSequence.value += 1
  loading.value = false
  requestError.value = ''
  detailRecord.value = null
}

function formatList(values: string[] | null | undefined) {
  return values && values.length > 0 ? values.join(', ') : '-'
}

function formatValue(value: string | number | null | undefined) {
  return value === null || value === undefined || value === '' ? '-' : String(value)
}
</script>
