package service

import (
	"context"
	"time"
)

type ProviderAccount struct {
	ID                int64      `json:"id"`
	ProviderKind      string     `json:"provider_kind"`
	ExternalAccountID string     `json:"external_account_id"`
	EncryptedPayload  string     `json:"encrypted_payload"`
	PayloadVersion    int        `json:"payload_version"`
	ImportCursor      string     `json:"import_cursor"`
	LastImportedAt    *time.Time `json:"last_imported_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `json:"deleted_at"`
}

type CollectorMailbox struct {
	ID                int64      `json:"id"`
	ProviderAccountID int64      `json:"provider_account_id"`
	EmailAddress      string     `json:"email_address"`
	DisplayName       string     `json:"display_name"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `json:"deleted_at"`
}

type MailboxCapability struct {
	ID             int64      `json:"id"`
	CollectorID    int64      `json:"collector_id"`
	CapabilityKind string     `json:"capability_kind"`
	Folder         string     `json:"folder"`
	SyncState      string     `json:"sync_state"`
	ImportCursor   string     `json:"import_cursor"`
	LastSyncedAt   *time.Time `json:"last_synced_at"`
	NextSyncDueAt  *time.Time `json:"next_sync_due_at"`
	LastError      string     `json:"last_error"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at"`
}

type RecipientIdentity struct {
	ID             int64      `json:"id"`
	CollectorID    int64      `json:"collector_id"`
	Name           string     `json:"name"`
	NormalizedName string     `json:"normalized_name"`
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
	ID                    int64      `json:"id"`
	CapabilityID          int64      `json:"capability_id"`
	MatchedValueID        *int64     `json:"matched_value_id"`
	Folder                string     `json:"folder"`
	RemoteMessageID       string     `json:"remote_message_id"`
	MessageID             string     `json:"message_id"`
	ReceivedAt            time.Time  `json:"received_at"`
	Snippet               string     `json:"snippet"`
	Subject               string     `json:"subject"`
	FromAddress           string     `json:"from_address"`
	ResolvedAddress       string     `json:"resolved_address"`
	EnvelopeRecipients    []string   `json:"envelope_recipients"`
	DeliveredTo           string     `json:"delivered_to"`
	OriginalTo            string     `json:"original_to"`
	MatchType             string     `json:"match_type"`
	ResolutionSourceField string     `json:"resolution_source_field"`
	ResolutionState       string     `json:"resolution_state"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type MailSyncJob struct {
	ID             int64      `json:"id"`
	CapabilityID   int64      `json:"capability_id"`
	BatchID        string     `json:"batch_id"`
	Status         string     `json:"status"`
	SyncReason     string     `json:"sync_reason"`
	ScheduledFor   time.Time  `json:"scheduled_for"`
	StartedAt      *time.Time `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	Retryable      bool       `json:"retryable"`
	RetryCount     int        `json:"retry_count"`
	NextRetryAt    *time.Time `json:"next_retry_at"`
	BackoffSeconds int        `json:"backoff_seconds"`
	LastError      string     `json:"last_error"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type MailboxRepository interface {
	CreateProviderAccount(ctx context.Context, account *ProviderAccount) (*ProviderAccount, error)
	UpdateProviderAccount(ctx context.Context, account *ProviderAccount) (*ProviderAccount, error)
	ListCollectorsByProviderAccountID(ctx context.Context, providerAccountID int64) ([]*CollectorMailbox, error)
	CreateCollector(ctx context.Context, mailbox *CollectorMailbox) (*CollectorMailbox, error)
	UpsertCapabilities(ctx context.Context, capabilities []*MailboxCapability) error
	ListCapabilitiesDue(ctx context.Context, now time.Time, limit int) ([]*MailboxCapability, error)
	CreateRecipientIdentity(ctx context.Context, identity *RecipientIdentity) (*RecipientIdentity, error)
	ReplaceRecipientMatchValues(ctx context.Context, recipientIdentityID int64, values []*RecipientMatchValue) error
	UpsertHeaders(ctx context.Context, headers []*MailHeader) error
	CreateSyncJobs(ctx context.Context, jobs []*MailSyncJob) ([]*MailSyncJob, error)
	ListSyncJobsByBatchID(ctx context.Context, batchID string) ([]*MailSyncJob, error)
}
