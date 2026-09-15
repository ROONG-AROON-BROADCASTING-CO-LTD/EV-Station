package repository

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

type Memory struct {
	mu                        sync.RWMutex
	sites                     map[uuid.UUID]domain.Site
	images                    map[uuid.UUID][]domain.SiteImage
	analyses                  map[uuid.UUID]domain.AnalysisRun
	users                     map[uuid.UUID]domain.User
	passwords                 map[uuid.UUID]string
	access                    map[uuid.UUID][]domain.SiteAccess
	lineProfiles              map[string]domain.LineCustomerProfile
	lineSubmissions           map[uuid.UUID]memoryLineSubmission
	lineNotificationRecipient string
	apiUsagePlans             map[string]domain.APIUsagePlan
	apiUsage                  map[string]int64
}

func NewMemory() *Memory {
	plans := defaultAPIUsagePlans()
	return &Memory{sites: make(map[uuid.UUID]domain.Site), images: make(map[uuid.UUID][]domain.SiteImage), analyses: make(map[uuid.UUID]domain.AnalysisRun), users: make(map[uuid.UUID]domain.User), passwords: make(map[uuid.UUID]string), access: make(map[uuid.UUID][]domain.SiteAccess), lineProfiles: make(map[string]domain.LineCustomerProfile), lineSubmissions: make(map[uuid.UUID]memoryLineSubmission), apiUsagePlans: plans, apiUsage: make(map[string]int64)}
}

type memoryLineSubmission struct {
	user      string
	requestID uuid.UUID
}

func defaultAPIUsagePlans() map[string]domain.APIUsagePlan {
	gistdaIncluded := int64(200)
	googleMapsIncluded := int64(10_000)
	googlePlacesIncluded := int64(5_000)
	googleMapsOverageTHB := 0.252
	googlePlacesOverageTHB := 1.152
	result := map[string]domain.APIUsagePlan{}
	for _, item := range []domain.APIUsagePlan{
		{ProviderID: "google-maps-js", DisplayName: "Google Maps JavaScript API", UnitLabel: "map loads", IncludedUnits: &googleMapsIncluded, OveragePriceTHB: &googleMapsOverageTHB},
		{ProviderID: "google-places", DisplayName: "Google Places API (New)", UnitLabel: "nearby-search requests", IncludedUnits: &googlePlacesIncluded, OveragePriceTHB: &googlePlacesOverageTHB},
		{ProviderID: "gemini-advisory", DisplayName: "Gemini AI", UnitLabel: "generations"},
		{ProviderID: "gistda-elevation", DisplayName: "GISTDA Sphere Elevation API", UnitLabel: "elevation requests", IncludedUnits: &gistdaIncluded},
	} {
		item.PricingNote = apiUsagePricingNote(item.ProviderID)
		result[item.ProviderID] = item
	}
	return result
}

func apiUsagePricingNote(providerID string) string {
	switch providerID {
	case "google-maps-js":
		return "ประมาณการจากราคา Dynamic Maps ของ Google: สิทธิ์ 10,000 ครั้ง/เดือน แล้ว ฿0.252/ครั้ง (US$7 ต่อ 1,000 ครั้ง ที่ 36 บาท/US$1)"
	case "google-places":
		return "ประมาณการจากราคา Nearby Search Pro ของ Google: สิทธิ์ 5,000 ครั้ง/เดือน แล้ว ฿1.152/ครั้ง (US$32 ต่อ 1,000 ครั้ง ที่ 36 บาท/US$1)"
	default:
		return ""
	}
}

func (m *Memory) RecordAPIUsage(_ context.Context, providerID string, units int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if units > 0 {
		m.apiUsage[providerID] += units
	}
	return nil
}
func (m *Memory) ListAPIUsage(_ context.Context, periodStart time.Time) ([]domain.APIUsageSummary, error) {
	if periodStart.IsZero() {
		periodStart = time.Now().UTC()
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	periodStart = time.Date(periodStart.Year(), periodStart.Month(), 1, 0, 0, 0, 0, time.UTC)
	result := make([]domain.APIUsageSummary, 0, len(m.apiUsagePlans))
	for _, plan := range m.apiUsagePlans {
		plan.PricingNote = apiUsagePricingNote(plan.ProviderID)
		item := domain.APIUsageSummary{APIUsagePlan: plan, UsedUnits: m.apiUsage[plan.ProviderID], PeriodStart: periodStart}
		if plan.IncludedUnits != nil {
			remaining := max(int64(0), *plan.IncludedUnits-item.UsedUnits)
			item.RemainingUnits = &remaining
			item.OverageUnits = max(int64(0), item.UsedUnits-*plan.IncludedUnits)
			if plan.OveragePriceTHB != nil {
				cost := float64(item.OverageUnits) * *plan.OveragePriceTHB
				item.EstimatedCostTHB = &cost
			}
		}
		result = append(result, item)
	}
	return result, nil
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
func (m *Memory) GetUserByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, ok := m.users[id]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	return user, nil
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
func (m *Memory) UpdateUser(_ context.Context, user domain.User, passwordHash *string) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[user.ID]; !ok {
		return domain.User{}, ErrNotFound
	}
	for id, existing := range m.users {
		if id != user.ID && existing.Email == user.Email {
			return domain.User{}, errorString("email already exists")
		}
	}
	m.users[user.ID] = user
	if passwordHash != nil {
		m.passwords[user.ID] = *passwordHash
	}
	return user, nil
}
func (m *Memory) DeleteUser(_ context.Context, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[userID]; !ok {
		return ErrNotFound
	}
	delete(m.users, userID)
	delete(m.passwords, userID)
	for siteID, grants := range m.access {
		filtered := grants[:0]
		for _, grant := range grants {
			if grant.UserID != userID {
				filtered = append(filtered, grant)
			}
		}
		m.access[siteID] = filtered
	}
	return nil
}
func (m *Memory) SetLineNotificationRecipient(_ context.Context, recipientID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lineNotificationRecipient = recipientID
	return nil
}
func (m *Memory) GetLineNotificationRecipient(_ context.Context) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.lineNotificationRecipient == "" {
		return "", ErrNotFound
	}
	return m.lineNotificationRecipient, nil
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

func (m *Memory) ListSiteImages(_ context.Context, siteID uuid.UUID) ([]domain.SiteImage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.sites[siteID]; !ok {
		return nil, ErrNotFound
	}
	items := make([]domain.SiteImage, 0, len(m.images[siteID]))
	for _, image := range m.images[siteID] {
		image.SizeBytes = int64(len(image.Data))
		image.Data = nil
		items = append(items, image)
	}
	return items, nil
}

func (m *Memory) GetSiteImage(_ context.Context, siteID, imageID uuid.UUID) (domain.SiteImage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.sites[siteID]; !ok {
		return domain.SiteImage{}, ErrNotFound
	}
	for _, image := range m.images[siteID] {
		if image.ID == imageID {
			image.SizeBytes = int64(len(image.Data))
			return image, nil
		}
	}
	return domain.SiteImage{}, ErrNotFound
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
