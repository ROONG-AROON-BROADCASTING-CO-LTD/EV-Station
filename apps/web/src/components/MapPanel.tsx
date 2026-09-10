import { CircleF, GoogleMap, LoadScript, MarkerF } from '@react-google-maps/api'
import { MapPin, Radio } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useI18n } from '../i18n/I18nProvider'
import { api } from '../services/api'

interface MapPanelProps { latitude?: number; longitude?: number; radiusMeters?: number; className?: string }

export function PrintableLocationMap({ latitude, longitude }: Pick<MapPanelProps, 'latitude' | 'longitude'>) {
	const { t } = useI18n()
  if (latitude === undefined || longitude === undefined) return null
  const center = `${latitude.toFixed(6)},${longitude.toFixed(6)}`
  const zoom = 14
  const tileCount = 2 ** zoom
  const exactX = ((longitude + 180) / 360) * tileCount
  const latitudeRadians = latitude * Math.PI / 180
  const exactY = (1 - Math.asinh(Math.tan(latitudeRadians)) / Math.PI) / 2 * tileCount
  const centerX = Math.floor(exactX)
  const centerY = Math.floor(exactY)
  const fractionX = exactX - centerX
  const fractionY = exactY - centerY
  const tiles = [-2, -1, 0, 1].flatMap(offsetY => [-2, -1, 0, 1, 2].map(offsetX => ({
    x: (centerX + offsetX + tileCount) % tileCount,
    y: Math.min(Math.max(centerY + offsetY, 0), tileCount - 1),
    key: `${offsetX}:${offsetY}`,
  })))
  // The tile canvas is intentionally larger than the printed viewport, so the
  // customer pin remains exactly in the centre even when it falls near a tile edge.
  const canvasOffsetX = ((0.5 - (2 + fractionX) / 3) / (5 / 3)) * 100
  const canvasOffsetY = ((0.5 - (2 + fractionY) / 2) / 2) * 100
  return <figure className="print-location-map">
    <div className="print-location-map-tiles">
      <div className="print-location-map-canvas" style={{ transform: `translate(${canvasOffsetX}%, ${canvasOffsetY}%)` }}>{tiles.map(tile => <img key={tile.key} src={`https://tile.openstreetmap.org/${zoom}/${tile.x}/${tile.y}.png`} alt="" />)}</div>
	  <span className="print-location-pin" style={{ left: '50%', top: '50%' }} aria-label={`${t('Customer site location')} ${center}`}><MapPin size={38} fill="#e5484d" strokeWidth={2.5} /></span>
    </div>
	<figcaption>{t('Customer site pin')} · {t('Coordinates')} {center}</figcaption>
  </figure>
}

export function MapPanel({ latitude, longitude, radiusMeters = 3000, className = '' }: MapPanelProps) {
  const { t } = useI18n()
  const key = import.meta.env.VITE_GOOGLE_MAPS_BROWSER_API_KEY
  const hasCoordinates = latitude !== undefined && longitude !== undefined
	const countedMapLoad = useRef(false)
	useEffect(() => {
		if (!key || !hasCoordinates || countedMapLoad.current) return
		countedMapLoad.current = true
		void api.recordGoogleMapsLoad().catch(() => undefined)
	}, [key, hasCoordinates])
  if (!hasCoordinates) {
    return <div className={`relative grid min-h-72 place-items-center overflow-hidden rounded-xl border border-line bg-[#eef3f7] ${className}`}>
      <div className="absolute inset-0 opacity-45 map-grid" />
      <div className="relative max-w-sm px-8 text-center"><span className="mx-auto grid h-12 w-12 place-items-center rounded-full bg-white text-brand shadow-panel"><MapPin /></span><h3 className="mt-4 font-bold">{t('Map preview unavailable')}</h3><p className="mt-2 text-sm leading-6 text-muted">{t('Search an address or enter valid coordinates to preview this location.')}</p></div>
    </div>
  }
  if (!key) {
    const mapPadding = 1.35
    const deltaLatitude = radiusMeters * mapPadding / 111_320
    const longitudeScale = Math.max(Math.cos(latitude * Math.PI / 180), 0.2)
    const deltaLongitude = radiusMeters * mapPadding / (111_320 * longitudeScale)
    const west = longitude - deltaLongitude
    const south = latitude - deltaLatitude
    const east = longitude + deltaLongitude
    const north = latitude + deltaLatitude
    const bbox = [west, south, east, north].map(value => value.toFixed(6)).join('%2C')
    const marker = `${latitude.toFixed(6)}%2C${longitude.toFixed(6)}`
    const source = `https://www.openstreetmap.org/export/embed.html?bbox=${bbox}&layer=mapnik&marker=${marker}`
    return <div className={`overflow-hidden rounded-xl border border-line bg-white ${className}`}>
      <iframe title={t('Map preview')} src={source} className="block h-full min-h-72 w-full border-0" loading="lazy" />
      <p className="border-t border-line bg-white px-3 py-2 text-xs text-muted">© <a className="font-semibold text-brand hover:underline" href="https://www.openstreetmap.org/copyright" target="_blank" rel="noreferrer">OpenStreetMap contributors</a> · {t('Interactive road map')}</p>
    </div>
  }
  const center = { lat: latitude, lng: longitude }
  return <div className={`overflow-hidden rounded-xl border border-line ${className}`}>
    <LoadScript googleMapsApiKey={key}>
      <GoogleMap center={center} zoom={18} mapContainerStyle={{width:'100%', height:'100%', minHeight:'288px'}} options={{streetViewControl:false,mapTypeControl:false,fullscreenControl:false,mapTypeId:'satellite'}}>
        <MarkerF position={center} />
        <CircleF center={center} radius={radiusMeters} options={{strokeColor:'#079455',strokeOpacity:.85,strokeWeight:2,fillColor:'#079455',fillOpacity:.08}} />
      </GoogleMap>
    </LoadScript>
  </div>
}

export function RadiusLabel({ meters }: { meters: number }) {
  const { t } = useI18n()
  return <span className="inline-flex items-center gap-2 text-sm font-semibold text-ink"><Radio size={16} className="text-brand" />{meters / 1000} {t('km radius')}</span>
}

export function RadiusSelector({ value, onChange }: { value: number; onChange: (meters: number) => void }) {
  const { t } = useI18n()
  return <div className="inline-flex rounded-lg border border-line bg-white p-1" role="group" aria-label={t('Analysis radius')}>
    {[1000, 2000, 3000].map(meters => <button key={meters} type="button" aria-pressed={value === meters} onClick={() => onChange(meters)} className={`min-h-9 rounded-md px-3 text-sm font-bold transition ${value === meters ? 'bg-brand text-white shadow-sm' : 'text-muted hover:bg-slate-100 hover:text-ink'}`}>{meters / 1000} {t('km')}</button>)}
  </div>
}
