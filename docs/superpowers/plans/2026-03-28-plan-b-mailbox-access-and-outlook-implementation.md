# Plan B Mailbox Access and Outlook Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an admin-only mailbox domain for provider accounts, collector mailboxes, recipient resolution, scheduled header sync, batch manual sync, and read-only inbox browsing with on-demand body fetch.

**Architecture:** Implement the mailbox domain as explicit PostgreSQL tables plus raw-SQL repositories, following the existing `scheduled_test` and `ops` patterns for stateful operational data. Keep protocol-specific logic behind mailbox provider interfaces, expose one dedicated admin REST surface under `/api/v1/admin/mailbox`, and ship four independent admin pages under a new `Mailbox` menu group. The first version synchronizes only normalized Inbox headers, resolves them to recipient identities, and fetches bodies lazily without creating a permanent full-mailbox mirror.

**Tech Stack:** Go, Gin, PostgreSQL, Wire, Vue 3, Vue Router, Pinia, Vitest, pnpm, Markdown

---

## Fixed Product Decisions

- Scope: B1 + B2 only; B3 remains a downstream consumer.
- Access: admin-only.
- UI entry: four admin pages, not `Settings`, not the existing `Accounts` module.
- Outlook import: plain-text bundle string with four `----`-separated segments.
- Inbox policy: default Inbox only for IMAP and Microsoft Graph; POP3 normalized to pseudo-folder `INBOX`.
- Initial backfill: the more conservative of the most recent 30 days or 500 messages.
- Sync modes: scheduled sync, single manual sync, batch manual sync.
- Recipient resolution priority: envelope recipient -> `Delivered-To` -> `X-Original-To` -> `To` -> `Cc`.
- Exact addresses must be globally unique while active.
- Suffix conflicts use longest-suffix-first, then explicit priority, then `ambiguous`.
- SMTP v1 scope: configuration storage plus outbound connectivity validation only.
- Provider, collector, and recipient admin flows support create, edit, and enable/disable in v1; hard delete is deferred.

## File Map

- Create: `backend/migrations/081_add_mailbox_domain_tables.sql` - create mailbox domain tables, indexes, and foreign keys.
- Modify: `backend/internal/repository/migrations_schema_integration_test.go` - assert the new mailbox schema and indexes exist after migrations.
- Create: `backend/internal/service/mailbox_port.go` - define mailbox domain models, repository interfaces, provider interfaces, and status constants.
- Create: `backend/internal/repository/mailbox_repo.go` - implement raw-SQL repositories for provider accounts, collector mailboxes, capabilities, recipient identities, match values, header cache, and sync jobs.
- Create: `backend/internal/repository/mailbox_repo_test.go` - integration tests for mailbox repository CRUD and query behavior.
- Modify: `backend/internal/repository/wire.go` - register the mailbox repository constructor.
- Create: `backend/internal/pkg/mailbox/provider.go` - define protocol adapter request and response contracts.
- Create: `backend/internal/pkg/mailbox/basic_client.go` - implement SMTP, IMAP, and POP3 validation plus header/body fetch helpers.
- Create: `backend/internal/pkg/mailbox/microsoft_client.go` - implement Outlook import parsing and Microsoft mailbox validation plus read helpers.
- Create: `backend/internal/pkg/mailbox/basic_client_test.go` - protocol adapter tests for IMAP incremental requests and POP3 dedupe rules.
- Create: `backend/internal/pkg/mailbox/microsoft_client_test.go` - protocol adapter tests for Outlook bundle validation and mailbox-read probes.
- Create: `backend/internal/service/mailbox_service.go` - CRUD, validation orchestration, list queries, and audit logging for provider accounts, collectors, capabilities, and recipient identities.
- Create: `backend/internal/service/mailbox_audit.go` - centralize structured audit logging for mailbox admin actions.
- Create: `backend/internal/service/mailbox_resolution_service.go` - resolve synchronized headers to recipient identities.
- Create: `backend/internal/service/mailbox_sync_service.go` - create sync jobs, execute bounded initial backfill, incremental sync, detail fetch, and batch sync splitting.
- Create: `backend/internal/service/mailbox_sync_runner_service.go` - periodically scan enabled inbound capabilities and run scheduled sync.
- Create: `backend/internal/service/mailbox_service_test.go` - unit tests for Outlook import parsing, exact-address uniqueness checks, and service validation rules.
- Create: `backend/internal/service/mailbox_resolution_service_test.go` - unit tests for source priority, suffix matching, and ambiguity handling.
- Create: `backend/internal/service/mailbox_sync_service_test.go` - unit tests for bounded backfill, batch sync fan-out, and detail-fetch state transitions.
- Modify: `backend/internal/service/wire.go` - register mailbox services and runner startup.
- Create: `backend/internal/handler/dto/mailbox.go` - map mailbox service models into API responses.
- Create: `backend/internal/handler/admin/mailbox_handler.go` - implement admin HTTP endpoints for providers, collectors, recipients, inbox, validation, and sync actions.
- Create: `backend/internal/handler/admin/mailbox_handler_test.go` - handler tests for request validation, list responses, and batch sync actions.
- Create: `backend/internal/integration/mailbox_integration_test.go` - backend integration tests for provider validation, capability testing, sync persistence, and on-demand detail fetch.
- Modify: `backend/internal/handler/handler.go` - add the new admin mailbox handler to the aggregated handler graph.
- Modify: `backend/internal/handler/wire.go` - register the new mailbox handler constructor.
- Modify: `backend/internal/server/routes/admin.go` - register `/api/v1/admin/mailbox/**` routes.
- Create: `frontend/src/types/mailbox.ts` - hold mailbox-specific frontend types.
- Modify: `frontend/src/types/index.ts` - export mailbox types without bloating the existing import surface.
- Create: `frontend/src/api/admin/mailbox.ts` - admin mailbox API wrapper.
- Create: `frontend/src/api/__tests__/mailbox.spec.ts` - API wrapper tests.
- Modify: `frontend/src/api/admin/index.ts` - export mailbox admin API helpers.
- Create: `frontend/src/views/admin/MailboxProvidersView.vue` - provider account page.
- Create: `frontend/src/views/admin/CollectorMailboxesView.vue` - collector mailbox and capability page.
- Create: `frontend/src/views/admin/RecipientIdentitiesView.vue` - recipient identity and match-value page.
- Create: `frontend/src/views/admin/MailInboxView.vue` - header-cache inbox page.
- Create: `frontend/src/components/admin/mailbox/ProviderAccountDialog.vue` - create/edit provider account dialog.
- Create: `frontend/src/components/admin/mailbox/CollectorMailboxDialog.vue` - create/edit collector mailbox and capability dialog.
- Create: `frontend/src/components/admin/mailbox/RecipientIdentityDialog.vue` - create/edit recipient identity and match values dialog.
- Create: `frontend/src/components/admin/mailbox/BatchSyncToolbar.vue` - collector-page selected-row sync controls and batch progress summary.
- Create: `frontend/src/components/admin/mailbox/MailInboxDetailDrawer.vue` - on-demand body and attachment-metadata drawer.
- Create: `frontend/src/views/admin/__tests__/MailboxProvidersView.spec.ts` - provider page tests.
- Create: `frontend/src/views/admin/__tests__/CollectorMailboxesView.spec.ts` - collector page tests.
- Create: `frontend/src/views/admin/__tests__/RecipientIdentitiesView.spec.ts` - recipient page tests.
- Create: `frontend/src/views/admin/__tests__/MailInboxView.spec.ts` - inbox page tests.
- Modify: `frontend/src/router/index.ts` - add the four admin routes.
- Modify: `frontend/src/router/__tests__/guards.spec.ts` - ensure the new routes remain admin-gated.
- Modify: `frontend/src/components/layout/AppSidebar.vue` - add the new `Mailbox` admin menu entries.
- Modify: `frontend/src/i18n/locales/en.ts` - add mailbox i18n strings.
- Modify: `frontend/src/i18n/locales/zh.ts` - add mailbox i18n strings.

### Route and Endpoint Contract

- Admin route: `/admin/mailbox/providers`
- Admin route: `/admin/mailbox/collectors`
- Admin route: `/admin/mailbox/recipients`
- Admin route: `/admin/mailbox/inbox`
- Admin API root: `/api/v1/admin/mailbox`
- Provider endpoints: `/providers`, `/providers/:id`, `/providers/:id/validate`, `/providers/:id/status`
- Collector endpoints: `/collectors`, `/collectors/:id`, `/collectors/:id/sync`, `/collectors/batch-sync`, `/collectors/:id/status`, `/capabilities/:id/test`, `/capabilities/:id/status`
- Recipient endpoints: `/recipients`, `/recipients/:id`, `/recipients/:id/import-exact-addresses`, `/recipients/:id/status`
- Inbox endpoints: `/inbox`, `/inbox/:id`, `/inbox/:id/detail`
- Sync job endpoints: `/sync-jobs/batches/:batch_id`

### Database Table Contract

- `mailbox_provider_accounts`
- `mailbox_collectors`
- `mailbox_capabilities`
- `mailbox_recipient_identities`
- `mailbox_recipient_match_values`
- `mailbox_header_cache`
- `mailbox_sync_jobs`

### Implementation Notes

- Prefer raw-SQL repositories for this slice; do not introduce Ent schema or generated Ent artifacts for the mailbox domain in v1.
- Store provider secrets encrypted at rest and never log raw bundles, passwords, access tokens, refresh tokens, or message bodies.
- Keep audit logging as structured service-level logs with mailbox-specific `audit.*` components instead of creating a dedicated audit table in v1.
- Do not require real mailbox credentials until the final integration verification step.

### Task 1: Create the Mailbox Schema and Domain Ports

**Files:**
- Create: `backend/migrations/081_add_mailbox_domain_tables.sql`
- Modify: `backend/internal/repository/migrations_schema_integration_test.go`
- Create: `backend/internal/service/mailbox_port.go`

- [ ] **Step 1: Write the failing integration schema test**

```go
func TestMigrationsRunner_CreatesMailboxDomainTables(t *testing.T) {
	tx := testTx(t)
	require.NoError(t, ApplyMigrations(context.Background(), integrationDB))

	var regclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.mailbox_provider_accounts')").Scan(&regclass))
	require.True(t, regclass.Valid)
	requireColumn(t, tx, "mailbox_provider_accounts", "provider_kind", "character varying", 32, false)
	requireColumn(t, tx, "mailbox_provider_accounts", "encrypted_payload", "text", 0, false)
	requireColumn(t, tx, "mailbox_provider_accounts", "last_imported_at", "timestamp with time zone", 0, true)

	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.mailbox_collectors')").Scan(&regclass))
	require.True(t, regclass.Valid)
	requireColumn(t, tx, "mailbox_collectors", "email_address", "character varying", 320, false)

	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.mailbox_capabilities')").Scan(&regclass))
	require.True(t, regclass.Valid)
	requireIndex(t, tx, "mailbox_capabilities", "idx_mailbox_capabilities_sync_due")

	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.mailbox_recipient_match_values')").Scan(&regclass))
	require.True(t, regclass.Valid)
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.mailbox_recipient_identities')").Scan(&regclass))
	require.True(t, regclass.Valid)
	requireColumn(t, tx, "mailbox_recipient_identities", "name", "character varying", 120, false)
	requireColumn(t, tx, "mailbox_recipient_match_values", "recipient_identity_id", "bigint", 0, false)
	requireIndex(t, tx, "mailbox_recipient_match_values", "uq_mailbox_recipient_exact_active")

	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.mailbox_header_cache')").Scan(&regclass))
	require.True(t, regclass.Valid)
	requireColumn(t, tx, "mailbox_header_cache", "received_at", "timestamp with time zone", 0, false)
	requireColumn(t, tx, "mailbox_header_cache", "snippet", "text", 0, true)
	requireColumn(t, tx, "mailbox_header_cache", "resolved_address", "character varying", 320, true)
	requireColumnExists(t, tx, "mailbox_header_cache", "envelope_recipients")
	requireColumnExists(t, tx, "mailbox_header_cache", "delivered_to")
	requireColumnExists(t, tx, "mailbox_header_cache", "original_to")
	requireColumn(t, tx, "mailbox_header_cache", "match_type", "character varying", 20, true)
	requireColumn(t, tx, "mailbox_header_cache", "matched_value_id", "bigint", 0, true)
	requireColumn(t, tx, "mailbox_header_cache", "resolution_source_field", "character varying", 40, true)
	requireColumn(t, tx, "mailbox_header_cache", "resolution_state", "character varying", 20, false)
	requireIndex(t, tx, "mailbox_header_cache", "uq_mailbox_header_capability_folder_remote")

	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.mailbox_sync_jobs')").Scan(&regclass))
	require.True(t, regclass.Valid)
	requireColumn(t, tx, "mailbox_sync_jobs", "batch_id", "character varying", 64, true)
	requireColumn(t, tx, "mailbox_sync_jobs", "retryable", "boolean", 0, false)
	requireColumn(t, tx, "mailbox_sync_jobs", "next_retry_at", "timestamp with time zone", 0, true)
}
```

If `requireColumn` is too strict for JSONB-backed evidence fields, add a tiny `requireColumnExists` helper in the same test file and use it for the recipient-evidence columns.

- [ ] **Step 2: Run the schema test to verify it fails**

Run: `go test -tags=integration ./internal/repository -run TestMigrationsRunner_CreatesMailboxDomainTables` from `backend/`
Expected: FAIL because the migration and table definitions do not exist yet.

- [ ] **Step 3: Write the migration and domain port file**

Use one migration file with explicit tables and indexes. The core shape should look like this:

```sql
CREATE TABLE mailbox_provider_accounts (
  id BIGSERIAL PRIMARY KEY,
  display_name VARCHAR(100) NOT NULL,
  provider_kind VARCHAR(32) NOT NULL,
  auth_kind VARCHAR(32) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'draft',
  encrypted_payload TEXT NOT NULL,
  mailbox_hint VARCHAR(320),
  provider_identifier VARCHAR(255),
  last_imported_at TIMESTAMPTZ,
  last_validation_at TIMESTAMPTZ,
  last_validation_error TEXT,
  disabled_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE mailbox_collectors (
  id BIGSERIAL PRIMARY KEY,
  email_address VARCHAR(320) NOT NULL,
  display_name VARCHAR(120) NOT NULL,
  business_tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE mailbox_capabilities (
  id BIGSERIAL PRIMARY KEY,
  collector_id BIGINT NOT NULL REFERENCES mailbox_collectors(id) ON DELETE CASCADE,
  provider_account_id BIGINT NOT NULL REFERENCES mailbox_provider_accounts(id),
  capability_kind VARCHAR(40) NOT NULL,
  connection_config JSONB NOT NULL DEFAULT '{}'::jsonb,
  cursor_state JSONB NOT NULL DEFAULT '{}'::jsonb,
  health_state VARCHAR(20) NOT NULL DEFAULT 'healthy',
  sync_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  sync_interval_seconds INTEGER NOT NULL DEFAULT 300,
  next_sync_at TIMESTAMPTZ,
  last_sync_at TIMESTAMPTZ,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE mailbox_recipient_identities (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(120) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE mailbox_recipient_match_values (
  id BIGSERIAL PRIMARY KEY,
  recipient_identity_id BIGINT NOT NULL REFERENCES mailbox_recipient_identities(id) ON DELETE CASCADE,
  match_type VARCHAR(20) NOT NULL,
  match_value VARCHAR(320) NOT NULL,
  priority INTEGER NOT NULL DEFAULT 100,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  source_type VARCHAR(32) NOT NULL DEFAULT 'manual',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_mailbox_recipient_exact_active
ON mailbox_recipient_match_values (lower(match_value))
WHERE match_type = 'exact_address' AND active = TRUE;

CREATE TABLE mailbox_header_cache (
  id BIGSERIAL PRIMARY KEY,
  capability_id BIGINT NOT NULL REFERENCES mailbox_capabilities(id) ON DELETE CASCADE,
  collector_id BIGINT NOT NULL REFERENCES mailbox_collectors(id) ON DELETE CASCADE,
  remote_message_id VARCHAR(255) NOT NULL,
  folder VARCHAR(64) NOT NULL DEFAULT 'INBOX',
  sender JSONB NOT NULL DEFAULT '{}'::jsonb,
  recipients JSONB NOT NULL DEFAULT '{}'::jsonb,
  subject TEXT NOT NULL DEFAULT '',
  received_at TIMESTAMPTZ NOT NULL,
  flags JSONB NOT NULL DEFAULT '[]'::jsonb,
  snippet TEXT,
  envelope_recipients JSONB NOT NULL DEFAULT '[]'::jsonb,
  delivered_to JSONB NOT NULL DEFAULT '[]'::jsonb,
  original_to JSONB NOT NULL DEFAULT '[]'::jsonb,
  resolved_recipient_identity_id BIGINT REFERENCES mailbox_recipient_identities(id),
  resolved_address VARCHAR(320),
  match_type VARCHAR(20),
  matched_value_id BIGINT REFERENCES mailbox_recipient_match_values(id),
  resolution_source_field VARCHAR(40),
  resolution_state VARCHAR(20) NOT NULL DEFAULT 'unresolved',
  detail_fetch_state VARCHAR(20) NOT NULL DEFAULT 'not_requested',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_mailbox_header_capability_folder_remote
ON mailbox_header_cache (capability_id, folder, remote_message_id);

CREATE TABLE mailbox_sync_jobs (
  id BIGSERIAL PRIMARY KEY,
  capability_id BIGINT NOT NULL REFERENCES mailbox_capabilities(id) ON DELETE CASCADE,
  batch_id VARCHAR(64),
  trigger_source VARCHAR(32) NOT NULL,
  state VARCHAR(20) NOT NULL DEFAULT 'queued',
  retryable BOOLEAN NOT NULL DEFAULT FALSE,
  retry_count INTEGER NOT NULL DEFAULT 0,
  next_retry_at TIMESTAMPTZ,
  error_summary TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Create the tables in this order inside the migration: provider accounts -> collectors -> capabilities -> recipient identities -> recipient match values -> header cache -> sync jobs.

In `backend/internal/service/mailbox_port.go`, define typed models and repository interfaces, for example:

```go
type ProviderAccount struct {
	ID                  int64
	DisplayName         string
	ProviderKind        string
	AuthKind            string
	Status              string
	EncryptedPayload    string
	MailboxHint         string
	ProviderIdentifier  string
	LastImportedAt      *time.Time
	LastValidationAt    *time.Time
	LastValidationError string
	DisabledAt          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type MailboxRepository interface {
	CreateProviderAccount(ctx context.Context, in *ProviderAccount) (*ProviderAccount, error)
	CreateCollector(ctx context.Context, in *CollectorMailbox) (*CollectorMailbox, error)
	CreateCapability(ctx context.Context, in *MailboxCapability) (*MailboxCapability, error)
	CreateRecipientIdentity(ctx context.Context, in *RecipientIdentity, values []*RecipientMatchValue) (*RecipientIdentity, error)
	ListHeaders(ctx context.Context, filter MailHeaderListFilter) ([]*MailHeader, int64, error)
	CreateSyncJobs(ctx context.Context, jobs []*MailSyncJob) ([]*MailSyncJob, error)
	ClaimDueCapabilities(ctx context.Context, now time.Time, limit int) ([]*MailboxCapability, error)
}
```

- [ ] **Step 4: Re-run the schema test**

Run: `go test -tags=integration ./internal/repository -run TestMigrationsRunner_CreatesMailboxDomainTables` from `backend/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/migrations/081_add_mailbox_domain_tables.sql backend/internal/repository/migrations_schema_integration_test.go backend/internal/service/mailbox_port.go
git commit -m "feat: add mailbox domain schema"
```

### Task 2: Implement the Raw-SQL Mailbox Repository Layer

**Files:**
- Create: `backend/internal/repository/mailbox_repo.go`
- Create: `backend/internal/repository/mailbox_repo_test.go`
- Modify: `backend/internal/repository/wire.go`

- [ ] **Step 1: Write failing repository tests**

```go
func TestMailboxRepository_CreateProviderCollectorAndCapability(t *testing.T) {
	repo := NewMailboxRepository(testDB(t))
	ctx := context.Background()

	provider, err := repo.CreateProviderAccount(ctx, &service.ProviderAccount{
		DisplayName:      "Outlook Seed",
		ProviderKind:     "microsoft",
		AuthKind:         "import_bundle",
		EncryptedPayload: "enc:test",
		Status:           "draft",
	})
	require.NoError(t, err)

	collector, err := repo.CreateCollector(ctx, &service.CollectorMailbox{
		EmailAddress: "collector@example.com",
		DisplayName:  "Collector",
		Enabled:      true,
	})
	require.NoError(t, err)

	capability, err := repo.CreateCapability(ctx, &service.MailboxCapability{
		CollectorID:        collector.ID,
		ProviderAccountID:  provider.ID,
		CapabilityKind:     "inbound.microsoft_graph",
		SyncEnabled:        true,
		SyncIntervalSeconds: 300,
	})
	require.NoError(t, err)
	require.Equal(t, provider.ID, capability.ProviderAccountID)
}

func TestMailboxRepository_RejectsDuplicateActiveExactAddress(t *testing.T) {
	repo := NewMailboxRepository(testDB(t))
	ctx := context.Background()

	_, err := repo.CreateRecipientIdentity(ctx,
		&service.RecipientIdentity{Name: "Identity A", Status: "active"},
		[]*service.RecipientMatchValue{{MatchType: "exact_address", MatchValue: "relay@example.com", Priority: 100, Active: true}},
	)
	require.NoError(t, err)

	_, err = repo.CreateRecipientIdentity(ctx,
		&service.RecipientIdentity{Name: "Identity B", Status: "active"},
		[]*service.RecipientMatchValue{{MatchType: "exact_address", MatchValue: "relay@example.com", Priority: 100, Active: true}},
	)
	require.Error(t, err)
}
```

- [ ] **Step 2: Run the repository tests to verify they fail**

Run: `go test -tags=integration ./internal/repository -run 'TestMailboxRepository_'` from `backend/`
Expected: FAIL because `NewMailboxRepository` and the repository methods do not exist yet.

- [ ] **Step 3: Implement the repository and Wire registration**

Create one repository implementation with transaction-safe helpers for related writes. The constructor and one method should look like this:

```go
type mailboxRepository struct {
	db *sql.DB
}

func NewMailboxRepository(db *sql.DB) service.MailboxRepository {
	return &mailboxRepository{db: db}
}

func (r *mailboxRepository) CreateProviderAccount(ctx context.Context, in *service.ProviderAccount) (*service.ProviderAccount, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO mailbox_provider_accounts
		(display_name, provider_kind, auth_kind, status, encrypted_payload, mailbox_hint, provider_identifier, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), NOW(), NOW())
		RETURNING id, display_name, provider_kind, auth_kind, status, encrypted_payload, COALESCE(mailbox_hint, ''), COALESCE(provider_identifier, ''), last_validation_at, COALESCE(last_validation_error, ''), disabled_at, created_at, updated_at
	`, in.DisplayName, in.ProviderKind, in.AuthKind, in.Status, in.EncryptedPayload, in.MailboxHint, in.ProviderIdentifier)
	return scanProviderAccount(row)
}
```

Also add SQL helpers for:

- provider account CRUD and paginated list
- collector mailbox CRUD and paginated list
- capability CRUD, due-sync lookup, and optimistic claim/update
- recipient identity CRUD plus transactional `match_values` replacement
- inbox header list and detail hydration
- sync job insert, list by batch id, and job-state updates

Then register the constructor in `backend/internal/repository/wire.go`:

```go
var ProviderSet = wire.NewSet(
	// ...existing providers...
	NewMailboxRepository,
)
```

- [ ] **Step 4: Re-run the repository tests**

Run: `go test -tags=integration ./internal/repository -run 'TestMailboxRepository_'` from `backend/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/repository/mailbox_repo.go backend/internal/repository/mailbox_repo_test.go backend/internal/repository/wire.go
git commit -m "feat: add mailbox repository layer"
```

### Task 3: Implement Provider Validation, CRUD Rules, and Recipient Resolution

**Files:**
- Create: `backend/internal/pkg/mailbox/provider.go`
- Create: `backend/internal/pkg/mailbox/basic_client.go`
- Create: `backend/internal/pkg/mailbox/microsoft_client.go`
- Create: `backend/internal/pkg/mailbox/basic_client_test.go`
- Create: `backend/internal/pkg/mailbox/microsoft_client_test.go`
- Create: `backend/internal/service/mailbox_service.go`
- Create: `backend/internal/service/mailbox_audit.go`
- Create: `backend/internal/service/mailbox_resolution_service.go`
- Create: `backend/internal/service/mailbox_service_test.go`
- Create: `backend/internal/service/mailbox_resolution_service_test.go`
- Modify: `backend/internal/service/wire.go`

- [ ] **Step 1: Write failing service tests**

```go
func TestMailboxService_ParseOutlookImportBundle(t *testing.T) {
	svc := NewMailboxService(fakeMailboxRepo{}, fakeProviderRegistry{})
	bundle := "user@example.com----secret----client-id----opaque-token-bundle"
	parsed, err := svc.ParseOutlookImportBundle(bundle)
	require.NoError(t, err)
	require.Equal(t, "user@example.com", parsed.MailboxIdentifier)
	require.Equal(t, "client-id", parsed.ProviderIdentifier)
}

func TestMailboxService_ParseOutlookImportBundleRejectsBadFormat(t *testing.T) {
	svc := NewMailboxService(fakeMailboxRepo{}, fakeProviderRegistry{})
	_, err := svc.ParseOutlookImportBundle("bad-format")
	require.ErrorIs(t, err, ErrMailboxImportFormat)
}

func TestMailboxService_ValidateProviderMapsMicrosoftFailuresToSpecCodes(t *testing.T) {
	repo := &fakeMailboxRepo{}
	registry := fakeProviderRegistry{validateErr: mailbox.ErrExpiredOrRevoked}
	svc := NewMailboxService(repo, registry)
	_, err := svc.ValidateProvider(context.Background(), 21)
	require.ErrorIs(t, err, ErrMailboxExpiredOrRevoked)
	require.Equal(t, service.MailboxProviderStatusInvalid, repo.updatedProvider.Status)
	require.Equal(t, "expired_or_revoked", repo.updatedProvider.LastValidationError)
}

func TestMailboxService_TestCapabilityUpdatesHealthState(t *testing.T) {
	repo := &fakeMailboxRepo{}
	registry := fakeProviderRegistry{validateResult: &mailbox.ValidationResult{LatencyMs: 42}}
	svc := NewMailboxService(repo, registry)

	result, err := svc.TestCapability(context.Background(), 21)
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, service.MailboxCapabilityHealthHealthy, repo.updatedCapability.HealthState)
	require.Equal(t, int64(42), result.LatencyMs)
}

func TestMailboxService_CreateProviderEmitsAuditLog(t *testing.T) {
	auditSink := &fakeMailboxAuditSink{}
	svc := NewMailboxService(fakeMailboxRepo{}, fakeProviderRegistry{}, auditSink)
	_, _ = svc.CreateProvider(context.Background(), service.CreateProviderAccountInput{
		DisplayName:   "Outlook Seed",
		ProviderKind:  "microsoft",
		AuthKind:      "import_bundle",
		ImportPayload: "user@example.com----secret----client----opaque",
		OperatorID:    7,
	})
	require.Equal(t, "provider.create", auditSink.lastAction)
}

func TestBasicClient_IMAPInitialListHeadersUsesInboxAndBoundedSince(t *testing.T) {
	client := NewBasicClient(fakeIMAPDialer{})
	page, err := client.ListHeaders(context.Background(), mailbox.ProviderPayload{CapabilityKind: "inbound.imap"}, mailbox.ListHeadersRequest{
		Folder: "INBOX",
		Since:  time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		Limit:  500,
	})
	require.NoError(t, err)
	require.NotNil(t, page)
}

func TestBasicClient_POP3DedupesByMessageIDAndReceivedAt(t *testing.T) {
	client := NewBasicClient(fakePOP3DialerWithDuplicates())
	page, err := client.ListHeaders(context.Background(), mailbox.ProviderPayload{CapabilityKind: "inbound.pop3"}, mailbox.ListHeadersRequest{Folder: "INBOX"})
	require.NoError(t, err)
	require.Len(t, page.Messages, 1)
}

func TestMicrosoftClient_ValidateOutlookBundleAndReadInbox(t *testing.T) {
	client := NewMicrosoftClient(fakeMicrosoftGateway{})
	result, err := client.Validate(context.Background(), mailbox.ProviderPayload{RawBundle: "user@example.com----secret----client----opaque"})
	require.NoError(t, err)
	require.Equal(t, "user@example.com", result.MailboxHint)
}

func TestResolutionService_PrefersExactAliasBeforeSuffix(t *testing.T) {
	resolver := NewMailboxResolutionService()
	result := resolver.Resolve([]string{"hidden@privaterelay.appleid.com"}, []service.RecipientMatchValue{
		{RecipientIdentityID: 1, MatchType: "domain_suffix", MatchValue: "appleid.com", Priority: 100, Active: true},
		{RecipientIdentityID: 2, MatchType: "exact_address", MatchValue: "hidden@privaterelay.appleid.com", Priority: 100, Active: true},
	})
	require.Equal(t, int64(2), result.RecipientIdentityID)
	require.Equal(t, "exact_address", result.MatchType)
}

func TestResolutionService_ReturnsAmbiguousWhenTopFieldResolvesToDifferentIdentities(t *testing.T) {
	resolver := NewMailboxResolutionService()
	result := resolver.ResolveSourceCandidates(service.MailResolutionInput{
		EnvelopeRecipients: []string{"a@example.com", "b@example.com"},
	}, []service.RecipientMatchValue{
		{RecipientIdentityID: 1, MatchType: "exact_address", MatchValue: "a@example.com", Priority: 100, Active: true},
		{RecipientIdentityID: 2, MatchType: "exact_address", MatchValue: "b@example.com", Priority: 100, Active: true},
	})
	require.Equal(t, service.MailResolutionStateAmbiguous, result.State)
}
```

- [ ] **Step 2: Run the service tests to verify they fail**

Run: `go test ./internal/service ./internal/pkg/mailbox -run 'TestMailboxService_|TestResolutionService_|TestBasicClient_|TestMicrosoftClient_'` from `backend/`
Expected: FAIL because the mailbox services and provider contracts do not exist yet.

- [ ] **Step 3: Implement provider adapters, validation, encryption entry points, and resolution logic**

Define adapter contracts in `backend/internal/pkg/mailbox/provider.go`:

```go
type ValidationResult struct {
	ProviderIdentifier string
	MailboxHint        string
	LatencyMs          int64
}

type HeaderPage struct {
	Messages   []NormalizedHeader
	NextCursor map[string]any
}

type ProviderClient interface {
	Validate(ctx context.Context, payload ProviderPayload) (*ValidationResult, error)
	ListHeaders(ctx context.Context, payload ProviderPayload, req ListHeadersRequest) (*HeaderPage, error)
	FetchMessageDetail(ctx context.Context, payload ProviderPayload, req FetchDetailRequest) (*MessageDetail, error)
}
```

The Outlook parser in `backend/internal/service/mailbox_service.go` should use the exact four-part contract:

```go
func (s *MailboxService) ParseOutlookImportBundle(raw string) (*OutlookImportBundle, error) {
	parts := strings.Split(strings.TrimSpace(raw), "----")
	if len(parts) != 4 {
		return nil, ErrMailboxImportFormat
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return nil, ErrMailboxImportFormat
		}
	}
	return &OutlookImportBundle{
		MailboxIdentifier: strings.TrimSpace(parts[0]),
		SecretSegment:     strings.TrimSpace(parts[1]),
		ProviderIdentifier: strings.TrimSpace(parts[2]),
		OpaqueTokenBundle: strings.TrimSpace(parts[3]),
	}, nil
}
```

Validation must also map Outlook-specific failures to the spec result codes and invalid-state handling:

```go
func (s *MailboxService) ValidateProvider(ctx context.Context, providerID int64) (*ProviderValidationResult, error) {
	provider, err := s.repo.GetProviderAccount(ctx, providerID)
	if err != nil {
		return nil, err
	}
	result, err := s.providers.ProviderFor(provider.ProviderKind).Validate(ctx, buildProviderPayloadFromAccount(provider))
	if err == nil {
		_ = s.repo.UpdateProviderValidation(ctx, providerID, MailboxProviderStatusActive, "", time.Now())
		return &ProviderValidationResult{Status: "active", ResultCode: "ok", LatencyMs: result.LatencyMs}, nil
	}
	switch {
	case errors.Is(err, mailbox.ErrInvalidFormat):
		_ = s.repo.UpdateProviderValidation(ctx, providerID, MailboxProviderStatusDraft, "invalid_format", time.Now())
		return nil, ErrMailboxImportFormat
	case errors.Is(err, mailbox.ErrExpiredOrRevoked):
		_ = s.repo.UpdateProviderValidation(ctx, providerID, MailboxProviderStatusInvalid, "expired_or_revoked", time.Now())
		return nil, ErrMailboxExpiredOrRevoked
	default:
		_ = s.repo.UpdateProviderValidation(ctx, providerID, MailboxProviderStatusInvalid, "validation_failed", time.Now())
		return nil, ErrMailboxValidationFailed
	}
}
```

Whenever a Microsoft bundle is created or replaced, update `last_imported_at` explicitly rather than relying on `created_at`; edit flows that keep the existing secret untouched must leave `last_imported_at` unchanged.

Add explicit capability testing so collector pages can validate IMAP, POP3, SMTP, and Microsoft inbound settings independently:

```go
func (s *MailboxService) TestCapability(ctx context.Context, capabilityID int64) (*CapabilityTestResult, error) {
	capability, provider, err := s.repo.GetCapabilityWithProvider(ctx, capabilityID)
	if err != nil {
		return nil, err
	}
	client := s.providers.ClientFor(capability.CapabilityKind, provider.ProviderKind)
	result, err := client.Validate(ctx, buildProviderPayload(provider, capability))
	if err != nil {
		_ = s.repo.UpdateCapabilityHealth(ctx, capabilityID, MailboxCapabilityHealthError, err.Error(), nil)
		return nil, err
	}
	_ = s.repo.UpdateCapabilityHealth(ctx, capabilityID, MailboxCapabilityHealthHealthy, "", ptrTime(time.Now()))
	return &CapabilityTestResult{Success: true, LatencyMs: result.LatencyMs}, nil
}
```

Add one audit helper in `backend/internal/service/mailbox_audit.go` and call it from provider create/update/status change, Outlook import, collector create/update, recipient create/update, match-value create/update, capability test, manual sync, batch sync, and inbox detail fetch:

```go
func (s *MailboxAuditService) Log(ctx context.Context, action string, operatorID int64, objectType string, objectID int64, result string, fields map[string]any) {
	logger.With(
		zap.String("component", "audit.mailbox"),
		zap.String("action", action),
		zap.Int64("operator_id", operatorID),
		zap.String("object_type", objectType),
		zap.Int64("object_id", objectID),
		zap.String("result", result),
		zap.Any("fields", fields),
	).Info("mailbox audit event")
}
```

The resolution service should enforce source-field short-circuiting:

```go
for _, source := range []candidateSource{
	{field: "envelope_recipient", values: input.EnvelopeRecipients},
	{field: "delivered_to", values: input.DeliveredTo},
	{field: "x_original_to", values: input.OriginalTo},
	{field: "to", values: input.To},
	{field: "cc", values: input.CC},
} {
	matches := collectMatches(source.values, values)
	if len(matches) == 0 {
		continue
	}
	if identitiesEqual(matches) {
		return resolved(matches[0], source.field)
	}
	return ambiguous(source.field, matches)
}
return unresolved()
```

Also wire the services in `backend/internal/service/wire.go` so the mailbox service receives the repository and provider clients. In the adapter tests, make the IMAP helper prove cursor-aware bounded reads, the POP3 helper prove message dedupe, and the Microsoft helper prove validation plus minimal Inbox-read success using the parsed import bundle.

- [ ] **Step 4: Re-run the service tests**

Run: `go test ./internal/service ./internal/pkg/mailbox -run 'TestMailboxService_|TestResolutionService_|TestBasicClient_|TestMicrosoftClient_'` from `backend/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/pkg/mailbox/provider.go backend/internal/pkg/mailbox/basic_client.go backend/internal/pkg/mailbox/microsoft_client.go backend/internal/pkg/mailbox/basic_client_test.go backend/internal/pkg/mailbox/microsoft_client_test.go backend/internal/service/mailbox_service.go backend/internal/service/mailbox_audit.go backend/internal/service/mailbox_resolution_service.go backend/internal/service/mailbox_service_test.go backend/internal/service/mailbox_resolution_service_test.go backend/internal/service/wire.go
git commit -m "feat: add mailbox validation and resolution services"
```

### Task 4: Implement Sync Orchestration, Batch Fan-Out, and Scheduled Runner

**Files:**
- Create: `backend/internal/service/mailbox_sync_service.go`
- Create: `backend/internal/service/mailbox_sync_runner_service.go`
- Create: `backend/internal/service/mailbox_sync_service_test.go`
- Modify: `backend/internal/service/wire.go`

- [ ] **Step 1: Write failing sync tests**

```go
func TestMailboxSyncService_CreateBatchSyncJobs_SplitsOneJobPerCapability(t *testing.T) {
	repo := &fakeMailboxRepo{}
	svc := NewMailboxSyncService(repo, fakeProviderRegistry{}, NewMailboxResolutionService(), time.Minute)

	err := svc.CreateBatchSyncJobs(context.Background(), service.BatchSyncRequest{
		CapabilityIDs: []int64{11, 12, 13},
		TriggeredBy:   7,
	})
	require.NoError(t, err)
	require.Len(t, repo.createdJobs, 3)
	require.NotEmpty(t, repo.createdJobs[0].BatchID)
	require.Equal(t, repo.createdJobs[0].BatchID, repo.createdJobs[2].BatchID)
}

func TestMailboxSyncService_RejectsConcurrentSyncForSameCapability(t *testing.T) {
	repo := &fakeMailboxRepo{runningCapabilityIDs: map[int64]bool{12: true}}
	svc := NewMailboxSyncService(repo, fakeProviderRegistry{}, NewMailboxResolutionService(), time.Minute)
	err := svc.CreateBatchSyncJobs(context.Background(), service.BatchSyncRequest{
		CapabilityIDs: []int64{12},
		TriggeredBy:   7,
	})
	require.ErrorIs(t, err, service.ErrMailboxSyncAlreadyRunning)
}

func TestMailboxSyncService_ExpandsCollectorIDsToInboundCapabilities(t *testing.T) {
	repo := &fakeMailboxRepo{collectorInboundCapabilities: map[int64][]int64{31: {101, 102}}}
	svc := NewMailboxSyncService(repo, fakeProviderRegistry{}, NewMailboxResolutionService(), time.Minute)
	err := svc.CreateBatchSyncJobs(context.Background(), service.BatchSyncRequest{CollectorIDs: []int64{31}, TriggeredBy: 7})
	require.NoError(t, err)
	require.Len(t, repo.createdJobs, 2)
}

func TestMailboxSyncService_FirstSyncUsesBoundedBackfill(t *testing.T) {
	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)
	svc := NewMailboxSyncService(&fakeMailboxRepo{}, fakeProviderRegistry{}, NewMailboxResolutionService(), time.Minute)
	req := svc.BuildListHeadersRequest(service.MailboxCapability{ID: 9, CapabilityKind: "inbound.imap"}, nil, now)
	require.Equal(t, 500, req.Limit)
	require.Equal(t, now.AddDate(0, 0, -30), req.Since)
}

func TestMailboxSyncService_FetchDetailMarksFailureWithoutChangingResolutionState(t *testing.T) {
	repo := &fakeMailboxRepo{
		headerByID: map[int64]*service.MailHeader{
			55: {ID: 55, CapabilityID: 9, ResolutionState: service.MailResolutionStateResolved, DetailFetchState: service.MailDetailFetchStateNotRequested},
		},
	}
	registry := fakeProviderRegistry{detailErr: errors.New("provider down")}
	svc := NewMailboxSyncService(repo, registry, NewMailboxResolutionService(), time.Minute)

	_, err := svc.FetchDetail(context.Background(), 55)
	require.Error(t, err)
	require.Equal(t, service.MailResolutionStateResolved, repo.updatedHeader.ResolutionState)
	require.Equal(t, service.MailDetailFetchStateFailed, repo.updatedHeader.DetailFetchState)
}

func TestMailboxSyncService_RunSync_PersistsHeadersCursorResolutionAndJobState(t *testing.T) {
	repo := &fakeMailboxRepo{}
	registry := fakeProviderRegistry{headerPage: &mailbox.HeaderPage{
		Messages: []mailbox.NormalizedHeader{{RemoteID: "msg-1", Subject: "Welcome", DeliveredTo: []string{"one@privaterelay.appleid.com"}}},
		NextCursor: map[string]any{"delta": "cursor-2"},
	}}
	svc := NewMailboxSyncService(repo, registry, NewMailboxResolutionService(), time.Minute)
	err := svc.RunSyncJob(context.Background(), &service.MailSyncJob{ID: 77, CapabilityID: 9, State: service.MailSyncJobStateQueued})
	require.NoError(t, err)
	require.Equal(t, service.MailSyncJobStateSucceeded, repo.updatedJob.State)
	require.Len(t, repo.upsertedHeaders, 1)
	require.Equal(t, "cursor-2", repo.updatedCapabilityCursor["delta"])
}

func TestMailboxSyncService_RunSync_MarksRetryableAndBackoffForTransientProviderErrors(t *testing.T) {
	repo := &fakeMailboxRepo{}
	registry := fakeProviderRegistry{listHeadersErr: mailbox.ErrTransientProviderFailure}
	svc := NewMailboxSyncService(repo, registry, NewMailboxResolutionService(), time.Minute)
	err := svc.RunSyncJob(context.Background(), &service.MailSyncJob{ID: 78, CapabilityID: 9, State: service.MailSyncJobStateQueued})
	require.Error(t, err)
	require.True(t, repo.updatedJob.Retryable)
	require.NotNil(t, repo.updatedJob.NextRetryAt)
}

func TestMailboxSyncRunnerService_CreatesScheduledJobsBeforeExecutingSync(t *testing.T) {
	repo := &fakeMailboxRepo{dueCapabilities: []*service.MailboxCapability{{ID: 9, CapabilityKind: "inbound.imap", SyncEnabled: true}}}
	runner := NewMailboxSyncRunnerService(repo, &fakeMailboxSyncService{})
	runner.runScheduledOnce(context.Background())
	require.Len(t, repo.createdJobs, 1)
	require.Equal(t, "schedule", repo.createdJobs[0].TriggerSource)
}
```

- [ ] **Step 2: Run the sync tests to verify they fail**

Run: `go test ./internal/service -run 'TestMailboxSyncService_|TestMailboxSyncRunnerService_'` from `backend/`
Expected: FAIL because the sync service and runner do not exist yet.

- [ ] **Step 3: Implement the sync service and cron-style runner**

The job fan-out should create one row per capability:

```go
func (s *MailboxSyncService) CreateBatchSyncJobs(ctx context.Context, req BatchSyncRequest) error {
	batchID := uuid.NewString()
	jobs := make([]*MailSyncJob, 0, len(req.CapabilityIDs))
	for _, capabilityID := range req.CapabilityIDs {
		jobs = append(jobs, &MailSyncJob{
			BatchID:       batchID,
			CapabilityID:  capabilityID,
			TriggerSource: "manual_batch",
			State:         MailSyncJobStateQueued,
			TriggeredBy:   req.TriggeredBy,
		})
	}
	_, err := s.repo.CreateSyncJobs(ctx, jobs)
	return err
}
```

Guard concurrent manual or scheduled sync on the same capability before creating a new job:

```go
func (s *MailboxSyncService) ensureCapabilityIdle(ctx context.Context, capabilityID int64) error {
	running, err := s.repo.HasActiveSyncJob(ctx, capabilityID)
	if err != nil {
		return err
	}
	if running {
		return ErrMailboxSyncAlreadyRunning
	}
	return nil
}
```

The actual sync pipeline must do more than enqueue work. `RunSyncJob` should:

```go
func (s *MailboxSyncService) RunSyncJob(ctx context.Context, job *MailSyncJob) error {
	_ = s.repo.UpdateSyncJobState(ctx, job.ID, MailSyncJobStateRunning, "")
	capability, provider, err := s.repo.GetCapabilityWithProvider(ctx, job.CapabilityID)
	if err != nil {
		return s.failJob(ctx, job.ID, err)
	}
	page, err := s.clientFor(capability, provider).ListHeaders(ctx, buildProviderPayload(provider, capability), s.BuildListHeadersRequest(*capability, capability.CursorState, time.Now()))
	if err != nil {
		return s.failJob(ctx, job.ID, err)
	}
	partial, err := s.upsertAndResolveHeaders(ctx, job, capability, page.Messages)
	if err != nil {
		return s.failJob(ctx, job.ID, err)
	}
	_ = s.repo.UpdateCapabilityCursor(ctx, capability.ID, page.NextCursor, time.Now())
	if partial {
		return s.repo.UpdateSyncJobState(ctx, job.ID, MailSyncJobStatePartial, "partial header failures")
	}
	return s.repo.UpdateSyncJobState(ctx, job.ID, MailSyncJobStateSucceeded, "")
}
```

`upsertAndResolveHeaders` must normalize headers, upsert rows in `mailbox_header_cache`, persist `flags` and `snippet`, run recipient resolution, persist `resolved_address`, `matched_value_id`, `resolution_source_field`, and `resolution_state`, and keep one bad message from failing the whole job.

For transient provider failures, classify the job as retryable and set exponential backoff metadata instead of only writing `failed` once:

```go
func (s *MailboxSyncService) failJob(ctx context.Context, jobID int64, err error) error {
	retryable := errors.Is(err, mailbox.ErrTransientProviderFailure) || errors.Is(err, context.DeadlineExceeded)
	nextRetryAt := (*time.Time)(nil)
	if retryable {
		backoff := time.Duration(min(60, 1<<min(5, s.repo.NextRetryCount(ctx, jobID)))) * time.Minute
		t := time.Now().Add(backoff)
		nextRetryAt = &t
	}
	return s.repo.MarkSyncJobFailed(ctx, jobID, retryable, nextRetryAt, err.Error())
}
```

The first-sync request builder should enforce the bounded window:

```go
func (s *MailboxSyncService) BuildListHeadersRequest(capability MailboxCapability, cursor map[string]any, now time.Time) mailbox.ListHeadersRequest {
	req := mailbox.ListHeadersRequest{Cursor: cursor, Folder: "INBOX"}
	if len(cursor) == 0 {
		req.Since = now.AddDate(0, 0, -30)
		req.Limit = 500
	}
	return req
}
```

Mirror the scheduled-test runner pattern for the scheduler:

```go
func (s *MailboxSyncRunnerService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		c := cron.New()
		_, err := c.AddFunc("*/5 * * * *", func() { s.runScheduled() })
		if err != nil {
			logger.LegacyPrintf("service.mailbox_sync_runner", "[MailboxSyncRunner] not started: %v", err)
			return
		}
		s.cron = c
		s.cron.Start()
	})
}
```

Do not bypass persisted jobs for scheduled sync. The runner should claim due inbound capabilities, create one `MailSyncJob` with `trigger_source = schedule` per capability, and then execute `RunSyncJob` on those persisted jobs:

```go
func (s *MailboxSyncRunnerService) runScheduledOnce(ctx context.Context) {
	capabilities, _ := s.repo.ClaimDueCapabilities(ctx, time.Now(), 100)
	for _, capability := range capabilities {
		job := &MailSyncJob{CapabilityID: capability.ID, TriggerSource: "schedule", State: MailSyncJobStateQueued}
		createdJobs, _ := s.repo.CreateSyncJobs(ctx, []*MailSyncJob{job})
		if len(createdJobs) == 1 {
			_ = s.syncService.RunSyncJob(ctx, createdJobs[0])
		}
	}
}
```

Persist capability state transitions explicitly so the API and UI can expose the spec state machine: `sync_enabled = false -> paused`, job start -> `syncing`, retryable failure -> `warning`, terminal failure -> `error`, successful validation or sync -> `healthy`.

When detail fetch fails, update only `detail_fetch_state = 'failed'` and keep the existing `resolution_state` unchanged.

- [ ] **Step 4: Re-run the sync tests**

Run: `go test ./internal/service -run 'TestMailboxSyncService_|TestMailboxSyncRunnerService_'` from `backend/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/mailbox_sync_service.go backend/internal/service/mailbox_sync_runner_service.go backend/internal/service/mailbox_sync_service_test.go backend/internal/service/wire.go
git commit -m "feat: add mailbox sync orchestration"
```

### Task 5: Expose the Admin Mailbox HTTP API

**Files:**
- Create: `backend/internal/handler/dto/mailbox.go`
- Create: `backend/internal/handler/admin/mailbox_handler.go`
- Create: `backend/internal/handler/admin/mailbox_handler_test.go`
- Modify: `backend/internal/handler/handler.go`
- Modify: `backend/internal/handler/wire.go`
- Modify: `backend/internal/server/routes/admin.go`

- [ ] **Step 1: Write failing handler tests**

```go
func TestMailboxHandler_CreateProvider_ValidatesBundleAndReturns201(t *testing.T) {
	service := &fakeMailboxAdminService{}
	h := NewMailboxHandler(service)
	w := performJSONRequest(t, h.CreateProvider, http.MethodPost, "/api/v1/admin/mailbox/providers", map[string]any{
		"display_name": "Outlook Seed",
		"provider_kind": "microsoft",
		"auth_kind": "import_bundle",
		"import_payload": "user@example.com----secret----client----opaque",
	})
	require.Equal(t, http.StatusCreated, w.Code)
}

func TestMailboxHandler_BatchSync_RequiresAtLeastOneCapability(t *testing.T) {
	h := NewMailboxHandler(&fakeMailboxAdminService{})
	w := performJSONRequest(t, h.BatchSyncCollectors, http.MethodPost, "/api/v1/admin/mailbox/collectors/batch-sync", map[string]any{
		"capability_ids": []int64{},
	})
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMailboxHandler_TestCapability_ReturnsIndependentHealthResult(t *testing.T) {
	service := &fakeMailboxAdminService{capabilityTestResult: &service.CapabilityTestResult{Success: true, LatencyMs: 42}}
	h := NewMailboxHandler(service)
	w := performJSONRequest(t, h.TestCapability, http.MethodPost, "/api/v1/admin/mailbox/capabilities/21/test", nil)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestMailboxHandler_ImportRecipientExactAddresses_AcceptsBulkPayload(t *testing.T) {
	service := &fakeMailboxAdminService{}
	h := NewMailboxHandler(service)
	w := performJSONRequest(t, h.ImportRecipientExactAddresses, http.MethodPost, "/api/v1/admin/mailbox/recipients/7/import-exact-addresses", map[string]any{
		"addresses": []string{"one@privaterelay.appleid.com", "two@privaterelay.appleid.com"},
	})
	require.Equal(t, http.StatusOK, w.Code)
}
```

- [ ] **Step 2: Run the handler tests to verify they fail**

Run: `go test ./internal/handler/admin -run 'TestMailboxHandler_'` from `backend/`
Expected: FAIL because the mailbox handler and routes do not exist yet.

- [ ] **Step 3: Implement DTOs, handler methods, and route registration**

The request surface should be explicit and narrow:

```go
type CreateProviderRequest struct {
	DisplayName   string `json:"display_name" binding:"required,max=100"`
	ProviderKind  string `json:"provider_kind" binding:"required,oneof=basic microsoft"`
	AuthKind      string `json:"auth_kind" binding:"required"`
	ImportPayload string `json:"import_payload"`
	MailboxHint   string `json:"mailbox_hint"`
}

type CapabilityInput struct {
	ID                  *int64         `json:"id"`
	CapabilityKind      string         `json:"capability_kind" binding:"required,oneof=inbound.imap inbound.pop3 inbound.microsoft_graph outbound.smtp"`
	ProviderAccountID   int64          `json:"provider_account_id" binding:"required"`
	ConnectionConfig    map[string]any `json:"connection_config"`
	SyncEnabled         bool           `json:"sync_enabled"`
	SyncIntervalSeconds int            `json:"sync_interval_seconds"`
}

type UpsertCollectorRequest struct {
	EmailAddress string            `json:"email_address" binding:"required,email"`
	DisplayName  string            `json:"display_name" binding:"required,max=120"`
	BusinessTags []string          `json:"business_tags"`
	Capabilities []CapabilityInput `json:"capabilities" binding:"required,min=1"`
}

type BatchSyncCollectorsRequest struct {
	CollectorIDs  []int64 `json:"collector_ids"`
	CapabilityIDs []int64 `json:"capability_ids"`
}

type ImportRecipientExactAddressesRequest struct {
	Addresses []string `json:"addresses" binding:"required,min=1,dive,email"`
}

type UpdateProviderStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active disabled invalid"`
}

type UpdateCollectorStatusRequest struct {
	Enabled bool `json:"enabled"`
}

type UpdateCapabilityStatusRequest struct {
	SyncEnabled bool `json:"sync_enabled"`
}

type UpdateRecipientStatusRequest struct {
	Active bool `json:"active"`
}

type ListInboxRequest struct {
	CollectorID      *int64  `form:"collector_id"`
	RecipientID      *int64  `form:"recipient_id"`
	Folder           string  `form:"folder"`
	StartDate        string  `form:"start_date"`
	EndDate          string  `form:"end_date"`
	ResolutionState  string  `form:"resolution_state"`
	DetailFetchState string  `form:"detail_fetch_state"`
	Page             int     `form:"page,default=1"`
	PageSize         int     `form:"page_size,default=20"`
}

type BatchSyncStatusDTO struct {
	BatchID       string `json:"batch_id"`
	QueuedCount   int    `json:"queued_count"`
	RunningCount  int    `json:"running_count"`
	SuccessCount  int    `json:"success_count"`
	PartialCount  int    `json:"partial_count"`
	FailureCount  int    `json:"failure_count"`
}
```

Enforce one batch-target mode per request: either `collector_ids` or `capability_ids` must be present, never both. When `collector_ids` are provided, the backend expands them to all enabled inbound capabilities on those collectors before job creation.

Return capability rows with both persisted sync control and status fields so the frontend can render the spec state machine without guessing:

```go
type CapabilityDTO struct {
	ID          int64  `json:"id"`
	Kind        string `json:"capability_kind"`
	HealthState string `json:"health_state"`
	SyncEnabled bool   `json:"sync_enabled"`
}
```

Do not return raw secrets in any read DTO. Provider and capability read responses should expose only masked summaries and replacement flags, for example:

```go
type ProviderAccountDTO struct {
	ID                int64  `json:"id"`
	DisplayName       string `json:"display_name"`
	ProviderKind      string `json:"provider_kind"`
	Status            string `json:"status"`
	MailboxHint       string `json:"mailbox_hint"`
	ProviderIdentifier string `json:"provider_identifier"`
	SecretConfigured  bool   `json:"secret_configured"`
	MaskedSecretHint  string `json:"masked_secret_hint"`
}
```

Write APIs should use replacement-only secret fields such as `import_payload` or `connection_config.secret_replacement`, and edit responses must never echo stored credentials or tokens.

Register the route group in `backend/internal/server/routes/admin.go`:

```go
func registerMailboxRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	mailbox := admin.Group("/mailbox")
	{
		providers := mailbox.Group("/providers")
		providers.GET("", h.Admin.Mailbox.ListProviders)
		providers.POST("", h.Admin.Mailbox.CreateProvider)
		providers.PUT("/:id", h.Admin.Mailbox.UpdateProvider)
		providers.POST("/:id/validate", h.Admin.Mailbox.ValidateProvider)
		providers.POST("/:id/status", h.Admin.Mailbox.UpdateProviderStatus)

		collectors := mailbox.Group("/collectors")
		collectors.GET("", h.Admin.Mailbox.ListCollectors)
		collectors.POST("", h.Admin.Mailbox.CreateCollector)
		collectors.PUT("/:id", h.Admin.Mailbox.UpdateCollector)
		collectors.POST("/:id/sync", h.Admin.Mailbox.SyncCollector)
		collectors.POST("/batch-sync", h.Admin.Mailbox.BatchSyncCollectors)
		collectors.POST("/:id/status", h.Admin.Mailbox.UpdateCollectorStatus)

		capabilities := mailbox.Group("/capabilities")
		capabilities.POST("", h.Admin.Mailbox.CreateCapability)
		capabilities.PUT("/:id", h.Admin.Mailbox.UpdateCapability)
		capabilities.POST("/:id/test", h.Admin.Mailbox.TestCapability)
		capabilities.POST("/:id/status", h.Admin.Mailbox.UpdateCapabilityStatus)

		mailbox.GET("/recipients", h.Admin.Mailbox.ListRecipients)
		mailbox.POST("/recipients", h.Admin.Mailbox.CreateRecipient)
		mailbox.PUT("/recipients/:id", h.Admin.Mailbox.UpdateRecipient)
		mailbox.POST("/recipients/:id/import-exact-addresses", h.Admin.Mailbox.ImportRecipientExactAddresses)
		mailbox.POST("/recipients/:id/status", h.Admin.Mailbox.UpdateRecipientStatus)
		mailbox.GET("/inbox", h.Admin.Mailbox.ListInbox)
		mailbox.GET("/inbox/:id", h.Admin.Mailbox.GetInboxHeader)
		mailbox.GET("/inbox/:id/detail", h.Admin.Mailbox.GetInboxDetail)

		syncJobs := mailbox.Group("/sync-jobs")
		syncJobs.GET("/batches/:batch_id", h.Admin.Mailbox.GetBatchSyncStatus)
	}
}
```

Also extend `backend/internal/handler/handler.go` and `backend/internal/handler/wire.go` to expose `Admin.Mailbox`.

- [ ] **Step 4: Re-run the handler tests**

Run: `go test ./internal/handler/admin -run 'TestMailboxHandler_'` from `backend/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler/dto/mailbox.go backend/internal/handler/admin/mailbox_handler.go backend/internal/handler/admin/mailbox_handler_test.go backend/internal/handler/handler.go backend/internal/handler/wire.go backend/internal/server/routes/admin.go
git commit -m "feat: add admin mailbox api"
```

### Task 5A: Add Backend Integration Coverage for Mailbox Flows

**Files:**
- Create: `backend/internal/integration/mailbox_integration_test.go`

- [ ] **Step 1: Write the failing backend integration tests**

```go
//go:build integration

func TestMailboxIntegration_ValidateCapabilitySyncAndPersistResolution(t *testing.T) {
	ctx := context.Background()
	h := newMailboxIntegrationHarness(t)

	provider := h.createProvider(t, "basic", "imap_password")
	collector := h.createCollectorWithCapabilities(t, provider.ID, []string{"inbound.imap"})

	_, err := h.mailboxService.TestCapability(ctx, collector.Capabilities[0].ID)
	require.NoError(t, err)

	err = h.syncService.RunSyncJob(ctx, &service.MailSyncJob{ID: h.seedSyncJob(t, collector.Capabilities[0].ID), CapabilityID: collector.Capabilities[0].ID, State: service.MailSyncJobStateQueued})
	require.NoError(t, err)

	headers, total, err := h.repo.ListHeaders(ctx, service.MailHeaderListFilter{CollectorID: collector.ID})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, service.MailResolutionStateResolved, headers[0].ResolutionState)
	require.NotEmpty(t, headers[0].ResolvedAddress)
	require.NotEmpty(t, headers[0].ResolutionSourceField)
}

func TestMailboxIntegration_FetchDetailOnDemand(t *testing.T) {
	ctx := context.Background()
	h := newMailboxIntegrationHarness(t)
	headerID := h.seedHeaderForDetail(t)
	detail, err := h.syncService.FetchDetail(ctx, headerID)
	require.NoError(t, err)
	require.NotEmpty(t, detail.TextBody)
	require.Equal(t, service.MailDetailFetchStateSucceeded, h.repo.MustGetHeader(t, headerID).DetailFetchState)
}
```

- [ ] **Step 2: Run the backend integration tests to verify they fail**

Run: `go test -tags=integration ./internal/integration -run 'TestMailboxIntegration_'` from `backend/`
Expected: FAIL because the mailbox integration harness and end-to-end persistence path do not exist yet.

- [ ] **Step 3: Implement the integration harness and cross-layer assertions**

In `backend/internal/integration/mailbox_integration_test.go`, construct the real repository, mailbox service, sync service, and fake protocol adapters against the integration database. The harness should:

```go
type mailboxIntegrationHarness struct {
	repo           service.MailboxRepository
	mailboxService *service.MailboxService
	syncService    *service.MailboxSyncService
}
```

Cover these spec-required paths automatically:

- provider validation and provider status persistence
- capability connectivity test and independent capability health update
- incremental sync, cursor persistence, and `mailbox_header_cache` upsert
- recipient resolution persistence into `mailbox_header_cache`
- on-demand message body fetch and `detail_fetch_state` update

- [ ] **Step 4: Re-run the backend integration tests**

Run: `go test -tags=integration ./internal/integration -run 'TestMailboxIntegration_'` from `backend/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/integration/mailbox_integration_test.go
git commit -m "test: add mailbox integration coverage"
```

### Task 6: Add Frontend Contracts, Routes, Sidebar, and Page Shells

**Files:**
- Create: `frontend/src/types/mailbox.ts`
- Modify: `frontend/src/types/index.ts`
- Create: `frontend/src/api/admin/mailbox.ts`
- Create: `frontend/src/api/__tests__/mailbox.spec.ts`
- Modify: `frontend/src/api/admin/index.ts`
- Create: `frontend/src/views/admin/MailboxProvidersView.vue`
- Create: `frontend/src/views/admin/CollectorMailboxesView.vue`
- Create: `frontend/src/views/admin/RecipientIdentitiesView.vue`
- Create: `frontend/src/views/admin/MailInboxView.vue`
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/router/__tests__/guards.spec.ts`
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/i18n/locales/en.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`

- [ ] **Step 1: Write failing API and route tests**

```ts
import { describe, expect, it, vi } from 'vitest'
import { createRouter, createWebHistory } from 'vue-router'
import * as mailboxApi from '@/api/admin/mailbox'
import routes from '@/router'

describe('mailbox admin api', () => {
  it('posts batch sync payload to the mailbox endpoint', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { batch_id: 'b1' } })
    await mailboxApi.batchSyncCollectors({ capability_ids: [11, 12] })
    expect(post).toHaveBeenCalledWith('/admin/mailbox/collectors/batch-sync', { capability_ids: [11, 12] })
  })

  it('posts exact-address import payload to the recipient import endpoint', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { imported: 2 } })
    await mailboxApi.importRecipientExactAddresses(7, ['one@privaterelay.appleid.com', 'two@privaterelay.appleid.com'])
    expect(post).toHaveBeenCalledWith('/admin/mailbox/recipients/7/import-exact-addresses', { addresses: ['one@privaterelay.appleid.com', 'two@privaterelay.appleid.com'] })
  })
})

describe('mailbox admin routes', () => {
  it('registers the admin mailbox pages as admin-only routes', () => {
    const mailboxProviders = routes.find(route => route.path === '/admin/mailbox/providers')
    expect(mailboxProviders?.meta?.requiresAdmin).toBe(true)
  })
})
```

- [ ] **Step 2: Run the frontend contract tests to verify they fail**

Run: `pnpm --dir frontend run test:run -- src/api/__tests__/mailbox.spec.ts src/router/__tests__/guards.spec.ts`
Expected: FAIL because the mailbox API wrapper, routes, and page components do not exist yet.

- [ ] **Step 3: Implement the shared types, API wrapper, routes, menu entries, and shell pages**

Define typed frontend contracts in `frontend/src/types/mailbox.ts`:

```ts
export interface MailboxProviderAccount {
  id: number
  display_name: string
  provider_kind: 'basic' | 'microsoft'
  auth_kind: string
  status: 'draft' | 'active' | 'invalid' | 'disabled'
  mailbox_hint: string
  provider_identifier: string
  last_validation_at: string | null
  last_validation_error: string
}

export interface MailboxHeaderRecord {
  id: number
  subject: string
  from: string
  resolved_recipient: string
  resolved_address: string
  source_mailbox: string
  folder: string
  flags: string[]
  snippet: string
  matched_value_id: number | null
  resolution_source_field: string | null
  resolution_state: 'resolved' | 'unresolved' | 'ambiguous'
  detail_fetch_state: 'not_requested' | 'succeeded' | 'failed'
  received_at: string
}
```

The API wrapper should mirror the backend shape exactly:

```ts
export async function batchSyncCollectors(payload: { collector_ids?: number[]; capability_ids?: number[] }) {
  const { data } = await apiClient.post<{ batch_id: string }>('/admin/mailbox/collectors/batch-sync', payload)
  return data
}

export async function importRecipientExactAddresses(recipientId: number, addresses: string[]) {
  const { data } = await apiClient.post<{ imported: number }>(`/admin/mailbox/recipients/${recipientId}/import-exact-addresses`, { addresses })
  return data
}

export async function updateRecipient(recipientId: number, payload: { name?: string; match_values?: RecipientMatchValueInput[] }) {
  const { data } = await apiClient.put(`/admin/mailbox/recipients/${recipientId}`, payload)
  return data
}

export async function validateProvider(providerId: number) {
  const { data } = await apiClient.post(`/admin/mailbox/providers/${providerId}/validate`)
  return data
}

export async function updateProviderStatus(providerId: number, payload: { status: 'active' | 'disabled' | 'invalid' }) {
  const { data } = await apiClient.post(`/admin/mailbox/providers/${providerId}/status`, payload)
  return data
}

export async function getBatchSyncStatus(batchId: string) {
  const { data } = await apiClient.get<BatchSyncStatus>(`/admin/mailbox/sync-jobs/batches/${batchId}`)
  return data
}
```

Add the four routes to `frontend/src/router/index.ts` with `requiresAdmin: true`, and add matching menu entries to `frontend/src/components/layout/AppSidebar.vue` immediately after `/admin/accounts`.

Use shell pages that already compile with `AppLayout` and `TablePageLayout`, for example:

```vue
<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex items-center justify-between gap-3">
          <h1 class="text-xl font-semibold">{{ t('admin.mailbox.providers.title') }}</h1>
        </div>
      </template>
      <template #table>
        <div class="rounded-xl border border-dashed border-gray-300 p-6 text-sm text-gray-500">
          {{ t('admin.mailbox.comingSoon') }}
        </div>
      </template>
    </TablePageLayout>
  </AppLayout>
</template>
```

- [ ] **Step 4: Re-run the frontend contract tests**

Run: `pnpm --dir frontend run test:run -- src/api/__tests__/mailbox.spec.ts src/router/__tests__/guards.spec.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/types/mailbox.ts frontend/src/types/index.ts frontend/src/api/admin/mailbox.ts frontend/src/api/__tests__/mailbox.spec.ts frontend/src/api/admin/index.ts frontend/src/views/admin/MailboxProvidersView.vue frontend/src/views/admin/CollectorMailboxesView.vue frontend/src/views/admin/RecipientIdentitiesView.vue frontend/src/views/admin/MailInboxView.vue frontend/src/router/index.ts frontend/src/router/__tests__/guards.spec.ts frontend/src/components/layout/AppSidebar.vue frontend/src/i18n/locales/en.ts frontend/src/i18n/locales/zh.ts
git commit -m "feat: add mailbox admin navigation and api contracts"
```

### Task 7: Build the Provider, Collector, and Recipient Admin Pages

**Files:**
- Create: `frontend/src/components/admin/mailbox/ProviderAccountDialog.vue`
- Create: `frontend/src/components/admin/mailbox/CollectorMailboxDialog.vue`
- Create: `frontend/src/components/admin/mailbox/BatchSyncToolbar.vue`
- Create: `frontend/src/components/admin/mailbox/RecipientIdentityDialog.vue`
- Create: `frontend/src/views/admin/__tests__/MailboxProvidersView.spec.ts`
- Create: `frontend/src/views/admin/__tests__/CollectorMailboxesView.spec.ts`
- Create: `frontend/src/views/admin/__tests__/RecipientIdentitiesView.spec.ts`
- Modify: `frontend/src/views/admin/MailboxProvidersView.vue`
- Modify: `frontend/src/views/admin/CollectorMailboxesView.vue`
- Modify: `frontend/src/views/admin/RecipientIdentitiesView.vue`

- [ ] **Step 1: Write failing view tests**

```ts
it('shows provider validation results after creating a Microsoft provider', async () => {
  vi.mocked(mailboxApi.listProviders).mockResolvedValue({ items: [], total: 0 })
  vi.mocked(mailboxApi.createProvider).mockResolvedValue({ id: 1, display_name: 'Outlook Seed', provider_kind: 'microsoft', auth_kind: 'import_bundle', status: 'active', mailbox_hint: 'user@example.com', provider_identifier: 'client-id', last_validation_at: null, last_validation_error: '' })
  const wrapper = render(MailboxProvidersView)
  await wrapper.find('[data-testid="provider-create-button"]').trigger('click')
  expect(await screen.findByText('Outlook Seed')).toBeInTheDocument()
})

it('validates and enables or disables a provider from the provider page', async () => {
  vi.mocked(mailboxApi.validateProvider).mockResolvedValue({ status: 'active', result_code: 'ok', latency_ms: 42 })
  vi.mocked(mailboxApi.updateProviderStatus).mockResolvedValue({ id: 1, status: 'disabled' })
  const wrapper = render(MailboxProvidersView)
  await wrapper.find('[data-testid="provider-validate-1"]').trigger('click')
  await wrapper.find('[data-testid="provider-status-toggle-1"]').trigger('click')
  expect(mailboxApi.validateProvider).toHaveBeenCalledWith(1)
  expect(mailboxApi.updateProviderStatus).toHaveBeenCalledWith(1, { status: 'disabled' })
})

it('lets the admin attach capabilities, test connectivity, and batch sync from the collector page', async () => {
  vi.mocked(mailboxApi.batchSyncCollectors).mockResolvedValue({ batch_id: 'b1' })
  const wrapper = render(CollectorMailboxesView)
  await wrapper.find('[data-testid="collector-create-button"]').trigger('click')
  await wrapper.find('[data-testid="add-capability-imap"]').trigger('click')
  await wrapper.find('[data-testid="save-collector-button"]').trigger('click')
  await wrapper.find('[data-testid="test-capability-0"]').trigger('click')
  await wrapper.find('[data-testid="select-collector-row-1"]').setValue(true)
  await wrapper.find('[data-testid="collector-batch-sync-button"]').trigger('click')
  expect(mailboxApi.batchSyncCollectors).toHaveBeenCalledWith({ collector_ids: [1] })
})

it('supports batch sync for selected inbound capabilities and shows batch progress summary', async () => {
  vi.mocked(mailboxApi.batchSyncCollectors).mockResolvedValue({ batch_id: 'b2' })
  vi.mocked(mailboxApi.getBatchSyncStatus).mockResolvedValue({ batch_id: 'b2', queued_count: 0, running_count: 1, success_count: 1, partial_count: 0, failure_count: 0 })
  const wrapper = render(CollectorMailboxesView)
  await wrapper.find('[data-testid="select-capability-101"]').setValue(true)
  await wrapper.find('[data-testid="capability-batch-sync-button"]').trigger('click')
  expect(mailboxApi.batchSyncCollectors).toHaveBeenCalledWith({ capability_ids: [101] })
  expect(await screen.findByText('1')).toBeInTheDocument()
})

it('lets the admin add multiple suffix rules and exact addresses', async () => {
  const wrapper = render(RecipientIdentitiesView)
  await wrapper.find('[data-testid="recipient-create-button"]').trigger('click')
  await wrapper.find('[data-testid="add-exact-address"]').trigger('click')
  await wrapper.find('[data-testid="add-domain-suffix"]').trigger('click')
  expect(wrapper.findAll('[data-testid="match-value-row"]').length).toBe(2)
})

it('imports iCloud relay exact addresses in bulk', async () => {
  vi.mocked(mailboxApi.importRecipientExactAddresses).mockResolvedValue({ imported: 2 })
  const wrapper = render(RecipientIdentitiesView)
  await wrapper.find('[data-testid="recipient-import-button"]').trigger('click')
  await wrapper.find('[data-testid="recipient-import-textarea"]').setValue('one@privaterelay.appleid.com\ntwo@privaterelay.appleid.com')
  await wrapper.find('[data-testid="recipient-import-submit"]').trigger('click')
  expect(mailboxApi.importRecipientExactAddresses).toHaveBeenCalledWith(expect.any(Number), ['one@privaterelay.appleid.com', 'two@privaterelay.appleid.com'])
})

it('edits match priority and active state inline', async () => {
  const wrapper = render(RecipientIdentitiesView)
  await wrapper.find('[data-testid="match-priority-0"]').setValue('5')
  await wrapper.find('[data-testid="match-active-0"]').setValue(false)
  await wrapper.find('[data-testid="recipient-save-button"]').trigger('click')
  expect(mailboxApi.updateRecipient).toHaveBeenCalledWith(expect.any(Number), expect.objectContaining({
    match_values: [expect.objectContaining({ priority: 5, active: false })]
  }))
})
```

- [ ] **Step 2: Run the view tests to verify they fail**

Run: `pnpm --dir frontend run test:run -- src/views/admin/__tests__/MailboxProvidersView.spec.ts src/views/admin/__tests__/CollectorMailboxesView.spec.ts src/views/admin/__tests__/RecipientIdentitiesView.spec.ts`
Expected: FAIL because the dialogs and the real page logic do not exist yet.

- [ ] **Step 3: Implement the dialogs and page logic**

The provider dialog should expose the Outlook import contract clearly:

```ts
const form = reactive({
  display_name: '',
  provider_kind: 'microsoft',
  auth_kind: 'import_bundle',
  import_payload: '',
  mailbox_hint: ''
})

async function submit() {
  await mailboxApi.createProvider(form)
  emit('saved')
}
```

For edit flows, keep secret inputs write-only: render `secret_configured` or `masked_secret_hint`, leave `import_payload` and other secret fields blank by default, and only send replacement values when the admin explicitly changes them.

The provider page must also expose two row actions backed by tests: `Validate` to show the latest validation result and `Enable/Disable` to flip provider status without reopening the edit dialog.

The recipient dialog should let one identity own many values:

```ts
const values = ref([{ match_type: 'exact_address', match_value: '', priority: 100, active: true }])

function addMatchValue(matchType: 'exact_address' | 'domain_suffix') {
  values.value.push({ match_type: matchType, match_value: '', priority: 100, active: true })
}
```

The recipient editor must also expose per-row priority and active toggles because suffix conflict resolution depends on both:

```ts
function updateMatchValue(index: number, patch: Partial<RecipientMatchValueInput>) {
  values.value[index] = { ...values.value[index], ...patch }
}
```

Add a bulk exact-address import flow for iCloud relay aliases:

```ts
const importText = ref('')

async function importExactAddresses(recipientId: number) {
  const addresses = importText.value
    .split(/\r?\n/)
    .map(value => value.trim())
    .filter(Boolean)
  if (!addresses.length) return
  await mailboxApi.importRecipientExactAddresses(recipientId, addresses)
  emit('saved')
}
```

The collector page should render collector rows plus their capabilities and action buttons for capability create/update, independent capability pause/enable, capability test, collector edit, single-row sync, collector-page batch sync, capability-level batch sync, and batch-progress polling through `getBatchSyncStatus`.

- [ ] **Step 4: Re-run the view tests**

Run: `pnpm --dir frontend run test:run -- src/views/admin/__tests__/MailboxProvidersView.spec.ts src/views/admin/__tests__/CollectorMailboxesView.spec.ts src/views/admin/__tests__/RecipientIdentitiesView.spec.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/admin/mailbox/ProviderAccountDialog.vue frontend/src/components/admin/mailbox/CollectorMailboxDialog.vue frontend/src/components/admin/mailbox/BatchSyncToolbar.vue frontend/src/components/admin/mailbox/RecipientIdentityDialog.vue frontend/src/views/admin/__tests__/MailboxProvidersView.spec.ts frontend/src/views/admin/__tests__/CollectorMailboxesView.spec.ts frontend/src/views/admin/__tests__/RecipientIdentitiesView.spec.ts frontend/src/views/admin/MailboxProvidersView.vue frontend/src/views/admin/CollectorMailboxesView.vue frontend/src/views/admin/RecipientIdentitiesView.vue
git commit -m "feat: add mailbox admin management pages"
```

### Task 8: Build the Inbox Page and Detail Drawer

**Files:**
- Create: `frontend/src/components/admin/mailbox/MailInboxDetailDrawer.vue`
- Create: `frontend/src/views/admin/__tests__/MailInboxView.spec.ts`
- Modify: `frontend/src/views/admin/MailInboxView.vue`

- [ ] **Step 1: Write the failing inbox test**

```ts
it('shows cached headers and fetches detail on demand', async () => {
  vi.mocked(mailboxApi.listInbox).mockResolvedValue({ items: [{ id: 9, subject: 'Welcome', from: 'sender@example.com', resolved_recipient: 'relay@example.com', source_mailbox: 'collector@example.com', folder: 'INBOX', resolution_state: 'resolved', detail_fetch_state: 'not_requested', received_at: '2026-03-28T12:00:00Z' }], total: 1 })
  vi.mocked(mailboxApi.getInboxDetail).mockResolvedValue({ html_body: '<p>Hello</p>', text_body: 'Hello', attachments: [] })
  const wrapper = render(MailInboxView)
  expect(await screen.findByText('Welcome')).toBeInTheDocument()
  await wrapper.find('[data-testid="open-detail-9"]').trigger('click')
  expect(await screen.findByText('Hello')).toBeInTheDocument()
})

it('passes date range filters to inbox list queries', async () => {
  vi.mocked(mailboxApi.listInbox).mockResolvedValue({ items: [], total: 0 })
  const wrapper = render(MailInboxView)
  await wrapper.find('[data-testid="inbox-date-start"]').setValue('2026-03-01')
  await wrapper.find('[data-testid="inbox-date-end"]').setValue('2026-03-28')
  await wrapper.find('[data-testid="inbox-apply-filters"]').trigger('click')
  expect(mailboxApi.listInbox).toHaveBeenLastCalledWith(expect.objectContaining({ start_date: '2026-03-01', end_date: '2026-03-28' }))
})
```

- [ ] **Step 2: Run the inbox test to verify it fails**

Run: `pnpm --dir frontend run test:run -- src/views/admin/__tests__/MailInboxView.spec.ts`
Expected: FAIL because the inbox page and detail drawer do not implement the required interactions yet.

- [ ] **Step 3: Implement the inbox page and detail drawer**

The detail drawer should fetch body data lazily:

```ts
watch(() => props.open && props.headerId, async (ready) => {
  if (!ready) return
  detail.value = await mailboxApi.getInboxDetail(props.headerId!)
})
```

Render filters for collector mailbox, recipient identity, folder, date range, resolution state, and detail fetch state, but keep the page read-only.

- [ ] **Step 4: Re-run the inbox test**

Run: `pnpm --dir frontend run test:run -- src/views/admin/__tests__/MailInboxView.spec.ts`
Expected: PASS

- [ ] **Step 5: Run the full verification suite**

Run these commands from the repository root in order:

```bash
make -C backend test
pnpm --dir frontend run test:run
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
```

Expected:

- backend tests and `golangci-lint` pass
- frontend Vitest suite passes
- frontend typecheck passes
- frontend production build passes

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/admin/mailbox/BatchSyncToolbar.vue frontend/src/components/admin/mailbox/MailInboxDetailDrawer.vue frontend/src/views/admin/__tests__/MailInboxView.spec.ts frontend/src/views/admin/MailInboxView.vue
git commit -m "feat: add mailbox inbox operations"
```

### Task 9: Run Real Mailbox Integration Verification With Fresh Test Accounts

**Files:**
- Verify only: runtime configuration and test credentials

- [ ] **Step 1: Provision the fresh test accounts defined in the spec**

Prepare these inputs outside git:

```text
- IMAP test inbox with harmless seed mail
- POP3 test inbox with harmless seed mail
- SMTP connectivity test account
- Outlook import-bundle test account
- catch-all forwarding test setup with at least two suffix rules
- imported iCloud relay aliases
```

Expected: all credentials remain outside tracked files and outside test fixtures committed to git.

- [ ] **Step 2: Exercise the admin mailbox flows manually against the running app**

Run the backend and frontend locally, then verify:

```text
1. create and validate a Microsoft provider account
2. create and validate a basic provider account
3. create collector mailboxes and attach inbound capabilities
4. create recipient identities with exact aliases and multiple suffix rules
5. trigger single sync and batch sync
6. verify resolved, unresolved, and ambiguous inbox rows
7. open detail view and verify lazy body fetch
```

Expected: every acceptance criterion in the spec is observable in the UI or API.

- [ ] **Step 3: Capture any integration-only follow-up as a separate issue, not ad-hoc scope creep**

Record only concrete defects or protocol edge cases such as:

```text
- provider-specific validation mismatch
- POP3 dedupe issue
- Outlook import normalization issue
- real-world header parsing gap
```

Expected: the implementation remains within the approved v1 spec.

- [ ] **Step 4: Commit**

```bash
# no code commit required if no code changed; otherwise commit only the verified fixes from captured issues
```
