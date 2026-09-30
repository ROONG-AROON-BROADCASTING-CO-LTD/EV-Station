import { ArrowUp, ArrowUpRight, Check, CircleAlert, FileText, MapPin, Printer, ShieldCheck, TriangleAlert, Upload, Zap } from 'lucide-react'
import { Link } from 'react-router-dom'
import { PublicHeader } from '../components/PublicHeader'
import { LanguageSwitcher, useI18n } from '../i18n/I18nProvider'

type Audience = 'customer' | 'admin'
type GuideStep = { title: string; body: string; bullets?: string[] }
const copy = (language: 'th' | 'en', th: string, en: string) => language === 'th' ? th : en

function GuideSection({ id, title, intro, steps }: { id: string; title: string; intro?: string; steps: GuideStep[] }) {
  return <section className="guide-section" id={id}>
    <div className="guide-section-heading"><h2>{title}</h2>{intro ? <p>{intro}</p> : null}</div>
    <ol className="guide-steps">{steps.map((step, index) => <li key={step.title}>
      <span className="guide-step-marker" aria-hidden="true">{String(index + 1).padStart(2, '0')}</span>
      <div><h3>{step.title}</h3><p>{step.body}</p>{step.bullets?.length ? <ul>{step.bullets.map(item => <li key={item}>{item}</li>)}</ul> : null}</div>
    </li>)}</ol>
  </section>
}

function EvidenceTable() {
  const { language } = useI18n()
  const rows = [
    [copy(language, 'Google Maps', 'Google Maps'), copy(language, 'ลูกค้าต้องใส่ลิงก์ที่ปักหมุดบนที่ดิน ระบบใช้ตำแหน่งนี้ดึงข้อมูลรอบพื้นที่', 'Customers must provide a pin on the plot. The system uses it to gather nearby-area evidence.'), copy(language, 'จำเป็นสำหรับลูกค้า', 'Required for customers')],
    [copy(language, 'ขนาดและหน่วยพื้นที่', 'Land area and unit'), copy(language, 'ระบุขนาดรวมของที่ดินเป็นตัวเลขบวก และเลือก ตร.ม., ไร่, งาน หรือ ตร.วา', 'Enter the total land area as a positive number and choose sq m, rai, ngan, or sq wah.'), copy(language, 'จำเป็น', 'Required')],
    [copy(language, 'อินเทอร์เน็ต', 'Internet access'), copy(language, 'เลือกมี / ไม่มี / ไม่แน่ใจ เพื่อให้ทีมพิจารณาความพร้อมของพื้นที่', 'Select available, unavailable, or unsure for the site review.'), copy(language, 'แนะนำ', 'Recommended')],
    [copy(language, 'รูปถ่ายพื้นที่', 'Site photos'), copy(language, 'ควรส่ง 6–10 รูปที่ถ่ายจากหลายมุม เพื่อให้ AI อ่านสภาพพื้นที่ที่มองเห็นได้', 'Provide 6–10 photos from different angles so AI can review visible site conditions.'), copy(language, 'จำเป็นต่อการวิเคราะห์ภาพ', 'Needed for image analysis')],
    [copy(language, 'เอกสารประกอบ', 'Supporting documents'), copy(language, 'แนบผังหรือเอกสารที่เกี่ยวข้องได้ ถ้าเอกสารมีเลขบัตรหรือข้อมูลส่วนบุคคลที่ไม่จำเป็น ควรปิดทับก่อนส่ง', 'Attach a site plan or relevant document. Hide unnecessary ID numbers or personal data before upload.'), copy(language, 'ไม่บังคับ', 'Optional')],
  ]
  return <div className="guide-table-wrap"><table className="guide-table"><thead><tr><th>{copy(language, 'ข้อมูล', 'Information')}</th><th>{copy(language, 'รายละเอียด', 'Details')}</th><th>{copy(language, 'ความสำคัญ', 'Priority')}</th></tr></thead><tbody>{rows.map(row => <tr key={row[0]}>{row.map((cell, index) => index === 0 ? <th scope="row" key={cell}>{cell}</th> : <td key={cell}>{cell}</td>)}</tr>)}</tbody></table></div>
}

function StatusTable() {
  const { language } = useI18n()
  const rows = [
    [copy(language, 'ส่งข้อมูลแล้ว', 'Submitted'), copy(language, 'ระบบบันทึกข้อมูลพื้นที่แล้ว ทีม RBC ยังไม่ได้เริ่มวิเคราะห์', 'The site is saved. RBC has not started the analysis yet.')],
    [copy(language, 'ต้องตรวจเพิ่ม', 'Needs review'), copy(language, 'ข้อมูลบางส่วนขาดหาย หรือรอบวิเคราะห์ไม่สำเร็จ ทีมควรตรวจข้อมูลและติดต่อขอรายละเอียดเพิ่มถ้าจำเป็น', 'Some information is missing or an analysis did not complete. Review the record and contact the customer if details are needed.')],
    [copy(language, 'กำลังวิเคราะห์', 'Analysis in progress'), copy(language, 'รับงานแล้วและกำลังรวบรวมข้อมูล หน้าผลวิเคราะห์จะอัปเดตเมื่อเสร็จ', 'The job was accepted and is collecting data. The report updates when it completes.')],
    [copy(language, 'ผลวิเคราะห์พร้อม', 'Result ready'), copy(language, 'เปิดดูคะแนน หลักฐาน สถานะข้อมูล คำอธิบาย AI และคำแนะนำด้านรูปแบบโครงการได้', 'Open the score, evidence, data status, AI explanation, and project-format guidance.')],
  ]
  return <div className="guide-table-wrap"><table className="guide-table"><thead><tr><th>{copy(language, 'สถานะ', 'Status')}</th><th>{copy(language, 'ความหมายและขั้นตอนถัดไป', 'Meaning and next step')}</th></tr></thead><tbody>{rows.map(([status, meaning]) => <tr key={status}><th scope="row">{status}</th><td>{meaning}</td></tr>)}</tbody></table></div>
}

export function GuidePage({ audience, accountName, onLogout }: { audience: Audience; accountName?: string; onLogout?: () => void }) {
  const { language } = useI18n()
  const isCustomer = audience === 'customer'
  const sections = isCustomer
    ? [
      ['prepare', copy(language, 'เตรียมข้อมูลก่อนส่ง', 'Prepare the information')],
      ['photos', copy(language, 'ถ่ายรูปอย่างไรให้วิเคราะห์ได้ดี', 'Take useful site photos')],
      ['submit', copy(language, 'ส่งข้อมูลและติดตามผล', 'Submit and track progress')],
      ['read-result', copy(language, 'อ่านผลอย่างเข้าใจ', 'Understand the result')],
    ]
    : [
      ['portfolio', copy(language, 'ตรวจรายการพื้นที่', 'Review the site portfolio')],
      ['run-analysis', copy(language, 'เริ่มวิเคราะห์', 'Run an analysis')],
      ['read-report', copy(language, 'อ่านรายงานและตรวจหลักฐาน', 'Read the report and evidence')],
      ['recommendation', copy(language, 'พิจารณาขนาดสถานีและตู้ชาร์จ', 'Review format and charger guidance')],
      ['team-accounts', copy(language, 'บัญชีและสิทธิ์ทีมงาน', 'Team accounts and permissions')],
    ]

  return <div className={`guide-page ${isCustomer ? 'guide-standalone landing-page public-site' : ''}`}>
    {isCustomer ? <PublicHeader authenticated={Boolean(accountName)} accountName={accountName} onLogout={onLogout}/> : null}
    <header className="guide-hero">
      <div className="guide-hero-copy"><p className="guide-kicker">RBC EV STATION · {copy(language, 'ศูนย์ช่วยเหลือ', 'HELP CENTRE')}</p><h1>{isCustomer ? copy(language, 'คู่มือสำหรับลูกค้า', 'Customer guide') : copy(language, 'คู่มือสำหรับแอดมินและทีมงาน', 'Admin and team guide')}</h1><p>{isCustomer ? copy(language, 'เตรียมข้อมูลที่ดิน ส่งทำเล และติดตามผลคัดกรองได้ตามขั้นตอนในหน้านี้', 'Prepare your land details, submit a location, and follow the screening process with this step-by-step guide.') : copy(language, 'ตั้งแต่รับข้อมูลลูกค้า ตรวจหลักฐาน เริ่มวิเคราะห์ ไปจนถึงสรุปผลและติดตามงาน', 'From receiving customer details and checking evidence to running an analysis, reviewing results, and following up.')}</p></div>
      <div className="guide-hero-actions"><LanguageSwitcher/><button className="guide-print-button" type="button" onClick={() => window.print()}><Printer size={17}/>{copy(language, 'พิมพ์ / บันทึก PDF', 'Print / Save PDF')}</button></div>
    </header>
    <div className="guide-layout">
      <aside className="guide-toc" aria-label={copy(language, 'สารบัญคู่มือ', 'Guide contents')}><strong>{copy(language, 'ในคู่มือนี้', 'IN THIS GUIDE')}</strong><nav>{sections.map(([id, label]) => <a key={id} href={`#${id}`}>{label}<ArrowUpRight size={14}/></a>)}</nav></aside>
      <article className="guide-article">
        {isCustomer ? <>
          <div className="guide-callout guide-callout-info"><MapPin size={20}/><p><strong>{copy(language, 'สรุปข้อมูลสำคัญ', 'The essentials')}</strong>{copy(language, ' ลิงก์ Google Maps ที่ปักหมุดบนที่ดิน ขนาดพื้นที่พร้อมหน่วย และรูปถ่ายหลายมุมเป็นข้อมูลหลัก ส่วนทางเข้า–ออกควรกว้างอย่างน้อย 7 เมตรเพื่อรองรับการวางผัง แต่ลูกค้าไม่ต้องกรอกตัวเลขนี้ในระบบ', ' are a Google Maps pin on the plot, land area with its unit, and photos from several angles. An entrance/exit should be at least 7 m wide for layout planning, but customers do not need to enter this measurement in the system.')}</p></div>
          <GuideSection id="prepare" title={copy(language, '1. เตรียมข้อมูลก่อนส่ง', '1. Prepare the information')} intro={copy(language, 'กรอกตามข้อมูลจริง หากไม่ทราบค่าบางรายการให้แจ้งทีม RBC แทนการเดาตัวเลข', 'Use the information you know. If a value is unknown, tell RBC instead of guessing.') } steps={[
            { title: copy(language, 'ปักหมุดตรงที่ดิน', 'Pin the actual plot'), body: copy(language, 'เปิด Google Maps วางหมุดให้อยู่ภายในที่ดินหรือบริเวณทางเข้า แล้วคัดลอกลิงก์มาใส่ในช่อง Google Maps URL ตรวจว่าหมุดไม่ไปอยู่บนถนนหรือแปลงข้างเคียง', 'Place a Google Maps pin on the plot or its entrance, copy the link into Google Maps URL, and check that it is not on the road or a neighbouring parcel.') },
            { title: copy(language, 'ระบุขนาดพื้นที่และหน่วย', 'Enter land area and unit'), body: copy(language, 'กรอกขนาดรวมของที่ดินและเลือกหน่วยให้ตรงกับเอกสารหรือข้อมูลที่มี เช่น ไร่ งาน ตารางวา หรือตารางเมตร อย่าสลับหน่วย เพราะมีผลกับรูปแบบ S / M / L ที่ระบบแสดง', 'Enter the total area and select the correct unit—rai, ngan, sq wah, or sq m. The unit affects the S / M / L layout shown by the system.') },
            { title: copy(language, 'ใส่ข้อมูลประกอบที่ทราบ', 'Add other known details'), body: copy(language, 'เลือกสถานะอินเทอร์เน็ตถ้าทราบ ส่วนทางเข้า–ออกให้ใช้เป็นข้อแนะนำในการวางผังว่า ควรกว้างอย่างน้อย 7 เมตร ไม่ต้องกรอกตัวเลขในระบบ', 'Select internet availability if known. For layout planning, the entrance/exit should be at least 7 m wide; customers do not need to enter this measurement in the system.') },
          ]}/>
          <section className="guide-section" id="photos"><div className="guide-section-heading"><h2>{copy(language, '2. ถ่ายรูปอย่างไรให้วิเคราะห์ได้ดี', '2. Take useful site photos')}</h2><p>{copy(language, 'แนะนำ 6–10 ภาพ ถ่ายในช่วงกลางวัน ให้เห็นพื้นที่จริงและสภาพแวดล้อม ถ่ายให้ครอบคลุมเท่าที่ทำได้ ไม่จำเป็นต้องได้มุมตรงตามตัวอย่างทุกภาพ และไม่ใช้ฟิลเตอร์หรือภาพจำลอง รูปช่วยให้ AI อ่านเฉพาะสิ่งที่มองเห็นได้', 'We recommend 6–10 daytime photos showing the real site and surroundings. Cover as many views as possible; the examples are not a rigid checklist. Avoid filters and mock-ups. AI can assess only what is visible.')}</p></div>
            <ol className="guide-photo-list">{[
              copy(language, 'มุมกว้างจากถนนให้เห็นหน้าที่ดินและทางเข้า', 'Wide view from the road showing the frontage and entrance'),
              copy(language, 'มองเข้าหาที่ดินจากด้านซ้ายและด้านขวาของถนน', 'Approach view from both directions along the road'),
              copy(language, 'ภาพมุมกว้างจากภายในที่ดินไปยังถนน', 'Wide view from inside the plot toward the road'),
              copy(language, 'ถ่ายแนวขอบพื้นที่ อาคาร เสา รั้ว และสิ่งกีดขวาง', 'Show plot edges, buildings, poles, fences, and obstructions'),
              copy(language, 'ถ่ายพื้นผิว ทางลาด ร่องระบายน้ำ บ่อน้ำ และจุดที่มีน้ำขัง ถ้ามี', 'Show surfaces, slopes, drains, ponds, or water-logging if present'),
              copy(language, 'ถ่ายบริเวณโดยรอบและจุดเชื่อมถนนที่รถจะเข้า–ออก', 'Show nearby surroundings and the road connection vehicles would use'),
            ].map((item, index) => <li key={item}><span>{index + 1}</span>{item}</li>)}</ol>
            <div className="guide-callout guide-callout-warning"><TriangleAlert size={20}/><p>{copy(language, 'รูปถ่ายใช้ดูสภาพที่มองเห็น ไม่ใช่การวัดแนวเขต ตรวจโครงสร้างใต้ดิน หรือยืนยันว่ารถสวนกันได้ หากมีโฉนดหรือผังที่ดิน แนบประกอบได้แต่ไม่ถือเป็นการตรวจสอบกรรมสิทธิ์', 'Photos show visible conditions; they do not survey boundaries, inspect underground structures, or confirm two-way traffic. A deed or plot plan can add context but is not a title verification.')}</p></div>
            <EvidenceTable/>
            <p className="guide-file-limit"><Upload size={16}/>{copy(language, 'รวมรูปและเอกสารแนบทั้งหมดได้สูงสุด 10 ไฟล์ ไฟล์ละไม่เกิน 10 MB และรวมไม่เกิน 50 MB รองรับ JPEG, PNG, WebP และ PDF', 'Upload up to 10 photos and documents combined, 10 MB per file and 50 MB total. JPEG, PNG, WebP, and PDF are supported.')}</p>
          </section>
          <GuideSection id="submit" title={copy(language, '3. ส่งข้อมูลและติดตามผล', '3. Submit and track progress')} intro={copy(language, 'ส่งผ่านหน้าเว็บหรือแบบฟอร์มใน LINE แล้วกลับมาตรวจสถานะในบัญชีของคุณ', 'Submit on the website or through the LINE form, then check progress in your account.') } steps={[
            { title: copy(language, 'ส่งผ่านเว็บไซต์', 'Submit on the website'), body: copy(language, 'เลือก “ส่งทำเลให้ประเมิน” สมัครหรือเข้าสู่ระบบ แล้วกรอกชื่อพื้นที่ ลิงก์แผนที่ ขนาดและรายละเอียดที่ดิน แนบรูปและเอกสารที่มี จากนั้นกดบันทึก', 'Choose “Submit a site for review,” register or sign in, enter the site name, map link, land area and details, attach available photos or documents, then save.') },
            { title: copy(language, 'ส่งผ่าน LINE', 'Submit through LINE'), body: copy(language, 'เปิดแบบฟอร์ม RBC EV Station จาก LINE อนุญาตให้แบบฟอร์มเชื่อมบัญชี LINE กรอกข้อมูลและแนบไฟล์ แล้วรอข้อความยืนยัน หากแนบไฟล์ไม่สำเร็จให้เปิดรายการพื้นที่และลองส่งไฟล์อีกครั้ง', 'Open the RBC EV Station form from LINE and allow it to connect to your LINE account. Complete the form and attach files. If upload fails, reopen the site record and retry the files.') },
            { title: copy(language, 'ติดตามบัญชีของคุณ', 'Follow your site'), body: copy(language, 'เข้าสู่ระบบแล้วเปิดแดชบอร์ด เลือกรายการพื้นที่เพื่อดูสถานะและผลวิเคราะห์ เมื่อทีมขอข้อมูลเพิ่ม ให้เตรียมข้อมูลหรือรูปตามที่ทีมติดต่อมา', 'Sign in and open your dashboard. Select a site to view its status and report. If the team requests more information, provide the requested details or photos.') },
          ]}/>
          <StatusTable/>
          <GuideSection id="read-result" title={copy(language, '4. อ่านผลอย่างเข้าใจ', '4. Understand the result')} intro={copy(language, 'รายงานช่วยจัดข้อมูลให้พิจารณา ไม่ได้ยืนยันว่าจะลงทุนแล้วได้รายได้หรือผลตอบแทนตามที่คาด', 'The report organises evidence for review. It does not guarantee revenue or investment returns.') } steps={[
            { title: copy(language, 'ดูคะแนนพร้อมสถานะข้อมูล', 'Read the score with its data status'), body: copy(language, 'ตรวจว่าข้อมูลแต่ละรายการเป็นข้อมูลตรวจสอบแล้ว ประมาณการ ประเมินเบื้องต้น หรือไม่มีข้อมูล คะแนนและข้อสรุปอาจมีข้อจำกัดเมื่อแหล่งข้อมูลครอบคลุมไม่ถึงพื้นที่', 'Check whether each item is verified, estimated, preliminary, or missing. Coverage gaps can limit scores and conclusions.') },
            { title: copy(language, 'อ่านจุดแข็ง ความเสี่ยง และสิ่งที่ต้องยืนยัน', 'Review strengths, risks, and required checks'), body: copy(language, 'ใช้คำอธิบายประกอบเพื่อเตรียมพูดคุยกับทีม RBC การอ่านภาพช่วยเห็นเฉพาะสภาพผิวดินและสิ่งที่ปรากฏในภาพ ไม่แทนการสำรวจหน้างาน', 'Use the explanation to prepare for a discussion with RBC. Image review covers visible surface conditions only and does not replace a site visit.') },
            { title: copy(language, 'รอการตรวจหน้างานและวิศวกรรม', 'Wait for site and engineering checks'), body: copy(language, 'ขนาดและจำนวนตู้ชาร์จเป็นแนวทางวางแผน ต้องยืนยันผังจริง กำลังไฟ จุดเชื่อมต่อ และขนาดหม้อแปลงโดยวิศวกรและ PEA/MEA ก่อนตัดสินใจติดตั้ง', 'Format and cabinet count are planning guidance. Engineers and PEA/MEA must confirm layout, power, connection point, and transformer before installation.') },
          ]}/>
          <div className="guide-end-links"><Link to="/investment">{copy(language, 'ดูรูปแบบโครงการและรายละเอียดลงทุน', 'Explore project formats and investment details')}<ArrowUpRight size={16}/></Link><Link to="/contact">{copy(language, 'ติดต่อทีม RBC', 'Contact RBC')}<ArrowUpRight size={16}/></Link></div>
        </> : <>
          <div className="guide-callout guide-callout-info"><ShieldCheck size={20}/><p><strong>{copy(language, 'หลักการทำงาน', 'Operating principle')}</strong>{copy(language, ' รักษาที่มาของข้อมูล แยกข้อมูลลูกค้าออกจากข้อมูลสาธารณะ ตรวจสถานะความน่าเชื่อถือทุกครั้ง และอย่าสรุปเกินหลักฐาน', ': keep customer and public evidence distinct, check data status, and never conclude beyond the evidence.')}</p></div>
          <GuideSection id="portfolio" title={copy(language, '1. ตรวจรายการพื้นที่', '1. Review the site portfolio')} intro={copy(language, 'เริ่มจากจัดลำดับรายการที่ต้องติดตาม แล้วเปิดข้อมูลและหลักฐานก่อนกดวิเคราะห์', 'Prioritise sites that need follow-up, then inspect the record and evidence before starting an analysis.') } steps={[
            { title: copy(language, 'เปิดแดชบอร์ด', 'Open the dashboard'), body: copy(language, 'แอดมินเห็นรายการพื้นที่ทั้งหมด ส่วนฝ่ายขายเห็นพื้นที่ที่ตนดูแล ลูกค้าเห็นเฉพาะพื้นที่ของตน ใช้ตัวกรอง สถานะ การค้นหา และเรียงตามวันที่หรือคะแนนเพื่อจัดคิวงาน', 'Admins see the portfolio, sales users see assigned sites, and customers see their own. Use search, status filters, and date or score sorting to prioritise work.') },
            { title: copy(language, 'เปิดรายละเอียดพื้นที่', 'Open the site record'), body: copy(language, 'ตรวจชื่อ พิกัด ลิงก์ Google Maps ขนาดที่ดิน หน่วย และสถานะอินเทอร์เน็ต เทียบหมุดกับภาพให้แน่ใจว่าชี้พื้นที่ถูกแปลง พร้อมแจ้งทีมว่าการวางผังต้องตรวจให้ทางเข้า–ออกกว้างอย่างน้อย 7 เมตร', 'Check the name, coordinates, map link, area and unit, and internet status. Compare the pin with photos to ensure it is on the correct plot, and note that the layout review should confirm an entrance/exit of at least 7 m.') },
            { title: copy(language, 'ตรวจเอกสารและขอข้อมูลเพิ่ม', 'Check evidence and request missing details'), body: copy(language, 'เปิดรูปและ PDF ทีละรายการ ตรวจวันถ่าย มุมมอง ความชัด และข้อจำกัดของภาพ หากข้อมูลจำเป็นขาด ให้ติดต่อผู้ส่งผ่านช่องทางติดต่อที่ให้ไว้และบันทึกสิ่งที่ยังรอ', 'Review each photo and PDF for date, angle, clarity, and limitations. If essential details are missing, contact the submitter through the supplied channel and track what is pending.') },
          ]}/>
          <GuideSection id="run-analysis" title={copy(language, '2. เริ่มวิเคราะห์', '2. Run an analysis')} steps={[
            { title: copy(language, 'เลือกรัศมี', 'Choose a radius'), body: copy(language, 'จากหน้ารายละเอียดพื้นที่ เลือกระยะค้นข้อมูลรอบหมุด 1, 3 หรือ 5 กิโลเมตร ให้สอดคล้องกับรูปแบบทำเลและใช้รัศมีเดียวกันเมื่อต้องการเปรียบเทียบหลายพื้นที่', 'On the site page, choose a 1, 3, or 5 km evidence radius. Use the same radius when comparing similar sites.') },
            { title: copy(language, 'กด Run Analysis หนึ่งครั้ง', 'Select Run Analysis once'), body: copy(language, 'ระบบรับงานเข้าคิวและเปิดหน้าผลวิเคราะห์ สถานะ Pending หรือ Running หมายถึงกำลังทำงาน หน้าจะติดตามผลให้ ไม่ต้องกดซ้ำระหว่างรอ', 'The job enters a queue and opens the report. Pending or Running means work is underway. The page checks for updates; do not submit duplicate runs while waiting.') },
            { title: copy(language, 'ถ้างานล้มเหลว', 'If the run fails'), body: copy(language, 'ตรวจว่าพิกัดถูกต้องและแหล่งข้อมูลพร้อมใช้งาน จากนั้นเปิดพื้นที่เดิมแล้วเริ่มรอบใหม่ หากเกิดซ้ำให้บันทึกชื่อพื้นที่ เวลา และรหัสรอบวิเคราะห์เพื่อส่งตรวจระบบ', 'Check the coordinates and source availability, then start another run from the site record. If it repeats, record the site, time, and analysis ID for troubleshooting.') },
          ]}/>
          <section className="guide-section" id="read-report"><div className="guide-section-heading"><h2>{copy(language, '3. อ่านรายงานและตรวจหลักฐาน', '3. Read the report and evidence')}</h2><p>{copy(language, 'คะแนนรวมไม่ควรอ่านลำพัง ให้ดูรายละเอียดรายตัวชี้วัด แหล่งข้อมูล และสถานะกำกับด้วย', 'Read the overall score together with each metric, its source, and its data status.')}</p></div>
            <div className="guide-metric-list">{[
              [copy(language, 'จราจรและการเข้า–ออก', 'Traffic and road access'), copy(language, 'อาจแสดงจำนวนรถจากข้อมูลสำรวจทางการเมื่อจับคู่ถนนและจุดสำรวจได้ หรือแสดงศักยภาพจากประเภทถนนเมื่อไม่มีจำนวนรถตรงพื้นที่ ต้องอ่านป้ายกำกับให้ชัด', 'May show an official count when a nearby survey point matches, or mapped road potential when no count is available. Read the label carefully.')],
              [copy(language, 'รถ EV และประชากร', 'EV demand and population'), copy(language, 'ข้อมูลรถ BEV เป็นแนวโน้มระดับจังหวัด ส่วนประชากรและอาคารเป็นค่าประมาณตามรัศมีและชุดข้อมูล ไม่ใช่จำนวนลูกค้าของสถานี', 'BEV registration is a provincial trend. Population and buildings are area estimates, not station customers.')],
              [copy(language, 'สถานที่สำคัญและคู่แข่ง', 'Nearby places and competitors'), copy(language, 'ดูประเภทและจำนวนที่พบจากแผนที่พร้อมวันที่ข้อมูล รายการศูนย์ชาร์จที่นับได้ไม่ครอบคลุมทุกผู้ให้บริการเสมอ', 'Review mapped categories and retrieval date. Charging-station listings may not cover every operator.')],
              [copy(language, 'น้ำท่วมและระบบไฟฟ้า', 'Flood and electrical evidence'), copy(language, 'แผนที่ความเสี่ยงและแผนที่สายไฟเป็นหลักฐานระดับพื้นที่ ไม่ใช่การสำรวจระดับแปลง และไม่ยืนยันกำลังไฟที่เหลือหรือจุดเชื่อมต่อ', 'Flood layers and grid maps are area evidence, not a parcel survey. They do not confirm spare capacity or a feasible connection.')],
              [copy(language, 'สภาพพื้นที่จากภาพ', 'Image-based site conditions'), copy(language, 'AI อธิบายพื้นผิวที่เห็น น้ำขังที่สังเกตได้ สิ่งกีดขวาง และอาจประเมินช่วงความกว้างทางเข้าเมื่อมีสเกลเทียบที่พอเชื่อถือได้เท่านั้น', 'AI describes visible surface, signs of ponding, and obstructions. It may estimate an entrance-width range only when the visual scale is reliable.')],
            ].map(([title, description]) => <article key={title}><h3>{title}</h3><p>{description}</p></article>)}</div>
            <div className="guide-status-key"><h3>{copy(language, 'สถานะข้อมูล', 'Data status')}</h3><div><span><Check/>{copy(language, 'ตรวจสอบแล้ว', 'Verified')}</span><span><TriangleAlert/>{copy(language, 'ประมาณการ', 'Estimated')}</span><span><CircleAlert/>{copy(language, 'ประเมินเบื้องต้น', 'Preliminary')}</span><span><FileText/>{copy(language, 'ไม่มีข้อมูล', 'Missing')}</span></div></div>
          </section>
          <GuideSection id="recommendation" title={copy(language, '4. พิจารณาขนาดสถานีและตู้ชาร์จ', '4. Review station format and charger guidance')} steps={[
            { title: copy(language, 'รูปแบบ S / M / L', 'S / M / L format'), body: copy(language, 'รูปแบบพื้นที่อิงขนาดที่ดินรวมและเกณฑ์แฟรนไชส์ที่ระบบตั้งไว้: S เริ่ม 50 ตร.วา, M เริ่ม 200 ตร.วา, L เริ่ม 400 ตร.วา ระบบเลือกขนาดที่รองรับพื้นที่ ส่วนหน้ากว้างทางเข้าอย่างน้อย 7 เมตรเป็นคำแนะนำให้ตรวจหน้างาน', 'Format uses total land area and configured franchise thresholds: S starts at 50 sq wah, M at 200, and L at 400. The system selects the largest supported format. Treat the 7 m entrance guidance as a site check.') },
            { title: copy(language, 'คำแนะนำจำนวนตู้ DC', 'DC cabinet count'), body: copy(language, 'เมื่อระบบสร้างคำแนะนำจาก AI จะพิจารณาข้อมูลจราจร แนวโน้ม BEV ประชากร สถานที่สำคัญ คู่แข่ง ขนาดพื้นที่ และหลักฐานไฟฟ้าที่มีร่วมกัน โดยไม่เกินเพดานผัง S/M/L หากผลระบุ initial_phase หมายถึงระบบใช้จำนวนตามเพดานผังจากพื้นที่ ยังไม่ได้ประเมินอุปสงค์ด้วย AI จำนวนหมายถึงตู้จริง ไม่ใช่หัวชาร์จ', 'When AI generates the recommendation, it considers traffic, BEV trend, population, nearby places, competitors, land area, and available grid evidence within the S/M/L layout ceiling. If the result says initial_phase, the count follows the land-based layout ceiling and demand has not been assessed by AI. A cabinet is a physical charger, not a connector.') },
            { title: copy(language, 'กำลังไฟและการอนุมัติ', 'Power and approval'), body: copy(language, 'หากยังไม่มีการยืนยันจากการไฟฟ้า ให้ถือกำลังไฟเป็นแนวทางระบบ ไม่ใช่คำยืนยันว่าจะจ่ายไฟได้ ขนาดติดตั้งจริง จุดเชื่อมต่อ หม้อแปลง และงบงานระบบต้องให้วิศวกรตรวจและประสาน PEA/MEA ก่อนเสนออนุมัติ', 'Without utility confirmation, treat power as planning guidance, not available capacity. Engineers must verify installed power, connection point, transformer, and electrical cost with PEA/MEA before approval.') },
            { title: copy(language, 'สรุปและส่งต่อ', 'Summarise and follow up'), body: copy(language, 'บันทึกเหตุผล จุดแข็ง ความเสี่ยง สมมติฐาน และข้อมูลที่ต้องยืนยัน แล้วดาวน์โหลดรายงาน PDF เพื่อใช้ประชุม ห้ามสรุปรายได้หรือ ROI หากยังไม่มีข้อมูลต้นทุน ราคา และจำนวนการชาร์จจริง', 'Record rationale, strengths, risks, assumptions, and checks. Download the PDF for discussion. Do not state revenue or ROI without cost, price, and real charging-volume inputs.') },
          ]}/>
          <GuideSection id="team-accounts" title={copy(language, '5. บัญชีและสิทธิ์ทีมงาน', '5. Team accounts and permissions')} intro={copy(language, 'ผู้ใช้ super admin เท่านั้นที่จัดการบัญชีทีมงานจากหน้า Settings ได้', 'Only super admins can manage team accounts in Settings.') } steps={[
            { title: copy(language, 'เพิ่มผู้ใช้ทีม', 'Create a team account'), body: copy(language, 'เปิด Settings แล้วกรอกชื่อแสดง อีเมล รหัสผ่านอย่างน้อย 8 ตัว และเลือก Admin หรือ Sales ตรวจอีเมลให้ถูกก่อนสร้างบัญชี', 'Open Settings, enter display name, email, a password of at least 8 characters, then choose Admin or Sales. Check the email before creating the account.') },
            { title: copy(language, 'จัดการบัญชี', 'Manage accounts'), body: copy(language, 'แก้ชื่อ อีเมล บทบาท เปลี่ยนรหัสผ่าน ปิดใช้งาน หรือลบบัญชีได้ การลบบัญชีเป็นการลบถาวร ควรใช้ปิดใช้งานเมื่อเพียงต้องการระงับการเข้าถึง', 'Edit a name, email, role, password, deactivate, or delete an account. Deletion is permanent; deactivate an account when access only needs to be suspended.') },
            { title: copy(language, 'รักษาความลับข้อมูลลูกค้า', 'Protect customer information'), body: copy(language, 'ให้สิทธิ์เฉพาะคนที่ต้องใช้ข้อมูล เก็บเอกสารไว้ในระบบ ไม่ส่งลิงก์หรือไฟล์ลูกค้าไปยังบุคคลที่ไม่เกี่ยวข้อง และอย่านำข้อมูลส่วนบุคคลไปใส่ในบันทึก AI', 'Grant access only to staff who need it. Keep evidence in the system, do not share customer files externally, and do not include personal details in AI notes.') },
          ]}/>
          <div className="guide-callout guide-callout-warning"><Zap size={20}/><p><strong>{copy(language, 'ก่อนอนุมัติติดตั้ง', 'Before approving installation')}</strong>{copy(language, ' ต้องมีการสำรวจพื้นที่จริง ตรวจทางเข้าและการจราจร ออกแบบไฟฟ้าและหม้อแปลงโดยวิศวกร และยืนยันเงื่อนไขการเชื่อมต่อกับ PEA/MEA รายงานในระบบไม่ใช่ใบอนุญาต แบบวิศวกรรม หรือคำรับรองผลตอบแทน', ': survey the site, verify access and traffic flow, obtain engineering design for electrical works and transformer, and confirm connection conditions with PEA/MEA. The report is not a permit, engineering design, or return guarantee.')}</p></div>
          <p className="guide-access-note">{copy(language, 'คู่มือนี้อธิบายฟังก์ชันที่มีในระบบ ณ ปัจจุบัน เมนูที่มองเห็นขึ้นกับบทบาทบัญชีของคุณ', 'This guide describes current system functions. Available menus depend on your account role.')}</p>
        </>}
        <a className="guide-back-top" href="#top" onClick={event => { event.preventDefault(); window.scrollTo({ top: 0, behavior: 'smooth' }) }}><ArrowUp size={15}/>{copy(language, 'กลับด้านบน', 'Back to top')}</a>
      </article>
    </div>
  </div>
}
