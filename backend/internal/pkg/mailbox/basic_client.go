package mailbox

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	defaultInitialIMAPWindow    = 7 * 24 * time.Hour
	pop3SeenMessageIDsCursorKey = "seen_remote_message_ids"
)

type BasicValidationRequest struct {
	Protocol string
	Host     string
	Port     int
	Username string
	Password string
}

type IMAPListRequest struct {
	Host     string
	Port     int
	Username string
	Password string
	Folder   string
	Limit    int
	Cursor   map[string]any
	Since    time.Time
	Bounded  bool
}

type POP3ListRequest struct {
	Host     string
	Port     int
	Username string
	Password string
	Folder   string
	Limit    int
	Cursor   map[string]any
}

type BasicProtocolTransport interface {
	ValidateBasic(ctx context.Context, req BasicValidationRequest) (*ValidationResult, error)
	ListIMAPHeaders(ctx context.Context, req IMAPListRequest) ([]Header, map[string]any, error)
	ListPOP3Headers(ctx context.Context, req POP3ListRequest) ([]Header, error)
}

type BasicClient struct {
	transport BasicProtocolTransport
	now       func() time.Time
}

type basicSettings struct {
	protocol string
	host     string
	port     int
	username string
	password string
	folder   string
}

type basicConfigTransport struct{}

func NewBasicProtocolTransport() BasicProtocolTransport {
	return basicConfigTransport{}
}

func NewBasicClient(transport BasicProtocolTransport) *BasicClient {
	if transport == nil {
		transport = NewBasicProtocolTransport()
	}
	return &BasicClient{
		transport: transport,
		now:       time.Now,
	}
}

func NewBasicClientWithTransport(transport BasicProtocolTransport) *BasicClient {
	return NewBasicClient(transport)
}

func (c *BasicClient) Validate(ctx context.Context, profile ProviderProfile) (*ValidationResult, error) {
	settings, err := decodeBasicSettings(profile.Payload, CapabilityProfile{})
	if err != nil {
		return &ValidationResult{
			Code:              ValidationCodeInvalidFormat,
			Message:           err.Error(),
			InvalidateAccount: true,
		}, nil
	}
	result, err := c.transport.ValidateBasic(ctx, BasicValidationRequest{
		Protocol: settings.protocol,
		Host:     settings.host,
		Port:     settings.port,
		Username: settings.username,
		Password: settings.password,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return &ValidationResult{Code: ValidationCodeOK}, nil
	}
	if strings.TrimSpace(result.Code) == "" {
		result.Code = ValidationCodeOK
	}
	return result, nil
}

func (c *BasicClient) ListHeaders(ctx context.Context, profile ProviderProfile, capability CapabilityProfile, limit int) (*HeaderPage, error) {
	settings, err := decodeBasicSettings(profile.Payload, capability)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 1
	}
	switch settings.protocol {
	case "imap":
		request := IMAPListRequest{
			Host:     settings.host,
			Port:     settings.port,
			Username: settings.username,
			Password: settings.password,
			Folder:   settings.folder,
			Limit:    limit,
			Cursor:   cloneMap(capability.CursorState),
		}
		if len(capability.CursorState) == 0 {
			request.Bounded = true
			request.Since = c.now().UTC().Add(-defaultInitialIMAPWindow)
			if capability.InitialBackfillSince != nil {
				request.Since = capability.InitialBackfillSince.UTC()
			}
			if capability.InitialBackfillPerDirection > 0 && request.Limit > capability.InitialBackfillPerDirection {
				request.Limit = capability.InitialBackfillPerDirection
			}
		}
		headers, nextCursor, err := c.transport.ListIMAPHeaders(ctx, request)
		if err != nil {
			return nil, err
		}
		return &HeaderPage{Headers: headers, NextCursor: cloneMap(nextCursor)}, nil
	case "pop3":
		headers, err := c.transport.ListPOP3Headers(ctx, POP3ListRequest{
			Host:     settings.host,
			Port:     settings.port,
			Username: settings.username,
			Password: settings.password,
			Folder:   settings.folder,
			Limit:    limit,
			Cursor:   cloneMap(capability.CursorState),
		})
		if err != nil {
			return nil, err
		}
		deduped := dedupeHeaders(headers)
		seenIDs := buildPOP3SeenIDList(capability.CursorState, deduped)
		filtered := filterPOP3HeadersBySeenIDs(deduped, capability.CursorState)
		nextCursor := cloneMap(capability.CursorState)
		if nextCursor == nil {
			nextCursor = map[string]any{}
		}
		nextCursor[pop3SeenMessageIDsCursorKey] = seenIDs
		return &HeaderPage{Headers: filtered, NextCursor: nextCursor}, nil
	default:
		return nil, fmt.Errorf("unsupported basic protocol %q", settings.protocol)
	}
}

func (basicConfigTransport) ValidateBasic(ctx context.Context, req BasicValidationRequest) (*ValidationResult, error) {
	if strings.TrimSpace(req.Protocol) == "" {
		return &ValidationResult{Code: ValidationCodeInvalidFormat, Message: "basic protocol is required", InvalidateAccount: true}, nil
	}
	if strings.TrimSpace(req.Host) == "" || strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		return &ValidationResult{Code: ValidationCodeInvalidFormat, Message: "basic provider credentials are incomplete", InvalidateAccount: true}, nil
	}
	return &ValidationResult{Code: ValidationCodeOK, MailboxIdentifier: strings.TrimSpace(req.Username)}, nil
}

func (basicConfigTransport) ListIMAPHeaders(ctx context.Context, req IMAPListRequest) ([]Header, map[string]any, error) {
	if strings.TrimSpace(req.Host) == "" || strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		return nil, nil, errors.New("basic provider credentials are incomplete")
	}
	return []Header{}, cloneMap(req.Cursor), nil
}

func (basicConfigTransport) ListPOP3Headers(ctx context.Context, req POP3ListRequest) ([]Header, error) {
	if strings.TrimSpace(req.Host) == "" || strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		return nil, errors.New("basic provider credentials are incomplete")
	}
	return []Header{}, nil
}

func decodeBasicSettings(payload map[string]any, capability CapabilityProfile) (basicSettings, error) {
	settings := basicSettings{
		protocol: strings.ToLower(stringValue(payload, "protocol")),
		host:     stringValue(payload, "host"),
		port:     intValue(payload, "port"),
		username: stringValue(payload, "username"),
		password: stringValue(payload, "password"),
		folder:   stringValue(capability.ConnectionConfig, "folder"),
	}
	if settings.protocol == "" {
		settings.protocol = inferProtocol(capability.Kind)
	}
	if settings.folder == "" {
		settings.folder = "INBOX"
	}
	if settings.protocol == "" {
		return basicSettings{}, errors.New("basic protocol is required")
	}
	if settings.host == "" || settings.username == "" || settings.password == "" {
		return basicSettings{}, errors.New("basic provider credentials are incomplete")
	}
	return settings, nil
}

func inferProtocol(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch {
	case strings.Contains(kind, "pop3"):
		return "pop3"
	case strings.Contains(kind, "imap"):
		return "imap"
	default:
		return ""
	}
}

func dedupeHeaders(headers []Header) []Header {
	seen := make(map[string]struct{}, len(headers))
	out := make([]Header, 0, len(headers))
	for _, header := range headers {
		key := strings.TrimSpace(header.RemoteMessageID)
		if key != "" {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
		}
		out = append(out, header)
	}
	return out
}

func filterPOP3HeadersBySeenIDs(headers []Header, cursor map[string]any) []Header {
	seen := make(map[string]struct{})
	for _, id := range stringSliceValue(cursor, pop3SeenMessageIDsCursorKey) {
		if id != "" {
			seen[id] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return headers
	}
	filtered := make([]Header, 0, len(headers))
	for _, header := range headers {
		id := strings.TrimSpace(header.RemoteMessageID)
		if id != "" {
			if _, ok := seen[id]; ok {
				continue
			}
		}
		filtered = append(filtered, header)
	}
	return filtered
}

func buildPOP3SeenIDList(cursor map[string]any, headers []Header) []string {
	seen := make(map[string]struct{})
	ordered := make([]string, 0)
	appendID := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ordered = append(ordered, id)
	}
	for _, id := range stringSliceValue(cursor, pop3SeenMessageIDsCursorKey) {
		appendID(id)
	}
	for _, header := range headers {
		appendID(header.RemoteMessageID)
	}
	sort.Strings(ordered)
	return ordered
}
