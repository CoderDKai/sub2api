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
	RecipientMatchTypeDomainSuffix = "domain_suffix"

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

	MailSyncTriggerSourceSchedule    = "schedule"
	MailSyncTriggerSourceManual      = "manual"
	MailSyncTriggerSourceManualBatch = "manual_batch"
	MailSyncTriggerSourceRetry       = "retry"

	ProviderAuthKindImportBundle = "import_bundle"
	ProviderAuthKindBasic        = "basic"
	ProviderAuthKindIMAPPassword = "imap_password"
	ProviderAuthKindSMTPPassword = "smtp_password"
	ProviderAuthKindPOP3Password = "pop3_password"
	ProviderAuthKindOAuth2       = "oauth2"
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
	BusinessTags []string   `json:"business_tags"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at"`
}

type MailboxConnectionConfig map[string]any

type MailboxCursorState map[string]any

type RecipientMatchSourceMetadata map[string]any

type MailboxCapability struct {
	ID                  int64                   `json:"id"`
	ProviderAccountID   int64                   `json:"provider_account_id"`
	CollectorID         int64                   `json:"collector_id"`
	CapabilityKind      string                  `json:"capability_kind"`
	ConnectionConfig    MailboxConnectionConfig `json:"connection_config"`
	CursorState         MailboxCursorState      `json:"cursor_state"`
	SyncEnabled         bool                    `json:"sync_enabled"`
	SyncIntervalSeconds int                     `json:"sync_interval_seconds"`
	NextSyncAt          *time.Time              `json:"next_sync_at"`
	LastSyncAt          *time.Time              `json:"last_sync_at"`
	HealthState         string                  `json:"health_state"`
	LastError           *string                 `json:"last_error"`
	CreatedAt           time.Time               `json:"created_at"`
	UpdatedAt           time.Time               `json:"updated_at"`
	DeletedAt           *time.Time              `json:"deleted_at"`
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
	ID                  int64                        `json:"id"`
	RecipientIdentityID int64                        `json:"recipient_identity_id"`
	MatchType           string                       `json:"match_type"`
	MatchValue          string                       `json:"match_value"`
	NormalizedValue     string                       `json:"normalized_value"`
	Active              bool                         `json:"active"`
	Priority            int                          `json:"priority"`
	SourceKind          string                       `json:"source_kind"`
	SourceMetadata      RecipientMatchSourceMetadata `json:"source_metadata"`
	CreatedAt           time.Time                    `json:"created_at"`
	UpdatedAt           time.Time                    `json:"updated_at"`
	DisabledAt          *time.Time                   `json:"disabled_at"`
}

type MailHeader struct {
	ID                          int64     `json:"id"`
	CollectorID                 int64     `json:"collector_id"`
	CapabilityID                int64     `json:"capability_id"`
	RemoteMessageID             string    `json:"remote_message_id"`
	Folder                      string    `json:"folder"`
	Sender                      *string   `json:"sender"`
	Recipients                  []string  `json:"recipients"`
	Subject                     string    `json:"subject"`
	ReceivedAt                  time.Time `json:"received_at"`
	Flags                       []string  `json:"flags"`
	Snippet                     string    `json:"snippet"`
	EnvelopeRecipients          []string  `json:"envelope_recipients"`
	DeliveredTo                 []string  `json:"delivered_to"`
	OriginalTo                  []string  `json:"original_to"`
	ResolvedRecipientIdentityID *int64    `json:"resolved_recipient_identity_id"`
	ResolvedAddress             *string   `json:"resolved_address"`
	MatchType                   *string   `json:"match_type"`
	MatchedValueID              *int64    `json:"matched_value_id"`
	ResolutionSourceField       *string   `json:"resolution_source_field"`
	ResolutionState             string    `json:"resolution_state"`
	DetailFetchState            string    `json:"detail_fetch_state"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
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
	Offset       int
	Limit        int
}

type MailboxListOptions struct {
	IncludeDeleted bool
	Offset         int
	Limit          int
}

type MailboxProvider interface {
	ProviderKind() string
	ValidateAccount(ctx context.Context, account *ProviderAccount) error
	DiscoverCapabilities(ctx context.Context, account *ProviderAccount, collector *CollectorMailbox) ([]*MailboxCapability, error)
	FetchHeaders(ctx context.Context, capability *MailboxCapability, cursorState MailboxCursorState, limit int) ([]*MailHeader, MailboxCursorState, error)
}

type MailboxRepository interface {
	CreateProviderAccount(ctx context.Context, account *ProviderAccount) (*ProviderAccount, error)
	GetProviderAccountByID(ctx context.Context, id int64) (*ProviderAccount, error)
	UpdateProviderAccount(ctx context.Context, account *ProviderAccount) (*ProviderAccount, error)
	ListProviderAccounts(ctx context.Context, opts MailboxListOptions) ([]*ProviderAccount, error)
	DeleteProviderAccount(ctx context.Context, id int64) error

	CreateCollector(ctx context.Context, collector *CollectorMailbox) (*CollectorMailbox, error)
	GetCollectorByID(ctx context.Context, id int64) (*CollectorMailbox, error)
	UpdateCollector(ctx context.Context, collector *CollectorMailbox) (*CollectorMailbox, error)
	ListCollectors(ctx context.Context, opts MailboxListOptions) ([]*CollectorMailbox, error)
	DeleteCollector(ctx context.Context, id int64) error

	CreateCapability(ctx context.Context, capability *MailboxCapability) (*MailboxCapability, error)
	GetCapabilityByID(ctx context.Context, id int64) (*MailboxCapability, error)
	UpdateCapability(ctx context.Context, capability *MailboxCapability) (*MailboxCapability, error)
	ListCapabilities(ctx context.Context, opts MailboxListOptions) ([]*MailboxCapability, error)
	DeleteCapability(ctx context.Context, id int64) error

	CreateRecipientIdentity(ctx context.Context, in *RecipientIdentity, values []*RecipientMatchValue) (*RecipientIdentity, error)
	GetRecipientIdentityByID(ctx context.Context, id int64) (*RecipientIdentity, error)
	UpdateRecipientIdentity(ctx context.Context, in *RecipientIdentity) (*RecipientIdentity, error)
	ListRecipientIdentities(ctx context.Context, opts MailboxListOptions) ([]*RecipientIdentity, error)
	DeleteRecipientIdentity(ctx context.Context, id int64) error
	ListRecipientMatchValues(ctx context.Context, recipientIdentityID int64) ([]*RecipientMatchValue, error)
	ListActiveRecipientMatchValues(ctx context.Context) ([]*RecipientMatchValue, error)
	ReplaceRecipientMatchValues(ctx context.Context, recipientIdentityID int64, values []*RecipientMatchValue) ([]*RecipientMatchValue, error)

	GetHeaderByID(ctx context.Context, id int64) (*MailHeader, error)
	ListHeaders(ctx context.Context, filter MailHeaderListFilter) ([]*MailHeader, int64, error)
	UpsertSyncHeaders(ctx context.Context, headers []*MailHeader) ([]*MailHeader, error)
	UpdateHeaderDetail(ctx context.Context, header *MailHeader) (*MailHeader, error)

	CreateSyncJobs(ctx context.Context, jobs []*MailSyncJob) ([]*MailSyncJob, error)
	ListSyncJobsByBatchID(ctx context.Context, batchID string) ([]*MailSyncJob, error)
	ListActiveSyncJobs(ctx context.Context, capabilityID *int64, limit int) ([]*MailSyncJob, error)
	UpdateSyncJobState(ctx context.Context, jobID int64, state string, startedAt, finishedAt, nextRetryAt *time.Time, errorSummary *string) (*MailSyncJob, error)

	ClaimDueCapabilities(ctx context.Context, now time.Time, limit int) ([]*MailboxCapability, error)
}
