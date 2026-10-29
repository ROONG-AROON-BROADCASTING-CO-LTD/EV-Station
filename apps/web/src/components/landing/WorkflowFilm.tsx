import { useEffect, useRef, useState } from 'react'
import { ArrowUpRight, Maximize, Pause, Play, RotateCcw } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useI18n } from '../../i18n/I18nProvider'

import { workflowChapters as chapters } from './workflowChapters'


const clock = (seconds: number) => `0:${String(Math.floor(seconds)).padStart(2, '0')}`

export function WorkflowFilm({ motionEnabled, investmentLink }: { motionEnabled: boolean; investmentLink: string }) {
  const { language } = useI18n()
  const lang = language === 'th' ? 0 : 1
  const t = (th: string, en: string) => language === 'th' ? th : en
  const video = useRef<HTMLVideoElement>(null)
  const frame = useRef<HTMLDivElement>(null)
  const pendingSeek = useRef<number | null>(null)
  const shouldPlay = useRef(false)
  const [loaded, setLoaded] = useState(false)
  const [playing, setPlaying] = useState(false)
  const [waiting, setWaiting] = useState(false)
  const [error, setError] = useState(false)
  const [elapsed, setElapsed] = useState(0)
  const [selected, setSelected] = useState(0)

  const play = () => {
    const media = video.current
    if (!media) return
    shouldPlay.current = true
    setError(false)
    setWaiting(true)
    if (!media.getAttribute('src')) media.src = '/rbc-motion/workflow-40s.mp4'
    if (media.error) media.load()
    if (media.ended) media.currentTime = 0
    void media.play().catch(() => { shouldPlay.current = false; setWaiting(false) })
  }

  const selectChapter = (index: number) => {
    const media = video.current
    // Land beyond the crossfade so a paused chapter has one clean, readable frame.
    const time = chapters[index].time + (index === 0 ? 0 : .65)
    setSelected(index)
    setElapsed(time)
    pendingSeek.current = time
    if (media && media.readyState >= 1) {
      media.currentTime = time
      pendingSeek.current = null
    }
  }

  useEffect(() => {
    const media = video.current
    if (!media) return
    // Never restart automatically after the visitor scrolls away or pauses motion.
    const pause = () => { shouldPlay.current = false; setWaiting(false); media.pause() }
    const visibility = () => { if (document.hidden) pause() }
    const observer = new IntersectionObserver(([entry]) => { if (!entry.isIntersecting) pause() }, { threshold: .05 })
    observer.observe(media)
    document.addEventListener('visibilitychange', visibility)
    if (!motionEnabled) pause()
    return () => { observer.disconnect(); document.removeEventListener('visibilitychange', visibility); pause() }
  }, [motionEnabled])

  return <section className="rbc-story" id="rbc-film-viewer" aria-labelledby="film-title">
    <div className="rbc-editorial-heading">
      <h2 id="film-title">{t('จากหนึ่งทำเล', 'From a location.')}<br/><em>{t('สู่ความเป็นไปได้ใหม่', 'To a new possibility.')}</em></h2>
      <p>{t('ดูแนวทางการทำงานของ RBC ใน 40 วินาที เลือกแต่ละช่วงเพื่อสำรวจสิ่งที่เกิดขึ้นต่อไป', 'Explore the RBC journey in 40 seconds. Choose a chapter to see what comes next.')}</p>
    </div>
    <div className="rbc-film-frame" ref={frame}>
      <div className="rbc-film-screen">
        <img src={`/rbc-motion/chapter-${selected + 1}.webp`} width="1280" height="720" alt={t('ภาพแนวคิด: ', 'Concept: ') + chapters[selected].title[lang]} loading="lazy" />
        <video ref={video} className={loaded && !error ? 'is-loaded' : ''} muted playsInline preload="none" aria-label={t('ภาพยนตร์แนวคิดโครงการ RBC 40 วินาที ไม่มีเสียง', 'RBC project concept film, 40 seconds, silent')}
          onLoadedMetadata={() => { if (video.current && pendingSeek.current !== null) { video.current.currentTime = pendingSeek.current; pendingSeek.current = null } }}
          onLoadedData={() => setLoaded(true)}
          onTimeUpdate={() => { const time = video.current?.currentTime ?? 0; setElapsed(time); setSelected(Math.min(3, Math.floor(time / 10))) }}
          onPlaying={() => { if (!shouldPlay.current) { video.current?.pause(); return } setPlaying(true); setWaiting(false) }}
          onPause={() => { setPlaying(false); setWaiting(false) }} onEnded={() => { shouldPlay.current = false; setPlaying(false) }}
          onWaiting={() => { if (shouldPlay.current) setWaiting(true) }}
          onError={() => { shouldPlay.current = false; setError(true); setWaiting(false); setPlaying(false) }} />
        {!playing && <button className="rbc-film-start" type="button" onClick={play} aria-label={t('เล่นภาพยนตร์แนวคิด 40 วินาที', 'Play the 40-second concept film')}><Play size={28} fill="currentColor"/><span>{waiting ? t('กำลังเตรียมวิดีโอ…', 'Loading film…') : elapsed >= 39.9 ? t('ดูอีกครั้ง', 'Watch again') : t('ชมเรื่องราว', 'Watch the story')}</span></button>}
        <span className="rbc-film-disclaimer">{t('ภาพประกอบแนวคิด • ไม่มีเสียง', 'Concept film · No sound')}</span>
      </div>
      <div className="rbc-film-controls">
        <button type="button" onClick={() => { if (playing) { shouldPlay.current = false; video.current?.pause() } else play() }} aria-label={playing ? t('หยุดวิดีโอชั่วคราว', 'Pause film') : t('เล่นวิดีโอ', 'Play film')}>{playing ? <Pause size={19}/> : elapsed >= 39.9 ? <RotateCcw size={19}/> : <Play size={19}/>}</button>
        <span className="rbc-film-time">{clock(elapsed)} / 0:40</span>
        <label className="rbc-film-scrub"><span className="rbc-sr-only">{t('ตำแหน่งในวิดีโอ', 'Video position')}</span><input type="range" min="0" max="39.9" step=".1" value={elapsed} aria-valuetext={`${clock(elapsed)} / 0:40`} onChange={event => { const time = Number(event.target.value); setElapsed(time); setSelected(Math.min(3, Math.floor(time / 10))); pendingSeek.current = time; if (video.current && video.current.readyState >= 1) { video.current.currentTime = time; pendingSeek.current = null } }} /></label>
        <button type="button" onClick={() => { if (frame.current?.requestFullscreen) void frame.current.requestFullscreen().catch(() => {}) }} aria-label={t('ดูเต็มหน้าจอ', 'View fullscreen')}><Maximize size={18}/></button>
      </div>
    </div>
    {error && <p className="rbc-film-error" role="status">{t('วิดีโอยังโหลดไม่สำเร็จ คุณยังเลือกดูภาพและอ่านแต่ละขั้นตอนได้ หรือกดเล่นเพื่อลองอีกครั้ง', 'The film could not load. Explore the chapter images and descriptions, or press play to try again.')}</p>}
    <div className="rbc-chapters" role="group" aria-label={t('เลือกช่วงของเรื่องราว', 'Choose a chapter')}>
      {chapters.map((chapter, index) => <button key={chapter.time} type="button" aria-pressed={selected === index} onClick={() => selectChapter(index)}><span className="rbc-chapter-progress" aria-hidden="true"><i style={{ transform: `scaleX(${Math.max(0, Math.min(1, (elapsed - chapter.time) / 10))})` }}/></span><span className="rbc-chapter-number">0{index + 1}</span><strong>{chapter.title[lang]}</strong><ArrowUpRight size={18}/></button>)}
    </div>
    <div className="rbc-story-caption"><p key={selected}>{chapters[selected].body[lang]}</p><Link className="rbc-text-link" to={investmentLink}>{t('เริ่มต้นจากพื้นที่ของคุณ', 'Start with your site')}<ArrowUpRight size={20}/></Link></div>
    <p className="rbc-story-note">{t('การประเมินเป็นข้อมูลเบื้องต้น ต้องตรวจสอบหน้างานและระบบไฟฟ้าก่อนดำเนินโครงการ', 'Assessment is preliminary. Site and electrical verification are required before proceeding.')}</p>
  </section>
}
