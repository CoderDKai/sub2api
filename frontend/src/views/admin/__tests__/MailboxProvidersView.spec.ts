import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { BasePaginationResponse, MailboxProviderAccount, ProviderValidationOutcome } from '@/types'
import MailboxProvidersView from '../MailboxProvidersView.vue'

const {
  listProviders,
  createProvider,
  validateProvider,
  updateProviderStatus
} = vi.hoisted(() => ({
  listProviders: vi.fn(),
  createProvider: vi.fn(),
  validateProvider: vi.fn(),
  updateProviderStatus: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    mailbox: {
      listProviders,
      createProvider,
      validateProvider,
      updateProviderStatus
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const TablePageLayoutStub = {
  template: '<div><slot name="filters" /><slot name="table" /></div>'
}

function createProviderRecord(overrides: Partial<MailboxProviderAccount> = {}): MailboxProviderAccount {
  return {
    id: 1,
    display_name: 'Outlook Seed',
    provider_kind: 'microsoft',
    auth_kind: 'import_bundle',
    status: 'active',
    mailbox_hint: 'boss@example.com',
    provider_identifier: 'provider-42',
    payload_version: 1,
    payload_configured: true,
    payload_summary: { mailbox_identifier: 'bo***om', provider_identifier: 'pr***42' },
    last_imported_at: null,
    last_validation_at: null,
    last_validation_error: null,
    created_at: '2026-03-30T10:00:00Z',
    updated_at: '2026-03-30T10:00:00Z',
    deleted_at: null,
    ...overrides
  }
}

function createPage(items: MailboxProviderAccount[] = []): BasePaginationResponse<MailboxProviderAccount> {
  return {
    items,
    total: items.length,
    page: 1,
    page_size: 50,
    pages: items.length ? 1 : 0
  }
}

describe('admin MailboxProvidersView', () => {
  beforeEach(() => {
    listProviders.mockReset()
    createProvider.mockReset()
    validateProvider.mockReset()
    updateProviderStatus.mockReset()

    listProviders
      .mockResolvedValueOnce({
        items: [createProviderRecord({ id: 1, display_name: 'Outlook Seed' })],
        total: 2,
        page: 1,
        page_size: 1,
        pages: 2
      })
      .mockResolvedValueOnce({
        items: [createProviderRecord({ id: 2, display_name: 'Second Provider' })],
        total: 2,
        page: 2,
        page_size: 1,
        pages: 2
      })
    createProvider.mockResolvedValue(createProviderRecord())
    validateProvider.mockResolvedValue({
      account: createProviderRecord({ last_validation_at: '2026-03-30T12:00:00Z' }),
      code: 'ok',
      message: 'Validated successfully'
    } satisfies ProviderValidationOutcome)
    updateProviderStatus.mockResolvedValue(createProviderRecord({ status: 'disabled' }))
  })

  it('lists providers, creates a microsoft import-bundle provider, validates it, and toggles status inline', async () => {
    const wrapper = mount(MailboxProvidersView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub
        }
      }
    })

    await flushPromises()

    expect(listProviders).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('Second Provider')

    await wrapper.find('[data-testid="provider-create-button"]').trigger('click')
    await wrapper.find('[data-testid="provider-display-name"]').setValue('Outlook Seed')
    await wrapper.find('[data-testid="provider-import-payload"]').setValue(
      'boss@example.com----provider-42----opaque-left----opaque-right'
    )
    await wrapper.find('[data-testid="provider-save-button"]').trigger('click')
    await flushPromises()

    expect(createProvider).toHaveBeenCalledWith({
      display_name: 'Outlook Seed',
      provider_kind: 'microsoft',
      auth_kind: 'import_bundle',
      encrypted_payload: 'boss@example.com----provider-42----opaque-left----opaque-right'
    })
    expect(wrapper.text()).toContain('Outlook Seed')

    await wrapper.find('[data-testid="provider-validate-1"]').trigger('click')
    await flushPromises()

    expect(validateProvider).toHaveBeenCalledWith(1)
    expect(wrapper.text()).toContain('Validated successfully')

    await wrapper.find('[data-testid="provider-status-toggle-1"]').trigger('click')
    await flushPromises()

    expect(updateProviderStatus).toHaveBeenCalledWith(1, { status: 'disabled' })
    expect(wrapper.text()).toContain('disabled')
  })
})
