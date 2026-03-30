package service

import (
	"context"
	"errors"
	"time"
)

type MailboxSyncRunnerService struct {
	repo    MailboxRepository
	syncSvc *MailboxSyncService
	now     func() time.Time
}

func NewMailboxSyncRunnerService(repo MailboxRepository, syncSvc *MailboxSyncService) *MailboxSyncRunnerService {
	return &MailboxSyncRunnerService{
		repo:    repo,
		syncSvc: syncSvc,
		now:     time.Now,
	}
}

func (s *MailboxSyncRunnerService) RunDue(ctx context.Context, limit int) ([]*MailSyncJob, error) {
	now := s.now().UTC()
	capabilities, err := s.repo.ClaimDueCapabilities(ctx, now, limit)
	if err != nil {
		return nil, err
	}
	if len(capabilities) == 0 {
		return []*MailSyncJob{}, nil
	}
	capabilityIDs := make([]int64, 0, len(capabilities))
	for _, capability := range capabilities {
		if capability == nil {
			continue
		}
		capabilityIDs = append(capabilityIDs, capability.ID)
	}
	jobs, err := s.syncSvc.CreateBatchSyncJobs(ctx, MailboxBatchSyncRequest{
		CapabilityIDs: capabilityIDs,
		TriggerSource: MailSyncTriggerSourceSchedule,
		ScheduledFor:  &now,
	})
	if err != nil {
		if requeueErr := s.requeueCapabilities(ctx, capabilities, now); requeueErr != nil {
			return nil, errors.Join(err, requeueErr)
		}
		return nil, err
	}
	for _, job := range jobs {
		if job == nil {
			continue
		}
		if _, err := s.syncSvc.RunSyncJob(ctx, job.ID); err != nil {
			return jobs, err
		}
	}
	return jobs, nil
}

func (s *MailboxSyncRunnerService) requeueCapabilities(ctx context.Context, capabilities []*MailboxCapability, now time.Time) error {
	for _, capability := range capabilities {
		if capability == nil {
			continue
		}
		updated := cloneMailboxCapabilityValue(capability)
		updated.NextSyncAt = cloneTimePointer(&now)
		if _, err := s.repo.UpdateCapability(ctx, updated); err != nil {
			return err
		}
	}
	return nil
}
