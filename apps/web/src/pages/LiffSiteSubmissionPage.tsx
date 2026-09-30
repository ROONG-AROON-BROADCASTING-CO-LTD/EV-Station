import { lazy, Suspense, useEffect, useState } from 'react'
import { useI18n } from '../i18n/I18nProvider'
import { api } from '../services/api'
import { loadLiffConfig, loadLiffSDK } from '../services/liff'
import type { LineCustomerProfile } from '../types/domain'

const NewSitePage = lazy(() => import('./NewSitePage').then(module => ({ default: module.NewSitePage })))

export function LiffSiteSubmissionPage() {
  const { t } = useI18n()
  const [idToken, setIDToken] = useState<string>()
  const [profile, setProfile] = useState<LineCustomerProfile | null>(null)
  const [error, setError] = useState<string>()
  const [completed, setCompleted] = useState<boolean>()
  useEffect(() => {
    let active = true
    const initialise = async () => { try {
      const [sdk, configResponse] = await Promise.all([loadLiffSDK(), loadLiffConfig('Unable to prepare the form.')])
      if (!configResponse.liffId) throw new Error('error.LIFF_NOT_READY')
      await sdk.init({ liffId: configResponse.liffId })
      if (!sdk.isLoggedIn()) { sdk.login({ redirectUri: `${location.origin}${location.pathname}` }); return }
      const token = sdk.getIDToken(); if (!token) throw new Error('Reopen this form from LINE to verify your identity.')
      let savedProfile: LineCustomerProfile | null = null
      try { savedProfile = await api.getLiffCustomerProfile(token) } catch { /* Submission can continue without a saved profile. */ }
      if (active) { setIDToken(token); setProfile(savedProfile); setError('') }
    } catch (nextError) { if (active) setError(nextError instanceof Error ? nextError.message : 'error.LIFF_CONNECTION') } }
    void initialise(); return () => { active = false }
  }, [])
  if (completed !== undefined) return <main className="mx-auto max-w-xl px-5 py-12"><section className="rounded-xl border border-line bg-white p-7 text-center shadow-panel"><h1 className="page-title">{t('Site information submitted')}</h1><p className="mt-3 text-muted">{t(completed ? 'A confirmation was sent to your LINE chat. Our team will review your information and contact you.' : 'Your information was saved, but the confirmation could not be sent. Add or unblock RBC EV Station, then try again.')}</p><a className="button-primary mt-6 inline-flex" href="https://line.me/R/ti/p/@424cdtee">{t('Return to RBC EV Station chat')}</a></section></main>
  if (!idToken) return <main className="mx-auto max-w-xl px-5 py-12"><section className="rounded-xl border border-line bg-white p-7 text-center shadow-panel"><h1 className="page-title">{t('Submit site information')}</h1><p className={error ? 'mt-3 text-red-600' : 'mt-3 text-muted'}>{error ? t(error) : t('Connecting to your LINE account…')}</p>{error && <button className="button-primary mt-6" onClick={() => location.reload()}>{t('Try again')}</button>}</section></main>
  return <Suspense fallback={<main className="px-4 py-6 text-center text-muted sm:px-6">{t('Loading…')}</main>}><main className="px-4 py-6 sm:px-6"><NewSitePage role="customer" liff={{ idToken, profile, onSubmitted: setCompleted }}/></main></Suspense>
}
