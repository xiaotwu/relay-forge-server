package scheduler

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/relay-forge/relay-forge/services/worker/internal/config"
	"github.com/relay-forge/relay-forge/services/worker/internal/jobs"
)

type Scheduler struct {
	jobs []Job
}

type Job struct {
	Name     string
	Interval time.Duration
	Fn       func(ctx context.Context) error
}

func New(cfg *config.Config) *Scheduler {
	s := &Scheduler{}

	s.jobs = []Job{
		{Name: "cleanup_expired_sessions", Interval: 1 * time.Hour, Fn: jobs.CleanupExpiredSessions(cfg)},
		{Name: "cleanup_expired_invites", Interval: 6 * time.Hour, Fn: jobs.CleanupExpiredInvites(cfg)},
		{Name: "cleanup_expired_password_resets", Interval: 1 * time.Hour, Fn: jobs.CleanupExpiredPasswordResets(cfg)},
		{Name: "archive_old_audit_logs", Interval: 24 * time.Hour, Fn: jobs.ArchiveOldAuditLogs(cfg)},
		{Name: "process_pending_file_scans", Interval: 5 * time.Minute, Fn: jobs.ProcessPendingFileScans(cfg)},
		{Name: "enforce_data_retention", Interval: 24 * time.Hour, Fn: jobs.EnforceDataRetention(cfg)},
	}

	return s
}

func (s *Scheduler) Start(ctx context.Context) {
	log.Info().Int("job_count", len(s.jobs)).Msg("scheduler started")

	for _, job := range s.jobs {
		go s.runJob(ctx, job)
	}

	<-ctx.Done()
	log.Info().Msg("scheduler stopping")
}

func (s *Scheduler) runJob(ctx context.Context, job Job) {
	// Run once immediately
	log.Info().Str("job", job.Name).Msg("running initial execution")
	if err := job.Fn(ctx); err != nil {
		log.Error().Err(err).Str("job", job.Name).Msg("job failed")
	}

	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Debug().Str("job", job.Name).Msg("running scheduled execution")
			if err := job.Fn(ctx); err != nil {
				log.Error().Err(err).Str("job", job.Name).Msg("job failed")
			}
		}
	}
}
