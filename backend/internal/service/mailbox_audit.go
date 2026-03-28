package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type MailboxAuditor interface {
	RecordProviderCreate(ctx context.Context, account *ProviderAccount)
	RecordProviderUpdate(ctx context.Context, before, after *ProviderAccount)
	RecordProviderStatus(ctx context.Context, account *ProviderAccount, reason string)
	RecordProviderImport(ctx context.Context, account *ProviderAccount)
	RecordCollectorUpdate(ctx context.Context, collector *CollectorMailbox)
	RecordRecipientUpdate(ctx context.Context, identity *RecipientIdentity)
	RecordCapabilityTest(ctx context.Context, capability *MailboxCapability, success bool)
	RecordManualSync(ctx context.Context, capabilityID int64)
	RecordBatchSync(ctx context.Context, batchID string, capabilityIDs []int64)
	RecordInboxDetailFetch(ctx context.Context, headerID int64)
}

type MailboxAuditLogger struct{}

func NewMailboxAuditLogger() MailboxAuditor {
	return &MailboxAuditLogger{}
}

func (a *MailboxAuditLogger) RecordProviderCreate(ctx context.Context, account *ProviderAccount) {
	logger.LegacyPrintf("service.mailbox", "audit provider_create id=%d kind=%s", providerID(account), providerKind(account))
}

func (a *MailboxAuditLogger) RecordProviderUpdate(ctx context.Context, before, after *ProviderAccount) {
	logger.LegacyPrintf("service.mailbox", "audit provider_update id=%d kind=%s", providerID(after), providerKind(after))
}

func (a *MailboxAuditLogger) RecordProviderStatus(ctx context.Context, account *ProviderAccount, reason string) {
	logger.LegacyPrintf("service.mailbox", "audit provider_status id=%d status=%s reason=%s", providerID(account), providerStatus(account), reason)
}

func (a *MailboxAuditLogger) RecordProviderImport(ctx context.Context, account *ProviderAccount) {
	logger.LegacyPrintf("service.mailbox", "audit provider_import id=%d kind=%s", providerID(account), providerKind(account))
}

func (a *MailboxAuditLogger) RecordCollectorUpdate(ctx context.Context, collector *CollectorMailbox) {
	logger.LegacyPrintf("service.mailbox", "audit collector_update id=%d", collectorID(collector))
}

func (a *MailboxAuditLogger) RecordRecipientUpdate(ctx context.Context, identity *RecipientIdentity) {
	logger.LegacyPrintf("service.mailbox", "audit recipient_update id=%d", recipientID(identity))
}

func (a *MailboxAuditLogger) RecordCapabilityTest(ctx context.Context, capability *MailboxCapability, success bool) {
	logger.LegacyPrintf("service.mailbox", "audit capability_test id=%d success=%t", capabilityID(capability), success)
}

func (a *MailboxAuditLogger) RecordManualSync(ctx context.Context, capabilityID int64) {
	logger.LegacyPrintf("service.mailbox", "audit manual_sync capability_id=%d", capabilityID)
}

func (a *MailboxAuditLogger) RecordBatchSync(ctx context.Context, batchID string, capabilityIDs []int64) {
	logger.LegacyPrintf("service.mailbox", "audit batch_sync batch_id=%s capability_count=%d", batchID, len(capabilityIDs))
}

func (a *MailboxAuditLogger) RecordInboxDetailFetch(ctx context.Context, headerID int64) {
	logger.LegacyPrintf("service.mailbox", "audit inbox_detail_fetch header_id=%d", headerID)
}

func providerID(account *ProviderAccount) int64 {
	if account == nil {
		return 0
	}
	return account.ID
}

func providerKind(account *ProviderAccount) string {
	if account == nil {
		return ""
	}
	return account.ProviderKind
}

func providerStatus(account *ProviderAccount) string {
	if account == nil {
		return ""
	}
	return account.Status
}

func collectorID(collector *CollectorMailbox) int64 {
	if collector == nil {
		return 0
	}
	return collector.ID
}

func recipientID(identity *RecipientIdentity) int64 {
	if identity == nil {
		return 0
	}
	return identity.ID
}

func capabilityID(capability *MailboxCapability) int64 {
	if capability == nil {
		return 0
	}
	return capability.ID
}
