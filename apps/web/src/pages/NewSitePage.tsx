import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { FileImage, FileText, Info, MapPin, Search, Upload } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate, useParams } from 'react-router-dom'
import { z } from 'zod'
import { MapPanel } from '../components/MapPanel'
import { ErrorState, LoadingState } from '../components/PageState'
import { api, errorMessageKey } from '../services/api'
import type { CreateSiteInput, LineCustomerProfile, UserRole } from '../types/domain'
import { useI18n } from '../i18n/I18nProvider'

const optionalNumber = z.preprocess(value => value === '' || value === undefined ? undefined : Number(value), z.number().optional())
const createSchema = (t: (key: string) => string, isCustomer: boolean) => z.object({
  name: z.string().trim().min(1, t('Site name is required')).max(160),
	contactName: z.string().trim().min(1, t('Contact name is required')).max(160),
	contactPhone: z.string().trim().min(1, t('Phone number is required')).max(40),
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
  notes: z.string().max(5000).optional(),
}).superRefine((value, ctx) => {
  const hasAddress = Boolean(value.address)
  const hasGoogleMapsUrl = Boolean(value.googleMapsUrl)
  const hasLat = value.latitude !== undefined
  const hasLng = value.longitude !== undefined
  if (isCustomer && !hasGoogleMapsUrl) ctx.addIssue({ code: 'custom', path: ['googleMapsUrl'], message: t('Google Maps link is required.') })
  if (!hasAddress && !hasGoogleMapsUrl && !(hasLat && hasLng)) ctx.addIssue({ code: 'custom', path: ['googleMapsUrl'], message: t('Provide an address, Google Maps link, or latitude and longitude.') })
  if (hasLat !== hasLng) ctx.addIssue({ code: 'custom', path: [hasLat ? 'longitude' : 'latitude'], message: t('Both coordinates are required.') })
  if (hasLat && (value.latitude! < -90 || value.latitude! > 90)) ctx.addIssue({ code: 'custom', path: ['latitude'], message: t('Latitude must be between -90 and 90.') })
  if (hasLng && (value.longitude! < -180 || value.longitude! > 180)) ctx.addIssue({ code: 'custom', path: ['longitude'], message: t('Longitude must be between -180 and 180.') })
})
type FormValues = z.input<ReturnType<typeof createSchema>>

const maxSiteImages = 10
const maxSiteImageBytes = 10 * 1024 * 1024
const maxSitePDFBytes = 10 * 1024 * 1024
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
  const { t } = useI18n()
  const isCustomer = role === 'customer'
  const schema = createSchema(t, isCustomer)
  const { id } = useParams()
  const isEditing = Boolean(id)
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [photos, setPhotos] = useState<File[]>([])
  const [documents, setDocuments] = useState<File[]>([])
  const [fileError, setFileError] = useState('')
  const liffRequestID = useRef(crypto.randomUUID())
  const siteQuery = useQuery({ queryKey: ['site', id], queryFn: () => api.getSite(id!), enabled: isEditing })
	const { register, handleSubmit, watch, setValue, reset, formState: { errors } } = useForm<FormValues>({ resolver: zodResolver(schema), defaultValues: { landSizeUnit: 'sqm', address: '', googleMapsUrl: '', notes: '' } })
  useEffect(() => {
    if (!siteQuery.data) return
	reset({ name: siteQuery.data.name, contactName: siteQuery.data.contactName || '', contactPhone: siteQuery.data.contactPhone || '', address: siteQuery.data.address || '', latitude: siteQuery.data.latitude, longitude: siteQuery.data.longitude, googleMapsUrl: siteQuery.data.googleMapsUrl || '', landSize: siteQuery.data.landSize, landSizeUnit: siteQuery.data.landSizeUnit, notes: siteQuery.data.notes || '', internetAvailable: siteQuery.data.internetAvailable, frontageMeters: siteQuery.data.frontageMeters })
  }, [reset, siteQuery.data])
  useEffect(() => {
    if (!liff?.profile || isEditing) return
    setValue('contactName', liff.profile.contactName)
    setValue('contactPhone', liff.profile.contactPhone)
  }, [isEditing, liff?.profile, setValue])
  const attachments = [...photos, ...documents]
  const mutation = useMutation({ mutationFn: async (input: CreateSiteInput) => {
    if (liff) {
      const saved = await api.createLiffSite(input, liff.idToken, liffRequestID.current)
      if (attachments.length) await api.uploadLiffSiteImages(saved.site.id, attachments, liff.idToken)
		const confirmation = await api.completeLiffSite(saved.site.id, liff.idToken)
		return { ...saved, ...confirmation }
    }
    const saved = isEditing ? await api.updateSite(id!, input) : await api.createSite(input)
    if (attachments.length) await api.uploadSiteImages(saved.id, attachments)
    return { site: saved, notificationAccepted: true }
  }, onSuccess: async result => {
    await queryClient.invalidateQueries({queryKey:['sites']})
    await queryClient.invalidateQueries({queryKey:['site', result.site.id]})
    if (liff) { liff.onSubmitted(result.notificationAccepted); return }
    navigate(`/sites/${result.site.id}`)
  } })
  const latitude = watch('latitude')
  const longitude = watch('longitude')
  const address = watch('address')
  const geocoding = useMutation({ mutationFn: () => api.searchAddress(String(address || '')) })
  const mapsResolution = useMutation({ mutationFn: (url: string) => liff ? api.resolveLiffGoogleMapsUrl(url, liff.idToken) : api.resolveGoogleMapsUrl(url), onSuccess: result => { setValue('latitude', result.latitude, { shouldValidate: true }); setValue('longitude', result.longitude, { shouldValidate: true }) } })
  const numberOrUndefined = (value: unknown) => value === '' || value === undefined || Number.isNaN(Number(value)) ? undefined : Number(value)

  if (siteQuery.isLoading) return <LoadingState label={t('Loading site…')} />
  if (siteQuery.isError) return <ErrorState error={siteQuery.error} />
  return <div><div className="flex flex-wrap items-start justify-between gap-4"><div><h1 className="page-title">{t(isEditing ? 'Edit Site' : 'New Site')}</h1><p className="mt-2 text-sm text-muted">{t(isEditing ? 'Update the customer-supplied details for this site.' : 'Record customer-supplied site details before verification.')}</p></div><div className="inline-flex items-center gap-2 text-sm text-muted"><Info size={16}/>{t('Saved inputs are not yet verified.')}</div></div>
    <form onSubmit={handleSubmit(values => mutation.mutate(toSiteInput(values)))} className="mt-8 grid gap-6 pb-24 xl:grid-cols-[minmax(0,0.95fr)_minmax(460px,1.05fr)]">
      <section className="rounded-xl border border-line bg-white p-6 shadow-panel"><h2 className="section-title">{t('Location')}</h2><div className="mt-5 space-y-5">
		<div className="grid gap-4 sm:grid-cols-2"><label className="field"><span>{t('Contact name *')}</span><input {...register('contactName')} /><FieldError message={errors.contactName?.message}/></label><label className="field"><span>{t('Phone number *')}</span><input inputMode="tel" {...register('contactPhone')} /><FieldError message={errors.contactPhone?.message}/></label></div>
        {liff?.profile ? <p className="rounded-lg bg-emerald-50 px-3 py-2 text-xs text-emerald-800">ใช้ข้อมูลติดต่อที่บันทึกไว้จากบัญชี LINE ของคุณ แก้ไขได้หากข้อมูลเปลี่ยน</p> : null}
        <label className="field"><span>{t('Project or location name *')}</span><input {...register('name')} placeholder={t('e.g. Bang Na Candidate')}/><div className="field-meta"><FieldError message={errors.name?.message}/><SourceNote /></div></label>
        {!isCustomer && <><div><label className="field"><span>{t('Address')}</span><input {...register('address')} placeholder={t('Enter a street address or place')}/><p className="field-hint">{t('Provide an Address OR Latitude + Longitude.')}</p><div className="field-meta"><FieldError message={errors.address?.message}/><SourceNote /></div></label><button type="button" className="button-secondary mt-2" onClick={() => geocoding.mutate()} disabled={geocoding.isPending || String(address || '').trim().length < 3}><Search size={16}/>{geocoding.isPending ? t('Searching…') : t('Search free map data')}</button><p className="mt-2 text-xs leading-5 text-muted">{t('The address is sent to OpenStreetMap Nominatim only when you press search. Results are preliminary and must be confirmed.')}</p></div>
        {geocoding.data ? <div className="rounded-lg border border-line bg-slate-50 p-3"><p className="text-xs font-bold uppercase tracking-wide text-muted">{t('Address matches')}</p>{geocoding.data.length ? <div className="mt-2 space-y-2">{geocoding.data.map(result => <button key={`${result.latitude}-${result.longitude}`} type="button" className="flex w-full items-start gap-2 rounded-lg bg-white p-3 text-left text-sm shadow-sm hover:ring-2 hover:ring-emerald-100" onClick={() => { setValue('latitude', result.latitude, { shouldValidate: true }); setValue('longitude', result.longitude, { shouldValidate: true }) }}><MapPin size={17} className="mt-0.5 shrink-0 text-brand"/><span><strong className="block text-ink">{result.displayName}</strong><small className="mt-1 block text-muted">{result.latitude.toFixed(6)}, {result.longitude.toFixed(6)} · {t('Preliminary match')}</small></span></button>)}</div> : <p className="mt-2 text-sm text-muted">{t('No address matches found.')}</p>}</div> : null}
        {geocoding.isError ? <p className="text-sm text-red-600">{t(errorMessageKey(geocoding.error))}</p> : null}
        <div className="grid gap-4 sm:grid-cols-2"><label className="field"><span>{t('Latitude')}</span><input type="number" step="any" {...register('latitude')} placeholder="13.7563"/><FieldError message={errors.latitude?.message}/></label><label className="field"><span>{t('Longitude')}</span><input type="number" step="any" {...register('longitude')} placeholder="100.5018"/><FieldError message={errors.longitude?.message}/></label></div></>}
        <label className="field"><span>{t('Google Maps URL')}</span><input type="url" {...register('googleMapsUrl', { onBlur: event => { const value = event.target.value.trim(); const coordinates = extractGoogleMapsCoordinates(value); if (coordinates) { setValue('latitude', coordinates.latitude, { shouldValidate: true }); setValue('longitude', coordinates.longitude, { shouldValidate: true }) } else if (value) mapsResolution.mutate(value) } })} placeholder="https://maps.google.com/…"/><p className="field-hint">{t('Paste a Google Maps link and the coordinates will be filled automatically.')}</p>{mapsResolution.isPending && <p className="text-xs text-muted">{t('Resolving Google Maps link…')}</p>}{mapsResolution.isSuccess && <p className="text-xs text-emerald-700">{t('Coordinates filled. Please verify the map pin before continuing.')}</p>}{mapsResolution.isError && <p className="text-xs text-amber-700">{liff ? 'บันทึกลิงก์ได้แล้ว ระบบจะส่งให้ทีมงานตรวจสอบพิกัดเพิ่มเติม' : t(errorMessageKey(mapsResolution.error))}</p>}<FieldError message={errors.googleMapsUrl?.message}/></label>
      </div><div className="my-7 border-t border-line"/><h2 className="section-title">{t('Land details')}</h2><div className="mt-5 space-y-5"><div className="grid gap-4 sm:grid-cols-2"><label className="field"><span>{t('Land size *')}</span><input type="number" step="0.01" {...register('landSize')} placeholder="2500"/><FieldError message={errors.landSize?.message}/></label><label className="field"><span>{t('Land size unit *')}</span><select {...register('landSizeUnit')}><option value="sqm">{t('Square metres')}</option><option value="rai">{t('Rai')}</option><option value="ngan">{t('Ngan')}</option><option value="sqwah">{t('Square wah')}</option></select></label></div><div className="grid gap-4 sm:grid-cols-2"><label className="field"><span>{t('Internet access')}</span><select {...register('internetAvailable', { setValueAs: value => value === '' ? undefined : value === 'yes' })}><option value="">{t('Not sure')}</option><option value="yes">{t('Available')}</option><option value="no">{t('Not available')}</option></select></label><label className="field"><span>{t('Entrance width (metres)')}</span><input type="number" step="0.1" min="0" {...register('frontageMeters')} placeholder={t('Unknown')}/></label></div>
		<label className="field"><span>{t('Photos of the site and surrounding area')}</span><span className="upload-area"><Upload size={22}/><span><strong>{t('Choose site photos')}</strong><small>{t('Attach up to 10 documents or photos. Include the site, entrance and surrounding road where possible.')}</small></span><input type="file" multiple accept="image/jpeg,image/png,image/webp" className="sr-only" onChange={event => { const selected = Array.from(event.target.files || []); const error = photos.length + documents.length + selected.length > maxSiteImages ? t('You can attach up to 10 documents or photos.') : selected.find(file => !supportedSitePhotoTypes.has(file.type)) ? t('Only JPEG, PNG, and WebP photos are supported.') : selected.find(file => file.size <= 0 || file.size > maxSiteImageBytes) ? t('Each document or photo must be 10 MB or smaller.') : ''; setFileError(error); if (!error) setPhotos(selected); event.currentTarget.value = '' }}/></span>{photos.map(file => <span key={`photo-${file.name}-${file.size}`} className="flex items-center gap-2 text-sm text-muted"><FileImage size={16}/> {file.name}</span>)}{fileError ? <span className="text-xs font-medium text-red-600">{fileError}</span> : null}</label>
		<label className="field"><span>{t('Supporting documents (optional)')}</span><span className="upload-area"><Upload size={22}/><span><strong>{t('Choose site documents')}</strong><small>{t('Land deed, site plan, or other related documents')}</small></span><input type="file" multiple accept="application/pdf,image/jpeg,image/png,image/webp" className="sr-only" onChange={event => { const selected = Array.from(event.target.files || []); const error = photos.length + documents.length + selected.length > maxSiteImages ? t('You can attach up to 10 documents or photos.') : selected.find(file => !supportedSiteEvidenceTypes.has(file.type)) ? t('Only JPEG, PNG, WebP, and PDF files are supported.') : selected.find(file => file.size <= 0 || file.size > maxSitePDFBytes) ? t('Each document or photo must be 10 MB or smaller.') : ''; setFileError(error); if (!error) setDocuments(selected); event.currentTarget.value = '' }}/></span>{documents.map(file => <span key={`document-${file.name}-${file.size}`} className="flex items-center gap-2 text-sm text-muted">{file.type === 'application/pdf' ? <FileText size={16}/> : <FileImage size={16}/>} {file.name}</span>)}</label>
      </div></section>
      <section className="flex min-h-[540px] flex-col rounded-xl border border-line bg-white p-4 shadow-panel"><div className="px-2 pb-4"><h2 className="section-title">{t('Site location preview')}</h2><p className="mt-1 text-sm text-muted">{t('Approximate location from user-supplied coordinates.')}</p></div><MapPanel className="flex-1" latitude={numberOrUndefined(latitude)} longitude={numberOrUndefined(longitude)}/></section>
      <div className={`flex items-center justify-end gap-3 xl:fixed xl:bottom-0 xl:right-0 xl:z-20 xl:border-t xl:border-line xl:bg-white xl:px-10 xl:py-4 ${liff ? 'xl:left-0' : 'xl:left-[236px]'}`}>{!liff && <button type="button" className="button-secondary" onClick={() => navigate(isEditing ? `/sites/${id}` : '/')}>{t('Cancel')}</button>}<button type="submit" className="button-primary" disabled={mutation.isPending || Boolean(fileError)}>{mutation.isPending ? t('Saving…') : t(isEditing ? 'Save changes' : 'Save Site')}</button></div>
      {mutation.isError && <p className="text-right text-sm text-red-600 xl:col-span-2">{t(errorMessageKey(mutation.error))}</p>}
    </form>
  </div>
}

function FieldError({ message }: { message?: string }) { return message ? <span className="text-xs font-medium text-red-600">{message}</span> : null }


function extractGoogleMapsCoordinates(value: string): { latitude: number; longitude: number } | undefined {
  const decoded = decodeURIComponent(value.trim())
  const candidates = decoded.match(/-?\d+(?:\.\d+)?\s*,\s*-?\d+(?:\.\d+)?/g) || []
  for (const candidate of candidates) {
    const [latitude, longitude] = candidate.split(',').map(Number)
    if (latitude >= -90 && latitude <= 90 && longitude >= -180 && longitude <= 180) return { latitude, longitude }
  }
  return undefined
}
