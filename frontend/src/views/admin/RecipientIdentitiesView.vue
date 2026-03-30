<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex items-center justify-between gap-3">
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.mailbox.recipients.title') }}
          </h1>

          <button
            data-testid="recipient-create-button"
            type="button"
            class="rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white dark:bg-white dark:text-dark-950"
            @click="openCreateRecipient"
          >
            Add recipient
          </button>
        </div>
      </template>

      <template #table>
        <div class="space-y-4">
          <RecipientIdentityDialog
            :open="dialogOpen"
            :initial-recipient="editingRecipient"
            :submitting="saving"
            @cancel="closeDialog"
            @save="handleDialogSave"
          />

          <p v-if="errorMessage" class="text-sm text-red-600 dark:text-red-400">
            {{ errorMessage }}
          </p>

          <div
            v-if="!recipients.length"
            class="rounded-xl border border-dashed border-gray-300 p-6 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
          >
            No recipients yet.
          </div>

          <div v-for="recipient in recipients" :key="recipient.id" class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
            <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                  {{ recipient.name }}
                </h2>
                <p class="text-sm text-gray-600 dark:text-gray-300">
                  {{ recipient.enabled ? 'Enabled' : 'Disabled' }}
                </p>
              </div>

              <div class="flex flex-wrap items-center gap-2">
                <button
                  :data-testid="`recipient-edit-${recipient.id}`"
                  type="button"
                  class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600"
                  @click="openEditRecipient(recipient)"
                >
                  Edit recipient
                </button>
                <button
                  :data-testid="`recipient-import-${recipient.id}`"
                  type="button"
                  class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600"
                  @click="openImport(recipient.id)"
                >
                  Import exact addresses
                </button>
                <button
                  :data-testid="`recipient-inline-save-${recipient.id}`"
                  type="button"
                  class="rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white dark:bg-white dark:text-dark-950"
                  @click="saveInlineRecipient(recipient)"
                >
                  Save inline changes
                </button>
              </div>
            </div>

            <div class="mt-4 space-y-3">
              <div
                v-for="matchValue in recipient.match_values"
                :key="matchValue.id"
                class="grid gap-3 rounded-xl border border-gray-200 p-3 dark:border-dark-700 md:grid-cols-[160px_1fr_100px_100px]"
              >
                <div class="text-sm font-medium text-gray-700 dark:text-gray-200">
                  {{ matchValue.match_type }}
                </div>
                <div class="text-sm text-gray-700 dark:text-gray-200">
                  {{ matchValue.match_value }}
                </div>
                <input
                  :data-testid="`match-priority-${matchValue.id}`"
                  v-model.number="matchValue.priority"
                  type="number"
                  min="0"
                  class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
                />
                <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
                  <input
                    :data-testid="`match-active-${matchValue.id}`"
                    v-model="matchValue.active"
                    type="checkbox"
                  />
                  Active
                </label>
              </div>
            </div>

            <div v-if="importTargetId === recipient.id" class="mt-4 rounded-xl border border-dashed border-gray-300 p-4 dark:border-dark-600">
              <textarea
                data-testid="recipient-import-textarea"
                v-model="importText"
                rows="4"
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900"
              />
              <div class="mt-3 flex items-center justify-between gap-3">
                <p class="text-sm text-gray-600 dark:text-gray-300">{{ importMessage }}</p>
                <button
                  data-testid="recipient-import-submit"
                  type="button"
                  class="rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white dark:bg-white dark:text-dark-950"
                  @click="submitImport(recipient.id)"
                >
                  Import addresses
                </button>
              </div>
            </div>
          </div>
        </div>
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminAPI } from '@/api/admin'
import RecipientIdentityDialog from '@/components/admin/mailbox/RecipientIdentityDialog.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import type {
  CreateRecipientPayload,
  RecipientIdentity,
  RecipientMatchValueInput,
  UpdateRecipientPayload
} from '@/types'

const { t } = useI18n()

const recipients = ref<RecipientIdentity[]>([])
const dialogOpen = ref(false)
const editingRecipient = ref<RecipientIdentity | null>(null)
const saving = ref(false)
const errorMessage = ref('')
const importTargetId = ref<number | null>(null)
const importText = ref('')
const importMessage = ref('')

onMounted(() => {
  void loadRecipients()
})

async function loadRecipients() {
  try {
    errorMessage.value = ''
    const allRecipients: RecipientIdentity[] = []
    let page = 1

    while (true) {
      const response = await adminAPI.mailbox.listRecipients({ page, page_size: 100 })
      allRecipients.push(...response.items)

      if (page >= response.pages || allRecipients.length >= response.total || response.items.length === 0) {
        break
      }

      page += 1
    }

    recipients.value = allRecipients
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to load recipients'
  }
}

function upsertRecipient(recipient: RecipientIdentity) {
  const next = [...recipients.value]
  const index = next.findIndex((item) => item.id === recipient.id)
  if (index >= 0) {
    next[index] = recipient
  } else {
    next.unshift(recipient)
  }
  recipients.value = next
}

function openCreateRecipient() {
  editingRecipient.value = null
  dialogOpen.value = true
}

function openEditRecipient(recipient: RecipientIdentity) {
  editingRecipient.value = {
    ...recipient,
    match_values: recipient.match_values.map((value) => ({ ...value }))
  }
  dialogOpen.value = true
}

function closeDialog() {
  dialogOpen.value = false
  editingRecipient.value = null
}

async function handleDialogSave(payload: CreateRecipientPayload | UpdateRecipientPayload) {
  saving.value = true
  try {
    if (editingRecipient.value) {
      const updated = await adminAPI.mailbox.updateRecipient(editingRecipient.value.id, payload)
      upsertRecipient(updated)
    } else {
      const created = await adminAPI.mailbox.createRecipient(payload)
      upsertRecipient(created)
    }
    closeDialog()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to save recipient'
  } finally {
    saving.value = false
  }
}

function buildMatchValueInputs(recipient: RecipientIdentity): RecipientMatchValueInput[] {
  return recipient.match_values.map((value) => ({
    match_type: value.match_type,
    match_value: value.match_value,
    priority: value.priority,
    active: value.active,
    source_kind: value.source_kind || 'manual',
    source_metadata: value.source_metadata
  }))
}

async function saveInlineRecipient(recipient: RecipientIdentity) {
  try {
    const updated = await adminAPI.mailbox.updateRecipient(recipient.id, {
      name: recipient.name,
      enabled: recipient.enabled,
      match_values: buildMatchValueInputs(recipient)
    })
    upsertRecipient(updated)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to save inline recipient changes'
  }
}

function openImport(recipientId: number) {
  importTargetId.value = recipientId
  importText.value = ''
  importMessage.value = ''
}

async function submitImport(recipientId: number) {
  const addresses = importText.value
    .split(/\r?\n/)
    .map((value) => value.trim())
    .filter(Boolean)

  if (!addresses.length) {
    return
  }

  try {
    const result = await adminAPI.mailbox.importRecipientExactAddresses(recipientId, addresses)
    importMessage.value = `Imported ${result.imported}`
    await loadRecipients()
  } catch (error) {
    importMessage.value = error instanceof Error ? error.message : 'Import failed'
  }
}
</script>
