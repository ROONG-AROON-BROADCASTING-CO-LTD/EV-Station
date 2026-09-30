import { lazy, Suspense, useEffect, useState } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { AppShell } from './components/AppShell'
import { LoadingState } from './components/PageState'
import { useI18n } from './i18n/I18nProvider'
import { LoginPage } from './pages/LoginPage'
import { LandingPage } from './pages/LandingPage'
import { InvestmentPage } from './pages/InvestmentPage'
import { BranchesPage } from './pages/BranchesPage'
import { ContactPage } from './pages/ContactPage'
import { LiffSiteSubmissionPage } from './pages/LiffSiteSubmissionPage'
import { LiffMySitesPage } from './pages/LiffMySitesPage'
import { AnalysisPage } from './pages/AnalysisPage'
import type { AuthSession } from './types/domain'
import { clearSession, readSession, saveSession } from './services/session'

const DashboardPage = lazy(() => import('./pages/DashboardPage').then(module => ({ default: module.DashboardPage })))
const NewSitePage = lazy(() => import('./pages/NewSitePage').then(module => ({ default: module.NewSitePage })))
const SitePage = lazy(() => import('./pages/SitePage').then(module => ({ default: module.SitePage })))
const DataSourcesPage = lazy(() => import('./pages/DataSourcesPage').then(module => ({ default: module.DataSourcesPage })))
const SettingsPage = lazy(() => import('./pages/SettingsPage').then(module => ({ default: module.SettingsPage })))
const APIUsagePage = lazy(() => import('./pages/APIUsagePage').then(module => ({ default: module.APIUsagePage })))
const NotInvestingPage = lazy(() => import('./pages/NotInvestingPage').then(module => ({ default: module.NotInvestingPage })))
const GuidePage = lazy(() => import('./pages/GuidePage').then(module => ({ default: module.GuidePage })))

function Placeholder({ title }: { title: string }) { const { t } = useI18n(); return <div><h1 className="page-title">{title}</h1><p className="mt-3 text-muted">{t('This module is prepared for a later MVP iteration.')}</p></div> }

export default function App() {
  const { t } = useI18n()
  const { pathname, search } = useLocation()
  const [session, setSession] = useState<AuthSession | null>(readSession)
  const authenticate = (next: AuthSession) => { saveSession(next); setSession(next) }
  const logout = () => { clearSession(); setSession(null) }
  useEffect(() => {
    const handleExpiredSession = () => setSession(null)
    window.addEventListener('rbc:auth-expired', handleExpiredSession)
    return () => window.removeEventListener('rbc:auth-expired', handleExpiredSession)
  }, [])
  if (pathname === '/liff/site-submission') {
    const query = new URLSearchParams(search)
    const liffState = query.get('liff.state')
    // LIFF preserves query parameters in liff.state while it redirects to the
    // configured endpoint, so accept both the direct and redirected forms.
    const stateQuery = liffState ? new URL(liffState, window.location.origin).searchParams : undefined
    return (query.get('view') === 'my-sites' || stateQuery?.get('view') === 'my-sites') ? <LiffMySitesPage /> : <LiffSiteSubmissionPage />
  }
  if (pathname === '/guide/customer') return <Suspense fallback={<LoadingState label={t('Loading page…')} />}><GuidePage audience="customer" accountName={session?.user.displayName} onLogout={session ? logout : undefined} /></Suspense>
  if (pathname === '/guide/admin' && !session) return <Navigate to="/login" replace />
  if (pathname === '/welcome') return <Navigate to="/" replace />
  if (!session) return <Routes><Route path="/" element={<LandingPage />} /><Route path="/investment" element={<InvestmentPage />} /><Route path="/branches" element={<BranchesPage />} /><Route path="/contact" element={<ContactPage />} /><Route path="/login" element={<><LandingPage /><LoginPage onAuthenticated={authenticate} /></>} /><Route path="*" element={<Navigate to="/" replace />} /></Routes>
  if (pathname === '/') return <LandingPage authenticated accountName={session.user.displayName} onLogout={logout} />
  if (pathname === '/investment') return <InvestmentPage authenticated accountName={session.user.displayName} onLogout={logout} />
  if (pathname === '/branches') return <BranchesPage authenticated accountName={session.user.displayName} onLogout={logout} />
  if (pathname === '/contact') return <ContactPage authenticated accountName={session.user.displayName} onLogout={logout} />
  return <AppShell onLogout={logout} userName={session.user.displayName} role={session.user.role}><Suspense fallback={<LoadingState label={t('Loading page…')}/> }><Routes>
    <Route path="/dashboard" element={<DashboardPage role={session.user.role} />} />
    <Route path="/guide/admin" element={session.user.role === 'customer' ? <Navigate to="/dashboard" replace /> : <GuidePage audience="admin" />} />
    <Route path="/sites/new" element={<NewSitePage role={session.user.role} />} />
		<Route path="/sites/:id/edit" element={<NewSitePage role={session.user.role} />} />
    <Route path="/sites/:id" element={<SitePage role={session.user.role} />} />
    <Route path="/analysis/:id" element={<AnalysisPage role={session.user.role} />} />
    <Route path="/analyses" element={session.user.role === 'customer' ? <Navigate to="/" replace /> : <Placeholder title={t('Analyses')} />} />
    <Route path="/data-sources" element={session.user.role === 'customer' ? <Navigate to="/" replace /> : <DataSourcesPage />} />
		<Route path="/api-usage" element={session.user.role === 'super_admin' || session.user.role === 'admin' ? <APIUsagePage /> : <Navigate to="/" replace />} />
    <Route path="/not-investing" element={session.user.role === 'super_admin' || session.user.role === 'admin' || session.user.role === 'sales' ? <NotInvestingPage /> : <Navigate to="/" replace />} />
    <Route path="/settings" element={<SettingsPage role={session.user.role} />} />
    <Route path="*" element={<Navigate to="/dashboard" replace />} />
  </Routes></Suspense></AppShell>
}
