package mailbox

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type basicTransportStub struct {
	imapRequests []IMAPListRequest
	pop3Requests []POP3ListRequest
	imapHeaders  []Header
	pop3Headers  []Header
	nextCursor   map[string]any
	validateResp *ValidationResult
	validateErr  error
	listErr      error
}

func (s *basicTransportStub) ValidateBasic(ctx context.Context, req BasicValidationRequest) (*ValidationResult, error) {
	if s.validateErr != nil {
		return nil, s.validateErr
	}
	if s.validateResp != nil {
		return s.validateResp, nil
	}
	return &ValidationResult{}, nil
}

func (s *basicTransportStub) ListIMAPHeaders(ctx context.Context, req IMAPListRequest) ([]Header, map[string]any, error) {
	s.imapRequests = append(s.imapRequests, req)
	if s.listErr != nil {
		return nil, nil, s.listErr
	}
	return append([]Header(nil), s.imapHeaders...), cloneMap(s.nextCursor), nil
}

func (s *basicTransportStub) ListPOP3Headers(ctx context.Context, req POP3ListRequest) ([]Header, error) {
	s.pop3Requests = append(s.pop3Requests, req)
	if s.listErr != nil {
		return nil, s.listErr
	}
	return append([]Header(nil), s.pop3Headers...), nil
}

func TestBasicClientIMAPUsesBoundedRequestOnInitialListHeaders(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	transport := &basicTransportStub{}
	client := NewBasicClientWithTransport(transport)
	client.now = func() time.Time { return fixedNow }

	page, err := client.ListHeaders(context.Background(), ProviderProfile{
		ProviderKind: "basic",
		AuthKind:     "basic",
		Payload: map[string]any{
			"protocol": "imap",
			"host":     "imap.example.com",
			"username": "alice@example.com",
			"password": "secret",
		},
	}, CapabilityProfile{
		Kind:             "imap",
		ConnectionConfig: map[string]any{"folder": "INBOX"},
	}, 25)
	require.NoError(t, err)
	require.NotNil(t, page)
	require.Len(t, transport.imapRequests, 1)
	require.Empty(t, transport.pop3Requests)
	require.True(t, transport.imapRequests[0].Bounded)
	require.Equal(t, fixedNow.Add(-defaultInitialIMAPWindow), transport.imapRequests[0].Since)
	require.Equal(t, 25, transport.imapRequests[0].Limit)
	require.Equal(t, "INBOX", transport.imapRequests[0].Folder)
}

func TestBasicClientPOP3DeduplicatesRemoteMessageIDs(t *testing.T) {
	transport := &basicTransportStub{
		pop3Headers: []Header{
			{RemoteMessageID: "msg-1", Subject: "first"},
			{RemoteMessageID: "msg-1", Subject: "duplicate"},
			{RemoteMessageID: "msg-2", Subject: "second"},
		},
	}
	client := NewBasicClientWithTransport(transport)

	page, err := client.ListHeaders(context.Background(), ProviderProfile{
		ProviderKind: "basic",
		AuthKind:     "basic",
		Payload: map[string]any{
			"protocol": "pop3",
			"host":     "pop3.example.com",
			"username": "alice@example.com",
			"password": "secret",
		},
	}, CapabilityProfile{
		Kind:             "pop3",
		ConnectionConfig: map[string]any{"folder": "INBOX"},
	}, 10)
	require.NoError(t, err)
	require.NotNil(t, page)
	require.Len(t, transport.pop3Requests, 1)
	require.Len(t, page.Headers, 2)
	require.Equal(t, "msg-1", page.Headers[0].RemoteMessageID)
	require.Equal(t, "msg-2", page.Headers[1].RemoteMessageID)
	require.Equal(t, []string{"msg-1", "msg-2"}, page.NextCursor[pop3SeenMessageIDsCursorKey])
}

func TestBasicClientPOP3FiltersPreviouslySeenMessagesAndAdvancesCursor(t *testing.T) {
	transport := &basicTransportStub{
		pop3Headers: []Header{
			{RemoteMessageID: "msg-1", Subject: "already seen"},
			{RemoteMessageID: "msg-2", Subject: "new"},
		},
	}
	client := NewBasicClientWithTransport(transport)

	page, err := client.ListHeaders(context.Background(), ProviderProfile{
		ProviderKind: "basic",
		AuthKind:     "basic",
		Payload: map[string]any{
			"protocol": "pop3",
			"host":     "pop3.example.com",
			"username": "alice@example.com",
			"password": "secret",
		},
	}, CapabilityProfile{
		Kind:             "pop3",
		ConnectionConfig: map[string]any{"folder": "INBOX"},
		CursorState:      map[string]any{pop3SeenMessageIDsCursorKey: []string{"msg-1"}},
	}, 10)
	require.NoError(t, err)
	require.Len(t, page.Headers, 1)
	require.Equal(t, "msg-2", page.Headers[0].RemoteMessageID)
	require.Equal(t, []string{"msg-1", "msg-2"}, page.NextCursor[pop3SeenMessageIDsCursorKey])
}

func TestBasicConfigTransportMakesBasicClientMinimallyUsable(t *testing.T) {
	client := NewBasicClient(NewBasicProtocolTransport())

	result, err := client.Validate(context.Background(), ProviderProfile{
		ProviderKind: "basic",
		AuthKind:     "basic",
		Payload: map[string]any{
			"protocol": "imap",
			"host":     "imap.example.com",
			"username": "alice@example.com",
			"password": "secret",
		},
	})
	require.NoError(t, err)
	require.Equal(t, ValidationCodeOK, result.Code)

	page, err := client.ListHeaders(context.Background(), ProviderProfile{
		ProviderKind: "basic",
		AuthKind:     "basic",
		Payload: map[string]any{
			"protocol": "imap",
			"host":     "imap.example.com",
			"username": "alice@example.com",
			"password": "secret",
		},
	}, CapabilityProfile{
		Kind:             "imap",
		ConnectionConfig: map[string]any{"folder": "INBOX"},
	}, 5)
	require.NoError(t, err)
	require.NotNil(t, page)
	require.Empty(t, page.Headers)
}
