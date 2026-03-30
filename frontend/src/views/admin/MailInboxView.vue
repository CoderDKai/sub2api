<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="space-y-4">
          <div class="flex items-center justify-between gap-3">
            <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
              {{ t('admin.mailbox.inbox.title') }}
            </h1>
          </div>

          <div class="grid gap-3 md:grid-cols-[180px_180px_minmax(0,1fr)_auto_auto]">
            <input
              data-testid="inbox-filter-collector-id"
              v-model.number="draftFilters.collectorId"
              type="number"
              min="1"
              placeholder="collector_id"
              class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
            />
            <input
              data-testid="inbox-filter-capability-id"
              v-model.number="draftFilters.capabilityId"
              type="number"
              min="1"
              placeholder="capability_id"
              class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
            />
            <input
              data-testid="inbox-filter-folder"
              v-model="draftFilters.folder"
              type="text"
              placeholder="folder"
              class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
            />
            <button
              data-testid="inbox-apply-filters"
              type="button"
              class="rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white dark:bg-white dark:text-dark-950"
              @click="applyFilters"
            >
              Apply
            </button>
            <button
              data-testid="inbox-refresh"
              type="button"
              class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600"
              @click="refreshPage"
            >
              Refresh
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <div class="space-y-4">
          <p v-if="errorMessage" class="text-sm text-red-600 dark:text-red-400">
            {{ errorMessage }}
          </p>

          <div
            v-if="loading && !headers.length"
            class="rounded-xl border border-dashed border-gray-300 p-6 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
          >
            Loading inbox...
          </div>

          <div
            v-else-if="!headers.length"
            class="rounded-xl border border-dashed border-gray-300 p-6 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
          >
            No inbox headers yet.
          </div>

          <div v-else class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>Subject</th>
                  <th>Sender</th>
                  <th>Folder</th>
                  <th>Received</th>
                  <th>Resolution</th>
                  <th>Detail</th>
                  <th>Snippet</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="header in headers" :key="header.id">
                  <td>
                    <div class="font-medium text-gray-900 dark:text-white">{{ header.subject || '-' }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {{ header.remote_message_id }}
                    </div>
                  </td>
                  <td>{{ header.sender || '-' }}</td>
                  <td>{{ header.folder }}</td>
                  <td>{{ header.received_at }}</td>
                  <td>{{ header.resolution_state }}</td>
                  <td>{{ header.detail_fetch_state }}</td>
                  <td class="max-w-xs whitespace-pre-wrap break-words">{{ header.snippet || '-' }}</td>
                  <td>
                    <button
                      :data-testid="`open-detail-${header.id}`"
                      type="button"
                      class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600"
                      @click="openDetail(header)"
                    >
                      Open detail
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <MailInboxDetailDrawer
      :open="detailDrawerOpen"
      :header-id="selectedHeaderId"
      :header="selectedHeader"
      @close="closeDetail"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminAPI } from '@/api/admin'
import MailInboxDetailDrawer from '@/components/admin/mailbox/MailInboxDetailDrawer.vue'
import Pagination from '@/components/common/Pagination.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import type { MailInboxListParams, MailboxHeaderRecord } from '@/types'

const { t } = useI18n()

const headers = ref<MailboxHeaderRecord[]>([])
const loading = ref(false)
const errorMessage = ref('')
const detailDrawerOpen = ref(false)
const selectedHeaderId = ref<number | null>(null)
const selectedHeader = ref<MailboxHeaderRecord | null>(null)
const pagination = ref({
  page: 1,
  page_size: 20,
  total: 0,
  pages: 0
})
const draftFilters = reactive({
  collectorId: null as number | null,
  capabilityId: null as number | null,
  folder: ''
})
const appliedFilters = ref({
  collectorId: null as number | null,
  capabilityId: null as number | null,
  folder: ''
})

onMounted(() => {
  void loadInbox(1)
})

async function loadInbox(page: number) {
  try {
    loading.value = true
    errorMessage.value = ''

    const response = await adminAPI.mailbox.listInbox(buildListParams(page, pagination.value.page_size))

    headers.value = response.items
    pagination.value = {
      page: response.page,
      page_size: response.page_size,
      total: response.total,
      pages: response.pages
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to load inbox'
  } finally {
    loading.value = false
  }
}

function buildListParams(page: number, pageSize: number): MailInboxListParams {
  const params: MailInboxListParams = {
    page,
    page_size: pageSize
  }

  const collectorId = parsePositiveInteger(appliedFilters.value.collectorId)
  const capabilityId = parsePositiveInteger(appliedFilters.value.capabilityId)
  const folder = appliedFilters.value.folder.trim()

  if (collectorId !== null) {
    params.collector_id = collectorId
  }
  if (capabilityId !== null) {
    params.capability_id = capabilityId
  }
  if (folder) {
    params.folder = folder
  }

  return params
}

function parsePositiveInteger(value: string | number | null) {
  if (typeof value === 'number') {
    return Number.isFinite(value) && value > 0 ? value : null
  }
  if (value === null) {
    return null
  }
  const trimmed = value.trim()
  if (!trimmed) return null
  const parsed = Number.parseInt(trimmed, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

function applyFilters() {
  pagination.value.page = 1
  appliedFilters.value = {
    collectorId: parsePositiveInteger(draftFilters.collectorId),
    capabilityId: parsePositiveInteger(draftFilters.capabilityId),
    folder: draftFilters.folder.trim()
  }
  closeDetail()
  void loadInbox(1)
}

function refreshPage() {
  void loadInbox(pagination.value.page)
}

function handlePageChange(page: number) {
  closeDetail()
  void loadInbox(page)
}

function handlePageSizeChange(pageSize: number) {
  pagination.value.page_size = pageSize
  closeDetail()
  void loadInbox(1)
}

function openDetail(header: MailboxHeaderRecord) {
  selectedHeader.value = { ...header }
  selectedHeaderId.value = header.id
  detailDrawerOpen.value = true
}

function closeDetail() {
  detailDrawerOpen.value = false
  selectedHeaderId.value = null
  selectedHeader.value = null
}
</script>
