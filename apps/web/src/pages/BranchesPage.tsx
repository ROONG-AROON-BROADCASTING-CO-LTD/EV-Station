import { ChevronLeft, ChevronRight, ExternalLink, MapPin, Phone, Smartphone, Store, X } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { PublicHeader } from '../components/PublicHeader'
import { useI18n } from '../i18n/I18nProvider'

type BranchPhoto = { src: string; altTh: string; altEn: string }
type Branch = { nameTh: string; nameEn: string; addressTh: string; addressEn: string; phone: string; map: string; photos?: BranchPhoto[] }

const branches: Branch[] = [
  {
    nameTh: 'สาขาพิษณุโลก',
    nameEn: 'Phitsanulok branch',
    addressTh: '654/18 ถนนพระองค์ขาว ซอย 4 ตำบลในเมือง อำเภอเมืองพิษณุโลก จังหวัดพิษณุโลก 65000',
    addressEn: '654/18 Phra Ong Khao Road, Soi 4, Nai Mueang, Mueang Phitsanulok, Phitsanulok 65000',
    phone: '080-174-7757',
    map: 'https://www.google.com/maps/search/?api=1&query=SuperBlack+Coffee+%E0%B8%AA%E0%B8%B2%E0%B8%82%E0%B8%B2+%E0%B8%9E%E0%B8%B4%E0%B8%A9%E0%B8%93%E0%B8%B8%E0%B9%82%E0%B8%A5%E0%B8%81',
    photos: [
      { src: '/super-black-coffee-phitsanulok-storefront.png', altTh: 'หน้าร้าน Super Black Coffee สาขาพิษณุโลก', altEn: 'Super Black Coffee Phitsanulok storefront at dusk' },
      { src: '/super-black-coffee-phitsanulok-chargers.png', altTh: 'จุดชาร์จ EV Super Black Coffee สาขาพิษณุโลก', altEn: 'EV charging bays at Super Black Coffee Phitsanulok' },
      { src: '/super-black-coffee-phitsanulok-front-angle.png', altTh: 'หน้าร้าน Super Black Coffee สาขาพิษณุโลก มุมเฉียง', altEn: 'Angled view of Super Black Coffee Phitsanulok' },
      { src: '/super-black-coffee-phitsanulok-front.png', altTh: 'หน้าร้าน Super Black Coffee สาขาพิษณุโลก ด้านหน้า', altEn: 'Front view of Super Black Coffee Phitsanulok' },
    ],
  },
  {
    nameTh: 'สาขาอยุธยา',
    nameEn: 'Ayutthaya branch',
    addressTh: '15/78 หมู่ 3 ถนนป่าโมก ตำบลท่าวาสุกรี อำเภอพระนครศรีอยุธยา จังหวัดพระนครศรีอยุธยา 13000',
    addressEn: '15/78 Moo 3, Pa Mok Road, Tha Wasukri, Phra Nakhon Si Ayutthaya, Phra Nakhon Si Ayutthaya 13000',
    phone: '061-884-9960',
    map: 'https://www.google.com/maps/search/?api=1&query=SuperBlackCoffee+%E0%B8%AA%E0%B8%B2%E0%B8%82%E0%B8%B2+%E0%B8%AD%E0%B8%A2%E0%B8%B8%E0%B8%98%E0%B8%A2%E0%B8%B2',
    photos: [
      { src: '/super-black-coffee-ayutthaya-storefront.jpg', altTh: 'หน้าร้าน Super Black Coffee สาขาอยุธยา', altEn: 'Super Black Coffee Ayutthaya storefront' },
      { src: '/super-black-coffee-ayutthaya-charger.jpg', altTh: 'จุดชาร์จ EV หน้า Super Black Coffee สาขาอยุธยา', altEn: 'EV charger outside Super Black Coffee Ayutthaya' },
      { src: '/super-black-coffee-ayutthaya-charger-closeup.jpg', altTh: 'เครื่องชาร์จ iGreen+ สาขาอยุธยา', altEn: 'iGreen+ charger at the Ayutthaya branch' },
    ],
  },
  { nameTh: 'สาขาประชาอุทิศ', nameEn: 'Pracha Uthit branch', addressTh: '68/10 ซ.ประชาอุทิศ 22 แขวงห้วยขวาง เขตห้วยขวาง กรุงเทพมหานคร 10310', addressEn: '68/10 Soi Pracha Uthit 22, Huai Khwang, Huai Khwang, Bangkok 10310', phone: '081-825-3584', map: 'https://www.google.com/maps/search/?api=1&query=Super+Black+Coffee+%E0%B8%AA%E0%B8%B2%E0%B8%82%E0%B8%B2+%E0%B8%9B%E0%B8%A3%E0%B8%B0%E0%B8%8A%E0%B8%B2%E0%B8%AD%E0%B8%B8%E0%B8%97%E0%B8%B4%E0%B8%A8' },
]

export function BranchesPage({ authenticated = false, accountName, onLogout }: { authenticated?: boolean; accountName?: string; onLogout?: () => void }) {
  const { language } = useI18n()
  const text = (th: string, en: string) => language === 'th' ? th : en
  const [galleryPhotos, setGalleryPhotos] = useState<BranchPhoto[]>([])
  const [activePhoto, setActivePhoto] = useState<number | null>(null)
  const closeGallery = useCallback(() => { setActivePhoto(null); setGalleryPhotos([]) }, [])
  const openGallery = useCallback((photos: BranchPhoto[], index: number) => { setGalleryPhotos(photos); setActivePhoto(index) }, [])
  const stepPhoto = useCallback((direction: number) => setActivePhoto(index => index === null ? null : (index + direction + galleryPhotos.length) % galleryPhotos.length), [galleryPhotos.length])

  useEffect(() => {
    if (activePhoto === null) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') closeGallery()
      if (event.key === 'ArrowLeft') stepPhoto(-1)
      if (event.key === 'ArrowRight') stepPhoto(1)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [activePhoto, closeGallery, stepPhoto])

  return (
    <main className="landing-page public-site">
      <PublicHeader authenticated={authenticated} accountName={accountName} onLogout={onLogout} />
      <section className="public-branches">
        <figure className="public-branches-visual">
          <img src="/super-black-coffee-phitsanulok-front-angle.png" alt={text('หน้าร้าน Super Black Coffee สาขาพิษณุโลก พร้อมพื้นที่จอด EV', 'Super Black Coffee Phitsanulok with EV parking')} />
          <figcaption>Super Black Coffee · {text('สาขาพิษณุโลก', 'Phitsanulok branch')}</figcaption>
        </figure>
        <div className="public-branches-copy">
          <div className="public-section-title">
            <h2>{text('สาขาและผลงาน', 'Branches & work')}</h2>
            <p>{text('Super Black Coffee คือพื้นที่พักและบริการของ RBC Group ที่เชื่อมประสบการณ์การเดินทางเข้ากับ EV Station', 'Super Black Coffee is an RBC Group service destination that connects the travel experience with EV Station.')}</p>
          </div>
          <div className="public-branch-list">
            {branches.map(branch => (
              <article key={branch.nameEn}>
                <Store size={20} aria-hidden="true" />
                <div className="public-branch-content">
                  <div className="public-branch-details">
                    <div className="public-branch-heading">
                      <strong>Super Black Coffee</strong>
                      <span>{text(branch.nameTh, branch.nameEn)}</span>
                    </div>
                    <small><MapPin size={15} aria-hidden="true" />{text(branch.addressTh, branch.addressEn)}</small>
                    <small><Phone size={15} aria-hidden="true" /><a href={'tel:' + branch.phone.replace(/-/g, '')}>{branch.phone}</a></small>
                    <a className="public-branch-map" href={branch.map} target="_blank" rel="noreferrer">
                      {text('เปิดใน Google Maps', 'Open in Google Maps')}<ExternalLink size={14} aria-hidden="true" />
                    </a>
                  </div>
                  {branch.photos ? (
                    <div className="public-branch-gallery" data-count={branch.photos.length} aria-label={text('ภาพสถานที่จริง ' + branch.nameTh, branch.nameEn + ' gallery')}>
                      {branch.photos.map((photo, index) => (
                        <button type="button" onClick={() => openGallery(branch.photos ?? [], index)} key={photo.src} aria-label={text(photo.altTh, photo.altEn)}>
                          <img src={photo.src} alt={text(photo.altTh, photo.altEn)} loading="lazy" />
                        </button>
                      ))}
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
      {activePhoto !== null && galleryPhotos[activePhoto] ? (
        <div className="public-gallery-modal" role="dialog" aria-modal="true" aria-label={text('แกลเลอรีสาขา Super Black Coffee', 'Super Black Coffee branch gallery')} onClick={closeGallery}>
          <button className="public-gallery-close" type="button" aria-label={text('ปิด', 'Close')} onClick={closeGallery}><X size={25} /></button>
          <button className="public-gallery-arrow previous" type="button" aria-label={text('ภาพก่อนหน้า', 'Previous image')} onClick={event => { event.stopPropagation(); stepPhoto(-1) }}><ChevronLeft size={30} /></button>
          <figure onClick={event => event.stopPropagation()}>
            <img src={galleryPhotos[activePhoto].src} alt={text(galleryPhotos[activePhoto].altTh, galleryPhotos[activePhoto].altEn)} />
            <figcaption>{activePhoto + 1} / {galleryPhotos.length}</figcaption>
          </figure>
          <button className="public-gallery-arrow next" type="button" aria-label={text('ภาพถัดไป', 'Next image')} onClick={event => { event.stopPropagation(); stepPhoto(1) }}><ChevronRight size={30} /></button>
        </div>
      ) : null}
    </main>
  )
}
