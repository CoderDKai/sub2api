import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type {
  BasePaginationResponse,
  CollectorMailbox,
  MailboxCapability,
  MailboxProviderAccount,
  TestCapabilityResult
} from '@/types'
import CollectorMailboxesView from '../CollectorMailboxesView.vue'

const {
  listCollectors,
  listProviders,
  createCollector,
  updateCollector,
  createCapability,
  updateCapability,
  testCapability,
  syncCollector,
  batchSyncCollectors,
  getBatchSyncStatus
} = vi.hoisted(() => ({
  listCollectors: vi.fn(),
  listProviders: vi.fn(),
  createCollector: vi.fn(),
  updateCollector: vi.fn(),
  createCapability: vi.fn(),
  updateCapability: vi.fn(),
  testCapability: vi.fn(),
  syncCollector: vi.fn(),
  batchSyncCollectors: vi.fn(),
  getBatchSyncStatus: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    mailbox: {
      listCollectors,
      listProviders,
      createCollector,
      updateCollector,
      createCapability,
      updateCapability,
      testCapability,
      syncCollector,
      batchSyncCollectors,
      getBatchSyncStatus
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

function createProvider(overrides: Partial<MailboxProviderAccount> = {}): MailboxProviderAccount {
  return {
    id: 3,
    display_name: 'Outlook Import',
    provider_kind: 'microsoft',
    auth_kind: 'import_bundle',
    status: 'active',
    mailbox_hint: 'boss@example.com',
    provider_identifier: 'provider-42',
    payload_version: 1,
    payload_configured: true,
    payload_summary: { configured: true },
    last_imported_at: null,
    last_validation_at: null,
    last_validation_error: null,
    created_at: '2026-03-30T10:00:00Z',
    updated_at: '2026-03-30T10:00:00Z',
    deleted_at: null,
    ...overrides
  }
}

function createCapabilityRecord(overrides: Partial<MailboxCapability> = {}): MailboxCapability {
  return {
    id: 101,
    provider_account_id: 3,
    collector_id: 1,
    capability_kind: 'imap-primary',
    sync_enabled: true,
    sync_interval_seconds: 300,
    next_sync_at: null,
    last_sync_at: null,
    health_state: 'healthy',
    last_error: null,
    connection_configured: true,
    connection_summary: { folder: 'INBOX' },
    created_at: '2026-03-30T10:00:00Z',
    updated_at: '2026-03-30T10:00:00Z',
    deleted_at: null,
    ...overrides
  }
}

function createCollectorRecord(overrides: Partial<CollectorMailbox> = {}): CollectorMailbox {
  return {
    id: 1,
    email_address: 'support@example.com',
    display_name: 'Support Inbox',
    enabled: true,
    business_tags: ['vip'],
    capabilities: [createCapabilityRecord()],
    created_at: '2026-03-30T10:00:00Z',
    updated_at: '2026-03-30T10:00:00Z',
    deleted_at: null,
    ...overrides
  }
}

function createCollectorPage(items: CollectorMailbox[]): BasePaginationResponse<CollectorMailbox> {
  return {
    items,
    total: items.length,
    page: 1,
    page_size: 50,
    pages: items.length ? 1 : 0
  }
}

describe('admin CollectorMailboxesView', () => {
  beforeEach(() => {
    vi.useFakeTimers()

    listCollectors.mockReset()
    listProviders.mockReset()
    createCollector.mockReset()
    updateCollector.mockReset()
    createCapability.mockReset()
    updateCapability.mockReset()
    testCapability.mockReset()
    syncCollector.mockReset()
    batchSyncCollectors.mockReset()
    getBatchSyncStatus.mockReset()

    listCollectors
      .mockResolvedValueOnce({
        items: [createCollectorRecord({
          capabilities: [createCapabilityRecord({ provider_account_id: 9, connection_summary: { folder: 'INBOX', host: 'imap.example.com' } })]
        })],
        total: 2,
        page: 1,
        page_size: 1,
        pages: 2
      })
      .mockResolvedValueOnce({
        items: [createCollectorRecord({
          id: 2,
          email_address: 'ops@example.com',
          display_name: 'Ops Inbox',
          capabilities: [createCapabilityRecord({ id: 201, collector_id: 2, provider_account_id: 9 })]
        })],
        total: 2,
        page: 2,
        page_size: 1,
        pages: 2
      })
    listProviders.mockResolvedValue({ items: [createProvider({ id: 9 })], total: 1, page: 1, page_size: 50, pages: 1 })
    createCollector.mockResolvedValue(createCollectorRecord({
      id: 2,
      email_address: 'billing@example.com',
      display_name: 'Billing Inbox',
      capabilities: [createCapabilityRecord({ id: 202, collector_id: 2 })]
    }))
    updateCollector.mockResolvedValue(createCollectorRecord({
      display_name: 'Support Inbox Updated',
      capabilities: [createCapabilityRecord({ provider_account_id: 9, connection_summary: { folder: 'INBOX', host: 'imap.example.com' } })]
    }))
    createCapability.mockResolvedValue(createCapabilityRecord({ id: 102, capability_kind: 'pop3-backup' }))
    updateCapability.mockResolvedValue(createCapabilityRecord({
      sync_interval_seconds: 600,
      connection_summary: { folder: 'Archive', host: 'imap.example.com' }
    }))
    testCapability.mockResolvedValue({
      capability: createCapabilityRecord({ health_state: 'healthy' }),
      result: {
        success: true,
        health_state: 'healthy',
        message: 'Connection ok'
      }
    } satisfies TestCapabilityResult)
    syncCollector.mockResolvedValue({ batch_id: 'single-1' })
    batchSyncCollectors.mockResolvedValue({ batch_id: 'batch-1' })
    getBatchSyncStatus.mockResolvedValue({
      batch_id: 'batch-1',
      queued_count: 0,
      running_count: 1,
      success_count: 1,
      partial_count: 0,
      failure_count: 0,
      cancelled_count: 0,
      jobs: []
    })
  })

  afterEach(() => {
    vi.clearAllTimers()
    vi.useRealTimers()
  })

  it('lists collectors, creates a collector with capabilities, edits collector basics, edits capabilities through safe fields only, tests capability, and syncs a collector', async () => {
    const wrapper = mount(CollectorMailboxesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Support Inbox')
    expect(wrapper.text()).toContain('Ops Inbox')
    expect(wrapper.text()).toContain('imap-primary')

    await wrapper.find('[data-testid="collector-create-button"]').trigger('click')
    await wrapper.find('[data-testid="collector-email-address"]').setValue('billing@example.com')
    await wrapper.find('[data-testid="collector-display-name"]').setValue('Billing Inbox')
    await wrapper.find('[data-testid="collector-provider-account-id"]').setValue('3')
    await wrapper.find('[data-testid="collector-add-initial-capability"]').trigger('click')
    await wrapper.find('[data-testid="collector-save-button"]').trigger('click')
    await flushPromises()

    expect(createCollector).toHaveBeenCalledWith({
      email_address: 'billing@example.com',
      display_name: 'Billing Inbox',
      enabled: true,
      business_tags: [],
      provider_account_id: 3,
      capabilities: [
        {
          capability_kind: 'imap-primary',
          connection_config: { folder: 'INBOX' },
          sync_enabled: true,
          sync_interval_seconds: 300
        }
      ]
    })
    expect(wrapper.text()).toContain('Billing Inbox')

    await wrapper.find('[data-testid="collector-edit-1"]').trigger('click')

    expect(wrapper.find('[data-testid="collector-provider-account-id"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="collector-add-initial-capability"]').exists()).toBe(false)

    await wrapper.find('[data-testid="collector-display-name"]').setValue('Support Inbox Updated')
    await wrapper.find('[data-testid="collector-save-button"]').trigger('click')
    await flushPromises()

    expect(updateCollector).toHaveBeenCalledWith(1, expect.objectContaining({
      email_address: 'support@example.com',
      display_name: 'Support Inbox Updated'
    }))

    await wrapper.find('[data-testid="add-capability-1"]').trigger('click')
    await wrapper.find('[data-testid="capability-kind-input"]').setValue('pop3-backup')
    await wrapper.find('[data-testid="capability-save-button"]').trigger('click')
    await flushPromises()

    expect(createCapability).toHaveBeenCalledWith({
      provider_account_id: 9,
      collector_id: 1,
      capability_kind: 'pop3-backup',
      connection_config: { folder: 'INBOX' },
      sync_enabled: true,
      sync_interval_seconds: 300
    })

    await wrapper.find('[data-testid="edit-capability-101"]').trigger('click')

    expect(wrapper.find('[data-testid="capability-folder-input"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="capability-provider-id-input"]').exists()).toBe(false)

    await wrapper.find('[data-testid="capability-sync-interval-input"]').setValue('600')
    await wrapper.find('[data-testid="capability-save-button"]').trigger('click')
    await flushPromises()

    expect(updateCapability).toHaveBeenCalledWith(101, {
      sync_enabled: true,
      sync_interval_seconds: 600
    })

    await wrapper.find('[data-testid="test-capability-101"]').trigger('click')
    await flushPromises()

    expect(testCapability).toHaveBeenCalledWith(101)
    expect(wrapper.text()).toContain('Connection ok')

    await wrapper.find('[data-testid="sync-collector-1"]').trigger('click')
    await flushPromises()

    expect(syncCollector).toHaveBeenCalledWith(1)
  })

  it('batch syncs selected collectors and capabilities and polls batch progress summary', async () => {
    const wrapper = mount(CollectorMailboxesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub
        }
      }
    })

    await flushPromises()

    await wrapper.find('[data-testid="select-collector-row-1"]').setValue(true)
    await wrapper.find('[data-testid="collector-batch-sync-button"]').trigger('click')
    await flushPromises()

    expect(batchSyncCollectors).toHaveBeenCalledWith({ collector_ids: [1] })
    expect(getBatchSyncStatus).toHaveBeenCalledWith('batch-1')

    await wrapper.find('[data-testid="select-capability-101"]').setValue(true)
    await wrapper.find('[data-testid="capability-batch-sync-button"]').trigger('click')
    await flushPromises()

    expect(batchSyncCollectors).toHaveBeenLastCalledWith({ capability_ids: [101] })

    vi.advanceTimersByTime(2000)
    await flushPromises()

    expect(getBatchSyncStatus).toHaveBeenCalled()
    expect(wrapper.text()).toContain('Success 1')
    expect(wrapper.text()).toContain('Running 1')
  })

  it('stops batch polling when status refresh fails', async () => {
    getBatchSyncStatus
      .mockResolvedValueOnce({
        batch_id: 'batch-1',
        queued_count: 1,
        running_count: 0,
        success_count: 0,
        partial_count: 0,
        failure_count: 0,
        cancelled_count: 0,
        jobs: []
      })
      .mockRejectedValueOnce(new Error('poll failed'))

    const wrapper = mount(CollectorMailboxesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub
        }
      }
    })

    await flushPromises()

    await wrapper.find('[data-testid="select-collector-row-1"]').setValue(true)
    await wrapper.find('[data-testid="collector-batch-sync-button"]').trigger('click')
    await flushPromises()

    vi.advanceTimersByTime(2000)
    await flushPromises()
    vi.advanceTimersByTime(4000)
    await flushPromises()

    expect(getBatchSyncStatus).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('poll failed')
  })

  it('does not send health_state when disabling an existing capability', async () => {
    const wrapper = mount(CollectorMailboxesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub
        }
      }
    })

    await flushPromises()

    await wrapper.find('[data-testid="edit-capability-101"]').trigger('click')
    await wrapper.find('[data-testid="capability-sync-enabled-input"]').setValue(false)
    await wrapper.find('[data-testid="capability-save-button"]').trigger('click')
    await flushPromises()

    expect(updateCapability).toHaveBeenCalledWith(101, {
      sync_enabled: false,
      sync_interval_seconds: 300
    })
  })
})
