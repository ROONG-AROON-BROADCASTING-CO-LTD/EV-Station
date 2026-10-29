import { ChevronLeft, ChevronRight, ExternalLink, Images, MapPin, Phone, Smartphone, Store, X } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { PublicHeader } from '../components/PublicHeader'
import { useI18n } from '../i18n/I18nProvider'

type BranchPhoto = { src: string; altTh: string; altEn: string }
type Branch = { id: string; format: 'S' | 'M' | 'L'; planned?: boolean; nameTh: string; nameEn: string; addressTh: string; addressEn: string; phone: string; map: string; photos?: BranchPhoto[] }

// Confirmed branch services, kept separate from investment estimates.
const branchServices = {
  S: [
    { th: 'EV Charging 1 สถานี', en: '1 EV charging station' },
    { th: 'ร้านกาแฟ Super Black Coffee', en: 'Super Black Coffee café' },
  ],
  M: [
    { th: 'EV Charging 2 สถานี', en: '2 EV charging stations' },
    { th: 'ร้านกาแฟ Super Black Coffee', en: 'Super Black Coffee café' },
    { th: 'ร้านอาหารและเบเกอรี่', en: 'Restaurant & bakery' },
    { th: 'ไปรษณีย์ BPOST65 EXPRESS', en: 'BPOST65 EXPRESS postal services' },
    { th: 'พื้นที่จัดงาน ประชุม และกิจกรรม', en: 'Event, meeting and activity space' },
  ],
  L: [
    { th: 'EV Charging 4 สถานี', en: '4 EV charging stations' },
    { th: 'ร้านกาแฟ Super Black Coffee', en: 'Super Black Coffee café' },
    { th: 'ร้านอาหารและเบเกอรี่', en: 'Restaurant & bakery' },
    { th: 'ไปรษณีย์ BPOST65 EXPRESS', en: 'BPOST65 EXPRESS postal services' },
    { th: 'Co-working / พื้นที่ทำงาน', en: 'Co-working / work zone' },
    { th: 'Mobile Café / ฟู้ดทรัก', en: 'Mobile Café / food truck' },
    { th: 'พื้นที่จัดงาน ประชุม และกิจกรรม', en: 'Event, meeting and activity space' },
  ],
}

function photoSources(src: string) {
  // Original customer-supplied PNG covers have no generated width variants.
  if (!src.endsWith('.webp')) return undefined
  if (src.startsWith('/pracha-uthit-ev-plan-')) return `${src} ${src.includes('-front.') ? 361 : 653}w`
  const width = src.includes('phitsanulok-storefront') ? 941
    : src.includes('phitsanulok') ? 1448
    : src.includes('charger-closeup') ? 1152 : 1600
  return [
    `${src.replace('.webp', '-480.webp')} 480w`,
    ...(width > 960 ? [`${src.replace('.webp', '-960.webp')} 960w`] : []),
    `${src} ${width}w`,
  ].join(', ')
}

const branches: Branch[] = [
  {
    id: 'phitsanulok', format: 'M', nameTh: 'สาขาพิษณุโลก',
    nameEn: 'Phitsanulok branch',
    addressTh: '654/18 ถนนพระองค์ขาว ซอย 4 ตำบลในเมือง อำเภอเมืองพิษณุโลก จังหวัดพิษณุโลก 65000',
    addressEn: '654/18 Phra Ong Khao Road, Soi 4, Nai Mueang, Mueang Phitsanulok, Phitsanulok 65000',
    phone: '080-174-7757',
    map: 'https://www.google.com/maps/search/?api=1&query=SuperBlack+Coffee+%E0%B8%AA%E0%B8%B2%E0%B8%82%E0%B8%B2+%E0%B8%9E%E0%B8%B4%E0%B8%A9%E0%B8%93%E0%B8%B8%E0%B9%82%E0%B8%A5%E0%B8%81',
    photos: [
      { src: '/super-black-coffee-phitsanulok-album-cover.png', altTh: 'Super Black Coffee สาขาพิษณุโลก ภาพปกอัลบั้ม', altEn: 'Super Black Coffee Phitsanulok album cover' },
      { src: '/super-black-coffee-phitsanulok-storefront.webp', altTh: 'หน้าร้าน Super Black Coffee สาขาพิษณุโลก', altEn: 'Super Black Coffee Phitsanulok storefront at dusk' },
      { src: '/super-black-coffee-phitsanulok-chargers.webp', altTh: 'จุดชาร์จ EV Super Black Coffee สาขาพิษณุโลก', altEn: 'EV charging bays at Super Black Coffee Phitsanulok' },
      { src: '/super-black-coffee-phitsanulok-front-angle.webp', altTh: 'หน้าร้าน Super Black Coffee สาขาพิษณุโลก มุมเฉียง', altEn: 'Angled view of Super Black Coffee Phitsanulok' },
      { src: '/super-black-coffee-phitsanulok-front.webp', altTh: 'หน้าร้าน Super Black Coffee สาขาพิษณุโลก ด้านหน้า', altEn: 'Front view of Super Black Coffee Phitsanulok' },
    ],
  },
  {
    id: 'ayutthaya', format: 'S', nameTh: 'สาขาอยุธยา',
    nameEn: 'Ayutthaya branch',
    addressTh: '15/78 หมู่ 3 ถนนป่าโมก ตำบลท่าวาสุกรี อำเภอพระนครศรีอยุธยา จังหวัดพระนครศรีอยุธยา 13000',
    addressEn: '15/78 Moo 3, Pa Mok Road, Tha Wasukri, Phra Nakhon Si Ayutthaya, Phra Nakhon Si Ayutthaya 13000',
    phone: '061-884-9960',
    map: 'https://www.google.com/maps/search/?api=1&query=SuperBlackCoffee+%E0%B8%AA%E0%B8%B2%E0%B8%82%E0%B8%B2+%E0%B8%AD%E0%B8%A2%E0%B8%B8%E0%B8%98%E0%B8%A2%E0%B8%B2',
    photos: [
      { src: '/super-black-coffee-ayutthaya-album-cover.png', altTh: 'Super Black Coffee สาขาอยุธยา ภาพปกอัลบั้ม', altEn: 'Super Black Coffee Ayutthaya album cover' },
      { src: '/super-black-coffee-ayutthaya-storefront.webp', altTh: 'หน้าร้าน Super Black Coffee สาขาอยุธยา', altEn: 'Super Black Coffee Ayutthaya storefront' },
      { src: '/super-black-coffee-ayutthaya-charger.webp', altTh: 'จุดชาร์จ EV หน้า Super Black Coffee สาขาอยุธยา', altEn: 'EV charger outside Super Black Coffee Ayutthaya' },
      { src: '/super-black-coffee-ayutthaya-charger-closeup.webp', altTh: 'เครื่องชาร์จ iGreen+ สาขาอยุธยา', altEn: 'iGreen+ charger at the Ayutthaya branch' },
    ],
  },
  { id: 'pracha-uthit', format: 'L', planned: true, nameTh: 'สาขาประชาอุทิศ', nameEn: 'Pracha Uthit branch', addressTh: '1060 ซอยประชาอุทิศ 22 แขวงห้วยขวาง เขตห้วยขวาง กรุงเทพมหานคร 10310', addressEn: '1060 Soi Pracha Uthit 22, Huai Khwang, Huai Khwang, Bangkok 10310', phone: '081-825-3584', map: 'https://www.google.com/maps/search/?api=1&query=1060+Soi+Pracha+Uthit+22+Huai+Khwang+Bangkok+10310', photos: [
    { src: '/super-black-coffee-pracha-uthit-album-cover.png', altTh: 'ภาพแนวคิด Super Black Coffee สาขาประชาอุทิศ ยังไม่ได้ก่อสร้าง', altEn: 'Super Black Coffee Pracha Uthit concept — not yet built' },
    { src: '/pracha-uthit-ev-plan-perspective.webp', altTh: 'ภาพแบบสถานีชาร์จประชาอุทิศ มุมเฉียง ยังไม่ได้ก่อสร้าง', altEn: 'Pracha Uthit EV station design, perspective view — not yet built' },
    { src: '/pracha-uthit-ev-plan-isometric.webp', altTh: 'ภาพแบบสถานีชาร์จประชาอุทิศ มุมสูง ยังไม่ได้ก่อสร้าง', altEn: 'Pracha Uthit EV station design, isometric view — not yet built' },
    { src: '/pracha-uthit-ev-plan-front.webp', altTh: 'ภาพแบบสถานีชาร์จประชาอุทิศ ด้านหน้า ยังไม่ได้ก่อสร้าง', altEn: 'Pracha Uthit EV station design, front view — not yet built' },
    { src: '/pracha-uthit-ev-plan-layout.webp', altTh: 'ภาพแบบสถานีชาร์จประชาอุทิศ ผัง 4 ช่องจอด ยังไม่ได้ก่อสร้าง', altEn: 'Pracha Uthit EV station design, four-bay layout — not yet built' },
  ] },
]

const formatOrder = { S: 0, M: 1, L: 2 }
const orderedBranches = [...branches].sort((a, b) => formatOrder[a.format] - formatOrder[b.format])

export function BranchesPage({ authenticated = false, accountName, onLogout }: { authenticated?: boolean; accountName?: string; onLogout?: () => void }) {
  const { language } = useI18n()
  const text = (th: string, en: string) => language === 'th' ? th : en
  const [searchParams, setSearchParams] = useSearchParams()
  const galleryBranch = branches.find(branch => branch.id === searchParams.get('album')) ?? null
  const galleryPhotos = galleryBranch?.photos ?? []
  const galleryRef = useRef<HTMLDivElement>(null)
  const touchStart = useRef<{ x: number; y: number } | null>(null)
  const [activePhoto, setActivePhoto] = useState(0)
  const closeGallery = useCallback(() => {
    setSearchParams(previous => { const next = new URLSearchParams(previous); next.delete('album'); return next }, { replace: true })
    setActivePhoto(0)
  }, [setSearchParams])
  const openGallery = useCallback((branch: Branch) => {
    setActivePhoto(0)
    setSearchParams(previous => { const next = new URLSearchParams(previous); next.set('album', branch.id); return next })
  }, [setSearchParams])
  const galleryOpen = galleryBranch !== null
  const stepPhoto = useCallback((direction: number) => setActivePhoto(index => galleryPhotos.length ? (index + direction + galleryPhotos.length) % galleryPhotos.length : 0), [galleryPhotos.length])

  useEffect(() => {
    if (!galleryOpen) return
    const opener = document.activeElement as HTMLElement | null
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    galleryRef.current?.querySelector<HTMLButtonElement>('.public-gallery-close')?.focus()
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') closeGallery()
      if (event.key === 'ArrowLeft') { event.preventDefault(); stepPhoto(-1) }
      if (event.key === 'ArrowRight') { event.preventDefault(); stepPhoto(1) }
      if (event.key === 'Tab') {
        const buttons = galleryRef.current?.querySelectorAll<HTMLButtonElement>('button')
        if (!buttons?.length) return
        const first = buttons[0], last = buttons[buttons.length - 1]
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => { window.removeEventListener('keydown', onKeyDown); document.body.style.overflow = previousOverflow; opener?.focus() }
  }, [galleryOpen, closeGallery, stepPhoto])

  return (
    <main className="landing-page public-site">
      <PublicHeader authenticated={authenticated} accountName={accountName} onLogout={onLogout} />
      <section className="public-branches">
        <figure className="public-branches-visual">
          <img src="/super-black-coffee-phitsanulok-front-angle.webp" srcSet={photoSources('/super-black-coffee-phitsanulok-front-angle.webp')} sizes="(max-width: 820px) 100vw, 50vw" decoding="async" alt={text('หน้าร้าน Super Black Coffee สาขาพิษณุโลก พร้อมพื้นที่จอด EV', 'Super Black Coffee Phitsanulok with EV parking')} />
          <figcaption>Super Black Coffee · {text('สาขาพิษณุโลก', 'Phitsanulok branch')}</figcaption>
        </figure>
        <div className="public-branches-copy">
          <div className="public-section-title">
            <h2>{text('สาขาและผลงาน', 'Locations & Projects')}</h2>
            <p>{text('Super Black Coffee คือพื้นที่พักและบริการของ RBC Group ที่เชื่อมประสบการณ์การเดินทางเข้ากับ EV Station', 'Super Black Coffee is an RBC Group service destination that connects the travel experience with EV Station.')}</p>
          </div>
          <div className="public-branch-list">
            {orderedBranches.map(branch => (
              <article key={branch.nameEn}>
                <Store size={20} aria-hidden="true" />
                <div className="public-branch-content">
                  <div className="public-branch-details">
                    <div className="public-branch-heading">
                      <strong>Super Black Coffee</strong>
                      <div className="public-branch-format-row"><span>{text(branch.nameTh, branch.nameEn)}</span><span className="public-branch-format">{text('ขนาด', 'Size')} {branch.format}</span></div>
                    </div>
                    {branch.planned ? <p className="public-branch-plan-note">{text('แบบโครงการสถานีชาร์จ — ยังไม่ได้ก่อสร้าง', 'EV station design — not yet built')}<small>{text('ตาม Pre Survey: DC 120 kW จำนวน 2 ตู้ และ 4 ช่องจอด', 'Pre Survey design: two DC 120 kW chargers and four parking bays')}</small></p> : null}
                    <div className="public-branch-services">
                      <h3>{branch.planned ? text('บริการตามแผนโครงการ', 'Planned project services') : text('บริการในสาขานี้', 'Services at this location')}</h3>
                      <ul>{branchServices[branch.format].map((service, index) => <li key={service.en}>{branch.planned && index === 0 ? text('สถานีชาร์จ DC 120 kW จำนวน 2 ตู้ (ตามแบบ)', 'Two DC 120 kW chargers (as designed)') : text(service.th, service.en)}</li>)}</ul>
                    </div>
                    <small><MapPin size={15} aria-hidden="true" />{text(branch.addressTh, branch.addressEn)}</small>
                    <small><Phone size={15} aria-hidden="true" /><a href={'tel:' + branch.phone.replace(/-/g, '')}>{branch.phone}</a></small>
                    <a className="public-branch-map" href={branch.map} target="_blank" rel="noreferrer">
                      {text('เปิดใน Google Maps', 'Open in Google Maps')}<ExternalLink size={14} aria-hidden="true" />
                    </a>
                  </div>
                  {branch.photos?.length ? (
                    <div className="public-branch-album-wrap">
                      <button type="button" className="public-branch-album" data-kind={branch.planned ? 'design' : 'photos'} onClick={() => openGallery(branch)} aria-haspopup="dialog" aria-label={text('เปิดอัลบั้ม ' + branch.nameTh, 'Open album: ' + branch.nameEn)}>
                        <img src={branch.photos[0].src} srcSet={photoSources(branch.photos[0].src)} sizes="(max-width: 820px) 100vw, 50vw" alt={text(branch.photos[0].altTh, branch.photos[0].altEn)} loading="lazy" decoding="async"/>
                        <span className="public-branch-album-footer"><span><strong>{branch.planned ? text('อัลบั้มแบบโครงการ', 'Project design album') : text('อัลบั้มภาพสาขา', 'Location photo album')}</strong><small>{branch.photos.length} {text('รูป · กดเพื่อดูอัลบั้ม', 'photos · Open album')}</small></span><Images size={24} aria-hidden="true"/></span>
                      </button>
                      {branch.planned ? <p className="public-branch-plan-source">{text('ภาพแบบ 3D จาก Pre Survey — Superblack Coffee Ratchada, Version 1.3 หน้า 4 โดย CS ENERGY · ยังไม่ได้ก่อสร้าง', '3D designs from Pre Survey — Superblack Coffee Ratchada, Version 1.3, page 4, by CS ENERGY · Not yet built')}</p> : null}
                    </div>
                  ) : null}
                </div>
              </article>
            ))}
          </div>
          <div className="public-branch-points">
            <div><Smartphone size={21} /><span>RBC EV Station<small>{text('ติดตามการวิเคราะห์และขั้นตอนโครงการในระบบเดียว', 'Track analysis and project steps in one system.')}</small></span></div>
          </div>
        </div>
      </section>
      {galleryOpen && galleryPhotos[activePhoto] ? (
        <div ref={galleryRef} className="public-gallery-modal" role="dialog" aria-modal="true" aria-labelledby="branch-album-title" onClick={closeGallery}>
          <div className="public-gallery-title" id="branch-album-title">Super Black Coffee · {galleryBranch ? text(galleryBranch.nameTh, galleryBranch.nameEn) : ''}<small>{galleryBranch?.planned ? text('แบบโครงการ — ยังไม่ได้ก่อสร้าง', 'Project design — not yet built') : text('อัลบั้มภาพสาขา', 'Location photo album')}</small></div>
          <button className="public-gallery-close" type="button" aria-label={text('ปิด', 'Close')} onClick={closeGallery}><X size={25} /></button>
          <button className="public-gallery-arrow previous" type="button" aria-label={text('ภาพก่อนหน้า', 'Previous image')} onClick={event => { event.stopPropagation(); stepPhoto(-1) }}><ChevronLeft size={30} /></button>
          <figure onClick={event => event.stopPropagation()} onTouchStart={event => { touchStart.current = { x: event.touches[0].clientX, y: event.touches[0].clientY } }} onTouchEnd={event => { const start = touchStart.current; touchStart.current = null; if (!start) return; const dx = event.changedTouches[0].clientX - start.x, dy = event.changedTouches[0].clientY - start.y; if (Math.abs(dx) > 50 && Math.abs(dx) > Math.abs(dy)) stepPhoto(dx < 0 ? 1 : -1) }}>
            <img src={galleryPhotos[activePhoto].src} alt={text(galleryPhotos[activePhoto].altTh, galleryPhotos[activePhoto].altEn)} />
            <figcaption aria-live="polite"><strong>{activePhoto + 1} / {galleryPhotos.length}</strong><span>{text(galleryPhotos[activePhoto].altTh, galleryPhotos[activePhoto].altEn)}</span></figcaption>
          </figure>
          <button className="public-gallery-arrow next" type="button" aria-label={text('ภาพถัดไป', 'Next image')} onClick={event => { event.stopPropagation(); stepPhoto(1) }}><ChevronRight size={30} /></button>
        </div>
      ) : null}
    </main>
  )
}
