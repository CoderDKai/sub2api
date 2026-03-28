//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMailboxResolutionServiceExactAddressBeatsDomainSuffix(t *testing.T) {
	repo := newMailboxRepositoryStub()
	repo.identities[1] = &RecipientIdentity{ID: 1, Name: "alias", Enabled: true}
	repo.matchValues[1] = []*RecipientMatchValue{{
		ID:                  11,
		RecipientIdentityID: 1,
		MatchType:           RecipientMatchTypeExactAddress,
		MatchValue:          "alias@example.com",
		NormalizedValue:     "alias@example.com",
		Active:              true,
		Priority:            100,
	}}
	repo.identities[2] = &RecipientIdentity{ID: 2, Name: "domain", Enabled: true}
	repo.matchValues[2] = []*RecipientMatchValue{{
		ID:                  22,
		RecipientIdentityID: 2,
		MatchType:           RecipientMatchTypeDomainSuffix,
		MatchValue:          "example.com",
		NormalizedValue:     "example.com",
		Active:              true,
		Priority:            10,
	}}

	service := NewMailboxResolutionService(repo)
	result, err := service.Resolve(context.Background(), MailboxRecipientResolutionInput{
		EnvelopeRecipients: []string{"alias@example.com"},
	})
	require.NoError(t, err)
	require.Equal(t, MailResolutionStateResolved, result.State)
	require.Equal(t, int64(1), result.Identity.ID)
	require.Equal(t, RecipientMatchTypeExactAddress, result.MatchValue.MatchType)
	require.Equal(t, MailboxResolutionFieldEnvelope, result.SourceField)
}

func TestMailboxResolutionServiceAmbiguousHighPriorityFieldReturnsAmbiguous(t *testing.T) {
	repo := newMailboxRepositoryStub()
	repo.identities[1] = &RecipientIdentity{ID: 1, Name: "alerts", Enabled: true}
	repo.matchValues[1] = []*RecipientMatchValue{{
		ID:                  11,
		RecipientIdentityID: 1,
		MatchType:           RecipientMatchTypeDomainSuffix,
		MatchValue:          "alerts.example.com",
		NormalizedValue:     "alerts.example.com",
		Active:              true,
		Priority:            100,
	}}
	repo.identities[2] = &RecipientIdentity{ID: 2, Name: "fallback", Enabled: true}
	repo.matchValues[2] = []*RecipientMatchValue{{
		ID:                  22,
		RecipientIdentityID: 2,
		MatchType:           RecipientMatchTypeDomainSuffix,
		MatchValue:          "example.com",
		NormalizedValue:     "example.com",
		Active:              true,
		Priority:            90,
	}}

	service := NewMailboxResolutionService(repo)
	result, err := service.Resolve(context.Background(), MailboxRecipientResolutionInput{
		EnvelopeRecipients: []string{"team@alerts.example.com"},
		To:                 []string{"alias@example.com"},
	})
	require.NoError(t, err)
	require.Equal(t, MailResolutionStateAmbiguous, result.State)
	require.Nil(t, result.Identity)
	require.Equal(t, MailboxResolutionFieldEnvelope, result.SourceField)
}
