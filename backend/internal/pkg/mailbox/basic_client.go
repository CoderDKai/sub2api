package mailbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const defaultInitialIMAPWindow = 7 * 24 * time.Hour

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

type noopBasicTransport struct{}

func NewBasicClient() *BasicClient {
	return NewBasicClientWithTransport(noopBasicTransport{})
}

func NewBasicClientWithTransport(transport BasicProtocolTransport) *BasicClient {
	if transport == nil {
		transport = noopBasicTransport{}
	}
	return &BasicClient{
		transport: transport,
		now:       time.Now,
	}
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
		return &HeaderPage{Headers: dedupeHeaders(headers), NextCursor: cloneMap(capability.CursorState)}, nil
	default:
		return nil, fmt.Errorf("unsupported basic protocol %q", settings.protocol)
	}
}

func (noopBasicTransport) ValidateBasic(ctx context.Context, req BasicValidationRequest) (*ValidationResult, error) {
	return &ValidationResult{Code: ValidationCodeValidationFailed, Message: "basic transport not configured"}, nil
}

func (noopBasicTransport) ListIMAPHeaders(ctx context.Context, req IMAPListRequest) ([]Header, map[string]any, error) {
	return nil, nil, errors.New("basic transport not configured")
}

func (noopBasicTransport) ListPOP3Headers(ctx context.Context, req POP3ListRequest) ([]Header, error) {
	return nil, errors.New("basic transport not configured")
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
