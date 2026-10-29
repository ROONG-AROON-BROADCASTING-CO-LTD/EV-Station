import { ChevronDown, CircleUserRound, LayoutDashboard, LogOut, MapPin, Menu, Settings, X } from 'lucide-react'
import { useEffect, useId, useRef, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { LanguageSwitcher, useI18n } from '../i18n/I18nProvider'
import './public-header.css'

type PublicHeaderProps = { authenticated?: boolean; accountName?: string; onLogout?: () => void }

const text = (language: 'th' | 'en', th: string, en: string) => language === 'th' ? th : en

export function PublicHeader({ authenticated = false, accountName, onLogout }: PublicHeaderProps) {
  const { language } = useI18n()
  const location = useLocation()
  const [menuOpen, setMenuOpen] = useState(false)
  const [accountMenuOpen, setAccountMenuOpen] = useState(false)
  const header = useRef<HTMLElement>(null)
  const accountMenu = useRef<HTMLDivElement>(null)
  const menuTrigger = useRef<HTMLButtonElement>(null)
  const accountTrigger = useRef<HTMLButtonElement>(null)
  const menuId = useId()
  const accountMenuId = useId()
  const navItems = [
    { href: '/', label: text(language, 'หน้าแรก', 'Home') },
    { href: '/investment', label: text(language, 'ลงทุน EV', 'Invest in EV') },
    { href: '/branches', label: text(language, 'สาขาและผลงาน', 'Locations & Projects') },
    { href: '/guide/customer', label: text(language, 'คู่มือการใช้งาน', 'User guide') },
    { href: '/contact', label: text(language, 'ติดต่อสอบถาม', 'Contact us') },
  ]
  const investmentLink = authenticated ? '/sites/new' : '/login?mode=register'
  const closeMenus = () => { setMenuOpen(false); setAccountMenuOpen(false) }
  const navLink = (item: (typeof navItems)[number], onClick?: () => void) => <Link key={item.href} to={item.href} aria-current={location.pathname === item.href ? 'page' : undefined} onClick={onClick}>{item.label}</Link>

  useEffect(() => {
    setMenuOpen(false)
    setAccountMenuOpen(false)
  }, [location.pathname, location.search])

  useEffect(() => {
    if (!menuOpen && !accountMenuOpen) return
    const dismissOutside = (event: PointerEvent) => {
      const target = event.target
      if (!(target instanceof Node)) return
      if (!header.current?.contains(target)) setMenuOpen(false)
      if (!accountMenu.current?.contains(target)) setAccountMenuOpen(false)
    }
    const dismissWithKeyboard = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      setMenuOpen(false)
      setAccountMenuOpen(false)
      if (accountMenuOpen) accountTrigger.current?.focus()
      else menuTrigger.current?.focus()
    }
    document.addEventListener('pointerdown', dismissOutside)
    document.addEventListener('keydown', dismissWithKeyboard)
    return () => {
      document.removeEventListener('pointerdown', dismissOutside)
      document.removeEventListener('keydown', dismissWithKeyboard)
    }
  }, [menuOpen, accountMenuOpen])

  useEffect(() => {
    const desktop = window.matchMedia('(min-width: 1121px)')
    const closeOnDesktop = () => { if (desktop.matches) setMenuOpen(false) }
    desktop.addEventListener('change', closeOnDesktop)
    return () => desktop.removeEventListener('change', closeOnDesktop)
  }, [])

  const logout = () => { closeMenus(); onLogout?.() }

  return <header className="landing-header" ref={header}>
    <div className="public-container landing-header-inner">
      <Link to="/" className="landing-brand" aria-label="RBC EV Station home" onClick={closeMenus}><span><strong>RBC <b>EV STATION</b></strong><em>SUSTAINABLE JOURNEY TOGETHER</em></span></Link>
      <nav className="landing-nav" aria-label={text(language, 'เมนูหลัก', 'Main navigation')}>{navItems.map(item => navLink(item, closeMenus))}</nav>
      <div className="landing-actions">
        <LanguageSwitcher />
        {authenticated ? <div className="landing-account-menu" ref={accountMenu}>
          <button ref={accountTrigger} type="button" className="landing-account-trigger" onClick={() => { setAccountMenuOpen(open => !open); setMenuOpen(false) }} aria-label={accountMenuOpen ? text(language, 'ปิดเมนูบัญชี', 'Close account menu') : text(language, 'เปิดเมนูบัญชี', 'Open account menu')} aria-expanded={accountMenuOpen} aria-controls={accountMenuId}>
            <CircleUserRound size={22} aria-hidden="true"/><span>{accountName || text(language, 'บัญชีของคุณ', 'Your account')}</span><ChevronDown className={accountMenuOpen ? 'landing-account-chevron open' : 'landing-account-chevron'} size={16} aria-hidden="true"/>
          </button>
          {accountMenuOpen ? <nav id={accountMenuId} className="landing-account-popover" aria-label={text(language, 'เมนูบัญชี', 'Account menu')}>
            <Link to="/dashboard" onClick={closeMenus}><LayoutDashboard size={18} aria-hidden="true"/>{text(language, 'แดชบอร์ด', 'Dashboard')}</Link>
            <Link to="/sites/new" onClick={closeMenus}><MapPin size={18} aria-hidden="true"/>{text(language, 'ส่งข้อมูลพื้นที่', 'Submit site information')}</Link>
            <Link to="/settings" onClick={closeMenus}><Settings size={18} aria-hidden="true"/>{text(language, 'ตั้งค่า', 'Settings')}</Link>
            {onLogout ? <button type="button" className="landing-account-logout" onClick={logout}><LogOut size={18} aria-hidden="true"/>{text(language, 'ออกจากระบบ', 'Log out')}</button> : null}
          </nav> : null}
        </div> : <Link className="landing-sign-in" to="/login" onClick={closeMenus}><CircleUserRound size={19} aria-hidden="true"/>{text(language, 'เข้าสู่ระบบ', 'Sign in')}</Link>}
      </div>
      <button ref={menuTrigger} type="button" className="landing-menu-button" onClick={() => { setMenuOpen(open => !open); setAccountMenuOpen(false) }} aria-label={menuOpen ? text(language, 'ปิดเมนู', 'Close menu') : text(language, 'เปิดเมนู', 'Open menu')} aria-expanded={menuOpen} aria-controls={menuId}>{menuOpen ? <X size={22} aria-hidden="true"/> : <Menu size={22} aria-hidden="true"/>}</button>
    </div>
    {menuOpen ? <nav id={menuId} className="landing-mobile-menu" aria-label={text(language, 'เมนูหลักบนมือถือ', 'Mobile navigation')}>
      {navItems.map(item => navLink(item, closeMenus))}
      <Link to={investmentLink} onClick={closeMenus}>{text(language, 'ส่งทำเลให้ประเมิน', 'Submit a site')}</Link>
      <div className="landing-mobile-account">
        {authenticated ? <>
          <p><CircleUserRound size={18} aria-hidden="true"/><span>{accountName || text(language, 'บัญชีของคุณ', 'Your account')}</span></p>
          <Link to="/dashboard" onClick={closeMenus}><LayoutDashboard size={18} aria-hidden="true"/>{text(language, 'แดชบอร์ด', 'Dashboard')}</Link>
          <Link to="/settings" onClick={closeMenus}><Settings size={18} aria-hidden="true"/>{text(language, 'ตั้งค่า', 'Settings')}</Link>
          {onLogout ? <button type="button" className="landing-account-logout" onClick={logout}><LogOut size={18} aria-hidden="true"/>{text(language, 'ออกจากระบบ', 'Log out')}</button> : null}
        </> : <Link to="/login" onClick={closeMenus}><CircleUserRound size={19} aria-hidden="true"/>{text(language, 'เข้าสู่ระบบ', 'Sign in')}</Link>}
      </div>
      <div className="landing-mobile-language"><LanguageSwitcher /></div>
    </nav> : null}
  </header>
}
