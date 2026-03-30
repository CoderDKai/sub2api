import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { BasePaginationResponse, RecipientIdentity } from '@/types'
import RecipientIdentitiesView from '../RecipientIdentitiesView.vue'

const {
  listRecipients,
  createRecipient,
  updateRecipient,
  importRecipientExactAddresses
} = vi.hoisted(() => ({
  listRecipients: vi.fn(),
  createRecipient: vi.fn(),
  updateRecipient: vi.fn(),
  importRecipientExactAddresses: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    mailbox: {
      listRecipients,
      createRecipient,
      updateRecipient,
      importRecipientExactAddresses
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

function createRecipientRecord(overrides: Partial<RecipientIdentity> = {}): RecipientIdentity {
  return {
    id: 7,
    name: 'Support Inbox',
    normalized_name: 'support inbox',
    enabled: true,
    match_values: [
      {
        id: 11,
        recipient_identity_id: 7,
        match_type: 'exact_address',
        match_value: 'relay@privaterelay.appleid.com',
        normalized_value: 'relay@privaterelay.appleid.com',
        active: true,
        priority: 100,
        source_kind: 'manual',
        source_metadata: { channel: 'icloud' },
        created_at: '2026-03-30T10:00:00Z',
        updated_at: '2026-03-30T10:00:00Z',
        disabled_at: null
      },
      {
        id: 12,
        recipient_identity_id: 7,
        match_type: 'domain_suffix',
        match_value: 'support.example.com',
        normalized_value: 'support.example.com',
        active: true,
        priority: 80,
        source_kind: 'manual',
        source_metadata: { scope: 'support' },
        created_at: '2026-03-30T10:00:00Z',
        updated_at: '2026-03-30T10:00:00Z',
        disabled_at: null
      }
    ],
    created_at: '2026-03-30T10:00:00Z',
    updated_at: '2026-03-30T10:00:00Z',
    deleted_at: null,
    ...overrides
  }
}

function createRecipientPage(items: RecipientIdentity[]): BasePaginationResponse<RecipientIdentity> {
  return {
    items,
    total: items.length,
    page: 1,
    page_size: 50,
    pages: items.length ? 1 : 0
  }
}

describe('admin RecipientIdentitiesView', () => {
  beforeEach(() => {
    listRecipients.mockReset()
    createRecipient.mockReset()
    updateRecipient.mockReset()
    importRecipientExactAddresses.mockReset()

    listRecipients
      .mockResolvedValueOnce({
        items: [createRecipientRecord()],
        total: 2,
        page: 1,
        page_size: 1,
        pages: 2
      })
      .mockResolvedValueOnce({
        items: [createRecipientRecord({ id: 8, name: 'Billing Inbox', normalized_name: 'billing inbox' })],
        total: 2,
        page: 2,
        page_size: 1,
        pages: 2
      })
    createRecipient.mockResolvedValue(createRecipientRecord({
      id: 8,
      name: 'Billing Inbox',
      normalized_name: 'billing inbox',
      match_values: [
        {
          id: 21,
          recipient_identity_id: 8,
          match_type: 'exact_address',
          match_value: 'billing@example.com',
          normalized_value: 'billing@example.com',
          active: true,
          priority: 100,
          source_kind: 'manual',
            source_metadata: { imported: true },
          created_at: '2026-03-30T10:00:00Z',
          updated_at: '2026-03-30T10:00:00Z',
          disabled_at: null
        },
        {
          id: 22,
          recipient_identity_id: 8,
          match_type: 'domain_suffix',
          match_value: 'billing.example.com',
          normalized_value: 'billing.example.com',
          active: true,
          priority: 50,
          source_kind: 'manual',
            source_metadata: { scope: 'billing' },
          created_at: '2026-03-30T10:00:00Z',
          updated_at: '2026-03-30T10:00:00Z',
          disabled_at: null
        }
      ]
    }))
    updateRecipient.mockResolvedValue(createRecipientRecord({
      name: 'Support Inbox Updated',
      normalized_name: 'support inbox updated',
      enabled: true,
      match_values: [
        {
          id: 31,
          recipient_identity_id: 7,
          match_type: 'exact_address',
          match_value: 'vip@example.com',
          normalized_value: 'vip@example.com',
          active: false,
          priority: 5,
          source_kind: 'manual',
            source_metadata: { channel: 'vip' },
          created_at: '2026-03-30T10:00:00Z',
          updated_at: '2026-03-30T10:00:00Z',
          disabled_at: null
        },
        {
          id: 32,
          recipient_identity_id: 7,
          match_type: 'domain_suffix',
          match_value: 'vip.example.com',
          normalized_value: 'vip.example.com',
          active: true,
          priority: 10,
          source_kind: 'manual',
            source_metadata: { scope: 'vip' },
          created_at: '2026-03-30T10:00:00Z',
          updated_at: '2026-03-30T10:00:00Z',
          disabled_at: null
        }
      ]
    }))
    importRecipientExactAddresses.mockResolvedValue({ imported: 2 })
  })

  it('lists recipients, creates a recipient with exact-address and domain-suffix rules, and edits it by replacing match values', async () => {
    const wrapper = mount(RecipientIdentitiesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Support Inbox')
    expect(listRecipients).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('Billing Inbox')

    await wrapper.find('[data-testid="recipient-create-button"]').trigger('click')
    await wrapper.find('[data-testid="recipient-name"]').setValue('Billing Inbox')
    await wrapper.find('[data-testid="add-exact-address"]').trigger('click')
    await wrapper.find('[data-testid="match-value-0"]').setValue('billing@example.com')
    await wrapper.find('[data-testid="add-domain-suffix"]').trigger('click')
    await wrapper.find('[data-testid="match-value-1"]').setValue('@billing.example.com')
    await wrapper.find('[data-testid="recipient-save-button"]').trigger('click')
    await flushPromises()

    expect(createRecipient).toHaveBeenCalledWith({
      name: 'Billing Inbox',
      enabled: true,
      match_values: [
        {
          match_type: 'exact_address',
          match_value: 'billing@example.com',
          priority: 100,
          active: true,
          source_kind: 'manual',
          source_metadata: {}
        },
        {
          match_type: 'domain_suffix',
          match_value: '@billing.example.com',
          priority: 100,
          active: true,
          source_kind: 'manual',
          source_metadata: {}
        }
      ]
    })
    expect(wrapper.text()).toContain('Billing Inbox')

    await wrapper.find('[data-testid="recipient-edit-7"]').trigger('click')
    await wrapper.find('[data-testid="recipient-name"]').setValue('Support Inbox Updated')
    await wrapper.find('[data-testid="match-value-0"]').setValue('vip@example.com')
    await wrapper.find('[data-testid="remove-match-value-1"]').trigger('click')
    await wrapper.find('[data-testid="recipient-save-button"]').trigger('click')
    await flushPromises()

    expect(updateRecipient).toHaveBeenCalledWith(7, {
      name: 'Support Inbox Updated',
      enabled: true,
      match_values: [
        {
          match_type: 'exact_address',
          match_value: 'vip@example.com',
          priority: 100,
          active: true,
          source_kind: 'manual',
          source_metadata: { channel: 'icloud' }
        }
      ]
    })
  })

  it('supports inline priority and active edits and bulk exact-address import', async () => {
    const wrapper = mount(RecipientIdentitiesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub
        }
      }
    })

    await flushPromises()

    await wrapper.find('[data-testid="match-priority-11"]').setValue('5')
    await wrapper.find('[data-testid="match-active-11"]').setValue(false)
    await wrapper.find('[data-testid="recipient-inline-save-7"]').trigger('click')
    await flushPromises()

    expect(updateRecipient).toHaveBeenCalledWith(7, expect.objectContaining({
      name: 'Support Inbox',
      enabled: true,
      match_values: expect.arrayContaining([
        expect.objectContaining({
          match_type: 'exact_address',
          match_value: 'relay@privaterelay.appleid.com',
          priority: 5,
          active: false,
          source_metadata: { channel: 'icloud' }
        })
      ])
    }))

    listRecipients.mockResolvedValueOnce(createRecipientPage([
      createRecipientRecord({
        match_values: [
          {
            id: 11,
            recipient_identity_id: 7,
            match_type: 'exact_address',
            match_value: 'relay@privaterelay.appleid.com',
            normalized_value: 'relay@privaterelay.appleid.com',
            active: false,
            priority: 5,
            source_kind: 'manual',
            source_metadata: { channel: 'icloud' },
            created_at: '2026-03-30T10:00:00Z',
            updated_at: '2026-03-30T10:00:00Z',
            disabled_at: null
          },
          {
            id: 12,
            recipient_identity_id: 7,
            match_type: 'exact_address',
            match_value: 'one@privaterelay.appleid.com',
            normalized_value: 'one@privaterelay.appleid.com',
            active: true,
            priority: 0,
            source_kind: 'import',
            source_metadata: { import: 'relay' },
            created_at: '2026-03-30T10:00:00Z',
            updated_at: '2026-03-30T10:00:00Z',
            disabled_at: null
          }
        ]
      })
    ]))

    await wrapper.find('[data-testid="recipient-import-7"]').trigger('click')
    await wrapper.find('[data-testid="recipient-import-textarea"]').setValue(
      'one@privaterelay.appleid.com\ntwo@privaterelay.appleid.com'
    )
    await wrapper.find('[data-testid="recipient-import-submit"]').trigger('click')
    await flushPromises()

    expect(importRecipientExactAddresses).toHaveBeenCalledWith(7, [
      'one@privaterelay.appleid.com',
      'two@privaterelay.appleid.com'
    ])
    expect(listRecipients).toHaveBeenCalledTimes(3)
    expect(wrapper.text()).toContain('one@privaterelay.appleid.com')
  })
})
