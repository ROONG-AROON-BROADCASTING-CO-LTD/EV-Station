import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { ArrowLeft, LogIn, UserPlus, X } from 'lucide-react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api } from '../services/api'
import { useI18n } from '../i18n/I18nProvider'
import type { AuthSession } from '../types/domain'

type LoginPageProps = {
  onAuthenticated: (session: AuthSession) => void
  onClose?: () => void
}

export function LoginPage({ onAuthenticated, onClose }: LoginPageProps) {
  const { language, t } = useI18n()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [mode, setMode] = useState<'login' | 'register'>(() => searchParams.get('mode') === 'register' ? 'register' : 'login')
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [otp, setOTP] = useState('')
  const [otpSent, setOTPSent] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const close = useCallback(() => { if (onClose) onClose(); else navigate('/') }, [navigate, onClose])
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

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      if (mode === 'register' && !otpSent) {
        await api.requestRegistrationOTP(email)
        setOTPSent(true)
        return
      }
      if (mode === 'register') await api.register(email, name, password, otp)
      const session = await api.login(email, password)
      onAuthenticated(session)
    } catch {
      setError(t(mode === 'login' ? 'Email or password is incorrect.' : 'Unable to create the account. Please check the details or use another email.'))
    } finally {
      setBusy(false)
    }
  }

  const switchMode = (nextMode: 'login' | 'register') => {
    setMode(nextMode)
    setOTPSent(false)
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
          <button type="button" role="tab" aria-selected={mode === 'login'} className={mode === 'login' ? 'active' : ''} onClick={() => switchMode('login')}>{t('Sign in')}</button>
          <button type="button" role="tab" aria-selected={mode === 'register'} className={mode === 'register' ? 'active' : ''} onClick={() => switchMode('register')}>{t('Sign up')}</button>
        </div>
        <div className="auth-modal-copy"><h1 id="auth-modal-title">{mode === 'login' ? t('Sign in') : t('Sign up')}</h1><p>{mode === 'login' ? (language === 'th' ? 'กรอกอีเมลและรหัสผ่านเพื่อเข้าสู่ระบบ' : 'Enter your email and password to access your account.') : (language === 'th' ? 'สร้างบัญชีเพื่อส่งทำเลและติดตามโครงการของคุณ' : 'Create an account to submit sites and track your project.')}</p></div>
        <form className="auth-modal-form" onSubmit={submit}>
          {mode === 'register' ? <label>{t('Display name')}<input required autoComplete="name" value={name} onChange={event => setName(event.target.value)} autoFocus /></label> : null}
          <label>{t('Email')}<input required type="email" autoComplete="email" value={email} onChange={event => setEmail(event.target.value)} autoFocus={mode === 'login'} /></label>
          <label>{t('Password')}<input required minLength={8} type="password" autoComplete={mode === 'login' ? 'current-password' : 'new-password'} value={password} onChange={event => setPassword(event.target.value)} /></label>
          {mode === 'register' && otpSent ? <label>{t('Verification code')}<input required inputMode="numeric" maxLength={6} value={otp} onChange={event => setOTP(event.target.value)} autoFocus /></label> : null}
          {error ? <p role="alert" className="auth-modal-error">{error}</p> : null}
          <button disabled={busy} className="auth-modal-submit" type="submit">{mode === 'login' ? <LogIn size={19}/> : <UserPlus size={19}/>} {busy ? t('Continue') : mode === 'login' ? t('Sign in') : otpSent ? t('Create account and sign in') : t('Send verification code')}</button>
        </form>
        <p className="auth-modal-note">{t('Customer accounts can access only the areas submitted by their own account.')}</p>
        <button type="button" className="auth-modal-return" onClick={close}><ArrowLeft size={17}/>{language === 'th' ? 'กลับหน้าแรก' : 'Back to home'}</button>
      </div>
    </section>
  </div>
}