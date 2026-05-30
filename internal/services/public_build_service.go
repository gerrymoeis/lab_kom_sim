package services

import (
	"fmt"
	"log"
	"sync"
	"time"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
)

type PublicBuildService struct {
	db      *database.DB
	cfg     config.PublicBuildConfig
	trigger chan struct{}
	stop    chan struct{}
	done    chan struct{}
	mu      sync.Mutex
}

func NewPublicBuildService(db *database.DB, cfg config.PublicBuildConfig) *PublicBuildService {
	return &PublicBuildService{
		db:   db,
		cfg:  cfg,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func (s *PublicBuildService) NotifyChange() {
	if s.trigger == nil {
		return
	}
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}

func (s *PublicBuildService) Start() {
	if !s.cfg.Enabled {
		log.Println("PublicBuild: disabled via PUBLIC_BUILD_ENABLED=false")
		return
	}
	s.trigger = make(chan struct{}, 1)
	log.Printf("PublicBuild: started — debounce=%ds out=%s repo=%s branch=%s",
		s.cfg.Interval, s.cfg.OutDir, s.cfg.RepoDir, s.cfg.Branch)

	go func() {
		defer close(s.done)
		for {
			select {
			case <-s.stop:
				return
			case <-s.trigger:
				s.debounceLoop()
			}
		}
	}()
}

func (s *PublicBuildService) debounceLoop() {
	for {
		timer := time.NewTimer(time.Duration(s.cfg.Interval) * time.Second)
		select {
		case <-timer.C:
			timer.Stop()
			s.runBuild()
			return
		case <-s.trigger:
			timer.Stop()
			select {
			case <-timer.C:
			default:
			}
			continue
		case <-s.stop:
			timer.Stop()
			select {
			case <-timer.C:
			default:
			}
			return
		}
	}
}

func (s *PublicBuildService) runBuild() {
	if !s.mu.TryLock() {
		log.Println("PublicBuild: previous build still running, skipping this cycle")
		return
	}
	defer s.mu.Unlock()

	if err := RunPublicBuild(s.db, s.cfg); err != nil {
		log.Printf("PublicBuild: FAILED — %v", err)
	} else {
		log.Println("PublicBuild: SUCCESS")
	}
}

func (s *PublicBuildService) Stop() {
	if !s.cfg.Enabled {
		return
	}
	close(s.stop)
	<-s.done
	log.Println("PublicBuild: stopped")
}

func (s *PublicBuildService) BuildNow() error {
	if !s.cfg.Enabled {
		return fmt.Errorf("public build is disabled")
	}
	if !s.mu.TryLock() {
		return fmt.Errorf("build already running")
	}
	defer s.mu.Unlock()
	return RunPublicBuild(s.db, s.cfg)
}

type MultiNotifier struct {
	notifiers []CUDNotifier
}

func NewMultiNotifier(n ...CUDNotifier) *MultiNotifier {
	return &MultiNotifier{notifiers: n}
}

func (m *MultiNotifier) NotifyChange() {
	for _, n := range m.notifiers {
		n.NotifyChange()
	}
}
