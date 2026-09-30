import { ArrowRight, BarChart3, BatteryCharging, Building2, CarFront, ClipboardCheck, MapPin, MessageCircle, Store, TrendingUp, Users, Zap } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useEffect, useRef, useState } from 'react'
import { useI18n } from '../i18n/I18nProvider'
import { PublicHeader } from '../components/PublicHeader'
import { api } from '../services/api'

function copy(language: 'th' | 'en', th: string, en: string) {
  return language === 'th' ? th : en
}

const evMarketComparison = {
  2024: { total: '44,911,738', nonBev: '44,684,248', bev: '227,490', share: '0.51%' },
  2025: { total: '45,376,703', nonBev: '45,046,361', bev: '330,342', share: '0.73%' },
  2026: { total: '45,947,597', nonBev: '45,456,101', bev: '491,496', share: '1.07%' },
  totalGrowth2025: '+1.04%',
  totalGrowth2026: '+1.26%',
  nonBevGrowth2025: '+0.81%',
  nonBevGrowth2026: '+0.91%',
  bevGrowth2025: '+45.21%',
  bevGrowth2026: '+48.79%',
  shareChange: '+0.56 จุดเปอร์เซ็นต์',
}
function SectionTitle({ title, description }: { title: string; description?: string }) {
  return <div className="public-section-title"><h2>{title}</h2>{description ? <p>{description}</p> : null}</div>
}

export function LandingPage({ authenticated = false, accountName, onLogout }: { authenticated?: boolean; accountName?: string; onLogout?: () => void }) {
  const { language } = useI18n()
  const investmentLink = authenticated ? '/sites/new' : '/login?mode=register'
  const [visitorOrdinal, setVisitorOrdinal] = useState<number | null>(null)
  const visitRegistered = useRef(false)

  useEffect(() => {
    if (visitRegistered.current) return
    visitRegistered.current = true
    api.registerVisitorOrdinal()
      .then(({ ordinal }) => setVisitorOrdinal(ordinal))
      .catch(() => undefined)
  }, [])

  return <main className="landing-page public-site">
    <PublicHeader authenticated={authenticated} accountName={accountName} onLogout={onLogout} />

    <section className="landing-hero" aria-labelledby="landing-title"><div className="landing-hero-art" aria-hidden="true"/><div className="landing-hero-copy"><h1 id="landing-title"><span>{copy(language, 'จุดเริ่มต้นสถานีชาร์จ', 'Your EV station starts here')}</span><strong><span>{copy(language, 'ลงทุน EV กับ', 'Invest in EV with')}</span><span>RBC Group</span></strong></h1><span>{copy(language, 'ส่งทำเลให้ทีม RBC ประเมินศักยภาพพื้นที่ ความพร้อมด้านไฟฟ้า และรูปแบบสถานีชาร์จที่เหมาะสม เพื่อประกอบการพิจารณาการลงทุน', 'Submit your site for an evidence-led review of location potential, power readiness, and the charging format that fits it.')}</span><Link className="landing-cta" to={investmentLink}>{copy(language, 'ส่งทำเลให้ประเมิน', 'Submit a site for review')}<ArrowRight size={19}/></Link><p className="landing-hero-signoff">RBC GROUP · EV INVESTMENT · SUPER BLACK COFFEE</p></div></section>

    <section className="public-proof-strip" aria-label={copy(language, 'สิ่งที่ระบบช่วยคุณตรวจสอบ', 'What the platform helps you review')}><div><MapPin size={28}/><span>{copy(language, 'วิเคราะห์ทำเลจริง', 'Analyse the real site')}</span></div><div><Zap size={28}/><span>{copy(language, 'ประเมินความพร้อมระบบไฟฟ้า', 'Review power readiness')}</span></div><div><ClipboardCheck size={28}/><span>{copy(language, 'ติดตามผลในบัญชีเดียว', 'Track progress in one account')}</span></div></section>
    <div className="public-visitor-ordinal" aria-live="polite"><Users size={20}/><span>{copy(language, 'จำนวนครั้งที่เข้าชมเว็บไซต์', 'Website visits')}</span><strong>{visitorOrdinal?.toLocaleString() ?? '200,000'}</strong></div>

    <section id="market" className="public-market">
      <div className="public-market-heading">
        <p className="public-eyebrow">THAILAND EV MARKET</p>
        <SectionTitle title={copy(language, 'ตลาด EV ไทย: ภาพรวม 3 ปี', 'Thailand EV market: three-year view')} description={copy(language, 'แสดงรถจดทะเบียนสะสมและ BEV ในช่วงสิ้นปี 2567 ถึง 30 กันยายน 2569 เพื่อให้เห็นทิศทางของตลาดอย่างต่อเนื่อง โดยตัวเลขปี 2569 เป็นประมาณการตามข้อมูลที่ได้รับ', 'Shows cumulative registered vehicles and BEVs from year-end 2024 to 30 September 2026. The 2026 figures are estimates based on supplied data.')}/>
      </div>
      <div className="public-market-comparison" role="table" aria-label={copy(language, 'เปรียบเทียบรถจดทะเบียนสามปี', 'Three-year registered vehicle comparison')}>
        <div className="public-market-row public-market-head" role="row"><span role="columnheader">{copy(language, 'รายการ', 'Measure')}</span><span role="columnheader">{copy(language, '2567', '2024')}<small>{copy(language, '31 ธ.ค. 67', '31 Dec 2024')}</small></span><span role="columnheader">{copy(language, '2568', '2025')}<small>{copy(language, '30 ก.ย. 68', '30 Sep 2025')}</small></span><span role="columnheader">{copy(language, '2569', '2026')}<small>{copy(language, 'ประมาณการ 30 ก.ย. 69', 'Estimated 30 Sep 2026')}</small></span></div>
        <div className="public-market-row" role="row"><strong role="rowheader">{copy(language, 'รถจดทะเบียนสะสมทั้งหมด', 'All registered vehicles')}</strong><span>{evMarketComparison[2024].total}</span><span>{evMarketComparison[2025].total}<small>{evMarketComparison.totalGrowth2025}</small></span><span>{evMarketComparison[2026].total}<small>{evMarketComparison.totalGrowth2026}</small></span></div>
        <div className="public-market-row" role="row"><strong role="rowheader">{copy(language, 'รถจดทะเบียนสะสมอื่น ๆ', 'Other registered vehicles')}</strong><span>{evMarketComparison[2024].nonBev}</span><span>{evMarketComparison[2025].nonBev}<small>{evMarketComparison.nonBevGrowth2025}</small></span><span>{evMarketComparison[2026].nonBev}<small>{evMarketComparison.nonBevGrowth2026}</small></span></div>
        <div className="public-market-row public-market-bev" role="row"><strong role="rowheader"><BatteryCharging size={18}/>{copy(language, 'รถ BEV สะสม', 'Cumulative BEVs')}</strong><span>{evMarketComparison[2024].bev}</span><span>{evMarketComparison[2025].bev}<small>{evMarketComparison.bevGrowth2025}</small></span><span>{evMarketComparison[2026].bev}<small>{evMarketComparison.bevGrowth2026}</small></span></div>
        <div className="public-market-row public-market-share" role="row"><strong role="rowheader">{copy(language, 'สัดส่วน BEV ต่อรถทั้งหมด', 'BEV share of all vehicles')}</strong><span>{evMarketComparison[2024].share}</span><span>{evMarketComparison[2025].share}</span><span>{evMarketComparison[2026].share}</span></div>
      </div>
      <div className="public-market-growth-grid"><article><CarFront size={24}/><span>{copy(language, 'ตลาดรถทั้งหมด', 'All vehicle market')}</span><strong>{evMarketComparison.totalGrowth2026}</strong><small>{copy(language, 'การเติบโตจากปี 2568 ถึงตัวเลขประมาณการปี 2569', 'Growth from 2025 to the 2026 estimate')}</small></article><article><Users size={24}/><span>{copy(language, 'รถจดทะเบียนสะสมอื่น ๆ', 'Other registered vehicles')}</span><strong>{evMarketComparison.nonBevGrowth2026}</strong><small>{copy(language, 'รวมรถทุกประเภท ยกเว้นรถ BEV', 'Includes every registered vehicle category except BEVs')}</small></article><article className="public-market-growth"><TrendingUp size={24}/><span>{copy(language, 'รถ BEV', 'BEVs')}</span><strong>{evMarketComparison.bevGrowth2026}</strong><small>{copy(language, 'BEV เพิ่ม 161,154 คันจากปี 2568 สู่ตัวเลขประมาณการปี 2569', 'BEVs increase by 161,154 vehicles from 2025 to the 2026 estimate')}</small></article></div>
      <div className="public-market-insight"><div><strong>{copy(language, 'สัดส่วน BEV ต่อรถจดทะเบียนทั้งหมด', 'BEVs as a share of registered vehicles')}</strong><span>{evMarketComparison[2024].share} → {evMarketComparison[2026].share}</span><div className="public-market-bar" aria-label={copy(language, 'สัดส่วน BEV เพิ่มจาก 0.51 เปอร์เซ็นต์ในปี 2567 เป็น 1.07 เปอร์เซ็นต์ในตัวเลขประมาณการปี 2569', 'BEV share increases from 0.51 percent in 2024 to 1.07 percent in the 2026 estimate')}><i/></div></div><p>{copy(language, 'ส่วนแบ่งเพิ่ม ' + evMarketComparison.shareChange, 'Share increases by ' + evMarketComparison.shareChange.replace('จุดเปอร์เซ็นต์', 'percentage points'))}</p></div>
    </section>

<section id="about" className="public-about"><div className="public-about-copy"><p className="public-eyebrow">RBC GROUP</p><SectionTitle title={copy(language, 'สร้างจุดหมายใหม่ให้การเดินทางด้วย EV', 'Building better destinations for EV travel')} description={copy(language, 'RBC Group วางรูปแบบธุรกิจที่เชื่อมสถานีชาร์จ EV พื้นที่บริการ และประสบการณ์ระหว่างการเดินทาง เพื่อให้การลงทุนและการใช้งานเกิดขึ้นบนข้อมูลของพื้นที่จริง', 'RBC Group connects EV charging, destination services, and the travel experience so investment and use are grounded in real-site evidence.')}/><p className="public-about-vision"><strong>{copy(language, 'วิสัยทัศน์', 'Vision')}</strong>{copy(language, 'พัฒนาพื้นที่ที่สนับสนุนการเดินทางพลังงานสะอาด พร้อมบริการที่มีคุณค่าในชีวิตประจำวัน', 'Develop destinations that support cleaner travel and provide useful everyday services.')}</p></div><div className="public-about-roles"><article><Building2 size={25}/><h3>RBC Group</h3><p>{copy(language, 'เจ้าของภาพรวมการลงทุนและการพัฒนาพื้นที่', 'Leads the investment and destination-development picture.')}</p></article><article><Zap size={25}/><h3>RBC EV Station</h3><p>{copy(language, 'วิเคราะห์ทำเล วางแผนรูปแบบสถานี และติดตามโครงการผ่านระบบเดียว', 'Assesses sites, plans station formats, and tracks projects in one system.')}</p></article><article><Store size={25}/><h3>Super Black Coffee</h3><p>{copy(language, 'พื้นที่บริการและจุดพักที่ช่วยเติมประสบการณ์ระหว่างการชาร์จ', 'A service destination that adds value to charging time.')}</p></article></div></section>

    <section id="reviews" className="public-reviews"><SectionTitle title={copy(language, 'เหตุผลที่เจ้าของพื้นที่เลือกคุยกับ RBC', 'Why site owners start a conversation with RBC')} description={copy(language, 'เปลี่ยนข้อมูลทำเลให้เป็นจุดเริ่มต้นของการวางแผนสถานีชาร์จและธุรกิจบริการที่เหมาะกับพื้นที่', 'Turn site information into a practical starting point for planning the right charging station and destination service.')}/><div className="public-review-grid">{[{ icon: MapPin, userTh: 'ผู้ใช้บริการ 01', userEn: 'Service user 01', titleTh: 'เริ่มจากข้อมูลพื้นที่จริง', titleEn: 'Start with the real site', detailTh: 'ส่งพิกัด ภาพ และรายละเอียดพื้นที่ เพื่อให้ทีม RBC เห็นบริบทของทำเลก่อนเริ่มพิจารณา', detailEn: 'Share coordinates, photos, and site details so the RBC team can understand the location before review.' }, { icon: BarChart3, userTh: 'ผู้ใช้บริการ 02', userEn: 'Service user 02', titleTh: 'วางแผนให้เหมาะกับทำเล', titleEn: 'Plan around the site', detailTh: 'ใช้รูปแบบ S / M / L เป็นแนวทางคุยเรื่องขนาดพื้นที่ จุดชาร์จ และองค์ประกอบของธุรกิจ', detailEn: 'Use the S / M / L formats to guide the discussion around space, charging capacity, and business elements.' }, { icon: MessageCircle, userTh: 'ผู้ใช้บริการ 03', userEn: 'Service user 03', titleTh: 'ติดต่อทีมงานได้โดยตรง', titleEn: 'Speak directly with the team', detailTh: 'ติดต่อสอบถามผ่าน LINE หรือช่องทางบริษัท เพื่อคุยรายละเอียดโครงการและนัดหมายต่อไป', detailEn: 'Contact the team through LINE or the company channels to discuss project details and arrange the next step.' }].map(item => { const Icon = item.icon; return <article key={item.titleTh} className="public-review-placeholder public-value-card"><div className="public-review-card-head"><span className="public-review-avatar"><Icon size={20}/></span><div><strong>{copy(language, item.titleTh, item.titleEn)}</strong><small>{copy(language, item.userTh, item.userEn)}</small></div></div><p>{copy(language, item.detailTh, item.detailEn)}</p></article> })}</div></section>
        <footer className="public-footer"><div className="public-footer-brand"><span className="public-footer-mark"><img src="/rbc-group-logo.png" alt="RBC Group"/></span><div><strong>RBC EV STATION</strong><small>SUSTAINABLE JOURNEY TOGETHER</small></div></div><div><strong>{copy(language, 'เริ่มต้นปรึกษาการลงทุน', 'Start an investment consultation')}</strong><p>{copy(language, 'ส่งพิกัด ภาพ และข้อมูลพื้นที่ผ่านระบบ RBC EV Station เพื่อให้ทีมงานตรวจสอบและติดต่อกลับตามข้อมูลบัญชีที่คุณให้ไว้', 'Submit coordinates, photos, and site details through RBC EV Station so the team can review them and follow up through your account.')}</p></div><div><strong>{copy(language, 'ผลประเมินเบื้องต้น', 'Preliminary assessment')}</strong><p>{copy(language, 'ผลที่แสดงในระบบเป็นผลประเมินเบื้องต้นจากข้อมูลพื้นที่ที่ได้รับ', 'Results shown in the system are preliminary assessments based on the submitted site information.')}</p></div></footer>
  </main>
}
