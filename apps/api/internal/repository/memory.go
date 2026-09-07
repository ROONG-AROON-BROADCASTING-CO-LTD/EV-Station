package repository

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

type Memory struct {
	mu        sync.RWMutex
	sites     map[uuid.UUID]domain.Site
	images    map[uuid.UUID][]domain.SiteImage
	analyses  map[uuid.UUID]domain.AnalysisRun
	users     map[uuid.UUID]domain.User
	passwords map[uuid.UUID]string
	access    map[uuid.UUID][]domain.SiteAccess
}

func NewMemory() *Memory {
	return &Memory{sites: make(map[uuid.UUID]domain.Site), images: make(map[uuid.UUID][]domain.SiteImage), analyses: make(map[uuid.UUID]domain.AnalysisRun), users: make(map[uuid.UUID]domain.User), passwords: make(map[uuid.UUID]string), access: make(map[uuid.UUID][]domain.SiteAccess)}
}

func (m *Memory) CreateUser(_ context.Context, user domain.User, passwordHash string) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.users {
		if existing.Email == user.Email {
			return domain.User{}, errorString("email already exists")
		}
	}
	m.users[user.ID] = user
	m.passwords[user.ID] = passwordHash
	return user, nil
}
func (m *Memory) GetUserByEmail(_ context.Context, email string) (domain.User, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for id, user := range m.users {
		if user.Email == email {
			return user, m.passwords[id], nil
		}
	}
	return domain.User{}, "", ErrNotFound
}
func (m *Memory) ListUsers(_ context.Context) ([]domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.User, 0, len(m.users))
	for _, user := range m.users {
		result = append(result, user)
	}
	return result, nil
}
func (m *Memory) SetSiteAccess(_ context.Context, access domain.SiteAccess) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sites[access.SiteID]; !ok {
		return ErrNotFound
	}
	for i, item := range m.access[access.SiteID] {
		if item.UserID == access.UserID {
			m.access[access.SiteID][i] = access
			return nil
		}
	}
	m.access[access.SiteID] = append(m.access[access.SiteID], access)
	return nil
}
func (m *Memory) ListSiteAccess(_ context.Context, siteID uuid.UUID) ([]domain.SiteAccess, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.sites[siteID]; !ok {
		return nil, ErrNotFound
	}
	return append([]domain.SiteAccess(nil), m.access[siteID]...), nil
}

func (m *Memory) DeleteSiteAccess(_ context.Context, siteID, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sites[siteID]; !ok {
		return ErrNotFound
	}
	access := m.access[siteID]
	for index, grant := range access {
		if grant.UserID == userID {
			m.access[siteID] = append(access[:index], access[index+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func (m *Memory) CreateSite(_ context.Context, site domain.Site) (domain.Site, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sites[site.ID] = site
	return site, nil
}

func (m *Memory) ListSites(_ context.Context) ([]domain.Site, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.Site, 0, len(m.sites))
	for _, site := range m.sites {
		result = append(result, site)
	}
	return result, nil
}

func (m *Memory) GetSite(_ context.Context, id uuid.UUID) (domain.Site, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	site, ok := m.sites[id]
	if !ok {
		return domain.Site{}, ErrNotFound
	}
	return site, nil
}

func (m *Memory) UpdateSite(_ context.Context, site domain.Site) (domain.Site, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sites[site.ID]; !ok {
		return domain.Site{}, ErrNotFound
	}
	m.sites[site.ID] = site
	return site, nil
}

func (m *Memory) DeleteSite(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sites[id]; !ok {
		return ErrNotFound
	}
	delete(m.sites, id)
	delete(m.images, id)
	for analysisID, run := range m.analyses {
		if run.SiteID == id {
			delete(m.analyses, analysisID)
		}
	}
	return nil
}

func (m *Memory) AddSiteImages(_ context.Context, siteID uuid.UUID, images []domain.SiteImage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sites[siteID]; !ok {
		return ErrNotFound
	}
	m.images[siteID] = append(m.images[siteID], images...)
	return nil
}

func (m *Memory) GetSiteImages(_ context.Context, siteID uuid.UUID) ([]domain.SiteImage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.sites[siteID]; !ok {
		return nil, ErrNotFound
	}
	return append([]domain.SiteImage(nil), m.images[siteID]...), nil
}

func (m *Memory) CreateAnalysis(_ context.Context, run domain.AnalysisRun) (domain.AnalysisRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.analyses[run.ID] = run
	return run, nil
}

func (m *Memory) CompleteAnalysis(_ context.Context, run domain.AnalysisRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.analyses[run.ID] = run
	return nil
}

func (m *Memory) UpdateAnalysisScoring(_ context.Context, run domain.AnalysisRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.analyses[run.ID]; !ok {
		return ErrNotFound
	}
	m.analyses[run.ID] = run
	return nil
}

func (m *Memory) UpdateAnalysisStationRecommendation(_ context.Context, id uuid.UUID, recommendation []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.analyses[id]
	if !ok {
		return ErrNotFound
	}
	run.StationRecommendation = append([]byte(nil), recommendation...)
	m.analyses[id] = run
	return nil
}

func (m *Memory) UpdateAnalysisAIAssessments(_ context.Context, id uuid.UUID, assessments []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.analyses[id]
	if !ok {
		return ErrNotFound
	}
	stored := map[string]json.RawMessage{}
	updates := map[string]json.RawMessage{}
	_ = json.Unmarshal(run.AIAssessments, &stored)
	if err := json.Unmarshal(assessments, &updates); err != nil {
		return err
	}
	for language, assessment := range updates {
		stored[language] = assessment
	}
	persisted, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	run.AIAssessments = persisted
	m.analyses[id] = run
	return nil
}

func (m *Memory) GetAnalysis(_ context.Context, id uuid.UUID) (domain.AnalysisRun, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	run, ok := m.analyses[id]
	if !ok {
		return domain.AnalysisRun{}, ErrNotFound
	}
	return run, nil
}

func (m *Memory) GetLatestCompletedAnalysisForSite(_ context.Context, siteID uuid.UUID) (domain.AnalysisRun, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var latest domain.AnalysisRun
	found := false
	for _, run := range m.analyses {
		if run.SiteID != siteID || run.Status != "completed" {
			continue
		}
		if !found || run.CreatedAt.After(latest.CreatedAt) {
			latest = run
			found = true
		}
	}
	if !found {
		return domain.AnalysisRun{}, ErrNotFound
	}
	return latest, nil
}
