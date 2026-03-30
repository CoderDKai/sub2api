import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { BasePaginationResponse } from '@/types'
import MailInboxView from '../MailInboxView.vue'

const { listInbox, getInboxDetail } = vi.hoisted(() => ({
  listInbox: vi.fn(),
  getInboxDetail: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    mailbox: {
      listInbox,
      getInboxDetail
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
  template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
}
const PaginationStub = {
  name: 'Pagination',
  emits: ['update:page', 'update:pageSize'],
  template: `
    <div data-testid="pagination-stub">
      <button data-testid="pagination-next" type="button" @click="$emit('update:page', 2)">next</button>
    </div>
  `
}

function createDeferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((nextResolve) => {
    resolve = nextResolve
  })
  return { promise, resolve }
}

function createInboxRecord(overrides: Record<string, unknown> = {}) {
  return {
    id: 101,
    collector_id: 1,
    capability_id: 11,
    remote_message_id: 'remote-101',
    folder: 'INBOX',
    sender: 'boss@example.com',
    recipients: ['support@example.com'],
    subject: 'Production alert',
    received_at: '2026-03-30T10:00:00Z',
    flags: ['seen'],
    snippet: 'Mailbox summary snippet',
    envelope_recipients: ['support@example.com'],
    delivered_to: ['support@example.com'],
    original_to: ['ops@example.com'],
    resolved_recipient_identity_id: 9,
    resolved_address: 'support@example.com',
    match_type: 'exact_address',
    matched_value_id: 99,
    resolution_source_field: 'delivered_to',
    resolution_state: 'resolved',
    detail_fetch_state: 'not_requested',
    created_at: '2026-03-30T10:00:00Z',
    updated_at: '2026-03-30T10:00:00Z',
    ...overrides
  }
}

function createInboxPage(items: Array<Record<string, unknown>>): BasePaginationResponse<Record<string, unknown>> {
  return {
    items,
    total: items.length,
    page: 1,
    page_size: 20,
    pages: 1
  }
}

describe('admin MailInboxView', () => {
  beforeEach(() => {
    listInbox.mockReset()
    getInboxDetail.mockReset()

    listInbox.mockResolvedValue(createInboxPage([
      createInboxRecord(),
      createInboxRecord({
        id: 102,
        remote_message_id: 'remote-102',
        subject: 'Escalation thread',
        sender: 'alerts@example.com',
        detail_fetch_state: 'failed',
        resolution_state: 'unresolved'
      })
    ]))
    getInboxDetail.mockResolvedValue(createInboxRecord({
      id: 101,
      detail_fetch_state: 'succeeded',
      resolution_state: 'resolved',
      snippet: 'Detail refreshed snippet',
      resolution_source_field: 'original_to'
    }))
  })

  it('loads inbox headers on first render and shows header rows', async () => {
    const wrapper = mount(MailInboxView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: PaginationStub,
          Teleport: true
        }
      }
    })

    await flushPromises()

    expect(listInbox).toHaveBeenCalledWith({ page: 1, page_size: 20 })
    expect(wrapper.text()).toContain('Production alert')
    expect(wrapper.text()).toContain('alerts@example.com')
    expect(wrapper.text()).toContain('Mailbox summary snippet')
  })

  it('lazy loads inbox detail only after opening the drawer and shows refreshed detail state', async () => {
    const wrapper = mount(MailInboxView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: PaginationStub,
          Teleport: true
        }
      }
    })

    await flushPromises()

    expect(getInboxDetail).not.toHaveBeenCalled()

    const openButton = wrapper.find('[data-testid="open-detail-101"]')
    expect(openButton.exists()).toBe(true)
    if (!openButton.exists()) return

    await openButton.trigger('click')
    await flushPromises()

    expect(getInboxDetail).toHaveBeenCalledWith(101)
    expect(wrapper.text()).toContain('Detail refreshed snippet')
    expect(wrapper.text()).toContain('succeeded')
    expect(wrapper.text()).toContain('ops@example.com')
  })

  it('keeps only the latest opened header detail when earlier requests resolve late', async () => {
    const firstDetail = createDeferred<Record<string, unknown>>()
    const secondDetail = createDeferred<Record<string, unknown>>()

    getInboxDetail.mockReset()
    getInboxDetail.mockImplementation((id: number) => {
      if (id === 101) return firstDetail.promise
      return secondDetail.promise
    })

    const wrapper = mount(MailInboxView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: PaginationStub,
          Teleport: true
        }
      }
    })

    await flushPromises()

    await wrapper.find('[data-testid="open-detail-101"]').trigger('click')
    await wrapper.find('[data-testid="open-detail-102"]').trigger('click')
    await flushPromises()

    secondDetail.resolve(createInboxRecord({
      id: 102,
      subject: 'Escalation thread',
      sender: 'alerts@example.com',
      snippet: 'Second detail wins',
      detail_fetch_state: 'succeeded'
    }))
    await flushPromises()

    expect(wrapper.text()).toContain('Second detail wins')
    expect(wrapper.text()).not.toContain('First detail leaked')

    firstDetail.resolve(createInboxRecord({
      id: 101,
      snippet: 'First detail leaked',
      detail_fetch_state: 'succeeded'
    }))
    await flushPromises()

    expect(wrapper.text()).toContain('Second detail wins')
    expect(wrapper.text()).not.toContain('First detail leaked')
  })

  it('applies only the supported real backend filters', async () => {
    listInbox
      .mockResolvedValueOnce(createInboxPage([createInboxRecord()]))
      .mockResolvedValueOnce(createInboxPage([createInboxRecord({ id: 103, folder: 'Archive' })]))

    const wrapper = mount(MailInboxView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: PaginationStub,
          Teleport: true
        }
      }
    })

    await flushPromises()

    const collectorInput = wrapper.find('[data-testid="inbox-filter-collector-id"]')
    const capabilityInput = wrapper.find('[data-testid="inbox-filter-capability-id"]')
    const folderInput = wrapper.find('[data-testid="inbox-filter-folder"]')
    const applyButton = wrapper.find('[data-testid="inbox-apply-filters"]')

    expect(collectorInput.exists()).toBe(true)
    expect(capabilityInput.exists()).toBe(true)
    expect(folderInput.exists()).toBe(true)
    expect(applyButton.exists()).toBe(true)
    if (!collectorInput.exists() || !capabilityInput.exists() || !folderInput.exists() || !applyButton.exists()) {
      return
    }

    await collectorInput.setValue('12')
    await capabilityInput.setValue('34')
    await folderInput.setValue('Archive')
    await applyButton.trigger('click')
    await flushPromises()

    expect(listInbox).toHaveBeenLastCalledWith({
      collector_id: 12,
      capability_id: 34,
      folder: 'Archive',
      page: 1,
      page_size: 20
    })
  })

  it('requests the next page with current page size when pagination changes', async () => {
    listInbox
      .mockResolvedValueOnce({
        items: [createInboxRecord()],
        total: 2,
        page: 1,
        page_size: 20,
        pages: 2
      })
      .mockResolvedValueOnce({
        items: [createInboxRecord({ id: 202, subject: 'Page 2 message', remote_message_id: 'remote-202' })],
        total: 2,
        page: 2,
        page_size: 20,
        pages: 2
      })

    const wrapper = mount(MailInboxView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: PaginationStub,
          Teleport: true
        }
      }
    })

    await flushPromises()

    const nextButton = wrapper.find('[data-testid="pagination-next"]')
    expect(nextButton.exists()).toBe(true)
    if (!nextButton.exists()) return

    await nextButton.trigger('click')
    await flushPromises()

    expect(listInbox).toHaveBeenLastCalledWith({ page: 2, page_size: 20 })
    expect(wrapper.text()).toContain('Page 2 message')
  })

  it('does not leak unapplied filter drafts into refresh or pagination requests', async () => {
    listInbox
      .mockResolvedValueOnce(createInboxPage([createInboxRecord()]))
      .mockResolvedValueOnce(createInboxPage([createInboxRecord({ id: 103, folder: 'Archive' })]))
      .mockResolvedValueOnce(createInboxPage([createInboxRecord({ id: 104, folder: 'Archive' })]))
      .mockResolvedValueOnce({
        items: [createInboxRecord({ id: 105, folder: 'Archive', subject: 'Applied page 2' })],
        total: 2,
        page: 2,
        page_size: 20,
        pages: 2
      })

    const wrapper = mount(MailInboxView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: PaginationStub,
          Teleport: true
        }
      }
    })

    await flushPromises()

    await wrapper.find('[data-testid="inbox-filter-collector-id"]').setValue('12')
    await wrapper.find('[data-testid="inbox-filter-capability-id"]').setValue('34')
    await wrapper.find('[data-testid="inbox-filter-folder"]').setValue('Archive')
    await wrapper.find('[data-testid="inbox-apply-filters"]').trigger('click')
    await flushPromises()

    await wrapper.find('[data-testid="inbox-filter-collector-id"]').setValue('98')
    await wrapper.find('[data-testid="inbox-filter-capability-id"]').setValue('76')
    await wrapper.find('[data-testid="inbox-filter-folder"]').setValue('Drafts')
    await wrapper.find('[data-testid="inbox-refresh"]').trigger('click')
    await flushPromises()

    expect(listInbox).toHaveBeenLastCalledWith({
      collector_id: 12,
      capability_id: 34,
      folder: 'Archive',
      page: 1,
      page_size: 20
    })

    await wrapper.find('[data-testid="pagination-next"]').trigger('click')
    await flushPromises()

    expect(listInbox).toHaveBeenLastCalledWith({
      collector_id: 12,
      capability_id: 34,
      folder: 'Archive',
      page: 2,
      page_size: 20
    })
    expect(wrapper.text()).toContain('Applied page 2')
  })

  it('shows an explicit failed-detail state when backend returns failed in a 200 response', async () => {
    getInboxDetail.mockReset()
    getInboxDetail.mockResolvedValue(createInboxRecord({
      id: 101,
      detail_fetch_state: 'failed',
      snippet: 'Backend failure summary'
    }))

    const wrapper = mount(MailInboxView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: PaginationStub,
          Teleport: true
        }
      }
    })

    await flushPromises()
    await wrapper.find('[data-testid="open-detail-101"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Detail fetch failed')
    expect(wrapper.text()).toContain('Backend failure summary')
    expect(wrapper.text()).toContain('failed')
  })
})
