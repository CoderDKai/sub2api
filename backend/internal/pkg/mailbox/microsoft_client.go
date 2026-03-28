package mailbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const defaultMicrosoftBaseURL = "https://graph.microsoft.com/v1.0"

type MicrosoftClient struct {
	httpClient *http.Client
	baseURL    string
}

type microsoftProfileResponse struct {
	ID                string `json:"id"`
	UserPrincipalName string `json:"userPrincipalName"`
	Mail              string `json:"mail"`
}

type microsoftMessagesResponse struct {
	Value []struct {
		ID                  string `json:"id"`
		Subject             string `json:"subject"`
		BodyPreview         string `json:"bodyPreview"`
		ReceivedDateTime    string `json:"receivedDateTime"`
		From                microsoftAddressContainer   `json:"from"`
		ToRecipients        []microsoftAddressContainer `json:"toRecipients"`
		CCRecipients        []microsoftAddressContainer `json:"ccRecipients"`
		InternetMessageHeaders []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"internetMessageHeaders"`
	} `json:"value"`
	NextLink string `json:"@odata.nextLink"`
}

type microsoftAddressContainer struct {
	EmailAddress struct {
		Address string `json:"address"`
	} `json:"emailAddress"`
}

func NewMicrosoftClient() *MicrosoftClient {
	return NewMicrosoftClientWithBaseURL(&http.Client{Timeout: 10 * time.Second}, defaultMicrosoftBaseURL)
}

func NewMicrosoftClientWithBaseURL(httpClient *http.Client, baseURL string) *MicrosoftClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultMicrosoftBaseURL
	}
	return &MicrosoftClient{httpClient: httpClient, baseURL: baseURL}
}

func (c *MicrosoftClient) Validate(ctx context.Context, profile ProviderProfile) (*ValidationResult, error) {
	accessToken := microsoftBearerToken(profile)
	if accessToken == "" {
		return &ValidationResult{
			Code:              ValidationCodeInvalidFormat,
			Message:           "microsoft token is required",
			InvalidateAccount: true,
		}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &ValidationResult{Code: ValidationCodeValidationFailed, Message: err.Error()}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &ValidationResult{
			Code:              ValidationCodeExpiredOrRevoked,
			Message:           strings.TrimSpace(resp.Status),
			InvalidateAccount: true,
		}, nil
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return &ValidationResult{Code: ValidationCodeValidationFailed, Message: strings.TrimSpace(resp.Status)}, nil
	}

	var body microsoftProfileResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return &ValidationResult{Code: ValidationCodeValidationFailed, Message: err.Error()}, nil
	}
	mailboxIdentifier := strings.TrimSpace(body.UserPrincipalName)
	if mailboxIdentifier == "" {
		mailboxIdentifier = strings.TrimSpace(body.Mail)
	}
	return &ValidationResult{
		Code:               ValidationCodeOK,
		ProviderIdentifier: coalesceMicrosoftIdentifier(strings.TrimSpace(body.ID), profile.ProviderIdentifier, stringValue(profile.Payload, "provider_identifier")),
		MailboxIdentifier:  coalesceMicrosoftIdentifier(mailboxIdentifier, profile.MailboxIdentifier, stringValue(profile.Payload, "mailbox_identifier")),
	}, nil
}

func (c *MicrosoftClient) ListHeaders(ctx context.Context, profile ProviderProfile, capability CapabilityProfile, limit int) (*HeaderPage, error) {
	accessToken := microsoftBearerToken(profile)
	if accessToken == "" {
		return nil, fmt.Errorf("microsoft token is required")
	}
	if limit <= 0 {
		limit = 1
	}
	requestURL := strings.TrimSpace(stringValue(capability.CursorState, "next_link"))
	if requestURL == "" {
		requestURL = c.mailFolderMessagesURL(stringValue(capability.ConnectionConfig, "folder"))
	}
	endpoint, err := url.Parse(requestURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(stringValue(capability.CursorState, "next_link")) == "" {
		query := endpoint.Query()
		query.Set("$top", fmt.Sprintf("%d", limit))
		query.Set("$select", "id,subject,bodyPreview,receivedDateTime,from,toRecipients,ccRecipients,internetMessageHeaders")
		endpoint.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("microsoft inbox read failed: %s", strings.TrimSpace(resp.Status))
	}

	var body microsoftMessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	page := &HeaderPage{
		Headers:    make([]Header, 0, len(body.Value)),
		NextCursor: map[string]any{},
	}
	for _, message := range body.Value {
		receivedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(message.ReceivedDateTime))
		if err != nil {
			receivedAt = time.Time{}
		}
		folder := strings.TrimSpace(stringValue(capability.ConnectionConfig, "folder"))
		if folder == "" {
			folder = "inbox"
		}
		header := Header{
			RemoteMessageID: strings.TrimSpace(message.ID),
			Folder:          folder,
			Sender:          strings.TrimSpace(message.From.EmailAddress.Address),
			Recipients:      collectMicrosoftAddresses(message.ToRecipients, message.CCRecipients),
			Subject:         strings.TrimSpace(message.Subject),
			ReceivedAt:      receivedAt.UTC(),
			Snippet:         strings.TrimSpace(message.BodyPreview),
			DeliveredTo:     collectHeaderValues(message.InternetMessageHeaders, "Delivered-To"),
			OriginalTo:      collectHeaderValues(message.InternetMessageHeaders, "X-Original-To"),
		}
		page.Headers = append(page.Headers, header)
	}
	if strings.TrimSpace(body.NextLink) != "" {
		page.NextCursor["next_link"] = strings.TrimSpace(body.NextLink)
	}
	return page, nil
}

func (c *MicrosoftClient) mailFolderMessagesURL(folder string) string {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		folder = "inbox"
	}
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return c.baseURL + "/me/mailFolders/" + url.PathEscape(folder) + "/messages"
	}
	base.Path = path.Join(base.Path, "me", "mailFolders", folder, "messages")
	return base.String()
}

func microsoftBearerToken(profile ProviderProfile) string {
	return firstStringValue(profile.Payload, "access_token", "token_bundle")
}

func coalesceMicrosoftIdentifier(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func collectMicrosoftAddresses(groups ...[]microsoftAddressContainer) []string {
	out := make([]string, 0)
	for _, group := range groups {
		for _, item := range group {
			address := strings.TrimSpace(item.EmailAddress.Address)
			if address != "" {
				out = append(out, address)
			}
		}
	}
	return out
}

func collectHeaderValues(headers []struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}, want string) []string {
	out := make([]string, 0)
	for _, header := range headers {
		if !strings.EqualFold(strings.TrimSpace(header.Name), want) {
			continue
		}
		value := strings.TrimSpace(header.Value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
