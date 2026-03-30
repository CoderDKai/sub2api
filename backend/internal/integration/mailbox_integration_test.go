//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	mailboxpkg "github.com/Wei-Shaw/sub2api/internal/pkg/mailbox"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

var integrationMailboxDB *sql.DB

func TestMailboxIntegration_RunWithCleanupAlwaysExecutesCleanup(t *testing.T) {
	cleaned := false
	code := runWithCleanup(func() int {
		return 7
	}, func() {
		cleaned = true
	})

	require.Equal(t, 7, code)
	require.True(t, cleaned)
}

func TestMailboxIntegration_BuildDockerHostFallbacksDoesNotOverrideExplicitHost(t *testing.T) {
	require.Nil(t, buildDockerHostFallbacks("unix:///explicit.sock", []string{"/tmp/docker.sock"}))
	require.Equal(t,
		[]string{"unix:///var/run/docker.sock", "unix:///Users/geniusk/.colima/default/docker.sock"},
		buildDockerHostFallbacks("", []string{"/var/run/docker.sock", "", "/var/run/docker.sock", "/Users/geniusk/.colima/default/docker.sock"}),
	)
}

func TestMailboxIntegration_ProviderValidationAndCapabilityHealthPersistence(t *testing.T) {
	ctx := context.Background()
	h := newMailboxIntegrationHarness(t)

	h.providerTransport.validateResult = &mailboxpkg.ValidationResult{
		Code:               mailboxpkg.ValidationCodeOK,
		ProviderIdentifier: "provider-validated-1",
		MailboxIdentifier:  "boss@example.com",
	}

	provider := h.mustCreateProvider(ctx, service.ProviderAccountStatusDraft)
	collector := h.mustCreateCollector(ctx)
	capability := h.mustCreateCapability(ctx, provider.ID, collector.ID, "imap-primary")

	validated, err := h.providerService.ValidateProvider(ctx, provider.ID)
	require.NoError(t, err)
	require.Len(t, h.providerTransport.validateCalls, 1)
	require.Equal(t, "imap", h.providerTransport.validateCalls[0].Protocol)
	require.Equal(t, "imap.example.com", h.providerTransport.validateCalls[0].Host)
	require.Equal(t, 993, h.providerTransport.validateCalls[0].Port)
	require.Equal(t, "boss@example.com", h.providerTransport.validateCalls[0].Username)
	require.Equal(t, "secret", h.providerTransport.validateCalls[0].Password)
	require.Equal(t, mailboxpkg.ValidationCodeOK, validated.Code)
	require.Equal(t, service.ProviderAccountStatusActive, validated.Account.Status)
	require.NotNil(t, validated.Account.LastValidationAt)
	require.NotNil(t, validated.Account.MailboxHint)
	require.Equal(t, "boss@example.com", *validated.Account.MailboxHint)
	require.NotNil(t, validated.Account.ProviderIdentifier)
	require.Equal(t, "provider-validated-1", *validated.Account.ProviderIdentifier)

	persistedProvider := h.mustGetProvider(ctx, provider.ID)
	require.Equal(t, service.ProviderAccountStatusActive, persistedProvider.Status)
	require.NotNil(t, persistedProvider.LastValidationAt)
	require.NotNil(t, persistedProvider.MailboxHint)
	require.Equal(t, "boss@example.com", *persistedProvider.MailboxHint)
	require.NotNil(t, persistedProvider.ProviderIdentifier)
	require.Equal(t, "provider-validated-1", *persistedProvider.ProviderIdentifier)

	h.providerTransport.listIMAPErr = errors.New("capability connectivity failed")
	updatedCapability, err := h.providerService.TestCapability(ctx, capability.ID)
	require.Error(t, err)
	require.NotNil(t, updatedCapability)
	require.Equal(t, service.MailboxCapabilityStateError, updatedCapability.HealthState)
	require.NotNil(t, updatedCapability.LastError)
	require.Contains(t, *updatedCapability.LastError, "capability connectivity failed")

	persistedCapability := h.mustGetCapability(ctx, capability.ID)
	require.Equal(t, service.MailboxCapabilityStateError, persistedCapability.HealthState)
	require.NotNil(t, persistedCapability.LastError)
	require.Contains(t, *persistedCapability.LastError, "capability connectivity failed")

	persistedProvider = h.mustGetProvider(ctx, provider.ID)
	require.Equal(t, service.ProviderAccountStatusActive, persistedProvider.Status)
	require.NotNil(t, persistedProvider.LastValidationAt)
}

func TestMailboxIntegration_RunSyncJobPersistsCursorUpsertAndRecipientResolution(t *testing.T) {
	ctx := context.Background()
	h := newMailboxIntegrationHarness(t)

	provider := h.mustCreateProvider(ctx, service.ProviderAccountStatusActive)
	collector := h.mustCreateCollector(ctx)
	capability := h.mustCreateCapabilityWithFolder(ctx, provider.ID, collector.ID, "imap-primary", "Receipts")
	identity, values := h.mustCreateRecipientIdentity(ctx, "Support", "support@example.com")

	firstReceivedAt := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	secondReceivedAt := firstReceivedAt.Add(3 * time.Minute)
	h.providerTransport.listIMAPResponses = []mailboxIMAPListResponse{{
		Headers: []mailboxpkg.Header{{
			RemoteMessageID:    "msg-1",
			Folder:             "Receipts",
			Sender:             "sender@example.com",
			Recipients:         []string{"Support <support@example.com>"},
			Subject:            "first subject",
			ReceivedAt:         firstReceivedAt,
			Flags:              []string{"seen"},
			Snippet:            "first snippet",
			EnvelopeRecipients: []string{"support@example.com"},
		}},
		NextCursor: map[string]any{"next": "cursor-1"},
	}, {
		Headers: []mailboxpkg.Header{{
			RemoteMessageID:    "msg-1",
			Folder:             "Receipts",
			Sender:             "sender@example.com",
			Recipients:         []string{"Support <support@example.com>", "Other <other@example.com>"},
			Subject:            "updated subject",
			ReceivedAt:         secondReceivedAt,
			Flags:              []string{"seen", "flagged"},
			Snippet:            "updated snippet",
			EnvelopeRecipients: []string{"support@example.com"},
		}},
		NextCursor: map[string]any{"next": "cursor-2"},
	}}

	firstJob := h.mustCreateQueuedSyncJob(ctx, capability.ID)
	firstRun, err := h.syncService.RunSyncJob(ctx, firstJob.ID)
	require.NoError(t, err)
	require.Equal(t, service.MailSyncJobStateSucceeded, firstRun.State)
	require.Len(t, h.providerTransport.listIMAPCalls, 1)
	require.Equal(t, "imap.example.com", h.providerTransport.listIMAPCalls[0].Host)
	require.Equal(t, 993, h.providerTransport.listIMAPCalls[0].Port)
	require.Equal(t, "boss@example.com", h.providerTransport.listIMAPCalls[0].Username)
	require.Equal(t, "secret", h.providerTransport.listIMAPCalls[0].Password)
	require.Equal(t, "Receipts", h.providerTransport.listIMAPCalls[0].Folder)
	require.True(t, h.providerTransport.listIMAPCalls[0].Bounded)
	require.Equal(t, 500, h.providerTransport.listIMAPCalls[0].Limit)
	require.NotNil(t, h.providerTransport.listIMAPCalls[0].Since)

	headers, total := h.mustListHeadersByCapability(ctx, capability.ID)
	require.Equal(t, int64(1), total)
	require.Len(t, headers, 1)
	firstHeaderID := headers[0].ID
	require.Equal(t, service.MailDetailFetchStateSucceeded, headers[0].DetailFetchState)
	require.Equal(t, service.MailResolutionStateResolved, headers[0].ResolutionState)
	require.NotNil(t, headers[0].ResolvedRecipientIdentityID)
	require.Equal(t, identity.ID, *headers[0].ResolvedRecipientIdentityID)
	require.NotNil(t, headers[0].MatchedValueID)
	require.Equal(t, values[0].ID, *headers[0].MatchedValueID)
	require.NotNil(t, headers[0].ResolvedAddress)
	require.Equal(t, "support@example.com", *headers[0].ResolvedAddress)
	require.NotNil(t, headers[0].MatchType)
	require.Equal(t, service.RecipientMatchTypeExactAddress, *headers[0].MatchType)
	require.NotNil(t, headers[0].ResolutionSourceField)
	require.Equal(t, service.MailboxResolutionFieldEnvelope, *headers[0].ResolutionSourceField)

	persistedCapability := h.mustGetCapability(ctx, capability.ID)
	require.Equal(t, service.MailboxCursorState{"next": "cursor-1", "initialized": true}, persistedCapability.CursorState)

	secondJob := h.mustCreateQueuedSyncJob(ctx, capability.ID)
	secondRun, err := h.syncService.RunSyncJob(ctx, secondJob.ID)
	require.NoError(t, err)
	require.Equal(t, service.MailSyncJobStateSucceeded, secondRun.State)
	require.Len(t, h.providerTransport.listIMAPCalls, 2)
	require.Equal(t, "imap.example.com", h.providerTransport.listIMAPCalls[1].Host)
	require.Equal(t, 993, h.providerTransport.listIMAPCalls[1].Port)
	require.Equal(t, "boss@example.com", h.providerTransport.listIMAPCalls[1].Username)
	require.Equal(t, "secret", h.providerTransport.listIMAPCalls[1].Password)
	require.Equal(t, "Receipts", h.providerTransport.listIMAPCalls[1].Folder)
	require.False(t, h.providerTransport.listIMAPCalls[1].Bounded)
	require.Equal(t, 100, h.providerTransport.listIMAPCalls[1].Limit)
	require.Equal(t, map[string]any{"next": "cursor-1", "initialized": true}, h.providerTransport.listIMAPCalls[1].Cursor)

	headers, total = h.mustListHeadersByCapability(ctx, capability.ID)
	require.Equal(t, int64(1), total)
	require.Len(t, headers, 1)
	require.Equal(t, firstHeaderID, headers[0].ID)
	require.Equal(t, "updated subject", headers[0].Subject)
	require.Equal(t, secondReceivedAt, headers[0].ReceivedAt)
	require.Equal(t, []string{"Support <support@example.com>", "Other <other@example.com>"}, headers[0].Recipients)
	require.Equal(t, []string{"seen", "flagged"}, headers[0].Flags)
	require.Equal(t, "updated snippet", headers[0].Snippet)
	require.Equal(t, service.MailDetailFetchStateSucceeded, headers[0].DetailFetchState)
	require.Equal(t, service.MailResolutionStateResolved, headers[0].ResolutionState)
	require.NotNil(t, headers[0].ResolvedRecipientIdentityID)
	require.Equal(t, identity.ID, *headers[0].ResolvedRecipientIdentityID)

	persistedCapability = h.mustGetCapability(ctx, capability.ID)
	require.Equal(t, service.MailboxCursorState{"next": "cursor-2", "initialized": true}, persistedCapability.CursorState)
	require.Equal(t, service.MailboxCapabilityStateHealthy, persistedCapability.HealthState)
}

func TestMailboxIntegration_OnDemandDetailFetchUpdatesDetailFetchState(t *testing.T) {
	ctx := context.Background()
	h := newMailboxIntegrationHarness(t)

	provider := h.mustCreateProvider(ctx, service.ProviderAccountStatusActive)
	collector := h.mustCreateCollector(ctx)
	capability := h.mustCreateCapability(ctx, provider.ID, collector.ID, "imap-primary")
	identity, values := h.mustCreateRecipientIdentity(ctx, "Alias", "alias@example.com")

	headers := h.mustUpsertHeaders(ctx, []*service.MailHeader{{
		CollectorID:      collector.ID,
		CapabilityID:     capability.ID,
		RemoteMessageID:  "detail-msg-1",
		Folder:           "INBOX",
		Recipients:       []string{"Undisclosed recipients:;"},
		Subject:          "detail subject",
		ReceivedAt:       time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC),
		Flags:            []string{"recent"},
		Snippet:          "detail snippet",
		DeliveredTo:      []string{"alias@example.com"},
		ResolutionState:  service.MailResolutionStateUnresolved,
		DetailFetchState: service.MailDetailFetchStateNotRequested,
	}})

	updated, err := h.syncService.FetchDetail(ctx, headers[0].ID)
	require.NoError(t, err)
	require.Equal(t, service.MailDetailFetchStateSucceeded, updated.DetailFetchState)
	require.Equal(t, service.MailResolutionStateResolved, updated.ResolutionState)
	require.NotNil(t, updated.ResolutionSourceField)
	require.Equal(t, service.MailboxResolutionFieldDeliveredTo, *updated.ResolutionSourceField)
	require.NotNil(t, updated.ResolvedRecipientIdentityID)
	require.Equal(t, identity.ID, *updated.ResolvedRecipientIdentityID)
	require.NotNil(t, updated.MatchedValueID)
	require.Equal(t, values[0].ID, *updated.MatchedValueID)
	require.NotNil(t, updated.ResolvedAddress)
	require.Equal(t, "alias@example.com", *updated.ResolvedAddress)

	persisted := h.mustGetHeader(ctx, headers[0].ID)
	require.Equal(t, service.MailDetailFetchStateSucceeded, persisted.DetailFetchState)
	require.Equal(t, service.MailResolutionStateResolved, persisted.ResolutionState)
	require.NotNil(t, persisted.ResolutionSourceField)
	require.Equal(t, service.MailboxResolutionFieldDeliveredTo, *persisted.ResolutionSourceField)
	require.NotNil(t, persisted.ResolvedRecipientIdentityID)
	require.Equal(t, identity.ID, *persisted.ResolvedRecipientIdentityID)
	require.NotNil(t, persisted.MatchedValueID)
	require.Equal(t, values[0].ID, *persisted.MatchedValueID)
	require.NotNil(t, persisted.ResolvedAddress)
	require.Equal(t, "alias@example.com", *persisted.ResolvedAddress)
}

func TestMain(m *testing.M) {
	os.Exit(runMailboxIntegrationMain(m))
}

func runMailboxIntegrationMain(m *testing.M) int {
	ctx := context.Background()
	restoreTestcontainersEnv := configureMailboxTestcontainersEnv()
	defer restoreTestcontainersEnv()

	db, cleanup, skip, err := setupMailboxIntegrationDB(ctx)
	if err != nil {
		log.Printf("failed to initialize mailbox integration db: %v", err)
		return 1
	}
	if skip {
		log.Printf("docker is not available; skipping mailbox integration tests")
		return 0
	}

	integrationMailboxDB = db
	return runWithCleanup(m.Run, cleanup)
}

type mailboxIntegrationHarness struct {
	t                 *testing.T
	repo              service.MailboxRepository
	db                *sql.DB
	providerTransport *mailboxBasicTransportFake
	providerService   *service.MailboxService
	syncService       *service.MailboxSyncService
}

type mailboxIMAPListCall struct {
	Host     string
	Port     int
	Username string
	Password string
	Folder   string
	Limit    int
	Cursor   map[string]any
	Since    *time.Time
	Bounded  bool
}

type mailboxBasicValidateCall struct {
	Protocol string
	Host     string
	Port     int
	Username string
	Password string
}

type mailboxIMAPListResponse struct {
	Headers    []mailboxpkg.Header
	NextCursor map[string]any
	Err        error
}

type mailboxBasicTransportFake struct {
	mu                sync.Mutex
	validateResult    *mailboxpkg.ValidationResult
	validateErr       error
	validateCalls     []mailboxBasicValidateCall
	listIMAPErr       error
	listIMAPResponses []mailboxIMAPListResponse
	listIMAPCalls     []mailboxIMAPListCall
}

func newMailboxIntegrationHarness(t *testing.T) *mailboxIntegrationHarness {
	t.Helper()
	require.NotNil(t, integrationMailboxDB)

	h := &mailboxIntegrationHarness{
		t:                 t,
		db:                integrationMailboxDB,
		repo:              repository.NewMailboxRepository(integrationMailboxDB),
		providerTransport: &mailboxBasicTransportFake{},
	}
	require.NoError(t, h.resetDatabase(context.Background()))

	basicClient := mailboxpkg.NewBasicClientWithTransport(h.providerTransport)
	h.providerService = service.NewMailboxService(h.repo, basicClient, nil, nil)
	h.syncService = service.NewMailboxSyncService(h.repo, service.NewMailboxResolutionService(h.repo), basicClient, nil, nil)
	return h
}

func (h *mailboxIntegrationHarness) resetDatabase(ctx context.Context) error {
	_, err := h.db.ExecContext(ctx, `
		TRUNCATE TABLE
			mailbox_sync_jobs,
			mailbox_header_cache,
			mailbox_recipient_match_values,
			mailbox_recipient_identities,
			mailbox_capabilities,
			mailbox_collectors,
			mailbox_provider_accounts
		RESTART IDENTITY CASCADE
	`)
	return err
}

func (h *mailboxIntegrationHarness) mustCreateProvider(ctx context.Context, status string) *service.ProviderAccount {
	h.t.Helper()
	account, err := h.repo.CreateProviderAccount(ctx, &service.ProviderAccount{
		DisplayName:      "Mailbox Integration Provider",
		ProviderKind:     service.MailboxProviderKindBasic,
		AuthKind:         service.ProviderAuthKindBasic,
		Status:           status,
		EncryptedPayload: `{"protocol":"imap","host":"imap.example.com","port":993,"username":"boss@example.com","password":"secret"}`,
		PayloadVersion:   1,
	})
	require.NoError(h.t, err)
	return account
}

func (h *mailboxIntegrationHarness) mustCreateCollector(ctx context.Context) *service.CollectorMailbox {
	h.t.Helper()
	collector, err := h.repo.CreateCollector(ctx, &service.CollectorMailbox{
		EmailAddress: "collector@example.com",
		DisplayName:  "Mailbox Collector",
		Enabled:      true,
		BusinessTags: []string{"integration"},
	})
	require.NoError(h.t, err)
	return collector
}

func (h *mailboxIntegrationHarness) mustCreateCapability(ctx context.Context, providerID, collectorID int64, kind string) *service.MailboxCapability {
	h.t.Helper()
	return h.mustCreateCapabilityWithFolder(ctx, providerID, collectorID, kind, "INBOX")
}

func (h *mailboxIntegrationHarness) mustCreateCapabilityWithFolder(ctx context.Context, providerID, collectorID int64, kind, folder string) *service.MailboxCapability {
	h.t.Helper()
	capability, err := h.repo.CreateCapability(ctx, &service.MailboxCapability{
		ProviderAccountID:   providerID,
		CollectorID:         collectorID,
		CapabilityKind:      kind,
		ConnectionConfig:    service.MailboxConnectionConfig{"folder": folder},
		CursorState:         service.MailboxCursorState{},
		SyncEnabled:         true,
		SyncIntervalSeconds: 300,
		HealthState:         service.MailboxCapabilityStateHealthy,
	})
	require.NoError(h.t, err)
	return capability
}

func (h *mailboxIntegrationHarness) mustCreateRecipientIdentity(ctx context.Context, name, address string) (*service.RecipientIdentity, []*service.RecipientMatchValue) {
	h.t.Helper()
	identity, err := h.repo.CreateRecipientIdentity(ctx, &service.RecipientIdentity{
		Name:           name,
		NormalizedName: strings.ToLower(strings.TrimSpace(name)),
		Enabled:        true,
	}, []*service.RecipientMatchValue{{
		MatchType:       service.RecipientMatchTypeExactAddress,
		MatchValue:      address,
		NormalizedValue: strings.ToLower(strings.TrimSpace(address)),
		Active:          true,
		Priority:        100,
		SourceKind:      "integration",
		SourceMetadata:  service.RecipientMatchSourceMetadata{"source": "mailbox_integration_test"},
	}})
	require.NoError(h.t, err)
	values, err := h.repo.ListRecipientMatchValues(ctx, identity.ID)
	require.NoError(h.t, err)
	require.Len(h.t, values, 1)
	return identity, values
}

func (h *mailboxIntegrationHarness) mustCreateQueuedSyncJob(ctx context.Context, capabilityID int64) *service.MailSyncJob {
	h.t.Helper()
	jobs, err := h.repo.CreateSyncJobs(ctx, []*service.MailSyncJob{{
		CapabilityID:  capabilityID,
		State:         service.MailSyncJobStateQueued,
		TriggerSource: service.MailSyncTriggerSourceManualBatch,
		ScheduledFor:  time.Now().UTC(),
	}})
	require.NoError(h.t, err)
	require.Len(h.t, jobs, 1)
	return jobs[0]
}

func (h *mailboxIntegrationHarness) mustUpsertHeaders(ctx context.Context, headers []*service.MailHeader) []*service.MailHeader {
	h.t.Helper()
	persisted, err := h.repo.UpsertSyncHeaders(ctx, headers)
	require.NoError(h.t, err)
	return persisted
}

func (h *mailboxIntegrationHarness) mustGetProvider(ctx context.Context, providerID int64) *service.ProviderAccount {
	h.t.Helper()
	provider, err := h.repo.GetProviderAccountByID(ctx, providerID)
	require.NoError(h.t, err)
	return provider
}

func (h *mailboxIntegrationHarness) mustGetCapability(ctx context.Context, capabilityID int64) *service.MailboxCapability {
	h.t.Helper()
	capability, err := h.repo.GetCapabilityByID(ctx, capabilityID)
	require.NoError(h.t, err)
	return capability
}

func (h *mailboxIntegrationHarness) mustGetHeader(ctx context.Context, headerID int64) *service.MailHeader {
	h.t.Helper()
	header, err := h.repo.GetHeaderByID(ctx, headerID)
	require.NoError(h.t, err)
	return header
}

func (h *mailboxIntegrationHarness) mustListHeadersByCapability(ctx context.Context, capabilityID int64) ([]*service.MailHeader, int64) {
	h.t.Helper()
	headers, total, err := h.repo.ListHeaders(ctx, service.MailHeaderListFilter{CapabilityID: &capabilityID, Limit: 10})
	require.NoError(h.t, err)
	return headers, total
}

func (f *mailboxBasicTransportFake) ValidateBasic(ctx context.Context, req mailboxpkg.BasicValidationRequest) (*mailboxpkg.ValidationResult, error) {
	_ = ctx
	f.mu.Lock()
	defer f.mu.Unlock()
	f.validateCalls = append(f.validateCalls, mailboxBasicValidateCall{
		Protocol: req.Protocol,
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
	})
	if f.validateErr != nil {
		return nil, f.validateErr
	}
	if f.validateResult == nil {
		return &mailboxpkg.ValidationResult{Code: mailboxpkg.ValidationCodeOK}, nil
	}
	clone := *f.validateResult
	return &clone, nil
}

func (f *mailboxBasicTransportFake) ListIMAPHeaders(ctx context.Context, req mailboxpkg.IMAPListRequest) ([]mailboxpkg.Header, map[string]any, error) {
	_ = ctx
	f.mu.Lock()
	defer f.mu.Unlock()

	call := mailboxIMAPListCall{
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
		Folder:   req.Folder,
		Limit:    req.Limit,
		Cursor:   cloneAnyMap(req.Cursor),
		Bounded:  req.Bounded,
	}
	if !req.Since.IsZero() {
		since := req.Since.UTC()
		call.Since = &since
	}
	f.listIMAPCalls = append(f.listIMAPCalls, call)

	if len(f.listIMAPResponses) > 0 {
		resp := f.listIMAPResponses[0]
		f.listIMAPResponses = f.listIMAPResponses[1:]
		return cloneMailboxHeaders(resp.Headers), cloneAnyMap(resp.NextCursor), resp.Err
	}
	if f.listIMAPErr != nil {
		return nil, nil, f.listIMAPErr
	}
	return []mailboxpkg.Header{}, cloneAnyMap(req.Cursor), nil
}

func (f *mailboxBasicTransportFake) ListPOP3Headers(ctx context.Context, req mailboxpkg.POP3ListRequest) ([]mailboxpkg.Header, error) {
	_ = ctx
	_ = req
	return []mailboxpkg.Header{}, nil
}

type mailboxPostgresContainer interface {
	ConnectionString(ctx context.Context, args ...string) (string, error)
	Terminate(ctx context.Context, opts ...testcontainers.TerminateOption) error
}

func runWithCleanup(run func() int, cleanup func()) int {
	defer cleanup()
	return run()
}

func setupMailboxIntegrationDB(ctx context.Context) (*sql.DB, func(), bool, error) {
	pgContainer, restoreDockerHost, err := startMailboxPostgresContainer(ctx)
	if err != nil {
		if restoreDockerHost != nil {
			restoreDockerHost()
		}
		if os.Getenv("CI") == "" && isDockerUnavailableError(err) {
			return nil, func() {}, true, nil
		}
		return nil, func() {}, false, err
	}

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	if err != nil {
		cleanup := func() {
			_ = pgContainer.Terminate(ctx)
			restoreDockerHost()
		}
		cleanup()
		return nil, func() {}, false, fmt.Errorf("get postgres dsn: %w", err)
	}

	db, err := openSQLWithRetry(ctx, dsn, 30*time.Second)
	if err != nil {
		cleanup := func() {
			_ = pgContainer.Terminate(ctx)
			restoreDockerHost()
		}
		cleanup()
		return nil, func() {}, false, fmt.Errorf("open sql db: %w", err)
	}
	if err := repository.ApplyMigrations(ctx, db); err != nil {
		cleanup := func() {
			_ = db.Close()
			_ = pgContainer.Terminate(ctx)
			restoreDockerHost()
		}
		cleanup()
		return nil, func() {}, false, fmt.Errorf("apply db migrations: %w", err)
	}

	cleanup := func() {
		_ = db.Close()
		_ = pgContainer.Terminate(ctx)
		restoreDockerHost()
	}
	return db, cleanup, false, nil
}

func startMailboxPostgresContainer(ctx context.Context) (mailboxPostgresContainer, func(), error) {
	var lastErr error
	for _, host := range append([]string{""}, mailboxDockerHostFallbacks()...) {
		restoreDockerHost := applyOptionalDockerHost(host)
		container, err := runMailboxPostgresContainer(ctx)
		if err == nil {
			return container, restoreDockerHost, nil
		}
		restoreDockerHost()
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("unable to start postgres container")
	}
	return nil, func() {}, lastErr
}

func dockerSocketCandidates() []string {
	home, _ := os.UserHomeDir()
	return []string{
		"/var/run/docker.sock",
		filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "docker.sock"),
		filepath.Join(home, ".docker", "run", "docker.sock"),
		filepath.Join(home, ".docker", "desktop", "docker.sock"),
		filepath.Join(home, ".colima", "default", "docker.sock"),
		filepath.Join("/run/user", strconv.Itoa(os.Getuid()), "docker.sock"),
	}
}

func mailboxDockerHostFallbacks() []string {
	availableSockets := make([]string, 0, len(dockerSocketCandidates()))
	for _, socket := range dockerSocketCandidates() {
		if socket == "" {
			continue
		}
		if _, err := os.Stat(socket); err == nil {
			availableSockets = append(availableSockets, socket)
		}
	}
	return buildDockerHostFallbacks(strings.TrimSpace(os.Getenv("DOCKER_HOST")), availableSockets)
}

func buildDockerHostFallbacks(currentHost string, sockets []string) []string {
	if strings.TrimSpace(currentHost) != "" {
		return nil
	}
	seen := map[string]struct{}{}
	fallbacks := make([]string, 0, len(sockets))
	for _, socket := range sockets {
		socket = strings.TrimSpace(socket)
		if socket == "" {
			continue
		}
		host := "unix://" + socket
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		fallbacks = append(fallbacks, host)
	}
	return fallbacks
}

func applyOptionalDockerHost(host string) func() {
	if strings.TrimSpace(host) == "" {
		return func() {}
	}
	previous, hadPrevious := os.LookupEnv("DOCKER_HOST")
	_ = os.Setenv("DOCKER_HOST", host)
	return func() {
		if hadPrevious {
			_ = os.Setenv("DOCKER_HOST", previous)
			return
		}
		_ = os.Unsetenv("DOCKER_HOST")
	}
}

func configureMailboxTestcontainersEnv() func() {
	previous, hadPrevious := os.LookupEnv("TESTCONTAINERS_RYUK_DISABLED")
	if strings.TrimSpace(previous) == "" {
		_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	}
	return func() {
		if hadPrevious {
			_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", previous)
			return
		}
		_ = os.Unsetenv("TESTCONTAINERS_RYUK_DISABLED")
	}
}

func runMailboxPostgresContainer(ctx context.Context) (_ *tcpostgres.PostgresContainer, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("start postgres container panic: %v", r)
		}
	}()
	return tcpostgres.Run(
		ctx,
		"postgres:18.1-alpine3.23",
		tcpostgres.WithDatabase("sub2api_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
}

func isDockerUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	markers := []string{
		"rootless docker not found",
		"cannot connect to the docker daemon",
		"is the docker daemon running",
		"docker socket",
		"no such file or directory",
		"error during connect",
		"docker host",
	}
	for _, marker := range markers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func openSQLWithRetry(ctx context.Context, dsn string, timeout time.Duration) (*sql.DB, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			lastErr = err
			time.Sleep(250 * time.Millisecond)
			continue
		}
		if err := pingWithTimeout(ctx, db, 2*time.Second); err != nil {
			lastErr = err
			_ = db.Close()
			time.Sleep(250 * time.Millisecond)
			continue
		}
		return db, nil
	}
	return nil, fmt.Errorf("db not ready after %s: %w", timeout, lastErr)
}

func pingWithTimeout(ctx context.Context, db *sql.DB, timeout time.Duration) error {
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return db.PingContext(pingCtx)
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	clone := make(map[string]any, len(in))
	for key, value := range in {
		clone[key] = value
	}
	return clone
}

func cloneMailboxHeaders(in []mailboxpkg.Header) []mailboxpkg.Header {
	if in == nil {
		return nil
	}
	clone := make([]mailboxpkg.Header, 0, len(in))
	for _, header := range in {
		copied := header
		copied.Recipients = append([]string(nil), header.Recipients...)
		copied.Flags = append([]string(nil), header.Flags...)
		copied.EnvelopeRecipients = append([]string(nil), header.EnvelopeRecipients...)
		copied.DeliveredTo = append([]string(nil), header.DeliveredTo...)
		copied.OriginalTo = append([]string(nil), header.OriginalTo...)
		clone = append(clone, copied)
	}
	return clone
}
