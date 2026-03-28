package service

import (
	"context"
	"time"
)

const (
	ProviderAccountStatusDraft    = "draft"
	ProviderAccountStatusActive   = "active"
	ProviderAccountStatusInvalid  = "invalid"
	ProviderAccountStatusDisabled = "disabled"

	MailboxCapabilityStateHealthy = "healthy"
	MailboxCapabilityStateWarning = "warning"
	MailboxCapabilityStateError   = "error"
	MailboxCapabilityStatePaused  = "paused"
	MailboxCapabilityStateSyncing = "syncing"

	RecipientMatchTypeExactAddress = "exact_address"

	MailResolutionStateResolved   = "resolved"
	MailResolutionStateUnresolved = "unresolved"
	MailResolutionStateAmbiguous  = "ambiguous"

	MailDetailFetchStateNotRequested = "not_requested"
	MailDetailFetchStateSucceeded    = "succeeded"
	MailDetailFetchStateFailed       = "failed"

	MailSyncJobStateQueued    = "queued"
	MailSyncJobStateRunning   = "running"
	MailSyncJobStateSucceeded = "succeeded"
	MailSyncJobStatePartial   = "partial"
	MailSyncJobStateFailed    = "failed"
	MailSyncJobStateCancelled = "cancelled"

	MailSyncTriggerSourceScheduled = "scheduled"
	MailSyncTriggerSourceManual    = "manual"
	MailSyncTriggerSourceRetry     = "retry"

	ProviderAuthKindOAuth2 = "oauth2"
	ProviderAuthKindBasic  = "basic"
)

type ProviderAccount struct {
	ID                  int64      `json:"id"`
	DisplayName         string     `json:"display_name"`
	ProviderKind        string     `json:"provider_kind"`
	AuthKind            string     `json:"auth_kind"`
	Status              string     `json:"status"`
	EncryptedPayload    string     `json:"encrypted_payload"`
	MailboxHint         *string    `json:"mailbox_hint"`
	ProviderIdentifier  *string    `json:"provider_identifier"`
	PayloadVersion      int        `json:"payload_version"`
	LastImportedAt      *time.Time `json:"last_imported_at"`
	LastValidationAt    *time.Time `json:"last_validation_at"`
	LastValidationError *string    `json:"last_validation_error"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	DeletedAt           *time.Time `json:"deleted_at"`
}

type CollectorMailbox struct {
	ID           int64      `json:"id"`
	EmailAddress string     `json:"email_address"`
	DisplayName  string     `json:"display_name"`
	Enabled      bool       `json:"enabled"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at"`
}

type MailboxCapability struct {
	ID                int64      `json:"id"`
	ProviderAccountID int64      `json:"provider_account_id"`
	CollectorID       int64      `json:"collector_id"`
	CapabilityKind    string     `json:"capability_kind"`
	Folder            string     `json:"folder"`
	State             string     `json:"state"`
	ImportCursor      *string    `json:"import_cursor"`
	LastSyncedAt      *time.Time `json:"last_synced_at"`
	SyncDueAt         time.Time  `json:"sync_due_at"`
	LastError         *string    `json:"last_error"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `json:"deleted_at"`
}

type RecipientIdentity struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	NormalizedName string     `json:"normalized_name"`
	Enabled        bool       `json:"enabled"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at"`
}

type RecipientMatchValue struct {
	ID                  int64      `json:"id"`
	RecipientIdentityID int64      `json:"recipient_identity_id"`
	MatchType           string     `json:"match_type"`
	MatchValue          string     `json:"match_value"`
	NormalizedValue     string     `json:"normalized_value"`
	Active              bool       `json:"active"`
	Priority            int        `json:"priority"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	DisabledAt          *time.Time `json:"disabled_at"`
}

type MailHeader struct {
	ID                         int64      `json:"id"`
	CollectorID                int64      `json:"collector_id"`
	CapabilityID               int64      `json:"capability_id"`
	RemoteMessageID            string     `json:"remote_message_id"`
	Folder                     string     `json:"folder"`
	Sender                     *string    `json:"sender"`
	Recipients                 []string   `json:"recipients"`
	Subject                    string     `json:"subject"`
	ReceivedAt                 time.Time  `json:"received_at"`
	Flags                      []string   `json:"flags"`
	Snippet                    string     `json:"snippet"`
	EnvelopeRecipients         []string   `json:"envelope_recipients"`
	DeliveredTo                []string   `json:"delivered_to"`
	OriginalTo                 []string   `json:"original_to"`
	ResolvedRecipientIdentityID *int64    `json:"resolved_recipient_identity_id"`
	ResolvedAddress            *string    `json:"resolved_address"`
	MatchType                  *string    `json:"match_type"`
	MatchedValueID             *int64     `json:"matched_value_id"`
	ResolutionSourceField      *string    `json:"resolution_source_field"`
	ResolutionState            string     `json:"resolution_state"`
	DetailFetchState           string     `json:"detail_fetch_state"`
	CreatedAt                  time.Time  `json:"created_at"`
	UpdatedAt                  time.Time  `json:"updated_at"`
}

type MailSyncJob struct {
	ID            int64      `json:"id"`
	CapabilityID  int64      `json:"capability_id"`
	BatchID       *string    `json:"batch_id"`
	State         string     `json:"state"`
	TriggerSource string     `json:"trigger_source"`
	ScheduledFor  time.Time  `json:"scheduled_for"`
	StartedAt     *time.Time `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
	Retryable     bool       `json:"retryable"`
	RetryCount    int        `json:"retry_count"`
	NextRetryAt   *time.Time `json:"next_retry_at"`
	ErrorSummary  *string    `json:"error_summary"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type MailHeaderListFilter struct {
	CollectorID  *int64
	CapabilityID *int64
	Folder       string
	Limit        int
}

type MailboxProvider interface {
	ProviderKind() string
	ValidateAccount(ctx context.Context, account *ProviderAccount) error
	DiscoverCapabilities(ctx context.Context, account *ProviderAccount, collector *CollectorMailbox) ([]*MailboxCapability, error)
	FetchHeaders(ctx context.Context, capability *MailboxCapability, cursor *string, limit int) ([]*MailHeader, *string, error)
}

type MailboxRepository interface {
	CreateProviderAccount(ctx context.Context, account *ProviderAccount) (*ProviderAccount, error)
	CreateCollector(ctx context.Context, collector *CollectorMailbox) (*CollectorMailbox, error)
	CreateCapability(ctx context.Context, capability *MailboxCapability) (*MailboxCapability, error)
	CreateRecipientIdentity(ctx context.Context, in *RecipientIdentity, values []*RecipientMatchValue) (*RecipientIdentity, error)
	ListHeaders(ctx context.Context, filter MailHeaderListFilter) ([]*MailHeader, int64, error)
	CreateSyncJobs(ctx context.Context, jobs []*MailSyncJob) ([]*MailSyncJob, error)
	ClaimDueCapabilities(ctx context.Context, now time.Time, limit int) ([]*MailboxCapability, error)
}
