package mailbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMicrosoftClientValidateSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/me", r.URL.Path)
		require.Equal(t, "Bearer opaque-left----opaque-right", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":                "provider-42",
			"userPrincipalName": "boss@example.com",
		})
	}))
	defer server.Close()

	client := NewMicrosoftClientWithBaseURL(server.Client(), server.URL)
	result, err := client.Validate(context.Background(), ProviderProfile{
		ProviderKind: "microsoft",
		AuthKind:     "import_bundle",
		Payload: map[string]any{
			"mailbox_identifier":  "boss@example.com",
			"provider_identifier": "provider-42",
			"token_bundle":        "opaque-left----opaque-right",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, ValidationCodeOK, result.Code)
	require.Equal(t, "provider-42", result.ProviderIdentifier)
	require.Equal(t, "boss@example.com", result.MailboxIdentifier)
}

func TestMicrosoftClientListHeadersUsesConnectionConfigFolder(t *testing.T) {
	receivedTop := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/me/mailFolders/Archive/messages", r.URL.Path)
		require.Equal(t, "Bearer token-123", r.Header.Get("Authorization"))
		receivedTop = r.URL.Query().Get("$top")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{map[string]any{
				"id":               "mail-1",
				"subject":          "hello",
				"bodyPreview":      "preview",
				"receivedDateTime": "2026-03-29T12:00:00Z",
				"from": map[string]any{
					"emailAddress": map[string]any{"address": "sender@example.com"},
				},
				"toRecipients": []map[string]any{{
					"emailAddress": map[string]any{"address": "to@example.com"},
				}},
				"ccRecipients": []map[string]any{{
					"emailAddress": map[string]any{"address": "cc@example.com"},
				}},
				"internetMessageHeaders": []map[string]any{
					{"name": "Delivered-To", "value": "delivered@example.com"},
					{"name": "X-Original-To", "value": "original@example.com"},
				},
			}},
			"@odata.nextLink": "https://graph.example.com/next",
		})
	}))
	defer server.Close()

	client := NewMicrosoftClientWithBaseURL(server.Client(), server.URL)
	page, err := client.ListHeaders(context.Background(), ProviderProfile{
		ProviderKind: "microsoft",
		AuthKind:     "oauth2",
		Payload: map[string]any{
			"access_token": "token-123",
		},
	}, CapabilityProfile{
		Kind:             "microsoft_inbox",
		ConnectionConfig: map[string]any{"folder": "Archive"},
	}, 7)
	require.NoError(t, err)
	require.NotNil(t, page)
	require.Equal(t, "7", receivedTop)
	require.Len(t, page.Headers, 1)
	require.Equal(t, "mail-1", page.Headers[0].RemoteMessageID)
	require.Equal(t, "Archive", page.Headers[0].Folder)
	require.Equal(t, "sender@example.com", page.Headers[0].Sender)
	require.Equal(t, []string{"to@example.com", "cc@example.com"}, page.Headers[0].Recipients)
	require.Equal(t, []string{"delivered@example.com"}, page.Headers[0].DeliveredTo)
	require.Equal(t, []string{"original@example.com"}, page.Headers[0].OriginalTo)
	require.Equal(t, "https://graph.example.com/next", page.NextCursor["next_link"])
	require.Equal(t, time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC), page.Headers[0].ReceivedAt)
}

func TestMicrosoftClientListHeadersUsesCursorStateNextLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/custom/next", r.URL.Path)
		require.Equal(t, "cursor-token", r.URL.Query().Get("page"))
		require.Equal(t, "Bearer token-123", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{},
		})
	}))
	defer server.Close()

	client := NewMicrosoftClientWithBaseURL(server.Client(), server.URL)
	page, err := client.ListHeaders(context.Background(), ProviderProfile{
		ProviderKind: "microsoft",
		AuthKind:     "oauth2",
		Payload: map[string]any{
			"access_token": "token-123",
		},
	}, CapabilityProfile{
		Kind:             "microsoft_inbox",
		ConnectionConfig: map[string]any{"folder": "Inbox"},
		CursorState:      map[string]any{"next_link": server.URL + "/custom/next?page=cursor-token"},
	}, 7)
	require.NoError(t, err)
	require.NotNil(t, page)
	require.Empty(t, page.Headers)
}
