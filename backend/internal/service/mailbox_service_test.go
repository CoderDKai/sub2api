package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	mailboxpkg "github.com/Wei-Shaw/sub2api/internal/pkg/mailbox"
	"github.com/stretchr/testify/require"
)

func TestParseOutlookImportBundleSuccess(t *testing.T) {
	bundle, err := mailboxpkg.ParseOutlookImportBundle("boss@example.com----provider-42----opaque-left----opaque-right")
	require.NoError(t, err)
	require.Equal(t, "boss@example.com", bundle.MailboxIdentifier)
	require.Equal(t, "provider-42", bundle.ProviderIdentifier)
	require.Equal(t, "opaque-left----opaque-right", bundle.TokenBundle)
}

func TestParseOutlookImportBundleInvalidFormat(t *testing.T) {
	_, err := mailboxpkg.ParseOutlookImportBundle("boss@example.com----provider-42----missing")
	require.ErrorIs(t, err, mailboxpkg.ErrMailboxImportFormat)

	_, err = mailboxpkg.ParseOutlookImportBundle("boss@example.com----------opaque-left----opaque-right")
	require.ErrorIs(t, err, mailboxpkg.ErrMailboxImportFormat)
}

func TestMailboxServiceCreateProviderParsesOutlookImportBundle(t *testing.T) {
	repo := newMailboxRepositoryStub()
	audit := &mailboxAuditStub{}
	service := newMailboxServiceForTest(repo, audit, nil)
	fixedNow := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	created, err := service.CreateProvider(context.Background(), MailboxProviderUpsertInput{
		DisplayName:      "Outlook Import",
		ProviderKind:     MailboxProviderKindMicrosoft,
		AuthKind:         ProviderAuthKindImportBundle,
		EncryptedPayload: "boss@example.com----provider-42----opaque-left----opaque-right",
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, ProviderAccountStatusDraft, created.Status)
	require.NotNil(t, created.MailboxHint)
	require.Equal(t, "boss@example.com", *created.MailboxHint)
	require.NotNil(t, created.ProviderIdentifier)
	require.Equal(t, "provider-42", *created.ProviderIdentifier)
	require.NotNil(t, created.LastImportedAt)
	require.Equal(t, fixedNow, created.LastImportedAt.UTC())

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(created.EncryptedPayload), &payload))
	require.Equal(t, "boss@example.com", payload["mailbox_identifier"])
	require.Equal(t, "provider-42", payload["provider_identifier"])
	require.Equal(t, "opaque-left----opaque-right", payload["token_bundle"])
	require.Contains(t, audit.events, "provider_create")
	require.Contains(t, audit.events, "provider_import")
}

func TestMailboxServiceUpdateProviderRejectsUnsupportedProvider(t *testing.T) {
	repo := newMailboxRepositoryStub()
	service := newMailboxServiceForTest(repo, &mailboxAuditStub{}, nil)

	_, err := service.UpdateProvider(context.Background(), 1, MailboxProviderUpsertInput{
		DisplayName:      "Bad",
		ProviderKind:     "unknown",
		AuthKind:         ProviderAuthKindBasic,
		EncryptedPayload: `{"protocol":"imap"}`,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrMailboxUnsupportedProvider)
}

func TestMailboxServiceValidateProviderMapsInvalidFormatAndInvalidates(t *testing.T) {
	repo := newMailboxRepositoryStub()
	repo.providers[1] = &ProviderAccount{
		ID:               1,
		DisplayName:      "Microsoft",
		ProviderKind:     MailboxProviderKindMicrosoft,
		AuthKind:         ProviderAuthKindOAuth2,
		Status:           ProviderAccountStatusActive,
		EncryptedPayload: `{"access_token":"token"}`,
	}
	service := newMailboxServiceForTest(repo, &mailboxAuditStub{}, map[string]mailboxpkg.ProviderClient{
		MailboxProviderKindMicrosoft: &providerClientStub{
			validateResult: &mailboxpkg.ValidationResult{
				Code:              mailboxpkg.ValidationCodeInvalidFormat,
				Message:           "bad token bundle",
				InvalidateAccount: true,
			},
		},
	})

	result, err := service.ValidateProvider(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, mailboxpkg.ValidationCodeInvalidFormat, result.Code)
	require.Equal(t, ProviderAccountStatusInvalid, result.Account.Status)
	require.NotNil(t, result.Account.LastValidationError)
	require.Equal(t, "bad token bundle", *result.Account.LastValidationError)
}

func TestMailboxServiceValidateProviderMapsValidationFailedWithoutInvalidation(t *testing.T) {
	repo := newMailboxRepositoryStub()
	repo.providers[1] = &ProviderAccount{
		ID:               1,
		DisplayName:      "Microsoft",
		ProviderKind:     MailboxProviderKindMicrosoft,
		AuthKind:         ProviderAuthKindOAuth2,
		Status:           ProviderAccountStatusActive,
		EncryptedPayload: `{"access_token":"token"}`,
	}
	service := newMailboxServiceForTest(repo, &mailboxAuditStub{}, map[string]mailboxpkg.ProviderClient{
		MailboxProviderKindMicrosoft: &providerClientStub{
			validateResult: &mailboxpkg.ValidationResult{
				Code:    mailboxpkg.ValidationCodeValidationFailed,
				Message: "graph unavailable",
			},
		},
	})

	result, err := service.ValidateProvider(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, mailboxpkg.ValidationCodeValidationFailed, result.Code)
	require.Equal(t, ProviderAccountStatusActive, result.Account.Status)
	require.NotNil(t, result.Account.LastValidationError)
	require.Equal(t, "graph unavailable", *result.Account.LastValidationError)
}

func TestMailboxServiceValidateProviderMapsExpiredAndInvalidates(t *testing.T) {
	repo := newMailboxRepositoryStub()
	repo.providers[1] = &ProviderAccount{
		ID:               1,
		DisplayName:      "Microsoft",
		ProviderKind:     MailboxProviderKindMicrosoft,
		AuthKind:         ProviderAuthKindOAuth2,
		Status:           ProviderAccountStatusActive,
		EncryptedPayload: `{"access_token":"token"}`,
	}
	service := newMailboxServiceForTest(repo, &mailboxAuditStub{}, map[string]mailboxpkg.ProviderClient{
		MailboxProviderKindMicrosoft: &providerClientStub{
			validateResult: &mailboxpkg.ValidationResult{
				Code:              mailboxpkg.ValidationCodeExpiredOrRevoked,
				Message:           "token expired",
				InvalidateAccount: true,
			},
		},
	})

	result, err := service.ValidateProvider(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, mailboxpkg.ValidationCodeExpiredOrRevoked, result.Code)
	require.Equal(t, ProviderAccountStatusInvalid, result.Account.Status)
	require.NotNil(t, result.Account.LastValidationError)
	require.Equal(t, "token expired", *result.Account.LastValidationError)
}

func TestMailboxServiceTestCapabilitySuccessUpdatesHealth(t *testing.T) {
	repo := newMailboxRepositoryStub()
	repo.providers[1] = &ProviderAccount{
		ID:               1,
		DisplayName:      "Basic",
		ProviderKind:     MailboxProviderKindBasic,
		AuthKind:         ProviderAuthKindBasic,
		Status:           ProviderAccountStatusActive,
		EncryptedPayload: `{"protocol":"imap","host":"imap.example.com","username":"boss@example.com","password":"secret"}`,
	}
	repo.capabilities[7] = &MailboxCapability{
		ID:                7,
		ProviderAccountID: 1,
		CollectorID:       2,
		CapabilityKind:    "imap",
		ConnectionConfig:  MailboxConnectionConfig{"folder": "INBOX"},
		CursorState:       MailboxCursorState{},
		HealthState:       MailboxCapabilityStateError,
	}
	audit := &mailboxAuditStub{}
	service := newMailboxServiceForTest(repo, audit, map[string]mailboxpkg.ProviderClient{
		MailboxProviderKindBasic: &providerClientStub{
			headerPage: &mailboxpkg.HeaderPage{Headers: []mailboxpkg.Header{{RemoteMessageID: "msg-1"}}},
		},
	})
	fixedNow := time.Date(2026, 3, 29, 13, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	updated, err := service.TestCapability(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, MailboxCapabilityStateHealthy, updated.HealthState)
	require.Nil(t, updated.LastError)
	require.NotNil(t, updated.LastSyncAt)
	require.Equal(t, fixedNow, updated.LastSyncAt.UTC())
	require.Contains(t, audit.events, "capability_test")
}

func TestMailboxServiceTestCapabilityMarksErrorWhenProviderProfileInvalid(t *testing.T) {
	repo := newMailboxRepositoryStub()
	repo.providers[1] = &ProviderAccount{
		ID:               1,
		DisplayName:      "Microsoft",
		ProviderKind:     MailboxProviderKindMicrosoft,
		AuthKind:         ProviderAuthKindOAuth2,
		Status:           ProviderAccountStatusActive,
		EncryptedPayload: `{"access_token":`,
	}
	repo.capabilities[7] = &MailboxCapability{
		ID:                7,
		ProviderAccountID: 1,
		CollectorID:       2,
		CapabilityKind:    "microsoft_inbox",
		ConnectionConfig:  MailboxConnectionConfig{"folder": "INBOX"},
		CursorState:       MailboxCursorState{},
		HealthState:       MailboxCapabilityStateHealthy,
	}
	audit := &mailboxAuditStub{}
	service := newMailboxServiceForTest(repo, audit, map[string]mailboxpkg.ProviderClient{
		MailboxProviderKindMicrosoft: &providerClientStub{},
	})

	updated, err := service.TestCapability(context.Background(), 7)
	require.Error(t, err)
	require.NotNil(t, updated)
	require.Equal(t, MailboxCapabilityStateError, updated.HealthState)
	require.NotNil(t, updated.LastError)
	require.Contains(t, *updated.LastError, "decode provider payload")
	require.Contains(t, audit.events, "capability_test")
}

func TestMailboxServiceTestCapabilityMarksErrorWhenProviderClientMissing(t *testing.T) {
	repo := newMailboxRepositoryStub()
	repo.providers[1] = &ProviderAccount{
		ID:               1,
		DisplayName:      "Microsoft",
		ProviderKind:     MailboxProviderKindMicrosoft,
		AuthKind:         ProviderAuthKindOAuth2,
		Status:           ProviderAccountStatusActive,
		EncryptedPayload: `{"access_token":"token"}`,
	}
	repo.capabilities[7] = &MailboxCapability{
		ID:                7,
		ProviderAccountID: 1,
		CollectorID:       2,
		CapabilityKind:    "microsoft_inbox",
		ConnectionConfig:  MailboxConnectionConfig{"folder": "INBOX"},
		CursorState:       MailboxCursorState{},
		HealthState:       MailboxCapabilityStateHealthy,
	}
	audit := &mailboxAuditStub{}
	service := newMailboxServiceForTest(repo, audit, nil)

	updated, err := service.TestCapability(context.Background(), 7)
	require.ErrorIs(t, err, ErrMailboxUnsupportedProvider)
	require.NotNil(t, updated)
	require.Equal(t, MailboxCapabilityStateError, updated.HealthState)
	require.NotNil(t, updated.LastError)
	require.Contains(t, *updated.LastError, ErrMailboxUnsupportedProvider.Error())
	require.Contains(t, audit.events, "capability_test")
}

type mailboxRepositoryStub struct {
	providers                    map[int64]*ProviderAccount
	collectors                   map[int64]*CollectorMailbox
	capabilities                 map[int64]*MailboxCapability
	identities                   map[int64]*RecipientIdentity
	matchValues                  map[int64][]*RecipientMatchValue
	listRecipientMatchValueCalls int
	listAllMatchValueCalls       int
	nextProviderID               int64
	nextCollectorID              int64
	nextCapabilityID             int64
	nextIdentityID               int64
	nextMatchValueID             int64
	createProviderErr            error
	updateProviderErr            error
	updateCapErr                 error
}

var _ MailboxRepository = (*mailboxRepositoryStub)(nil)

func newMailboxRepositoryStub() *mailboxRepositoryStub {
	return &mailboxRepositoryStub{
		providers:    make(map[int64]*ProviderAccount),
		collectors:   make(map[int64]*CollectorMailbox),
		capabilities: make(map[int64]*MailboxCapability),
		identities:   make(map[int64]*RecipientIdentity),
		matchValues:  make(map[int64][]*RecipientMatchValue),
	}
}

func (r *mailboxRepositoryStub) CreateProviderAccount(ctx context.Context, account *ProviderAccount) (*ProviderAccount, error) {
	if r.createProviderErr != nil {
		return nil, r.createProviderErr
	}
	r.nextProviderID++
	cloned := cloneProviderAccount(account)
	cloned.ID = r.nextProviderID
	r.providers[cloned.ID] = cloned
	return cloneProviderAccount(cloned), nil
}

func (r *mailboxRepositoryStub) GetProviderAccountByID(ctx context.Context, id int64) (*ProviderAccount, error) {
	account, ok := r.providers[id]
	if !ok {
		return nil, errors.New("provider not found")
	}
	return cloneProviderAccount(account), nil
}

func (r *mailboxRepositoryStub) UpdateProviderAccount(ctx context.Context, account *ProviderAccount) (*ProviderAccount, error) {
	if r.updateProviderErr != nil {
		return nil, r.updateProviderErr
	}
	cloned := cloneProviderAccount(account)
	r.providers[cloned.ID] = cloned
	return cloneProviderAccount(cloned), nil
}

func (r *mailboxRepositoryStub) ListProviderAccounts(ctx context.Context, opts MailboxListOptions) ([]*ProviderAccount, error) {
	items := make([]*ProviderAccount, 0, len(r.providers))
	for _, account := range r.providers {
		items = append(items, cloneProviderAccount(account))
	}
	return items, nil
}

func (r *mailboxRepositoryStub) DeleteProviderAccount(ctx context.Context, id int64) error {
	delete(r.providers, id)
	return nil
}

func (r *mailboxRepositoryStub) CreateCollector(ctx context.Context, collector *CollectorMailbox) (*CollectorMailbox, error) {
	r.nextCollectorID++
	cloned := cloneCollectorMailbox(collector)
	cloned.ID = r.nextCollectorID
	r.collectors[cloned.ID] = cloned
	return cloneCollectorMailbox(cloned), nil
}

func (r *mailboxRepositoryStub) GetCollectorByID(ctx context.Context, id int64) (*CollectorMailbox, error) {
	collector, ok := r.collectors[id]
	if !ok {
		return nil, errors.New("collector not found")
	}
	return cloneCollectorMailbox(collector), nil
}

func (r *mailboxRepositoryStub) UpdateCollector(ctx context.Context, collector *CollectorMailbox) (*CollectorMailbox, error) {
	cloned := cloneCollectorMailbox(collector)
	r.collectors[cloned.ID] = cloned
	return cloneCollectorMailbox(cloned), nil
}

func (r *mailboxRepositoryStub) ListCollectors(ctx context.Context, opts MailboxListOptions) ([]*CollectorMailbox, error) {
	items := make([]*CollectorMailbox, 0, len(r.collectors))
	for _, collector := range r.collectors {
		items = append(items, cloneCollectorMailbox(collector))
	}
	return items, nil
}

func (r *mailboxRepositoryStub) DeleteCollector(ctx context.Context, id int64) error {
	delete(r.collectors, id)
	return nil
}

func (r *mailboxRepositoryStub) CreateCapability(ctx context.Context, capability *MailboxCapability) (*MailboxCapability, error) {
	r.nextCapabilityID++
	cloned := cloneMailboxCapability(capability)
	cloned.ID = r.nextCapabilityID
	r.capabilities[cloned.ID] = cloned
	return cloneMailboxCapability(cloned), nil
}

func (r *mailboxRepositoryStub) GetCapabilityByID(ctx context.Context, id int64) (*MailboxCapability, error) {
	capability, ok := r.capabilities[id]
	if !ok {
		return nil, errors.New("capability not found")
	}
	return cloneMailboxCapability(capability), nil
}

func (r *mailboxRepositoryStub) UpdateCapability(ctx context.Context, capability *MailboxCapability) (*MailboxCapability, error) {
	if r.updateCapErr != nil {
		return nil, r.updateCapErr
	}
	cloned := cloneMailboxCapability(capability)
	r.capabilities[cloned.ID] = cloned
	return cloneMailboxCapability(cloned), nil
}

func (r *mailboxRepositoryStub) ListCapabilities(ctx context.Context, opts MailboxListOptions) ([]*MailboxCapability, error) {
	items := make([]*MailboxCapability, 0, len(r.capabilities))
	for _, capability := range r.capabilities {
		items = append(items, cloneMailboxCapability(capability))
	}
	return items, nil
}

func (r *mailboxRepositoryStub) DeleteCapability(ctx context.Context, id int64) error {
	delete(r.capabilities, id)
	return nil
}

func (r *mailboxRepositoryStub) CreateRecipientIdentity(ctx context.Context, in *RecipientIdentity, values []*RecipientMatchValue) (*RecipientIdentity, error) {
	r.nextIdentityID++
	identity := cloneRecipientIdentity(in)
	identity.ID = r.nextIdentityID
	r.identities[identity.ID] = identity
	createdValues := make([]*RecipientMatchValue, 0, len(values))
	for _, value := range values {
		r.nextMatchValueID++
		cloned := cloneRecipientMatchValue(value)
		cloned.ID = r.nextMatchValueID
		cloned.RecipientIdentityID = identity.ID
		createdValues = append(createdValues, cloned)
	}
	r.matchValues[identity.ID] = createdValues
	return cloneRecipientIdentity(identity), nil
}

func (r *mailboxRepositoryStub) GetRecipientIdentityByID(ctx context.Context, id int64) (*RecipientIdentity, error) {
	identity, ok := r.identities[id]
	if !ok {
		return nil, errors.New("identity not found")
	}
	return cloneRecipientIdentity(identity), nil
}

func (r *mailboxRepositoryStub) UpdateRecipientIdentity(ctx context.Context, in *RecipientIdentity) (*RecipientIdentity, error) {
	identity := cloneRecipientIdentity(in)
	r.identities[identity.ID] = identity
	return cloneRecipientIdentity(identity), nil
}

func (r *mailboxRepositoryStub) UpdateRecipientIdentityWithMatchValues(ctx context.Context, in *RecipientIdentity, values []*RecipientMatchValue) (*RecipientIdentity, []*RecipientMatchValue, error) {
	identity := cloneRecipientIdentity(in)
	r.identities[identity.ID] = identity
	updated := make([]*RecipientMatchValue, 0, len(values))
	for _, value := range values {
		r.nextMatchValueID++
		cloned := cloneRecipientMatchValue(value)
		cloned.ID = r.nextMatchValueID
		cloned.RecipientIdentityID = identity.ID
		updated = append(updated, cloned)
	}
	r.matchValues[identity.ID] = updated
	return cloneRecipientIdentity(identity), updated, nil
}

func (r *mailboxRepositoryStub) ListRecipientIdentities(ctx context.Context, opts MailboxListOptions) ([]*RecipientIdentity, error) {
	items := make([]*RecipientIdentity, 0, len(r.identities))
	for _, identity := range r.identities {
		items = append(items, cloneRecipientIdentity(identity))
	}
	return items, nil
}

func (r *mailboxRepositoryStub) DeleteRecipientIdentity(ctx context.Context, id int64) error {
	delete(r.identities, id)
	delete(r.matchValues, id)
	return nil
}

func (r *mailboxRepositoryStub) ListRecipientMatchValues(ctx context.Context, recipientIdentityID int64) ([]*RecipientMatchValue, error) {
	r.listRecipientMatchValueCalls++
	values := r.matchValues[recipientIdentityID]
	cloned := make([]*RecipientMatchValue, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, cloneRecipientMatchValue(value))
	}
	return cloned, nil
}

func (r *mailboxRepositoryStub) ListActiveRecipientMatchValues(ctx context.Context) ([]*RecipientMatchValue, error) {
	r.listAllMatchValueCalls++
	cloned := make([]*RecipientMatchValue, 0)
	for _, values := range r.matchValues {
		for _, value := range values {
			if value == nil || !value.Active {
				continue
			}
			cloned = append(cloned, cloneRecipientMatchValue(value))
		}
	}
	return cloned, nil
}

func (r *mailboxRepositoryStub) ReplaceRecipientMatchValues(ctx context.Context, recipientIdentityID int64, values []*RecipientMatchValue) ([]*RecipientMatchValue, error) {
	updated := make([]*RecipientMatchValue, 0, len(values))
	for _, value := range values {
		r.nextMatchValueID++
		cloned := cloneRecipientMatchValue(value)
		cloned.ID = r.nextMatchValueID
		cloned.RecipientIdentityID = recipientIdentityID
		updated = append(updated, cloned)
	}
	r.matchValues[recipientIdentityID] = updated
	return updated, nil
}

func (r *mailboxRepositoryStub) GetHeaderByID(ctx context.Context, id int64) (*MailHeader, error) {
	panic("unexpected GetHeaderByID call")
}

func (r *mailboxRepositoryStub) ListHeaders(ctx context.Context, filter MailHeaderListFilter) ([]*MailHeader, int64, error) {
	panic("unexpected ListHeaders call")
}

func (r *mailboxRepositoryStub) UpsertSyncHeaders(ctx context.Context, headers []*MailHeader) ([]*MailHeader, error) {
	panic("unexpected UpsertSyncHeaders call")
}

func (r *mailboxRepositoryStub) UpdateHeaderDetail(ctx context.Context, header *MailHeader) (*MailHeader, error) {
	panic("unexpected UpdateHeaderDetail call")
}

func (r *mailboxRepositoryStub) CreateSyncJobs(ctx context.Context, jobs []*MailSyncJob) ([]*MailSyncJob, error) {
	panic("unexpected CreateSyncJobs call")
}

func (r *mailboxRepositoryStub) ListSyncJobsByBatchID(ctx context.Context, batchID string) ([]*MailSyncJob, error) {
	panic("unexpected ListSyncJobsByBatchID call")
}

func (r *mailboxRepositoryStub) ClaimRunnableRetrySyncJobs(ctx context.Context, now time.Time, limit int) ([]*MailSyncJob, error) {
	panic("unexpected ClaimRunnableRetrySyncJobs call")
}

func (r *mailboxRepositoryStub) ListActiveSyncJobs(ctx context.Context, capabilityID *int64, limit int) ([]*MailSyncJob, error) {
	panic("unexpected ListActiveSyncJobs call")
}

func (r *mailboxRepositoryStub) UpdateSyncJobState(ctx context.Context, jobID int64, state string, startedAt, finishedAt, nextRetryAt *time.Time, errorSummary *string) (*MailSyncJob, error) {
	panic("unexpected UpdateSyncJobState call")
}

func (r *mailboxRepositoryStub) ClaimDueCapabilities(ctx context.Context, now time.Time, limit int) ([]*MailboxCapability, error) {
	panic("unexpected ClaimDueCapabilities call")
}

type providerClientStub struct {
	validateResult *mailboxpkg.ValidationResult
	validateErr    error
	headerPage     *mailboxpkg.HeaderPage
	listErr        error
}

func (s *providerClientStub) Validate(ctx context.Context, profile mailboxpkg.ProviderProfile) (*mailboxpkg.ValidationResult, error) {
	if s.validateErr != nil {
		return nil, s.validateErr
	}
	if s.validateResult != nil {
		return s.validateResult, nil
	}
	return &mailboxpkg.ValidationResult{Code: mailboxpkg.ValidationCodeOK}, nil
}

func (s *providerClientStub) ListHeaders(ctx context.Context, profile mailboxpkg.ProviderProfile, capability mailboxpkg.CapabilityProfile, limit int) (*mailboxpkg.HeaderPage, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.headerPage != nil {
		return s.headerPage, nil
	}
	return &mailboxpkg.HeaderPage{}, nil
}

type mailboxAuditStub struct {
	events []string
}

func (a *mailboxAuditStub) RecordProviderCreate(ctx context.Context, account *ProviderAccount) {
	a.events = append(a.events, "provider_create")
}

func (a *mailboxAuditStub) RecordProviderUpdate(ctx context.Context, before, after *ProviderAccount) {
	a.events = append(a.events, "provider_update")
}

func (a *mailboxAuditStub) RecordProviderStatus(ctx context.Context, account *ProviderAccount, reason string) {
	a.events = append(a.events, "provider_status")
}

func (a *mailboxAuditStub) RecordProviderImport(ctx context.Context, account *ProviderAccount) {
	a.events = append(a.events, "provider_import")
}

func (a *mailboxAuditStub) RecordCollectorUpdate(ctx context.Context, collector *CollectorMailbox) {
	a.events = append(a.events, "collector_update")
}

func (a *mailboxAuditStub) RecordRecipientUpdate(ctx context.Context, identity *RecipientIdentity) {
	a.events = append(a.events, "recipient_update")
}

func (a *mailboxAuditStub) RecordCapabilityTest(ctx context.Context, capability *MailboxCapability, success bool) {
	a.events = append(a.events, "capability_test")
}

func (a *mailboxAuditStub) RecordManualSync(ctx context.Context, capabilityID int64) {
	a.events = append(a.events, "manual_sync")
}

func (a *mailboxAuditStub) RecordBatchSync(ctx context.Context, batchID string, capabilityIDs []int64) {
	a.events = append(a.events, "batch_sync")
}

func (a *mailboxAuditStub) RecordInboxDetailFetch(ctx context.Context, headerID int64) {
	a.events = append(a.events, "inbox_detail_fetch")
}

func newMailboxServiceForTest(repo MailboxRepository, audit MailboxAuditor, providers map[string]mailboxpkg.ProviderClient) *MailboxService {
	if providers == nil {
		providers = map[string]mailboxpkg.ProviderClient{}
	}
	service := newMailboxServiceWithProviders(repo, audit, providers)
	service.now = func() time.Time { return time.Now().UTC() }
	return service
}

func cloneProviderAccount(in *ProviderAccount) *ProviderAccount {
	if in == nil {
		return nil
	}
	clone := *in
	clone.MailboxHint = cloneStringPtr(in.MailboxHint)
	clone.ProviderIdentifier = cloneStringPtr(in.ProviderIdentifier)
	clone.LastImportedAt = cloneTimePtr(in.LastImportedAt)
	clone.LastValidationAt = cloneTimePtr(in.LastValidationAt)
	clone.LastValidationError = cloneStringPtr(in.LastValidationError)
	clone.DeletedAt = cloneTimePtr(in.DeletedAt)
	return &clone
}

func cloneCollectorMailbox(in *CollectorMailbox) *CollectorMailbox {
	if in == nil {
		return nil
	}
	clone := *in
	clone.BusinessTags = append([]string(nil), in.BusinessTags...)
	clone.DeletedAt = cloneTimePtr(in.DeletedAt)
	return &clone
}

func cloneMailboxCapability(in *MailboxCapability) *MailboxCapability {
	if in == nil {
		return nil
	}
	clone := *in
	clone.ConnectionConfig = MailboxConnectionConfig(mailboxCloneMapStringAny(in.ConnectionConfig))
	clone.CursorState = MailboxCursorState(mailboxCloneMapStringAny(in.CursorState))
	clone.NextSyncAt = cloneTimePtr(in.NextSyncAt)
	clone.LastSyncAt = cloneTimePtr(in.LastSyncAt)
	clone.LastError = cloneStringPtr(in.LastError)
	clone.DeletedAt = cloneTimePtr(in.DeletedAt)
	return &clone
}

func cloneRecipientIdentity(in *RecipientIdentity) *RecipientIdentity {
	if in == nil {
		return nil
	}
	clone := *in
	clone.DeletedAt = cloneTimePtr(in.DeletedAt)
	return &clone
}

func cloneRecipientMatchValue(in *RecipientMatchValue) *RecipientMatchValue {
	if in == nil {
		return nil
	}
	clone := *in
	clone.SourceMetadata = RecipientMatchSourceMetadata(mailboxCloneMapStringAny(in.SourceMetadata))
	clone.DisabledAt = cloneTimePtr(in.DisabledAt)
	return &clone
}

func mailboxCloneMapStringAny(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	clone := make(map[string]any, len(in))
	for k, v := range in {
		clone[k] = v
	}
	return clone
}

func cloneStringPtr(in *string) *string {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneTimePtr(in *time.Time) *time.Time {
	if in == nil {
		return nil
	}
	out := in.UTC()
	return &out
}
