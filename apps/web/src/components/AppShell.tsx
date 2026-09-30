import { Ban, BarChart3, Bell, ChevronDown, CircleHelp, Gauge, Globe2, Home, LogOut, MapPin, PanelLeftClose, PanelLeftOpen, Search, Settings } from 'lucide-react'
import { useState, type PropsWithChildren } from 'react'
import { Link, NavLink, useLocation } from 'react-router-dom'
import { useI18n } from '../i18n/I18nProvider'
import type { UserRole } from '../types/domain'

const nav = [{ to: '/dashboard', label: 'Dashboard', icon: Gauge }, { to: '/sites/new', label: 'Sites', icon: MapPin }]

function roleLabel(role: UserRole) {
  if (role === 'super_admin') return 'Super Admin'
  if (role === 'admin') return 'Admin'
  if (role === 'sales') return 'Sales'
  return 'Customer'
}

export function AppShell({ children, onLogout, userName, role }: PropsWithChildren<{ onLogout: () => void; userName: string; role: UserRole }>) {
  const { t } = useI18n()
	const { pathname } = useLocation()
	const [sidebarOpen, setSidebarOpen] = useState(true)
  const visibleNavigation = nav.map(item => {
    if (item.to !== '/dashboard') return { ...item, label: 'Submit site information' }
    if (role === 'customer') return { ...item, label: 'Your site information' }
    if (role === 'sales') return { ...item, label: 'My customer sites' }
    return { ...item, label: 'All customer areas' }
  })
  const managementNavigation = [
    ...(role === 'super_admin' || role === 'admin' ? [{ to: '/api-usage', label: 'API usage', icon: BarChart3 }] : []),
    ...(role === 'super_admin' || role === 'admin' || role === 'sales' ? [{ to: '/not-investing', label: 'Sites not investing', icon: Ban }] : []),
  ]
  const guidePath = role === 'customer' ? '/guide/customer' : '/guide/admin'
  const guideNavigation = { to: guidePath, label: 'Usage guide', icon: CircleHelp }
  const navigationWithSettings = [...visibleNavigation, ...managementNavigation, guideNavigation, { to: '/settings', label: 'Settings', icon: Settings }]
  const activeNavigation = navigationWithSettings.find(item => pathname === item.to)
    ?? (pathname.startsWith('/analysis/') ? { to: pathname, label: 'Analysis result', icon: BarChart3 }
      : pathname.startsWith('/sites/') ? visibleNavigation[1]
      : navigationWithSettings[0])
  return <div className="app-shell min-h-screen bg-[#f7f9fc] text-ink lg:grid" data-sidebar={sidebarOpen ? 'open' : 'collapsed'}>
    <aside className="sidebar-rail hidden min-h-screen flex-col bg-[#042447] text-white lg:sticky lg:top-0 lg:flex lg:h-screen">
      <button type="button" onClick={() => setSidebarOpen(open => !open)} className="sidebar-toggle" aria-label={t(sidebarOpen ? 'Close sidebar' : 'Open sidebar')} title={t(sidebarOpen ? 'Close sidebar' : 'Open sidebar')}>{sidebarOpen ? <PanelLeftClose size={18}/> : <PanelLeftOpen size={18}/>}</button>
      <div className="sidebar-brand flex min-h-36 items-center border-b border-white/10 px-7"><div className="flex items-center gap-3"><span className="grid h-[62px] w-[62px] shrink-0 place-items-center overflow-hidden rounded-full border border-[#d9b54c] bg-[#070b14] shadow-[0_10px_30px_rgba(0,0,0,0.2)]"><img className="h-full w-full object-contain" src="/rbc-group-logo.png" alt="RBC Group"/></span><div className="sidebar-copy whitespace-nowrap"><p className="text-[22px] font-extrabold tracking-tight">RBC</p><p className="text-xs font-bold tracking-[0.13em] text-[#79e6a5]">EV STATION</p></div></div></div>
      <nav className="sidebar-navigation space-y-1 px-3 py-6" aria-label={t('Primary navigation')}>{[...visibleNavigation, ...managementNavigation, guideNavigation].map(({ to, label, icon: Icon }) => <NavLink key={to} to={to} end={to === '/dashboard'} title={t(label)} aria-label={t(label)} className={({ isActive }) => `sidebar-nav-link relative flex items-center gap-3 rounded-lg px-5 py-3.5 text-sm font-bold transition ${isActive ? 'bg-white/10 text-white before:absolute before:bottom-2 before:left-0 before:top-2 before:w-1 before:rounded-r-full before:bg-[#00a5a5]' : 'text-slate-300 hover:bg-white/5 hover:text-white'}`}><Icon className="sidebar-nav-icon shrink-0" size={19} strokeWidth={1.9}/><span className="sidebar-copy whitespace-nowrap">{t(label)}</span></NavLink>)}</nav>
      <div className="sidebar-account mt-auto px-4 pb-4"><div title={userName} className="sidebar-user-card rounded-xl bg-white/10 px-3.5 py-3"><p className="sidebar-copy truncate text-sm font-bold text-white">{userName}</p><p className="sidebar-copy mt-1 text-xs font-semibold text-[#8bd7d2]">{t(roleLabel(role))}</p></div><a href="/" title={t('Back to main website')} aria-label={t('Back to main website')} className="sidebar-nav-link mt-3 flex items-center gap-3 rounded-lg px-3.5 py-3 text-sm font-bold text-slate-300 transition hover:bg-white/5 hover:text-white"><Home className="sidebar-nav-icon shrink-0" size={18}/><span className="sidebar-copy whitespace-nowrap">{t('Back to main website')}</span></a><NavLink to="/settings" title={t('Settings')} aria-label={t('Settings')} className={({ isActive }) => `sidebar-nav-link mt-2 flex items-center gap-3 rounded-lg px-3.5 py-3 text-sm font-bold transition ${isActive ? 'bg-white/10 text-white' : 'text-slate-300 hover:bg-white/5 hover:text-white'}`}><Settings className="sidebar-nav-icon shrink-0" size={18}/><span className="sidebar-copy whitespace-nowrap">{t('Settings')}</span></NavLink><div className="mt-2 border-t border-white/10 pt-3"><button onClick={onLogout} title={t('Log out')} aria-label={t('Log out')} className="sidebar-nav-link flex w-full items-center gap-3 rounded-lg px-3.5 py-3 text-left text-sm font-bold text-slate-300 transition hover:bg-white/5 hover:text-white"><LogOut className="sidebar-nav-icon shrink-0" size={18}/><span className="sidebar-copy whitespace-nowrap">{t('Log out')}</span></button></div></div>
    </aside>
    <div className="min-w-0"><header className="flex h-[72px] items-center border-b border-slate-200 bg-white px-4 sm:px-7 lg:px-8"><button type="button" onClick={() => setSidebarOpen(open => !open)} className="mr-3 hidden h-11 w-11 items-center justify-center rounded-lg text-slate-600 transition hover:bg-slate-100 hover:text-[#08244d] lg:inline-flex" aria-label={t(sidebarOpen ? 'Close sidebar' : 'Open sidebar')} title={t(sidebarOpen ? 'Close sidebar' : 'Open sidebar')}>{sidebarOpen ? <PanelLeftClose size={20}/> : <PanelLeftOpen size={20}/>}</button><div className="relative hidden max-w-[450px] flex-1 lg:block"><Search className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" size={19}/><input className="h-10 w-full rounded-lg border border-slate-200 bg-slate-50 pl-10 pr-4 text-sm text-slate-700 outline-none focus:border-[#009b4c]" placeholder="Search sites, locations, franchisees…"/></div><div className="flex min-w-0 items-center gap-2 font-extrabold text-[#08244d] lg:hidden"><span className="grid h-8 w-8 shrink-0 place-items-center overflow-hidden rounded-full border border-[#d4ad43] bg-[#070b14]"><img className="h-full w-full object-contain" src="/rbc-group-logo.png" alt="RBC Group"/></span><span className="truncate text-[15px] sm:text-base">{t(activeNavigation.label)}</span></div><div className="ml-auto flex items-center gap-2 sm:gap-3"><button className="hidden h-10 items-center gap-2 rounded-lg border border-slate-200 px-3 text-sm font-bold text-[#08244d] sm:inline-flex"><Globe2 size={17}/>Global<ChevronDown size={15}/></button><button className="relative grid h-10 w-10 place-items-center rounded-lg text-[#08244d] hover:bg-slate-50"><Bell size={19}/><span className="absolute right-2 top-2 h-2 w-2 rounded-full bg-[#009b4c]"/></button><Link to={guidePath} className="hidden h-10 w-10 place-items-center rounded-lg text-[#08244d] hover:bg-slate-50 sm:grid" aria-label={t('Usage guide')} title={t('Usage guide')}><CircleHelp size={20}/></Link><span className="grid h-9 w-9 place-items-center rounded-full bg-[#253f6b] text-xs font-black text-white">{userName.slice(0, 2).toUpperCase()}</span></div></header><main className="mx-auto w-full max-w-[1680px] px-4 py-5 pb-28 sm:px-7 sm:py-6 sm:pb-24 lg:px-8 lg:py-7">{children}</main><nav className="fixed inset-x-0 bottom-0 z-30 grid border-t border-slate-200 bg-white/95 px-2 pt-2 shadow-[0_-8px_24px_rgba(15,35,70,0.08)] backdrop-blur lg:hidden" style={{ gridTemplateColumns: `repeat(${navigationWithSettings.length}, minmax(0, 1fr))`, paddingBottom: 'calc(env(safe-area-inset-bottom) + 0.5rem)' }} aria-label={t('Primary navigation')}>{navigationWithSettings.map(({ to, label, icon: Icon }) => <NavLink key={to} to={to} end={to === '/dashboard'} className={({ isActive }) => `flex min-h-14 min-w-0 flex-col items-center justify-center gap-1 rounded-lg px-1 text-center text-[11px] font-bold leading-4 ${isActive ? 'bg-emerald-50 text-emerald-700' : 'text-slate-500'}`}><Icon size={19}/><span className="line-clamp-2">{t(label)}</span></NavLink>)}</nav></div>
  </div>
}
