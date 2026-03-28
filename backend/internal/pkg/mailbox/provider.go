package mailbox

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	ValidationCodeOK               = "ok"
	ValidationCodeInvalidFormat    = "invalid_format"
	ValidationCodeValidationFailed = "validation_failed"
	ValidationCodeExpiredOrRevoked = "expired_or_revoked"
)

var ErrMailboxImportFormat = errors.New("mailbox import format")

type OutlookImportBundle struct {
	MailboxIdentifier  string
	ProviderIdentifier string
	TokenBundle        string
}

type ProviderProfile struct {
	ProviderKind       string
	AuthKind           string
	Payload            map[string]any
	MailboxIdentifier  string
	ProviderIdentifier string
}

type CapabilityProfile struct {
	Kind             string
	ConnectionConfig map[string]any
	CursorState      map[string]any
}

type ValidationResult struct {
	Code              string
	Message           string
	ProviderIdentifier string
	MailboxIdentifier string
	InvalidateAccount bool
}

type Header struct {
	RemoteMessageID    string
	Folder             string
	Sender             string
	Recipients         []string
	Subject            string
	ReceivedAt         time.Time
	Flags              []string
	Snippet            string
	EnvelopeRecipients []string
	DeliveredTo        []string
	OriginalTo         []string
}

type HeaderPage struct {
	Headers    []Header
	NextCursor map[string]any
}

type ProviderClient interface {
	Validate(ctx context.Context, profile ProviderProfile) (*ValidationResult, error)
	ListHeaders(ctx context.Context, profile ProviderProfile, capability CapabilityProfile, limit int) (*HeaderPage, error)
}

func ParseOutlookImportBundle(raw string) (*OutlookImportBundle, error) {
	parts := strings.Split(strings.TrimSpace(raw), "----")
	if len(parts) != 4 {
		return nil, ErrMailboxImportFormat
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
		if parts[i] == "" {
			return nil, ErrMailboxImportFormat
		}
	}
	return &OutlookImportBundle{
		MailboxIdentifier:  parts[0],
		ProviderIdentifier: parts[1],
		TokenBundle:        parts[2] + "----" + parts[3],
	}, nil
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func stringValue(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func intValue(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func stringSliceValue(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok {
		return nil
	}
	slice, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(slice))
	for _, item := range slice {
		s, ok := item.(string)
		if ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}
