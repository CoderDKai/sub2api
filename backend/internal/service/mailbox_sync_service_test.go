package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	mailboxpkg "github.com/Wei-Shaw/sub2api/internal/pkg/mailbox"
	"github.com/stretchr/testify/require"
)

func TestMailboxSyncService_CreateBatchSyncJobsCreatesOneJobPerCapabilityWithSharedBatchID(t *testing.T) {
	repo := newMailboxSyncRepositoryStub()
	repo.capabilities[11] = &MailboxCapability{ID: 11, ProviderAccountID: 1, CollectorID: 7, CapabilityKind: "imap-primary", SyncEnabled: true, HealthState: MailboxCapabilityStateHealthy}
	repo.capabilities[12] = &MailboxCapability{ID: 12, ProviderAccountID: 1, CollectorID: 7, CapabilityKind: "microsoft_inbox", SyncEnabled: true, HealthState: MailboxCapabilityStateHealthy}

	svc := newMailboxSyncServiceForTest(repo, &syncResolverStub{})
	jobs, err := svc.CreateBatchSyncJobs(context.Background(), MailboxBatchSyncRequest{
		CapabilityIDs: []int64{11, 12},
	})
	require.NoError(t, err)
	require.Len(t, jobs, 2)
	require.NotNil(t, jobs[0].BatchID)
	require.Equal(t, jobs[0].BatchID, jobs[1].BatchID)
	require.Equal(t, MailSyncTriggerSourceManualBatch, jobs[0].TriggerSource)
	require.Equal(t, MailSyncJobStateQueued, jobs[0].State)
	require.Equal(t, int64(11), jobs[0].CapabilityID)
	require.Equal(t, int64(12), jobs[1].CapabilityID)
}

func TestMailboxSyncService_CreateBatchSyncJobsRejectsConcurrentCapabilitySync(t *testing.T) {
	repo := newMailboxSyncRepositoryStub()
	repo.capabilities[11] = &MailboxCapability{ID: 11, ProviderAccountID: 1, CollectorID: 7, CapabilityKind: "imap-primary", SyncEnabled: true, HealthState: MailboxCapabilityStateHealthy}
	repo.jobs[41] = &MailSyncJob{ID: 41, CapabilityID: 11, State: MailSyncJobStateRunning, TriggerSource: MailSyncTriggerSourceManual}

	svc := newMailboxSyncServiceForTest(repo, &syncResolverStub{})
	_, err := svc.CreateBatchSyncJobs(context.Background(), MailboxBatchSyncRequest{CapabilityIDs: []int64{11}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already active")
}

func TestMailboxSyncService_CreateBatchSyncJobsRejectsInProcessConcurrentCapabilitySync(t *testing.T) {
	repo := newMailboxSyncRepositoryStub()
	repo.capabilities[11] = &MailboxCapability{ID: 11, ProviderAccountID: 1, CollectorID: 7, CapabilityKind: "imap-primary", SyncEnabled: true, HealthState: MailboxCapabilityStateHealthy}
	repo.createSyncJobsStarted = make(chan struct{}, 1)
	repo.createSyncJobsRelease = make(chan struct{})

	svc := newMailboxSyncServiceForTest(repo, &syncResolverStub{})
	errCh := make(chan error, 1)
	go func() {
		_, err := svc.CreateBatchSyncJobs(context.Background(), MailboxBatchSyncRequest{CapabilityIDs: []int64{11}})
		errCh <- err
	}()

	select {
	case <-repo.createSyncJobsStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first create_sync_jobs call")
	}

	_, err := svc.CreateBatchSyncJobs(context.Background(), MailboxBatchSyncRequest{CapabilityIDs: []int64{11}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already in progress")

	close(repo.createSyncJobsRelease)
	require.NoError(t, <-errCh)
}

func TestMailboxSyncService_CreateBatchSyncJobsExpandsCollectorIDsToInboundCapabilities(t *testing.T) {
	repo := newMailboxSyncRepositoryStub()
	repo.capabilities[11] = &MailboxCapability{ID: 11, ProviderAccountID: 1, CollectorID: 9, CapabilityKind: "imap-primary", SyncEnabled: true, HealthState: MailboxCapabilityStateHealthy}
	repo.capabilities[12] = &MailboxCapability{ID: 12, ProviderAccountID: 1, CollectorID: 9, CapabilityKind: "smtp-outbound", SyncEnabled: true, HealthState: MailboxCapabilityStateHealthy}
	repo.capabilities[13] = &MailboxCapability{ID: 13, ProviderAccountID: 1, CollectorID: 9, CapabilityKind: "microsoft_inbox", SyncEnabled: true, HealthState: MailboxCapabilityStateHealthy}
	repo.capabilities[14] = &MailboxCapability{ID: 14, ProviderAccountID: 1, CollectorID: 10, CapabilityKind: "imap-other", SyncEnabled: true, HealthState: MailboxCapabilityStateHealthy}

	svc := newMailboxSyncServiceForTest(repo, &syncResolverStub{})
	jobs, err := svc.CreateBatchSyncJobs(context.Background(), MailboxBatchSyncRequest{CollectorIDs: []int64{9}})
	require.NoError(t, err)
	require.Len(t, jobs, 2)
	require.Equal(t, []int64{11, 13}, []int64{jobs[0].CapabilityID, jobs[1].CapabilityID})
}

func TestMailboxSyncService_BuildListHeadersRequestUsesBoundedBackfillOnFirstSync(t *testing.T) {
	fixedNow := time.Date(2026, 3, 30, 8, 0, 0, 0, time.UTC)
	repo := newMailboxSyncRepositoryStub()
	repo.providers[3] = &ProviderAccount{ID: 3, ProviderKind: MailboxProviderKindBasic, AuthKind: ProviderAuthKindBasic, EncryptedPayload: `{"protocol":"imap","host":"imap.example.com","username":"boss@example.com","password":"secret"}`}
	capability := &MailboxCapability{ID: 11, ProviderAccountID: 3, CollectorID: 7, CapabilityKind: "imap-primary", ConnectionConfig: MailboxConnectionConfig{"folder": "INBOX"}, SyncEnabled: true, HealthState: MailboxCapabilityStateHealthy}

	svc := newMailboxSyncServiceForTest(repo, &syncResolverStub{})
	svc.now = func() time.Time { return fixedNow }

	req, err := svc.BuildListHeadersRequest(context.Background(), capability, 2000)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Equal(t, 500, req.Limit)
	require.NotNil(t, req.InitialBackfillSince)
	require.Equal(t, fixedNow.Add(-30*24*time.Hour), req.InitialBackfillSince.UTC())
	require.Equal(t, 500, req.InitialBackfillPerDirection)
	require.Equal(t, "INBOX", req.CapabilityProfile.ConnectionConfig["folder"])
}

func TestMailboxSyncService_FetchDetailFailureOnlyMarksDetailFetchStateFailed(t *testing.T) {
	repo := newMailboxSyncRepositoryStub()
	header := &MailHeader{ID: 71, CollectorID: 7, CapabilityID: 11, RemoteMessageID: "msg-1", Folder: "INBOX", ResolutionState: MailResolutionStateResolved, DetailFetchState: MailDetailFetchStateNotRequested}
	repo.headers[header.ID] = cloneMailHeader(header)

	svc := newMailboxSyncServiceForTest(repo, &syncResolverStub{err: errors.New("resolver boom")})
	updated, err := svc.FetchDetail(context.Background(), header.ID)
	require.Error(t, err)
	require.NotNil(t, updated)
	require.Equal(t, MailDetailFetchStateFailed, updated.DetailFetchState)
	require.Equal(t, MailResolutionStateResolved, updated.ResolutionState)
	require.Equal(t, MailResolutionStateResolved, repo.headers[header.ID].ResolutionState)
}

func TestMailboxSyncService_RunSyncJobPersistsHeadersCursorResolutionAndJobState(t *testing.T) {
	fixedNow := time.Date(2026, 3, 30, 9, 0, 0, 0, time.UTC)
	repo := newMailboxSyncRepositoryStub()
	repo.providers[3] = &ProviderAccount{ID: 3, ProviderKind: MailboxProviderKindBasic, AuthKind: ProviderAuthKindBasic, EncryptedPayload: `{"protocol":"imap","host":"imap.example.com","username":"boss@example.com","password":"secret"}`}
	repo.capabilities[11] = &MailboxCapability{ID: 11, ProviderAccountID: 3, CollectorID: 7, CapabilityKind: "imap-primary", ConnectionConfig: MailboxConnectionConfig{"folder": "INBOX"}, CursorState: MailboxCursorState{}, SyncEnabled: true, HealthState: MailboxCapabilityStateHealthy}
	repo.jobs[91] = &MailSyncJob{ID: 91, CapabilityID: 11, State: MailSyncJobStateQueued, TriggerSource: MailSyncTriggerSourceManualBatch}
	repo.identities[5] = &RecipientIdentity{ID: 5, Name: "Support", NormalizedName: "support", Enabled: true}
	repo.matchValues[5] = []*RecipientMatchValue{{ID: 8, RecipientIdentityID: 5, MatchType: RecipientMatchTypeExactAddress, MatchValue: "support@example.com", NormalizedValue: "support@example.com", Active: true, Priority: 100}}

	provider := &syncProviderClientStub{
		headerPage: &mailboxpkg.HeaderPage{
			Headers: []mailboxpkg.Header{{
				RemoteMessageID:    "msg-1",
				Folder:             "INBOX",
				Recipients:         []string{"Support <support@example.com>"},
				Subject:            "hello",
				ReceivedAt:         fixedNow.Add(-5 * time.Minute),
				EnvelopeRecipients: []string{"support@example.com"},
			}},
			NextCursor: map[string]any{"next": "cursor-2"},
		},
	}
	svc := newMailboxSyncServiceForTest(repo, NewMailboxResolutionService(repo))
	svc.now = func() time.Time { return fixedNow }
	svc.providers = map[string]mailboxpkg.ProviderClient{MailboxProviderKindBasic: provider}

	job, err := svc.RunSyncJob(context.Background(), 91)
	require.NoError(t, err)
	require.NotNil(t, job)
	require.Len(t, provider.listCalls, 1)
	require.Equal(t, 500, provider.listCalls[0].Limit)
	require.NotNil(t, provider.listCalls[0].Capability.InitialBackfillSince)
	require.Equal(t, fixedNow.Add(-30*24*time.Hour), provider.listCalls[0].Capability.InitialBackfillSince.UTC())
	require.Equal(t, 500, provider.listCalls[0].Capability.InitialBackfillPerDirection)
	require.Equal(t, MailSyncJobStateSucceeded, job.State)
	require.NotNil(t, job.StartedAt)
	require.NotNil(t, job.FinishedAt)
	require.Len(t, repo.headers, 1)
	persisted := repo.mustHeaderByRemoteID(t, 11, "INBOX", "msg-1")
	require.Equal(t, MailDetailFetchStateSucceeded, persisted.DetailFetchState)
	require.Equal(t, MailResolutionStateResolved, persisted.ResolutionState)
	require.NotNil(t, persisted.ResolvedRecipientIdentityID)
	require.Equal(t, int64(5), *persisted.ResolvedRecipientIdentityID)
	require.Equal(t, MailboxCursorState{"next": "cursor-2", mailboxSyncCursorInitializedKey: true}, repo.capabilities[11].CursorState)
	require.Equal(t, MailboxCapabilityStateHealthy, repo.capabilities[11].HealthState)
}

func TestMailboxSyncService_RunSyncJobMarksCapabilityInitializedWhenNextCursorEmpty(t *testing.T) {
	fixedNow := time.Date(2026, 3, 30, 9, 30, 0, 0, time.UTC)
	repo := newMailboxSyncRepositoryStub()
	repo.providers[3] = &ProviderAccount{ID: 3, ProviderKind: MailboxProviderKindBasic, AuthKind: ProviderAuthKindBasic, EncryptedPayload: `{"protocol":"imap","host":"imap.example.com","username":"boss@example.com","password":"secret"}`}
	repo.capabilities[11] = &MailboxCapability{ID: 11, ProviderAccountID: 3, CollectorID: 7, CapabilityKind: "imap-primary", ConnectionConfig: MailboxConnectionConfig{"folder": "INBOX"}, CursorState: MailboxCursorState{}, SyncEnabled: true, HealthState: MailboxCapabilityStateHealthy}
	repo.jobs[93] = &MailSyncJob{ID: 93, CapabilityID: 11, State: MailSyncJobStateQueued, TriggerSource: MailSyncTriggerSourceManualBatch}

	provider := &syncProviderClientStub{headerPage: &mailboxpkg.HeaderPage{Headers: []mailboxpkg.Header{}, NextCursor: nil}}
	svc := newMailboxSyncServiceForTest(repo, &syncResolverStub{})
	svc.now = func() time.Time { return fixedNow }
	svc.providers = map[string]mailboxpkg.ProviderClient{MailboxProviderKindBasic: provider}

	job, err := svc.RunSyncJob(context.Background(), 93)
	require.NoError(t, err)
	require.NotNil(t, job)
	require.Equal(t, MailboxCursorState{mailboxSyncCursorInitializedKey: true}, repo.capabilities[11].CursorState)

	nextReq, err := svc.BuildListHeadersRequest(context.Background(), repo.capabilities[11], 100)
	require.NoError(t, err)
	require.Nil(t, nextReq.InitialBackfillSince)
	require.Zero(t, nextReq.InitialBackfillPerDirection)
	require.Nil(t, nextReq.CapabilityProfile.InitialBackfillSince)
	require.Zero(t, nextReq.CapabilityProfile.InitialBackfillPerDirection)
	require.Equal(t, 100, nextReq.Limit)
}

func TestMailboxSyncService_RunSyncJobTransientProviderErrorSchedulesRetryWithBackoff(t *testing.T) {
	fixedNow := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	repo := newMailboxSyncRepositoryStub()
	repo.providers[3] = &ProviderAccount{ID: 3, ProviderKind: MailboxProviderKindBasic, AuthKind: ProviderAuthKindBasic, EncryptedPayload: `{"protocol":"imap","host":"imap.example.com","username":"boss@example.com","password":"secret"}`}
	repo.capabilities[11] = &MailboxCapability{ID: 11, ProviderAccountID: 3, CollectorID: 7, CapabilityKind: "imap-primary", ConnectionConfig: MailboxConnectionConfig{"folder": "INBOX"}, SyncEnabled: true, HealthState: MailboxCapabilityStateHealthy}
	repo.jobs[92] = &MailSyncJob{ID: 92, CapabilityID: 11, State: MailSyncJobStateQueued, TriggerSource: MailSyncTriggerSourceManualBatch}

	svc := newMailboxSyncServiceForTest(repo, &syncResolverStub{})
	svc.now = func() time.Time { return fixedNow }
	svc.providers = map[string]mailboxpkg.ProviderClient{
		MailboxProviderKindBasic: &syncProviderClientStub{listErr: errors.New("temporary provider outage")},
	}

	job, err := svc.RunSyncJob(context.Background(), 92)
	require.Error(t, err)
	require.NotNil(t, job)
	require.Equal(t, MailSyncJobStateFailed, job.State)
	require.Equal(t, MailboxCapabilityStateWarning, repo.capabilities[11].HealthState)
	retry := repo.lastCreatedJob()
	require.NotNil(t, retry)
	require.Equal(t, MailSyncTriggerSourceRetry, retry.TriggerSource)
	require.True(t, retry.Retryable)
	require.Equal(t, 1, retry.RetryCount)
	require.Equal(t, fixedNow.Add(30*time.Second), retry.ScheduledFor)
	require.NotNil(t, retry.NextRetryAt)
	require.Equal(t, fixedNow.Add(30*time.Second), retry.NextRetryAt.UTC())
}

func TestMailboxSyncRunnerService_RunDueCreatesScheduleJobsBeforeExecuting(t *testing.T) {
	fixedNow := time.Date(2026, 3, 30, 11, 0, 0, 0, time.UTC)
	repo := newMailboxSyncRepositoryStub()
	repo.providers[3] = &ProviderAccount{ID: 3, ProviderKind: MailboxProviderKindBasic, AuthKind: ProviderAuthKindBasic, EncryptedPayload: `{"protocol":"imap","host":"imap.example.com","username":"boss@example.com","password":"secret"}`}
	repo.capabilities[11] = &MailboxCapability{ID: 11, ProviderAccountID: 3, CollectorID: 7, CapabilityKind: "imap-primary", ConnectionConfig: MailboxConnectionConfig{"folder": "INBOX"}, SyncEnabled: true, HealthState: MailboxCapabilityStateHealthy}
	repo.claimedCapabilityIDs = []int64{11}

	provider := &syncProviderClientStub{headerPage: &mailboxpkg.HeaderPage{Headers: []mailboxpkg.Header{}}}
	syncSvc := newMailboxSyncServiceForTest(repo, &syncResolverStub{})
	syncSvc.now = func() time.Time { return fixedNow }
	syncSvc.providers = map[string]mailboxpkg.ProviderClient{MailboxProviderKindBasic: provider}
	runner := NewMailboxSyncRunnerService(repo, syncSvc)
	runner.now = func() time.Time { return fixedNow }

	jobs, err := runner.RunDue(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, MailSyncTriggerSourceSchedule, jobs[0].TriggerSource)
	require.Equal(t, []string{"claim_retry_jobs", "claim_due", "create_jobs"}, repo.events[:3])
	require.Contains(t, repo.events, "provider_list")
}

func TestMailboxSyncRunnerService_RunDueExecutesRunnableRetryJobs(t *testing.T) {
	fixedNow := time.Date(2026, 3, 30, 11, 15, 0, 0, time.UTC)
	repo := newMailboxSyncRepositoryStub()
	repo.providers[3] = &ProviderAccount{ID: 3, ProviderKind: MailboxProviderKindBasic, AuthKind: ProviderAuthKindBasic, EncryptedPayload: `{"protocol":"imap","host":"imap.example.com","username":"boss@example.com","password":"secret"}`}
	repo.capabilities[11] = &MailboxCapability{ID: 11, ProviderAccountID: 3, CollectorID: 7, CapabilityKind: "imap-primary", ConnectionConfig: MailboxConnectionConfig{"folder": "INBOX"}, CursorState: MailboxCursorState{mailboxSyncCursorInitializedKey: true}, SyncEnabled: true, HealthState: MailboxCapabilityStateWarning}
	repo.jobs[301] = &MailSyncJob{ID: 301, CapabilityID: 11, State: MailSyncJobStateQueued, TriggerSource: MailSyncTriggerSourceRetry, ScheduledFor: fixedNow.Add(-1 * time.Minute), Retryable: true, RetryCount: 1, NextRetryAt: ptrTime(fixedNow.Add(-30 * time.Second))}

	provider := &syncProviderClientStub{headerPage: &mailboxpkg.HeaderPage{Headers: []mailboxpkg.Header{}}}
	syncSvc := newMailboxSyncServiceForTest(repo, &syncResolverStub{})
	syncSvc.now = func() time.Time { return fixedNow }
	syncSvc.providers = map[string]mailboxpkg.ProviderClient{MailboxProviderKindBasic: provider}
	runner := NewMailboxSyncRunnerService(repo, syncSvc)
	runner.now = func() time.Time { return fixedNow }

	jobs, err := runner.RunDue(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, int64(301), jobs[0].ID)
	require.Equal(t, MailSyncJobStateRunning, repo.claimedRetryJobs[0].State)
	require.Equal(t, MailSyncJobStateSucceeded, repo.jobs[301].State)
	require.Len(t, provider.listCalls, 1)
	require.Equal(t, []string{"claim_retry_jobs", "provider_list"}, repo.events[:2])
}

func TestMailboxSyncRunnerService_RunDueExecutesRetryJobsBeforeScheduleClaims(t *testing.T) {
	fixedNow := time.Date(2026, 3, 30, 11, 20, 0, 0, time.UTC)
	repo := newMailboxSyncRepositoryStub()
	repo.providers[3] = &ProviderAccount{ID: 3, ProviderKind: MailboxProviderKindBasic, AuthKind: ProviderAuthKindBasic, EncryptedPayload: `{"protocol":"imap","host":"imap.example.com","username":"boss@example.com","password":"secret"}`}
	repo.capabilities[11] = &MailboxCapability{ID: 11, ProviderAccountID: 3, CollectorID: 7, CapabilityKind: "imap-primary", ConnectionConfig: MailboxConnectionConfig{"folder": "INBOX"}, CursorState: MailboxCursorState{mailboxSyncCursorInitializedKey: true}, SyncEnabled: true, HealthState: MailboxCapabilityStateWarning, NextSyncAt: ptrTime(fixedNow.Add(-1 * time.Minute))}
	repo.retryJobBlocksClaims = true
	repo.claimedCapabilityIDs = []int64{11}
	repo.jobs[302] = &MailSyncJob{ID: 302, CapabilityID: 11, State: MailSyncJobStateQueued, TriggerSource: MailSyncTriggerSourceRetry, ScheduledFor: fixedNow.Add(-2 * time.Minute), Retryable: true, RetryCount: 1, NextRetryAt: ptrTime(fixedNow.Add(-30 * time.Second))}

	provider := &syncProviderClientStub{headerPage: &mailboxpkg.HeaderPage{Headers: []mailboxpkg.Header{}}}
	syncSvc := newMailboxSyncServiceForTest(repo, &syncResolverStub{})
	syncSvc.now = func() time.Time { return fixedNow }
	syncSvc.providers = map[string]mailboxpkg.ProviderClient{MailboxProviderKindBasic: provider}
	runner := NewMailboxSyncRunnerService(repo, syncSvc)
	runner.now = func() time.Time { return fixedNow }

	jobs, err := runner.RunDue(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, jobs, 2)
	require.Equal(t, MailSyncJobStateSucceeded, repo.jobs[302].State)
	require.Equal(t, MailSyncTriggerSourceSchedule, repo.jobs[jobs[1].ID].TriggerSource)
	require.Len(t, provider.listCalls, 2)
	require.Equal(t, []string{"claim_retry_jobs", "provider_list", "claim_due", "create_jobs", "provider_list"}, repo.events[:5])
}

func TestMailboxSyncRunnerService_RunDueExecutesOnlyClaimedRetryJobs(t *testing.T) {
	fixedNow := time.Date(2026, 3, 30, 11, 25, 0, 0, time.UTC)
	repo := newMailboxSyncRepositoryStub()
	repo.providers[3] = &ProviderAccount{ID: 3, ProviderKind: MailboxProviderKindBasic, AuthKind: ProviderAuthKindBasic, EncryptedPayload: `{"protocol":"imap","host":"imap.example.com","username":"boss@example.com","password":"secret"}`}
	repo.capabilities[11] = &MailboxCapability{ID: 11, ProviderAccountID: 3, CollectorID: 7, CapabilityKind: "imap-primary", ConnectionConfig: MailboxConnectionConfig{"folder": "INBOX"}, CursorState: MailboxCursorState{mailboxSyncCursorInitializedKey: true}, SyncEnabled: true, HealthState: MailboxCapabilityStateWarning}
	repo.jobs[303] = &MailSyncJob{ID: 303, CapabilityID: 11, State: MailSyncJobStateQueued, TriggerSource: MailSyncTriggerSourceRetry, ScheduledFor: fixedNow.Add(-2 * time.Minute), Retryable: true, RetryCount: 1, NextRetryAt: ptrTime(fixedNow.Add(-30 * time.Second))}
	repo.retryClaimLimit = 0

	provider := &syncProviderClientStub{headerPage: &mailboxpkg.HeaderPage{Headers: []mailboxpkg.Header{}}}
	syncSvc := newMailboxSyncServiceForTest(repo, &syncResolverStub{})
	syncSvc.now = func() time.Time { return fixedNow }
	syncSvc.providers = map[string]mailboxpkg.ProviderClient{MailboxProviderKindBasic: provider}
	runner := NewMailboxSyncRunnerService(repo, syncSvc)
	runner.now = func() time.Time { return fixedNow }

	jobs, err := runner.RunDue(context.Background(), 10)
	require.NoError(t, err)
	require.Empty(t, jobs)
	require.Equal(t, MailSyncJobStateQueued, repo.jobs[303].State)
	require.Empty(t, provider.listCalls)
}

func TestMailboxSyncRunnerService_RunDueRequeuesCapabilitiesWhenScheduleJobCreationFails(t *testing.T) {
	fixedNow := time.Date(2026, 3, 30, 11, 30, 0, 0, time.UTC)
	futureSync := fixedNow.Add(5 * time.Minute)
	repo := newMailboxSyncRepositoryStub()
	repo.capabilities[11] = &MailboxCapability{
		ID:                11,
		ProviderAccountID: 3,
		CollectorID:       7,
		CapabilityKind:    "imap-primary",
		ConnectionConfig:  MailboxConnectionConfig{"folder": "INBOX"},
		SyncEnabled:       true,
		HealthState:       MailboxCapabilityStateHealthy,
		NextSyncAt:        &futureSync,
	}
	repo.claimedCapabilityIDs = []int64{11}
	repo.claimAdvanceTo = &futureSync
	repo.trackRequeueUpdates = true
	repo.createSyncErr = errors.New("insert failed")

	syncSvc := newMailboxSyncServiceForTest(repo, &syncResolverStub{})
	syncSvc.now = func() time.Time { return fixedNow }
	runner := NewMailboxSyncRunnerService(repo, syncSvc)
	runner.now = func() time.Time { return fixedNow }

	jobs, err := runner.RunDue(context.Background(), 10)
	require.Error(t, err)
	require.Empty(t, jobs)
	require.NotNil(t, repo.capabilities[11].NextSyncAt)
	require.Equal(t, fixedNow, repo.capabilities[11].NextSyncAt.UTC())
	require.Contains(t, repo.events, "requeue_due")
}

func TestMailboxSyncRunnerService_RunDueContinuesExecutingRemainingJobsAfterFailure(t *testing.T) {
	fixedNow := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	repo := newMailboxSyncRepositoryStub()
	repo.providers[3] = &ProviderAccount{ID: 3, ProviderKind: MailboxProviderKindBasic, AuthKind: ProviderAuthKindBasic, EncryptedPayload: `{"protocol":"imap","host":"imap.example.com","username":"boss@example.com","password":"secret"}`}
	repo.capabilities[11] = &MailboxCapability{ID: 11, ProviderAccountID: 3, CollectorID: 7, CapabilityKind: "imap-primary", ConnectionConfig: MailboxConnectionConfig{"folder": "INBOX"}, SyncEnabled: false, HealthState: MailboxCapabilityStateHealthy}
	repo.capabilities[12] = &MailboxCapability{ID: 12, ProviderAccountID: 3, CollectorID: 8, CapabilityKind: "imap-secondary", ConnectionConfig: MailboxConnectionConfig{"folder": "INBOX"}, SyncEnabled: true, HealthState: MailboxCapabilityStateHealthy}
	repo.claimedCapabilityIDs = []int64{11, 12}

	provider := &syncProviderClientStub{headerPage: &mailboxpkg.HeaderPage{Headers: []mailboxpkg.Header{}}}
	syncSvc := newMailboxSyncServiceForTest(repo, &syncResolverStub{})
	syncSvc.now = func() time.Time { return fixedNow }
	syncSvc.providers = map[string]mailboxpkg.ProviderClient{MailboxProviderKindBasic: provider}
	runner := NewMailboxSyncRunnerService(repo, syncSvc)
	runner.now = func() time.Time { return fixedNow }

	jobs, err := runner.RunDue(context.Background(), 10)
	require.Error(t, err)
	require.Len(t, jobs, 2)
	require.Equal(t, MailSyncJobStateFailed, repo.jobs[jobs[0].ID].State)
	require.Equal(t, MailSyncJobStateSucceeded, repo.jobs[jobs[1].ID].State)
	require.NotEqual(t, MailSyncJobStateQueued, repo.jobs[jobs[1].ID].State)
	require.Len(t, provider.listCalls, 1)
}

type mailboxSyncRepositoryStub struct {
	*mailboxRepositoryStub
	headers               map[int64]*MailHeader
	headerKeys            map[string]int64
	jobs                  map[int64]*MailSyncJob
	nextHeaderID          int64
	nextJobID             int64
	claimedCapabilityIDs  []int64
	events                []string
	createdJobs           []*MailSyncJob
	updatedHeaders        []*MailHeader
	updatedJobs           []*MailSyncJob
	createSyncJobsStarted chan struct{}
	createSyncJobsRelease chan struct{}
	createSyncJobsOnce    sync.Once
	claimAdvanceTo        *time.Time
	retryJobBlocksClaims  bool
	trackRequeueUpdates   bool
	claimedRetryJobs      []*MailSyncJob
	retryClaimLimit       int
	jobUpdateErr          error
	createSyncErr         error
	claimErr              error
	upsertHeaderErr       error
	updateHeaderErr       error
}

func newMailboxSyncRepositoryStub() *mailboxSyncRepositoryStub {
	return &mailboxSyncRepositoryStub{
		mailboxRepositoryStub: newMailboxRepositoryStub(),
		headers:               make(map[int64]*MailHeader),
		headerKeys:            make(map[string]int64),
		jobs:                  make(map[int64]*MailSyncJob),
		retryClaimLimit:       -1,
	}
}

func (r *mailboxSyncRepositoryStub) GetHeaderByID(ctx context.Context, id int64) (*MailHeader, error) {
	header, ok := r.headers[id]
	if !ok {
		return nil, errors.New("header not found")
	}
	return cloneMailHeader(header), nil

}

func (r *mailboxSyncRepositoryStub) CreateSyncJobs(ctx context.Context, jobs []*MailSyncJob) ([]*MailSyncJob, error) {
	if r.createSyncErr != nil {
		return nil, r.createSyncErr
	}
	if r.createSyncJobsStarted != nil {
		r.createSyncJobsOnce.Do(func() {
			r.createSyncJobsStarted <- struct{}{}
		})
	}
	if r.createSyncJobsRelease != nil {
		<-r.createSyncJobsRelease
	}
	r.events = append(r.events, "create_jobs")
	created := make([]*MailSyncJob, 0, len(jobs))
	for _, job := range jobs {
		if job == nil {
			continue
		}
		r.nextJobID++
		cloned := cloneMailSyncJob(job)
		cloned.ID = r.nextJobID
		if cloned.State == "" {
			cloned.State = MailSyncJobStateQueued
		}
		r.jobs[cloned.ID] = cloned
		r.createdJobs = append(r.createdJobs, cloneMailSyncJob(cloned))
		created = append(created, cloneMailSyncJob(cloned))
	}
	return created, nil
}

func (r *mailboxSyncRepositoryStub) ListSyncJobsByBatchID(ctx context.Context, batchID string) ([]*MailSyncJob, error) {
	items := make([]*MailSyncJob, 0)
	for _, job := range r.jobs {
		if job.BatchID != nil && *job.BatchID == batchID {
			items = append(items, cloneMailSyncJob(job))
		}
	}
	return items, nil
}

func (r *mailboxSyncRepositoryStub) ListActiveSyncJobs(ctx context.Context, capabilityID *int64, limit int) ([]*MailSyncJob, error) {
	items := make([]*MailSyncJob, 0)
	for _, job := range r.jobs {
		if job.State != MailSyncJobStateQueued && job.State != MailSyncJobStateRunning {
			continue
		}
		if capabilityID != nil && job.CapabilityID != *capabilityID {
			continue
		}
		items = append(items, cloneMailSyncJob(job))
	}
	return items, nil
}

func (r *mailboxSyncRepositoryStub) UpdateSyncJobState(ctx context.Context, jobID int64, state string, startedAt, finishedAt, nextRetryAt *time.Time, errorSummary *string) (*MailSyncJob, error) {
	if r.jobUpdateErr != nil {
		return nil, r.jobUpdateErr
	}
	job, ok := r.jobs[jobID]
	if !ok {
		return nil, errors.New("job not found")
	}
	job.State = state
	if startedAt != nil {
		job.StartedAt = cloneTimePtr(startedAt)
	}
	job.FinishedAt = cloneTimePtr(finishedAt)
	job.NextRetryAt = cloneTimePtr(nextRetryAt)
	job.ErrorSummary = cloneStringPtr(errorSummary)
	r.updatedJobs = append(r.updatedJobs, cloneMailSyncJob(job))
	return cloneMailSyncJob(job), nil
}

func (r *mailboxSyncRepositoryStub) ClaimDueCapabilities(ctx context.Context, now time.Time, limit int) ([]*MailboxCapability, error) {
	if r.claimErr != nil {
		return nil, r.claimErr
	}
	r.events = append(r.events, "claim_due")
	items := make([]*MailboxCapability, 0, len(r.claimedCapabilityIDs))
	for _, id := range r.claimedCapabilityIDs {
		if r.retryJobBlocksClaims && r.hasQueuedOrRunningJobForCapability(id) {
			continue
		}
		if r.claimAdvanceTo != nil {
			repoCap := cloneMailboxCapability(r.capabilities[id])
			repoCap.NextSyncAt = cloneTimePtr(r.claimAdvanceTo)
			r.capabilities[id] = repoCap
		}
		items = append(items, cloneMailboxCapability(r.capabilities[id]))
	}
	return items, nil
}

func (r *mailboxSyncRepositoryStub) ClaimRunnableRetrySyncJobs(ctx context.Context, now time.Time, limit int) ([]*MailSyncJob, error) {
	r.events = append(r.events, "claim_retry_jobs")
	items := make([]*MailSyncJob, 0)
	for _, job := range r.jobs {
		if job == nil || job.State != MailSyncJobStateQueued || job.TriggerSource != MailSyncTriggerSourceRetry {
			continue
		}
		if job.NextRetryAt == nil || job.NextRetryAt.After(now) {
			continue
		}
		items = append(items, cloneMailSyncJob(job))
	}
	claimLimit := limit
	if r.retryClaimLimit >= 0 {
		claimLimit = r.retryClaimLimit
	}
	if claimLimit == 0 {
		return []*MailSyncJob{}, nil
	}
	if claimLimit > 0 && len(items) > claimLimit {
		items = items[:claimLimit]
	}
	claimed := make([]*MailSyncJob, 0, len(items))
	for _, job := range items {
		stored := r.jobs[job.ID]
		stored.State = MailSyncJobStateRunning
		startedAt := now
		stored.StartedAt = &startedAt
		claimedJob := cloneMailSyncJob(stored)
		r.claimedRetryJobs = append(r.claimedRetryJobs, claimedJob)
		claimed = append(claimed, claimedJob)
	}
	return claimed, nil
}

func (r *mailboxSyncRepositoryStub) UpdateCapability(ctx context.Context, capability *MailboxCapability) (*MailboxCapability, error) {
	updated, err := r.mailboxRepositoryStub.UpdateCapability(ctx, capability)
	if err != nil {
		return nil, err
	}
	if r.trackRequeueUpdates {
		r.events = append(r.events, "requeue_due")
	}
	return updated, nil
}

func (r *mailboxSyncRepositoryStub) UpsertSyncHeaders(ctx context.Context, headers []*MailHeader) ([]*MailHeader, error) {
	if r.upsertHeaderErr != nil {
		return nil, r.upsertHeaderErr
	}
	stored := make([]*MailHeader, 0, len(headers))
	for _, header := range headers {
		if header == nil {
			continue
		}
		key := syncHeaderKey(header.CapabilityID, header.Folder, header.RemoteMessageID)
		id, ok := r.headerKeys[key]
		if !ok {
			r.nextHeaderID++
			id = r.nextHeaderID
			r.headerKeys[key] = id
		}
		cloned := cloneMailHeader(header)
		cloned.ID = id
		r.headers[id] = cloned
		stored = append(stored, cloneMailHeader(cloned))
	}
	return stored, nil
}

func (r *mailboxSyncRepositoryStub) UpdateHeaderDetail(ctx context.Context, header *MailHeader) (*MailHeader, error) {
	if r.updateHeaderErr != nil {
		return nil, r.updateHeaderErr
	}
	cloned := cloneMailHeader(header)
	r.headers[cloned.ID] = cloned
	r.updatedHeaders = append(r.updatedHeaders, cloneMailHeader(cloned))
	return cloneMailHeader(cloned), nil
}

func (r *mailboxSyncRepositoryStub) lastCreatedJob() *MailSyncJob {
	if len(r.createdJobs) == 0 {
		return nil
	}
	return cloneMailSyncJob(r.createdJobs[len(r.createdJobs)-1])
}

func (r *mailboxSyncRepositoryStub) hasQueuedOrRunningJobForCapability(capabilityID int64) bool {
	for _, job := range r.jobs {
		if job == nil || job.CapabilityID != capabilityID {
			continue
		}
		if job.State == MailSyncJobStateQueued || job.State == MailSyncJobStateRunning {
			return true
		}
	}
	return false
}

func (r *mailboxSyncRepositoryStub) mustHeaderByRemoteID(t *testing.T, capabilityID int64, folder, remoteMessageID string) *MailHeader {
	t.Helper()
	id, ok := r.headerKeys[syncHeaderKey(capabilityID, folder, remoteMessageID)]
	require.True(t, ok)
	return cloneMailHeader(r.headers[id])
}

type syncProviderClientStub struct {
	headerPage *mailboxpkg.HeaderPage
	listErr    error
	listCalls  []syncListCall
}

type syncListCall struct {
	Profile    mailboxpkg.ProviderProfile
	Capability mailboxpkg.CapabilityProfile
	Limit      int
}

func (s *syncProviderClientStub) Validate(ctx context.Context, profile mailboxpkg.ProviderProfile) (*mailboxpkg.ValidationResult, error) {
	return &mailboxpkg.ValidationResult{Code: mailboxpkg.ValidationCodeOK}, nil
}

func (s *syncProviderClientStub) ListHeaders(ctx context.Context, profile mailboxpkg.ProviderProfile, capability mailboxpkg.CapabilityProfile, limit int) (*mailboxpkg.HeaderPage, error) {
	s.listCalls = append(s.listCalls, syncListCall{Profile: profile, Capability: capability, Limit: limit})
	if repo, ok := ctx.Value(syncRepoEventsKey{}).(*mailboxSyncRepositoryStub); ok {
		repo.events = append(repo.events, "provider_list")
	}
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.headerPage != nil {
		return s.headerPage, nil
	}
	return &mailboxpkg.HeaderPage{}, nil
}

type syncResolverStub struct {
	result *MailboxRecipientResolutionResult
	err    error
}

func (s *syncResolverStub) Resolve(ctx context.Context, input MailboxRecipientResolutionInput) (*MailboxRecipientResolutionResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &MailboxRecipientResolutionResult{State: MailResolutionStateUnresolved}, nil
}

type syncRepoEventsKey struct{}

func newMailboxSyncServiceForTest(repo *mailboxSyncRepositoryStub, resolver mailboxRecipientResolver) *MailboxSyncService {
	if resolver == nil {
		resolver = &syncResolverStub{}
	}
	svc := NewMailboxSyncService(repo, resolver, nil, nil, &mailboxAuditStub{})
	svc.now = func() time.Time { return time.Now().UTC() }
	svc.contextDecorator = func(ctx context.Context) context.Context {
		return context.WithValue(ctx, syncRepoEventsKey{}, repo)
	}
	return svc
}

func cloneMailHeader(in *MailHeader) *MailHeader {
	if in == nil {
		return nil
	}
	clone := *in
	clone.Sender = cloneStringPtr(in.Sender)
	clone.Recipients = append([]string(nil), in.Recipients...)
	clone.Flags = append([]string(nil), in.Flags...)
	clone.EnvelopeRecipients = append([]string(nil), in.EnvelopeRecipients...)
	clone.DeliveredTo = append([]string(nil), in.DeliveredTo...)
	clone.OriginalTo = append([]string(nil), in.OriginalTo...)
	clone.ResolvedRecipientIdentityID = cloneInt64Ptr(in.ResolvedRecipientIdentityID)
	clone.ResolvedAddress = cloneStringPtr(in.ResolvedAddress)
	clone.MatchType = cloneStringPtr(in.MatchType)
	clone.MatchedValueID = cloneInt64Ptr(in.MatchedValueID)
	clone.ResolutionSourceField = cloneStringPtr(in.ResolutionSourceField)
	return &clone
}

func cloneMailSyncJob(in *MailSyncJob) *MailSyncJob {
	if in == nil {
		return nil
	}
	clone := *in
	clone.BatchID = cloneStringPtr(in.BatchID)
	clone.StartedAt = cloneTimePtr(in.StartedAt)
	clone.FinishedAt = cloneTimePtr(in.FinishedAt)
	clone.NextRetryAt = cloneTimePtr(in.NextRetryAt)
	clone.ErrorSummary = cloneStringPtr(in.ErrorSummary)
	return &clone
}

func cloneInt64Ptr(in *int64) *int64 {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func syncHeaderKey(capabilityID int64, folder, remoteMessageID string) string {
	return fmt.Sprintf("%d:%s:%s", capabilityID, folder, remoteMessageID)
}
