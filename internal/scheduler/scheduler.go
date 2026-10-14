package scheduler

import (
	"context"
	"fmt"
	"sync"

	"upstream-x/internal/service"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	settings *service.SettingsService
	detector *service.DetectorService
	mu       sync.Mutex
	cron     *cron.Cron
}

func New(settings *service.SettingsService, detector *service.DetectorService) *Scheduler {
	return &Scheduler{
		settings: settings,
		detector: detector,
	}
}

func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	interval, err := s.settings.GetScheduleInterval()
	if err != nil {
		return err
	}

	return s.startLocked(interval)
}

func (s *Scheduler) UpdateInterval(minutes int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startLocked(minutes)
}

func (s *Scheduler) Stop() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron == nil {
		return context.Background()
	}
	ctx := s.cron.Stop()
	s.cron = nil
	return ctx
}

func (s *Scheduler) startLocked(minutes int) error {
	if s.cron != nil {
		s.cron.Stop()
	}

	spec := fmt.Sprintf("@every %dm", minutes)
	instance := cron.New()
	if _, err := instance.AddFunc(spec, func() {
		_ = s.detector.CheckAllEnabled(context.Background(), "scheduler")
	}); err != nil {
		return err
	}
	instance.Start()
	s.cron = instance
	return nil
}
