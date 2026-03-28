//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

var _ service.MailboxRepository = (*mailboxRepository)(nil)

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
	require.NotNil(t, dueCapabilities[0].NextSyncAt)
	require.True(t, dueCapabilities[0].NextSyncAt.After(now))

	dueCapabilities, err = repo.ClaimDueCapabilities(ctx, now, 10)
	require.NoError(t, err)
	require.Empty(t, dueCapabilities)
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

	accounts, err := repo.ListProviderAccounts(ctx, service.MailboxListOptions{Offset: 0, Limit: 10})
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, account.ID, accounts[0].ID)

	secondAccount := mustCreateMailboxProviderAccount(t, ctx, repo)
	pagedAccounts, err := repo.ListProviderAccounts(ctx, service.MailboxListOptions{Offset: 1, Limit: 1})
	require.NoError(t, err)
	require.Len(t, pagedAccounts, 1)
	require.Equal(t, secondAccount.ID, pagedAccounts[0].ID)

	require.NoError(t, repo.DeleteProviderAccount(ctx, account.ID))
	accounts, err = repo.ListProviderAccounts(ctx, service.MailboxListOptions{Offset: 0, Limit: 10})
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, secondAccount.ID, accounts[0].ID)

	accounts, err = repo.ListProviderAccounts(ctx, service.MailboxListOptions{IncludeDeleted: true, Offset: 0, Limit: 10})
	require.NoError(t, err)
	require.Len(t, accounts, 2)
	require.NotNil(t, accounts[0].DeletedAt)
}

func TestMailboxRepository_ProviderAccountNormalizesEmptyOptionalStringsToNull(t *testing.T) {
	ctx := context.Background()
	repo := newMailboxRepositoryForTest(t)
	empty := ""

	account, err := repo.CreateProviderAccount(ctx, &service.ProviderAccount{
		DisplayName:        "Nullable Provider",
		ProviderKind:       "imap",
		AuthKind:           service.ProviderAuthKindBasic,
		Status:             service.ProviderAccountStatusActive,
		EncryptedPayload:   `{"token":"nullable"}`,
		MailboxHint:        &empty,
		ProviderIdentifier: &empty,
		PayloadVersion:     1,
	})
	require.NoError(t, err)
	require.Nil(t, account.MailboxHint)
	require.Nil(t, account.ProviderIdentifier)

	var mailboxHint sql.NullString
	var providerIdentifier sql.NullString
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT mailbox_hint, provider_identifier
		FROM mailbox_provider_accounts
		WHERE id = $1
	`, account.ID).Scan(&mailboxHint, &providerIdentifier))
	require.False(t, mailboxHint.Valid)
	require.False(t, providerIdentifier.Valid)

	hintValue := "value@example.com"
	identifierValue := "provider-value"
	updated, err := repo.UpdateProviderAccount(ctx, &service.ProviderAccount{
		ID:                 account.ID,
		DisplayName:        account.DisplayName,
		ProviderKind:       account.ProviderKind,
		AuthKind:           account.AuthKind,
		Status:             account.Status,
		EncryptedPayload:   account.EncryptedPayload,
		MailboxHint:        &hintValue,
		ProviderIdentifier: &identifierValue,
		PayloadVersion:     account.PayloadVersion,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.MailboxHint)
	require.NotNil(t, updated.ProviderIdentifier)

	updated, err = repo.UpdateProviderAccount(ctx, &service.ProviderAccount{
		ID:                 account.ID,
		DisplayName:        updated.DisplayName,
		ProviderKind:       updated.ProviderKind,
		AuthKind:           updated.AuthKind,
		Status:             updated.Status,
		EncryptedPayload:   updated.EncryptedPayload,
		MailboxHint:        &empty,
		ProviderIdentifier: &empty,
		PayloadVersion:     updated.PayloadVersion,
	})
	require.NoError(t, err)
	require.Nil(t, updated.MailboxHint)
	require.Nil(t, updated.ProviderIdentifier)

	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT mailbox_hint, provider_identifier
		FROM mailbox_provider_accounts
		WHERE id = $1
	`, account.ID).Scan(&mailboxHint, &providerIdentifier))
	require.False(t, mailboxHint.Valid)
	require.False(t, providerIdentifier.Valid)
}

func TestMailboxRepository_DeletedParentHidesCapabilitiesFromListAndClaim(t *testing.T) {
	ctx := context.Background()
	repo := newMailboxRepositoryForTest(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	providerAccount := mustCreateMailboxProviderAccount(t, ctx, repo)
	collector := mustCreateMailboxCollector(t, ctx, repo)
	providerCapability, err := repo.CreateCapability(ctx, &service.MailboxCapability{
		ProviderAccountID:   providerAccount.ID,
		CollectorID:         collector.ID,
		CapabilityKind:      "imap-provider-delete",
		ConnectionConfig:    service.MailboxConnectionConfig{"host": "imap.example.com"},
		CursorState:         service.MailboxCursorState{},
		SyncEnabled:         true,
		SyncIntervalSeconds: 60,
		NextSyncAt:          ptrTime(now.Add(-1 * time.Minute)),
		HealthState:         service.MailboxCapabilityStateHealthy,
	})
	require.NoError(t, err)

	otherProviderAccount := mustCreateMailboxProviderAccount(t, ctx, repo)
	otherCollector := mustCreateMailboxCollector(t, ctx, repo)
	otherCapability, err := repo.CreateCapability(ctx, &service.MailboxCapability{
		ProviderAccountID:   otherProviderAccount.ID,
		CollectorID:         otherCollector.ID,
		CapabilityKind:      "imap-other",
		ConnectionConfig:    service.MailboxConnectionConfig{"host": "imap.other.example.com"},
		CursorState:         service.MailboxCursorState{},
		SyncEnabled:         true,
		SyncIntervalSeconds: 60,
		NextSyncAt:          ptrTime(now.Add(-1 * time.Minute)),
		HealthState:         service.MailboxCapabilityStateHealthy,
	})
	require.NoError(t, err)

	require.NoError(t, repo.DeleteProviderAccount(ctx, providerAccount.ID))

	capabilities, err := repo.ListCapabilities(ctx, service.MailboxListOptions{Offset: 0, Limit: 10})
	require.NoError(t, err)
	require.Len(t, capabilities, 1)
	require.Equal(t, otherCapability.ID, capabilities[0].ID)

	claimed, err := repo.ClaimDueCapabilities(ctx, now, 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, otherCapability.ID, claimed[0].ID)

	collectorAccount := mustCreateMailboxProviderAccount(t, ctx, repo)
	collectorToDelete := mustCreateMailboxCollector(t, ctx, repo)
	collectorCapability, err := repo.CreateCapability(ctx, &service.MailboxCapability{
		ProviderAccountID:   collectorAccount.ID,
		CollectorID:         collectorToDelete.ID,
		CapabilityKind:      "imap-collector-delete",
		ConnectionConfig:    service.MailboxConnectionConfig{"host": "imap.collector.example.com"},
		CursorState:         service.MailboxCursorState{},
		SyncEnabled:         true,
		SyncIntervalSeconds: 60,
		NextSyncAt:          ptrTime(now.Add(-2 * time.Minute)),
		HealthState:         service.MailboxCapabilityStateHealthy,
	})
	require.NoError(t, err)

	require.NoError(t, repo.DeleteCollector(ctx, collectorToDelete.ID))

	capabilities, err = repo.ListCapabilities(ctx, service.MailboxListOptions{Offset: 0, Limit: 20})
	require.NoError(t, err)
	for _, capability := range capabilities {
		require.NotEqual(t, providerCapability.ID, capability.ID)
		require.NotEqual(t, collectorCapability.ID, capability.ID)
	}

	claimed, err = repo.ClaimDueCapabilities(ctx, now, 20)
	require.NoError(t, err)
	for _, capability := range claimed {
		require.NotEqual(t, providerCapability.ID, capability.ID)
		require.NotEqual(t, collectorCapability.ID, capability.ID)
	}
}

func TestMailboxRepository_CreateOrUpdateCapabilityRejectsDeletedParents(t *testing.T) {
	ctx := context.Background()
	repo := newMailboxRepositoryForTest(t)
	nextSyncAt := time.Now().UTC().Add(5 * time.Minute).Truncate(time.Microsecond)

	deletedProvider := mustCreateMailboxProviderAccount(t, ctx, repo)
	activeCollector := mustCreateMailboxCollector(t, ctx, repo)
	require.NoError(t, repo.DeleteProviderAccount(ctx, deletedProvider.ID))

	_, err := repo.CreateCapability(ctx, &service.MailboxCapability{
		ProviderAccountID:   deletedProvider.ID,
		CollectorID:         activeCollector.ID,
		CapabilityKind:      "imap-deleted-provider",
		ConnectionConfig:    service.MailboxConnectionConfig{"host": "imap.example.com"},
		CursorState:         service.MailboxCursorState{},
		SyncEnabled:         true,
		SyncIntervalSeconds: 300,
		NextSyncAt:          &nextSyncAt,
		HealthState:         service.MailboxCapabilityStateHealthy,
	})
	require.Error(t, err)

	activeProvider := mustCreateMailboxProviderAccount(t, ctx, repo)
	deletedCollector := mustCreateMailboxCollector(t, ctx, repo)
	require.NoError(t, repo.DeleteCollector(ctx, deletedCollector.ID))

	_, err = repo.CreateCapability(ctx, &service.MailboxCapability{
		ProviderAccountID:   activeProvider.ID,
		CollectorID:         deletedCollector.ID,
		CapabilityKind:      "imap-deleted-collector",
		ConnectionConfig:    service.MailboxConnectionConfig{"host": "imap.example.com"},
		CursorState:         service.MailboxCursorState{},
		SyncEnabled:         true,
		SyncIntervalSeconds: 300,
		NextSyncAt:          &nextSyncAt,
		HealthState:         service.MailboxCapabilityStateHealthy,
	})
	require.Error(t, err)

	providerForUpdate := mustCreateMailboxProviderAccount(t, ctx, repo)
	collectorForUpdate := mustCreateMailboxCollector(t, ctx, repo)
	capability, err := repo.CreateCapability(ctx, &service.MailboxCapability{
		ProviderAccountID:   providerForUpdate.ID,
		CollectorID:         collectorForUpdate.ID,
		CapabilityKind:      "imap-update-check",
		ConnectionConfig:    service.MailboxConnectionConfig{"host": "imap.example.com"},
		CursorState:         service.MailboxCursorState{},
		SyncEnabled:         true,
		SyncIntervalSeconds: 300,
		NextSyncAt:          &nextSyncAt,
		HealthState:         service.MailboxCapabilityStateHealthy,
	})
	require.NoError(t, err)

	deletedUpdateProvider := mustCreateMailboxProviderAccount(t, ctx, repo)
	require.NoError(t, repo.DeleteProviderAccount(ctx, deletedUpdateProvider.ID))
	capability.ProviderAccountID = deletedUpdateProvider.ID
	_, err = repo.UpdateCapability(ctx, capability)
	require.Error(t, err)

	capability.ProviderAccountID = providerForUpdate.ID
	deletedUpdateCollector := mustCreateMailboxCollector(t, ctx, repo)
	require.NoError(t, repo.DeleteCollector(ctx, deletedUpdateCollector.ID))
	capability.CollectorID = deletedUpdateCollector.ID
	_, err = repo.UpdateCapability(ctx, capability)
	require.Error(t, err)
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

	secondCollector := mustCreateMailboxCollector(t, ctx, repo)
	collectors, err := repo.ListCollectors(ctx, service.MailboxListOptions{Offset: 1, Limit: 1})
	require.NoError(t, err)
	require.Len(t, collectors, 1)
	require.Equal(t, secondCollector.ID, collectors[0].ID)

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

	otherCapability, err := repo.CreateCapability(ctx, &service.MailboxCapability{
		ProviderAccountID:   account.ID,
		CollectorID:         secondCollector.ID,
		CapabilityKind:      "imap-crud-2",
		ConnectionConfig:    service.MailboxConnectionConfig{"host": "imap2.example.com"},
		CursorState:         service.MailboxCursorState{"cursor": "2"},
		SyncEnabled:         true,
		SyncIntervalSeconds: 180,
		HealthState:         service.MailboxCapabilityStateHealthy,
	})
	require.NoError(t, err)
	capabilities, err := repo.ListCapabilities(ctx, service.MailboxListOptions{Offset: 1, Limit: 1})
	require.NoError(t, err)
	require.Len(t, capabilities, 1)
	require.Equal(t, otherCapability.ID, capabilities[0].ID)

	queuedJobs, err := repo.CreateSyncJobs(ctx, []*service.MailSyncJob{{
		CapabilityID:  capability.ID,
		BatchID:       ptrString("batch-1"),
		State:         service.MailSyncJobStateQueued,
		TriggerSource: service.MailSyncTriggerSourceManual,
		ScheduledFor:  time.Now().UTC(),
	}})
	require.NoError(t, err)
	require.Len(t, queuedJobs, 1)

	batchJobs, err := repo.ListSyncJobsByBatchID(ctx, "batch-1")
	require.NoError(t, err)
	require.Len(t, batchJobs, 1)
	require.Equal(t, queuedJobs[0].ID, batchJobs[0].ID)

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
	capabilities, err = repo.ListCapabilities(ctx, service.MailboxListOptions{Offset: 0, Limit: 10})
	require.NoError(t, err)
	require.Len(t, capabilities, 1)
	require.Equal(t, otherCapability.ID, capabilities[0].ID)

	require.NoError(t, repo.DeleteCollector(ctx, collector.ID))
	collectors, err = repo.ListCollectors(ctx, service.MailboxListOptions{Offset: 0, Limit: 10})
	require.NoError(t, err)
	require.Len(t, collectors, 1)
	require.Equal(t, secondCollector.ID, collectors[0].ID)
}

func TestMailboxRepository_SyncJobsRejectDeletedCapabilityAndHideDeletedParentJobs(t *testing.T) {
	ctx := context.Background()
	repo := newMailboxRepositoryForTest(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	deletedCapability := mustCreateMailboxCapability(t, ctx, repo, mailboxCapabilitySeed{CapabilityKind: "imap-deleted-cap"})
	_, err := repo.CreateSyncJobs(ctx, []*service.MailSyncJob{{
		CapabilityID:  deletedCapability.ID,
		State:         service.MailSyncJobStateQueued,
		TriggerSource: service.MailSyncTriggerSourceManual,
		ScheduledFor:  now,
	}})
	require.NoError(t, err)
	require.NoError(t, repo.DeleteCapability(ctx, deletedCapability.ID))

	activeJobs, err := repo.ListActiveSyncJobs(ctx, nil, 10)
	require.NoError(t, err)
	require.Empty(t, activeJobs)

	_, err = repo.CreateSyncJobs(ctx, []*service.MailSyncJob{{
		CapabilityID:  deletedCapability.ID,
		State:         service.MailSyncJobStateQueued,
		TriggerSource: service.MailSyncTriggerSourceManual,
		ScheduledFor:  now,
	}})
	require.Error(t, err)

	providerAccount := mustCreateMailboxProviderAccount(t, ctx, repo)
	collector := mustCreateMailboxCollector(t, ctx, repo)
	parentDeletedCapability, err := repo.CreateCapability(ctx, &service.MailboxCapability{
		ProviderAccountID:   providerAccount.ID,
		CollectorID:         collector.ID,
		CapabilityKind:      "imap-parent-delete",
		ConnectionConfig:    service.MailboxConnectionConfig{"host": "imap.example.com"},
		CursorState:         service.MailboxCursorState{},
		SyncEnabled:         true,
		SyncIntervalSeconds: 120,
		HealthState:         service.MailboxCapabilityStateHealthy,
	})
	require.NoError(t, err)

	activeCapability := mustCreateMailboxCapability(t, ctx, repo, mailboxCapabilitySeed{CapabilityKind: "imap-active-sync-job"})
	_, err = repo.CreateSyncJobs(ctx, []*service.MailSyncJob{{
		CapabilityID:  parentDeletedCapability.ID,
		State:         service.MailSyncJobStateQueued,
		TriggerSource: service.MailSyncTriggerSourceManual,
		ScheduledFor:  now,
	}, {
		CapabilityID:  activeCapability.ID,
		State:         service.MailSyncJobStateQueued,
		TriggerSource: service.MailSyncTriggerSourceManual,
		ScheduledFor:  now.Add(1 * time.Second),
	}})
	require.NoError(t, err)

	require.NoError(t, repo.DeleteProviderAccount(ctx, providerAccount.ID))

	activeJobs, err = repo.ListActiveSyncJobs(ctx, nil, 10)
	require.NoError(t, err)
	require.Len(t, activeJobs, 1)
	require.Equal(t, activeCapability.ID, activeJobs[0].CapabilityID)

	_, err = repo.CreateSyncJobs(ctx, []*service.MailSyncJob{{
		CapabilityID:  parentDeletedCapability.ID,
		State:         service.MailSyncJobStateQueued,
		TriggerSource: service.MailSyncTriggerSourceManual,
		ScheduledFor:  now.Add(2 * time.Second),
	}})
	require.Error(t, err)
}

func TestMailboxRepository_RecipientIdentityCRUDAndReplaceMatchValues(t *testing.T) {
	ctx := context.Background()
	repo := newMailboxRepositoryForTest(t)

	identity, err := repo.CreateRecipientIdentity(ctx, &service.RecipientIdentity{
		Name:           "Team Inbox",
		NormalizedName: "team inbox",
		Enabled:        true,
	}, []*service.RecipientMatchValue{{
		MatchType:       service.RecipientMatchTypeExactAddress,
		MatchValue:      "team@example.com",
		NormalizedValue: "team@example.com",
		Active:          true,
		Priority:        100,
		SourceKind:      "manual",
	}})
	require.NoError(t, err)

	gotIdentity, err := repo.GetRecipientIdentityByID(ctx, identity.ID)
	require.NoError(t, err)
	require.Equal(t, identity.ID, gotIdentity.ID)

	matchValues, err := repo.ListRecipientMatchValues(ctx, identity.ID)
	require.NoError(t, err)
	require.Len(t, matchValues, 1)
	require.Equal(t, "team@example.com", matchValues[0].NormalizedValue)

	identity.Name = "Team Inbox Updated"
	identity.Enabled = false
	updatedIdentity, err := repo.UpdateRecipientIdentity(ctx, identity)
	require.NoError(t, err)
	require.Equal(t, "Team Inbox Updated", updatedIdentity.Name)
	require.False(t, updatedIdentity.Enabled)

	replacedValues, err := repo.ReplaceRecipientMatchValues(ctx, identity.ID, []*service.RecipientMatchValue{{
		MatchType:       service.RecipientMatchTypeDomainSuffix,
		MatchValue:      "example.org",
		NormalizedValue: "example.org",
		Active:          true,
		Priority:        50,
		SourceKind:      "manual",
	}})
	require.NoError(t, err)
	require.Len(t, replacedValues, 1)
	require.Equal(t, service.RecipientMatchTypeDomainSuffix, replacedValues[0].MatchType)

	matchValues, err = repo.ListRecipientMatchValues(ctx, identity.ID)
	require.NoError(t, err)
	require.Len(t, matchValues, 1)
	require.Equal(t, service.RecipientMatchTypeDomainSuffix, matchValues[0].MatchType)

	secondIdentity, err := repo.CreateRecipientIdentity(ctx, &service.RecipientIdentity{
		Name:           "Fallback Inbox",
		NormalizedName: "fallback inbox",
		Enabled:        true,
	}, nil)
	require.NoError(t, err)

	identities, err := repo.ListRecipientIdentities(ctx, service.MailboxListOptions{Offset: 1, Limit: 1})
	require.NoError(t, err)
	require.Len(t, identities, 1)
	require.Equal(t, secondIdentity.ID, identities[0].ID)

	require.NoError(t, repo.DeleteRecipientIdentity(ctx, identity.ID))
	identities, err = repo.ListRecipientIdentities(ctx, service.MailboxListOptions{Offset: 0, Limit: 10})
	require.NoError(t, err)
	require.Len(t, identities, 1)
	require.Equal(t, secondIdentity.ID, identities[0].ID)

	identities, err = repo.ListRecipientIdentities(ctx, service.MailboxListOptions{IncludeDeleted: true, Offset: 0, Limit: 10})
	require.NoError(t, err)
	require.Len(t, identities, 2)
	require.NotNil(t, identities[0].DeletedAt)
}

func TestMailboxRepository_ReplaceRecipientMatchValuesRejectsDeletedIdentity(t *testing.T) {
	ctx := context.Background()
	repo := newMailboxRepositoryForTest(t)

	identity, err := repo.CreateRecipientIdentity(ctx, &service.RecipientIdentity{
		Name:           "Deleted Identity",
		NormalizedName: "deleted identity",
		Enabled:        true,
	}, nil)
	require.NoError(t, err)
	require.NoError(t, repo.DeleteRecipientIdentity(ctx, identity.ID))

	_, err = repo.ReplaceRecipientMatchValues(ctx, identity.ID, []*service.RecipientMatchValue{{
		MatchType:       service.RecipientMatchTypeExactAddress,
		MatchValue:      "deleted@example.com",
		NormalizedValue: "deleted@example.com",
		Active:          true,
		Priority:        10,
		SourceKind:      "manual",
	}})
	require.Error(t, err)

	matchValues, err := repo.ListRecipientMatchValues(ctx, identity.ID)
	require.NoError(t, err)
	require.Empty(t, matchValues)
}

func TestMailboxRepository_GetHeaderByIDReturnsHydratedDetail(t *testing.T) {
	ctx := context.Background()
	repo := newMailboxRepositoryForTest(t)
	capability := mustCreateMailboxCapability(t, ctx, repo, mailboxCapabilitySeed{CapabilityKind: "imap-detail"})
	now := time.Now().UTC().Truncate(time.Microsecond)

	headerID := mustInsertMailboxHeader(t, ctx, capability.CollectorID, capability.ID, mailboxHeaderSeed{
		RemoteMessageID: "detail-1",
		Folder:          "INBOX",
		Subject:         "detail subject",
		ReceivedAt:      now,
		Recipients:      []string{"detail@example.com"},
		Flags:           []string{"seen", "answered"},
	})

	header, err := repo.GetHeaderByID(ctx, headerID)
	require.NoError(t, err)
	require.Equal(t, headerID, header.ID)
	require.Equal(t, []string{"detail@example.com"}, header.Recipients)
	require.Equal(t, []string{"seen", "answered"}, header.Flags)
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

func mustInsertMailboxHeader(t *testing.T, ctx context.Context, collectorID, capabilityID int64, seed mailboxHeaderSeed) int64 {
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

	var id int64
	err = integrationDB.QueryRowContext(ctx, `
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
		RETURNING id
	`, collectorID, capabilityID, seed.RemoteMessageID, seed.Folder, string(recipientsJSON), seed.Subject, seed.ReceivedAt, string(flagsJSON), seed.Snippet, string(emptyJSON), string(emptyJSON), string(emptyJSON)).Scan(&id)
	require.NoError(t, err)
	return id
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
