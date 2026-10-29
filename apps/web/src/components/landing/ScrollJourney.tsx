import { useEffect, useRef, useState, useSyncExternalStore } from 'react'
import { ArrowDown, ArrowLeft, ArrowRight, ArrowUpRight, ChevronDown, Play } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useI18n } from '../../i18n/I18nProvider'
import { WorkflowFilm } from './WorkflowFilm'
import { workflowChapters } from './workflowChapters'
import { journeyProgress, journeyStep } from './journeyProgress'
import './scroll-journey.css'

const shortScreenQuery = '(max-height: 720px), (max-width: 700px) and (max-height: 820px)'
const subscribeShortScreen = (callback: () => void) => {
  const media = window.matchMedia(shortScreenQuery)
  media.addEventListener('change', callback)
  return () => media.removeEventListener('change', callback)
}

export function ScrollJourney({ motionEnabled, reducedMotion, investmentLink }: { motionEnabled: boolean; reducedMotion: boolean; investmentLink: string }) {
  const { language } = useI18n()
  const lang = language === 'th' ? 0 : 1
  const t = (th: string, en: string) => language === 'th' ? th : en
  const section = useRef<HTMLElement>(null)
  const stage = useRef<HTMLDivElement>(null)
  const filmDisclosure = useRef<HTMLDetailsElement>(null)
  const scrollToFilm = useRef(false)
  const selected = useRef(0)
  const [current, setCurrent] = useState(0)
  const [filmOpen, setFilmOpen] = useState(false)
  const shortScreen = useSyncExternalStore(subscribeShortScreen, () => window.matchMedia(shortScreenQuery).matches, () => true)
  const pinned = !reducedMotion && !shortScreen

  useEffect(() => {
    const element = section.current
    const panel = stage.current
    if (!element || !panel || !pinned) return
    let frame = 0
    let visible = false
    const paint = () => {
      frame = 0
      if (document.hidden || !visible) return
      const header = document.querySelector('.landing-header')?.getBoundingClientRect().height ?? 84
      element.style.setProperty('--journey-header', `${header}px`)
      const progress = journeyProgress(element.getBoundingClientRect().top, element.offsetHeight, panel.offsetHeight, header)
      const index = journeyStep(progress)
      element.style.setProperty('--journey-progress', String(progress))
      element.style.setProperty('--scene-progress', String(progress * workflowChapters.length - index))
      if (index !== selected.current) { selected.current = index; setCurrent(index) }
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
  }, [pinned])

  useEffect(() => {
    if (!filmOpen || !scrollToFilm.current) return
    scrollToFilm.current = false
    const frame = requestAnimationFrame(() => filmDisclosure.current?.scrollIntoView({ behavior: motionEnabled ? 'smooth' : 'instant', block: 'start' }))
    return () => cancelAnimationFrame(frame)
  }, [filmOpen, motionEnabled])

  const select = (index: number) => {
    if (!pinned || !section.current || !stage.current) { selected.current = index; setCurrent(index); return }
    const header = document.querySelector('.landing-header')?.getBoundingClientRect().height ?? 84
    const distance = Math.max(0, section.current.offsetHeight - stage.current.offsetHeight)
    const top = window.scrollY + section.current.getBoundingClientRect().top - header + distance * ((index + .12) / workflowChapters.length)
    window.scrollTo({ top, behavior: motionEnabled ? 'smooth' : 'instant' })
  }
  const skip = () => filmDisclosure.current?.nextElementSibling?.scrollIntoView({ behavior: motionEnabled ? 'smooth' : 'instant', block: 'start' })
  const watch = () => {
    if (filmOpen) {
      filmDisclosure.current?.scrollIntoView({ behavior: motionEnabled ? 'smooth' : 'instant', block: 'start' })
      return
    }
    scrollToFilm.current = true
    setFilmOpen(true)
  }
  const chapter = workflowChapters[current]

  return <>
    <section ref={section} id="rbc-story" className={`rbc-scroll-journey${pinned ? '' : ' is-static'}${motionEnabled ? '' : ' is-quiet'}`} aria-labelledby="journey-title">
      <div ref={stage} className="rbc-journey-stage">
        <div className="rbc-journey-images" aria-hidden="true">{workflowChapters.map((item, index) => <img key={item.time} className={index === current ? 'is-current' : ''} src={`/rbc-motion/chapter-${index + 1}.webp`} width="1280" height="720" alt="" loading="lazy" style={{ objectPosition: ['42% center', '64% center', '58% center', '60% center'][index] }}/>)}</div>
        <div className="rbc-journey-shade"/>
        <header className="rbc-journey-header"><h2 id="journey-title">{t('จากหนึ่งทำเล', 'From your location.')}<span>{t('สู่จุดหมายใหม่', 'To a new destination.')}</span></h2><div className="rbc-journey-actions"><button type="button" onClick={watch}><Play size={16}/><span>{t('ชมภาพยนตร์ 40 วินาที', 'Watch the 40-second film')}</span></button><button type="button" onClick={skip}>{t('ข้ามไปส่วนถัดไป', 'Skip this story')}<ArrowDown size={16}/></button></div></header>
        <div className="rbc-journey-copy" key={current}><span className="rbc-journey-count">0{current + 1}<span>/ 04</span></span><h3>{chapter.title[lang]}</h3><p>{chapter.body[lang]}</p><Link className="rbc-text-link" to={investmentLink}>{t('เริ่มต้นจากพื้นที่ของคุณ', 'Start with your site')}<ArrowUpRight size={19}/></Link><small>{t('ภาพแนวคิด • การประเมินเบื้องต้น ต้องตรวจสอบหน้างานและระบบไฟฟ้า', 'Concept imagery · Preliminary assessment. Site and electrical verification required.')}</small></div>
        <footer className="rbc-journey-footer"><div className="rbc-journey-navigation"><button type="button" disabled={current === 0} onClick={() => select(current - 1)} aria-label={t('ขั้นตอนก่อนหน้า', 'Previous step')}><ArrowLeft size={19}/></button><div className="rbc-journey-steps" role="group" aria-label={t('เลือกขั้นตอนโครงการ', 'Choose a project step')}>{workflowChapters.map((item, index) => <button key={item.time} type="button" aria-pressed={current === index} onClick={() => select(index)}><span>0{index + 1}</span><strong>{item.title[lang]}</strong></button>)}</div><button type="button" disabled={current === 3} onClick={() => select(current + 1)} aria-label={t('ขั้นตอนถัดไป', 'Next step')}><ArrowRight size={19}/></button></div><div className="rbc-journey-progress" aria-hidden="true"><i style={pinned ? undefined : { transform: `scaleX(${(current + 1) / 4})` }}/></div><p className="rbc-journey-scroll-hint">{pinned ? t('เลื่อนเพื่อสำรวจ • ย้อนกลับได้ทุกขั้นตอน', 'Scroll to explore · Scroll back to revisit') : t('เลือกขั้นตอนเพื่อสำรวจเรื่องราว', 'Choose a step to explore the story')}</p></footer>
      </div>
    </section>
    <details ref={filmDisclosure} className="rbc-journey-film" open={filmOpen} onToggle={event => setFilmOpen(event.currentTarget.open)}><summary><Play size={18}/>{t('ชมภาพยนตร์แนวคิด 40 วินาที', 'Watch the 40-second concept film')}<ChevronDown size={19}/></summary>{filmOpen && <WorkflowFilm motionEnabled={motionEnabled} investmentLink={investmentLink}/>}</details>
  </>
}
