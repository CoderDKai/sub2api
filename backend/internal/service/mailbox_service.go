package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	mailboxpkg "github.com/Wei-Shaw/sub2api/internal/pkg/mailbox"
)

const (
	MailboxProviderKindBasic     = "basic"
	MailboxProviderKindMicrosoft = "microsoft"
)

var (
	ErrMailboxUnsupportedProvider = errors.New("mailbox unsupported provider")
	ErrMailboxUnsupportedAuthKind = errors.New("mailbox unsupported auth kind")
	ErrMailboxPayloadRequired     = errors.New("mailbox payload is required")
)

type MailboxProviderUpsertInput struct {
	DisplayName      string
	ProviderKind     string
	AuthKind         string
	EncryptedPayload string
	Status           string
}

type ProviderValidationOutcome struct {
	Account *ProviderAccount
	Code    string
	Message string
}

type MailboxService struct {
	repo      MailboxRepository
	audit     MailboxAuditor
	providers map[string]mailboxpkg.ProviderClient
	now       func() time.Time
}

func NewMailboxService(repo MailboxRepository, basicClient *mailboxpkg.BasicClient, microsoftClient *mailboxpkg.MicrosoftClient, audit MailboxAuditor) *MailboxService {
	providers := map[string]mailboxpkg.ProviderClient{}
	if basicClient != nil {
		providers[MailboxProviderKindBasic] = basicClient
	}
	if microsoftClient != nil {
		providers[MailboxProviderKindMicrosoft] = microsoftClient
	}
	return newMailboxServiceWithProviders(repo, audit, providers)
}

func newMailboxServiceWithProviders(repo MailboxRepository, audit MailboxAuditor, providers map[string]mailboxpkg.ProviderClient) *MailboxService {
	if audit == nil {
		audit = NewMailboxAuditLogger()
	}
	if providers == nil {
		providers = map[string]mailboxpkg.ProviderClient{}
	}
	return &MailboxService{
		repo:      repo,
		audit:     audit,
		providers: providers,
		now:       time.Now,
	}
}

func (s *MailboxService) CreateProvider(ctx context.Context, input MailboxProviderUpsertInput) (*ProviderAccount, error) {
	account, imported, err := s.prepareProviderAccount(nil, input)
	if err != nil {
		return nil, err
	}
	created, err := s.repo.CreateProviderAccount(ctx, account)
	if err != nil {
		return nil, err
	}
	s.audit.RecordProviderCreate(ctx, created)
	if imported {
		s.audit.RecordProviderImport(ctx, created)
	}
	return created, nil
}

func (s *MailboxService) UpdateProvider(ctx context.Context, id int64, input MailboxProviderUpsertInput) (*ProviderAccount, error) {
	if err := validateProviderSupport(strings.TrimSpace(input.ProviderKind), strings.TrimSpace(input.AuthKind)); err != nil {
		return nil, err
	}
	current, err := s.repo.GetProviderAccountByID(ctx, id)
	if err != nil {
		return nil, err
	}
	updated, imported, err := s.prepareProviderAccount(current, input)
	if err != nil {
		return nil, err
	}
	updated.ID = id
	persisted, err := s.repo.UpdateProviderAccount(ctx, updated)
	if err != nil {
		return nil, err
	}
	s.audit.RecordProviderUpdate(ctx, current, persisted)
	if imported {
		s.audit.RecordProviderImport(ctx, persisted)
	}
	return persisted, nil
}

func (s *MailboxService) ValidateProvider(ctx context.Context, providerID int64) (*ProviderValidationOutcome, error) {
	account, err := s.repo.GetProviderAccountByID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	client, err := s.providerClient(account.ProviderKind)
	if err != nil {
		return nil, err
	}
	profile, err := buildProviderProfile(account)
	result := &mailboxpkg.ValidationResult{Code: mailboxpkg.ValidationCodeOK}
	if err != nil {
		result = &mailboxpkg.ValidationResult{
			Code:              mailboxpkg.ValidationCodeInvalidFormat,
			Message:           err.Error(),
			InvalidateAccount: true,
		}
	} else {
		result, err = client.Validate(ctx, profile)
		if err != nil {
			result = &mailboxpkg.ValidationResult{Code: mailboxpkg.ValidationCodeValidationFailed, Message: err.Error()}
		}
	}
	if result == nil {
		result = &mailboxpkg.ValidationResult{Code: mailboxpkg.ValidationCodeOK}
	}
	if strings.TrimSpace(result.Code) == "" {
		result.Code = mailboxpkg.ValidationCodeOK
	}

	updated := cloneProviderAccountValue(account)
	now := s.now().UTC()
	updated.LastValidationAt = &now
	if result.Code == mailboxpkg.ValidationCodeOK {
		updated.Status = ProviderAccountStatusActive
		updated.LastValidationError = nil
		if strings.TrimSpace(result.ProviderIdentifier) != "" {
			updated.ProviderIdentifier = stringPtr(strings.TrimSpace(result.ProviderIdentifier))
		}
		if strings.TrimSpace(result.MailboxIdentifier) != "" {
			updated.MailboxHint = stringPtr(strings.TrimSpace(result.MailboxIdentifier))
		}
	} else {
		message := strings.TrimSpace(result.Message)
		if message == "" {
			message = result.Code
		}
		updated.LastValidationError = stringPtr(message)
		if result.InvalidateAccount {
			updated.Status = ProviderAccountStatusInvalid
		}
	}
	persisted, err := s.repo.UpdateProviderAccount(ctx, updated)
	if err != nil {
		return nil, err
	}
	if persisted.Status != account.Status || result.Code != mailboxpkg.ValidationCodeOK {
		s.audit.RecordProviderStatus(ctx, persisted, result.Code)
	}
	return &ProviderValidationOutcome{Account: persisted, Code: result.Code, Message: result.Message}, nil
}

func (s *MailboxService) TestCapability(ctx context.Context, capabilityID int64) (*MailboxCapability, error) {
	capability, err := s.repo.GetCapabilityByID(ctx, capabilityID)
	if err != nil {
		return nil, err
	}
	account, err := s.repo.GetProviderAccountByID(ctx, capability.ProviderAccountID)
	if err != nil {
		return nil, err
	}
	client, err := s.providerClient(account.ProviderKind)
	if err != nil {
		return nil, err
	}
	profile, err := buildProviderProfile(account)
	if err != nil {
		return nil, err
	}
	_, listErr := client.ListHeaders(ctx, profile, mailboxpkg.CapabilityProfile{
		Kind:             capability.CapabilityKind,
		ConnectionConfig: mapFromMailboxConnectionConfig(capability.ConnectionConfig),
		CursorState:      mapFromMailboxCursorState(capability.CursorState),
	}, 1)

	updated := cloneMailboxCapabilityValue(capability)
	now := s.now().UTC()
	success := listErr == nil
	if success {
		updated.HealthState = MailboxCapabilityStateHealthy
		updated.LastError = nil
		updated.LastSyncAt = &now
	} else {
		message := listErr.Error()
		updated.HealthState = MailboxCapabilityStateError
		updated.LastError = &message
	}
	persisted, err := s.repo.UpdateCapability(ctx, updated)
	if err != nil {
		return nil, err
	}
	s.audit.RecordCapabilityTest(ctx, persisted, success)
	return persisted, nil
}

func (s *MailboxService) prepareProviderAccount(current *ProviderAccount, input MailboxProviderUpsertInput) (*ProviderAccount, bool, error) {
	providerKind := strings.TrimSpace(input.ProviderKind)
	authKind := strings.TrimSpace(input.AuthKind)
	if err := validateProviderSupport(providerKind, authKind); err != nil {
		return nil, false, err
	}
	payload := strings.TrimSpace(input.EncryptedPayload)
	if payload == "" {
		return nil, false, ErrMailboxPayloadRequired
	}
	account := &ProviderAccount{}
	if current != nil {
		account = cloneProviderAccountValue(current)
	}
	account.DisplayName = strings.TrimSpace(input.DisplayName)
	account.ProviderKind = providerKind
	account.AuthKind = authKind
	if status := strings.TrimSpace(input.Status); status != "" {
		account.Status = status
	} else if current == nil {
		account.Status = ProviderAccountStatusDraft
	}
	if authKind == ProviderAuthKindImportBundle {
		bundle, err := mailboxpkg.ParseOutlookImportBundle(payload)
		if err != nil {
			return nil, false, err
		}
		encodedPayload, err := json.Marshal(map[string]any{
			"mailbox_identifier":  bundle.MailboxIdentifier,
			"provider_identifier": bundle.ProviderIdentifier,
			"token_bundle":        bundle.TokenBundle,
		})
		if err != nil {
			return nil, false, err
		}
		account.EncryptedPayload = string(encodedPayload)
		account.MailboxHint = stringPtr(bundle.MailboxIdentifier)
		account.ProviderIdentifier = stringPtr(bundle.ProviderIdentifier)
		now := s.now().UTC()
		account.LastImportedAt = &now
		return account, true, nil
	}
	account.EncryptedPayload = payload
	return account, false, nil
}

func (s *MailboxService) providerClient(providerKind string) (mailboxpkg.ProviderClient, error) {
	client, ok := s.providers[strings.TrimSpace(providerKind)]
	if !ok || client == nil {
		return nil, ErrMailboxUnsupportedProvider
	}
	return client, nil
}

func validateProviderSupport(providerKind, authKind string) error {
	switch providerKind {
	case MailboxProviderKindBasic:
		switch authKind {
		case ProviderAuthKindBasic, ProviderAuthKindIMAPPassword, ProviderAuthKindPOP3Password:
			return nil
		default:
			return ErrMailboxUnsupportedAuthKind
		}
	case MailboxProviderKindMicrosoft:
		switch authKind {
		case ProviderAuthKindOAuth2, ProviderAuthKindImportBundle:
			return nil
		default:
			return ErrMailboxUnsupportedAuthKind
		}
	default:
		return ErrMailboxUnsupportedProvider
	}
}

func buildProviderProfile(account *ProviderAccount) (mailboxpkg.ProviderProfile, error) {
	payload := make(map[string]any)
	if strings.TrimSpace(account.EncryptedPayload) != "" {
		if err := json.Unmarshal([]byte(account.EncryptedPayload), &payload); err != nil {
			return mailboxpkg.ProviderProfile{}, fmt.Errorf("decode provider payload: %w", err)
		}
	}
	profile := mailboxpkg.ProviderProfile{
		ProviderKind: account.ProviderKind,
		AuthKind:     account.AuthKind,
		Payload:      payload,
	}
	if account.MailboxHint != nil {
		profile.MailboxIdentifier = strings.TrimSpace(*account.MailboxHint)
	}
	if account.ProviderIdentifier != nil {
		profile.ProviderIdentifier = strings.TrimSpace(*account.ProviderIdentifier)
	}
	return profile, nil
}

func mapFromMailboxConnectionConfig(in MailboxConnectionConfig) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mapFromMailboxCursorState(in MailboxCursorState) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneProviderAccountValue(in *ProviderAccount) *ProviderAccount {
	if in == nil {
		return nil
	}
	clone := *in
	clone.MailboxHint = cloneStringPointer(in.MailboxHint)
	clone.ProviderIdentifier = cloneStringPointer(in.ProviderIdentifier)
	clone.LastImportedAt = cloneTimePointer(in.LastImportedAt)
	clone.LastValidationAt = cloneTimePointer(in.LastValidationAt)
	clone.LastValidationError = cloneStringPointer(in.LastValidationError)
	clone.DeletedAt = cloneTimePointer(in.DeletedAt)
	return &clone
}

func cloneMailboxCapabilityValue(in *MailboxCapability) *MailboxCapability {
	if in == nil {
		return nil
	}
	clone := *in
	clone.ConnectionConfig = MailboxConnectionConfig(mapFromMailboxConnectionConfig(in.ConnectionConfig))
	clone.CursorState = MailboxCursorState(mapFromMailboxCursorState(in.CursorState))
	clone.NextSyncAt = cloneTimePointer(in.NextSyncAt)
	clone.LastSyncAt = cloneTimePointer(in.LastSyncAt)
	clone.LastError = cloneStringPointer(in.LastError)
	clone.DeletedAt = cloneTimePointer(in.DeletedAt)
	return &clone
}

func cloneStringPointer(in *string) *string {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneTimePointer(in *time.Time) *time.Time {
	if in == nil {
		return nil
	}
	out := in.UTC()
	return &out
}

func stringPtr(v string) *string {
	return &v
}
