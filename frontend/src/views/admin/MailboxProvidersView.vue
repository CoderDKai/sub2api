<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex items-center justify-between gap-3">
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.mailbox.providers.title') }}
          </h1>

          <button
            data-testid="provider-create-button"
            type="button"
            class="rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white dark:bg-white dark:text-dark-950"
            @click="dialogOpen = true"
          >
            Add provider
          </button>
        </div>
      </template>

      <template #table>
        <div class="space-y-4">
          <ProviderAccountDialog
            :open="dialogOpen"
            :submitting="saving"
            @cancel="dialogOpen = false"
            @save="handleCreate"
          />

          <p v-if="errorMessage" class="text-sm text-red-600 dark:text-red-400">
            {{ errorMessage }}
          </p>

          <div
            v-if="!providers.length"
            class="rounded-xl border border-dashed border-gray-300 p-6 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
          >
            No providers yet.
          </div>

          <div v-else class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-700">
            <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800/80">
                <tr>
                  <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">Provider</th>
                  <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">Mailbox</th>
                  <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">Status</th>
                  <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">Payload</th>
                  <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">Last result</th>
                  <th class="px-4 py-3 text-right font-medium text-gray-600 dark:text-gray-300">Actions</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                <tr v-for="provider in providers" :key="provider.id">
                  <td class="px-4 py-3">
                    <div class="font-medium text-gray-900 dark:text-white">{{ provider.display_name }}</div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">
                      {{ provider.provider_kind }} / {{ provider.auth_kind }}
                    </div>
                  </td>
                  <td class="px-4 py-3 text-gray-700 dark:text-gray-200">
                    {{ provider.mailbox_hint || '-' }}
                  </td>
                  <td class="px-4 py-3 text-gray-700 dark:text-gray-200">
                    {{ provider.status }}
                  </td>
                  <td class="px-4 py-3 text-gray-700 dark:text-gray-200">
                    {{ provider.payload_configured ? 'Configured' : 'Missing' }}
                  </td>
                  <td class="px-4 py-3 text-gray-700 dark:text-gray-200">
                    {{ validationMessages[provider.id] || provider.last_validation_error || '-' }}
                  </td>
                  <td class="px-4 py-3">
                    <div class="flex justify-end gap-2">
                      <button
                        :data-testid="`provider-validate-${provider.id}`"
                        type="button"
                        class="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600"
                        @click="handleValidate(provider.id)"
                      >
                        Validate
                      </button>
                      <button
                        :data-testid="`provider-status-toggle-${provider.id}`"
                        type="button"
                        class="rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white dark:bg-white dark:text-dark-950"
                        @click="handleToggleStatus(provider)"
                      >
                        {{ provider.status === 'active' ? 'Disable' : 'Enable' }}
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
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
import ProviderAccountDialog from '@/components/admin/mailbox/ProviderAccountDialog.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import type { CreateProviderPayload, MailboxProviderAccount } from '@/types'

const { t } = useI18n()

const providers = ref<MailboxProviderAccount[]>([])
const dialogOpen = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const validationMessages = ref<Record<number, string>>({})

onMounted(() => {
  void loadProviders()
})

async function loadProviders() {
  try {
    errorMessage.value = ''
    const allProviders: MailboxProviderAccount[] = []
    let page = 1

    while (true) {
      const response = await adminAPI.mailbox.listProviders({ page, page_size: 100 })
      allProviders.push(...response.items)

      if (page >= response.pages || allProviders.length >= response.total || response.items.length === 0) {
        break
      }

      page += 1
    }

    providers.value = allProviders
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to load providers'
  }
}

function upsertProvider(provider: MailboxProviderAccount) {
  const next = [...providers.value]
  const index = next.findIndex((item) => item.id === provider.id)
  if (index >= 0) {
    next[index] = provider
  } else {
    next.unshift(provider)
  }
  providers.value = next
}

async function handleCreate(payload: CreateProviderPayload) {
  saving.value = true
  try {
    const created = await adminAPI.mailbox.createProvider(payload)
    upsertProvider(created)
    dialogOpen.value = false
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to create provider'
  } finally {
    saving.value = false
  }
}

async function handleValidate(providerId: number) {
  try {
    const outcome = await adminAPI.mailbox.validateProvider(providerId)
    if (outcome.account) {
      upsertProvider(outcome.account)
    }
    validationMessages.value = {
      ...validationMessages.value,
      [providerId]: outcome.message || outcome.code || '-'
    }
  } catch (error) {
    validationMessages.value = {
      ...validationMessages.value,
      [providerId]: error instanceof Error ? error.message : 'Validation failed'
    }
  }
}

async function handleToggleStatus(provider: MailboxProviderAccount) {
  const nextStatus = provider.status === 'active' ? 'disabled' : 'active'
  try {
    const updated = await adminAPI.mailbox.updateProviderStatus(provider.id, { status: nextStatus })
    upsertProvider(updated)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to update provider status'
  }
}
</script>
