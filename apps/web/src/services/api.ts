import type { AIAssessment, AnalysisRun, APIUsageSummary, AuthSession, CreateSiteInput, DataSourceCatalogEntry, FranchisePlan, GeocodingResult, GoogleMapsResolution, LineCustomerProfile, Site, SiteAccess, SiteAttachment, StationRecommendation, User, UserRole } from '../types/domain'
import { clearSession, getSessionToken } from './session'

const baseURL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api/v1'

type Envelope<T> = { data: T }
type ErrorEnvelope = { error?: { code?: string; message?: string } }

export class APIError extends Error {
  constructor(public code: string, message: string, public status: number) { super(message) }
}

const authExpiredEvent = 'rbc:auth-expired'

function clearExpiredSession() {
  clearSession()
  window.dispatchEvent(new Event(authExpiredEvent))
}

function throwAPIError<T>(response: Response, body: Envelope<T> & ErrorEnvelope, requestToken: string | null): never {
  const code = body.error?.code || 'REQUEST_FAILED'
  if (response.status === 401 && requestToken && requestToken === getSessionToken() && (code === 'INVALID_TOKEN' || code === 'AUTH_REQUIRED')) clearExpiredSession()
  if ([408, 504, 524].includes(response.status)) throw new APIError('REQUEST_TIMEOUT', 'The request timed out.', response.status)
  throw new APIError(code, body.error?.message || 'Request failed.', response.status)
}

export function errorMessageKey(error: unknown) {
  if (error instanceof APIError && error.code === 'AUTH_UNAVAILABLE') return 'Sign-in verification is temporarily unavailable. Please try again.'
  if (error instanceof APIError && error.code === 'REQUEST_TIMEOUT') return 'The request took too long. Check the latest status before trying again; submitted work may still be processing.'
  return error instanceof APIError ? `error.${error.code}` : 'error.REQUEST_FAILED'
}

async function readEnvelope<T>(response: Response, token: string | null): Promise<T> {
  const body = await response.json().catch(error => {
    if (response.ok) throw error
    return {}
  }) as Envelope<T> & ErrorEnvelope
  if (!response.ok) throwAPIError(response, body, token)
  return body.data
}

type APIRequestOptions = { json?: boolean }

function headerRecord(headers?: HeadersInit) {
  if (!headers) return {}
  if (headers instanceof Headers) return Object.fromEntries(headers.entries())
  return Object.fromEntries(Array.isArray(headers) ? headers : Object.entries(headers))
}

async function authenticatedRequest<T>(path: string, token: string | null, init?: RequestInit, options: APIRequestOptions = {}): Promise<T> {
  const { json = true } = options
  return withRequestDeadline(async signal => {
    const response = await fetch(`${baseURL}${path}`, {
      ...init,
      signal,
      headers: { ...(json ? { 'Content-Type': 'application/json' } : {}), ...(token ? { Authorization: `Bearer ${token}` } : {}), ...headerRecord(init?.headers) },
    })
    return readEnvelope<T>(response, token)
  }, json ? 60_000 : 120_000, init?.signal)
}

// Bound both the response headers and body. Never retry mutations automatically:
// the server may already have accepted a submission when the connection fails.
async function withRequestDeadline<T>(operation: (signal: AbortSignal) => Promise<T>, milliseconds: number, parent?: AbortSignal | null): Promise<T> {
  const controller = new AbortController()
  let timedOut = false
  const abort = () => controller.abort()
  if (parent?.aborted) abort()
  else parent?.addEventListener('abort', abort, { once: true })
  const timer = setTimeout(() => { timedOut = true; controller.abort() }, milliseconds)
  try {
    return await operation(controller.signal)
  } catch (error) {
    if (timedOut) throw new APIError('REQUEST_TIMEOUT', 'The request timed out.', 0)
    throw error
  } finally {
    clearTimeout(timer)
    parent?.removeEventListener('abort', abort)
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  return authenticatedRequest<T>(path, getSessionToken(), init)
}

async function upload<T>(path: string, form: FormData): Promise<T> {
  return authenticatedRequest<T>(path, getSessionToken(), { method: 'POST', body: form }, { json: false })
}

async function download(path: string): Promise<Blob> {
  const token = getSessionToken()
  return withRequestDeadline(async signal => {
    const response = await fetch(`${baseURL}${path}`, { signal, headers: token ? { Authorization: `Bearer ${token}` } : undefined })
    if ([408, 504, 524].includes(response.status)) throw new APIError('REQUEST_TIMEOUT', 'The request timed out.', response.status)
    if (!response.ok) await readEnvelope(response, token)
    return response.blob()
  }, 120_000)
}

async function liffRequest<T>(path: string, idToken: string, init?: RequestInit): Promise<T> {
  return authenticatedRequest<T>(path, idToken, init)
}

async function liffUpload<T>(path: string, idToken: string, form: FormData): Promise<T> {
  return authenticatedRequest<T>(path, idToken, { method: 'POST', body: form }, { json: false })
}

function createImageUploadForm(photos: File[], documents: File[]) { const form = new FormData(); photos.forEach(file => form.append('photos', file)); documents.forEach(file => form.append('images', file)); return form }

export const api = {
  listDocumentTemplates: () => request<import('../types/domain').DocumentTemplate[]>('/document-templates'),
  uploadDocumentTemplate: (file: File, title: string, requestId: string) => {
    const form=new FormData();form.append('file',file);form.append('title',title)
    return authenticatedRequest<import('../types/domain').DocumentTemplate>('/document-templates',getSessionToken(),{method:'POST',body:form,headers:{'X-Request-ID':requestId}},{json:false})
  },
  documentTemplateFile: (id:string) => download(`/document-templates/${id}/file?download=1`),
  listDocuments: (siteID?: string) => request<import('../types/domain').ProjectDocument[]>(siteID ? `/sites/${siteID}/documents` : '/documents'),
  uploadDocument: (siteID: string, file: File, title: string, category: string, documentID?: string, requestId?: string) => {
    const form = new FormData(); form.append('file', file); form.append('title', title); form.append('category', category)
    return authenticatedRequest<import('../types/domain').ProjectDocument>(`/sites/${siteID}/documents${documentID ? `/${documentID}/versions` : ''}`, getSessionToken(), { method:'POST', body:form, headers: requestId ? {'X-Request-ID':requestId} : undefined }, {json:false})
  },
  updateDocument: (siteID: string, documentID: string, metadata: import('../types/domain').DocumentMetadata, expectedVersion: number) => request<import('../types/domain').ProjectDocument>(`/sites/${siteID}/documents/${documentID}`, { method: 'PUT', body: JSON.stringify({...metadata,expectedVersion}) }),
  documentVersions: (siteID: string, documentID: string) => request<import('../types/domain').DocumentVersion[]>(`/sites/${siteID}/documents/${documentID}/versions`),
  documentEvents: (siteID: string, documentID: string) => request<import('../types/domain').DocumentEvent[]>(`/sites/${siteID}/documents/${documentID}/events`),
  documentFile: (siteID: string, documentID: string, version: number, asDownload = false) => download(`/sites/${siteID}/documents/${documentID}/versions/${version}/file${asDownload ? '?download=1' : ''}`),
  registerVisitorOrdinal: () => request<{ ordinal: number }>('/public/visitor-ordinal', { method: 'POST' }),
  login: (email: string, password: string) => request<AuthSession>('/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }),
  requestRegistrationOTP: (email: string) => request<{ sent: boolean }>('/auth/register/request-otp', { method: 'POST', body: JSON.stringify({ email }) }),
  register: (email: string, displayName: string, password: string, otp: string) => request<User>('/auth/register', { method: 'POST', body: JSON.stringify({ email, displayName, password, otp }) }),
	createUser: (email: string, displayName: string, password: string, role: UserRole) => request<User>('/users', { method: 'POST', body: JSON.stringify({ email, displayName, password, role }) }),
	listUsers: () => request<User[]>('/users'),
	updateUser: (id: string, input: { email: string; displayName: string; password?: string; role: UserRole; isActive: boolean }) => request<User>(`/users/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
	deleteUser: (id: string) => request<void>(`/users/${id}`, { method: 'DELETE' }),
	 recommendStation: (id: string, refresh = false) => request<StationRecommendation>(`/analyses/${id}/station-recommendation${refresh ? '?refresh=true' : ''}`, { method: 'POST' }),
  listSites: () => request<Site[]>('/sites'),
  getSite: (id: string) => request<Site>(`/sites/${id}`),
	getLatestAnalysisForSite: (id: string) => request<AnalysisRun | null>(`/sites/${id}/latest-analysis`),
	listSiteImages: (id: string) => request<SiteAttachment[]>(`/sites/${id}/images`),
	downloadSiteImage: (siteID: string, imageID: string) => download(`/sites/${siteID}/images/${imageID}`),
  createSite: (input: CreateSiteInput, requestId?: string) => request<Site>('/sites', { method: 'POST', body: JSON.stringify(input), headers: requestId ? { 'X-Request-ID': requestId } : undefined }),
	createLiffSite: (input: CreateSiteInput, idToken: string, requestId: string) => liffRequest<{ site: Site }>('/liff/sites', idToken, { method: 'POST', body: JSON.stringify({ ...input, requestId }) }),
	getLiffCustomerProfile: (idToken: string) => liffRequest<{ profile: LineCustomerProfile | null }>('/liff/customer-profile', idToken).then(result => result.profile),
	completeLiffSite: (id: string, idToken: string) => liffRequest<{ notificationAccepted: boolean }>(`/liff/sites/${id}/complete`, idToken, { method: 'POST' }),
	listLiffSites: (idToken: string) => liffRequest<Array<Site & { customerStatus: 'pending_review' | 'analysis_completed' }>>('/liff/sites', idToken),
	resolveLiffGoogleMapsUrl: (url: string, idToken: string) => liffRequest<GoogleMapsResolution>('/liff/maps/resolve', idToken, { method: 'POST', body: JSON.stringify({ url }) }),
	uploadLiffSiteImages: (id: string, photos: File[], documents: File[], idToken: string) => liffUpload<{ count: number }>(`/liff/sites/${id}/images`, idToken, createImageUploadForm(photos, documents)),
	updateSite: (id: string, input: CreateSiteInput) => request<Site>(`/sites/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
	uploadSiteImages: (id: string, photos: File[], documents: File[]) => upload<{ count: number }>(`/sites/${id}/images`, createImageUploadForm(photos, documents)),
	deleteSite: (id: string) => request<void>(`/sites/${id}`, { method: 'DELETE' }),
	listSiteAccess: (id: string) => request<SiteAccess[]>(`/sites/${id}/access`),
	setSiteAccess: (id: string, userId: string, role: UserRole) => request<void>(`/sites/${id}/access`, { method: 'PUT', body: JSON.stringify({ userId, role }) }),
	deleteSiteAccess: (id: string, userId: string) => request<void>(`/sites/${id}/access/${userId}`, { method: 'DELETE' }),
  runAnalysis: (siteId: string, radiusMeters = 3000) => request<AnalysisRun>(`/sites/${siteId}/analyses`, { method: 'POST', body: JSON.stringify({ radiusMeters }) }),
  getAnalysis: (id: string) => request<AnalysisRun>(`/analyses/${id}`),
  recalculatePreliminary: (id: string) => request<AnalysisRun>(`/analyses/${id}/recalculate-preliminary`, { method: 'POST' }),
  generateAIAssessment: (id: string, language: 'th' | 'en', refresh = false) => request<AIAssessment>(`/analyses/${id}/ai-assessment${refresh ? '?refresh=true' : ''}`, { method: 'POST', body: JSON.stringify({ language }) }),
  generateAIAssessments: async (id: string, languages: readonly ('th' | 'en')[], refresh = false) => {
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
	getAPIUsage: () => request<APIUsageSummary[]>('/api-usage'),
	recordGoogleMapsLoad: () => request<void>('/api-usage/google-maps-load', { method: 'POST' }),
  getFranchisePlans: () => request<FranchisePlan[]>('/financial/plans'),
}
