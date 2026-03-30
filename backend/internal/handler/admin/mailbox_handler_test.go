package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMailboxHandler_CreateProviderValidatesBundleAndReturnsCreated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMailboxHandlerRepoStub()
	providerService := service.NewMailboxService(repo, nil, nil, nil)
	handler := NewMailboxHandler(providerService, &mailboxSyncServiceStub{}, repo)
	router := newMailboxHandlerTestRouter(handler)

	body := []byte(`{
		"display_name":"Outlook Import",
		"provider_kind":"microsoft",
		"auth_kind":"import_bundle",
		"encrypted_payload":"boss@example.com----provider-42----opaque-left----opaque-right"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/providers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, repo.createdProviders, 1)
	require.Contains(t, repo.createdProviders[0].EncryptedPayload, `"mailbox_identifier":"boss@example.com"`)
	require.Contains(t, repo.createdProviders[0].EncryptedPayload, `"provider_identifier":"provider-42"`)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data := resp.Data.(map[string]any)
	require.Equal(t, true, data["payload_configured"])
	_, hasRawPayload := data["encrypted_payload"]
	require.False(t, hasRawPayload)
	_, hasSummary := data["payload_summary"]
	require.True(t, hasSummary)
}

func TestMailboxHandler_CreateProviderRejectsInvalidStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMailboxHandlerRepoStub()
	providerService := service.NewMailboxService(repo, nil, nil, nil)
	handler := NewMailboxHandler(providerService, &mailboxSyncServiceStub{}, repo)
	router := newMailboxHandlerTestRouter(handler)

	body := []byte(`{
		"display_name":"Outlook Import",
		"provider_kind":"microsoft",
		"auth_kind":"import_bundle",
		"status":"bogus",
		"encrypted_payload":"boss@example.com----provider-42----opaque-left----opaque-right"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/providers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, repo.createdProviders)
}

func TestMailboxHandler_UpdateProviderStatusAcceptsPost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMailboxHandlerRepoStub()
	repo.providers[3] = &service.ProviderAccount{
		ID:           3,
		DisplayName:  "Mailbox",
		ProviderKind: service.MailboxProviderKindMicrosoft,
		AuthKind:     service.ProviderAuthKindOAuth2,
		Status:       service.ProviderAccountStatusActive,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, repo)
	router := newMailboxHandlerTestRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/providers/3/status", bytes.NewBufferString(`{"status":"disabled"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, service.ProviderAccountStatusDisabled, repo.providers[3].Status)
}

func TestMailboxHandler_BatchSyncCollectorsRequiresTargetIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, newMailboxHandlerRepoStub())
	router := newMailboxHandlerTestRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/collectors/batch-sync", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestMailboxHandler_UpdateCollectorUpdatesFieldsAndReturnsCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMailboxHandlerRepoStub()
	now := time.Now().UTC()
	repo.collectors[4] = &service.CollectorMailbox{
		ID:           4,
		EmailAddress: "old@example.com",
		DisplayName:  "Old Collector",
		Enabled:      true,
		BusinessTags: []string{"legacy"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	repo.capabilities[8] = &service.MailboxCapability{
		ID:                  8,
		ProviderAccountID:   3,
		CollectorID:         4,
		CapabilityKind:      "imap-primary",
		ConnectionConfig:    service.MailboxConnectionConfig{"folder": "INBOX"},
		SyncEnabled:         true,
		SyncIntervalSeconds: 300,
		HealthState:         service.MailboxCapabilityStateHealthy,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, repo)
	router := newMailboxHandlerTestRouter(handler)

	body := []byte(`{
		"email_address":"support@example.com",
		"display_name":"Support Inbox",
		"enabled":false,
		"business_tags":["vip","apac","vip"]
	}`)
	req := httptest.NewRequest(http.MethodPut, "/collectors/4", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "support@example.com", repo.collectors[4].EmailAddress)
	require.Equal(t, "Support Inbox", repo.collectors[4].DisplayName)
	require.False(t, repo.collectors[4].Enabled)
	require.Equal(t, []string{"vip", "apac"}, repo.collectors[4].BusinessTags)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data := resp.Data.(map[string]any)
	require.Equal(t, "support@example.com", data["email_address"])
	require.Equal(t, "Support Inbox", data["display_name"])
	require.Equal(t, false, data["enabled"])
	capabilities := data["capabilities"].([]any)
	require.Len(t, capabilities, 1)
	capability := capabilities[0].(map[string]any)
	require.Equal(t, float64(8), capability["id"])
	require.Equal(t, "imap-primary", capability["capability_kind"])
}

func TestMailboxHandler_UpsertCollectorRejectsInvalidEmailAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, newMailboxHandlerRepoStub())
	router := newMailboxHandlerTestRouter(handler)

	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "blank", body: `{"email_address":"   "}`},
		{name: "invalid", body: `{"email_address":"not-an-email"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/collectors", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestMailboxHandler_CreateCapabilityAppliesDefaultSyncSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMailboxHandlerRepoStub()
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, repo)
	router := newMailboxHandlerTestRouter(handler)

	body := []byte(`{
		"provider_account_id":3,
		"collector_id":7,
		"capability_kind":"imap-primary",
		"connection_config":{"folder":"INBOX"},
		"sync_enabled":false
	}`)
	req := httptest.NewRequest(http.MethodPost, "/capabilities", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, repo.capabilities, 1)
	require.Equal(t, defaultMailboxSyncIntervalSeconds, repo.capabilities[1].SyncIntervalSeconds)
	require.False(t, repo.capabilities[1].SyncEnabled)
	require.Equal(t, service.MailboxCapabilityStatePaused, repo.capabilities[1].HealthState)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data := resp.Data.(map[string]any)
	require.Equal(t, float64(defaultMailboxSyncIntervalSeconds), data["sync_interval_seconds"])
	require.Equal(t, false, data["sync_enabled"])
	require.Equal(t, service.MailboxCapabilityStatePaused, data["health_state"])
}

func TestMailboxHandler_CreateCapabilityDefaultsToHealthyWhenSyncEnabledOmitted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMailboxHandlerRepoStub()
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, repo)
	router := newMailboxHandlerTestRouter(handler)

	body := []byte(`{
		"provider_account_id":3,
		"collector_id":7,
		"capability_kind":"imap-primary",
		"connection_config":{"folder":"INBOX"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/capabilities", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, repo.capabilities, 1)
	require.True(t, repo.capabilities[1].SyncEnabled)
	require.Equal(t, defaultMailboxSyncIntervalSeconds, repo.capabilities[1].SyncIntervalSeconds)
	require.Equal(t, service.MailboxCapabilityStateHealthy, repo.capabilities[1].HealthState)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data := resp.Data.(map[string]any)
	require.Equal(t, true, data["sync_enabled"])
	require.Equal(t, float64(defaultMailboxSyncIntervalSeconds), data["sync_interval_seconds"])
	require.Equal(t, service.MailboxCapabilityStateHealthy, data["health_state"])
}

func TestMailboxHandler_CreateCapabilityRejectsBlankCapabilityKind(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, newMailboxHandlerRepoStub())
	router := newMailboxHandlerTestRouter(handler)

	body := []byte(`{
		"provider_account_id":3,
		"collector_id":7,
		"capability_kind":"   ",
		"connection_config":{"folder":"INBOX"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/capabilities", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestMailboxHandler_UpdateCapabilityUpdatesConnectionConfigAndSyncState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMailboxHandlerRepoStub()
	now := time.Now().UTC()
	repo.capabilities[9] = &service.MailboxCapability{
		ID:                  9,
		ProviderAccountID:   3,
		CollectorID:         7,
		CapabilityKind:      "imap-primary",
		ConnectionConfig:    service.MailboxConnectionConfig{"folder": "INBOX"},
		SyncEnabled:         true,
		SyncIntervalSeconds: 180,
		HealthState:         service.MailboxCapabilityStateWarning,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, repo)
	router := newMailboxHandlerTestRouter(handler)

	body := []byte(`{
		"connection_config":{"folder":"Archive","host":"imap.example.com"},
		"sync_enabled":false,
		"sync_interval_seconds":600
	}`)
	req := httptest.NewRequest(http.MethodPut, "/capabilities/9", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, repo.capabilities[9].SyncEnabled)
	require.Equal(t, 600, repo.capabilities[9].SyncIntervalSeconds)
	require.Equal(t, service.MailboxCapabilityStatePaused, repo.capabilities[9].HealthState)
	require.Equal(t, "Archive", repo.capabilities[9].ConnectionConfig["folder"])
	require.Equal(t, "imap.example.com", repo.capabilities[9].ConnectionConfig["host"])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data := resp.Data.(map[string]any)
	require.Equal(t, false, data["sync_enabled"])
	require.Equal(t, float64(600), data["sync_interval_seconds"])
	require.Equal(t, service.MailboxCapabilityStatePaused, data["health_state"])
}

func TestMailboxHandler_UpdateCapabilityReEnablingPreservesHealthStateWhenOmitted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMailboxHandlerRepoStub()
	now := time.Now().UTC()
	repo.capabilities[10] = &service.MailboxCapability{
		ID:                  10,
		ProviderAccountID:   3,
		CollectorID:         7,
		CapabilityKind:      "imap-primary",
		ConnectionConfig:    service.MailboxConnectionConfig{"folder": "INBOX"},
		SyncEnabled:         false,
		SyncIntervalSeconds: 180,
		HealthState:         service.MailboxCapabilityStateWarning,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, repo)
	router := newMailboxHandlerTestRouter(handler)

	req := httptest.NewRequest(http.MethodPut, "/capabilities/10", bytes.NewBufferString(`{"sync_enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, repo.capabilities[10].SyncEnabled)
	require.Equal(t, service.MailboxCapabilityStateWarning, repo.capabilities[10].HealthState)
}

func TestMailboxHandler_CreateRecipientCreatesIdentityWithMatchValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMailboxHandlerRepoStub()
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, repo)
	router := newMailboxHandlerTestRouter(handler)

	body := []byte(`{
		"name":"Support Inbox",
		"match_values":[
			{"match_type":"exact_address","match_value":"Support@Example.com","priority":90,"source_kind":"manual"},
			{"match_type":"domain_suffix","match_value":"@Example.org","priority":50,"active":false,"source_kind":"rule","source_metadata":{"channel":"mailbox"}}
		]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/recipients", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, repo.identities, 1)
	require.Equal(t, "Support Inbox", repo.identities[1].Name)
	require.Len(t, repo.matchValues[1], 2)
	require.Equal(t, service.RecipientMatchTypeExactAddress, repo.matchValues[1][0].MatchType)
	require.Equal(t, "support@example.com", repo.matchValues[1][0].NormalizedValue)
	require.Equal(t, service.RecipientMatchTypeDomainSuffix, repo.matchValues[1][1].MatchType)
	require.Equal(t, "example.org", repo.matchValues[1][1].NormalizedValue)
	require.Equal(t, "example.org", repo.matchValues[1][1].MatchValue)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data := resp.Data.(map[string]any)
	require.Equal(t, "Support Inbox", data["name"])
	matchValues := data["match_values"].([]any)
	require.Len(t, matchValues, 2)
}

func TestMailboxHandler_CreateRecipientRejectsInvalidInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, newMailboxHandlerRepoStub())
	router := newMailboxHandlerTestRouter(handler)

	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "blank recipient name", body: `{"name":"   ","match_values":[{"match_type":"exact_address","match_value":"boss@example.com"}]}`},
		{name: "empty match values", body: `{"name":"Support","match_values":[]}`},
		{name: "blank match value", body: `{"name":"Support","match_values":[{"match_type":"exact_address","match_value":"   "}]}`},
		{name: "invalid exact address", body: `{"name":"Support","match_values":[{"match_type":"exact_address","match_value":"not-an-email"}]}`},
		{name: "empty domain suffix after normalize", body: `{"name":"Support","match_values":[{"match_type":"domain_suffix","match_value":"  @  "}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/recipients", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestMailboxHandler_UpdateRecipientReplacesMatchValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMailboxHandlerRepoStub()
	now := time.Now().UTC()
	repo.identities[5] = &service.RecipientIdentity{
		ID:             5,
		Name:           "Support",
		NormalizedName: "support",
		Enabled:        true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	repo.matchValues[5] = []*service.RecipientMatchValue{{
		ID:                  1,
		RecipientIdentityID: 5,
		MatchType:           service.RecipientMatchTypeExactAddress,
		MatchValue:          "legacy@example.com",
		NormalizedValue:     "legacy@example.com",
		Active:              true,
		Priority:            100,
		SourceKind:          "manual",
	}}
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, repo)
	router := newMailboxHandlerTestRouter(handler)

	body := []byte(`{
		"name":"VIP Support",
		"enabled":false,
		"match_values":[
			{"match_type":"exact_address","match_value":"vip@example.com","priority":10,"active":false,"source_kind":"manual"},
			{"match_type":"domain_suffix","match_value":"vip.example.com","priority":5,"active":true,"source_kind":"rule","source_metadata":{"team":"vip"}}
		]
	}`)
	req := httptest.NewRequest(http.MethodPut, "/recipients/5", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, repo.updateRecipientAtomicCalls)
	require.Zero(t, repo.updateRecipientIdentityCalls)
	require.Zero(t, repo.replaceRecipientMatchValuesCalls)
	require.Equal(t, "VIP Support", repo.identities[5].Name)
	require.False(t, repo.identities[5].Enabled)
	require.Len(t, repo.replacedMatchValues[5], 2)
	require.Equal(t, "vip@example.com", repo.replacedMatchValues[5][0].NormalizedValue)
	require.Equal(t, 10, repo.replacedMatchValues[5][0].Priority)
	require.False(t, repo.replacedMatchValues[5][0].Active)
	require.Equal(t, "vip.example.com", repo.replacedMatchValues[5][1].NormalizedValue)
	require.Equal(t, 5, repo.replacedMatchValues[5][1].Priority)
	require.True(t, repo.replacedMatchValues[5][1].Active)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data := resp.Data.(map[string]any)
	require.Equal(t, "VIP Support", data["name"])
	require.Equal(t, false, data["enabled"])
	matchValues := data["match_values"].([]any)
	require.Len(t, matchValues, 2)
}

func TestMailboxHandler_TestCapabilityReturnsIndependentHealthResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixedNow := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	providerService := &mailboxProviderServiceStub{
		testCapabilityResult: &service.MailboxCapability{
			ID:                9,
			ProviderAccountID: 3,
			CollectorID:       7,
			CapabilityKind:    "imap-primary",
			SyncEnabled:       true,
			HealthState:       service.MailboxCapabilityStateHealthy,
			LastSyncAt:        &fixedNow,
			CreatedAt:         fixedNow,
			UpdatedAt:         fixedNow,
		},
	}
	handler := NewMailboxHandler(providerService, &mailboxSyncServiceStub{}, newMailboxHandlerRepoStub())
	router := newMailboxHandlerTestRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/capabilities/9/test", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data := resp.Data.(map[string]any)
	result := data["result"].(map[string]any)
	require.Equal(t, true, result["success"])
	require.Equal(t, service.MailboxCapabilityStateHealthy, result["health_state"])
	capability := data["capability"].(map[string]any)
	require.Equal(t, float64(9), capability["id"])
	require.Equal(t, service.MailboxCapabilityStateHealthy, capability["health_state"])
}

func TestMailboxHandler_ImportRecipientExactAddressesAcceptsBulkPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMailboxHandlerRepoStub()
	repo.identities[7] = &service.RecipientIdentity{
		ID:             7,
		Name:           "Support",
		NormalizedName: "support",
		Enabled:        true,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, repo)
	router := newMailboxHandlerTestRouter(handler)

	body := []byte(`{"addresses":["one@privaterelay.appleid.com","two@privaterelay.appleid.com"]}`)
	req := httptest.NewRequest(http.MethodPost, "/recipients/7/import-exact-addresses", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, repo.replacedMatchValues[7], 2)
	values := []string{
		repo.replacedMatchValues[7][0].NormalizedValue,
		repo.replacedMatchValues[7][1].NormalizedValue,
	}
	sort.Strings(values)
	require.Equal(t, []string{"one@privaterelay.appleid.com", "two@privaterelay.appleid.com"}, values)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data := resp.Data.(map[string]any)
	require.Equal(t, float64(2), data["imported"])
}

func TestMailboxHandler_ImportRecipientExactAddressesRejectsInvalidEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMailboxHandlerRepoStub()
	repo.identities[7] = &service.RecipientIdentity{
		ID:             7,
		Name:           "Support",
		NormalizedName: "support",
		Enabled:        true,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, repo)
	router := newMailboxHandlerTestRouter(handler)

	body := []byte(`{"addresses":["valid@example.com","not-an-email"]}`)
	req := httptest.NewRequest(http.MethodPost, "/recipients/7/import-exact-addresses", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	_, replaced := repo.replacedMatchValues[7]
	require.False(t, replaced)
}

func TestMailboxHandler_UpsertCollectorRollsBackWhenCapabilityCreateFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMailboxHandlerRepoStub()
	repo.createCapabilityErr = errors.New("capability boom")
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, repo)
	router := newMailboxHandlerTestRouter(handler)

	body := []byte(`{
		"email_address":"collector@example.com",
		"display_name":"Collector",
		"provider_account_id":3,
		"capabilities":[{"capability_kind":"imap-primary"}]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/collectors", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Empty(t, repo.collectors)
	require.Equal(t, []int64{1}, repo.deletedCollectorIDs)
}

func TestMailboxHandler_UpsertCollectorRejectsBlankNestedCapabilityKind(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, newMailboxHandlerRepoStub())
	router := newMailboxHandlerTestRouter(handler)

	body := []byte(`{
		"email_address":"collector@example.com",
		"provider_account_id":3,
		"capabilities":[{"capability_kind":"   "}]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/collectors", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestMailboxHandler_ListProvidersReturnsPaginatedPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMailboxHandlerRepoStub()
	for i := int64(1); i <= 60; i++ {
		repo.providers[i] = &service.ProviderAccount{
			ID:           i,
			DisplayName:  "provider",
			ProviderKind: service.MailboxProviderKindMicrosoft,
			AuthKind:     service.ProviderAuthKindOAuth2,
			Status:       service.ProviderAccountStatusActive,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
	}
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, repo)
	router := newMailboxHandlerTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/providers?page=2&page_size=25", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data := resp.Data.(map[string]any)
	require.Equal(t, float64(60), data["total"])
	require.Equal(t, float64(2), data["page"])
	require.Equal(t, float64(25), data["page_size"])
	items := data["items"].([]any)
	require.Len(t, items, 25)
}

func TestMailboxHandler_ListRecipientsReturnsPaginatedPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMailboxHandlerRepoStub()
	for i := int64(1); i <= 55; i++ {
		repo.identities[i] = &service.RecipientIdentity{
			ID:             i,
			Name:           "Recipient",
			NormalizedName: "recipient",
			Enabled:        true,
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}
		repo.matchValues[i] = []*service.RecipientMatchValue{{
			ID:                  i,
			RecipientIdentityID: i,
			MatchType:           service.RecipientMatchTypeExactAddress,
			MatchValue:          "r@example.com",
			NormalizedValue:     "r@example.com",
			Active:              true,
		}}
	}
	handler := NewMailboxHandler(&mailboxProviderServiceStub{}, &mailboxSyncServiceStub{}, repo)
	router := newMailboxHandlerTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/recipients?page=3&page_size=20", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data := resp.Data.(map[string]any)
	require.Equal(t, float64(55), data["total"])
	require.Equal(t, float64(3), data["page"])
	items := data["items"].([]any)
	require.Len(t, items, 15)
}

func newMailboxHandlerTestRouter(handler *MailboxHandler) *gin.Engine {
	router := gin.New()
	router.GET("/providers", handler.ListProviders)
	router.POST("/providers/:id/status", handler.UpdateProviderStatus)
	router.PUT("/providers/:id/status", handler.UpdateProviderStatus)
	router.POST("/collectors", handler.UpsertCollector)
	router.PUT("/collectors/:id", handler.UpdateCollector)
	router.POST("/providers", handler.CreateProvider)
	router.POST("/collectors/batch-sync", handler.BatchSyncCollectors)
	router.GET("/collectors", handler.ListCollectors)
	router.GET("/capabilities", handler.ListCapabilities)
	router.POST("/capabilities", handler.CreateCapability)
	router.PUT("/capabilities/:id", handler.UpdateCapability)
	router.POST("/capabilities/:id/test", handler.TestCapability)
	router.GET("/recipients", handler.ListRecipients)
	router.POST("/recipients", handler.CreateRecipient)
	router.PUT("/recipients/:id", handler.UpdateRecipient)
	router.POST("/recipients/:id/import-exact-addresses", handler.ImportRecipientExactAddresses)
	return router
}

type mailboxProviderServiceStub struct {
	createProviderResult *service.ProviderAccount
	createProviderErr    error
	validateResult       *service.ProviderValidationOutcome
	validateErr          error
	testCapabilityResult *service.MailboxCapability
	testCapabilityErr    error
}

func (s *mailboxProviderServiceStub) CreateProvider(ctx context.Context, input service.MailboxProviderUpsertInput) (*service.ProviderAccount, error) {
	if s.createProviderErr != nil {
		return nil, s.createProviderErr
	}
	if s.createProviderResult != nil {
		return s.createProviderResult, nil
	}
	now := time.Now().UTC()
	return &service.ProviderAccount{
		ID:               1,
		DisplayName:      input.DisplayName,
		ProviderKind:     input.ProviderKind,
		AuthKind:         input.AuthKind,
		Status:           service.ProviderAccountStatusDraft,
		CreatedAt:        now,
		UpdatedAt:        now,
		EncryptedPayload: input.EncryptedPayload,
	}, nil
}

func (s *mailboxProviderServiceStub) ValidateProvider(ctx context.Context, providerID int64) (*service.ProviderValidationOutcome, error) {
	if s.validateErr != nil {
		return nil, s.validateErr
	}
	if s.validateResult != nil {
		return s.validateResult, nil
	}
	return &service.ProviderValidationOutcome{Code: "ok", Message: ""}, nil
}

func (s *mailboxProviderServiceStub) TestCapability(ctx context.Context, capabilityID int64) (*service.MailboxCapability, error) {
	if s.testCapabilityErr != nil {
		return s.testCapabilityResult, s.testCapabilityErr
	}
	if s.testCapabilityResult == nil {
		return nil, errors.New("missing test capability result")
	}
	return s.testCapabilityResult, nil
}

type mailboxSyncServiceStub struct {
	createBatchSyncResult []*service.MailSyncJob
	createBatchSyncErr    error
	fetchDetailResult     *service.MailHeader
	fetchDetailErr        error
}

func (s *mailboxSyncServiceStub) CreateBatchSyncJobs(ctx context.Context, input service.MailboxBatchSyncRequest) ([]*service.MailSyncJob, error) {
	if s.createBatchSyncErr != nil {
		return nil, s.createBatchSyncErr
	}
	if s.createBatchSyncResult != nil {
		return s.createBatchSyncResult, nil
	}
	return []*service.MailSyncJob{}, nil
}

func (s *mailboxSyncServiceStub) FetchDetail(ctx context.Context, headerID int64) (*service.MailHeader, error) {
	if s.fetchDetailErr != nil {
		return s.fetchDetailResult, s.fetchDetailErr
	}
	return s.fetchDetailResult, nil
}

type mailboxHandlerRepoStub struct {
	providers                        map[int64]*service.ProviderAccount
	collectors                       map[int64]*service.CollectorMailbox
	capabilities                     map[int64]*service.MailboxCapability
	identities                       map[int64]*service.RecipientIdentity
	matchValues                      map[int64][]*service.RecipientMatchValue
	createdProviders                 []*service.ProviderAccount
	createdIdentities                []*service.RecipientIdentity
	replacedMatchValues              map[int64][]*service.RecipientMatchValue
	updateRecipientAtomicCalls       int
	updateRecipientIdentityCalls     int
	replaceRecipientMatchValuesCalls int
	nextProviderID                   int64
	nextCollectorID                  int64
	nextCapabilityID                 int64
	nextIdentityID                   int64
	nextMatchValueID                 int64
	createCapabilityErr              error
	deletedCollectorIDs              []int64
}

var _ service.MailboxRepository = (*mailboxHandlerRepoStub)(nil)

func newMailboxHandlerRepoStub() *mailboxHandlerRepoStub {
	return &mailboxHandlerRepoStub{
		providers:           map[int64]*service.ProviderAccount{},
		collectors:          map[int64]*service.CollectorMailbox{},
		capabilities:        map[int64]*service.MailboxCapability{},
		identities:          map[int64]*service.RecipientIdentity{},
		matchValues:         map[int64][]*service.RecipientMatchValue{},
		replacedMatchValues: map[int64][]*service.RecipientMatchValue{},
	}
}

func (r *mailboxHandlerRepoStub) CreateProviderAccount(ctx context.Context, account *service.ProviderAccount) (*service.ProviderAccount, error) {
	r.nextProviderID++
	clone := *account
	clone.ID = r.nextProviderID
	if clone.PayloadVersion == 0 {
		clone.PayloadVersion = 1
	}
	now := time.Now().UTC()
	clone.CreatedAt = now
	clone.UpdatedAt = now
	r.providers[clone.ID] = &clone
	r.createdProviders = append(r.createdProviders, &clone)
	return &clone, nil
}

func (r *mailboxHandlerRepoStub) GetProviderAccountByID(ctx context.Context, id int64) (*service.ProviderAccount, error) {
	provider, ok := r.providers[id]
	if !ok {
		return nil, errors.New("provider not found")
	}
	clone := *provider
	return &clone, nil
}

func (r *mailboxHandlerRepoStub) UpdateProviderAccount(ctx context.Context, account *service.ProviderAccount) (*service.ProviderAccount, error) {
	clone := *account
	clone.UpdatedAt = time.Now().UTC()
	r.providers[clone.ID] = &clone
	return &clone, nil
}

func (r *mailboxHandlerRepoStub) ListProviderAccounts(ctx context.Context, opts service.MailboxListOptions) ([]*service.ProviderAccount, error) {
	items := make([]*service.ProviderAccount, 0, len(r.providers))
	for _, provider := range r.providers {
		clone := *provider
		items = append(items, &clone)
	}
	return items, nil
}

func (r *mailboxHandlerRepoStub) DeleteProviderAccount(ctx context.Context, id int64) error {
	return nil
}
func (r *mailboxHandlerRepoStub) CreateCollector(ctx context.Context, collector *service.CollectorMailbox) (*service.CollectorMailbox, error) {
	r.nextCollectorID++
	clone := *collector
	clone.ID = r.nextCollectorID
	clone.CreatedAt = time.Now().UTC()
	clone.UpdatedAt = clone.CreatedAt
	r.collectors[clone.ID] = &clone
	return &clone, nil
}
func (r *mailboxHandlerRepoStub) GetCollectorByID(ctx context.Context, id int64) (*service.CollectorMailbox, error) {
	collector, ok := r.collectors[id]
	if !ok {
		return nil, errors.New("collector not found")
	}
	clone := *collector
	return &clone, nil
}
func (r *mailboxHandlerRepoStub) UpdateCollector(ctx context.Context, collector *service.CollectorMailbox) (*service.CollectorMailbox, error) {
	clone := *collector
	clone.UpdatedAt = time.Now().UTC()
	r.collectors[clone.ID] = &clone
	return &clone, nil
}
func (r *mailboxHandlerRepoStub) ListCollectors(ctx context.Context, opts service.MailboxListOptions) ([]*service.CollectorMailbox, error) {
	items := make([]*service.CollectorMailbox, 0, len(r.collectors))
	for _, collector := range r.collectors {
		clone := *collector
		items = append(items, &clone)
	}
	return mailboxPaginateStub(items, opts.Offset, opts.Limit), nil
}
func (r *mailboxHandlerRepoStub) DeleteCollector(ctx context.Context, id int64) error {
	r.deletedCollectorIDs = append(r.deletedCollectorIDs, id)
	delete(r.collectors, id)
	for capabilityID, capability := range r.capabilities {
		if capability != nil && capability.CollectorID == id {
			delete(r.capabilities, capabilityID)
		}
	}
	return nil
}
func (r *mailboxHandlerRepoStub) CreateCapability(ctx context.Context, capability *service.MailboxCapability) (*service.MailboxCapability, error) {
	if r.createCapabilityErr != nil {
		return nil, r.createCapabilityErr
	}
	r.nextCapabilityID++
	clone := *capability
	clone.ID = r.nextCapabilityID
	clone.CreatedAt = time.Now().UTC()
	clone.UpdatedAt = clone.CreatedAt
	r.capabilities[clone.ID] = &clone
	return &clone, nil
}
func (r *mailboxHandlerRepoStub) GetCapabilityByID(ctx context.Context, id int64) (*service.MailboxCapability, error) {
	capability, ok := r.capabilities[id]
	if !ok {
		return nil, errors.New("capability not found")
	}
	clone := *capability
	return &clone, nil
}
func (r *mailboxHandlerRepoStub) UpdateCapability(ctx context.Context, capability *service.MailboxCapability) (*service.MailboxCapability, error) {
	clone := *capability
	clone.UpdatedAt = time.Now().UTC()
	r.capabilities[clone.ID] = &clone
	return &clone, nil
}
func (r *mailboxHandlerRepoStub) ListCapabilities(ctx context.Context, opts service.MailboxListOptions) ([]*service.MailboxCapability, error) {
	items := make([]*service.MailboxCapability, 0, len(r.capabilities))
	for _, capability := range r.capabilities {
		clone := *capability
		items = append(items, &clone)
	}
	return mailboxPaginateStub(items, opts.Offset, opts.Limit), nil
}
func (r *mailboxHandlerRepoStub) DeleteCapability(ctx context.Context, id int64) error {
	panic("unexpected DeleteCapability call")
}
func (r *mailboxHandlerRepoStub) CreateRecipientIdentity(ctx context.Context, in *service.RecipientIdentity, values []*service.RecipientMatchValue) (*service.RecipientIdentity, error) {
	r.nextIdentityID++
	clone := *in
	clone.ID = r.nextIdentityID
	now := time.Now().UTC()
	clone.CreatedAt = now
	clone.UpdatedAt = now
	r.identities[clone.ID] = &clone
	r.createdIdentities = append(r.createdIdentities, &clone)
	createdValues, err := r.ReplaceRecipientMatchValues(ctx, clone.ID, values)
	if err != nil {
		return nil, err
	}
	r.matchValues[clone.ID] = createdValues
	return &clone, nil
}

func (r *mailboxHandlerRepoStub) GetRecipientIdentityByID(ctx context.Context, id int64) (*service.RecipientIdentity, error) {
	identity, ok := r.identities[id]
	if !ok {
		return nil, errors.New("recipient not found")
	}
	clone := *identity
	return &clone, nil
}

func (r *mailboxHandlerRepoStub) UpdateRecipientIdentity(ctx context.Context, in *service.RecipientIdentity) (*service.RecipientIdentity, error) {
	r.updateRecipientIdentityCalls++
	clone := *in
	r.identities[clone.ID] = &clone
	return &clone, nil
}

func (r *mailboxHandlerRepoStub) ListRecipientIdentities(ctx context.Context, opts service.MailboxListOptions) ([]*service.RecipientIdentity, error) {
	items := make([]*service.RecipientIdentity, 0, len(r.identities))
	for _, identity := range r.identities {
		clone := *identity
		items = append(items, &clone)
	}
	return mailboxPaginateStub(items, opts.Offset, opts.Limit), nil
}

func (r *mailboxHandlerRepoStub) DeleteRecipientIdentity(ctx context.Context, id int64) error {
	return nil
}

func (r *mailboxHandlerRepoStub) ListRecipientMatchValues(ctx context.Context, recipientIdentityID int64) ([]*service.RecipientMatchValue, error) {
	values := r.matchValues[recipientIdentityID]
	out := make([]*service.RecipientMatchValue, 0, len(values))
	for _, value := range values {
		clone := *value
		out = append(out, &clone)
	}
	return out, nil
}

func (r *mailboxHandlerRepoStub) ListActiveRecipientMatchValues(ctx context.Context) ([]*service.RecipientMatchValue, error) {
	panic("unexpected ListActiveRecipientMatchValues call")
}

func (r *mailboxHandlerRepoStub) ReplaceRecipientMatchValues(ctx context.Context, recipientIdentityID int64, values []*service.RecipientMatchValue) ([]*service.RecipientMatchValue, error) {
	r.replaceRecipientMatchValuesCalls++
	out := make([]*service.RecipientMatchValue, 0, len(values))
	for _, value := range values {
		r.nextMatchValueID++
		clone := *value
		clone.ID = r.nextMatchValueID
		clone.RecipientIdentityID = recipientIdentityID
		out = append(out, &clone)
	}
	r.replacedMatchValues[recipientIdentityID] = out
	r.matchValues[recipientIdentityID] = out
	return out, nil
}

func (r *mailboxHandlerRepoStub) UpdateRecipientIdentityWithMatchValues(ctx context.Context, in *service.RecipientIdentity, values []*service.RecipientMatchValue) (*service.RecipientIdentity, []*service.RecipientMatchValue, error) {
	r.updateRecipientAtomicCalls++
	clone := *in
	clone.UpdatedAt = time.Now().UTC()
	r.identities[clone.ID] = &clone
	updatedValues := make([]*service.RecipientMatchValue, 0, len(values))
	for _, value := range values {
		r.nextMatchValueID++
		valueClone := *value
		valueClone.ID = r.nextMatchValueID
		valueClone.RecipientIdentityID = clone.ID
		updatedValues = append(updatedValues, &valueClone)
	}
	r.replacedMatchValues[clone.ID] = updatedValues
	r.matchValues[clone.ID] = updatedValues
	return &clone, updatedValues, nil
}

func (r *mailboxHandlerRepoStub) GetHeaderByID(ctx context.Context, id int64) (*service.MailHeader, error) {
	panic("unexpected GetHeaderByID call")
}
func (r *mailboxHandlerRepoStub) ListHeaders(ctx context.Context, filter service.MailHeaderListFilter) ([]*service.MailHeader, int64, error) {
	panic("unexpected ListHeaders call")
}
func (r *mailboxHandlerRepoStub) UpsertSyncHeaders(ctx context.Context, headers []*service.MailHeader) ([]*service.MailHeader, error) {
	panic("unexpected UpsertSyncHeaders call")
}
func (r *mailboxHandlerRepoStub) UpdateHeaderDetail(ctx context.Context, header *service.MailHeader) (*service.MailHeader, error) {
	panic("unexpected UpdateHeaderDetail call")
}
func (r *mailboxHandlerRepoStub) CreateSyncJobs(ctx context.Context, jobs []*service.MailSyncJob) ([]*service.MailSyncJob, error) {
	panic("unexpected CreateSyncJobs call")
}
func (r *mailboxHandlerRepoStub) ListSyncJobsByBatchID(ctx context.Context, batchID string) ([]*service.MailSyncJob, error) {
	panic("unexpected ListSyncJobsByBatchID call")
}
func (r *mailboxHandlerRepoStub) ClaimRunnableRetrySyncJobs(ctx context.Context, now time.Time, limit int) ([]*service.MailSyncJob, error) {
	panic("unexpected ClaimRunnableRetrySyncJobs call")
}
func (r *mailboxHandlerRepoStub) ListActiveSyncJobs(ctx context.Context, capabilityID *int64, limit int) ([]*service.MailSyncJob, error) {
	panic("unexpected ListActiveSyncJobs call")
}
func (r *mailboxHandlerRepoStub) UpdateSyncJobState(ctx context.Context, jobID int64, state string, startedAt, finishedAt, nextRetryAt *time.Time, errorSummary *string) (*service.MailSyncJob, error) {
	panic("unexpected UpdateSyncJobState call")
}
func (r *mailboxHandlerRepoStub) ClaimDueCapabilities(ctx context.Context, now time.Time, limit int) ([]*service.MailboxCapability, error) {
	panic("unexpected ClaimDueCapabilities call")
}

func mailboxPaginateStub[T any](items []*T, offset, limit int) []*T {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []*T{}
	}
	if limit <= 0 {
		limit = 50
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}
