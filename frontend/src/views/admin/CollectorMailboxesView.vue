<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex items-center justify-between gap-3">
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.mailbox.collectors.title') }}
          </h1>

          <button
            data-testid="collector-create-button"
            type="button"
            class="rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white dark:bg-white dark:text-dark-950"
            @click="openCreateCollector"
          >
            Add collector
          </button>
        </div>
      </template>

      <template #table>
        <div class="space-y-4">
          <CollectorMailboxDialog
            :open="collectorDialogOpen"
            :initial-collector="editingCollector"
            :submitting="collectorSaving"
            @cancel="closeCollectorDialog"
            @save="handleCollectorSave"
          />

          <BatchSyncToolbar
            :selected-collector-count="selectedCollectorIds.length"
            :selected-capability-count="selectedCapabilityIds.length"
            :summary="batchSummary"
            @sync-collectors="triggerCollectorBatchSync"
            @sync-capabilities="triggerCapabilityBatchSync"
          />

          <p v-if="errorMessage" class="text-sm text-red-600 dark:text-red-400">
            {{ errorMessage }}
          </p>

          <div
            v-if="!collectors.length"
            class="rounded-xl border border-dashed border-gray-300 p-6 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
          >
            No collectors yet.
          </div>

          <div v-for="collector in collectors" :key="collector.id" class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
            <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div class="space-y-1">
                <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
                  <input
                    :data-testid="`select-collector-row-${collector.id}`"
                    type="checkbox"
                    :checked="selectedCollectorIds.includes(collector.id)"
                    @change="onCollectorSelectionChange(collector.id, $event)"
                  />
                  Select collector
                </label>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                  {{ collector.display_name }}
                </h2>
                <p class="text-sm text-gray-600 dark:text-gray-300">{{ collector.email_address }}</p>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ collector.enabled ? 'Enabled' : 'Disabled' }}
                </p>
              </div>

              <div class="flex flex-wrap items-center gap-2">
                <button
                  :data-testid="`collector-edit-${collector.id}`"
                  type="button"
                  class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600"
                  @click="openEditCollector(collector)"
                >
                  Edit collector
                </button>
                <button
                  :data-testid="`add-capability-${collector.id}`"
                  type="button"
                  class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600"
                  @click="openCreateCapability(collector.id)"
                >
                  Add capability
                </button>
                <button
                  :data-testid="`sync-collector-${collector.id}`"
                  type="button"
                  class="rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white dark:bg-white dark:text-dark-950"
                  @click="triggerCollectorSync(collector.id)"
                >
                  Sync now
                </button>
              </div>
            </div>

            <div v-if="capabilityEditor && capabilityEditor.collectorId === collector.id" class="mt-4 rounded-xl border border-dashed border-gray-300 p-4 dark:border-dark-600">
              <div v-if="!capabilityEditor.capabilityId" class="grid gap-3 md:grid-cols-4">
                <input
                  data-testid="capability-kind-input"
                  v-model="capabilityEditor.capabilityKind"
                  type="text"
                  class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
                />
                <input
                  data-testid="capability-provider-id-input"
                  v-model.number="capabilityEditor.providerAccountId"
                  type="number"
                  min="1"
                  class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
                />
                <input
                  data-testid="capability-folder-input"
                  v-model="capabilityEditor.folder"
                  type="text"
                  class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
                />
                <input
                  data-testid="capability-sync-interval-input"
                  v-model.number="capabilityEditor.syncIntervalSeconds"
                  type="number"
                  min="1"
                  class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
                />
              </div>
              <div v-else class="grid gap-3 md:grid-cols-[minmax(0,1fr)_180px_180px]">
                <div class="rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:text-gray-200">
                  {{ capabilityEditor.capabilityKind }}
                </div>
                <input
                  data-testid="capability-sync-interval-input"
                  v-model.number="capabilityEditor.syncIntervalSeconds"
                  type="number"
                  min="1"
                  class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
                />
                <label class="flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:text-gray-200">
                  <input
                    data-testid="capability-sync-enabled-input"
                    v-model="capabilityEditor.syncEnabled"
                    type="checkbox"
                  />
                  Sync enabled
                </label>
              </div>
              <div class="mt-3 flex items-center justify-end gap-2">
                <button
                  type="button"
                  class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600"
                  @click="capabilityEditor = null"
                >
                  Cancel
                </button>
                <button
                  data-testid="capability-save-button"
                  type="button"
                  class="rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white dark:bg-white dark:text-dark-950"
                  @click="saveCapability"
                >
                  Save capability
                </button>
              </div>
            </div>

            <div class="mt-4 space-y-3">
              <div
                v-for="capability in collector.capabilities"
                :key="capability.id"
                class="rounded-xl border border-gray-200 p-3 dark:border-dark-700"
              >
                <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                  <div class="space-y-1">
                    <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
                      <input
                        :data-testid="`select-capability-${capability.id}`"
                        type="checkbox"
                        :checked="selectedCapabilityIds.includes(capability.id)"
                        @change="onCapabilitySelectionChange(capability.id, $event)"
                      />
                      Select capability
                    </label>
                    <p class="font-medium text-gray-900 dark:text-white">{{ capability.capability_kind }}</p>
                    <p class="text-sm text-gray-600 dark:text-gray-300">
                      Folder {{ capability.connection_summary?.folder ?? 'INBOX' }} / {{ capability.health_state }}
                    </p>
                    <p v-if="capabilityMessages[capability.id]" class="text-sm text-gray-700 dark:text-gray-200">
                      {{ capabilityMessages[capability.id] }}
                    </p>
                  </div>

                  <div class="flex flex-wrap items-center gap-2">
                    <button
                      :data-testid="`edit-capability-${capability.id}`"
                      type="button"
                      class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600"
                      @click="openEditCapability(capability)"
                    >
                      Edit capability
                    </button>
                    <button
                      :data-testid="`test-capability-${capability.id}`"
                      type="button"
                      class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600"
                      @click="runCapabilityTest(capability.id)"
                    >
                      Test capability
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminAPI } from '@/api/admin'
import BatchSyncToolbar from '@/components/admin/mailbox/BatchSyncToolbar.vue'
import CollectorMailboxDialog from '@/components/admin/mailbox/CollectorMailboxDialog.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import type {
  BatchSyncStatus,
  CollectorMailbox,
  CreateCollectorPayload,
  MailboxCapability,
  MailboxProviderAccount
} from '@/types'

type CapabilityEditorState = {
  collectorId: number
  capabilityId?: number
  providerAccountId: number
  capabilityKind: string
  folder: string
  syncEnabled: boolean
  syncIntervalSeconds: number
}

const { t } = useI18n()

const collectors = ref<CollectorMailbox[]>([])
const providers = ref<MailboxProviderAccount[]>([])
const collectorDialogOpen = ref(false)
const editingCollector = ref<CollectorMailbox | null>(null)
const collectorSaving = ref(false)
const errorMessage = ref('')
const capabilityEditor = ref<CapabilityEditorState | null>(null)
const capabilityMessages = ref<Record<number, string>>({})
const selectedCollectorIds = ref<number[]>([])
const selectedCapabilityIds = ref<number[]>([])
const batchSummary = ref<BatchSyncStatus | null>(null)

let batchTimer: number | null = null

onMounted(() => {
  void loadPage()
})

onBeforeUnmount(() => {
  stopBatchPolling()
})

async function loadPage() {
  try {
    errorMessage.value = ''
    const [allCollectors, allProviders] = await Promise.all([
      loadAllCollectors(),
      loadAllProviders()
    ])
    collectors.value = allCollectors
    providers.value = allProviders
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to load collectors'
  }
}

async function loadAllCollectors() {
  const items: CollectorMailbox[] = []
  let page = 1

  while (true) {
    const response = await adminAPI.mailbox.listCollectors({ page, page_size: 100 })
    items.push(...response.items)

    if (page >= response.pages || items.length >= response.total || response.items.length === 0) {
      return items
    }

    page += 1
  }
}

async function loadAllProviders() {
  const items: MailboxProviderAccount[] = []
  let page = 1

  while (true) {
    const response = await adminAPI.mailbox.listProviders({ page, page_size: 100 })
    items.push(...response.items)

    if (page >= response.pages || items.length >= response.total || response.items.length === 0) {
      return items
    }

    page += 1
  }
}

function replaceCollector(collector: CollectorMailbox) {
  const next = [...collectors.value]
  const index = next.findIndex((item) => item.id === collector.id)
  if (index >= 0) {
    next[index] = collector
  } else {
    next.unshift(collector)
  }
  collectors.value = next
}

function updateCapabilityInCollectors(capability: MailboxCapability) {
  collectors.value = collectors.value.map((collector) => {
    if (collector.id !== capability.collector_id) {
      return collector
    }
    const capabilities = [...collector.capabilities]
    const index = capabilities.findIndex((item) => item.id === capability.id)
    if (index >= 0) {
      capabilities[index] = capability
    } else {
      capabilities.push(capability)
    }
    return {
      ...collector,
      capabilities
    }
  })
}

function openCreateCollector() {
  editingCollector.value = null
  collectorDialogOpen.value = true
}

function openEditCollector(collector: CollectorMailbox) {
  editingCollector.value = collector
  collectorDialogOpen.value = true
}

function closeCollectorDialog() {
  collectorDialogOpen.value = false
  editingCollector.value = null
}

async function handleCollectorSave(payload: CreateCollectorPayload) {
  collectorSaving.value = true
  try {
    if (editingCollector.value) {
      const updated = await adminAPI.mailbox.updateCollector(editingCollector.value.id, {
        email_address: payload.email_address,
        display_name: payload.display_name,
        enabled: payload.enabled,
        business_tags: payload.business_tags
      })
      replaceCollector(updated)
    } else {
      const created = await adminAPI.mailbox.createCollector(payload)
      replaceCollector(created)
    }
    closeCollectorDialog()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to save collector'
  } finally {
    collectorSaving.value = false
  }
}

function openCreateCapability(collectorId: number) {
  const collector = collectors.value.find((item) => item.id === collectorId)
  const inferredProviderId = collector?.capabilities[0]?.provider_account_id ?? providers.value[0]?.id ?? 0

  capabilityEditor.value = {
    collectorId,
    providerAccountId: inferredProviderId,
    capabilityKind: 'imap-primary',
    folder: 'INBOX',
    syncEnabled: true,
    syncIntervalSeconds: 300
  }
}

function openEditCapability(capability: MailboxCapability) {
  capabilityEditor.value = {
    collectorId: capability.collector_id,
    capabilityId: capability.id,
    providerAccountId: capability.provider_account_id,
    capabilityKind: capability.capability_kind,
    folder: String(capability.connection_summary?.folder ?? 'INBOX'),
    syncEnabled: capability.sync_enabled,
    syncIntervalSeconds: capability.sync_interval_seconds
  }
}

async function saveCapability() {
  if (!capabilityEditor.value) {
    return
  }

  const current = capabilityEditor.value
  try {
    if (current.capabilityId) {
      const updated = await adminAPI.mailbox.updateCapability(current.capabilityId, {
        sync_enabled: current.syncEnabled,
        sync_interval_seconds: current.syncIntervalSeconds
      })
      updateCapabilityInCollectors(updated)
    } else {
      const created = await adminAPI.mailbox.createCapability({
        provider_account_id: current.providerAccountId,
        collector_id: current.collectorId,
        capability_kind: current.capabilityKind,
        connection_config: { folder: current.folder },
        sync_enabled: current.syncEnabled,
        sync_interval_seconds: current.syncIntervalSeconds
      })
      updateCapabilityInCollectors(created)
    }
    capabilityEditor.value = null
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to save capability'
  }
}

async function runCapabilityTest(capabilityId: number) {
  try {
    const result = await adminAPI.mailbox.testCapability(capabilityId)
    if (result.capability) {
      updateCapabilityInCollectors(result.capability)
    }
    capabilityMessages.value = {
      ...capabilityMessages.value,
      [capabilityId]: result.result.message || (result.result.success ? 'Capability test ok' : 'Capability test failed')
    }
  } catch (error) {
    capabilityMessages.value = {
      ...capabilityMessages.value,
      [capabilityId]: error instanceof Error ? error.message : 'Capability test failed'
    }
  }
}

async function triggerCollectorSync(collectorId: number) {
  try {
    await adminAPI.mailbox.syncCollector(collectorId)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to sync collector'
  }
}

function onCollectorSelectionChange(collectorId: number, event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  selectedCollectorIds.value = checked
    ? [...selectedCollectorIds.value, collectorId]
    : selectedCollectorIds.value.filter((id) => id !== collectorId)
}

function onCapabilitySelectionChange(capabilityId: number, event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  selectedCapabilityIds.value = checked
    ? [...selectedCapabilityIds.value, capabilityId]
    : selectedCapabilityIds.value.filter((id) => id !== capabilityId)
}

async function triggerCollectorBatchSync() {
  if (!selectedCollectorIds.value.length) {
    return
  }
  try {
    const result = await adminAPI.mailbox.batchSyncCollectors({
      collector_ids: selectedCollectorIds.value
    })
    await startBatchPolling(result.batch_id)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to start collector batch sync'
  }
}

async function triggerCapabilityBatchSync() {
  if (!selectedCapabilityIds.value.length) {
    return
  }
  try {
    const result = await adminAPI.mailbox.batchSyncCollectors({
      capability_ids: selectedCapabilityIds.value
    })
    await startBatchPolling(result.batch_id)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to start capability batch sync'
  }
}

async function startBatchPolling(batchId: string) {
  stopBatchPolling()
  const shouldContinue = await pollBatch(batchId)
  if (!shouldContinue) {
    return
  }
  batchTimer = window.setInterval(() => {
    void pollBatch(batchId)
  }, 2000)
}

async function pollBatch(batchId: string) {
  try {
    const status = await adminAPI.mailbox.getBatchSyncStatus(batchId)
    batchSummary.value = status
    if (status.queued_count === 0 && status.running_count === 0) {
      stopBatchPolling()
      return false
    }
    return true
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to refresh batch status'
    stopBatchPolling()
    return false
  }
}

function stopBatchPolling() {
  if (batchTimer !== null) {
    window.clearInterval(batchTimer)
    batchTimer = null
  }
}
</script>
