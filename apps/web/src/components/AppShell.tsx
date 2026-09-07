import { BarChart3, Database, Gauge, LogOut, MapPin, Menu, Settings, Zap } from 'lucide-react'
import type { PropsWithChildren } from 'react'
import { NavLink } from 'react-router-dom'
import { LanguageSwitcher, useI18n } from '../i18n/I18nProvider'

const nav = [
  { to: '/', label: 'Dashboard', icon: Gauge },
  { to: '/sites/new', label: 'Sites', icon: MapPin },
  { to: '/analyses', label: 'Analyses', icon: BarChart3 },
  { to: '/data-sources', label: 'Data Sources', icon: Database },
]

export function AppShell({ children, onLogout, userName, role }: PropsWithChildren<{ onLogout: () => void; userName: string; role: 'owner' | 'sales' | 'viewer' }>) {
  const { t } = useI18n()
  return <div className="min-h-screen bg-white text-ink lg:grid lg:grid-cols-[236px_minmax(0,1fr)]">
    <aside className="hidden border-r border-line bg-white lg:flex lg:min-h-screen lg:flex-col lg:sticky lg:top-0 lg:h-screen">
      <div className="flex h-28 items-center gap-3 border-b border-line px-7">
        <span className="relative grid h-14 w-14 place-items-center overflow-hidden rounded-full border-2 border-brand bg-white"><Zap className="absolute z-0 text-brand opacity-15" size={34} /><img className="relative z-10 h-full w-full object-contain" src="/rbc-group-logo.jpg" alt="RBC Group" /></span>
        <div><div className="text-2xl font-extrabold tracking-tight">RBC</div><div className="text-sm font-bold tracking-wide text-brand">EV STATION</div></div>
      </div>
      <nav className="space-y-1 p-4" aria-label={t('Primary navigation')}>
        {nav.map(({to, label, icon: Icon}) => <NavLink key={to} to={to} end={to === '/'} className={({isActive}) => `flex items-center gap-3 rounded-lg px-4 py-3 text-sm font-semibold transition ${isActive ? 'bg-emerald-50 text-brand' : 'text-ink hover:bg-canvas'}`}>
          <Icon size={19} strokeWidth={1.8} />{t(label)}
        </NavLink>)}
      </nav>
	  <div className="mt-auto space-y-4 px-6 py-6"><div className="rounded-xl bg-slate-50 px-3 py-2 text-xs"><p className="font-bold text-ink">{userName}</p><p className="mt-0.5 text-brand">{t(role === 'owner' ? 'Owner' : role === 'sales' ? 'Sales' : 'Viewer')}</p></div><LanguageSwitcher />{role === 'owner' ? <NavLink to="/settings" className="flex items-center gap-3 text-sm font-semibold text-muted hover:text-ink"><Settings size={19} />{t('Settings')}</NavLink> : null}<button onClick={onLogout} className="flex items-center gap-3 text-sm font-semibold text-muted hover:text-ink"><LogOut size={19} />{t('Log out')}</button></div>
    </aside>
    <div className="min-w-0">
      <header className="flex h-16 items-center justify-between border-b border-line px-5 lg:hidden">
        <div className="flex items-center gap-2 font-extrabold"><span className="relative grid h-9 w-9 place-items-center overflow-hidden rounded-full border border-brand bg-white"><Zap className="absolute z-0 text-brand opacity-15" size={23} /><img className="relative z-10 h-full w-full object-contain" src="/rbc-group-logo.jpg" alt="RBC Group" /></span>RBC EV STATION</div><div className="flex items-center gap-3"><LanguageSwitcher /><Menu size={22} /></div>
      </header>
      <main className="mx-auto w-full max-w-[1500px] px-5 py-7 sm:px-8 lg:px-10 lg:py-9">{children}</main>
    </div>
  </div>
}
