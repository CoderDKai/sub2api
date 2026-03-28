-- Mailbox domain tables for provider imports, recipient resolution, header cache, and sync jobs.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS mailbox_provider_accounts (
    id BIGSERIAL PRIMARY KEY,
    provider_kind VARCHAR(32) NOT NULL,
    external_account_id VARCHAR(255) NOT NULL,
    encrypted_payload TEXT NOT NULL,
    payload_version INT NOT NULL DEFAULT 1,
    import_cursor TEXT,
    last_imported_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    UNIQUE(provider_kind, external_account_id)
);

CREATE TABLE IF NOT EXISTS mailbox_collectors (
    id BIGSERIAL PRIMARY KEY,
    provider_account_id BIGINT NOT NULL REFERENCES mailbox_provider_accounts(id) ON DELETE CASCADE,
    email_address VARCHAR(320) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_mailbox_collectors_provider_email_active
    ON mailbox_collectors (provider_account_id, email_address)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS mailbox_capabilities (
    id BIGSERIAL PRIMARY KEY,
    collector_id BIGINT NOT NULL REFERENCES mailbox_collectors(id) ON DELETE CASCADE,
    capability_kind VARCHAR(32) NOT NULL,
    folder VARCHAR(128) NOT NULL DEFAULT 'INBOX',
    sync_state VARCHAR(32) NOT NULL DEFAULT 'pending',
    import_cursor TEXT,
    last_synced_at TIMESTAMPTZ,
    next_sync_due_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_mailbox_capabilities_collector_kind_folder_active
    ON mailbox_capabilities (collector_id, capability_kind, folder)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_mailbox_capabilities_sync_due
    ON mailbox_capabilities (next_sync_due_at)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS mailbox_recipient_identities (
    id BIGSERIAL PRIMARY KEY,
    collector_id BIGINT NOT NULL REFERENCES mailbox_collectors(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    normalized_name VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_mailbox_recipient_identities_collector_id
    ON mailbox_recipient_identities (collector_id)
    WHERE deleted_at IS NULL;

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
    WHERE match_type = 'exact' AND active = TRUE;

CREATE TABLE IF NOT EXISTS mailbox_header_cache (
    id BIGSERIAL PRIMARY KEY,
    capability_id BIGINT NOT NULL REFERENCES mailbox_capabilities(id) ON DELETE CASCADE,
    matched_value_id BIGINT REFERENCES mailbox_recipient_match_values(id) ON DELETE SET NULL,
    folder VARCHAR(128) NOT NULL,
    remote_message_id VARCHAR(255) NOT NULL,
    message_id VARCHAR(255),
    received_at TIMESTAMPTZ NOT NULL,
    snippet TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    from_address TEXT,
    resolved_address TEXT,
    envelope_recipients JSONB NOT NULL DEFAULT '[]'::jsonb,
    delivered_to TEXT,
    original_to TEXT,
    match_type VARCHAR(32),
    resolution_source_field VARCHAR(64),
    resolution_state VARCHAR(32) NOT NULL DEFAULT 'pending',
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
    batch_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'queued',
    sync_reason VARCHAR(32) NOT NULL DEFAULT 'scheduled',
    scheduled_for TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    retryable BOOLEAN NOT NULL DEFAULT FALSE,
    retry_count INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    backoff_seconds INT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mailbox_sync_jobs_batch_id
    ON mailbox_sync_jobs (batch_id);

CREATE INDEX IF NOT EXISTS idx_mailbox_sync_jobs_status_scheduled_for
    ON mailbox_sync_jobs (status, scheduled_for);

CREATE INDEX IF NOT EXISTS idx_mailbox_sync_jobs_next_retry_at
    ON mailbox_sync_jobs (next_retry_at)
    WHERE retryable = TRUE;
