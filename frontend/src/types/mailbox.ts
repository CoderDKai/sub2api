export type MailboxProviderKind = 'basic' | 'microsoft'

export type MailboxProviderStatus = 'draft' | 'active' | 'invalid' | 'disabled'

export type MailboxCapabilityHealthState = 'healthy' | 'warning' | 'error' | 'paused' | 'syncing'

export type RecipientMatchType = 'exact_address' | 'domain_suffix'

export interface MailboxListParams {
  page?: number
  page_size?: number
}

export interface MailInboxListParams {
  collector_id?: number
  capability_id?: number
  folder?: string
  page?: number
  page_size?: number
}

export type MailDetailFetchState = 'not_requested' | 'succeeded' | 'failed'

export interface MailboxHeaderRecord {
  id: number
  collector_id: number
  capability_id: number
  remote_message_id: string
  folder: string
  sender: string | null
  recipients: string[]
  subject: string
  received_at: string
  flags: string[]
  snippet: string
  envelope_recipients: string[]
  delivered_to: string[]
  original_to: string[]
  resolved_recipient_identity_id: number | null
  resolved_address: string | null
  match_type: string | null
  matched_value_id: number | null
  resolution_source_field: string | null
  resolution_state: string
  detail_fetch_state: MailDetailFetchState
  created_at: string
  updated_at: string
}

export interface MailboxProviderAccount {
  id: number
  display_name: string
  provider_kind: MailboxProviderKind
  auth_kind: string
  status: MailboxProviderStatus
  mailbox_hint: string | null
  provider_identifier: string | null
  payload_version: number
  payload_configured: boolean
  payload_summary?: Record<string, unknown> | null
  last_imported_at: string | null
  last_validation_at: string | null
  last_validation_error: string | null
  created_at: string
  updated_at: string
  deleted_at: string | null
}

export interface CreateProviderPayload {
  display_name: string
  provider_kind: MailboxProviderKind
  auth_kind: string
  encrypted_payload: string
  status?: MailboxProviderStatus
}

export interface CollectorCapabilityInput {
  capability_kind: string
  connection_config?: Record<string, unknown>
  sync_enabled?: boolean
  sync_interval_seconds?: number
}

export interface CollectorMailbox {
  id: number
  email_address: string
  display_name: string
  enabled: boolean
  business_tags: string[]
  capabilities: MailboxCapability[]
  created_at: string
  updated_at: string
  deleted_at: string | null
}

export interface CreateCollectorPayload {
  email_address: string
  display_name: string
  enabled?: boolean
  business_tags?: string[]
  provider_account_id?: number
  capabilities?: CollectorCapabilityInput[]
}

export interface UpdateCollectorPayload {
  email_address: string
  display_name: string
  enabled?: boolean
  business_tags?: string[]
}

export interface MailboxCapability {
  id: number
  provider_account_id: number
  collector_id: number
  capability_kind: string
  sync_enabled: boolean
  sync_interval_seconds: number
  next_sync_at: string | null
  last_sync_at: string | null
  health_state: MailboxCapabilityHealthState
  last_error: string | null
  connection_configured: boolean
  connection_summary?: Record<string, unknown> | null
  connection_config?: Record<string, unknown>
  created_at: string
  updated_at: string
  deleted_at: string | null
}

export interface CreateCapabilityPayload {
  provider_account_id: number
  collector_id: number
  capability_kind: string
  connection_config?: Record<string, unknown>
  sync_enabled?: boolean
  sync_interval_seconds?: number
}

export interface UpdateCapabilityPayload {
  connection_config?: Record<string, unknown>
  sync_enabled?: boolean
  sync_interval_seconds?: number
  health_state?: MailboxCapabilityHealthState
}

export interface CapabilityHealthResult {
  success: boolean
  health_state: MailboxCapabilityHealthState | ''
  message: string
}

export interface TestCapabilityResult {
  capability: MailboxCapability | null
  result: CapabilityHealthResult
}

export interface RecipientMatchValue {
  id: number
  recipient_identity_id: number
  match_type: RecipientMatchType
  match_value: string
  normalized_value: string
  active: boolean
  priority: number
  source_kind: string
  source_metadata: Record<string, unknown>
  created_at: string
  updated_at: string
  disabled_at: string | null
}

export interface RecipientIdentity {
  id: number
  name: string
  normalized_name: string
  enabled: boolean
  match_values: RecipientMatchValue[]
  created_at: string
  updated_at: string
  deleted_at: string | null
}

export interface RecipientMatchValueInput {
  match_type: RecipientMatchType
  match_value: string
  active?: boolean
  priority?: number
  source_kind?: string
  source_metadata?: Record<string, unknown>
}

export interface CreateRecipientPayload {
  name: string
  enabled?: boolean
  match_values: RecipientMatchValueInput[]
}

export interface UpdateRecipientPayload {
  name: string
  enabled?: boolean
  match_values: RecipientMatchValueInput[]
}

export interface BatchSyncStatus {
  batch_id: string
  queued_count: number
  running_count: number
  success_count: number
  partial_count: number
  failure_count: number
  cancelled_count?: number
  jobs?: Array<Record<string, unknown>>
}

export interface ProviderValidationOutcome {
  account: MailboxProviderAccount | null
  code: string
  message: string
}

export interface MailboxBatchSyncPayload {
  collector_ids?: number[]
  capability_ids?: number[]
}

export interface UpdateProviderStatusPayload {
  status: Extract<MailboxProviderStatus, 'active' | 'disabled' | 'invalid'>
}
