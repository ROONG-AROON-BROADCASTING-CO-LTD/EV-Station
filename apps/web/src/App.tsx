import { lazy, Suspense, useEffect, useState } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { AppShell } from './components/AppShell'
import { LoadingState } from './components/PageState'
import { useI18n } from './i18n/I18nProvider'
import { LoginPage } from './pages/LoginPage'
import type { AuthSession } from './types/domain'

const AnalysisPage = lazy(() => import('./pages/AnalysisPage').then(module => ({ default: module.AnalysisPage })))
const DashboardPage = lazy(() => import('./pages/DashboardPage').then(module => ({ default: module.DashboardPage })))
const NewSitePage = lazy(() => import('./pages/NewSitePage').then(module => ({ default: module.NewSitePage })))
const SitePage = lazy(() => import('./pages/SitePage').then(module => ({ default: module.SitePage })))
const DataSourcesPage = lazy(() => import('./pages/DataSourcesPage').then(module => ({ default: module.DataSourcesPage })))
const SettingsPage = lazy(() => import('./pages/SettingsPage').then(module => ({ default: module.SettingsPage })))

function Placeholder({ title }: { title: string }) { const { t } = useI18n(); return <div><h1 className="page-title">{title}</h1><p className="mt-3 text-muted">{t('This module is prepared for a later MVP iteration.')}</p></div> }

export default function App() {
  const { t } = useI18n()
  const [session, setSession] = useState<AuthSession | null>(() => { try { const saved = localStorage.getItem('rbc-session'); return saved ? JSON.parse(saved) : null } catch { return null } })
  const authenticate = (next: AuthSession) => { localStorage.setItem('rbc-session', JSON.stringify(next)); localStorage.setItem('rbc-session-token', next.token); setSession(next) }
  const logout = () => { localStorage.removeItem('rbc-session'); localStorage.removeItem('rbc-session-token'); setSession(null) }
  useEffect(() => {
    const handleExpiredSession = () => setSession(null)
    window.addEventListener('rbc:auth-expired', handleExpiredSession)
    return () => window.removeEventListener('rbc:auth-expired', handleExpiredSession)
  }, [])
  if (!session) return <LoginPage onAuthenticated={authenticate} />
  return <AppShell onLogout={logout} userName={session.user.displayName} role={session.user.role}><Suspense fallback={<LoadingState label={t('Loading page…')}/> }><Routes>
    <Route path="/" element={<DashboardPage />} />
    <Route path="/sites/new" element={<NewSitePage />} />
		<Route path="/sites/:id/edit" element={<NewSitePage />} />
    <Route path="/sites/:id" element={<SitePage />} />
    <Route path="/analysis/:id" element={<AnalysisPage />} />
    <Route path="/analyses" element={<Placeholder title={t('Analyses')} />} />
    <Route path="/data-sources" element={<DataSourcesPage />} />
    <Route path="/settings" element={<SettingsPage role={session.user.role} />} />
    <Route path="*" element={<Navigate to="/" replace />} />
  </Routes></Suspense></AppShell>
}
