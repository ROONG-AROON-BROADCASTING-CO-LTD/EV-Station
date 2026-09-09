import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Globe2, ShieldCheck, UserPlus, Users } from 'lucide-react'
import { useState } from 'react'
import { ErrorState, LoadingState } from '../components/PageState'
import { LanguageSwitcher, useI18n } from '../i18n/I18nProvider'
import { api } from '../services/api'
import type { UserRole } from '../types/domain'

const roleKeys: Record<UserRole, string> = { super_admin: 'Super Admin', admin: 'Admin', sales: 'Sales', customer: 'Customer' }

export function SettingsPage({ role }: { role: UserRole }) {
	const { t } = useI18n()
	const queryClient = useQueryClient()
	const [newUser, setNewUser] = useState({ displayName: '', email: '', password: '', role: 'sales' as UserRole })
	const canManageTeam = role === 'super_admin'
	const users = useQuery({ queryKey: ['users'], queryFn: api.listUsers, enabled: canManageTeam })
	const createUser = useMutation({ mutationFn: () => api.createUser(newUser.email, newUser.displayName, newUser.password, newUser.role), onSuccess: () => { setNewUser({ displayName: '', email: '', password: '', role: 'sales' }); queryClient.invalidateQueries({ queryKey: ['users'] }) } })

	if (canManageTeam && users.isLoading) return <LoadingState label={t('Loading page…')} />
	if (canManageTeam && users.isError) return <ErrorState error={users.error} />
	return <div><div className="flex items-start gap-3"><span className="grid h-11 w-11 place-items-center rounded-xl bg-emerald-50 text-brand"><SettingsIcon role={role}/></span><div><h1 className="page-title">{t('Settings')}</h1>{canManageTeam ? <p className="mt-1 text-sm text-muted">{t('Create team accounts. Sales users can access only areas they create.')}</p> : null}</div></div>
		<section className="mt-7 max-w-xl rounded-2xl border border-line bg-white p-6 shadow-panel"><div className="flex items-center gap-2"><Globe2 size={19} className="text-brand"/><h2 className="section-title">{t('Language')}</h2></div><p className="mt-2 text-sm text-muted">{t('Choose display language')}</p><div className="mt-4"><LanguageSwitcher/></div></section>
		{canManageTeam ? <div className="mt-6 grid gap-6 xl:grid-cols-2"><section className="rounded-2xl border border-line bg-white p-6 shadow-panel"><div className="flex items-center gap-2"><UserPlus size={19} className="text-brand"/><h2 className="section-title">{t('Create team account')}</h2></div><form className="mt-5 space-y-4" onSubmit={event => { event.preventDefault(); createUser.mutate() }}><Field label={t('Display name')}><input required value={newUser.displayName} onChange={event => setNewUser({ ...newUser, displayName: event.target.value })} className="input"/></Field><Field label={t('Email')}><input required type="email" value={newUser.email} onChange={event => setNewUser({ ...newUser, email: event.target.value })} className="input"/></Field><Field label={t('Password')}><input required minLength={8} type="password" value={newUser.password} onChange={event => setNewUser({ ...newUser, password: event.target.value })} className="input"/></Field><Field label={t('Role')}><select value={newUser.role} onChange={event => setNewUser({ ...newUser, role: event.target.value as UserRole })} className="input"><option value="admin">{t('Admin')}</option><option value="sales">{t('Sales')}</option></select></Field>{createUser.isError ? <p className="text-sm text-red-700">{t('Unable to create this user.')}</p> : null}<button className="button-primary" disabled={createUser.isPending}>{t(createUser.isPending ? 'Creating…' : 'Create user')}</button></form></section>
			<section className="rounded-2xl border border-line bg-white p-6 shadow-panel"><div className="flex items-center gap-2"><Users size={19} className="text-brand"/><h2 className="section-title">{t('Team accounts')}</h2></div><div className="mt-5 space-y-3">{users.data?.map(user => <div key={user.id} className="rounded-xl border border-line px-4 py-3"><p className="font-bold text-ink">{user.displayName}</p><p className="mt-0.5 text-sm text-muted">{user.email} · {t(roleKeys[user.role])}</p></div>)}</div></section></div> : null}
	</div>
}

function SettingsIcon({ role }: { role: UserRole }) { return role === 'admin' || role === 'super_admin' ? <ShieldCheck size={23}/> : <Globe2 size={23}/> }

function Field({ label, children }: { label: string; children: React.ReactNode }) { return <label className="block text-sm font-semibold text-ink">{label}<span className="mt-1.5 block">{children}</span></label> }
