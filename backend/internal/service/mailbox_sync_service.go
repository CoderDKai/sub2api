package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	mailboxpkg "github.com/Wei-Shaw/sub2api/internal/pkg/mailbox"
)

const (
	mailboxSyncDefaultListLimit        = 100
	mailboxSyncInitialBackfillDays     = 30
	mailboxSyncInitialBackfillPerDir   = 500
	mailboxSyncActiveJobScanLimit      = 1000
	mailboxSyncRetryBaseBackoff        = 30 * time.Second
	mailboxSyncRetryMaxBackoff         = 15 * time.Minute
	mailboxSyncDefaultTriggerSource    = MailSyncTriggerSourceManualBatch
	mailboxSyncTransientErrorThreshold = 0
)

var ErrMailboxSyncJobNotFound = errors.New("mailbox sync job not found")

type MailboxBatchSyncRequest struct {
	CapabilityIDs []int64
	CollectorIDs  []int64
	TriggerSource string
	ScheduledFor  *time.Time
}

type MailboxListHeadersRequest struct {
	ProviderProfile             mailboxpkg.ProviderProfile
	CapabilityProfile           mailboxpkg.CapabilityProfile
	Limit                       int
	InitialBackfillSince        *time.Time
	InitialBackfillPerDirection int
}

type mailboxRecipientResolver interface {
	Resolve(ctx context.Context, input MailboxRecipientResolutionInput) (*MailboxRecipientResolutionResult, error)
}

type mailboxSyncHeaderStore interface {
	UpsertSyncHeaders(ctx context.Context, headers []*MailHeader) ([]*MailHeader, error)
	UpdateHeaderDetail(ctx context.Context, header *MailHeader) (*MailHeader, error)
}

type MailboxSyncService struct {
	repo             MailboxRepository
	resolver         mailboxRecipientResolver
	audit            MailboxAuditor
	providers        map[string]mailboxpkg.ProviderClient
	guardMu          sync.Mutex
	inFlightCaps     map[int64]struct{}
	now              func() time.Time
	contextDecorator func(context.Context) context.Context
}

func NewMailboxSyncService(repo MailboxRepository, resolver mailboxRecipientResolver, basicClient *mailboxpkg.BasicClient, microsoftClient *mailboxpkg.MicrosoftClient, audit MailboxAuditor) *MailboxSyncService {
	providers := map[string]mailboxpkg.ProviderClient{}
	if basicClient != nil {
		providers[MailboxProviderKindBasic] = basicClient
	}
	if microsoftClient != nil {
		providers[MailboxProviderKindMicrosoft] = microsoftClient
	}
	if audit == nil {
		audit = NewMailboxAuditLogger()
	}
	return &MailboxSyncService{
		repo:         repo,
		resolver:     resolver,
		audit:        audit,
		providers:    providers,
		inFlightCaps: map[int64]struct{}{},
		now:          time.Now,
		contextDecorator: func(ctx context.Context) context.Context {
			return ctx
		},
	}
}

func (s *MailboxSyncService) CreateBatchSyncJobs(ctx context.Context, input MailboxBatchSyncRequest) ([]*MailSyncJob, error) {
	capabilities, err := s.collectSyncCapabilities(ctx, input)
	if err != nil {
		return nil, err
	}
	if len(capabilities) == 0 {
		return []*MailSyncJob{}, nil
	}
	release, err := s.acquireCapabilityGuards(capabilities)
	if err != nil {
		return nil, err
	}
	defer release()
	activeJobs, err := s.repo.ListActiveSyncJobs(ctx, nil, mailboxSyncActiveJobScanLimit)
	if err != nil {
		return nil, err
	}
	activeByCapability := make(map[int64]*MailSyncJob, len(activeJobs))
	for _, job := range activeJobs {
		if job == nil {
			continue
		}
		activeByCapability[job.CapabilityID] = job
	}
	for _, capability := range capabilities {
		if capability == nil {
			continue
		}
		if _, exists := activeByCapability[capability.ID]; exists {
			return nil, fmt.Errorf("capability %d already active", capability.ID)
		}
	}

	now := s.now().UTC()
	scheduledFor := now
	if input.ScheduledFor != nil {
		scheduledFor = input.ScheduledFor.UTC()
	}
	triggerSource := strings.TrimSpace(input.TriggerSource)
	if triggerSource == "" {
		triggerSource = mailboxSyncDefaultTriggerSource
	}
	batchID := fmt.Sprintf("mail-sync-%d", now.UnixNano())
	jobs := make([]*MailSyncJob, 0, len(capabilities))
	capabilityIDs := make([]int64, 0, len(capabilities))
	for _, capability := range capabilities {
		jobs = append(jobs, &MailSyncJob{
			CapabilityID:  capability.ID,
			BatchID:       stringPtr(batchID),
			State:         MailSyncJobStateQueued,
			TriggerSource: triggerSource,
			ScheduledFor:  scheduledFor,
		})
		capabilityIDs = append(capabilityIDs, capability.ID)
	}
	created, err := s.repo.CreateSyncJobs(ctx, jobs)
	if err != nil {
		return nil, err
	}
	if triggerSource == MailSyncTriggerSourceManual {
		for _, capabilityID := range capabilityIDs {
			s.audit.RecordManualSync(ctx, capabilityID)
		}
	} else {
		s.audit.RecordBatchSync(ctx, batchID, capabilityIDs)
	}
	return created, nil
}

func (s *MailboxSyncService) BuildListHeadersRequest(ctx context.Context, capability *MailboxCapability, requestedLimit int) (*MailboxListHeadersRequest, error) {
	if capability == nil {
		return nil, errors.New("mailbox capability is required")
	}
	account, err := s.repo.GetProviderAccountByID(ctx, capability.ProviderAccountID)
	if err != nil {
		return nil, err
	}
	profile, err := buildProviderProfile(account)
	if err != nil {
		return nil, err
	}
	limit := requestedLimit
	if limit <= 0 {
		limit = mailboxSyncDefaultListLimit
	}
	req := &MailboxListHeadersRequest{
		ProviderProfile: profile,
		CapabilityProfile: mailboxpkg.CapabilityProfile{
			Kind:             capability.CapabilityKind,
			ConnectionConfig: mapFromMailboxConnectionConfig(capability.ConnectionConfig),
			CursorState:      mapFromMailboxCursorState(capability.CursorState),
		},
		Limit: limit,
	}
	if len(capability.CursorState) == 0 {
		since := s.now().UTC().Add(-mailboxSyncInitialBackfillDays * 24 * time.Hour)
		req.InitialBackfillSince = &since
		req.InitialBackfillPerDirection = mailboxSyncInitialBackfillPerDir
		req.CapabilityProfile.InitialBackfillSince = &since
		req.CapabilityProfile.InitialBackfillPerDirection = mailboxSyncInitialBackfillPerDir
		req.Limit = mailboxSyncInitialBackfillPerDir
	}
	return req, nil
}

func (s *MailboxSyncService) RunSyncJob(ctx context.Context, jobID int64) (*MailSyncJob, error) {
	job, err := s.findActiveSyncJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	capability, err := s.repo.GetCapabilityByID(ctx, job.CapabilityID)
	if err != nil {
		return nil, err
	}
	if !capability.SyncEnabled {
		return s.failJob(ctx, job, capability, errors.New("capability sync disabled"))
	}
	if _, err := s.updateCapabilityHealth(ctx, capability, MailboxCapabilityStateSyncing, nil, false); err != nil {
		return nil, err
	}
	startedAt := s.now().UTC()
	job, err = s.repo.UpdateSyncJobState(ctx, job.ID, MailSyncJobStateRunning, &startedAt, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	listReq, err := s.BuildListHeadersRequest(ctx, capability, mailboxSyncDefaultListLimit)
	if err != nil {
		return s.failJob(ctx, job, capability, err)
	}
	client, err := s.providerClient(listReq.ProviderProfile.ProviderKind)
	if err != nil {
		return s.failJob(ctx, job, capability, err)
	}
	page, err := client.ListHeaders(s.decorateContext(ctx), listReq.ProviderProfile, listReq.CapabilityProfile, listReq.Limit)
	if err != nil {
		return s.failJob(ctx, job, capability, err)
	}
	storedHeaders, err := s.persistHeaders(ctx, capability, page)
	if err != nil {
		return s.failJob(ctx, job, capability, err)
	}
	detailFailures := 0
	for _, header := range storedHeaders {
		if header == nil {
			continue
		}
		if _, err := s.FetchDetail(ctx, header.ID); err != nil {
			detailFailures++
		}
	}
	finishedAt := s.now().UTC()
	updatedCapability := cloneMailboxCapabilityValue(capability)
	updatedCapability.CursorState = MailboxCursorState(mapFromMailboxCursorState(pageNextCursor(page)))
	updatedCapability.LastSyncAt = &finishedAt
	updatedCapability.LastError = nil
	updatedCapability.HealthState = MailboxCapabilityStateHealthy
	jobState := MailSyncJobStateSucceeded
	var errorSummary *string
	if detailFailures > mailboxSyncTransientErrorThreshold {
		message := fmt.Sprintf("detail fetch failures: %d", detailFailures)
		updatedCapability.HealthState = MailboxCapabilityStateWarning
		updatedCapability.LastError = stringPtr(message)
		jobState = MailSyncJobStatePartial
		errorSummary = stringPtr(message)
	}
	if _, err := s.repo.UpdateCapability(ctx, updatedCapability); err != nil {
		return nil, err
	}
	job, err = s.repo.UpdateSyncJobState(ctx, job.ID, jobState, nil, &finishedAt, nil, errorSummary)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (s *MailboxSyncService) FetchDetail(ctx context.Context, headerID int64) (*MailHeader, error) {
	store, err := s.headerStore()
	if err != nil {
		return nil, err
	}
	header, err := s.repo.GetHeaderByID(ctx, headerID)
	if err != nil {
		return nil, err
	}
	result, err := s.resolver.Resolve(ctx, MailboxRecipientResolutionInput{
		EnvelopeRecipients: append([]string(nil), header.EnvelopeRecipients...),
		DeliveredTo:        append([]string(nil), header.DeliveredTo...),
		XOriginalTo:        append([]string(nil), header.OriginalTo...),
		To:                 append([]string(nil), header.Recipients...),
	})
	if err != nil {
		updated := cloneMailHeaderValue(header)
		updated.DetailFetchState = MailDetailFetchStateFailed
		persisted, updateErr := store.UpdateHeaderDetail(ctx, updated)
		if updateErr != nil {
			return nil, updateErr
		}
		return persisted, err
	}
	updated := cloneMailHeaderValue(header)
	applyResolutionResult(updated, result)
	updated.DetailFetchState = MailDetailFetchStateSucceeded
	persisted, err := store.UpdateHeaderDetail(ctx, updated)
	if err != nil {
		return nil, err
	}
	s.audit.RecordInboxDetailFetch(ctx, headerID)
	return persisted, nil
}

func (s *MailboxSyncService) failJob(ctx context.Context, job *MailSyncJob, capability *MailboxCapability, cause error) (*MailSyncJob, error) {
	if cause == nil {
		cause = errors.New("mailbox sync failed")
	}
	now := s.now().UTC()
	summary := cause.Error()
	jobState, err := s.repo.UpdateSyncJobState(ctx, job.ID, MailSyncJobStateFailed, nil, &now, nil, &summary)
	if err != nil {
		return nil, err
	}
	retryable := isTransientMailboxSyncError(cause)
	if capability != nil {
		healthState := MailboxCapabilityStateError
		if !capability.SyncEnabled {
			healthState = MailboxCapabilityStatePaused
		} else if retryable {
			healthState = MailboxCapabilityStateWarning
		}
		if _, updateErr := s.updateCapabilityHealth(ctx, capability, healthState, &summary, false); updateErr != nil {
			return nil, updateErr
		}
	}
	if retryable {
		nextRetryAt := now.Add(syncRetryBackoff(job.RetryCount))
		_, createErr := s.repo.CreateSyncJobs(ctx, []*MailSyncJob{{
			CapabilityID:  job.CapabilityID,
			BatchID:       cloneStringPointer(job.BatchID),
			State:         MailSyncJobStateQueued,
			TriggerSource: MailSyncTriggerSourceRetry,
			ScheduledFor:  nextRetryAt,
			Retryable:     true,
			RetryCount:    job.RetryCount + 1,
			NextRetryAt:   &nextRetryAt,
			ErrorSummary:  &summary,
		}})
		if createErr != nil {
			return nil, createErr
		}
	}
	return jobState, cause
}

func (s *MailboxSyncService) providerClient(providerKind string) (mailboxpkg.ProviderClient, error) {
	client, ok := s.providers[strings.TrimSpace(providerKind)]
	if !ok || client == nil {
		return nil, ErrMailboxUnsupportedProvider
	}
	return client, nil
}

func (s *MailboxSyncService) collectSyncCapabilities(ctx context.Context, input MailboxBatchSyncRequest) ([]*MailboxCapability, error) {
	unique := make(map[int64]*MailboxCapability)
	for _, capabilityID := range input.CapabilityIDs {
		if capabilityID == 0 {
			continue
		}
		capability, err := s.repo.GetCapabilityByID(ctx, capabilityID)
		if err != nil {
			return nil, err
		}
		if capability != nil {
			unique[capability.ID] = capability
		}
	}
	if len(input.CollectorIDs) > 0 {
		capabilities, err := s.repo.ListCapabilities(ctx, MailboxListOptions{Limit: mailboxSyncActiveJobScanLimit})
		if err != nil {
			return nil, err
		}
		collectors := make(map[int64]struct{}, len(input.CollectorIDs))
		for _, collectorID := range input.CollectorIDs {
			if collectorID != 0 {
				collectors[collectorID] = struct{}{}
			}
		}
		for _, capability := range capabilities {
			if capability == nil {
				continue
			}
			if _, ok := collectors[capability.CollectorID]; !ok {
				continue
			}
			if !isInboundMailboxCapability(capability.CapabilityKind) {
				continue
			}
			unique[capability.ID] = capability
		}
	}
	ids := make([]int64, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	capabilities := make([]*MailboxCapability, 0, len(ids))
	for _, id := range ids {
		capability := unique[id]
		if capability == nil {
			continue
		}
		capabilities = append(capabilities, capability)
	}
	return capabilities, nil
}

func (s *MailboxSyncService) findActiveSyncJob(ctx context.Context, jobID int64) (*MailSyncJob, error) {
	jobs, err := s.repo.ListActiveSyncJobs(ctx, nil, mailboxSyncActiveJobScanLimit)
	if err != nil {
		return nil, err
	}
	for _, job := range jobs {
		if job != nil && job.ID == jobID {
			return job, nil
		}
	}
	return nil, ErrMailboxSyncJobNotFound
}

func (s *MailboxSyncService) persistHeaders(ctx context.Context, capability *MailboxCapability, page *mailboxpkg.HeaderPage) ([]*MailHeader, error) {
	store, err := s.headerStore()
	if err != nil {
		return nil, err
	}
	if page == nil || len(page.Headers) == 0 {
		return []*MailHeader{}, nil
	}
	headers := make([]*MailHeader, 0, len(page.Headers))
	for _, header := range page.Headers {
		persisted := &MailHeader{
			CollectorID:        capability.CollectorID,
			CapabilityID:       capability.ID,
			RemoteMessageID:    strings.TrimSpace(header.RemoteMessageID),
			Folder:             strings.TrimSpace(header.Folder),
			Recipients:         append([]string(nil), header.Recipients...),
			Subject:            header.Subject,
			ReceivedAt:         header.ReceivedAt,
			Flags:              append([]string(nil), header.Flags...),
			Snippet:            header.Snippet,
			EnvelopeRecipients: append([]string(nil), header.EnvelopeRecipients...),
			DeliveredTo:        append([]string(nil), header.DeliveredTo...),
			OriginalTo:         append([]string(nil), header.OriginalTo...),
			ResolutionState:    MailResolutionStateUnresolved,
			DetailFetchState:   MailDetailFetchStateNotRequested,
		}
		if sender := strings.TrimSpace(header.Sender); sender != "" {
			persisted.Sender = &sender
		}
		if persisted.Folder == "" {
			if folder, ok := capability.ConnectionConfig["folder"].(string); ok {
				persisted.Folder = strings.TrimSpace(folder)
			}
		}
		headers = append(headers, persisted)
	}
	return store.UpsertSyncHeaders(ctx, headers)
}

func (s *MailboxSyncService) updateCapabilityHealth(ctx context.Context, capability *MailboxCapability, healthState string, lastError *string, updateLastSync bool) (*MailboxCapability, error) {
	updated := cloneMailboxCapabilityValue(capability)
	updated.HealthState = healthState
	updated.LastError = cloneStringPointer(lastError)
	if updateLastSync {
		now := s.now().UTC()
		updated.LastSyncAt = &now
	}
	return s.repo.UpdateCapability(ctx, updated)
}

func (s *MailboxSyncService) headerStore() (mailboxSyncHeaderStore, error) {
	store, ok := s.repo.(mailboxSyncHeaderStore)
	if !ok || store == nil {
		return nil, errors.New("mailbox header store is not configured")
	}
	return store, nil
}

func (s *MailboxSyncService) decorateContext(ctx context.Context) context.Context {
	if s.contextDecorator == nil {
		return ctx
	}
	return s.contextDecorator(ctx)
}

func (s *MailboxSyncService) acquireCapabilityGuards(capabilities []*MailboxCapability) (func(), error) {
	if len(capabilities) == 0 {
		return func() {}, nil
	}
	ids := make([]int64, 0, len(capabilities))
	for _, capability := range capabilities {
		if capability == nil || capability.ID == 0 {
			continue
		}
		ids = append(ids, capability.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	s.guardMu.Lock()
	defer s.guardMu.Unlock()
	for _, id := range ids {
		if _, exists := s.inFlightCaps[id]; exists {
			return nil, fmt.Errorf("capability %d already in progress", id)
		}
	}
	for _, id := range ids {
		s.inFlightCaps[id] = struct{}{}
	}
	return func() {
		s.guardMu.Lock()
		defer s.guardMu.Unlock()
		for _, id := range ids {
			delete(s.inFlightCaps, id)
		}
	}, nil
}

func applyResolutionResult(header *MailHeader, result *MailboxRecipientResolutionResult) {
	if header == nil {
		return
	}
	state := MailResolutionStateUnresolved
	if result != nil && strings.TrimSpace(result.State) != "" {
		state = result.State
	}
	header.ResolutionState = state
	header.ResolvedRecipientIdentityID = nil
	header.ResolvedAddress = nil
	header.MatchType = nil
	header.MatchedValueID = nil
	header.ResolutionSourceField = nil
	if result == nil {
		return
	}
	if strings.TrimSpace(result.Address) != "" {
		header.ResolvedAddress = stringPtr(strings.TrimSpace(result.Address))
	}
	if strings.TrimSpace(result.SourceField) != "" {
		header.ResolutionSourceField = stringPtr(strings.TrimSpace(result.SourceField))
	}
	if result.Identity != nil {
		header.ResolvedRecipientIdentityID = &result.Identity.ID
	}
	if result.MatchValue != nil {
		header.MatchedValueID = &result.MatchValue.ID
		header.MatchType = stringPtr(strings.TrimSpace(result.MatchValue.MatchType))
	}
}

func isInboundMailboxCapability(kind string) bool {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		return false
	}
	return strings.Contains(kind, "imap") || strings.Contains(kind, "pop3") || strings.Contains(kind, "inbox")
}

func isTransientMailboxSyncError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	transientMarkers := []string{"temporary", "temporarily", "timeout", "timed out", "unavailable", "rate limit", "too many requests", "connection reset", "eof", "transient"}
	for _, marker := range transientMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	type temporary interface{ Temporary() bool }
	var temp temporary
	if errors.As(err, &temp) {
		return temp.Temporary()
	}
	return false
}

func syncRetryBackoff(retryCount int) time.Duration {
	if retryCount < 0 {
		retryCount = 0
	}
	backoff := mailboxSyncRetryBaseBackoff << retryCount
	if backoff > mailboxSyncRetryMaxBackoff {
		return mailboxSyncRetryMaxBackoff
	}
	return backoff
}

func pageNextCursor(page *mailboxpkg.HeaderPage) map[string]any {
	if page == nil {
		return nil
	}
	return cloneMailboxSyncMap(page.NextCursor)
}

func cloneMailHeaderValue(in *MailHeader) *MailHeader {
	if in == nil {
		return nil
	}
	clone := *in
	clone.Sender = cloneStringPointer(in.Sender)
	clone.Recipients = append([]string(nil), in.Recipients...)
	clone.Flags = append([]string(nil), in.Flags...)
	clone.EnvelopeRecipients = append([]string(nil), in.EnvelopeRecipients...)
	clone.DeliveredTo = append([]string(nil), in.DeliveredTo...)
	clone.OriginalTo = append([]string(nil), in.OriginalTo...)
	if in.ResolvedRecipientIdentityID != nil {
		identityID := *in.ResolvedRecipientIdentityID
		clone.ResolvedRecipientIdentityID = &identityID
	}
	clone.ResolvedAddress = cloneStringPointer(in.ResolvedAddress)
	clone.MatchType = cloneStringPointer(in.MatchType)
	if in.MatchedValueID != nil {
		matchedValueID := *in.MatchedValueID
		clone.MatchedValueID = &matchedValueID
	}
	clone.ResolutionSourceField = cloneStringPointer(in.ResolutionSourceField)
	return &clone
}

func cloneMailboxSyncMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	clone := make(map[string]any, len(in))
	for key, value := range in {
		clone[key] = value
	}
	return clone
}
