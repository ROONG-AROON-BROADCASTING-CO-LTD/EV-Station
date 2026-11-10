import { publicLineUrl, publicLoginHref } from '../data/publicContact'
import { useEffect, useState } from 'react'
import { ArrowUpRight, MessageCircle } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useI18n } from '../i18n/I18nProvider'
import { loadLiffConfig } from '../services/liff'

export function SubmissionChannels({ authenticated }: { authenticated: boolean }) {
  const { language } = useI18n()
  const text = (th: string, en: string) => language === 'th' ? th : en
  const [liffURL, setLiffURL] = useState<string>()
  const [failed, setFailed] = useState(false)
  const [attempt, setAttempt] = useState(0)
  useEffect(() => {
    let active = true
    setFailed(false)
    void loadLiffConfig().then(config => {
      if (active) setLiffURL(`https://liff.line.me/${config.liffId}`)
    }).catch(() => { if (active) setFailed(true) })
    return () => { active = false }
  }, [attempt])

  return <section id="submit-site" className="not-italic scroll-mt-32" aria-label={text('เลือกช่องทางส่งข้อมูลพื้นที่', 'Choose how to submit your site')}>
    <div className="my-5 space-y-2">
      <h2 className="text-xl font-bold">{text('ข้อมูลที่ควรเตรียมก่อนส่งทำเล', 'What to prepare before submitting')}</h2>
      <ul className="list-disc space-y-1 ps-5 text-sm leading-6">
        <li>{text('พิกัดทำเลหรือลิงก์ปักหมุดบนแผนที่', 'Site coordinates or a map pin link')}</li>
        <li>{text('รูปพื้นที่และทางเข้า–ออก (ถ้ามี)', 'Photos of the site and its access points (if available)')}</li>
        <li>{text('ขนาดที่ดินโดยประมาณ (ถ้าทราบ)', 'Approximate plot size (if known)')}</li>
      </ul>
    </div>
    <div className="my-5 space-y-4 sm:hidden">
      <h2 className="text-xl font-bold">{text('ส่งพื้นที่ผ่าน LINE', 'Submit your site through LINE')}</h2>
      <p className="text-sm leading-6">{text('ส่งพิกัด แนบรูป และติดตามพื้นที่ของคุณด้วยบัญชี LINE', 'Send coordinates, attach photos and track your sites with your LINE account.')}</p>
      {liffURL ? <div className="flex flex-wrap gap-3">
        <a className="landing-cta" href={liffURL}>{text('เปิดแบบฟอร์มใน LINE', 'Open the form in LINE')}<ArrowUpRight size={18}/></a>
        <a className="inline-flex min-h-12 items-center rounded-lg border border-white px-6 py-3 text-white hover:bg-white/10 focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-white" href={`${liffURL}?view=my-sites`}>{text('ติดตามพื้นที่ที่ส่งแล้ว', 'Track submitted sites')}</a>
      </div> : failed ? <div role="status" className="text-sm"><p>{text('เปิดแบบฟอร์ม LINE ไม่สำเร็จ กรุณาลองอีกครั้งหรือติดต่อทีมงาน', 'The LINE form could not be loaded. Try again or contact our team.')}</p><button type="button" className="mt-2 underline" onClick={() => setAttempt(value => value + 1)}>{text('ลองอีกครั้ง', 'Try again')}</button></div> : <p role="status" className="text-sm">{text('กำลังเตรียมแบบฟอร์ม LINE…', 'Preparing the LINE form…')}</p>}
    </div>
    <div className="flex flex-col items-start gap-4">
      <Link className="landing-cta" to={authenticated ? '/sites/new' : publicLoginHref('/contact#submit-site', language, new URLSearchParams({ mode: 'register', returnTo: '/sites/new' }))}>{text('ส่งข้อมูลผ่านเว็บไซต์', 'Submit through the website')}<ArrowUpRight size={18}/></Link>
      <p className="text-sm leading-6">{text('เข้าสู่ระบบหรือสมัครบัญชี เพื่อส่งข้อมูลและเปิดรายงานบนเว็บไซต์', 'Sign in or create an account to submit your site and view its report on the website.')}</p>
      <a className="inline-flex min-h-12 items-center gap-3 rounded-lg border border-white px-6 py-3 text-white hover:bg-white/10 focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-white" href={publicLineUrl} target="_blank" rel="noreferrer">{text('แอด LINE เพื่อคุยกับทีมงาน', 'Add LINE to talk to our team')}<MessageCircle size={18}/></a>
      <span className="text-sm text-white">LINE @424cdtee</span>
    </div>
  </section>
}
