import { describe, it, expect, vi, beforeEach } from 'vitest'
import { apiClient } from '@/api/client'
import { adminAPI, mailboxAPI } from '@/api/admin'
import * as mailboxApi from '@/api/admin/mailbox'

describe('mailbox admin api', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('posts batch sync payload to the mailbox endpoint', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { batch_id: 'b1' } } as any)

    await mailboxApi.batchSyncCollectors({ capability_ids: [11, 12] })

    expect(post).toHaveBeenCalledWith('/admin/mailbox/collectors/batch-sync', {
      capability_ids: [11, 12]
    })
  })

  it('posts exact-address import payload to the recipient import endpoint', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { imported: 2 } } as any)

    await mailboxApi.importRecipientExactAddresses(7, [
      'one@privaterelay.appleid.com',
      'two@privaterelay.appleid.com'
    ])

    expect(post).toHaveBeenCalledWith('/admin/mailbox/recipients/7/import-exact-addresses', {
      addresses: ['one@privaterelay.appleid.com', 'two@privaterelay.appleid.com']
    })
  })

  it('puts recipient updates to the recipient endpoint', async () => {
    const put = vi.spyOn(apiClient, 'put').mockResolvedValue({
      data: { id: 7, name: 'Support', normalized_name: 'support', enabled: true }
    } as any)

    await mailboxApi.updateRecipient(7, {
      name: 'Support',
      match_values: [{ match_type: 'exact_address', match_value: 'support@example.com' }]
    })

    expect(put).toHaveBeenCalledWith('/admin/mailbox/recipients/7', {
      name: 'Support',
      match_values: [{ match_type: 'exact_address', match_value: 'support@example.com' }]
    })
  })

  it('posts provider validation to the validate endpoint', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({
      data: { code: 'ok', message: '', account: { id: 3 } }
    } as any)

    await mailboxApi.validateProvider(3)

    expect(post).toHaveBeenCalledWith('/admin/mailbox/providers/3/validate')
  })

  it('posts provider status updates to the status endpoint', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({
      data: { id: 3, status: 'disabled' }
    } as any)

    await mailboxApi.updateProviderStatus(3, { status: 'disabled' })

    expect(post).toHaveBeenCalledWith('/admin/mailbox/providers/3/status', {
      status: 'disabled'
    })
  })

  it('gets batch sync status through the admin mailbox barrel', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: {
        batch_id: 'batch-1',
        queued_count: 1,
        running_count: 0,
        success_count: 2,
        partial_count: 0,
        failure_count: 0
      }
    } as any)

    expect(adminAPI.mailbox).toBe(mailboxAPI)

    await adminAPI.mailbox.getBatchSyncStatus('batch-1')

    expect(get).toHaveBeenCalledWith('/admin/mailbox/sync-jobs/batches/batch-1')
  })

  it('gets providers from the provider list endpoint', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { items: [], total: 0, page: 1, page_size: 50, pages: 1 }
    } as any)

    await mailboxApi.listProviders({ page: 2, page_size: 100 })

    expect(get).toHaveBeenCalledWith('/admin/mailbox/providers', {
      params: { page: 2, page_size: 100 }
    })
  })

  it('gets inbox headers from the inbox endpoint with real query params', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { items: [], total: 0, page: 1, page_size: 20, pages: 1 }
    } as any)

    const listInbox = (mailboxApi as Record<string, any>).listInbox

    expect(typeof listInbox).toBe('function')
    if (!listInbox) return

    await listInbox({
      collector_id: 1,
      capability_id: 2,
      folder: 'INBOX',
      page: 3,
      page_size: 20
    })

    expect(get).toHaveBeenCalledWith('/admin/mailbox/inbox', {
      params: {
        collector_id: 1,
        capability_id: 2,
        folder: 'INBOX',
        page: 3,
        page_size: 20
      }
    })
  })

  it('gets inbox header by id from the inbox header endpoint', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { id: 101, subject: 'header' }
    } as any)

    const getInboxHeader = (mailboxApi as Record<string, any>).getInboxHeader

    expect(typeof getInboxHeader).toBe('function')
    if (!getInboxHeader) return

    await getInboxHeader(101)

    expect(get).toHaveBeenCalledWith('/admin/mailbox/inbox/101')
  })

  it('gets inbox detail by id from the inbox detail endpoint through the admin mailbox barrel', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { id: 101, subject: 'detail', detail_fetch_state: 'succeeded' }
    } as any)

    expect(adminAPI.mailbox).toBe(mailboxAPI)
    expect(typeof (adminAPI.mailbox as Record<string, any>).getInboxDetail).toBe('function')
    if (!(adminAPI.mailbox as Record<string, any>).getInboxDetail) return

    await (adminAPI.mailbox as Record<string, any>).getInboxDetail(101)

    expect(get).toHaveBeenCalledWith('/admin/mailbox/inbox/101/detail')
  })

  it('posts provider creation to the provider endpoint', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { id: 1 } } as any)

    await mailboxApi.createProvider({
      display_name: 'Outlook Seed',
      provider_kind: 'microsoft',
      auth_kind: 'import_bundle',
      encrypted_payload: 'boss@example.com----provider-42----opaque-left----opaque-right'
    })

    expect(post).toHaveBeenCalledWith('/admin/mailbox/providers', {
      display_name: 'Outlook Seed',
      provider_kind: 'microsoft',
      auth_kind: 'import_bundle',
      encrypted_payload: 'boss@example.com----provider-42----opaque-left----opaque-right'
    })
  })

  it('gets collectors from the collector list endpoint', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { items: [], total: 0, page: 1, page_size: 50, pages: 1 }
    } as any)

    await mailboxApi.listCollectors({ page: 3, page_size: 75 })

    expect(get).toHaveBeenCalledWith('/admin/mailbox/collectors', {
      params: { page: 3, page_size: 75 }
    })
  })

  it('posts collector creation payload to the collector endpoint', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { id: 5 } } as any)

    await mailboxApi.createCollector({
      email_address: 'support@example.com',
      display_name: 'Support Inbox',
      enabled: true,
      business_tags: ['vip'],
      provider_account_id: 3,
      capabilities: [{ capability_kind: 'imap-primary' }]
    })

    expect(post).toHaveBeenCalledWith('/admin/mailbox/collectors', {
      email_address: 'support@example.com',
      display_name: 'Support Inbox',
      enabled: true,
      business_tags: ['vip'],
      provider_account_id: 3,
      capabilities: [{ capability_kind: 'imap-primary' }]
    })
  })

  it('puts collector updates to the collector endpoint', async () => {
    const put = vi.spyOn(apiClient, 'put').mockResolvedValue({ data: { id: 5 } } as any)

    await mailboxApi.updateCollector(5, {
      email_address: 'support@example.com',
      display_name: 'Support Inbox',
      enabled: false,
      business_tags: ['vip']
    })

    expect(put).toHaveBeenCalledWith('/admin/mailbox/collectors/5', {
      email_address: 'support@example.com',
      display_name: 'Support Inbox',
      enabled: false,
      business_tags: ['vip']
    })
  })

  it('posts capability creation payload to the capability endpoint', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { id: 8 } } as any)

    await mailboxApi.createCapability({
      provider_account_id: 3,
      collector_id: 5,
      capability_kind: 'imap-primary',
      connection_config: { folder: 'INBOX' },
      sync_enabled: true,
      sync_interval_seconds: 300
    })

    expect(post).toHaveBeenCalledWith('/admin/mailbox/capabilities', {
      provider_account_id: 3,
      collector_id: 5,
      capability_kind: 'imap-primary',
      connection_config: { folder: 'INBOX' },
      sync_enabled: true,
      sync_interval_seconds: 300
    })
  })

  it('puts capability updates to the capability endpoint', async () => {
    const put = vi.spyOn(apiClient, 'put').mockResolvedValue({ data: { id: 8 } } as any)

    await mailboxApi.updateCapability(8, {
      connection_config: { folder: 'Archive' },
      sync_enabled: false,
      sync_interval_seconds: 600
    })

    expect(put).toHaveBeenCalledWith('/admin/mailbox/capabilities/8', {
      connection_config: { folder: 'Archive' },
      sync_enabled: false,
      sync_interval_seconds: 600
    })
  })

  it('posts capability test requests to the capability test endpoint', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({
      data: { capability: { id: 8 }, result: { success: true } }
    } as any)

    await mailboxApi.testCapability(8)

    expect(post).toHaveBeenCalledWith('/admin/mailbox/capabilities/8/test')
  })

  it('posts collector sync requests to the collector sync endpoint', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { batch_id: 'sync-1' } } as any)

    await mailboxApi.syncCollector(5)

    expect(post).toHaveBeenCalledWith('/admin/mailbox/collectors/5/sync')
  })

  it('gets recipients from the recipient list endpoint', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { items: [], total: 0, page: 1, page_size: 50, pages: 1 }
    } as any)

    await mailboxApi.listRecipients({ page: 4, page_size: 80 })

    expect(get).toHaveBeenCalledWith('/admin/mailbox/recipients', {
      params: { page: 4, page_size: 80 }
    })
  })

  it('posts recipient creation to the recipient endpoint', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { id: 9 } } as any)

    await mailboxApi.createRecipient({
      name: 'Support Inbox',
      enabled: true,
      match_values: [{ match_type: 'exact_address', match_value: 'support@example.com' }]
    })

    expect(post).toHaveBeenCalledWith('/admin/mailbox/recipients', {
      name: 'Support Inbox',
      enabled: true,
      match_values: [{ match_type: 'exact_address', match_value: 'support@example.com' }]
    })
  })
})
