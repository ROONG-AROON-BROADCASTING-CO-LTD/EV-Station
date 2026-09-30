import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { FileImage, FileText, Info, Upload } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate, useParams } from 'react-router-dom'
import { z } from 'zod'
import { MapPanel } from '../components/MapPanel'
import { ErrorState, LoadingState } from '../components/PageState'
import { APIError, api, errorMessageKey } from '../services/api'
import type { CreateSiteInput, LineCustomerProfile, SiteAttachment, UserRole } from '../types/domain'
import { LanguageSwitcher, useI18n } from '../i18n/I18nProvider'

const optionalNumber = z.preprocess(value => value === '' || value === undefined ? undefined : Number(value), z.number().optional())
const createSchema = (t: (key: string) => string, isCustomer: boolean) => z.object({
  name: z.string().trim().min(1, t('Site name is required')).max(160),
	contactName: z.string().trim().max(160).optional(),
	contactPhone: z.string().trim().max(40).optional(),
  address: z.string().trim().max(1000).optional(),
  latitude: optionalNumber,
  longitude: optionalNumber,
  googleMapsUrl: z.union([z.literal(''), z.url(t('Enter a valid URL'))]).optional(),
  landSize: z.preprocess(value => Number(value), z.number().positive(t('Land size must be greater than zero'))),
  landSizeUnit: z.enum(['sqm','rai','ngan','sqwah']),
	// react-hook-form turns the select into a boolean before validation. Keep an
	// already-normalised boolean intact; otherwise selecting "yes" was converted
	// back to false by the previous string-only comparison.
	internetAvailable: z.preprocess(value => {
		if (value === '' || value === undefined) return undefined
		if (typeof value === 'boolean') return value
		return value === 'yes'
	}, z.boolean().optional()),
	frontageMeters: optionalNumber,
	electricalSupplyType: z.enum(['unknown', 'overhead', 'underground']).default('unknown'),
  notes: z.string().max(5000).optional(),
}).superRefine((value, ctx) => {
  const hasGoogleMapsUrl = Boolean(value.googleMapsUrl)
  const hasLat = value.latitude !== undefined
  const hasLng = value.longitude !== undefined
  if (isCustomer && !hasGoogleMapsUrl) ctx.addIssue({ code: 'custom', path: ['googleMapsUrl'], message: t('Google Maps link is required.') })
  if (!hasGoogleMapsUrl && !(hasLat && hasLng)) ctx.addIssue({ code: 'custom', path: ['googleMapsUrl'], message: t('Provide a Google Maps link or latitude and longitude.') })
  if (hasLat !== hasLng) ctx.addIssue({ code: 'custom', path: [hasLat ? 'longitude' : 'latitude'], message: t('Both coordinates are required.') })
  if (hasLat && (value.latitude! < -90 || value.latitude! > 90)) ctx.addIssue({ code: 'custom', path: ['latitude'], message: t('Latitude must be between -90 and 90.') })
  if (hasLng && (value.longitude! < -180 || value.longitude! > 180)) ctx.addIssue({ code: 'custom', path: ['longitude'], message: t('Longitude must be between -180 and 180.') })
})
type FormValues = z.input<ReturnType<typeof createSchema>>

const maxSiteImages = 10
const maxSiteImageBytes = 10 * 1024 * 1024
const maxSitePDFBytes = 10 * 1024 * 1024
const maxSiteEvidenceBytes = 50 * 1024 * 1024
const supportedSiteEvidenceTypes = new Set(['image/jpeg', 'image/png', 'image/webp', 'application/pdf'])
const supportedSitePhotoTypes = new Set(['image/jpeg', 'image/png', 'image/webp'])

function toSiteInput(values: FormValues): CreateSiteInput {
  const numberOrUndefined = (value: unknown) => value === '' || value === undefined || Number.isNaN(Number(value)) ? undefined : Number(value)
  return { ...values, latitude: numberOrUndefined(values.latitude), longitude: numberOrUndefined(values.longitude), landSize: Number(values.landSize) } as CreateSiteInput
}

const SourceNote = () => { const { t } = useI18n(); return <span className="text-xs text-muted">{t('Source: User supplied')}</span> }

type LiffSubmission = {
  idToken: string
  profile: LineCustomerProfile | null
  onSubmitted: (notificationAccepted: boolean) => void
}

export function NewSitePage({ role, liff }: { role: UserRole; liff?: LiffSubmission }) {
  const { t, language } = useI18n()
  const isCustomer = role === 'customer'
  const schema = createSchema(t, isCustomer)
  const { id } = useParams()
  const isEditing = Boolean(id)
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [photos, setPhotos] = useState<File[]>([])
  const [documents, setDocuments] = useState<File[]>([])
  const [photoError, setPhotoError] = useState('')
  const [documentError, setDocumentError] = useState('')
  const photoInputRef = useRef<HTMLInputElement>(null)
  const documentInputRef = useRef<HTMLInputElement>(null)
  const liffRequestID = useRef(crypto.randomUUID())
  const siteQuery = useQuery({ queryKey: ['site', id], queryFn: () => api.getSite(id!), enabled: isEditing })
	const existingAttachments = useQuery({ queryKey: ['site-images', id], queryFn: () => api.listSiteImages(id!), enabled: isEditing })
	const { register, handleSubmit, watch, getValues, setValue, reset, formState: { errors } } = useForm<FormValues>({ resolver: zodResolver(schema), defaultValues: { landSizeUnit: 'sqm', address: '', googleMapsUrl: '', notes: '', electricalSupplyType: 'unknown' } })
  useEffect(() => {
    if (!siteQuery.data) return
	reset({ name: siteQuery.data.name, contactName: siteQuery.data.contactName || '', contactPhone: siteQuery.data.contactPhone || '', address: siteQuery.data.address || '', latitude: siteQuery.data.latitude, longitude: siteQuery.data.longitude, googleMapsUrl: siteQuery.data.googleMapsUrl || '', landSize: siteQuery.data.landSize, landSizeUnit: siteQuery.data.landSizeUnit, notes: siteQuery.data.notes || '', internetAvailable: siteQuery.data.internetAvailable, frontageMeters: siteQuery.data.frontageMeters, electricalSupplyType: siteQuery.data.electricalSupplyType || 'unknown' })
  }, [reset, siteQuery.data])
  useEffect(() => {
    if (!liff?.profile || isEditing) return
    setValue('contactName', liff.profile.contactName)
    setValue('contactPhone', liff.profile.contactPhone)
  }, [isEditing, liff?.profile, setValue])
  const attachments = [...photos, ...documents]
  const fileError = photoError || documentError
  const mutation = useMutation({ mutationFn: async (input: CreateSiteInput) => {
    const totalEvidenceBytes = attachments.reduce((sum, file) => sum + file.size, existingAttachments.data?.reduce((sum, item) => sum + item.sizeBytes, 0) || 0)
    if (totalEvidenceBytes > maxSiteEvidenceBytes) throw new APIError('SITE_EVIDENCE_LIMIT', 'The site evidence exceeds 50 MB.', 400)
    if (liff) {
      const saved = await api.createLiffSite(input, liff.idToken, liffRequestID.current)
		if (attachments.length) await api.uploadLiffSiteImages(saved.site.id, photos, documents, liff.idToken)
		const confirmation = await api.completeLiffSite(saved.site.id, liff.idToken)
		return { ...saved, ...confirmation }
    }
    const saved = isEditing ? await api.updateSite(id!, input) : await api.createSite(input)
    if (attachments.length) await api.uploadSiteImages(saved.id, photos, documents)
    return { site: saved, notificationAccepted: true }
  }, onSuccess: async result => {
    await queryClient.invalidateQueries({queryKey:['sites']})
    await queryClient.invalidateQueries({queryKey:['site', result.site.id]})
    if (liff) { liff.onSubmitted(result.notificationAccepted); return }
    navigate(`/sites/${result.site.id}`)
  } })
  const latitude = watch('latitude')
  const longitude = watch('longitude')
	const mapsResolution = useMutation({ mutationFn: (url: string) => liff ? api.resolveLiffGoogleMapsUrl(url, liff.idToken) : api.resolveGoogleMapsUrl(url), onSuccess: result => {
		setValue('latitude', result.latitude, { shouldValidate: true })
		setValue('longitude', result.longitude, { shouldValidate: true })
		if (!getValues('name')?.trim() && result.suggestedName) setValue('name', result.suggestedName, { shouldValidate: true })
	} })
  const numberOrUndefined = (value: unknown) => value === '' || value === undefined || Number.isNaN(Number(value)) ? undefined : Number(value)

  if (siteQuery.isLoading) return <LoadingState label={t('Loading site…')} />
  if (siteQuery.isError) return <ErrorState error={siteQuery.error} />
  return <div><div className="flex flex-wrap items-start justify-between gap-4"><div><h1 className="page-title">{t(isEditing ? 'Edit Site' : 'New Site')}</h1><p className="mt-2 text-sm text-muted">{t(isEditing ? 'Update the customer-supplied details for this site.' : 'Record customer-supplied site details before verification.')}</p></div><div className="flex w-full items-center justify-between gap-3 sm:w-auto sm:flex-col sm:items-end"><LanguageSwitcher/><div className="inline-flex items-center gap-2 text-sm text-muted"><Info size={16}/>{t('Saved inputs are not yet verified.')}</div></div></div>
    <form onSubmit={handleSubmit(values => mutation.mutate(toSiteInput(values)))} className="mt-8 grid gap-6 pb-24 xl:grid-cols-[minmax(0,0.95fr)_minmax(460px,1.05fr)]">
      <section className="rounded-xl border border-line bg-white p-6 shadow-panel"><h2 className="section-title">{t('Location')}</h2><div className="mt-5 space-y-5">
		{liff ? <><input type="hidden" {...register('contactName')} /><input type="hidden" {...register('contactPhone')} /></> : null}
        <label className="field"><span>{t('Project or location name *')}</span><input {...register('name')} placeholder={t('e.g. Bang Na Candidate')}/><p className="field-hint">{t('After the map link is confirmed, the system suggests a name from the nearest road and area. You can edit it anytime.')}</p><div className="field-meta"><FieldError message={errors.name?.message}/><SourceNote /></div></label>
        {!isCustomer && <div className="grid gap-4 sm:grid-cols-2"><label className="field"><span>{t('Latitude')}</span><input type="number" step="any" {...register('latitude')} placeholder="13.7563"/><FieldError message={errors.latitude?.message}/></label><label className="field"><span>{t('Longitude')}</span><input type="number" step="any" {...register('longitude')} placeholder="100.5018"/><FieldError message={errors.longitude?.message}/></label></div>}
        <label className="field"><span>{t('Google Maps URL')}</span><input type="url" {...register('googleMapsUrl', { onBlur: event => { const value = event.target.value.trim(); if (value) mapsResolution.mutate(value) } })} placeholder="https://maps.google.com/…"/><p className="field-hint">{t('Paste a Google Maps link and the coordinates will be filled automatically.')}</p>{mapsResolution.isPending && <p className="text-xs text-muted">{t('Resolving Google Maps link…')}</p>}{mapsResolution.isSuccess && <p className="text-xs text-emerald-700">{t('Coordinates and a suggested site name are filled. Please verify the map pin before continuing.')}</p>}{mapsResolution.isError && <p className="text-xs text-amber-700">{liff ? 'บันทึกลิงก์ได้แล้ว ระบบจะส่งให้ทีมงานตรวจสอบพิกัดเพิ่มเติม' : t(errorMessageKey(mapsResolution.error))}</p>}<FieldError message={errors.googleMapsUrl?.message}/></label>
      </div><div className="my-7 border-t border-line"/><h2 className="section-title">{t('Land details')}</h2><div className="mt-5 space-y-5"><div className="grid gap-4 sm:grid-cols-2"><label className="field"><span>{t('Land size *')}</span><input type="number" step="0.01" {...register('landSize')} placeholder="2500"/><FieldError message={errors.landSize?.message}/></label><label className="field"><span>{t('Land size unit *')}</span><select {...register('landSizeUnit')}><option value="sqm">{t('Square metres')}</option><option value="rai">{t('Rai')}</option><option value="ngan">{t('Ngan')}</option><option value="sqwah">{t('Square wah')}</option></select></label></div><label className="field"><span>{t('Internet access')}</span><select {...register('internetAvailable', { setValueAs: value => value === '' ? undefined : value === 'yes' })}><option value="">{t('Not sure')}</option><option value="yes">{t('Available')}</option><option value="no">{t('Not available')}</option></select></label><p className="field-hint">{language === 'th' ? 'คำแนะนำการวางผัง: ทางเข้า–ออกควรกว้างอย่างน้อย 7 เมตร ทีมงานจะตรวจสอบในขั้นตอนออกแบบ ลูกค้าไม่ต้องกรอกข้อมูลส่วนนี้' : 'Layout guidance: the entrance/exit should be at least 7 m wide. The team verifies this during design; customers do not need to enter it.'}</p>
		{isEditing ? <ExistingEvidence siteID={id!} attachments={existingAttachments.data || []} loading={existingAttachments.isLoading} failed={existingAttachments.isError} /> : null}
		<div className="field"><span>{t('Photos of the site and surrounding area')}</span><input ref={photoInputRef} type="file" multiple accept="image/jpeg,image/png,image/webp" className="sr-only" onChange={event => { const selected = Array.from(event.target.files || []); const error = photos.length + documents.length + (existingAttachments.data?.length || 0) + selected.length > maxSiteImages ? t('You can attach up to 10 documents or photos.') : selected.find(file => !supportedSitePhotoTypes.has(file.type)) ? t('Only JPEG, PNG, and WebP photos are supported.') : selected.find(file => file.size <= 0 || file.size > maxSiteImageBytes) ? t('Each document or photo must be 10 MB or smaller.') : ''; setPhotoError(error); if (!error) setPhotos(selected); event.currentTarget.value = '' }}/><button type="button" className="upload-area w-full text-left" onClick={() => photoInputRef.current?.click()}><Upload size={22}/><span><strong>{t('Choose site photos')}</strong><small>{t('Upload JPEG, PNG, or WebP photos. Include the site, entrance, and surrounding road where possible.')}</small></span></button>{photos.map(file => <span key={`photo-${file.name}-${file.size}`} className="flex items-center gap-2 text-sm text-muted"><FileImage size={16}/> {file.name}</span>)}{photoError ? <span role="alert" className="text-xs font-medium text-red-600">{photoError}</span> : null}</div>
		<div className="field"><span>{t('Supporting documents (optional)')}</span><input ref={documentInputRef} type="file" multiple accept="application/pdf,image/jpeg,image/png,image/webp" className="sr-only" onChange={event => { const selected = Array.from(event.target.files || []); const error = photos.length + documents.length + (existingAttachments.data?.length || 0) + selected.length > maxSiteImages ? t('You can attach up to 10 documents or photos.') : selected.find(file => !supportedSiteEvidenceTypes.has(file.type)) ? t('Only JPEG, PNG, WebP, and PDF files are supported.') : selected.find(file => file.size <= 0 || file.size > maxSitePDFBytes) ? t('Each document or photo must be 10 MB or smaller.') : ''; setDocumentError(error); if (!error) setDocuments(selected); event.currentTarget.value = '' }}/><button type="button" className="upload-area w-full text-left" onClick={() => documentInputRef.current?.click()}><Upload size={22}/><span><strong>{t('Choose site documents')}</strong><small>{t('Land deed, site plan, or related JPEG, PNG, WebP, or PDF files')}</small></span></button>{documents.map(file => <span key={`document-${file.name}-${file.size}`} className="flex items-center gap-2 text-sm text-muted">{file.type === 'application/pdf' ? <FileText size={16}/> : <FileImage size={16}/>} {file.name}</span>)}{documentError ? <span role="alert" className="text-xs font-medium text-red-600">{documentError}</span> : null}</div>
		<p className="text-xs text-muted">{t('Up to 10 files per site, 10 MB each, and 50 MB total.')}</p>
      </div></section>
      <section className="flex min-h-[320px] flex-col rounded-xl border border-line bg-white p-4 shadow-panel sm:min-h-[420px] xl:min-h-[540px]"><div className="px-2 pb-4"><h2 className="section-title">{t('Site location preview')}</h2><p className="mt-1 text-sm text-muted">{t('Approximate location from user-supplied coordinates.')}</p></div><MapPanel className="flex-1" latitude={numberOrUndefined(latitude)} longitude={numberOrUndefined(longitude)}/></section>
      <div className="flex items-center justify-end gap-3 xl:sticky xl:bottom-4 xl:z-20 xl:col-span-2 xl:rounded-xl xl:border xl:border-line xl:bg-white xl:px-6 xl:py-4 xl:shadow-panel">{!liff && <button type="button" className="button-secondary" onClick={() => navigate(isEditing ? `/sites/${id}` : '/')}>{t('Cancel')}</button>}<button type="submit" className="button-primary" disabled={mutation.isPending || Boolean(fileError)}>{mutation.isPending ? t('Saving…') : t(isEditing ? 'Save changes' : 'Save Site')}</button></div>
      {mutation.isError && <p className="text-right text-sm text-red-600 xl:col-span-2">{t(errorMessageKey(mutation.error))}</p>}
    </form>
  </div>
}

function FieldError({ message }: { message?: string }) { return message ? <span role="alert" className="text-xs font-medium text-red-600">{message}</span> : null }

function ExistingEvidence({ siteID, attachments, loading, failed }: { siteID: string; attachments: SiteAttachment[]; loading: boolean; failed: boolean }) {
	if (loading) return <div className="rounded-xl border border-line bg-slate-50 p-4 text-sm text-muted">กำลังโหลดรูปและเอกสารเดิม…</div>
	if (failed) return <div role="alert" className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">ไม่สามารถโหลดรูปและเอกสารเดิมได้ กรุณาลองเปิดหน้าอีกครั้ง</div>
	if (!attachments.length) return <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 p-4 text-sm text-muted">ยังไม่มีรูปหรือเอกสารแนบในรายการนี้</div>
	return <div className="rounded-xl border border-line bg-slate-50 p-4"><div className="flex items-center justify-between gap-3"><div><p className="text-sm font-bold text-ink">รูปและเอกสารเดิม</p><p className="mt-1 text-xs text-muted">ไฟล์เดิมจะยังอยู่เมื่อบันทึกการแก้ไข และสามารถกดเปิดเพื่อตรวจสอบได้</p></div><span className="rounded-full bg-white px-2.5 py-1 text-xs font-bold text-slate-600">{attachments.length} ไฟล์</span></div><div className="mt-4 grid gap-3 sm:grid-cols-2">{attachments.map(attachment => <ExistingEvidenceCard key={attachment.id} siteID={siteID} attachment={attachment} />)}</div></div>
}

function ExistingEvidenceCard({ siteID, attachment }: { siteID: string; attachment: SiteAttachment }) {
	const file = useQuery({ queryKey: ['site-image-file', siteID, attachment.id], queryFn: () => api.downloadSiteImage(siteID, attachment.id) })
	const [objectURL, setObjectURL] = useState<string>()
	useEffect(() => {
		if (!file.data) return
		const url = URL.createObjectURL(file.data)
		setObjectURL(url)
		return () => URL.revokeObjectURL(url)
	}, [file.data])
	const isPDF = attachment.mimeType === 'application/pdf'
	const label = isPDF ? 'เอกสาร PDF' : 'รูปภาพพื้นที่'
	return <article className="overflow-hidden rounded-lg border border-line bg-white"><div className="flex aspect-[4/3] items-center justify-center bg-slate-50">{file.isLoading ? <span className="text-xs text-muted">กำลังโหลด…</span> : objectURL && !isPDF ? <a href={objectURL} target="_blank" rel="noreferrer" className="h-full w-full" aria-label={`เปิด${label}`}><img src={objectURL} alt={label} className="h-full w-full object-cover" /></a> : objectURL && isPDF ? <a href={objectURL} target="_blank" rel="noreferrer" className="grid h-full w-full place-items-center text-red-600 hover:bg-red-50" aria-label={`เปิด${label}`}><FileText size={38} /></a> : <span className="text-xs text-red-600">โหลดไฟล์ไม่สำเร็จ</span>}</div><div className="flex items-center gap-2 p-3"><span className={isPDF ? 'text-red-600' : 'text-emerald-700'}>{isPDF ? <FileText size={16} /> : <FileImage size={16} />}</span><div className="min-w-0"><p className="text-sm font-bold text-ink">{label}</p><p className="text-xs text-muted">{formatFileSize(attachment.sizeBytes)}</p></div></div></article>
}

function formatFileSize(bytes: number) { return bytes >= 1024 * 1024 ? `${(bytes / (1024 * 1024)).toFixed(1)} MB` : `${Math.max(1, Math.round(bytes / 1024))} KB` }
