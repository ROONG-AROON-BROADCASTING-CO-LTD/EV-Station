import { ChevronDown, CircleUserRound, LayoutDashboard, LogOut, MapPin, Menu, Settings, X } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { LanguageSwitcher, useI18n } from '../i18n/I18nProvider'

type PublicHeaderProps = { authenticated?: boolean; accountName?: string; onLogout?: () => void }

const text = (language: 'th' | 'en', th: string, en: string) => language === 'th' ? th : en

export function PublicHeader({ authenticated = false, accountName, onLogout }: PublicHeaderProps) {
  const { language } = useI18n()
  const [menuOpen, setMenuOpen] = useState(false)
  const [accountMenuOpen, setAccountMenuOpen] = useState(false)
  const navItems = [
    { href: '/', label: text(language, 'หน้าแรก', 'Home') },
    { href: '/investment', label: text(language, 'ลงทุน EV', 'Invest in EV') },
    { href: '/branches', label: text(language, 'สาขาและผลงาน', 'Branches & work') },
    { href: '/guide/customer', label: text(language, 'คู่มือการใช้งาน', 'User guide') },
    { href: '/contact', label: text(language, 'ติดต่อสอบถาม', 'Contact us') },
  ]
  const investmentLink = authenticated ? '/sites/new' : '/login?mode=register'
  const navLink = (item: (typeof navItems)[number], onClick?: () => void) => <Link key={item.href} to={item.href} onClick={onClick}>{item.label}</Link>

  return <header className="landing-header">
    <Link to="/" className="landing-brand" aria-label="RBC EV Station home"><span><strong>RBC <b>EV STATION</b></strong><em>SUSTAINABLE JOURNEY TOGETHER</em></span></Link>
    <nav className="landing-nav" aria-label={text(language, 'เมนูหลัก', 'Main navigation')}>{navItems.map(item => navLink(item))}</nav>
    <div className="landing-actions"><LanguageSwitcher />{authenticated ? <div className="landing-account-menu"><button type="button" className="landing-account-trigger" onClick={() => setAccountMenuOpen(open => !open)} aria-label={text(language, 'เปิดเมนูบัญชี', 'Open account menu')} aria-expanded={accountMenuOpen}><CircleUserRound size={22}/><span>{accountName || text(language, 'บัญชีของคุณ', 'Your account')}</span><ChevronDown className={accountMenuOpen ? 'landing-account-chevron open' : 'landing-account-chevron'} size={16}/></button>{accountMenuOpen ? <nav className="landing-account-popover" aria-label={text(language, 'เมนูบัญชี', 'Account menu')}><Link to="/dashboard" onClick={() => setAccountMenuOpen(false)}><LayoutDashboard size={18}/>{text(language, 'แดชบอร์ด', 'Dashboard')}</Link><Link to="/sites/new" onClick={() => setAccountMenuOpen(false)}><MapPin size={18}/>{text(language, 'ส่งข้อมูลพื้นที่', 'Submit site information')}</Link><Link to="/settings" onClick={() => setAccountMenuOpen(false)}><Settings size={18}/>{text(language, 'ตั้งค่า', 'Settings')}</Link>{onLogout ? <button type="button" className="landing-account-logout" onClick={() => { setAccountMenuOpen(false); onLogout() }}><LogOut size={18}/>{text(language, 'ออกจากระบบ', 'Log out')}</button> : null}</nav> : null}</div> : <Link className="landing-sign-in" to="/login"><CircleUserRound size={19}/>{text(language, 'เข้าสู่ระบบ', 'Sign in')}</Link>}</div>
    <button type="button" className="landing-menu-button" onClick={() => setMenuOpen(open => !open)} aria-label={text(language, 'เปิดเมนู', 'Open menu')} aria-expanded={menuOpen}>{menuOpen ? <X size={22}/> : <Menu size={22}/>}</button>
    {menuOpen ? <div className="landing-mobile-menu">{navItems.map(item => navLink(item, () => setMenuOpen(false)))}<Link to={investmentLink} onClick={() => setMenuOpen(false)}>{text(language, 'ส่งทำเลให้ประเมิน', 'Submit a site')}</Link><div className="landing-mobile-language"><LanguageSwitcher /></div></div> : null}
  </header>
}
