package dto

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type MailboxProviderAccount struct {
	ID                  int64          `json:"id"`
	DisplayName         string         `json:"display_name"`
	ProviderKind        string         `json:"provider_kind"`
	AuthKind            string         `json:"auth_kind"`
	Status              string         `json:"status"`
	MailboxHint         *string        `json:"mailbox_hint"`
	ProviderIdentifier  *string        `json:"provider_identifier"`
	PayloadVersion      int            `json:"payload_version"`
	PayloadConfigured   bool           `json:"payload_configured"`
	PayloadSummary      map[string]any `json:"payload_summary,omitempty"`
	LastImportedAt      *time.Time     `json:"last_imported_at"`
	LastValidationAt    *time.Time     `json:"last_validation_at"`
	LastValidationError *string        `json:"last_validation_error"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           *time.Time     `json:"deleted_at"`
}

type CollectorMailbox struct {
	ID           int64               `json:"id"`
	EmailAddress string              `json:"email_address"`
	DisplayName  string              `json:"display_name"`
	Enabled      bool                `json:"enabled"`
	BusinessTags []string            `json:"business_tags"`
	Capabilities []MailboxCapability `json:"capabilities,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
	DeletedAt    *time.Time          `json:"deleted_at"`
}

type MailboxCapability struct {
	ID                   int64          `json:"id"`
	ProviderAccountID    int64          `json:"provider_account_id"`
	CollectorID          int64          `json:"collector_id"`
	CapabilityKind       string         `json:"capability_kind"`
	SyncEnabled          bool           `json:"sync_enabled"`
	SyncIntervalSeconds  int            `json:"sync_interval_seconds"`
	NextSyncAt           *time.Time     `json:"next_sync_at"`
	LastSyncAt           *time.Time     `json:"last_sync_at"`
	HealthState          string         `json:"health_state"`
	LastError            *string        `json:"last_error"`
	ConnectionConfigured bool           `json:"connection_configured"`
	ConnectionSummary    map[string]any `json:"connection_summary,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            *time.Time     `json:"deleted_at"`
}

type RecipientMatchValue struct {
	ID                  int64                                `json:"id"`
	RecipientIdentityID int64                                `json:"recipient_identity_id"`
	MatchType           string                               `json:"match_type"`
	MatchValue          string                               `json:"match_value"`
	NormalizedValue     string                               `json:"normalized_value"`
	Active              bool                                 `json:"active"`
	Priority            int                                  `json:"priority"`
	SourceKind          string                               `json:"source_kind"`
	SourceMetadata      service.RecipientMatchSourceMetadata `json:"source_metadata"`
	CreatedAt           time.Time                            `json:"created_at"`
	UpdatedAt           time.Time                            `json:"updated_at"`
	DisabledAt          *time.Time                           `json:"disabled_at"`
}

type RecipientIdentity struct {
	ID             int64                 `json:"id"`
	Name           string                `json:"name"`
	NormalizedName string                `json:"normalized_name"`
	Enabled        bool                  `json:"enabled"`
	MatchValues    []RecipientMatchValue `json:"match_values,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
	DeletedAt      *time.Time            `json:"deleted_at"`
}

type MailboxHeaderRecord struct {
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

type ProviderValidationOutcome struct {
	Account *MailboxProviderAccount `json:"account"`
	Code    string                  `json:"code"`
	Message string                  `json:"message"`
}

type CapabilityHealthResult struct {
	Success     bool   `json:"success"`
	HealthState string `json:"health_state"`
	Message     string `json:"message"`
}

type TestCapabilityResponse struct {
	Capability *MailboxCapability     `json:"capability"`
	Result     CapabilityHealthResult `json:"result"`
}

type BatchSyncStatusDTO struct {
	BatchID        string        `json:"batch_id"`
	QueuedCount    int           `json:"queued_count"`
	RunningCount   int           `json:"running_count"`
	SuccessCount   int           `json:"success_count"`
	PartialCount   int           `json:"partial_count"`
	FailureCount   int           `json:"failure_count"`
	CancelledCount int           `json:"cancelled_count,omitempty"`
	Jobs           []MailSyncJob `json:"jobs,omitempty"`
}

func MailboxProviderAccountFromService(account *service.ProviderAccount) *MailboxProviderAccount {
	if account == nil {
		return nil
	}
	return &MailboxProviderAccount{
		ID:                  account.ID,
		DisplayName:         account.DisplayName,
		ProviderKind:        account.ProviderKind,
		AuthKind:            account.AuthKind,
		Status:              account.Status,
		MailboxHint:         account.MailboxHint,
		ProviderIdentifier:  account.ProviderIdentifier,
		PayloadVersion:      account.PayloadVersion,
		PayloadConfigured:   strings.TrimSpace(account.EncryptedPayload) != "",
		PayloadSummary:      summarizeMaskedJSON(account.EncryptedPayload),
		LastImportedAt:      account.LastImportedAt,
		LastValidationAt:    account.LastValidationAt,
		LastValidationError: account.LastValidationError,
		CreatedAt:           account.CreatedAt,
		UpdatedAt:           account.UpdatedAt,
		DeletedAt:           account.DeletedAt,
	}
}

func CollectorMailboxFromService(collector *service.CollectorMailbox, capabilities []*service.MailboxCapability) *CollectorMailbox {
	if collector == nil {
		return nil
	}
	out := &CollectorMailbox{
		ID:           collector.ID,
		EmailAddress: collector.EmailAddress,
		DisplayName:  collector.DisplayName,
		Enabled:      collector.Enabled,
		BusinessTags: append([]string(nil), collector.BusinessTags...),
		CreatedAt:    collector.CreatedAt,
		UpdatedAt:    collector.UpdatedAt,
		DeletedAt:    collector.DeletedAt,
	}
	if len(capabilities) > 0 {
		out.Capabilities = make([]MailboxCapability, 0, len(capabilities))
		for _, capability := range capabilities {
			mapped := MailboxCapabilityFromService(capability)
			if mapped != nil {
				out.Capabilities = append(out.Capabilities, *mapped)
			}
		}
	}
	return out
}

func MailboxCapabilityFromService(capability *service.MailboxCapability) *MailboxCapability {
	if capability == nil {
		return nil
	}
	return &MailboxCapability{
		ID:                   capability.ID,
		ProviderAccountID:    capability.ProviderAccountID,
		CollectorID:          capability.CollectorID,
		CapabilityKind:       capability.CapabilityKind,
		SyncEnabled:          capability.SyncEnabled,
		SyncIntervalSeconds:  capability.SyncIntervalSeconds,
		NextSyncAt:           capability.NextSyncAt,
		LastSyncAt:           capability.LastSyncAt,
		HealthState:          capability.HealthState,
		LastError:            capability.LastError,
		ConnectionConfigured: len(capability.ConnectionConfig) > 0,
		ConnectionSummary:    summarizeMaskedValue(map[string]any(capability.ConnectionConfig)),
		CreatedAt:            capability.CreatedAt,
		UpdatedAt:            capability.UpdatedAt,
		DeletedAt:            capability.DeletedAt,
	}
}

func RecipientMatchValueFromService(value *service.RecipientMatchValue) *RecipientMatchValue {
	if value == nil {
		return nil
	}
	return &RecipientMatchValue{
		ID:                  value.ID,
		RecipientIdentityID: value.RecipientIdentityID,
		MatchType:           value.MatchType,
		MatchValue:          value.MatchValue,
		NormalizedValue:     value.NormalizedValue,
		Active:              value.Active,
		Priority:            value.Priority,
		SourceKind:          value.SourceKind,
		SourceMetadata:      value.SourceMetadata,
		CreatedAt:           value.CreatedAt,
		UpdatedAt:           value.UpdatedAt,
		DisabledAt:          value.DisabledAt,
	}
}

func RecipientIdentityFromService(identity *service.RecipientIdentity, values []*service.RecipientMatchValue) *RecipientIdentity {
	if identity == nil {
		return nil
	}
	out := &RecipientIdentity{
		ID:             identity.ID,
		Name:           identity.Name,
		NormalizedName: identity.NormalizedName,
		Enabled:        identity.Enabled,
		CreatedAt:      identity.CreatedAt,
		UpdatedAt:      identity.UpdatedAt,
		DeletedAt:      identity.DeletedAt,
	}
	if len(values) > 0 {
		out.MatchValues = make([]RecipientMatchValue, 0, len(values))
		for _, value := range values {
			mapped := RecipientMatchValueFromService(value)
			if mapped != nil {
				out.MatchValues = append(out.MatchValues, *mapped)
			}
		}
	}
	return out
}

func MailboxHeaderRecordFromService(header *service.MailHeader) *MailboxHeaderRecord {
	if header == nil {
		return nil
	}
	return &MailboxHeaderRecord{
		ID:                          header.ID,
		CollectorID:                 header.CollectorID,
		CapabilityID:                header.CapabilityID,
		RemoteMessageID:             header.RemoteMessageID,
		Folder:                      header.Folder,
		Sender:                      header.Sender,
		Recipients:                  append([]string(nil), header.Recipients...),
		Subject:                     header.Subject,
		ReceivedAt:                  header.ReceivedAt,
		Flags:                       append([]string(nil), header.Flags...),
		Snippet:                     header.Snippet,
		EnvelopeRecipients:          append([]string(nil), header.EnvelopeRecipients...),
		DeliveredTo:                 append([]string(nil), header.DeliveredTo...),
		OriginalTo:                  append([]string(nil), header.OriginalTo...),
		ResolvedRecipientIdentityID: header.ResolvedRecipientIdentityID,
		ResolvedAddress:             header.ResolvedAddress,
		MatchType:                   header.MatchType,
		MatchedValueID:              header.MatchedValueID,
		ResolutionSourceField:       header.ResolutionSourceField,
		ResolutionState:             header.ResolutionState,
		DetailFetchState:            header.DetailFetchState,
		CreatedAt:                   header.CreatedAt,
		UpdatedAt:                   header.UpdatedAt,
	}
}

func MailSyncJobFromService(job *service.MailSyncJob) *MailSyncJob {
	if job == nil {
		return nil
	}
	return &MailSyncJob{
		ID:            job.ID,
		CapabilityID:  job.CapabilityID,
		BatchID:       job.BatchID,
		State:         job.State,
		TriggerSource: job.TriggerSource,
		ScheduledFor:  job.ScheduledFor,
		StartedAt:     job.StartedAt,
		FinishedAt:    job.FinishedAt,
		Retryable:     job.Retryable,
		RetryCount:    job.RetryCount,
		NextRetryAt:   job.NextRetryAt,
		ErrorSummary:  job.ErrorSummary,
		CreatedAt:     job.CreatedAt,
		UpdatedAt:     job.UpdatedAt,
	}
}

func ProviderValidationOutcomeFromService(outcome *service.ProviderValidationOutcome) *ProviderValidationOutcome {
	if outcome == nil {
		return nil
	}
	return &ProviderValidationOutcome{
		Account: MailboxProviderAccountFromService(outcome.Account),
		Code:    outcome.Code,
		Message: outcome.Message,
	}
}

func BatchSyncStatusFromJobs(batchID string, jobs []*service.MailSyncJob) BatchSyncStatusDTO {
	out := BatchSyncStatusDTO{BatchID: batchID}
	if len(jobs) == 0 {
		return out
	}
	out.Jobs = make([]MailSyncJob, 0, len(jobs))
	for _, job := range jobs {
		mapped := MailSyncJobFromService(job)
		if mapped == nil {
			continue
		}
		out.Jobs = append(out.Jobs, *mapped)
		switch job.State {
		case service.MailSyncJobStateQueued:
			out.QueuedCount++
		case service.MailSyncJobStateRunning:
			out.RunningCount++
		case service.MailSyncJobStateSucceeded:
			out.SuccessCount++
		case service.MailSyncJobStatePartial:
			out.PartialCount++
		case service.MailSyncJobStateFailed:
			out.FailureCount++
		case service.MailSyncJobStateCancelled:
			out.CancelledCount++
		}
	}
	return out
}

func summarizeMaskedJSON(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return map[string]any{"configured": true}
	}
	return summarizeMaskedValue(payload)
}

func summarizeMaskedValue(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	out := make(map[string]any, len(payload))
	for key, value := range payload {
		out[key] = maskMailboxValue(value)
	}
	return out
}

func maskMailboxValue(value any) any {
	switch typed := value.(type) {
	case string:
		return maskSecretString(typed)
	case bool, float64, float32, int, int64, int32, uint, uint64, uint32:
		return typed
	case []string:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, maskSecretString(item))
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, maskMailboxValue(item))
		}
		return out
	case map[string]any:
		return summarizeMaskedValue(typed)
	default:
		return "[configured]"
	}
}

func maskSecretString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "@") {
		parts := strings.SplitN(value, "@", 2)
		local := parts[0]
		if len(local) > 1 {
			local = local[:1] + "***"
		} else {
			local = "***"
		}
		return local + "@" + parts[1]
	}
	if len(value) <= 4 {
		return "***"
	}
	return value[:2] + "***" + value[len(value)-2:]
}
