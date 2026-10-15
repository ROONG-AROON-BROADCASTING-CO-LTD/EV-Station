import type { CreateSiteInput } from '../types/domain'

export interface SiteDraft {
  values: Partial<CreateSiteInput>
  requestId: string
  savedSiteId?: string
  hadFiles: boolean
  updatedAt: number
}

// Tab-scoped, account-scoped and short-lived. Never store files or credentials.
export function readSiteDraft(key: string): SiteDraft | undefined {
  try {
    const draft = JSON.parse(sessionStorage.getItem(key) || 'null') as SiteDraft | null
    if (draft && Number.isFinite(draft.updatedAt) && draft.updatedAt <= Date.now() && Date.now() - draft.updatedAt < 24 * 60 * 60 * 1000 && draft.values && /^[0-9a-f-]{36}$/i.test(draft.requestId)) return draft
    sessionStorage.removeItem(key)
  } catch { /* Storage may be disabled by the browser. */ }
}

export function writeSiteDraft(key: string, draft?: SiteDraft) {
  try {
    if (draft) sessionStorage.setItem(key, JSON.stringify(draft))
    else sessionStorage.removeItem(key)
  } catch { /* A storage quota must not stop submission. */ }
}
