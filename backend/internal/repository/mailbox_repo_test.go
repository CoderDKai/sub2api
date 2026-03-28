//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMailboxRepository_CreateCapabilityAssociatesProviderAccount(t *testing.T) {
	ctx := context.Background()
	repo := newMailboxRepositoryForTest(t)

	account := mustCreateMailboxProviderAccount(t, ctx, repo)
	collector := mustCreateMailboxCollector(t, ctx, repo)
	nextSyncAt := time.Now().UTC().Add(10 * time.Minute).Truncate(time.Microsecond)

	capability, err := repo.CreateCapability(ctx, &service.MailboxCapability{
		ProviderAccountID:   account.ID,
		CollectorID:         collector.ID,
		CapabilityKind:      "imap",
		ConnectionConfig:    service.MailboxConnectionConfig{"host": "imap.example.com", "port": float64(993)},
		CursorState:         service.MailboxCursorState{"uid_validity": "42"},
		SyncEnabled:         true,
		SyncIntervalSeconds: 300,
		NextSyncAt:          &nextSyncAt,
		HealthState:         service.MailboxCapabilityStateHealthy,
	})
	require.NoError(t, err)
	require.NotZero(t, capability.ID)
	require.Equal(t, account.ID, capability.ProviderAccountID)
	require.Equal(t, collector.ID, capability.CollectorID)

	var providerAccountID int64
	var collectorID int64
	var capabilityKind string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT provider_account_id, collector_id, capability_kind
		FROM mailbox_capabilities
		WHERE id = $1
	`, capability.ID).Scan(&providerAccountID, &collectorID, &capabilityKind))
	require.Equal(t, account.ID, providerAccountID)
	require.Equal(t, collector.ID, collectorID)
	require.Equal(t, "imap", capabilityKind)
}

func TestMailboxRepository_CreateRecipientIdentityRejectsDuplicateExactAddress(t *testing.T) {
	ctx := context.Background()
	repo := newMailboxRepositoryForTest(t)

	_, err := repo.CreateRecipientIdentity(ctx, &service.RecipientIdentity{
		Name:           "Alice",
		NormalizedName: "alice",
		Enabled:        true,
	}, []*service.RecipientMatchValue{{
		MatchType:       service.RecipientMatchTypeExactAddress,
		MatchValue:      "Alice@Example.com",
		NormalizedValue: "alice@example.com",
		Active:          true,
		Priority:        100,
		SourceKind:      "manual",
		SourceMetadata:  service.RecipientMatchSourceMetadata{"source": "seed"},
	}})
	require.NoError(t, err)

	_, err = repo.CreateRecipientIdentity(ctx, &service.RecipientIdentity{
		Name:           "Alice Duplicate",
		NormalizedName: "alice duplicate",
		Enabled:        true,
	}, []*service.RecipientMatchValue{{
		MatchType:       service.RecipientMatchTypeExactAddress,
		MatchValue:      "alice@example.com",
		NormalizedValue: "alice@example.com",
		Active:          true,
		Priority:        90,
		SourceKind:      "manual",
		SourceMetadata:  service.RecipientMatchSourceMetadata{"source": "duplicate"},
	}})
	require.Error(t, err)

	require.Equal(t, int64(1), mailboxRowCount(t, ctx, "mailbox_recipient_identities"))
	require.Equal(t, int64(1), mailboxRowCount(t, ctx, "mailbox_recipient_match_values"))
}

func TestMailboxRepository_CreateSyncJobsReturnsIDsAndClaimDueCapabilitiesSkipsActiveJobs(t *testing.T) {
	ctx := context.Background()
	repo := newMailboxRepositoryForTest(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	dueCapability := mustCreateMailboxCapability(t, ctx, repo, mailboxCapabilitySeed{
		CapabilityKind:      "imap-due",
		SyncEnabled:         true,
		SyncIntervalSeconds: 120,
		NextSyncAt:          ptrTime(now.Add(-2 * time.Minute)),
	})
	activeCapability := mustCreateMailboxCapability(t, ctx, repo, mailboxCapabilitySeed{
		CapabilityKind:      "imap-active",
		SyncEnabled:         true,
		SyncIntervalSeconds: 120,
		NextSyncAt:          ptrTime(now.Add(-1 * time.Minute)),
	})
	_ = mustCreateMailboxCapability(t, ctx, repo, mailboxCapabilitySeed{
		CapabilityKind:      "imap-future",
		SyncEnabled:         true,
		SyncIntervalSeconds: 120,
		NextSyncAt:          ptrTime(now.Add(15 * time.Minute)),
	})
	_ = mustCreateMailboxCapability(t, ctx, repo, mailboxCapabilitySeed{
		CapabilityKind:      "imap-disabled",
		SyncEnabled:         false,
		SyncIntervalSeconds: 120,
		NextSyncAt:          ptrTime(now.Add(-3 * time.Minute)),
	})

	jobs, err := repo.CreateSyncJobs(ctx, []*service.MailSyncJob{{
		CapabilityID:  activeCapability.ID,
		State:         service.MailSyncJobStateQueued,
		TriggerSource: service.MailSyncTriggerSourceSchedule,
		ScheduledFor:  now.Add(-30 * time.Second),
		Retryable:     true,
	}})
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NotZero(t, jobs[0].ID)
	require.Equal(t, activeCapability.ID, jobs[0].CapabilityID)

	dueCapabilities, err := repo.ClaimDueCapabilities(ctx, now, 10)
	require.NoError(t, err)
	require.Len(t, dueCapabilities, 1)
	require.Equal(t, dueCapability.ID, dueCapabilities[0].ID)
}

func TestMailboxRepository_ListHeadersFiltersByCollectorAndFolder(t *testing.T) {
	ctx := context.Background()
	repo := newMailboxRepositoryForTest(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	first := mustCreateMailboxCapability(t, ctx, repo, mailboxCapabilitySeed{CapabilityKind: "imap-primary"})
	second := mustCreateMailboxCapability(t, ctx, repo, mailboxCapabilitySeed{CapabilityKind: "imap-secondary"})

	mustInsertMailboxHeader(t, ctx, first.CollectorID, first.ID, mailboxHeaderSeed{
		RemoteMessageID: "msg-1",
		Folder:          "INBOX",
		Subject:         "newest",
		ReceivedAt:      now,
		Recipients:      []string{"team@example.com"},
		Flags:           []string{"seen"},
	})
	mustInsertMailboxHeader(t, ctx, first.CollectorID, first.ID, mailboxHeaderSeed{
		RemoteMessageID: "msg-2",
		Folder:          "Archive",
		Subject:         "archived",
		ReceivedAt:      now.Add(-1 * time.Hour),
	})
	mustInsertMailboxHeader(t, ctx, second.CollectorID, second.ID, mailboxHeaderSeed{
		RemoteMessageID: "msg-3",
		Folder:          "INBOX",
		Subject:         "other collector",
		ReceivedAt:      now.Add(-2 * time.Hour),
	})

	headers, total, err := repo.ListHeaders(ctx, service.MailHeaderListFilter{
		CollectorID: &first.CollectorID,
		Folder:      "INBOX",
		Limit:       10,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, headers, 1)
	require.Equal(t, "msg-1", headers[0].RemoteMessageID)
	require.Equal(t, first.ID, headers[0].CapabilityID)
	require.Equal(t, []string{"team@example.com"}, headers[0].Recipients)
	require.Equal(t, []string{"seen"}, headers[0].Flags)
}

func TestMailboxRepository_ProviderAccountCRUDAndList(t *testing.T) {
	ctx := context.Background()
	repo := newMailboxRepositoryForTest(t)

	account := mustCreateMailboxProviderAccount(t, ctx, repo)

	got, err := repo.GetProviderAccountByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, account.ID, got.ID)

	updatedName := "Updated Provider"
	updatedStatus := service.ProviderAccountStatusInvalid
	updatedPayload := `{"token":"updated"}`
	updatedHint := "updated@example.com"
	updatedIdentifier := "updated-provider"
	updated, err := repo.UpdateProviderAccount(ctx, &service.ProviderAccount{
		ID:                 account.ID,
		DisplayName:        updatedName,
		ProviderKind:       got.ProviderKind,
		AuthKind:           got.AuthKind,
		Status:             updatedStatus,
		EncryptedPayload:   updatedPayload,
		MailboxHint:        &updatedHint,
		ProviderIdentifier: &updatedIdentifier,
		PayloadVersion:     got.PayloadVersion + 1,
	})
	require.NoError(t, err)
	require.Equal(t, updatedName, updated.DisplayName)
	require.Equal(t, updatedStatus, updated.Status)

	accounts, err := repo.ListProviderAccounts(ctx, false, 10)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, account.ID, accounts[0].ID)

	require.NoError(t, repo.DeleteProviderAccount(ctx, account.ID))
	accounts, err = repo.ListProviderAccounts(ctx, false, 10)
	require.NoError(t, err)
	require.Empty(t, accounts)

	accounts, err = repo.ListProviderAccounts(ctx, true, 10)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.NotNil(t, accounts[0].DeletedAt)
}

func TestMailboxRepository_CollectorCapabilityAndSyncJobBaseMethods(t *testing.T) {
	ctx := context.Background()
	repo := newMailboxRepositoryForTest(t)
	account := mustCreateMailboxProviderAccount(t, ctx, repo)
	collector := mustCreateMailboxCollector(t, ctx, repo)

	collector.DisplayName = "Updated Collector"
	collector.BusinessTags = []string{"vip"}
	updatedCollector, err := repo.UpdateCollector(ctx, collector)
	require.NoError(t, err)
	require.Equal(t, "Updated Collector", updatedCollector.DisplayName)
	gotCollector, err := repo.GetCollectorByID(ctx, collector.ID)
	require.NoError(t, err)
	require.Equal(t, collector.ID, gotCollector.ID)

	collectors, err := repo.ListCollectors(ctx, false, 10)
	require.NoError(t, err)
	require.Len(t, collectors, 1)
	require.Equal(t, collector.ID, collectors[0].ID)

	nextSyncAt := time.Now().UTC().Add(5 * time.Minute).Truncate(time.Microsecond)
	capability, err := repo.CreateCapability(ctx, &service.MailboxCapability{
		ProviderAccountID:   account.ID,
		CollectorID:         collector.ID,
		CapabilityKind:      "imap-crud",
		ConnectionConfig:    service.MailboxConnectionConfig{"host": "imap.example.com"},
		CursorState:         service.MailboxCursorState{"cursor": "1"},
		SyncEnabled:         true,
		SyncIntervalSeconds: 180,
		NextSyncAt:          &nextSyncAt,
		HealthState:         service.MailboxCapabilityStateHealthy,
	})
	require.NoError(t, err)

	gotCapability, err := repo.GetCapabilityByID(ctx, capability.ID)
	require.NoError(t, err)
	require.Equal(t, capability.ID, gotCapability.ID)

	capability.HealthState = service.MailboxCapabilityStateWarning
	capability.LastError = ptrString("temporary issue")
	updatedCapability, err := repo.UpdateCapability(ctx, capability)
	require.NoError(t, err)
	require.Equal(t, service.MailboxCapabilityStateWarning, updatedCapability.HealthState)

	capabilities, err := repo.ListCapabilities(ctx, false, 10)
	require.NoError(t, err)
	require.Len(t, capabilities, 1)

	queuedJobs, err := repo.CreateSyncJobs(ctx, []*service.MailSyncJob{{
		CapabilityID:  capability.ID,
		State:         service.MailSyncJobStateQueued,
		TriggerSource: service.MailSyncTriggerSourceManual,
		ScheduledFor:  time.Now().UTC(),
	}})
	require.NoError(t, err)
	require.Len(t, queuedJobs, 1)

	activeJobs, err := repo.ListActiveSyncJobs(ctx, &capability.ID, 10)
	require.NoError(t, err)
	require.Len(t, activeJobs, 1)
	require.Equal(t, queuedJobs[0].ID, activeJobs[0].ID)

	startedAt := time.Now().UTC().Truncate(time.Microsecond)
	finishedAt := startedAt.Add(30 * time.Second)
	updatedJob, err := repo.UpdateSyncJobState(ctx, queuedJobs[0].ID, service.MailSyncJobStateSucceeded, &startedAt, &finishedAt, nil, nil)
	require.NoError(t, err)
	require.Equal(t, service.MailSyncJobStateSucceeded, updatedJob.State)
	require.NotNil(t, updatedJob.FinishedAt)

	activeJobs, err = repo.ListActiveSyncJobs(ctx, &capability.ID, 10)
	require.NoError(t, err)
	require.Empty(t, activeJobs)

	require.NoError(t, repo.DeleteCapability(ctx, capability.ID))
	capabilities, err = repo.ListCapabilities(ctx, false, 10)
	require.NoError(t, err)
	require.Empty(t, capabilities)

	require.NoError(t, repo.DeleteCollector(ctx, collector.ID))
	collectors, err = repo.ListCollectors(ctx, false, 10)
	require.NoError(t, err)
	require.Empty(t, collectors)
}

type mailboxCapabilitySeed struct {
	CapabilityKind      string
	SyncEnabled         bool
	SyncIntervalSeconds int
	NextSyncAt          *time.Time
	HealthState         string
	ConnectionConfig    service.MailboxConnectionConfig
	CursorState         service.MailboxCursorState
}

type mailboxHeaderSeed struct {
	RemoteMessageID string
	Folder          string
	Subject         string
	ReceivedAt      time.Time
	Recipients      []string
	Flags           []string
	Snippet         string
}

func newMailboxRepositoryForTest(t *testing.T) *mailboxRepository {
	t.Helper()
	cleanupMailboxTables(t)
	return &mailboxRepository{db: integrationDB}
}

func cleanupMailboxTables(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	tables := []string{
		"mailbox_header_cache",
		"mailbox_sync_jobs",
		"mailbox_recipient_match_values",
		"mailbox_recipient_identities",
		"mailbox_capabilities",
		"mailbox_collectors",
		"mailbox_provider_accounts",
	}
	for _, table := range tables {
		_, err := integrationDB.ExecContext(ctx, "DELETE FROM "+table)
		require.NoError(t, err, "cleanup %s", table)
	}
}

func mustCreateMailboxProviderAccount(t *testing.T, ctx context.Context, repo service.MailboxRepository) *service.ProviderAccount {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	mailboxHint := "support-" + suffix + "@example.com"
	providerIdentifier := "provider-" + suffix

	account, err := repo.CreateProviderAccount(ctx, &service.ProviderAccount{
		DisplayName:        "Mailbox Account " + suffix,
		ProviderKind:       "imap",
		AuthKind:           service.ProviderAuthKindBasic,
		Status:             service.ProviderAccountStatusActive,
		EncryptedPayload:   `{"token":"encrypted"}`,
		MailboxHint:        &mailboxHint,
		ProviderIdentifier: &providerIdentifier,
		PayloadVersion:     2,
	})
	require.NoError(t, err)
	require.NotZero(t, account.ID)
	return account
}

func mustCreateMailboxCollector(t *testing.T, ctx context.Context, repo service.MailboxRepository) *service.CollectorMailbox {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	collector, err := repo.CreateCollector(ctx, &service.CollectorMailbox{
		EmailAddress: "collector-" + suffix + "@example.com",
		DisplayName:  "Collector " + suffix,
		Enabled:      true,
		BusinessTags: []string{"support", "priority"},
	})
	require.NoError(t, err)
	require.NotZero(t, collector.ID)
	return collector
}

func mustCreateMailboxCapability(t *testing.T, ctx context.Context, repo service.MailboxRepository, seed mailboxCapabilitySeed) *service.MailboxCapability {
	t.Helper()
	account := mustCreateMailboxProviderAccount(t, ctx, repo)
	collector := mustCreateMailboxCollector(t, ctx, repo)

	connectionConfig := seed.ConnectionConfig
	if connectionConfig == nil {
		connectionConfig = service.MailboxConnectionConfig{"host": "imap.example.com"}
	}
	cursorState := seed.CursorState
	if cursorState == nil {
		cursorState = service.MailboxCursorState{}
	}
	healthState := seed.HealthState
	if healthState == "" {
		healthState = service.MailboxCapabilityStateHealthy
	}
	if seed.SyncIntervalSeconds == 0 {
		seed.SyncIntervalSeconds = 300
	}

	capability, err := repo.CreateCapability(ctx, &service.MailboxCapability{
		ProviderAccountID:   account.ID,
		CollectorID:         collector.ID,
		CapabilityKind:      seed.CapabilityKind,
		ConnectionConfig:    connectionConfig,
		CursorState:         cursorState,
		SyncEnabled:         seed.SyncEnabled,
		SyncIntervalSeconds: seed.SyncIntervalSeconds,
		NextSyncAt:          seed.NextSyncAt,
		HealthState:         healthState,
	})
	require.NoError(t, err)
	return capability
}

func mustInsertMailboxHeader(t *testing.T, ctx context.Context, collectorID, capabilityID int64, seed mailboxHeaderSeed) {
	t.Helper()
	recipientsJSON, err := json.Marshal(seed.Recipients)
	require.NoError(t, err)
	flagsJSON, err := json.Marshal(seed.Flags)
	require.NoError(t, err)
	emptyJSON, err := json.Marshal([]string{})
	require.NoError(t, err)
	if seed.Snippet == "" {
		seed.Snippet = "snippet"
	}

	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO mailbox_header_cache (
			collector_id,
			capability_id,
			remote_message_id,
			folder,
			recipients,
			subject,
			received_at,
			flags,
			snippet,
			envelope_recipients,
			delivered_to,
			original_to
		) VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8::jsonb, $9, $10::jsonb, $11::jsonb, $12::jsonb)
	`, collectorID, capabilityID, seed.RemoteMessageID, seed.Folder, string(recipientsJSON), seed.Subject, seed.ReceivedAt, string(flagsJSON), seed.Snippet, string(emptyJSON), string(emptyJSON), string(emptyJSON))
	require.NoError(t, err)
}

func mailboxRowCount(t *testing.T, ctx context.Context, table string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count))
	return count
}

func ptrTime(v time.Time) *time.Time {
	return &v
}

func ptrString(v string) *string {
	return &v
}
