-- Mailbox domain tables for provider accounts, collectors, recipient matching, headers, and sync jobs.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS mailbox_provider_accounts (
    id BIGSERIAL PRIMARY KEY,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    provider_kind VARCHAR(32) NOT NULL,
    auth_kind VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    encrypted_payload TEXT NOT NULL,
    mailbox_hint VARCHAR(320),
    provider_identifier VARCHAR(255) NOT NULL,
    payload_version INT NOT NULL DEFAULT 1,
    last_imported_at TIMESTAMPTZ,
    last_validation_at TIMESTAMPTZ,
    last_validation_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_mailbox_provider_accounts_provider_external_active
    ON mailbox_provider_accounts (provider_kind, provider_identifier)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS mailbox_collectors (
    id BIGSERIAL PRIMARY KEY,
    email_address VARCHAR(320) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_mailbox_collectors_email_active
    ON mailbox_collectors (email_address)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS mailbox_capabilities (
    id BIGSERIAL PRIMARY KEY,
    provider_account_id BIGINT NOT NULL REFERENCES mailbox_provider_accounts(id) ON DELETE CASCADE,
    collector_id BIGINT NOT NULL REFERENCES mailbox_collectors(id) ON DELETE CASCADE,
    capability_kind VARCHAR(32) NOT NULL,
    folder VARCHAR(128) NOT NULL DEFAULT 'INBOX',
    state VARCHAR(32) NOT NULL DEFAULT 'pending',
    import_cursor TEXT,
    last_synced_at TIMESTAMPTZ,
    sync_due_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_mailbox_capabilities_provider_collector_kind_folder_active
    ON mailbox_capabilities (provider_account_id, collector_id, capability_kind, folder)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_mailbox_capabilities_sync_due
    ON mailbox_capabilities (sync_due_at)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS mailbox_recipient_identities (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    normalized_name VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS mailbox_recipient_match_values (
    id BIGSERIAL PRIMARY KEY,
    recipient_identity_id BIGINT NOT NULL REFERENCES mailbox_recipient_identities(id) ON DELETE CASCADE,
    match_type VARCHAR(32) NOT NULL,
    match_value TEXT NOT NULL,
    normalized_value TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    priority INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    disabled_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_mailbox_recipient_match_values_identity_id
    ON mailbox_recipient_match_values (recipient_identity_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_mailbox_recipient_exact_active
    ON mailbox_recipient_match_values (normalized_value)
    WHERE match_type = 'exact_address' AND active = TRUE;

CREATE TABLE IF NOT EXISTS mailbox_header_cache (
    id BIGSERIAL PRIMARY KEY,
    collector_id BIGINT NOT NULL REFERENCES mailbox_collectors(id) ON DELETE CASCADE,
    capability_id BIGINT NOT NULL REFERENCES mailbox_capabilities(id) ON DELETE CASCADE,
    remote_message_id VARCHAR(255) NOT NULL,
    folder VARCHAR(128) NOT NULL,
    sender TEXT,
    recipients JSONB NOT NULL DEFAULT '[]'::jsonb,
    subject TEXT NOT NULL DEFAULT '',
    received_at TIMESTAMPTZ NOT NULL,
    flags JSONB NOT NULL DEFAULT '[]'::jsonb,
    snippet TEXT NOT NULL DEFAULT '',
    envelope_recipients JSONB NOT NULL DEFAULT '[]'::jsonb,
    delivered_to JSONB NOT NULL DEFAULT '[]'::jsonb,
    original_to JSONB NOT NULL DEFAULT '[]'::jsonb,
    resolved_recipient_identity_id BIGINT REFERENCES mailbox_recipient_identities(id) ON DELETE SET NULL,
    resolved_address TEXT,
    match_type VARCHAR(32),
    matched_value_id BIGINT REFERENCES mailbox_recipient_match_values(id) ON DELETE SET NULL,
    resolution_source_field VARCHAR(64),
    resolution_state VARCHAR(32) NOT NULL DEFAULT 'pending',
    detail_fetch_state VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_mailbox_header_capability_folder_remote
    ON mailbox_header_cache (capability_id, folder, remote_message_id);

CREATE INDEX IF NOT EXISTS idx_mailbox_header_cache_received_at
    ON mailbox_header_cache (received_at DESC);

CREATE TABLE IF NOT EXISTS mailbox_sync_jobs (
    id BIGSERIAL PRIMARY KEY,
    capability_id BIGINT NOT NULL REFERENCES mailbox_capabilities(id) ON DELETE CASCADE,
    batch_id VARCHAR(64),
    state VARCHAR(32) NOT NULL DEFAULT 'queued',
    trigger_source VARCHAR(32) NOT NULL DEFAULT 'scheduled',
    scheduled_for TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    retryable BOOLEAN NOT NULL DEFAULT FALSE,
    retry_count INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    error_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mailbox_sync_jobs_batch_id
    ON mailbox_sync_jobs (batch_id);

CREATE INDEX IF NOT EXISTS idx_mailbox_sync_jobs_state_scheduled_for
    ON mailbox_sync_jobs (state, scheduled_for);

CREATE INDEX IF NOT EXISTS idx_mailbox_sync_jobs_next_retry_at
    ON mailbox_sync_jobs (next_retry_at)
    WHERE retryable = TRUE;
