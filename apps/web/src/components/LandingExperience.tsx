import { useRef, useState, type ReactNode } from 'react'
import { ArrowDown, ArrowUpRight, Pause, Play, Zap } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useI18n } from '../i18n/I18nProvider'
import { AmbientVideo } from './landing/AmbientVideo'
import { ScrollJourney } from './landing/ScrollJourney'
import { useMotionPreference } from './landing/useMotionPreference'
import { useScrollReveal } from './landing/useScrollReveal'
import './landing-experience.css'

export function LandingExperience({ investmentLink, afterHero }: { investmentLink: string; afterHero?: ReactNode }) {
  const { language } = useI18n()
  const t = (th: string, en: string) => language === 'th' ? th : en
  const root = useRef<HTMLDivElement>(null)
  const [destination, setDestination] = useState(0)
  const [paused, setPaused] = useState(false)
  const reduced = useMotionPreference()
  const motionEnabled = !paused && !reduced
  const destinations = [
    { name: t('อยุธยา', 'Ayutthaya'), id: 'ayutthaya', image: '/rbc-motion/ayutthaya.webp', size: 'S', detail: t('สถานีชาร์จและ Super Black Coffee', 'EV charging and Super Black Coffee') },
    { name: t('พิษณุโลก', 'Phitsanulok'), id: 'phitsanulok', image: '/rbc-motion/phitsanulok.webp', size: 'M', detail: t('สถานีชาร์จ คาเฟ่ และพื้นที่บริการ', 'EV charging, a café, and destination services') },
    { name: t('ประชาอุทิศ', 'Pracha Uthit'), id: 'pracha-uthit', image: '/rbc-motion/pracha-uthit.webp', size: 'L', detail: t('แนวคิดโครงการ • ยังไม่ได้ก่อสร้าง', 'Project concept · Not yet built') },
  ]

  useScrollReveal(root, motionEnabled)

  const goToStory = () => document.getElementById('rbc-story')?.scrollIntoView({ behavior: motionEnabled ? 'smooth' : 'instant', block: 'start' })

  return <div ref={root} className={`rbc-experience${motionEnabled ? '' : ' motion-paused'}`}>
    <section className="rbc-cinema" aria-labelledby="landing-title">
      <AmbientVideo enabled={motionEnabled}/>
      <div className="rbc-cinema-shade"/>
      <div className="rbc-cinema-content">
        <h1 id="landing-title"><span>{t('เติมพลังให้การเดินทาง', 'Power the journey.')}</span><span>{t('เติมโอกาสให้พื้นที่คุณ', 'Reimagine your space.')}</span></h1>
        <div className="rbc-cinema-bottom">
          <p>{t('เริ่มต้นสถานีชาร์จ EV กับ RBC Group\nจากทำเลของคุณ สู่จุดหมายใหม่ของการเดินทาง', 'Start your EV station with RBC Group.\nFrom your location to a new destination.')}</p>
          <Link className="rbc-motion-link" to={investmentLink}>{t('ส่งทำเลให้ประเมิน', 'Explore your site’s potential')}<ArrowUpRight/></Link>
        </div>
      </div>
      <button className="rbc-film-invitation" type="button" onClick={goToStory}>
        <span className="rbc-play-orbit"><ArrowDown size={22}/></span>
        <span>{t('ดูเส้นทางจากทำเลสู่สถานี', 'From site to station')}<small>{t('เลื่อนสำรวจ 4 ขั้นตอนของโครงการ', 'Scroll through the four project steps')}</small></span><ArrowDown size={18}/>
      </button>
      <div className="rbc-cinema-foot"><a href="#rbc-project"><ArrowDown size={16}/>{t('รู้จักโครงการ RBC', 'Discover RBC')}</a><span>{t('ภาพประกอบแนวคิด', 'Concept imagery')}</span></div>
      <div className="rbc-hero-rule" aria-hidden="true"><i/></div>
    </section>

    {afterHero}

    <section className="rbc-presenter" id="rbc-project" aria-labelledby="presenter-title">
      <div className="rbc-presenter-copy">
        <h2 id="presenter-title"><span>{t('จุดเริ่มต้นสถานีชาร์จ', 'Your EV station starts here.')}</span><span>{t('ลงทุน EV กับ RBC Group', 'Invest in EV with RBC Group.')}</span></h2>
        <p>{t('ส่งทำเลให้ทีม RBC ประเมินศักยภาพพื้นที่ ความพร้อมด้านไฟฟ้า และรูปแบบสถานีชาร์จที่เหมาะสม เพื่อประกอบการพิจารณาการลงทุน', 'Submit your site for an evidence-led review of location potential, power readiness, and the charging format that fits it.')}</p>
        <Link className="rbc-text-link" to="/investment">{t('ดูรูปแบบการลงทุน', 'Explore investment formats')}<ArrowUpRight/></Link>
      </div>
      <figure className="rbc-presenter-art">
        <picture>
          <source media="(max-width: 700px)" srcSet="/rbc-motion/presenter-hero-mobile.webp"/>
          <img src="/rbc-motion/presenter-hero.webp" width="1672" height="941" loading="lazy" alt={t('พรีเซนเตอร์ RBC พร้อมสถานีชาร์จ EV และ Super Black Coffee — ภาพประกอบแนวคิด', 'RBC presenter with an EV charging station and Super Black Coffee — concept illustration')}/>
        </picture>
        <figcaption>{t('ภาพประกอบแนวคิด', 'Concept imagery')}</figcaption>
      </figure>
      <div className="rbc-hero-rule" aria-hidden="true"><i/></div>
    </section>

    <div className="rbc-intro"><span aria-hidden="true"><Zap size={22}/></span><p>{t('พลังงานที่เชื่อม', 'Energy that connects')} <strong>{t('พื้นที่ ผู้คน และการเดินทาง', 'places, people, and possibility.')}</strong></p><a href="#rbc-story" aria-label={t('สำรวจแนวทางโครงการ', 'Explore the project journey')}><ArrowDown size={22}/></a></div>
    <ScrollJourney motionEnabled={motionEnabled} reducedMotion={reduced} investmentLink={investmentLink}/>

    <section className="rbc-destinations" aria-labelledby="destinations-title"><div className="rbc-editorial-heading"><h2 id="destinations-title">{t('ระหว่างชาร์จ', 'While you charge.')}<br/><em>{t('มีมากกว่าการรอ', 'More than a pause.')}</em></h2><Link className="rbc-text-link" to="/branches">{t('สาขาและผลงาน', 'Locations & Projects')}<ArrowUpRight/></Link></div>
      <div className="rbc-destination-stage"><div className="rbc-destination-images">{destinations.map((item, index) => <img key={item.image} className={destination === index ? 'is-selected' : ''} src={item.image} alt={destination === index ? `Super Black Coffee ${item.name} — ${t('ภาพแนวคิด', 'concept image')}` : ''} aria-hidden={destination !== index} loading="lazy" width="1200" height="675" />)}</div><div className="rbc-destination-shade"/><div className="rbc-destination-caption"><h3>Super Black Coffee<span>{destinations[destination].name}</span></h3><p>{destinations[destination].detail}</p><Link to={`/branches?album=${destinations[destination].id}`} aria-label={t('เปิดอัลบั้มสาขา', 'Open location albums')}><ArrowUpRight size={24}/></Link></div><span className="rbc-destination-note">{t('ภาพประกอบแนวคิด', 'Concept imagery')}</span></div>
      <div className="rbc-destination-selector" role="group" aria-label={t('เลือกสาขา', 'Choose location')}>{destinations.map((item,index) => <button type="button" key={item.size} aria-pressed={destination === index} onClick={() => setDestination(index)}><span>{item.size}</span>{item.name}<ArrowUpRight size={19}/></button>)}</div>
    </section>
    <div className="rbc-motion-control">{reduced ? <span>{t('ลดการเคลื่อนไหวตามอุปกรณ์', 'Reduced motion enabled')}</span> : <button type="button" onClick={() => setPaused(value => !value)} aria-pressed={paused}>{paused ? <Play size={15}/> : <Pause size={15}/>} {paused ? t('เปิดการเคลื่อนไหว', 'Enable motion') : t('พักการเคลื่อนไหว', 'Pause motion')}</button>}</div>
  </div>
}
