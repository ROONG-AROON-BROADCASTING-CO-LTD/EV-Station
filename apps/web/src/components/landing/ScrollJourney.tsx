import { useEffect, useRef, useState } from 'react'
import { ArrowDown, ArrowUpRight } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useI18n } from '../../i18n/I18nProvider'
import { workflowChapters } from './workflowChapters'
import './scroll-journey.css'

const imagePositions = ['42% center', '64% center', '58% center', '60% center']

export function ScrollJourney({ motionEnabled, reducedMotion, investmentLink }: { motionEnabled: boolean; reducedMotion: boolean; investmentLink: string }) {
  const { language } = useI18n()
  const lang = language === 'th' ? 0 : 1
  const t = (th: string, en: string) => language === 'th' ? th : en
  const section = useRef<HTMLElement>(null)
  const rows = useRef<(HTMLElement | null)[]>([])
  const [current, setCurrent] = useState(0)

  useEffect(() => {
    const element = section.current
    if (!element) return
    let frame = 0
    let visible = false
    const paint = () => {
      frame = 0
      if (document.hidden || !visible) return
      const header = document.querySelector('.landing-header')?.getBoundingClientRect().height ?? 84
      element.style.setProperty('--journey-header', `${header}px`)
      // Pick the closest row in either direction, including large wheel jumps
      // and browser scroll restoration. Work only while this section is visible.
      const focus = header + (window.innerHeight - header) / 2
      let nearest = 0
      let distance = Infinity
      rows.current.forEach((row, index) => {
        if (!row) return
        const rect = row.getBoundingClientRect()
        const delta = Math.abs(rect.top + rect.height / 2 - focus)
        if (delta < distance) { distance = delta; nearest = index }
      })
      setCurrent(previous => previous === nearest ? previous : nearest)
    }
    const schedule = () => { if (!frame && visible && !document.hidden) frame = requestAnimationFrame(paint) }
    const observer = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting
      if (!visible && frame) { cancelAnimationFrame(frame); frame = 0 }
      schedule()
    })
    observer.observe(element)
    window.addEventListener('scroll', schedule, { passive: true })
    window.addEventListener('resize', schedule)
    document.addEventListener('visibilitychange', schedule)
    return () => {
      observer.disconnect()
      cancelAnimationFrame(frame)
      window.removeEventListener('scroll', schedule)
      window.removeEventListener('resize', schedule)
      document.removeEventListener('visibilitychange', schedule)
    }
  }, [])

  const select = (index: number) => {
    setCurrent(index)
    rows.current[index]?.scrollIntoView({ behavior: motionEnabled && !reducedMotion ? 'smooth' : 'instant', block: 'center' })
  }
  const skip = () => section.current?.nextElementSibling?.scrollIntoView({ behavior: motionEnabled && !reducedMotion ? 'smooth' : 'instant', block: 'start' })

  return <section ref={section} id="rbc-story" className={`rbc-scroll-journey${!motionEnabled || reducedMotion ? ' is-quiet' : ''}`} aria-labelledby="journey-title">
    <header className="rbc-journey-header">
      <h2 id="journey-title">{t('จากหนึ่งทำเล', 'From your location.')}<span>{t('สู่จุดหมายใหม่', 'To a new destination.')}</span></h2>
      <button type="button" className="rbc-journey-skip" onClick={skip}>{t('ข้ามไปส่วนถัดไป', 'Skip this story')}<ArrowDown size={16}/></button>
    </header>
    <div className="rbc-journey-grid">
      <div className="rbc-journey-visual" aria-hidden="true">
        <div className="rbc-journey-sticky">
          <div className="rbc-journey-images">{workflowChapters.map((item, index) => <img key={item.time} className={index === current ? 'is-current' : ''} src={`/rbc-motion/chapter-${index + 1}.webp`} width="1280" height="720" alt="" loading="lazy" style={{ objectPosition: imagePositions[index] }}/>)}</div>
          <div className="rbc-journey-image-caption"><span>0{current + 1} / 04</span><span>{t('จากทำเล สู่สถานีชาร์จ', 'From site to station')}</span></div>
        </div>
      </div>
      <div className="rbc-journey-chapters">
        {workflowChapters.map((chapter, index) => <article key={chapter.time} ref={node => { rows.current[index] = node }} id={`rbc-story-step-${index + 1}`} className={`rbc-journey-chapter${index === current ? ' is-current' : ''}`} aria-labelledby={`journey-step-title-${index}`}>
          <img className="rbc-journey-mobile-image" src={`/rbc-motion/chapter-${index + 1}.webp`} width="1280" height="720" alt={t(`ภาพแนวคิด: ${chapter.title[0]}`, `Concept: ${chapter.title[1]}`)} loading="lazy"/>
          <span className="rbc-journey-count">0{index + 1}<span>/ 04</span></span>
          <h3 id={`journey-step-title-${index}`}><button type="button" aria-pressed={index === current} onClick={() => select(index)}>{chapter.title[lang]}</button></h3>
          <p>{chapter.body[lang]}</p>
          <Link className="rbc-text-link" to={investmentLink}>{t('เริ่มต้นจากพื้นที่ของคุณ', 'Start with your site')}<ArrowUpRight size={18}/></Link>
        </article>)}
      </div>
    </div>
    <p className="rbc-journey-disclaimer">{t('ภาพแนวคิด • การประเมินเบื้องต้น ต้องตรวจสอบหน้างานและระบบไฟฟ้า', 'Concept imagery · Preliminary assessment. Site and electrical verification required.')}</p>
  </section>
}
