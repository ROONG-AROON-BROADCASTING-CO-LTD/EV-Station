package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func (m *Memory) GetLineCustomerProfile(_ context.Context, user string) (domain.LineCustomerProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	profile, ok := m.lineProfiles[user]
	if !ok {
		return domain.LineCustomerProfile{}, ErrNotFound
	}
	return profile, nil
}

func (m *Memory) UpsertLineCustomerProfile(_ context.Context, user string, profile domain.LineCustomerProfile) (domain.LineCustomerProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	if existing, ok := m.lineProfiles[user]; ok {
		profile.CreatedAt = existing.CreatedAt
	} else {
		profile.CreatedAt = now
	}
	profile.UpdatedAt = now
	m.lineProfiles[user] = profile
	return profile, nil
}

func (m *Memory) SaveLineSubmission(_ context.Context, user string, requestID uuid.UUID, site domain.Site) (domain.Site, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for siteID, submission := range m.lineSubmissions {
		if submission.user == user && submission.requestID == requestID {
			return m.sites[siteID], nil
		}
	}
	m.sites[site.ID] = site
	m.lineSubmissions[site.ID] = memoryLineSubmission{user: user, requestID: requestID}
	return site, nil
}

func (m *Memory) OwnsLineSite(_ context.Context, user string, siteID uuid.UUID) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	submission, ok := m.lineSubmissions[siteID]
	return ok && submission.user == user, nil
}

func (m *Memory) ListLineSites(_ context.Context, user string) ([]domain.Site, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := []domain.Site{}
	for siteID, submission := range m.lineSubmissions {
		if submission.user == user {
			result = append(result, m.sites[siteID])
		}
	}
	return result, nil
}
