//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate(t *testing.T) {
	tx := testTx(t)

	// Re-apply migrations to verify idempotency (no errors, no duplicate rows).
	require.NoError(t, ApplyMigrations(context.Background(), integrationDB))

	// schema_migrations should have at least the current migration set.
	var applied int
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM schema_migrations").Scan(&applied))
	require.GreaterOrEqual(t, applied, 7, "expected schema_migrations to contain applied migrations")

	// users: columns required by repository queries
	requireColumn(t, tx, "users", "username", "character varying", 100, false)
	requireColumn(t, tx, "users", "notes", "text", 0, false)

	// accounts: schedulable and rate-limit fields
	requireColumn(t, tx, "accounts", "notes", "text", 0, true)
	requireColumn(t, tx, "accounts", "schedulable", "boolean", 0, false)
	requireColumn(t, tx, "accounts", "rate_limited_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "accounts", "rate_limit_reset_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "accounts", "overload_until", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "accounts", "session_window_status", "character varying", 20, true)

	// api_keys: key length should be 128
	requireColumn(t, tx, "api_keys", "key", "character varying", 128, false)

	// redeem_codes: subscription fields
	requireColumn(t, tx, "redeem_codes", "group_id", "bigint", 0, true)
	requireColumn(t, tx, "redeem_codes", "validity_days", "integer", 0, false)

	// usage_logs: billing_type used by filters/stats
	requireColumn(t, tx, "usage_logs", "billing_type", "smallint", 0, false)
	requireColumn(t, tx, "usage_logs", "request_type", "smallint", 0, false)
	requireColumn(t, tx, "usage_logs", "openai_ws_mode", "boolean", 0, false)

	// usage_billing_dedup: billing idempotency narrow table
	var usageBillingDedupRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.usage_billing_dedup')").Scan(&usageBillingDedupRegclass))
	require.True(t, usageBillingDedupRegclass.Valid, "expected usage_billing_dedup table to exist")
	requireColumn(t, tx, "usage_billing_dedup", "request_fingerprint", "character varying", 64, false)
	requireIndex(t, tx, "usage_billing_dedup", "idx_usage_billing_dedup_request_api_key")
	requireIndex(t, tx, "usage_billing_dedup", "idx_usage_billing_dedup_created_at_brin")

	var usageBillingDedupArchiveRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.usage_billing_dedup_archive')").Scan(&usageBillingDedupArchiveRegclass))
	require.True(t, usageBillingDedupArchiveRegclass.Valid, "expected usage_billing_dedup_archive table to exist")
	requireColumn(t, tx, "usage_billing_dedup_archive", "request_fingerprint", "character varying", 64, false)
	requireIndex(t, tx, "usage_billing_dedup_archive", "usage_billing_dedup_archive_pkey")

	// settings table should exist
	var settingsRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.settings')").Scan(&settingsRegclass))
	require.True(t, settingsRegclass.Valid, "expected settings table to exist")

	// security_secrets table should exist
	var securitySecretsRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.security_secrets')").Scan(&securitySecretsRegclass))
	require.True(t, securitySecretsRegclass.Valid, "expected security_secrets table to exist")

	// user_allowed_groups table should exist
	var uagRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.user_allowed_groups')").Scan(&uagRegclass))
	require.True(t, uagRegclass.Valid, "expected user_allowed_groups table to exist")

	// user_subscriptions: deleted_at for soft delete support (migration 012)
	requireColumn(t, tx, "user_subscriptions", "deleted_at", "timestamp with time zone", 0, true)

	// orphan_allowed_groups_audit table should exist (migration 013)
	var orphanAuditRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.orphan_allowed_groups_audit')").Scan(&orphanAuditRegclass))
	require.True(t, orphanAuditRegclass.Valid, "expected orphan_allowed_groups_audit table to exist")

	// account_groups: created_at should be timestamptz
	requireColumn(t, tx, "account_groups", "created_at", "timestamp with time zone", 0, false)

	// user_allowed_groups: created_at should be timestamptz
	requireColumn(t, tx, "user_allowed_groups", "created_at", "timestamp with time zone", 0, false)
}

func TestMigrationsRunner_CreatesMailboxDomainTables(t *testing.T) {
	tx := testTx(t)

	requireTableExists(t, tx, "mailbox_provider_accounts")
	requireColumnExists(t, tx, "mailbox_provider_accounts", "display_name")
	requireColumnExists(t, tx, "mailbox_provider_accounts", "provider_kind")
	requireColumnExists(t, tx, "mailbox_provider_accounts", "auth_kind")
	requireColumnExists(t, tx, "mailbox_provider_accounts", "status")
	requireColumnExists(t, tx, "mailbox_provider_accounts", "encrypted_payload")
	requireColumnExists(t, tx, "mailbox_provider_accounts", "mailbox_hint")
	requireColumn(t, tx, "mailbox_provider_accounts", "provider_identifier", "character varying", 255, true)
	requireColumnExists(t, tx, "mailbox_provider_accounts", "last_imported_at")
	requireColumnExists(t, tx, "mailbox_provider_accounts", "last_validation_at")
	requireColumnExists(t, tx, "mailbox_provider_accounts", "last_validation_error")
	requireColumnDefaultContains(t, tx, "mailbox_provider_accounts", "status", "draft")
	requireIndex(t, tx, "mailbox_provider_accounts", "uq_mailbox_provider_accounts_provider_external_active")
	requireIndexDefinitionContains(t, tx, "uq_mailbox_provider_accounts_provider_external_active", "provider_kind", "provider_identifier", "deleted_at")

	requireTableExists(t, tx, "mailbox_collectors")
	requireColumnExists(t, tx, "mailbox_collectors", "email_address")
	requireColumnNotExists(t, tx, "mailbox_collectors", "provider_account_id")

	requireTableExists(t, tx, "mailbox_capabilities")
	requireColumnExists(t, tx, "mailbox_capabilities", "provider_account_id")
	requireColumnDefaultContains(t, tx, "mailbox_capabilities", "state", "healthy")
	requireIndex(t, tx, "mailbox_capabilities", "idx_mailbox_capabilities_sync_due")

	requireTableExists(t, tx, "mailbox_recipient_identities")
	requireColumnExists(t, tx, "mailbox_recipient_identities", "name")
	requireColumnNotExists(t, tx, "mailbox_recipient_identities", "collector_id")

	requireTableExists(t, tx, "mailbox_recipient_match_values")
	requireColumnExists(t, tx, "mailbox_recipient_match_values", "recipient_identity_id")
	requireColumnNotExists(t, tx, "mailbox_recipient_match_values", "collector_id")
	requireIndex(t, tx, "mailbox_recipient_match_values", "uq_mailbox_recipient_exact_active")
	requireIndexDefinitionContains(t, tx, "uq_mailbox_recipient_exact_active", "normalized_value", "exact_address", "active")

	requireTableExists(t, tx, "mailbox_header_cache")
	requireColumnExists(t, tx, "mailbox_header_cache", "collector_id")
	requireColumnExists(t, tx, "mailbox_header_cache", "capability_id")
	requireColumnExists(t, tx, "mailbox_header_cache", "remote_message_id")
	requireColumnExists(t, tx, "mailbox_header_cache", "folder")
	requireColumnExists(t, tx, "mailbox_header_cache", "sender")
	requireColumn(t, tx, "mailbox_header_cache", "recipients", "jsonb", 0, false)
	requireColumnExists(t, tx, "mailbox_header_cache", "subject")
	requireColumnExists(t, tx, "mailbox_header_cache", "received_at")
	requireColumn(t, tx, "mailbox_header_cache", "flags", "jsonb", 0, false)
	requireColumnExists(t, tx, "mailbox_header_cache", "snippet")
	requireColumnExists(t, tx, "mailbox_header_cache", "resolved_address")
	requireColumn(t, tx, "mailbox_header_cache", "envelope_recipients", "jsonb", 0, false)
	requireColumn(t, tx, "mailbox_header_cache", "delivered_to", "jsonb", 0, false)
	requireColumn(t, tx, "mailbox_header_cache", "original_to", "jsonb", 0, false)
	requireColumnExists(t, tx, "mailbox_header_cache", "resolved_recipient_identity_id")
	requireColumnExists(t, tx, "mailbox_header_cache", "match_type")
	requireColumnExists(t, tx, "mailbox_header_cache", "matched_value_id")
	requireColumnExists(t, tx, "mailbox_header_cache", "resolution_source_field")
	requireColumnExists(t, tx, "mailbox_header_cache", "resolution_state")
	requireColumnExists(t, tx, "mailbox_header_cache", "detail_fetch_state")
	requireColumnDefaultContains(t, tx, "mailbox_header_cache", "resolution_state", "unresolved")
	requireColumnDefaultContains(t, tx, "mailbox_header_cache", "detail_fetch_state", "not_requested")
	requireIndex(t, tx, "mailbox_header_cache", "uq_mailbox_header_capability_folder_remote")

	requireTableExists(t, tx, "mailbox_sync_jobs")
	requireColumn(t, tx, "mailbox_sync_jobs", "batch_id", "character varying", 64, true)
	requireColumnExists(t, tx, "mailbox_sync_jobs", "state")
	requireColumnExists(t, tx, "mailbox_sync_jobs", "trigger_source")
	requireColumnExists(t, tx, "mailbox_sync_jobs", "retryable")
	requireColumnExists(t, tx, "mailbox_sync_jobs", "retry_count")
	requireColumnExists(t, tx, "mailbox_sync_jobs", "next_retry_at")
	requireColumnExists(t, tx, "mailbox_sync_jobs", "error_summary")
	requireColumnDefaultContains(t, tx, "mailbox_sync_jobs", "state", "queued")
}

func requireTableExists(t *testing.T, tx *sql.Tx, table string) {
	t.Helper()

	var regclass sql.NullString
	err := tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.' || $1)", table).Scan(&regclass)
	require.NoError(t, err, "query to_regclass for %s", table)
	require.True(t, regclass.Valid, "expected %s table to exist", table)
}

func requireIndex(t *testing.T, tx *sql.Tx, table, index string) {
	t.Helper()

	var exists bool
	err := tx.QueryRowContext(context.Background(), `
SELECT EXISTS (
	SELECT 1
	FROM pg_indexes
	WHERE schemaname = 'public'
	  AND tablename = $1
	  AND indexname = $2
)
`, table, index).Scan(&exists)
	require.NoError(t, err, "query pg_indexes for %s.%s", table, index)
	require.True(t, exists, "expected index %s on %s", index, table)
}

func requireIndexDefinitionContains(t *testing.T, tx *sql.Tx, index string, parts ...string) {
	t.Helper()

	var definition string
	err := tx.QueryRowContext(context.Background(), `
SELECT indexdef
FROM pg_indexes
WHERE schemaname = 'public'
  AND indexname = $1
`, index).Scan(&definition)
	require.NoError(t, err, "query pg_indexes definition for %s", index)

	for _, part := range parts {
		require.Contains(t, definition, part, "expected index %s definition to contain %q", index, part)
	}
}

func requireColumn(t *testing.T, tx *sql.Tx, table, column, dataType string, maxLen int, nullable bool) {
	t.Helper()

	var row struct {
		DataType string
		MaxLen   sql.NullInt64
		Nullable string
	}

	err := tx.QueryRowContext(context.Background(), `
SELECT
  data_type,
  character_maximum_length,
  is_nullable
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = $1
  AND column_name = $2
`, table, column).Scan(&row.DataType, &row.MaxLen, &row.Nullable)
	require.NoError(t, err, "query information_schema.columns for %s.%s", table, column)
	require.Equal(t, dataType, row.DataType, "data_type mismatch for %s.%s", table, column)

	if maxLen > 0 {
		require.True(t, row.MaxLen.Valid, "expected maxLen for %s.%s", table, column)
		require.Equal(t, int64(maxLen), row.MaxLen.Int64, "maxLen mismatch for %s.%s", table, column)
	}

	if nullable {
		require.Equal(t, "YES", row.Nullable, "nullable mismatch for %s.%s", table, column)
	} else {
		require.Equal(t, "NO", row.Nullable, "nullable mismatch for %s.%s", table, column)
	}
}

func requireColumnExists(t *testing.T, tx *sql.Tx, table, column string) {
	t.Helper()

	var exists bool
	err := tx.QueryRowContext(context.Background(), `
SELECT EXISTS (
	SELECT 1
	FROM information_schema.columns
	WHERE table_schema = 'public'
	  AND table_name = $1
	  AND column_name = $2
)
`, table, column).Scan(&exists)
	require.NoError(t, err, "query information_schema.columns for %s.%s", table, column)
	require.True(t, exists, "expected column %s on %s", column, table)
}

func requireColumnDefaultContains(t *testing.T, tx *sql.Tx, table, column, fragment string) {
	t.Helper()

	var defaultValue sql.NullString
	err := tx.QueryRowContext(context.Background(), `
SELECT column_default
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = $1
  AND column_name = $2
`, table, column).Scan(&defaultValue)
	require.NoError(t, err, "query information_schema.columns default for %s.%s", table, column)
	require.True(t, defaultValue.Valid, "expected default for %s.%s", table, column)
	require.Contains(t, defaultValue.String, fragment, "expected default for %s.%s to contain %q", table, column, fragment)
}

func requireColumnNotExists(t *testing.T, tx *sql.Tx, table, column string) {
	t.Helper()

	var exists bool
	err := tx.QueryRowContext(context.Background(), `
SELECT EXISTS (
	SELECT 1
	FROM information_schema.columns
	WHERE table_schema = 'public'
	  AND table_name = $1
	  AND column_name = $2
)
`, table, column).Scan(&exists)
	require.NoError(t, err, "query information_schema.columns for %s.%s", table, column)
	require.False(t, exists, "expected column %s on %s to be absent", column, table)
}
