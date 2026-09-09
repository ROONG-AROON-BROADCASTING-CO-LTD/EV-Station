import { Gauge, LogOut, MapPin, PanelLeftClose, PanelLeftOpen, Settings } from 'lucide-react'
import { useState, type PropsWithChildren } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import { useI18n } from '../i18n/I18nProvider'
import type { UserRole } from '../types/domain'

const nav = [{ to: '/', label: 'Dashboard', icon: Gauge }, { to: '/sites/new', label: 'Sites', icon: MapPin }]

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
    if (item.to !== '/') return { ...item, label: 'Submit site information' }
    if (role === 'customer') return { ...item, label: 'Your site information' }
    if (role === 'sales') return { ...item, label: 'My customer sites' }
    return { ...item, label: 'All customer areas' }
  })
  const navigationWithSettings = [...visibleNavigation, { to: '/settings', label: 'Settings', icon: Settings }]
  const activeNavigation = navigationWithSettings.find(item => pathname === item.to) ?? navigationWithSettings[0]
  return <div className="app-shell min-h-screen bg-[#f7f9fc] text-ink lg:grid" data-sidebar={sidebarOpen ? 'open' : 'collapsed'}>
    <aside className="hidden min-h-screen flex-col bg-[#042447] text-white lg:sticky lg:top-0 lg:flex lg:h-screen">
      <div className={`flex items-center ${sidebarOpen ? 'min-h-36 border-b border-white/10 px-7' : 'h-16 justify-center px-2'}`}><div className={`flex items-center ${sidebarOpen ? 'gap-3' : ''}`}><span className={`grid place-items-center overflow-hidden rounded-full border border-[#d9b54c] bg-[#070b14] shadow-[0_10px_30px_rgba(0,0,0,0.2)] ${sidebarOpen ? 'h-[62px] w-[62px]' : 'h-10 w-10 border-0 shadow-none'}`}><img className="h-full w-full object-contain" src="/rbc-group-logo.jpg" alt="RBC Group"/></span>{sidebarOpen ? <div><p className="text-[22px] font-extrabold tracking-tight">RBC</p><p className="text-xs font-bold tracking-[0.13em] text-[#79e6a5]">EV STATION</p></div> : null}</div></div>
      <nav className={`space-y-1 ${sidebarOpen ? 'px-3 py-6' : 'px-2 py-4'}`} aria-label={t('Primary navigation')}>{visibleNavigation.map(({ to, label, icon: Icon }) => <NavLink key={to} to={to} end={to === '/'} title={t(label)} aria-label={t(label)} className={({ isActive }) => `relative flex text-sm font-bold transition ${sidebarOpen ? 'rounded-r-lg px-5 py-3.5 gap-3' : 'h-11 items-center justify-center rounded-lg'} ${isActive ? sidebarOpen ? 'bg-white/10 text-white before:absolute before:bottom-2 before:left-0 before:top-2 before:w-1 before:rounded-r-full before:bg-[#36d878]' : 'bg-white/10 text-white' : 'text-slate-300 hover:bg-white/5 hover:text-white'}`}><Icon size={sidebarOpen ? 19 : 21} strokeWidth={1.9}/><span className={sidebarOpen ? '' : 'hidden'}>{t(label)}</span></NavLink>)}</nav>
      {sidebarOpen ? <div className="mt-auto px-4 pb-4"><div title={userName} className="rounded-xl bg-white/10 px-3.5 py-3"><p className="truncate text-sm font-bold text-white">{userName}</p><p className="mt-1 text-xs font-semibold text-[#79e6a5]">{t(roleLabel(role))}</p></div><NavLink to="/settings" title={t('Settings')} aria-label={t('Settings')} className={({ isActive }) => `mt-3 flex gap-3 rounded-lg px-3.5 py-3 text-sm font-bold transition ${isActive ? 'bg-white/10 text-white' : 'text-slate-300 hover:bg-white/5 hover:text-white'}`}><Settings size={18}/>{t('Settings')}</NavLink><div className="mt-2 border-t border-white/10 pt-3"><button onClick={onLogout} title={t('Log out')} aria-label={t('Log out')} className="flex w-full gap-3 rounded-lg px-3.5 py-3 text-left text-sm font-bold text-slate-300 transition hover:bg-white/5 hover:text-white"><LogOut size={18}/>{t('Log out')}</button></div></div> : <div className="mt-auto flex flex-col items-center gap-1.5 pb-4"><NavLink to="/settings" title={t('Settings')} aria-label={t('Settings')} className={({ isActive }) => `grid h-10 w-10 place-items-center rounded-lg transition ${isActive ? 'bg-white/10 text-white' : 'text-slate-300 hover:bg-white/5 hover:text-white'}`}><Settings size={20}/></NavLink><button onClick={onLogout} title={t('Log out')} aria-label={t('Log out')} className="grid h-10 w-10 place-items-center rounded-lg text-slate-300 transition hover:bg-white/5 hover:text-white"><LogOut size={20}/></button><span title={userName} className="mt-2 grid h-8 w-8 place-items-center rounded-full bg-[#36d878] text-xs font-black text-[#042447]">{userName.slice(0, 1).toUpperCase()}</span></div>}
    </aside>
    <div className="min-w-0"><header className="flex h-16 items-center border-b border-slate-200 bg-white px-5 sm:px-7 lg:px-8"><button type="button" onClick={() => setSidebarOpen(open => !open)} className="mr-3 hidden h-9 w-9 items-center justify-center rounded-lg text-slate-600 transition hover:bg-slate-100 hover:text-[#08244d] lg:inline-flex" aria-label={t(sidebarOpen ? 'Close sidebar' : 'Open sidebar')} title={t(sidebarOpen ? 'Close sidebar' : 'Open sidebar')}>{sidebarOpen ? <PanelLeftClose size={20}/> : <PanelLeftOpen size={20}/>}</button><div className="flex items-center gap-2 font-extrabold text-[#08244d]"><span className="grid h-9 w-9 place-items-center overflow-hidden rounded-full border border-[#d4ad43] bg-[#070b14] lg:hidden"><img className="h-full w-full object-contain" src="/rbc-group-logo.jpg" alt="RBC Group"/></span><span>{t(activeNavigation.label)}</span></div></header><main className="mx-auto w-full max-w-[1600px] px-4 py-6 pb-24 sm:px-7 lg:px-8 lg:py-8">{children}</main><nav className="fixed inset-x-0 bottom-0 z-30 grid grid-cols-3 border-t border-slate-200 bg-white/95 px-3 py-2 shadow-[0_-8px_24px_rgba(15,35,70,0.08)] backdrop-blur lg:hidden" aria-label={t('Primary navigation')}>{navigationWithSettings.map(({ to, label, icon: Icon }) => <NavLink key={to} to={to} end={to === '/'} className={({ isActive }) => `flex min-h-12 flex-col items-center justify-center gap-0.5 rounded-lg text-[11px] font-bold ${isActive ? 'bg-emerald-50 text-emerald-700' : 'text-slate-500'}`}><Icon size={18}/>{t(label)}</NavLink>)}</nav></div>
  </div>
}
