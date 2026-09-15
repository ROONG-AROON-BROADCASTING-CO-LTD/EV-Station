import { lazy, Suspense, useEffect, useState } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { AppShell } from './components/AppShell'
import { LoadingState } from './components/PageState'
import { useI18n } from './i18n/I18nProvider'
import { LoginPage } from './pages/LoginPage'
import { LiffSiteSubmissionPage } from './pages/LiffSiteSubmissionPage'
import { LiffMySitesPage } from './pages/LiffMySitesPage'
import { AnalysisPage } from './pages/AnalysisPage'
import type { AuthSession } from './types/domain'

const DashboardPage = lazy(() => import('./pages/DashboardPage').then(module => ({ default: module.DashboardPage })))
const NewSitePage = lazy(() => import('./pages/NewSitePage').then(module => ({ default: module.NewSitePage })))
const SitePage = lazy(() => import('./pages/SitePage').then(module => ({ default: module.SitePage })))
const DataSourcesPage = lazy(() => import('./pages/DataSourcesPage').then(module => ({ default: module.DataSourcesPage })))
const SettingsPage = lazy(() => import('./pages/SettingsPage').then(module => ({ default: module.SettingsPage })))
const APIUsagePage = lazy(() => import('./pages/APIUsagePage').then(module => ({ default: module.APIUsagePage })))
const NotInvestingPage = lazy(() => import('./pages/NotInvestingPage').then(module => ({ default: module.NotInvestingPage })))

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
  if (window.location.pathname === '/liff/site-submission') {
    const query = new URLSearchParams(window.location.search)
    const liffState = query.get('liff.state')
    // LIFF preserves query parameters in liff.state while it redirects to the
    // configured endpoint, so accept both the direct and redirected forms.
    const stateQuery = liffState ? new URL(liffState, window.location.origin).searchParams : undefined
    return (query.get('view') === 'my-sites' || stateQuery?.get('view') === 'my-sites') ? <LiffMySitesPage /> : <LiffSiteSubmissionPage />
  }
  if (!session) return <LoginPage onAuthenticated={authenticate} />
  return <AppShell onLogout={logout} userName={session.user.displayName} role={session.user.role}><Suspense fallback={<LoadingState label={t('Loading page…')}/> }><Routes>
    <Route path="/" element={<DashboardPage role={session.user.role} />} />
    <Route path="/sites/new" element={<NewSitePage role={session.user.role} />} />
		<Route path="/sites/:id/edit" element={<NewSitePage role={session.user.role} />} />
    <Route path="/sites/:id" element={<SitePage />} />
    <Route path="/analysis/:id" element={<AnalysisPage />} />
    <Route path="/analyses" element={session.user.role === 'customer' ? <Navigate to="/" replace /> : <Placeholder title={t('Analyses')} />} />
    <Route path="/data-sources" element={session.user.role === 'customer' ? <Navigate to="/" replace /> : <DataSourcesPage />} />
		<Route path="/api-usage" element={session.user.role === 'super_admin' || session.user.role === 'admin' ? <APIUsagePage /> : <Navigate to="/" replace />} />
    <Route path="/not-investing" element={session.user.role === 'super_admin' || session.user.role === 'admin' || session.user.role === 'sales' ? <NotInvestingPage /> : <Navigate to="/" replace />} />
    <Route path="/settings" element={<SettingsPage role={session.user.role} />} />
    <Route path="*" element={<Navigate to="/" replace />} />
  </Routes></Suspense></AppShell>
}
