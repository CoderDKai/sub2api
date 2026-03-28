package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	defaultMailboxCapabilitySyncInterval = 300
	defaultMailboxHeaderListLimit        = 50
	defaultMailboxClaimLimit             = 100
)

type mailboxRepository struct {
	db *sql.DB
}

type sqlScanRow interface {
	Scan(dest ...any) error
}

func NewMailboxRepository(db *sql.DB) service.MailboxRepository {
	return &mailboxRepository{db: db}
}

func (r *mailboxRepository) CreateProviderAccount(ctx context.Context, account *service.ProviderAccount) (*service.ProviderAccount, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}
	if account == nil {
		return nil, errors.New("provider account is nil")
	}

	status := account.Status
	if status == "" {
		status = service.ProviderAccountStatusDraft
	}
	payloadVersion := account.PayloadVersion
	if payloadVersion == 0 {
		payloadVersion = 1
	}

	row := r.db.QueryRowContext(ctx, `
		INSERT INTO mailbox_provider_accounts (
			display_name,
			provider_kind,
			auth_kind,
			status,
			encrypted_payload,
			mailbox_hint,
			provider_identifier,
			payload_version,
			last_imported_at,
			last_validation_at,
			last_validation_error
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING
			id,
			display_name,
			provider_kind,
			auth_kind,
			status,
			encrypted_payload,
			mailbox_hint,
			provider_identifier,
			payload_version,
			last_imported_at,
			last_validation_at,
			last_validation_error,
			created_at,
			updated_at,
			deleted_at
	`, account.DisplayName, account.ProviderKind, account.AuthKind, status, account.EncryptedPayload, account.MailboxHint, account.ProviderIdentifier, payloadVersion, account.LastImportedAt, account.LastValidationAt, account.LastValidationError)

	created, err := scanProviderAccount(row)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *mailboxRepository) GetProviderAccountByID(ctx context.Context, id int64) (*service.ProviderAccount, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			display_name,
			provider_kind,
			auth_kind,
			status,
			encrypted_payload,
			mailbox_hint,
			provider_identifier,
			payload_version,
			last_imported_at,
			last_validation_at,
			last_validation_error,
			created_at,
			updated_at,
			deleted_at
		FROM mailbox_provider_accounts
		WHERE id = $1
	`, id)

	return scanProviderAccount(row)
}

func (r *mailboxRepository) UpdateProviderAccount(ctx context.Context, account *service.ProviderAccount) (*service.ProviderAccount, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}
	if account == nil {
		return nil, errors.New("provider account is nil")
	}

	status := account.Status
	if status == "" {
		status = service.ProviderAccountStatusDraft
	}
	payloadVersion := account.PayloadVersion
	if payloadVersion == 0 {
		payloadVersion = 1
	}

	row := r.db.QueryRowContext(ctx, `
		UPDATE mailbox_provider_accounts
		SET
			display_name = $2,
			provider_kind = $3,
			auth_kind = $4,
			status = $5,
			encrypted_payload = $6,
			mailbox_hint = $7,
			provider_identifier = $8,
			payload_version = $9,
			last_imported_at = $10,
			last_validation_at = $11,
			last_validation_error = $12,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			display_name,
			provider_kind,
			auth_kind,
			status,
			encrypted_payload,
			mailbox_hint,
			provider_identifier,
			payload_version,
			last_imported_at,
			last_validation_at,
			last_validation_error,
			created_at,
			updated_at,
			deleted_at
	`, account.ID, account.DisplayName, account.ProviderKind, account.AuthKind, status, account.EncryptedPayload, account.MailboxHint, account.ProviderIdentifier, payloadVersion, account.LastImportedAt, account.LastValidationAt, account.LastValidationError)

	return scanProviderAccount(row)
}

func (r *mailboxRepository) ListProviderAccounts(ctx context.Context, includeDeleted bool, limit int) ([]*service.ProviderAccount, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}

	query := `
		SELECT
			id,
			display_name,
			provider_kind,
			auth_kind,
			status,
			encrypted_payload,
			mailbox_hint,
			provider_identifier,
			payload_version,
			last_imported_at,
			last_validation_at,
			last_validation_error,
			created_at,
			updated_at,
			deleted_at
		FROM mailbox_provider_accounts`
	args := make([]any, 0, 1)
	if !includeDeleted {
		query += ` WHERE deleted_at IS NULL`
	}
	query += ` ORDER BY id ASC LIMIT $1`
	args = append(args, normalizeMailboxListLimit(limit))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	accounts := make([]*service.ProviderAccount, 0)
	for rows.Next() {
		account, err := scanProviderAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *mailboxRepository) DeleteProviderAccount(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return errors.New("mailbox repository db is nil")
	}

	res, err := r.db.ExecContext(ctx, `
		UPDATE mailbox_provider_accounts
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(res)
}

func (r *mailboxRepository) CreateCollector(ctx context.Context, collector *service.CollectorMailbox) (*service.CollectorMailbox, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}
	if collector == nil {
		return nil, errors.New("collector is nil")
	}

	businessTags, err := marshalJSONB(collector.BusinessTags, []byte("[]"))
	if err != nil {
		return nil, err
	}

	row := r.db.QueryRowContext(ctx, `
		INSERT INTO mailbox_collectors (
			email_address,
			display_name,
			enabled,
			business_tags
		) VALUES ($1, $2, $3, $4::jsonb)
		RETURNING
			id,
			email_address,
			display_name,
			enabled,
			business_tags,
			created_at,
			updated_at,
			deleted_at
	`, collector.EmailAddress, collector.DisplayName, collector.Enabled, string(businessTags))

	created, err := scanCollector(row)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *mailboxRepository) GetCollectorByID(ctx context.Context, id int64) (*service.CollectorMailbox, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			email_address,
			display_name,
			enabled,
			business_tags,
			created_at,
			updated_at,
			deleted_at
		FROM mailbox_collectors
		WHERE id = $1
	`, id)

	return scanCollector(row)
}

func (r *mailboxRepository) UpdateCollector(ctx context.Context, collector *service.CollectorMailbox) (*service.CollectorMailbox, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}
	if collector == nil {
		return nil, errors.New("collector is nil")
	}

	businessTags, err := marshalJSONB(collector.BusinessTags, []byte("[]"))
	if err != nil {
		return nil, err
	}

	row := r.db.QueryRowContext(ctx, `
		UPDATE mailbox_collectors
		SET
			email_address = $2,
			display_name = $3,
			enabled = $4,
			business_tags = $5::jsonb,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			email_address,
			display_name,
			enabled,
			business_tags,
			created_at,
			updated_at,
			deleted_at
	`, collector.ID, collector.EmailAddress, collector.DisplayName, collector.Enabled, string(businessTags))

	return scanCollector(row)
}

func (r *mailboxRepository) ListCollectors(ctx context.Context, includeDeleted bool, limit int) ([]*service.CollectorMailbox, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}

	query := `
		SELECT
			id,
			email_address,
			display_name,
			enabled,
			business_tags,
			created_at,
			updated_at,
			deleted_at
		FROM mailbox_collectors`
	if !includeDeleted {
		query += ` WHERE deleted_at IS NULL`
	}
	query += ` ORDER BY id ASC LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, normalizeMailboxListLimit(limit))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	collectors := make([]*service.CollectorMailbox, 0)
	for rows.Next() {
		collector, err := scanCollector(rows)
		if err != nil {
			return nil, err
		}
		collectors = append(collectors, collector)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return collectors, nil
}

func (r *mailboxRepository) DeleteCollector(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return errors.New("mailbox repository db is nil")
	}

	res, err := r.db.ExecContext(ctx, `
		UPDATE mailbox_collectors
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(res)
}

func (r *mailboxRepository) CreateCapability(ctx context.Context, capability *service.MailboxCapability) (*service.MailboxCapability, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}
	if capability == nil {
		return nil, errors.New("capability is nil")
	}

	connectionConfig, err := marshalJSONB(capability.ConnectionConfig, []byte("{}"))
	if err != nil {
		return nil, err
	}
	cursorState, err := marshalJSONB(capability.CursorState, []byte("{}"))
	if err != nil {
		return nil, err
	}
	syncInterval := capability.SyncIntervalSeconds
	if syncInterval <= 0 {
		syncInterval = defaultMailboxCapabilitySyncInterval
	}
	healthState := capability.HealthState
	if healthState == "" {
		healthState = service.MailboxCapabilityStateHealthy
	}

	row := r.db.QueryRowContext(ctx, `
		INSERT INTO mailbox_capabilities (
			provider_account_id,
			collector_id,
			capability_kind,
			connection_config,
			cursor_state,
			sync_enabled,
			sync_interval_seconds,
			next_sync_at,
			last_sync_at,
			health_state,
			last_error
		) VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6, $7, $8, $9, $10, $11)
		RETURNING
			id,
			provider_account_id,
			collector_id,
			capability_kind,
			connection_config,
			cursor_state,
			sync_enabled,
			sync_interval_seconds,
			next_sync_at,
			last_sync_at,
			health_state,
			last_error,
			created_at,
			updated_at,
			deleted_at
	`, capability.ProviderAccountID, capability.CollectorID, capability.CapabilityKind, string(connectionConfig), string(cursorState), capability.SyncEnabled, syncInterval, capability.NextSyncAt, capability.LastSyncAt, healthState, capability.LastError)

	created, err := scanMailboxCapability(row)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *mailboxRepository) GetCapabilityByID(ctx context.Context, id int64) (*service.MailboxCapability, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			provider_account_id,
			collector_id,
			capability_kind,
			connection_config,
			cursor_state,
			sync_enabled,
			sync_interval_seconds,
			next_sync_at,
			last_sync_at,
			health_state,
			last_error,
			created_at,
			updated_at,
			deleted_at
		FROM mailbox_capabilities
		WHERE id = $1
	`, id)

	return scanMailboxCapability(row)
}

func (r *mailboxRepository) UpdateCapability(ctx context.Context, capability *service.MailboxCapability) (*service.MailboxCapability, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}
	if capability == nil {
		return nil, errors.New("capability is nil")
	}

	connectionConfig, err := marshalJSONB(capability.ConnectionConfig, []byte("{}"))
	if err != nil {
		return nil, err
	}
	cursorState, err := marshalJSONB(capability.CursorState, []byte("{}"))
	if err != nil {
		return nil, err
	}
	syncInterval := capability.SyncIntervalSeconds
	if syncInterval <= 0 {
		syncInterval = defaultMailboxCapabilitySyncInterval
	}
	healthState := capability.HealthState
	if healthState == "" {
		healthState = service.MailboxCapabilityStateHealthy
	}

	row := r.db.QueryRowContext(ctx, `
		UPDATE mailbox_capabilities
		SET
			provider_account_id = $2,
			collector_id = $3,
			capability_kind = $4,
			connection_config = $5::jsonb,
			cursor_state = $6::jsonb,
			sync_enabled = $7,
			sync_interval_seconds = $8,
			next_sync_at = $9,
			last_sync_at = $10,
			health_state = $11,
			last_error = $12,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			provider_account_id,
			collector_id,
			capability_kind,
			connection_config,
			cursor_state,
			sync_enabled,
			sync_interval_seconds,
			next_sync_at,
			last_sync_at,
			health_state,
			last_error,
			created_at,
			updated_at,
			deleted_at
	`, capability.ID, capability.ProviderAccountID, capability.CollectorID, capability.CapabilityKind, string(connectionConfig), string(cursorState), capability.SyncEnabled, syncInterval, capability.NextSyncAt, capability.LastSyncAt, healthState, capability.LastError)

	return scanMailboxCapability(row)
}

func (r *mailboxRepository) ListCapabilities(ctx context.Context, includeDeleted bool, limit int) ([]*service.MailboxCapability, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}

	query := `
		SELECT
			id,
			provider_account_id,
			collector_id,
			capability_kind,
			connection_config,
			cursor_state,
			sync_enabled,
			sync_interval_seconds,
			next_sync_at,
			last_sync_at,
			health_state,
			last_error,
			created_at,
			updated_at,
			deleted_at
		FROM mailbox_capabilities`
	if !includeDeleted {
		query += ` WHERE deleted_at IS NULL`
	}
	query += ` ORDER BY id ASC LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, normalizeMailboxListLimit(limit))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	capabilities := make([]*service.MailboxCapability, 0)
	for rows.Next() {
		capability, err := scanMailboxCapability(rows)
		if err != nil {
			return nil, err
		}
		capabilities = append(capabilities, capability)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return capabilities, nil
}

func (r *mailboxRepository) DeleteCapability(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return errors.New("mailbox repository db is nil")
	}

	res, err := r.db.ExecContext(ctx, `
		UPDATE mailbox_capabilities
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(res)
}

func (r *mailboxRepository) CreateRecipientIdentity(ctx context.Context, in *service.RecipientIdentity, values []*service.RecipientMatchValue) (*service.RecipientIdentity, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}
	if in == nil {
		return nil, errors.New("recipient identity is nil")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		INSERT INTO mailbox_recipient_identities (
			name,
			normalized_name,
			enabled
		) VALUES ($1, $2, $3)
		RETURNING id, name, normalized_name, enabled, created_at, updated_at, deleted_at
	`, in.Name, in.NormalizedName, in.Enabled)

	created, err := scanRecipientIdentity(row)
	if err != nil {
		return nil, err
	}

	for _, value := range values {
		if value == nil {
			continue
		}
		sourceKind := value.SourceKind
		if sourceKind == "" {
			sourceKind = "manual"
		}
		sourceMetadata, err := marshalJSONB(value.SourceMetadata, []byte("{}"))
		if err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO mailbox_recipient_match_values (
				recipient_identity_id,
				match_type,
				match_value,
				normalized_value,
				active,
				priority,
				source_kind,
				source_metadata,
				disabled_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9)
		`, created.ID, value.MatchType, value.MatchValue, value.NormalizedValue, value.Active, value.Priority, sourceKind, string(sourceMetadata), value.DisabledAt)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return created, nil
}

func (r *mailboxRepository) ListHeaders(ctx context.Context, filter service.MailHeaderListFilter) ([]*service.MailHeader, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, errors.New("mailbox repository db is nil")
	}

	whereClause, args := buildMailboxHeaderFilter(filter)

	var total int64
	countQuery := "SELECT COUNT(*) FROM mailbox_header_cache h WHERE " + whereClause
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultMailboxHeaderListLimit
	}
	selectArgs := append(append([]any(nil), args...), limit)
	query := `
		SELECT
			id,
			collector_id,
			capability_id,
			remote_message_id,
			folder,
			sender,
			recipients,
			subject,
			received_at,
			flags,
			snippet,
			envelope_recipients,
			delivered_to,
			original_to,
			resolved_recipient_identity_id,
			resolved_address,
			match_type,
			matched_value_id,
			resolution_source_field,
			resolution_state,
			detail_fetch_state,
			created_at,
			updated_at
		FROM mailbox_header_cache h
		WHERE ` + whereClause + `
		ORDER BY h.received_at DESC, h.id DESC
		LIMIT $` + fmt.Sprintf("%d", len(selectArgs))

	rows, err := r.db.QueryContext(ctx, query, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	headers := make([]*service.MailHeader, 0, limit)
	for rows.Next() {
		header, err := scanMailHeader(rows)
		if err != nil {
			return nil, 0, err
		}
		headers = append(headers, header)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return headers, total, nil
}

func (r *mailboxRepository) CreateSyncJobs(ctx context.Context, jobs []*service.MailSyncJob) ([]*service.MailSyncJob, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}
	if len(jobs) == 0 {
		return []*service.MailSyncJob{}, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	created := make([]*service.MailSyncJob, 0, len(jobs))
	for _, job := range jobs {
		if job == nil {
			continue
		}
		state := job.State
		if state == "" {
			state = service.MailSyncJobStateQueued
		}
		triggerSource := job.TriggerSource
		if triggerSource == "" {
			triggerSource = service.MailSyncTriggerSourceSchedule
		}
		scheduledFor := job.ScheduledFor
		if scheduledFor.IsZero() {
			scheduledFor = time.Now().UTC()
		}

		row := tx.QueryRowContext(ctx, `
			INSERT INTO mailbox_sync_jobs (
				capability_id,
				batch_id,
				state,
				trigger_source,
				scheduled_for,
				started_at,
				finished_at,
				retryable,
				retry_count,
				next_retry_at,
				error_summary
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING
				id,
				capability_id,
				batch_id,
				state,
				trigger_source,
				scheduled_for,
				started_at,
				finished_at,
				retryable,
				retry_count,
				next_retry_at,
				error_summary,
				created_at,
				updated_at
		`, job.CapabilityID, job.BatchID, state, triggerSource, scheduledFor, job.StartedAt, job.FinishedAt, job.Retryable, job.RetryCount, job.NextRetryAt, job.ErrorSummary)

		createdJob, err := scanMailSyncJob(row)
		if err != nil {
			return nil, err
		}
		created = append(created, createdJob)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return created, nil
}

func (r *mailboxRepository) ListActiveSyncJobs(ctx context.Context, capabilityID *int64, limit int) ([]*service.MailSyncJob, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}

	query := `
		SELECT
			id,
			capability_id,
			batch_id,
			state,
			trigger_source,
			scheduled_for,
			started_at,
			finished_at,
			retryable,
			retry_count,
			next_retry_at,
			error_summary,
			created_at,
			updated_at
		FROM mailbox_sync_jobs
		WHERE state IN ($1, $2)`
	args := []any{service.MailSyncJobStateQueued, service.MailSyncJobStateRunning}
	if capabilityID != nil {
		args = append(args, *capabilityID)
		query += fmt.Sprintf(" AND capability_id = $%d", len(args))
	}
	args = append(args, normalizeMailboxListLimit(limit))
	query += fmt.Sprintf(" ORDER BY scheduled_for ASC, id ASC LIMIT $%d", len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	jobs := make([]*service.MailSyncJob, 0)
	for rows.Next() {
		job, err := scanMailSyncJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *mailboxRepository) UpdateSyncJobState(ctx context.Context, jobID int64, state string, startedAt, finishedAt, nextRetryAt *time.Time, errorSummary *string) (*service.MailSyncJob, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}
	if strings.TrimSpace(state) == "" {
		return nil, errors.New("sync job state is required")
	}

	row := r.db.QueryRowContext(ctx, `
		UPDATE mailbox_sync_jobs
		SET
			state = $2,
			started_at = COALESCE($3, started_at),
			finished_at = $4,
			next_retry_at = $5,
			error_summary = $6,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			capability_id,
			batch_id,
			state,
			trigger_source,
			scheduled_for,
			started_at,
			finished_at,
			retryable,
			retry_count,
			next_retry_at,
			error_summary,
			created_at,
			updated_at
	`, jobID, state, startedAt, finishedAt, nextRetryAt, errorSummary)

	return scanMailSyncJob(row)
}

func (r *mailboxRepository) ClaimDueCapabilities(ctx context.Context, now time.Time, limit int) ([]*service.MailboxCapability, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mailbox repository db is nil")
	}
	if limit <= 0 {
		limit = defaultMailboxClaimLimit
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			c.id,
			c.provider_account_id,
			c.collector_id,
			c.capability_kind,
			c.connection_config,
			c.cursor_state,
			c.sync_enabled,
			c.sync_interval_seconds,
			c.next_sync_at,
			c.last_sync_at,
			c.health_state,
			c.last_error,
			c.created_at,
			c.updated_at,
			c.deleted_at
		FROM mailbox_capabilities c
		WHERE c.deleted_at IS NULL
			AND c.sync_enabled = TRUE
			AND c.next_sync_at IS NOT NULL
			AND c.next_sync_at <= $1
			AND NOT EXISTS (
				SELECT 1
				FROM mailbox_sync_jobs j
				WHERE j.capability_id = c.id
					AND j.state IN ($3, $4)
			)
		ORDER BY c.next_sync_at ASC, c.id ASC
		LIMIT $2
	`, now, limit, service.MailSyncJobStateQueued, service.MailSyncJobStateRunning)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	capabilities := make([]*service.MailboxCapability, 0, limit)
	for rows.Next() {
		capability, err := scanMailboxCapability(rows)
		if err != nil {
			return nil, err
		}
		capabilities = append(capabilities, capability)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return capabilities, nil
}

func buildMailboxHeaderFilter(filter service.MailHeaderListFilter) (string, []any) {
	conditions := []string{"1=1"}
	args := make([]any, 0, 3)
	if filter.CollectorID != nil {
		args = append(args, *filter.CollectorID)
		conditions = append(conditions, fmt.Sprintf("h.collector_id = $%d", len(args)))
	}
	if filter.CapabilityID != nil {
		args = append(args, *filter.CapabilityID)
		conditions = append(conditions, fmt.Sprintf("h.capability_id = $%d", len(args)))
	}
	folder := strings.TrimSpace(filter.Folder)
	if folder != "" {
		args = append(args, folder)
		conditions = append(conditions, fmt.Sprintf("h.folder = $%d", len(args)))
	}
	return strings.Join(conditions, " AND "), args
}

func scanProviderAccount(row sqlScanRow) (*service.ProviderAccount, error) {
	var account service.ProviderAccount
	var mailboxHint sql.NullString
	var providerIdentifier sql.NullString
	var lastImportedAt sql.NullTime
	var lastValidationAt sql.NullTime
	var lastValidationError sql.NullString
	var deletedAt sql.NullTime

	err := row.Scan(
		&account.ID,
		&account.DisplayName,
		&account.ProviderKind,
		&account.AuthKind,
		&account.Status,
		&account.EncryptedPayload,
		&mailboxHint,
		&providerIdentifier,
		&account.PayloadVersion,
		&lastImportedAt,
		&lastValidationAt,
		&lastValidationError,
		&account.CreatedAt,
		&account.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return nil, err
	}
	account.MailboxHint = nullableStringPtr(mailboxHint)
	account.ProviderIdentifier = nullableStringPtr(providerIdentifier)
	account.LastImportedAt = nullableTimePtr(lastImportedAt)
	account.LastValidationAt = nullableTimePtr(lastValidationAt)
	account.LastValidationError = nullableStringPtr(lastValidationError)
	account.DeletedAt = nullableTimePtr(deletedAt)
	return &account, nil
}

func scanCollector(row sqlScanRow) (*service.CollectorMailbox, error) {
	var collector service.CollectorMailbox
	var businessTagsRaw []byte
	var deletedAt sql.NullTime

	err := row.Scan(
		&collector.ID,
		&collector.EmailAddress,
		&collector.DisplayName,
		&collector.Enabled,
		&businessTagsRaw,
		&collector.CreatedAt,
		&collector.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return nil, err
	}
	businessTags, err := decodeJSONStringSlice(businessTagsRaw)
	if err != nil {
		return nil, err
	}
	collector.BusinessTags = businessTags
	collector.DeletedAt = nullableTimePtr(deletedAt)
	return &collector, nil
}

func scanMailboxCapability(row sqlScanRow) (*service.MailboxCapability, error) {
	var capability service.MailboxCapability
	var connectionConfigRaw []byte
	var cursorStateRaw []byte
	var nextSyncAt sql.NullTime
	var lastSyncAt sql.NullTime
	var lastError sql.NullString
	var deletedAt sql.NullTime

	err := row.Scan(
		&capability.ID,
		&capability.ProviderAccountID,
		&capability.CollectorID,
		&capability.CapabilityKind,
		&connectionConfigRaw,
		&cursorStateRaw,
		&capability.SyncEnabled,
		&capability.SyncIntervalSeconds,
		&nextSyncAt,
		&lastSyncAt,
		&capability.HealthState,
		&lastError,
		&capability.CreatedAt,
		&capability.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return nil, err
	}
	connectionConfig, err := decodeMailboxConnectionConfig(connectionConfigRaw)
	if err != nil {
		return nil, err
	}
	cursorState, err := decodeMailboxCursorState(cursorStateRaw)
	if err != nil {
		return nil, err
	}
	capability.ConnectionConfig = connectionConfig
	capability.CursorState = cursorState
	capability.NextSyncAt = nullableTimePtr(nextSyncAt)
	capability.LastSyncAt = nullableTimePtr(lastSyncAt)
	capability.LastError = nullableStringPtr(lastError)
	capability.DeletedAt = nullableTimePtr(deletedAt)
	return &capability, nil
}

func scanRecipientIdentity(row sqlScanRow) (*service.RecipientIdentity, error) {
	var identity service.RecipientIdentity
	var deletedAt sql.NullTime
	err := row.Scan(
		&identity.ID,
		&identity.Name,
		&identity.NormalizedName,
		&identity.Enabled,
		&identity.CreatedAt,
		&identity.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return nil, err
	}
	identity.DeletedAt = nullableTimePtr(deletedAt)
	return &identity, nil
}

func scanMailHeader(row sqlScanRow) (*service.MailHeader, error) {
	var header service.MailHeader
	var sender sql.NullString
	var recipientsRaw []byte
	var flagsRaw []byte
	var envelopeRecipientsRaw []byte
	var deliveredToRaw []byte
	var originalToRaw []byte
	var resolvedRecipientIdentityID sql.NullInt64
	var resolvedAddress sql.NullString
	var matchType sql.NullString
	var matchedValueID sql.NullInt64
	var resolutionSourceField sql.NullString

	err := row.Scan(
		&header.ID,
		&header.CollectorID,
		&header.CapabilityID,
		&header.RemoteMessageID,
		&header.Folder,
		&sender,
		&recipientsRaw,
		&header.Subject,
		&header.ReceivedAt,
		&flagsRaw,
		&header.Snippet,
		&envelopeRecipientsRaw,
		&deliveredToRaw,
		&originalToRaw,
		&resolvedRecipientIdentityID,
		&resolvedAddress,
		&matchType,
		&matchedValueID,
		&resolutionSourceField,
		&header.ResolutionState,
		&header.DetailFetchState,
		&header.CreatedAt,
		&header.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	header.Sender = nullableStringPtr(sender)
	header.Recipients, err = decodeJSONStringSlice(recipientsRaw)
	if err != nil {
		return nil, err
	}
	header.Flags, err = decodeJSONStringSlice(flagsRaw)
	if err != nil {
		return nil, err
	}
	header.EnvelopeRecipients, err = decodeJSONStringSlice(envelopeRecipientsRaw)
	if err != nil {
		return nil, err
	}
	header.DeliveredTo, err = decodeJSONStringSlice(deliveredToRaw)
	if err != nil {
		return nil, err
	}
	header.OriginalTo, err = decodeJSONStringSlice(originalToRaw)
	if err != nil {
		return nil, err
	}
	header.ResolvedRecipientIdentityID = nullableInt64Ptr(resolvedRecipientIdentityID)
	header.ResolvedAddress = nullableStringPtr(resolvedAddress)
	header.MatchType = nullableStringPtr(matchType)
	header.MatchedValueID = nullableInt64Ptr(matchedValueID)
	header.ResolutionSourceField = nullableStringPtr(resolutionSourceField)
	return &header, nil
}

func scanMailSyncJob(row sqlScanRow) (*service.MailSyncJob, error) {
	var job service.MailSyncJob
	var batchID sql.NullString
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	var nextRetryAt sql.NullTime
	var errorSummary sql.NullString

	err := row.Scan(
		&job.ID,
		&job.CapabilityID,
		&batchID,
		&job.State,
		&job.TriggerSource,
		&job.ScheduledFor,
		&startedAt,
		&finishedAt,
		&job.Retryable,
		&job.RetryCount,
		&nextRetryAt,
		&errorSummary,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	job.BatchID = nullableStringPtr(batchID)
	job.StartedAt = nullableTimePtr(startedAt)
	job.FinishedAt = nullableTimePtr(finishedAt)
	job.NextRetryAt = nullableTimePtr(nextRetryAt)
	job.ErrorSummary = nullableStringPtr(errorSummary)
	return &job, nil
}

func marshalJSONB(v any, fallback []byte) ([]byte, error) {
	if v == nil {
		return fallback, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return fallback, nil
	}
	return data, nil
}

func decodeJSONStringSlice(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return []string{}, nil
	}
	return out, nil
}

func decodeMailboxConnectionConfig(raw []byte) (service.MailboxConnectionConfig, error) {
	if len(raw) == 0 {
		return service.MailboxConnectionConfig{}, nil
	}
	var out service.MailboxConnectionConfig
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return service.MailboxConnectionConfig{}, nil
	}
	return out, nil
}

func decodeMailboxCursorState(raw []byte) (service.MailboxCursorState, error) {
	if len(raw) == 0 {
		return service.MailboxCursorState{}, nil
	}
	var out service.MailboxCursorState
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return service.MailboxCursorState{}, nil
	}
	return out, nil
}

func normalizeMailboxListLimit(limit int) int {
	if limit <= 0 {
		return defaultMailboxHeaderListLimit
	}
	return limit
}

func ensureRowsAffected(res sql.Result) error {
	if res == nil {
		return sql.ErrNoRows
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func nullableStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	value := v.String
	return &value
}

func nullableTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	value := v.Time.UTC()
	return &value
}

func nullableInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	value := v.Int64
	return &value
}
