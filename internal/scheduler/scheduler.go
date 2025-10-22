package scheduler

import (
	"context"
	"log/slog"
	"time"
)

type SurveyLifecycleProvider interface {
	ActivateScheduledSurveys(ctx context.Context, now time.Time) (int64, error)
	ArchiveExpiredSurveys(ctx context.Context, now time.Time) (int64, error)
}

type SurveyScheduler struct {
	provider SurveyLifecycleProvider
	interval time.Duration
}

func NewSurveyScheduler(provider SurveyLifecycleProvider, interval time.Duration) *SurveyScheduler {
	if interval <= 0 {
		interval = time.Minute
	}
	return &SurveyScheduler{provider: provider, interval: interval}
}

func (s *SurveyScheduler) Start(ctx context.Context) {
	if s.provider == nil {
		slog.Warn("survey scheduler skipped: no provider configured")
		return
	}
	ticker := time.NewTicker(s.interval)
	go func() {
		defer ticker.Stop()
		s.run(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.run(ctx)
			}
		}
	}()
}

func (s *SurveyScheduler) run(ctx context.Context) {
	now := time.Now().UTC()
	if opened, err := s.provider.ActivateScheduledSurveys(ctx, now); err != nil {
		slog.Error("activate scheduled surveys failed", "err", err)
	} else if opened > 0 {
		slog.Info("surveys activated", "count", opened)
	}
	if archived, err := s.provider.ArchiveExpiredSurveys(ctx, now); err != nil {
		slog.Error("archive expired surveys failed", "err", err)
	} else if archived > 0 {
		slog.Info("surveys archived", "count", archived)
	}
}
