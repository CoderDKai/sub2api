package service

import (
	"context"
	"net/mail"
	"strings"
)

const (
	MailboxResolutionFieldEnvelope    = "envelope"
	MailboxResolutionFieldDeliveredTo = "delivered_to"
	MailboxResolutionFieldXOriginalTo = "x_original_to"
	MailboxResolutionFieldTo          = "to"
	MailboxResolutionFieldCC          = "cc"
)

type MailboxRecipientResolutionInput struct {
	EnvelopeRecipients []string
	DeliveredTo        []string
	XOriginalTo        []string
	To                 []string
	CC                 []string
}

type MailboxRecipientResolutionResult struct {
	State       string
	SourceField string
	Address     string
	Identity    *RecipientIdentity
	MatchValue  *RecipientMatchValue
}

type MailboxResolutionService struct {
	repo MailboxRepository
}

type mailboxResolutionEntry struct {
	identity *RecipientIdentity
	value    *RecipientMatchValue
}

type mailboxFieldResolution struct {
	address   string
	identities map[int64]mailboxResolutionEntry
}

func NewMailboxResolutionService(repo MailboxRepository) *MailboxResolutionService {
	return &MailboxResolutionService{repo: repo}
}

func (s *MailboxResolutionService) Resolve(ctx context.Context, input MailboxRecipientResolutionInput) (*MailboxRecipientResolutionResult, error) {
	entries, err := s.loadResolutionEntries(ctx)
	if err != nil {
		return nil, err
	}
	fields := []struct {
		name      string
		addresses []string
	}{
		{name: MailboxResolutionFieldEnvelope, addresses: input.EnvelopeRecipients},
		{name: MailboxResolutionFieldDeliveredTo, addresses: input.DeliveredTo},
		{name: MailboxResolutionFieldXOriginalTo, addresses: input.XOriginalTo},
		{name: MailboxResolutionFieldTo, addresses: input.To},
		{name: MailboxResolutionFieldCC, addresses: input.CC},
	}
	for _, field := range fields {
		resolved := resolveMailboxField(field.addresses, entries)
		if resolved == nil {
			continue
		}
		if len(resolved.identities) > 1 {
			return &MailboxRecipientResolutionResult{State: MailResolutionStateAmbiguous, SourceField: field.name, Address: resolved.address}, nil
		}
		for _, entry := range resolved.identities {
			return &MailboxRecipientResolutionResult{
				State:       MailResolutionStateResolved,
				SourceField: field.name,
				Address:     resolved.address,
				Identity:    entry.identity,
				MatchValue:  entry.value,
			}, nil
		}
	}
	return &MailboxRecipientResolutionResult{State: MailResolutionStateUnresolved}, nil
}

func (s *MailboxResolutionService) loadResolutionEntries(ctx context.Context) ([]mailboxResolutionEntry, error) {
	identities, err := s.repo.ListRecipientIdentities(ctx, MailboxListOptions{})
	if err != nil {
		return nil, err
	}
	entries := make([]mailboxResolutionEntry, 0)
	for _, identity := range identities {
		if identity == nil || !identity.Enabled {
			continue
		}
		values, err := s.repo.ListRecipientMatchValues(ctx, identity.ID)
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			if value == nil || !value.Active {
				continue
			}
			entries = append(entries, mailboxResolutionEntry{identity: identity, value: value})
		}
	}
	return entries, nil
}

func resolveMailboxField(rawAddresses []string, entries []mailboxResolutionEntry) *mailboxFieldResolution {
	addresses := normalizeMailboxAddresses(rawAddresses)
	exact := collectMailboxMatches(addresses, entries, RecipientMatchTypeExactAddress)
	if exact != nil {
		return exact
	}
	return collectMailboxMatches(addresses, entries, RecipientMatchTypeDomainSuffix)
}

func collectMailboxMatches(addresses []string, entries []mailboxResolutionEntry, matchType string) *mailboxFieldResolution {
	for _, address := range addresses {
		resolved := &mailboxFieldResolution{address: address, identities: map[int64]mailboxResolutionEntry{}}
		for _, entry := range entries {
			if !mailboxMatchAddress(address, entry.value, matchType) {
				continue
			}
			existing, ok := resolved.identities[entry.identity.ID]
			if !ok || entry.value.Priority > existing.value.Priority {
				resolved.identities[entry.identity.ID] = entry
			}
		}
		if len(resolved.identities) > 0 {
			return resolved
		}
	}
	return nil
}

func mailboxMatchAddress(address string, value *RecipientMatchValue, matchType string) bool {
	if value == nil || value.MatchType != matchType {
		return false
	}
	normalizedAddress := strings.ToLower(strings.TrimSpace(address))
	normalizedValue := strings.ToLower(strings.TrimSpace(value.NormalizedValue))
	switch matchType {
	case RecipientMatchTypeExactAddress:
		return normalizedAddress == normalizedValue
	case RecipientMatchTypeDomainSuffix:
		domain := normalizedAddress
		if at := strings.LastIndex(normalizedAddress, "@"); at >= 0 {
			domain = normalizedAddress[at+1:]
		}
		normalizedValue = strings.TrimPrefix(normalizedValue, "@")
		return domain == normalizedValue || strings.HasSuffix(domain, "."+normalizedValue)
	default:
		return false
	}
}

func normalizeMailboxAddresses(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parsed, err := mail.ParseAddressList(value)
		if err == nil && len(parsed) > 0 {
			for _, address := range parsed {
				normalized := strings.ToLower(strings.TrimSpace(address.Address))
				if normalized != "" {
					out = append(out, normalized)
				}
			}
			continue
		}
		out = append(out, strings.ToLower(value))
	}
	return out
}
