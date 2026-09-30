import { useMutation, useQuery } from '@tanstack/react-query'
import { ArrowLeft, Database, FileImage, FileText, MapPin, Play } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { DataStatus } from '../components/DataStatus'
import { MapPanel, RadiusLabel, RadiusSelector } from '../components/MapPanel'
import { ErrorState, LoadingState } from '../components/PageState'
import { useI18n } from '../i18n/I18nProvider'
import { api } from '../services/api'
import type { SiteAttachment, UserRole } from '../types/domain'

export function SitePage({ role }: { role: UserRole }) {
  const { t } = useI18n()
	const { id = '' } = useParams()
	const navigate = useNavigate()
  const [radiusMeters, setRadiusMeters] = useState(3000)
  const site = useQuery({ queryKey: ['site', id], queryFn: () => api.getSite(id), enabled: Boolean(id) })
  const attachments = useQuery({ queryKey: ['site-images', id], queryFn: () => api.listSiteImages(id), enabled: Boolean(id) })
	const analysis = useMutation({ mutationFn: () => api.runAnalysis(id, radiusMeters), onSuccess: run => navigate(`/analysis/${run.id}`) })
  if (site.isLoading) return <LoadingState label={t('Loading site…')} />
  if (site.isError || !site.data) return <ErrorState error={site.error} message="error.SITE_NOT_FOUND" />
  const value = site.data
  const canRunAnalysis = role === 'super_admin' || role === 'admin' || role === 'sales'
  return <div><Link to="/dashboard" className="inline-flex items-center gap-2 text-sm font-semibold text-muted hover:text-ink"><ArrowLeft size={16}/>{t('Dashboard')}</Link><div className="mt-5 flex flex-wrap items-start justify-between gap-5"><div><h1 className="page-title">{value.name}</h1><div className="mt-3 flex flex-wrap items-center gap-4"><DataStatus status={value.inputStatus}/><span className="text-sm text-muted">{t('Customer-supplied inputs have not been independently verified.')}</span></div></div>{canRunAnalysis ? <button className="button-primary" onClick={() => analysis.mutate()} disabled={analysis.isPending}><Play size={17}/>{t(analysis.isPending ? 'Running…' : 'Run Analysis')}</button> : null}</div>
    {canRunAnalysis ? <div className="mt-6 rounded-xl border border-line bg-slate-50 p-4"><p className="text-sm font-bold text-ink">{t('Analysis radius')}</p><p className="mt-1 text-xs leading-5 text-muted">{t('Choose the distance around this site used to collect area data.')}</p><div className="mt-3"><RadiusSelector value={radiusMeters} onChange={setRadiusMeters} /></div></div> : <div className="mt-6 rounded-xl border border-emerald-100 bg-emerald-50/60 p-4 text-sm leading-6 text-emerald-900">{t('The RBC team will review your information and begin the site analysis when it is ready.')}</div>}
    <div className="mt-6 grid gap-5 xl:mt-8 xl:grid-cols-[0.72fr_1.28fr]"><section className="rounded-xl border border-line p-4 shadow-panel sm:p-6"><h2 className="section-title">{t('Site record')}</h2><dl className="mt-5 space-y-4 sm:mt-6 sm:space-y-5"><Detail icon={MapPin} label={t('Location')} value={value.address || `${value.latitude}, ${value.longitude}`}/><Detail icon={Database} label={t('Land size')} value={`${value.landSize.toLocaleString()} ${t(value.landSizeUnit)}`}/></dl><div className="mt-6 border-t border-line pt-4 sm:mt-7 sm:pt-5"><h3 className="text-sm font-bold">{t('Input provenance')}</h3><p className="mt-2 text-sm leading-6 text-muted">{t('Source: User supplied')}<br/>{t('Status: Preliminary')}<br/>{t('Verification: Not completed')}</p></div></section><section><div className="mb-3 flex items-center justify-between"><h2 className="section-title">{t('Map')}</h2><RadiusLabel meters={radiusMeters}/></div><MapPanel latitude={value.latitude} longitude={value.longitude} radiusMeters={radiusMeters} className="min-h-[280px] sm:min-h-[480px]"/></section></div>
    <SiteEvidence siteID={id} attachments={attachments.data || []} loading={attachments.isLoading} failed={attachments.isError} t={t}/>
    {analysis.isError && <div className="mt-6"><ErrorState error={analysis.error}/></div>}
  </div>
}

function Detail({ icon: Icon, label, value }: { icon: typeof MapPin; label: string; value: string }) { return <div className="flex gap-3"><Icon size={18} className="mt-0.5 text-brand"/><div><dt className="text-xs font-bold uppercase tracking-wide text-muted">{label}</dt><dd className="mt-1 text-sm font-medium">{value}</dd></div></div> }

function SiteEvidence({ siteID, attachments, loading, failed, t }: { siteID: string; attachments: SiteAttachment[]; loading: boolean; failed: boolean; t: (key: string) => string }) {
  if (loading) return <section className="mt-6 rounded-xl border border-line bg-white p-6 shadow-panel"><p className="text-sm text-muted">{t('Loading site photos and documents…')}</p></section>
  if (failed) return <section className="mt-6 rounded-xl border border-red-100 bg-red-50 p-6 text-sm text-red-700">{t('Unable to load the site photos and documents.')}</section>
  return <section className="mt-6 rounded-xl border border-line bg-white p-6 shadow-panel"><div className="flex flex-wrap items-start justify-between gap-3"><div><h2 className="section-title">{t('Customer photos and documents')}</h2><p className="mt-1 text-sm text-muted">{t('When analysis starts, the system sends site photos, map images, and attached land-deed or PDF documents to Gemini for preliminary screening only.')}</p></div><span className="rounded-full bg-slate-100 px-3 py-1 text-xs font-bold text-slate-600">{attachments.length} {t('files')}</span></div>{attachments.length ? <div className="mt-5 grid gap-4 sm:grid-cols-2 xl:grid-cols-3">{attachments.map(attachment => <EvidenceCard key={attachment.id} siteID={siteID} attachment={attachment}/>)}</div> : <div className="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 p-5 text-sm text-muted">{t('The customer has not attached photos or documents yet.')}</div>}</section>
}

function EvidenceCard({ siteID, attachment }: { siteID: string; attachment: SiteAttachment }) {
  const { t } = useI18n()
  const file = useQuery({ queryKey: ['site-image-file', siteID, attachment.id], queryFn: () => api.downloadSiteImage(siteID, attachment.id) })
  const [objectURL, setObjectURL] = useState<string>()
  useEffect(() => {
    if (!file.data) return
    const url = URL.createObjectURL(file.data)
    setObjectURL(url)
    return () => URL.revokeObjectURL(url)
  }, [file.data])
  const isPDF = attachment.mimeType === 'application/pdf'
  const label = t(isPDF ? 'PDF document' : 'Site photo')
  return <article className="overflow-hidden rounded-xl border border-line bg-slate-50"><div className="flex aspect-[4/3] items-center justify-center bg-white">{file.isLoading ? <span className="text-sm text-muted">{t('Loading…')}</span> : objectURL && !isPDF ? <a href={objectURL} target="_blank" rel="noreferrer" className="h-full w-full"><img src={objectURL} alt={label} className="h-full w-full object-cover"/></a> : objectURL && isPDF ? <a href={objectURL} target="_blank" rel="noreferrer" className="grid h-full w-full place-items-center text-red-600 hover:bg-red-50"><FileText size={42}/></a> : <span className="text-sm text-red-600">{t('Unable to load the file.')}</span>}</div><div className="flex items-center gap-2 p-3 text-sm"><span className={isPDF ? 'text-red-600' : 'text-emerald-700'}>{isPDF ? <FileText size={17}/> : <FileImage size={17}/>}</span><div className="min-w-0"><p className="font-bold text-ink">{label}</p><p className="text-xs text-muted">{formatFileSize(attachment.sizeBytes)}</p></div></div></article>
}

function formatFileSize(bytes: number) { return bytes >= 1024 * 1024 ? `${(bytes / (1024 * 1024)).toFixed(1)} MB` : `${Math.max(1, Math.round(bytes / 1024))} KB` }
