package trending

import (
	"context"
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"

	"erg.ninja/pkg/logger"
)

type Scheduler struct {
	cron *cron.Cron
	svc  *Service
	log  *logger.Logger
	spec string
}

func NewScheduler(svc *Service, log *logger.Logger, spec string) *Scheduler {
	if log == nil {
		log = logger.NoOp()
	}
	return &Scheduler{
		cron: cron.New(cron.WithSeconds()),
		svc:  svc,
		log:  log,
		spec: normalizeCronSpec(spec),
	}
}

func (s *Scheduler) Start() error {
	if s.svc == nil {
		return nil
	}
	if _, err := s.cron.AddFunc(s.spec, func() {
		ctx := context.Background()
		if _, err := s.svc.Refresh(ctx); err != nil {
			s.log.Error().Err(err).Msg("trending: scheduled refresh failed")
		}
	}); err != nil {
		return fmt.Errorf("trending scheduler add func: %w", err)
	}
	s.cron.Start()
	return nil
}

func (s *Scheduler) Stop() context.Context {
	if s == nil || s.cron == nil {
		return context.Background()
	}
	return s.cron.Stop()
}

func normalizeCronSpec(spec string) string {
	if spec == "" {
		return "0 */30 * * * *"
	}
	if len(strings.Fields(spec)) == 5 {
		return "0 " + spec
	}
	return spec
}
