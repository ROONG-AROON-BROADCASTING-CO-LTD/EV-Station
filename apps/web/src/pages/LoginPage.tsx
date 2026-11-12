import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { ArrowLeft, LogIn, UserPlus, X } from 'lucide-react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api, APIError } from '../services/api'
import { useI18n } from '../i18n/I18nProvider'
import { publicLoginReturn } from '../data/publicContact'
import type { AuthSession } from '../types/domain'

type LoginPageProps = {
  onAuthenticated: (session: AuthSession) => void
  onClose?: () => void
}

export function LoginPage({ onAuthenticated, onClose }: LoginPageProps) {
  const { language, t } = useI18n()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const displayLanguage = searchParams.get('lang') === 'en' ? 'en' : language
  const [mode, setMode] = useState<'login' | 'register' | 'reset'>(() => searchParams.get('mode') === 'register' ? 'register' : 'login')
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [otp, setOTP] = useState('')
  const [delivery, setDelivery] = useState<{ email: string; sentAt: number }>()
  const [resendAvailableAt, setResendAvailableAt] = useState(0)
  const [now, setNow] = useState(Date.now)
  const requestInFlight = useRef(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const normalizedEmail = email.trim().toLowerCase()
  const otpSent = delivery?.email === normalizedEmail
  const resendSeconds = Math.max(0, Math.ceil((resendAvailableAt - now) / 1000))
  const returnPath = publicLoginReturn(searchParams.get('from'), displayLanguage)
  const close = useCallback(() => { if (onClose) onClose(); else navigate(returnPath, { replace: true }) }, [navigate, onClose, returnPath])
  const closeLabel = language === 'th' ? 'ปิดหน้าต่างเข้าสู่ระบบ' : 'Close sign-in dialog'

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') close() }
    document.body.classList.add('auth-modal-open')
    window.addEventListener('keydown', onKeyDown)
    return () => {
      document.body.classList.remove('auth-modal-open')
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [close])

  useEffect(() => {
    if (resendAvailableAt <= Date.now()) return
    const timer = window.setInterval(() => {
      const next = Date.now()
      setNow(next)
      if (next >= resendAvailableAt) window.clearInterval(timer)
    }, 1000)
    return () => window.clearInterval(timer)
  }, [resendAvailableAt])

  const requestCode = async () => {
    // This UI cooldown prevents accidental repeat sends; the server remains the rate-limit authority.
    if (requestInFlight.current || Date.now() < resendAvailableAt) return
    requestInFlight.current = true
    setBusy(true)
    setError('')
    setOTP('')
    try {
      const response = mode === 'reset' ? await api.requestPasswordResetOTP(normalizedEmail) : await api.requestRegistrationOTP(normalizedEmail)
      if (!response.sent) throw new Error('OTP was not sent')
      const sentAt = Date.now()
      setDelivery({ email: normalizedEmail, sentAt })
      setNow(sentAt)
      setResendAvailableAt(sentAt + 60_000)
    } catch (requestError) {
      if (requestError instanceof APIError && requestError.code === 'AUTH_RATE_LIMITED') {
        const limitedAt = Date.now()
        setNow(limitedAt)
        setResendAvailableAt(limitedAt + 15 * 60_000)
        setError(t('Too many attempts. Please wait 15 minutes before trying again.'))
      } else setError(t('Unable to send verification code. Please try again.'))
    } finally {
      requestInFlight.current = false
      setBusy(false)
    }
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (requestInFlight.current) return
    if (mode !== 'login' && !otpSent) { await requestCode(); return }
    requestInFlight.current = true
    setBusy(true)
    setError('')
    try {
      if (mode === 'reset') {
        await api.resetPassword(normalizedEmail, password, otp)
        setMode('login')
        setPassword('')
        setOTP('')
        setDelivery(undefined)
        setNotice(language === 'th' ? 'เปลี่ยนรหัสผ่านแล้ว กรุณาเข้าสู่ระบบด้วยรหัสใหม่' : 'Password reset. Please sign in with your new password.')
        return
      }
      if (mode === 'register') await api.register(normalizedEmail, name, password, otp)
      const session = await api.login(normalizedEmail, password)
      onAuthenticated(session)
    } catch (submitError) {
      if (submitError instanceof APIError && submitError.code === 'OTP_INVALID') setError(t('The verification code is invalid or expired. Request a new code and try again.'))
      else if (submitError instanceof APIError && submitError.code === 'AUTH_RATE_LIMITED') setError(t('Too many attempts. Please wait 15 minutes before trying again.'))
      else if (mode === 'reset') setError(language === 'th' ? 'เปลี่ยนรหัสผ่านไม่สำเร็จ กรุณาตรวจสอบข้อมูลและลองใหม่' : 'Password reset failed. Check your details and try again.')
      else setError(t(mode === 'login' ? 'Email or password is incorrect.' : 'Unable to create the account. Please check the details or use another email.'))
    } finally {
      requestInFlight.current = false
      setBusy(false)
    }
  }

  const switchMode = (nextMode: 'login' | 'register' | 'reset') => {
    if (requestInFlight.current) return
    setNotice('')
    setMode(nextMode)
    setDelivery(undefined)
    setOTP('')
    setError('')
  }

  return <div className="auth-modal-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) close() }}>
    <section className="auth-modal" role="dialog" aria-modal="true" aria-labelledby="auth-modal-title">
      <header className="auth-modal-header">
        <div className="auth-modal-brand"><img src="/rbc-group-logo.png" alt="RBC Group"/><div><p>RBC EV STATION</p><span>{language === 'th' ? 'ระบบประเมินทำเลสถานีชาร์จ' : 'EV charging-site assessment system'}</span></div></div>
        <button type="button" onClick={close} className="auth-modal-close" aria-label={closeLabel} title={closeLabel}><X size={22}/></button>
      </header>
      <div className="auth-modal-body">
        <div className="auth-modal-tabs" role="tablist" aria-label={t('Account access')}>
          <button type="button" role="tab" disabled={busy} aria-selected={mode === 'login'} className={mode === 'login' ? 'active' : ''} onClick={() => switchMode('login')}>{t('Sign in')}</button>
          <button type="button" role="tab" disabled={busy} aria-selected={mode === 'register'} className={mode === 'register' ? 'active' : ''} onClick={() => switchMode('register')}>{t('Sign up')}</button>
        </div>
        <div className="auth-modal-copy"><h1 id="auth-modal-title">{mode === 'login' ? t('Sign in') : mode === 'reset' ? (language === 'th' ? 'กู้รหัสผ่าน' : 'Reset password') : t('Sign up')}</h1><p>{mode === 'login' ? (language === 'th' ? 'กรอกอีเมลและรหัสผ่านเพื่อเข้าสู่ระบบ' : 'Enter your email and password to access your account.') : mode === 'reset' ? (language === 'th' ? 'กรอกอีเมลและรหัสผ่านใหม่ แล้วขอรหัสยืนยันทางอีเมล' : 'Enter your email and a new password, then request a verification code.') : (language === 'th' ? 'สร้างบัญชีเพื่อส่งทำเลและติดตามโครงการของคุณ' : 'Create an account to submit sites and track your project.')}</p></div>
        <form className="auth-modal-form" onSubmit={submit}>
          {searchParams.get('reason') === 'expired' ? <p role="status">{language === 'th' ? 'เซสชันหมดอายุ กรุณาเข้าสู่ระบบอีกครั้งเพื่อกลับไปทำงานต่อ หากมีไฟล์ที่ยังไม่ได้ส่ง กรุณาเลือกไฟล์อีกครั้ง' : 'Your session expired. Sign in again to continue. Please reselect any files that were not uploaded.'}</p> : null}
          {mode === 'register' ? <label>{t('Display name')}<input required disabled={busy} autoComplete="name" value={name} onChange={event => setName(event.target.value)} autoFocus /></label> : null}
          <label>{t('Email')}<input required disabled={busy} type="email" autoComplete="email" value={email} onChange={event => { setEmail(event.target.value); setDelivery(undefined); setOTP(''); setError('') }} autoFocus={mode === 'login'} /></label>
          <label>{t('Password')}<input required disabled={busy} minLength={8} type="password" autoComplete={mode === 'login' ? 'current-password' : 'new-password'} value={password} onChange={event => setPassword(event.target.value)} /></label>
          {mode !== 'login' && otpSent && delivery ? <><div role="status" className="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm leading-6 text-emerald-900"><p className="break-all">{t('Verification code sent to')} <strong>{delivery.email}</strong></p><p>{t('Sent at')} <time dateTime={new Date(delivery.sentAt).toISOString()}>{new Date(delivery.sentAt).toLocaleTimeString(language === 'th' ? 'th-TH' : 'en-GB')}</time></p><p>{t('Use the newest code. Check your inbox and spam folder.')}</p></div><label>{t('Verification code')}<input required disabled={busy} inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" maxLength={6} value={otp} onChange={event => setOTP(event.target.value.replace(/\D/g, ''))} autoFocus /></label><button type="button" className="button-secondary" disabled={busy || resendSeconds > 0} onClick={() => void requestCode()}>{t('Resend verification code')}</button></> : null}
          {mode !== 'login' && resendSeconds > 0 ? <p className="text-sm text-muted">{t('Send again in')} {resendSeconds} {t('seconds')}</p> : null}
          {notice ? <p role="status" className="text-sm text-emerald-800">{notice}</p> : null}
          {error ? <p role="alert" className="auth-modal-error">{error}</p> : null}
          <button disabled={busy || (mode !== 'login' && !otpSent && resendSeconds > 0)} className="auth-modal-submit" type="submit">{mode === 'login' ? <LogIn size={19}/> : <UserPlus size={19}/>} {busy ? t(mode === 'register' && !otpSent ? 'Sending verification code…' : 'Continue') : mode === 'login' ? t('Sign in') : otpSent ? (mode === 'reset' ? (language === 'th' ? 'เปลี่ยนรหัสผ่าน' : 'Reset password') : t('Create account and sign in')) : t('Send verification code')}</button>
        </form>
        <p className="auth-modal-note">{t('Customer accounts can access only the areas submitted by their own account.')}</p>
        {mode === 'login' ? <button type="button" className="auth-modal-note underline" onClick={() => switchMode('reset')}>{language === 'th' ? 'ลืมรหัสผ่าน?' : 'Forgot password?'}</button> : null}
        {mode === 'login' ? <p className="auth-modal-note"><a className="underline" href="mailto:rbcevstation@gmail.com">{t('Need help signing in? Contact the RBC team')}</a></p> : <details className="auth-modal-note"><summary>{t('How RBC uses your information')}</summary><p className="mt-2">{t('RBC uses your email to verify your account. When you submit a site, the team uses the location, details and attachments you provide to assess the site and contact you about the project.')}</p><a className="underline" href="mailto:rbcevstation@gmail.com">{t('Contact RBC about your information')}</a></details>}
        <button type="button" className="auth-modal-return" onClick={close}><ArrowLeft size={17}/>{t('Back to previous page')}</button>
      </div>
    </section>
  </div>
}
