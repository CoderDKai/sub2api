package admin

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"sort"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	mailboxpkg "github.com/Wei-Shaw/sub2api/internal/pkg/mailbox"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const defaultMailboxSyncIntervalSeconds = 300
const mailboxAdminListBatchSize = 200

type mailboxProviderService interface {
	CreateProvider(ctx context.Context, input service.MailboxProviderUpsertInput) (*service.ProviderAccount, error)
	ValidateProvider(ctx context.Context, providerID int64) (*service.ProviderValidationOutcome, error)
	TestCapability(ctx context.Context, capabilityID int64) (*service.MailboxCapability, error)
}

type mailboxSyncService interface {
	CreateBatchSyncJobs(ctx context.Context, input service.MailboxBatchSyncRequest) ([]*service.MailSyncJob, error)
	FetchDetail(ctx context.Context, headerID int64) (*service.MailHeader, error)
}

type MailboxHandler struct {
	providerService mailboxProviderService
	syncService     mailboxSyncService
	repo            service.MailboxRepository
}

type CreateProviderRequest struct {
	DisplayName      string `json:"display_name" binding:"required"`
	ProviderKind     string `json:"provider_kind" binding:"required"`
	AuthKind         string `json:"auth_kind" binding:"required"`
	EncryptedPayload string `json:"encrypted_payload" binding:"required"`
	Status           string `json:"status" binding:"omitempty,oneof=draft active invalid disabled"`
}

type CapabilityInput struct {
	CapabilityKind      string         `json:"capability_kind" binding:"required"`
	ConnectionConfig    map[string]any `json:"connection_config"`
	SyncEnabled         *bool          `json:"sync_enabled"`
	SyncIntervalSeconds int            `json:"sync_interval_seconds"`
}

type UpsertCollectorRequest struct {
	EmailAddress      string            `json:"email_address" binding:"required"`
	DisplayName       string            `json:"display_name"`
	Enabled           *bool             `json:"enabled"`
	BusinessTags      []string          `json:"business_tags"`
	ProviderAccountID *int64            `json:"provider_account_id"`
	Capabilities      []CapabilityInput `json:"capabilities"`
}

type UpdateCollectorRequest struct {
	EmailAddress string   `json:"email_address" binding:"required"`
	DisplayName  string   `json:"display_name"`
	Enabled      *bool    `json:"enabled"`
	BusinessTags []string `json:"business_tags"`
}

type CreateCapabilityRequest struct {
	ProviderAccountID   int64          `json:"provider_account_id" binding:"required"`
	CollectorID         int64          `json:"collector_id" binding:"required"`
	CapabilityKind      string         `json:"capability_kind" binding:"required"`
	ConnectionConfig    map[string]any `json:"connection_config"`
	SyncEnabled         *bool          `json:"sync_enabled"`
	SyncIntervalSeconds int            `json:"sync_interval_seconds"`
}

type UpdateCapabilityRequest struct {
	ConnectionConfig    map[string]any `json:"connection_config"`
	SyncEnabled         *bool          `json:"sync_enabled"`
	SyncIntervalSeconds int            `json:"sync_interval_seconds"`
	HealthState         string         `json:"health_state" binding:"omitempty,oneof=healthy warning error paused syncing"`
}

type RecipientMatchValueInput struct {
	MatchType      string         `json:"match_type" binding:"required,oneof=exact_address domain_suffix"`
	MatchValue     string         `json:"match_value" binding:"required"`
	Priority       *int           `json:"priority"`
	Active         *bool          `json:"active"`
	SourceKind     string         `json:"source_kind"`
	SourceMetadata map[string]any `json:"source_metadata"`
}

type CreateRecipientRequest struct {
	Name        string                     `json:"name" binding:"required"`
	Enabled     *bool                      `json:"enabled"`
	MatchValues []RecipientMatchValueInput `json:"match_values" binding:"required,min=1"`
}

type UpdateRecipientRequest struct {
	Name        string                     `json:"name" binding:"required"`
	Enabled     *bool                      `json:"enabled"`
	MatchValues []RecipientMatchValueInput `json:"match_values" binding:"required,min=1"`
}

type BatchSyncCollectorsRequest struct {
	CollectorIDs  []int64 `json:"collector_ids"`
	CapabilityIDs []int64 `json:"capability_ids"`
}

type ImportRecipientExactAddressesRequest struct {
	Addresses []string `json:"addresses" binding:"required,min=1"`
}

type UpdateProviderStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=draft active invalid disabled"`
}

type UpdateCollectorStatusRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

type UpdateCapabilityStatusRequest struct {
	SyncEnabled *bool  `json:"sync_enabled"`
	HealthState string `json:"health_state" binding:"omitempty,oneof=healthy warning error paused syncing"`
}

type UpdateRecipientStatusRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

type ListInboxRequest struct {
	CollectorID  *int64 `form:"collector_id"`
	CapabilityID *int64 `form:"capability_id"`
	Folder       string `form:"folder"`
	Page         int    `form:"page"`
	PageSize     int    `form:"page_size"`
}

func NewMailboxHandler(providerService mailboxProviderService, syncService mailboxSyncService, repo service.MailboxRepository) *MailboxHandler {
	return &MailboxHandler{
		providerService: providerService,
		syncService:     syncService,
		repo:            repo,
	}
}

func (h *MailboxHandler) ListProviders(c *gin.Context) {
	accounts, err := collectAllMailboxItems(func(opts service.MailboxListOptions) ([]*service.ProviderAccount, error) {
		return h.repo.ListProviderAccounts(c.Request.Context(), opts)
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.MailboxProviderAccount, 0, len(accounts))
	for _, account := range accounts {
		mapped := dto.MailboxProviderAccountFromService(account)
		if mapped != nil {
			out = append(out, *mapped)
		}
	}
	respondMailboxPage(c, out)
}

func (h *MailboxHandler) CreateProvider(c *gin.Context) {
	if h.providerService == nil {
		response.Error(c, http.StatusServiceUnavailable, "mailbox provider service unavailable")
		return
	}
	var req CreateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	account, err := h.providerService.CreateProvider(c.Request.Context(), service.MailboxProviderUpsertInput{
		DisplayName:      strings.TrimSpace(req.DisplayName),
		ProviderKind:     strings.TrimSpace(req.ProviderKind),
		AuthKind:         strings.TrimSpace(req.AuthKind),
		EncryptedPayload: strings.TrimSpace(req.EncryptedPayload),
		Status:           strings.TrimSpace(req.Status),
	})
	if err != nil {
		if isMailboxBadRequestError(err) {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.MailboxProviderAccountFromService(account))
}

func (h *MailboxHandler) ValidateProvider(c *gin.Context) {
	if h.providerService == nil {
		response.Error(c, http.StatusServiceUnavailable, "mailbox provider service unavailable")
		return
	}
	id, ok := parseMailboxIDParam(c, "id")
	if !ok {
		return
	}
	outcome, err := h.providerService.ValidateProvider(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.ProviderValidationOutcomeFromService(outcome))
}

func (h *MailboxHandler) UpdateProviderStatus(c *gin.Context) {
	var req UpdateProviderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	id, ok := parseMailboxIDParam(c, "id")
	if !ok {
		return
	}
	current, err := h.repo.GetProviderAccountByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	current.Status = req.Status
	updated, err := h.repo.UpdateProviderAccount(c.Request.Context(), current)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.MailboxProviderAccountFromService(updated))
}

func (h *MailboxHandler) ListCollectors(c *gin.Context) {
	collectors, err := collectAllMailboxItems(func(opts service.MailboxListOptions) ([]*service.CollectorMailbox, error) {
		return h.repo.ListCollectors(c.Request.Context(), opts)
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	capabilities, err := collectAllMailboxItems(func(opts service.MailboxListOptions) ([]*service.MailboxCapability, error) {
		return h.repo.ListCapabilities(c.Request.Context(), opts)
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	byCollector := make(map[int64][]*service.MailboxCapability)
	for _, capability := range capabilities {
		if capability == nil {
			continue
		}
		byCollector[capability.CollectorID] = append(byCollector[capability.CollectorID], capability)
	}
	out := make([]dto.CollectorMailbox, 0, len(collectors))
	for _, collector := range collectors {
		mapped := dto.CollectorMailboxFromService(collector, byCollector[collector.ID])
		if mapped != nil {
			out = append(out, *mapped)
		}
	}
	respondMailboxPage(c, out)
}

func (h *MailboxHandler) UpsertCollector(c *gin.Context) {
	var req UpsertCollectorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	emailAddress, err := normalizeAndValidateMailboxAddress(req.EmailAddress)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if len(req.Capabilities) > 0 && (req.ProviderAccountID == nil || *req.ProviderAccountID <= 0) {
		response.BadRequest(c, "provider_account_id is required when capabilities are present")
		return
	}
	for _, capabilityInput := range req.Capabilities {
		if _, err := normalizeCapabilityKind(capabilityInput.CapabilityKind); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	collector, err := h.repo.CreateCollector(c.Request.Context(), &service.CollectorMailbox{
		EmailAddress: emailAddress,
		DisplayName:  strings.TrimSpace(req.DisplayName),
		Enabled:      enabled,
		BusinessTags: normalizeStringSlice(req.BusinessTags),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	createdCapabilities := make([]*service.MailboxCapability, 0, len(req.Capabilities))
	for _, capabilityInput := range req.Capabilities {
		syncEnabled := true
		if capabilityInput.SyncEnabled != nil {
			syncEnabled = *capabilityInput.SyncEnabled
		}
		healthState := service.MailboxCapabilityStateHealthy
		if !syncEnabled {
			healthState = service.MailboxCapabilityStatePaused
		}
		interval := capabilityInput.SyncIntervalSeconds
		if interval <= 0 {
			interval = defaultMailboxSyncIntervalSeconds
		}
		created, createErr := h.repo.CreateCapability(c.Request.Context(), &service.MailboxCapability{
			ProviderAccountID:   *req.ProviderAccountID,
			CollectorID:         collector.ID,
			CapabilityKind:      strings.TrimSpace(capabilityInput.CapabilityKind),
			ConnectionConfig:    service.MailboxConnectionConfig(capabilityInput.ConnectionConfig),
			SyncEnabled:         syncEnabled,
			SyncIntervalSeconds: interval,
			HealthState:         healthState,
		})
		if createErr != nil {
			if rollbackErr := h.repo.DeleteCollector(c.Request.Context(), collector.ID); rollbackErr != nil {
				response.ErrorFrom(c, errors.Join(createErr, rollbackErr))
				return
			}
			response.ErrorFrom(c, createErr)
			return
		}
		createdCapabilities = append(createdCapabilities, created)
	}
	response.Created(c, dto.CollectorMailboxFromService(collector, createdCapabilities))
}

func (h *MailboxHandler) SyncCollector(c *gin.Context) {
	if h.syncService == nil {
		response.Error(c, http.StatusServiceUnavailable, "mailbox sync service unavailable")
		return
	}
	id, ok := parseMailboxIDParam(c, "id")
	if !ok {
		return
	}
	jobs, err := h.syncService.CreateBatchSyncJobs(c.Request.Context(), service.MailboxBatchSyncRequest{
		CollectorIDs:  []int64{id},
		TriggerSource: service.MailSyncTriggerSourceManual,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Accepted(c, gin.H{"batch_id": mailboxBatchIDFromJobs(jobs)})
}

func (h *MailboxHandler) BatchSyncCollectors(c *gin.Context) {
	if h.syncService == nil {
		response.Error(c, http.StatusServiceUnavailable, "mailbox sync service unavailable")
		return
	}
	var req BatchSyncCollectorsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if !hasExactlyOneMailboxTarget(req.CollectorIDs, req.CapabilityIDs) {
		response.BadRequest(c, "exactly one of collector_ids or capability_ids is required")
		return
	}
	jobs, err := h.syncService.CreateBatchSyncJobs(c.Request.Context(), service.MailboxBatchSyncRequest{
		CollectorIDs:  normalizePositiveIDs(req.CollectorIDs),
		CapabilityIDs: normalizePositiveIDs(req.CapabilityIDs),
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Accepted(c, gin.H{"batch_id": mailboxBatchIDFromJobs(jobs)})
}

func (h *MailboxHandler) UpdateCollectorStatus(c *gin.Context) {
	var req UpdateCollectorStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	id, ok := parseMailboxIDParam(c, "id")
	if !ok {
		return
	}
	collector, err := h.repo.GetCollectorByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	collector.Enabled = *req.Enabled
	updated, err := h.repo.UpdateCollector(c.Request.Context(), collector)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.CollectorMailboxFromService(updated, nil))
}

func (h *MailboxHandler) UpdateCollector(c *gin.Context) {
	var req UpdateCollectorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	emailAddress, err := normalizeAndValidateMailboxAddress(req.EmailAddress)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	id, ok := parseMailboxIDParam(c, "id")
	if !ok {
		return
	}
	collector, err := h.repo.GetCollectorByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	collector.EmailAddress = emailAddress
	collector.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.Enabled != nil {
		collector.Enabled = *req.Enabled
	}
	collector.BusinessTags = normalizeStringSlice(req.BusinessTags)
	updated, err := h.repo.UpdateCollector(c.Request.Context(), collector)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	capabilities, err := h.listCollectorCapabilities(c.Request.Context(), updated.ID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.CollectorMailboxFromService(updated, capabilities))
}

func (h *MailboxHandler) ListCapabilities(c *gin.Context) {
	capabilities, err := collectAllMailboxItems(func(opts service.MailboxListOptions) ([]*service.MailboxCapability, error) {
		return h.repo.ListCapabilities(c.Request.Context(), opts)
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.MailboxCapability, 0, len(capabilities))
	for _, capability := range capabilities {
		mapped := dto.MailboxCapabilityFromService(capability)
		if mapped != nil {
			out = append(out, *mapped)
		}
	}
	respondMailboxPage(c, out)
}

func (h *MailboxHandler) TestCapability(c *gin.Context) {
	if h.providerService == nil {
		response.Error(c, http.StatusServiceUnavailable, "mailbox provider service unavailable")
		return
	}
	id, ok := parseMailboxIDParam(c, "id")
	if !ok {
		return
	}
	capability, err := h.providerService.TestCapability(c.Request.Context(), id)
	if err != nil && capability == nil {
		response.ErrorFrom(c, err)
		return
	}
	result := dto.CapabilityHealthResult{Success: err == nil}
	if capability != nil {
		result.HealthState = capability.HealthState
	}
	if err != nil {
		result.Message = err.Error()
	}
	response.Success(c, dto.TestCapabilityResponse{
		Capability: dto.MailboxCapabilityFromService(capability),
		Result:     result,
	})
}

func (h *MailboxHandler) CreateCapability(c *gin.Context) {
	var req CreateCapabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	capabilityKind, err := normalizeCapabilityKind(req.CapabilityKind)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	syncEnabled := true
	if req.SyncEnabled != nil {
		syncEnabled = *req.SyncEnabled
	}
	interval := req.SyncIntervalSeconds
	if interval <= 0 {
		interval = defaultMailboxSyncIntervalSeconds
	}
	healthState := service.MailboxCapabilityStateHealthy
	if !syncEnabled {
		healthState = service.MailboxCapabilityStatePaused
	}
	created, err := h.repo.CreateCapability(c.Request.Context(), &service.MailboxCapability{
		ProviderAccountID:   req.ProviderAccountID,
		CollectorID:         req.CollectorID,
		CapabilityKind:      capabilityKind,
		ConnectionConfig:    service.MailboxConnectionConfig(req.ConnectionConfig),
		SyncEnabled:         syncEnabled,
		SyncIntervalSeconds: interval,
		HealthState:         healthState,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.MailboxCapabilityFromService(created))
}

func (h *MailboxHandler) UpdateCapabilityStatus(c *gin.Context) {
	var req UpdateCapabilityStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.SyncEnabled == nil && strings.TrimSpace(req.HealthState) == "" {
		response.BadRequest(c, "at least one of sync_enabled or health_state is required")
		return
	}
	id, ok := parseMailboxIDParam(c, "id")
	if !ok {
		return
	}
	capability, err := h.repo.GetCapabilityByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if req.SyncEnabled != nil {
		capability.SyncEnabled = *req.SyncEnabled
		if !*req.SyncEnabled && strings.TrimSpace(req.HealthState) == "" {
			capability.HealthState = service.MailboxCapabilityStatePaused
		}
	}
	if state := strings.TrimSpace(req.HealthState); state != "" {
		capability.HealthState = state
	}
	updated, err := h.repo.UpdateCapability(c.Request.Context(), capability)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.MailboxCapabilityFromService(updated))
}

func (h *MailboxHandler) UpdateCapability(c *gin.Context) {
	var req UpdateCapabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.ConnectionConfig == nil && req.SyncEnabled == nil && req.SyncIntervalSeconds <= 0 && strings.TrimSpace(req.HealthState) == "" {
		response.BadRequest(c, "at least one capability field is required")
		return
	}
	id, ok := parseMailboxIDParam(c, "id")
	if !ok {
		return
	}
	capability, err := h.repo.GetCapabilityByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if req.ConnectionConfig != nil {
		capability.ConnectionConfig = service.MailboxConnectionConfig(req.ConnectionConfig)
	}
	if req.SyncEnabled != nil {
		capability.SyncEnabled = *req.SyncEnabled
		if !*req.SyncEnabled && strings.TrimSpace(req.HealthState) == "" {
			capability.HealthState = service.MailboxCapabilityStatePaused
		}
	}
	if req.SyncIntervalSeconds > 0 {
		capability.SyncIntervalSeconds = req.SyncIntervalSeconds
	}
	if state := strings.TrimSpace(req.HealthState); state != "" {
		capability.HealthState = state
	}
	updated, err := h.repo.UpdateCapability(c.Request.Context(), capability)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.MailboxCapabilityFromService(updated))
}

func (h *MailboxHandler) ListRecipients(c *gin.Context) {
	identities, err := collectAllMailboxItems(func(opts service.MailboxListOptions) ([]*service.RecipientIdentity, error) {
		return h.repo.ListRecipientIdentities(c.Request.Context(), opts)
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.RecipientIdentity, 0, len(identities))
	for _, identity := range identities {
		values, valuesErr := h.repo.ListRecipientMatchValues(c.Request.Context(), identity.ID)
		if valuesErr != nil {
			response.ErrorFrom(c, valuesErr)
			return
		}
		mapped := dto.RecipientIdentityFromService(identity, values)
		if mapped != nil {
			out = append(out, *mapped)
		}
	}
	respondMailboxPage(c, out)
}

func (h *MailboxHandler) ImportRecipientExactAddresses(c *gin.Context) {
	var req ImportRecipientExactAddressesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	id, ok := parseMailboxIDParam(c, "id")
	if !ok {
		return
	}
	if _, err := h.repo.GetRecipientIdentityByID(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	existing, err := h.repo.ListRecipientMatchValues(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	merged, imported, err := mergeExactRecipientAddresses(existing, req.Addresses)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if _, err := h.repo.ReplaceRecipientMatchValues(c.Request.Context(), id, merged); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"imported": imported})
}

func (h *MailboxHandler) CreateRecipient(c *gin.Context) {
	var req CreateRecipientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	name, normalizedName, values, err := normalizeAndValidateRecipientInput(req.Name, req.MatchValues)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	identity, err := h.repo.CreateRecipientIdentity(c.Request.Context(), &service.RecipientIdentity{
		Name:           name,
		NormalizedName: normalizedName,
		Enabled:        enabled,
	}, values)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	storedValues, err := h.repo.ListRecipientMatchValues(c.Request.Context(), identity.ID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.RecipientIdentityFromService(identity, storedValues))
}

func (h *MailboxHandler) UpdateRecipientStatus(c *gin.Context) {
	var req UpdateRecipientStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	id, ok := parseMailboxIDParam(c, "id")
	if !ok {
		return
	}
	identity, err := h.repo.GetRecipientIdentityByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	identity.Enabled = *req.Enabled
	updated, err := h.repo.UpdateRecipientIdentity(c.Request.Context(), identity)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	values, err := h.repo.ListRecipientMatchValues(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.RecipientIdentityFromService(updated, values))
}

func (h *MailboxHandler) UpdateRecipient(c *gin.Context) {
	var req UpdateRecipientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	name, normalizedName, values, err := normalizeAndValidateRecipientInput(req.Name, req.MatchValues)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	id, ok := parseMailboxIDParam(c, "id")
	if !ok {
		return
	}
	identity, err := h.repo.GetRecipientIdentityByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	identity.Name = name
	identity.NormalizedName = normalizedName
	if req.Enabled != nil {
		identity.Enabled = *req.Enabled
	}
	updated, values, err := h.repo.UpdateRecipientIdentityWithMatchValues(c.Request.Context(), identity, values)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.RecipientIdentityFromService(updated, values))
}

func (h *MailboxHandler) ListInbox(c *gin.Context) {
	var req ListInboxRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	headers, total, err := h.repo.ListHeaders(c.Request.Context(), service.MailHeaderListFilter{
		CollectorID:  req.CollectorID,
		CapabilityID: req.CapabilityID,
		Folder:       strings.TrimSpace(req.Folder),
		Offset:       (page - 1) * pageSize,
		Limit:        pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.MailboxHeaderRecord, 0, len(headers))
	for _, header := range headers {
		mapped := dto.MailboxHeaderRecordFromService(header)
		if mapped != nil {
			out = append(out, *mapped)
		}
	}
	response.Paginated(c, out, total, page, pageSize)
}

func (h *MailboxHandler) GetInboxHeader(c *gin.Context) {
	id, ok := parseMailboxIDParam(c, "id")
	if !ok {
		return
	}
	header, err := h.repo.GetHeaderByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.MailboxHeaderRecordFromService(header))
}

func (h *MailboxHandler) GetInboxDetail(c *gin.Context) {
	id, ok := parseMailboxIDParam(c, "id")
	if !ok {
		return
	}
	if h.syncService == nil {
		response.Error(c, http.StatusServiceUnavailable, "mailbox sync service unavailable")
		return
	}
	header, err := h.syncService.FetchDetail(c.Request.Context(), id)
	if err != nil && header == nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.MailboxHeaderRecordFromService(header))
}

func (h *MailboxHandler) GetBatchSyncStatus(c *gin.Context) {
	batchID := strings.TrimSpace(c.Param("batch_id"))
	jobs, err := h.repo.ListSyncJobsByBatchID(c.Request.Context(), batchID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.BatchSyncStatusFromJobs(batchID, jobs))
}

func parseMailboxIDParam(c *gin.Context, name string) (int64, bool) {
	value, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || value <= 0 {
		response.BadRequest(c, "invalid id")
		return 0, false
	}
	return value, true
}

func hasExactlyOneMailboxTarget(collectorIDs, capabilityIDs []int64) bool {
	hasCollectors := len(normalizePositiveIDs(collectorIDs)) > 0
	hasCapabilities := len(normalizePositiveIDs(capabilityIDs)) > 0
	return hasCollectors != hasCapabilities
}

func normalizePositiveIDs(ids []int64) []int64 {
	set := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := set[id]; exists {
			continue
		}
		set[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func mailboxBatchIDFromJobs(jobs []*service.MailSyncJob) string {
	for _, job := range jobs {
		if job != nil && job.BatchID != nil {
			return *job.BatchID
		}
	}
	return ""
}

func mergeExactRecipientAddresses(existing []*service.RecipientMatchValue, addresses []string) ([]*service.RecipientMatchValue, int, error) {
	merged := make([]*service.RecipientMatchValue, 0, len(existing)+len(addresses))
	existingExact := make(map[string]struct{})
	for _, value := range existing {
		if value == nil {
			continue
		}
		clone := *value
		merged = append(merged, &clone)
		if clone.MatchType == service.RecipientMatchTypeExactAddress {
			existingExact[strings.TrimSpace(strings.ToLower(clone.NormalizedValue))] = struct{}{}
		}
	}
	imported := 0
	priority := 1000
	for _, raw := range addresses {
		normalized, err := normalizeAndValidateMailboxAddress(raw)
		if err != nil {
			return nil, 0, errors.New("addresses must contain only valid emails")
		}
		if _, exists := existingExact[normalized]; exists {
			continue
		}
		existingExact[normalized] = struct{}{}
		merged = append(merged, &service.RecipientMatchValue{
			MatchType:       service.RecipientMatchTypeExactAddress,
			MatchValue:      normalized,
			NormalizedValue: normalized,
			Active:          true,
			Priority:        priority,
			SourceKind:      "admin_bulk_import",
			SourceMetadata: service.RecipientMatchSourceMetadata{
				"imported_via": "admin_mailbox_api",
			},
		})
		priority--
		imported++
	}
	return merged, imported, nil
}

func normalizeMailboxAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, err := mail.ParseAddress(value); err == nil {
		return strings.ToLower(strings.TrimSpace(parsed.Address))
	}
	return strings.ToLower(value)
}

func normalizeStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func normalizeRecipientIdentityName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeCapabilityKind(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New("capability_kind is required")
	}
	return normalized, nil
}

func normalizeAndValidateMailboxAddress(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errors.New("email_address is required")
	}
	parsed, err := mail.ParseAddress(trimmed)
	if err != nil || !strings.EqualFold(strings.TrimSpace(parsed.Address), trimmed) {
		return "", errors.New("email_address must be a valid email")
	}
	return strings.ToLower(parsed.Address), nil
}

func normalizeDomainSuffix(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.TrimLeft(normalized, "@")
	return strings.TrimSpace(normalized)
}

func normalizeRecipientMatchValue(matchType string, matchValue string) string {
	switch strings.TrimSpace(matchType) {
	case service.RecipientMatchTypeExactAddress:
		return normalizeMailboxAddress(matchValue)
	case service.RecipientMatchTypeDomainSuffix:
		return normalizeDomainSuffix(matchValue)
	default:
		return strings.ToLower(strings.TrimSpace(matchValue))
	}
}

func normalizeAndValidateRecipientInput(name string, inputs []RecipientMatchValueInput) (string, string, []*service.RecipientMatchValue, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return "", "", nil, errors.New("name is required")
	}
	if len(inputs) == 0 {
		return "", "", nil, errors.New("match_values is required")
	}
	values := make([]*service.RecipientMatchValue, 0, len(inputs))
	for _, input := range inputs {
		active := true
		if input.Active != nil {
			active = *input.Active
		}
		priority := 0
		if input.Priority != nil {
			priority = *input.Priority
		}
		matchType := strings.TrimSpace(input.MatchType)
		matchValue := strings.TrimSpace(input.MatchValue)
		if matchValue == "" {
			return "", "", nil, errors.New("match_value is required")
		}
		switch matchType {
		case service.RecipientMatchTypeExactAddress:
			normalized, err := normalizeAndValidateMailboxAddress(matchValue)
			if err != nil {
				return "", "", nil, errors.New("exact_address match_value must be a valid email")
			}
			matchValue = normalized
		case service.RecipientMatchTypeDomainSuffix:
			matchValue = normalizeDomainSuffix(matchValue)
			if matchValue == "" {
				return "", "", nil, errors.New("domain_suffix match_value is required")
			}
		}
		values = append(values, &service.RecipientMatchValue{
			MatchType:       matchType,
			MatchValue:      matchValue,
			NormalizedValue: normalizeRecipientMatchValue(matchType, matchValue),
			Active:          active,
			Priority:        priority,
			SourceKind:      strings.TrimSpace(input.SourceKind),
			SourceMetadata:  service.RecipientMatchSourceMetadata(input.SourceMetadata),
		})
	}
	return trimmedName, normalizeRecipientIdentityName(trimmedName), values, nil
}

func buildRecipientMatchValues(inputs []RecipientMatchValueInput) []*service.RecipientMatchValue {
	out := make([]*service.RecipientMatchValue, 0, len(inputs))
	for _, input := range inputs {
		active := true
		if input.Active != nil {
			active = *input.Active
		}
		priority := 0
		if input.Priority != nil {
			priority = *input.Priority
		}
		matchType := strings.TrimSpace(input.MatchType)
		matchValue := strings.TrimSpace(input.MatchValue)
		out = append(out, &service.RecipientMatchValue{
			MatchType:       matchType,
			MatchValue:      matchValue,
			NormalizedValue: normalizeRecipientMatchValue(matchType, matchValue),
			Active:          active,
			Priority:        priority,
			SourceKind:      strings.TrimSpace(input.SourceKind),
			SourceMetadata:  service.RecipientMatchSourceMetadata(input.SourceMetadata),
		})
	}
	return out
}

func (h *MailboxHandler) listCollectorCapabilities(ctx context.Context, collectorID int64) ([]*service.MailboxCapability, error) {
	capabilities, err := collectAllMailboxItems(func(opts service.MailboxListOptions) ([]*service.MailboxCapability, error) {
		return h.repo.ListCapabilities(ctx, opts)
	})
	if err != nil {
		return nil, err
	}
	out := make([]*service.MailboxCapability, 0)
	for _, capability := range capabilities {
		if capability == nil || capability.CollectorID != collectorID {
			continue
		}
		out = append(out, capability)
	}
	return out, nil
}

func isMailboxBadRequestError(err error) bool {
	return errors.Is(err, mailboxpkg.ErrMailboxImportFormat) ||
		errors.Is(err, service.ErrMailboxUnsupportedProvider) ||
		errors.Is(err, service.ErrMailboxUnsupportedAuthKind) ||
		errors.Is(err, service.ErrMailboxPayloadRequired)
}

func collectAllMailboxItems[T any](fetch func(service.MailboxListOptions) ([]*T, error)) ([]*T, error) {
	offset := 0
	out := make([]*T, 0)
	for {
		batch, err := fetch(service.MailboxListOptions{Offset: offset, Limit: mailboxAdminListBatchSize})
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			return out, nil
		}
		out = append(out, batch...)
		offset += len(batch)
		if len(batch) < mailboxAdminListBatchSize {
			return out, nil
		}
	}
}

func respondMailboxPage[T any](c *gin.Context, items []T) {
	page, pageSize := response.ParsePagination(c)
	offset := (page - 1) * pageSize
	if offset >= len(items) {
		response.Paginated(c, []T{}, int64(len(items)), page, pageSize)
		return
	}
	end := offset + pageSize
	if end > len(items) {
		end = len(items)
	}
	response.Paginated(c, items[offset:end], int64(len(items)), page, pageSize)
}
