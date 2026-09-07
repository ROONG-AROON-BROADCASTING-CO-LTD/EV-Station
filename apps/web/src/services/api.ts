import type { AIAssessment, AnalysisRun, AuthSession, CreateSiteInput, DataSourceCatalogEntry, FranchisePlan, GeocodingResult, GoogleMapsResolution, Site, SiteAccess, User, UserRole } from '../types/domain'

const baseURL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api/v1'

type Envelope<T> = { data: T }
type ErrorEnvelope = { error?: { code?: string; message?: string } }

export class APIError extends Error {
  constructor(public code: string, message: string, public status: number) { super(message) }
}

const authExpiredEvent = 'rbc:auth-expired'

function clearExpiredSession() {
  localStorage.removeItem('rbc-session')
  localStorage.removeItem('rbc-session-token')
  window.dispatchEvent(new Event(authExpiredEvent))
}

function throwAPIError<T>(response: Response, body: Envelope<T> & ErrorEnvelope): never {
  const code = body.error?.code || 'REQUEST_FAILED'
  if (response.status === 401 && (code === 'INVALID_TOKEN' || code === 'AUTH_REQUIRED')) clearExpiredSession()
  throw new APIError(code, body.error?.message || 'Request failed.', response.status)
}

export function errorMessageKey(error: unknown) {
  return error instanceof APIError ? `error.${error.code}` : 'error.REQUEST_FAILED'
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = localStorage.getItem('rbc-session-token')
  const response = await fetch(`${baseURL}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}), ...init?.headers },
  })
  const body = await response.json().catch(() => ({})) as Envelope<T> & ErrorEnvelope
  if (!response.ok) throwAPIError(response, body)
  return body.data
}

async function upload<T>(path: string, form: FormData): Promise<T> {
  const token = localStorage.getItem('rbc-session-token')
  const response = await fetch(`${baseURL}${path}`, { method: 'POST', body: form, headers: token ? { Authorization: `Bearer ${token}` } : undefined })
  const body = await response.json().catch(() => ({})) as Envelope<T> & ErrorEnvelope
  if (!response.ok) throwAPIError(response, body)
  return body.data
}

export const api = {
  login: (email: string, password: string) => request<AuthSession>('/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }),
  register: (email: string, displayName: string, password: string, role: UserRole = 'owner') => request<User>('/auth/register', { method: 'POST', body: JSON.stringify({ email, displayName, password, role }) }),
	createUser: (email: string, displayName: string, password: string, role: UserRole) => request<User>('/users', { method: 'POST', body: JSON.stringify({ email, displayName, password, role }) }),
	listUsers: () => request<User[]>('/users'),
	 recommendStation: (id: string, refresh = false) => request<import('../components/StationRecommendation').StationProposal>(`/analyses/${id}/station-recommendation${refresh ? '?refresh=true' : ''}`, { method: 'POST' }),
  listSites: () => request<Site[]>('/sites'),
  getSite: (id: string) => request<Site>(`/sites/${id}`),
  getLatestAnalysisForSite: (id: string) => request<AnalysisRun | null>(`/sites/${id}/latest-analysis`),
  createSite: (input: CreateSiteInput) => request<Site>('/sites', { method: 'POST', body: JSON.stringify(input) }),
	updateSite: (id: string, input: CreateSiteInput) => request<Site>(`/sites/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
	uploadSiteImages: (id: string, files: File[]) => { const form = new FormData(); files.forEach(file => form.append('images', file)); return upload<{ count: number }>(`/sites/${id}/images`, form) },
	deleteSite: (id: string) => request<void>(`/sites/${id}`, { method: 'DELETE' }),
	listSiteAccess: (id: string) => request<SiteAccess[]>(`/sites/${id}/access`),
	setSiteAccess: (id: string, userId: string, role: UserRole) => request<void>(`/sites/${id}/access`, { method: 'PUT', body: JSON.stringify({ userId, role }) }),
	deleteSiteAccess: (id: string, userId: string) => request<void>(`/sites/${id}/access/${userId}`, { method: 'DELETE' }),
  runAnalysis: (siteId: string, radiusMeters = 3000) => request<AnalysisRun>(`/sites/${siteId}/analyses`, { method: 'POST', body: JSON.stringify({ radiusMeters }) }),
  getAnalysis: (id: string) => request<AnalysisRun>(`/analyses/${id}`),
  recalculatePreliminary: (id: string) => request<AnalysisRun>(`/analyses/${id}/recalculate-preliminary`, { method: 'POST' }),
  generateAIAssessment: (id: string, language: 'th' | 'en', refresh = false) => request<AIAssessment>(`/analyses/${id}/ai-assessment${refresh ? '?refresh=true' : ''}`, { method: 'POST', body: JSON.stringify({ language }) }),
  generateBilingualAssessment: async (id: string, refresh = false) => {
    const languages = ['th', 'en'] as const
    const results = await Promise.allSettled(languages.map(language => api.generateAIAssessment(id, language, refresh)))
    const assessments: Partial<Record<'th' | 'en', AIAssessment>> = {}
    const errors: Partial<Record<'th' | 'en', unknown>> = {}
    results.forEach((result, index) => {
      const language = languages[index]
      if (result.status === 'fulfilled') assessments[language] = result.value
      else errors[language] = result.reason
    })
    return { assessments, errors }
  },
  searchAddress: (query: string) => request<GeocodingResult[]>(`/geocoding/search?q=${encodeURIComponent(query)}&limit=5`),
  resolveGoogleMapsUrl: (url: string) => request<GoogleMapsResolution>('/maps/resolve', { method: 'POST', body: JSON.stringify({ url }) }),
  getDataSources: () => request<DataSourceCatalogEntry[]>('/data-sources'),
  getFranchisePlans: () => request<FranchisePlan[]>('/financial/plans'),
}
