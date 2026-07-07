package app

import "sync/atomic"

type ReadinessState struct {
	databaseReady       atomic.Bool
	whatsmeowStoreReady atomic.Bool
	clientHubReady      atomic.Bool
	restorationStarted  atomic.Bool
}

func (s *ReadinessState) MarkDatabaseReady() {
	s.databaseReady.Store(true)
}

func (s *ReadinessState) MarkWhatsmeowStoreReady() {
	s.whatsmeowStoreReady.Store(true)
}

func (s *ReadinessState) MarkClientHubReady() {
	s.clientHubReady.Store(true)
}

func (s *ReadinessState) MarkRestorationStarted() {
	s.restorationStarted.Store(true)
}

func (s *ReadinessState) Ready() bool {
	return s.databaseReady.Load() &&
		s.whatsmeowStoreReady.Load() &&
		s.clientHubReady.Load() &&
		s.restorationStarted.Load()
}
