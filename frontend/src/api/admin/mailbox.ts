import { apiClient } from '../client'
import type {
  BasePaginationResponse,
  BatchSyncStatus,
  CollectorMailbox,
  CreateCapabilityPayload,
  CreateCollectorPayload,
  CreateProviderPayload,
  CreateRecipientPayload,
  MailInboxListParams,
  MailboxHeaderRecord,
  MailboxListParams,
  MailboxBatchSyncPayload,
  MailboxCapability,
  MailboxProviderAccount,
  ProviderValidationOutcome,
  RecipientIdentity,
  TestCapabilityResult,
  UpdateCapabilityPayload,
  UpdateCollectorPayload,
  UpdateProviderStatusPayload,
  UpdateRecipientPayload
} from '@/types'

export async function listProviders(
  params?: MailboxListParams
): Promise<BasePaginationResponse<MailboxProviderAccount>> {
  const { data } = await apiClient.get<BasePaginationResponse<MailboxProviderAccount>>(
    '/admin/mailbox/providers',
    params ? { params } : undefined
  )
  return data
}

export async function listInbox(
  params?: MailInboxListParams
): Promise<BasePaginationResponse<MailboxHeaderRecord>> {
  const { data } = await apiClient.get<BasePaginationResponse<MailboxHeaderRecord>>(
    '/admin/mailbox/inbox',
    params ? { params } : undefined
  )
  return data
}

export async function getInboxHeader(id: number): Promise<MailboxHeaderRecord> {
  const { data } = await apiClient.get<MailboxHeaderRecord>(`/admin/mailbox/inbox/${id}`)
  return data
}

export async function getInboxDetail(id: number): Promise<MailboxHeaderRecord> {
  const { data } = await apiClient.get<MailboxHeaderRecord>(`/admin/mailbox/inbox/${id}/detail`)
  return data
}

export async function createProvider(
  payload: CreateProviderPayload
): Promise<MailboxProviderAccount> {
  const { data } = await apiClient.post<MailboxProviderAccount>(
    '/admin/mailbox/providers',
    payload
  )
  return data
}

export async function listCollectors(
  params?: MailboxListParams
): Promise<BasePaginationResponse<CollectorMailbox>> {
  const { data } = await apiClient.get<BasePaginationResponse<CollectorMailbox>>(
    '/admin/mailbox/collectors',
    params ? { params } : undefined
  )
  return data
}

export async function createCollector(
  payload: CreateCollectorPayload
): Promise<CollectorMailbox> {
  const { data } = await apiClient.post<CollectorMailbox>('/admin/mailbox/collectors', payload)
  return data
}

export async function updateCollector(
  collectorId: number,
  payload: UpdateCollectorPayload
): Promise<CollectorMailbox> {
  const { data } = await apiClient.put<CollectorMailbox>(
    `/admin/mailbox/collectors/${collectorId}`,
    payload
  )
  return data
}

export async function syncCollector(collectorId: number): Promise<{ batch_id: string }> {
  const { data } = await apiClient.post<{ batch_id: string }>(
    `/admin/mailbox/collectors/${collectorId}/sync`
  )
  return data
}

export async function createCapability(
  payload: CreateCapabilityPayload
): Promise<MailboxCapability> {
  const { data } = await apiClient.post<MailboxCapability>('/admin/mailbox/capabilities', payload)
  return data
}

export async function updateCapability(
  capabilityId: number,
  payload: UpdateCapabilityPayload
): Promise<MailboxCapability> {
  const { data } = await apiClient.put<MailboxCapability>(
    `/admin/mailbox/capabilities/${capabilityId}`,
    payload
  )
  return data
}

export async function testCapability(capabilityId: number): Promise<TestCapabilityResult> {
  const { data } = await apiClient.post<TestCapabilityResult>(
    `/admin/mailbox/capabilities/${capabilityId}/test`
  )
  return data
}

export async function batchSyncCollectors(
  payload: MailboxBatchSyncPayload
): Promise<{ batch_id: string }> {
  const { data } = await apiClient.post<{ batch_id: string }>(
    '/admin/mailbox/collectors/batch-sync',
    payload
  )
  return data
}

export async function listRecipients(
  params?: MailboxListParams
): Promise<BasePaginationResponse<RecipientIdentity>> {
  const { data } = await apiClient.get<BasePaginationResponse<RecipientIdentity>>(
    '/admin/mailbox/recipients',
    params ? { params } : undefined
  )
  return data
}

export async function createRecipient(
  payload: CreateRecipientPayload
): Promise<RecipientIdentity> {
  const { data } = await apiClient.post<RecipientIdentity>('/admin/mailbox/recipients', payload)
  return data
}

export async function importRecipientExactAddresses(
  recipientId: number,
  addresses: string[]
): Promise<{ imported: number }> {
  const { data } = await apiClient.post<{ imported: number }>(
    `/admin/mailbox/recipients/${recipientId}/import-exact-addresses`,
    { addresses }
  )
  return data
}

export async function updateRecipient(
  recipientId: number,
  payload: UpdateRecipientPayload
): Promise<RecipientIdentity> {
  const { data } = await apiClient.put<RecipientIdentity>(
    `/admin/mailbox/recipients/${recipientId}`,
    payload
  )
  return data
}

export async function validateProvider(
  providerId: number
): Promise<ProviderValidationOutcome> {
  const { data } = await apiClient.post<ProviderValidationOutcome>(
    `/admin/mailbox/providers/${providerId}/validate`
  )
  return data
}

export async function updateProviderStatus(
  providerId: number,
  payload: UpdateProviderStatusPayload
): Promise<MailboxProviderAccount> {
  const { data } = await apiClient.post<MailboxProviderAccount>(
    `/admin/mailbox/providers/${providerId}/status`,
    payload
  )
  return data
}

export async function getBatchSyncStatus(batchId: string): Promise<BatchSyncStatus> {
  const { data } = await apiClient.get<BatchSyncStatus>(
    `/admin/mailbox/sync-jobs/batches/${batchId}`
  )
  return data
}

const mailboxAPI = {
  listInbox,
  getInboxHeader,
  getInboxDetail,
  listProviders,
  createProvider,
  listCollectors,
  createCollector,
  updateCollector,
  syncCollector,
  createCapability,
  updateCapability,
  testCapability,
  batchSyncCollectors,
  listRecipients,
  createRecipient,
  importRecipientExactAddresses,
  updateRecipient,
  validateProvider,
  updateProviderStatus,
  getBatchSyncStatus
}

export default mailboxAPI
