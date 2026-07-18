package service

import (
	"context"
	"sync"
	"time"

	"homelab-api/internal/beszel"
	"homelab-api/internal/domain"
)

type BeszelProvider interface {
	Configured() bool
	Fetch(context.Context) (beszel.RawData, map[string]string)
}

type KumaProvider interface {
	Configured() bool
	Fetch(context.Context) ([]domain.Service, error)
}

type Store struct {
	beszel BeszelProvider
	kuma   KumaProvider

	mu          sync.RWMutex
	snapshot    domain.Snapshot
	refreshDone bool
	subscribers map[chan domain.Snapshot]struct{}
}

func NewStore(beszelProvider BeszelProvider, kumaProvider KumaProvider) *Store {
	return &Store{
		beszel:      beszelProvider,
		kuma:        kumaProvider,
		subscribers: make(map[chan domain.Snapshot]struct{}),
	}
}

func (s *Store) Run(ctx context.Context, interval time.Duration) {
	s.Refresh(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Refresh(ctx)
		}
	}
}

func (s *Store) Refresh(ctx context.Context) {
	now := time.Now().UTC()
	snapshot := domain.Snapshot{
		GeneratedAt: now,
		Sources:     make([]domain.SourceStatus, 0, 2),
		Systems:     []domain.System{},
		Containers:  []domain.Container{},
		Services:    []domain.Service{},
		Alerts:      []domain.Alert{},
	}

	if s.beszel != nil {
		source := domain.SourceStatus{Name: "beszel", Configured: s.beszel.Configured(), Status: "not_configured"}
		if source.Configured {
			data, errorsByCollection := s.beszel.Fetch(ctx)
			snapshot.Systems = normalizeSystems(data)
			snapshot.Containers = normalizeContainers(data)
			snapshot.Alerts = normalizeAlerts(data.Alerts)
			if len(errorsByCollection) == 0 {
				source.Status = "ok"
				source.LastSuccessAt = &now
			} else if len(data.Systems) > 0 || len(data.SystemStats) > 0 || len(data.Containers) > 0 || len(data.ContainerStats) > 0 || len(data.Realtime) > 0 || len(data.Alerts) > 0 {
				source.Status = "degraded"
				source.Error = sourceError(errorsByCollection)
			} else {
				source.Status = "error"
				source.Error = "data unavailable"
			}
		}
		snapshot.Sources = append(snapshot.Sources, source)
	}

	if s.kuma != nil {
		source := domain.SourceStatus{Name: "uptime_kuma", Configured: s.kuma.Configured(), Status: "not_configured"}
		if source.Configured {
			services, err := s.kuma.Fetch(ctx)
			if services == nil {
				services = []domain.Service{}
			}
			snapshot.Services = services
			if err == nil {
				source.Status = "ok"
				source.LastSuccessAt = &now
			} else {
				source.Status = "error"
				source.Error = "metrics unavailable"
			}
		}
		snapshot.Sources = append(snapshot.Sources, source)
	}

	summarize(&snapshot)
	s.mu.Lock()
	s.snapshot = snapshot
	s.refreshDone = true
	for subscriber := range s.subscribers {
		select {
		case subscriber <- snapshot:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *Store) Snapshot() domain.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

func (s *Store) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.refreshDone {
		return false
	}
	for _, source := range s.snapshot.Sources {
		if source.Configured && (source.Status == "ok" || source.Status == "degraded") {
			return true
		}
	}
	return false
}

func (s *Store) Subscribe() (<-chan domain.Snapshot, func()) {
	channel := make(chan domain.Snapshot, 1)
	s.mu.Lock()
	s.subscribers[channel] = struct{}{}
	current := s.snapshot
	s.mu.Unlock()
	if !current.GeneratedAt.IsZero() {
		channel <- current
	}
	return channel, func() {
		s.mu.Lock()
		if _, ok := s.subscribers[channel]; ok {
			delete(s.subscribers, channel)
			close(channel)
		}
		s.mu.Unlock()
	}
}
