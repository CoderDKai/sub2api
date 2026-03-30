<template>
  <section class="rounded-xl border border-gray-200 bg-gray-50 p-3 dark:border-dark-600 dark:bg-dark-800/80">
    <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
      <div class="text-sm text-gray-600 dark:text-gray-300">
        <p>Collectors selected: {{ selectedCollectorCount }}</p>
        <p>Capabilities selected: {{ selectedCapabilityCount }}</p>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <button
          data-testid="collector-batch-sync-button"
          type="button"
          class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600"
          :disabled="selectedCollectorCount === 0"
          @click="$emit('sync-collectors')"
        >
          Batch sync collectors
        </button>
        <button
          data-testid="capability-batch-sync-button"
          type="button"
          class="rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white dark:bg-white dark:text-dark-950"
          :disabled="selectedCapabilityCount === 0"
          @click="$emit('sync-capabilities')"
        >
          Batch sync capabilities
        </button>
      </div>
    </div>

    <p v-if="summary" class="mt-3 text-sm text-gray-700 dark:text-gray-200">
      Queued {{ summary.queued_count }} Running {{ summary.running_count }} Success {{ summary.success_count }} Partial {{ summary.partial_count }} Failure {{ summary.failure_count }}
    </p>
  </section>
</template>

<script setup lang="ts">
import type { BatchSyncStatus } from '@/types'

defineProps<{
  selectedCollectorCount: number
  selectedCapabilityCount: number
  summary: BatchSyncStatus | null
}>()

defineEmits<{
  'sync-collectors': []
  'sync-capabilities': []
}>()
</script>
