import { ArrowRight, BarChart3, Building2, ChevronRight, ClipboardCheck, ExternalLink, MapPin, Maximize2, ShieldCheck, X, Zap } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { PublicHeader } from '../components/PublicHeader'
import { PublicReviewDetails } from '../components/PublicReviewDetails'
import { useI18n } from '../i18n/I18nProvider'
import { chargerModels, standaloneStationModels } from '../data/publicInvestment'

const copy = (language: 'th' | 'en', th: string, en: string) => language === 'th' ? th : en
function SectionTitle({ title, description }: { title: string; description?: string }) { return <div className="public-section-title"><h2>{title}</h2>{description ? <p>{description}</p> : null}</div> }

function ACChargerSpecifications({ language }: { language: 'th' | 'en' }) {
  const variants = [
    { id: 'ac-74-no-screen', power: '7.4 kW', screen: false, warranty: 3 },
    { id: 'ac-74-screen', power: '7.4 kW', screen: true, warranty: 5 },
    { id: 'ac-22-no-screen', power: '22 kW', screen: false, warranty: 3 },
    { id: 'ac-22-screen', power: '22 kW', screen: true, warranty: 5 },
  ]
  return <section id="ac-chargers" className="public-warranty" aria-labelledby="ac-chargers-title">
    <div className="public-warranty-heading"><div><Zap size={25}/><h3 id="ac-chargers-title">{copy(language, 'เครื่องชาร์จ AC', 'AC chargers')}</h3></div><p>{copy(language, 'AC 7.4 kW และ 22 kW แยกรุ่นมีจอและไม่มีจอ พร้อมการรับประกันของแต่ละรุ่น การเลือกใช้อุปกรณ์ขึ้นอยู่กับรถที่รองรับและระบบไฟของพื้นที่', 'AC 7.4 kW and 22 kW, with separate screen and no-screen variants and warranties. Equipment selection depends on vehicle compatibility and site electricity supply.')}</p></div>
    <div className="public-charger-specs">{variants.map(variant => <article className="public-charger-spec" key={variant.id} aria-labelledby={variant.id}>
      <header><h4 id={variant.id}>AC {variant.power}</h4><p>{copy(language, variant.screen ? 'รุ่นมีจอ' : 'รุ่นไม่มีจอ', variant.screen ? 'With-screen model' : 'No-screen model')}</p></header>
      <dl>
        <div><dt>{copy(language, 'ประเภท', 'Type')}</dt><dd>AC</dd></div>
        <div><dt>{copy(language, 'กำลังชาร์จ', 'Rated power')}</dt><dd>{variant.power}</dd></div>
        <div><dt>{copy(language, 'หัวชาร์จ', 'Connector')}</dt><dd>Type 2</dd></div>
        <div><dt>{copy(language, 'สายชาร์จ', 'Cable length')}</dt><dd>{copy(language, '5 เมตร', '5 metres')}</dd></div>
        <div className="public-ac-display"><dt>{copy(language, 'หน้าจอ', 'Display')}</dt><dd>{variant.screen ? copy(language, 'LCD 4.3 นิ้ว', '4.3-inch LCD') : variant.power === '22 kW' ? copy(language, 'ต้องยืนยันกับผู้จำหน่าย — เอกสารระบุขัดกัน', 'Supplier confirmation required — conflicting brochure details') : copy(language, 'ไม่มีจอ', 'No screen')}</dd></div>
        <div className="public-charger-warranty"><dt>{copy(language, 'รับประกันสินค้า', 'Product warranty')}</dt><dd>{variant.warranty} {copy(language, 'ปี', 'years')}</dd></div>
      </dl>
    </article>)}</div>
    <p className="public-ac-context-note">{copy(language, 'ข้อมูลตามเอกสารที่ได้รับ: รุ่น AC 22 kW ไม่มีจอ มีข้อความระบุ LCD 4.3 นิ้วในรายการสเปกด้วย จึงต้องยืนยันหน้าจอและรุ่นสินค้าก่อนสั่งซื้อ อุปกรณ์ AC ที่รวมในโครงการและเงื่อนไขรับประกันให้ยึดตามใบเสนอราคาและเอกสารรับประกันของรุ่นที่ส่งมอบ', 'Based on the supplied brochure: the AC 22 kW no-screen model also lists a 4.3-inch LCD, so confirm the display and exact model before ordering. AC equipment included in a project and warranty terms follow the quotation and the delivered model’s warranty documentation.')}</p>
  </section>
}

function ChargerSpecifications({ language }: { language: 'th' | 'en' }) {
  const groups = [
    {
      id: 'charger-compact-title', model: 'iGreen CSEVC3401E', power: 'DC 20 / 30 kW',
      description: copy(language, 'สำหรับพื้นที่ที่เหมาะกับเครื่องชาร์จกำลังต่ำ', 'For sites suited to lower-power DC charging'),
      specs: [
        [copy(language, 'ประเภท', 'Type'), 'DC'],
        [copy(language, 'กำลังชาร์จ', 'Rated power'), '20 / 30 kW'],
        [copy(language, 'แรงดันขาออก', 'Output voltage'), '150–1000 V DC'],
        [copy(language, 'กระแสสูงสุด', 'Maximum current'), '100 A'],
        [copy(language, 'สายชาร์จ', 'Cable length'), copy(language, '5 เมตร', '5 metres')],
        [copy(language, 'หน้าจอ', 'Display'), copy(language, 'LCD 4.3 นิ้ว', '4.3-inch LCD')],
      ],
      warranty: copy(language, '2 ปี ทั้งรุ่น 20 และ 30 kW', '2 years for both 20 and 30 kW models'),
      supply: copy(language, 'ใช้ระบบไฟร่วมกับร้านหรือบ้านได้เมื่อกำลังไฟรองรับ โดยแยกวงจรชาร์จและอุปกรณ์ป้องกันให้เหมาะสม วิศวกรและการไฟฟ้าต้องตรวจโหลดรวมก่อนติดตั้ง', 'May share the shop or home electricity supply when capacity is sufficient, with a dedicated charging circuit and suitable protection. Engineers and the utility must check the combined load before installation.'),
    },
    {
      id: 'charger-high-power-title', model: 'iGreen CSEVC3108E', power: 'DC 120 kW',
      description: copy(language, 'ตระกูลเครื่องชาร์จสองหัวสำหรับพื้นที่ที่ต้องการกำลังสูง', 'Dual-gun family for sites needing higher charging power'),
      specs: [
        [copy(language, 'ประเภท', 'Type'), 'DC'],
        [copy(language, 'กำลังชาร์จ', 'Rated power'), '120 kW'],
        [copy(language, 'แรงดันขาออก', 'Output voltage'), '150–1000 V DC'],
        [copy(language, 'กระแสสูงสุด', 'Maximum current'), copy(language, '200 A หรือ 100 A ต่อหัว ตามรุ่นย่อย', '200 A or 100 A per gun, by configuration')],
        [copy(language, 'สายชาร์จ', 'Cable length'), copy(language, '5 เมตร', '5 metres')],
        [copy(language, 'หน้าจอ', 'Display'), copy(language, 'LCD 7 นิ้ว (200 A) หรือ 4.3 นิ้ว (100 A)', '7-inch LCD (200 A) or 4.3-inch LCD (100 A)')],
      ],
      warranty: copy(language, '2 ปี ทั้งรุ่นจอ 7 นิ้วและ 4.3 นิ้ว', '2 years for both 7-inch and 4.3-inch configurations'),
      supply: copy(language, 'แนวทางโครงการ RBC: แยกหม้อแปลงสำหรับสถานีชาร์จออกจากระบบไฟร้านกาแฟ ขนาดหม้อแปลง จุดเชื่อมต่อ และงานที่รวมในราคายึดตามแบบวิศวกรรมและใบเสนอราคาโครงการ', 'RBC project approach: provide a dedicated charging-station transformer, separate from the café supply. Transformer sizing, connection point, and works included in the price follow the engineering design and project quotation.'),
    },
  ]

  return <><section className="public-warranty" aria-labelledby="igreen-warranty-title">
    <div className="public-warranty-heading"><div><ShieldCheck size={25}/><h3 id="igreen-warranty-title">{copy(language, 'เครื่องชาร์จ DC ที่ใช้ในโครงการ', 'DC chargers used in projects')}</h3></div><p>{copy(language, 'สองกลุ่มเครื่องชาร์จที่เลือกใช้: DC 20/30 kW และ DC 120 kW โดยแยกสเปกและการรับประกันให้เห็นชัดในแต่ละกลุ่ม กำลังติดตั้งจริงต้องตรวจจากพื้นที่', 'Two selected charger groups: DC 20/30 kW and DC 120 kW. Each shows its specifications and warranty separately. Final installed power requires a site review.')}</p></div>
    <div className="public-charger-specs">{groups.map(group => <article className="public-charger-spec" key={group.id} aria-labelledby={group.id}>
      <header><span>{group.model}</span><h4 id={group.id}>{group.power}</h4><p>{group.description}</p></header>
      <dl>{group.specs.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}<div className="public-charger-warranty"><dt>{copy(language, 'รับประกันสินค้า', 'Product warranty')}</dt><dd>{group.warranty}</dd></div></dl>
      <div className="public-charger-supply"><h5>{copy(language, 'การขอไฟฟ้าและระบบจ่ายไฟ', 'Electricity connection and supply')}</h5><p>{group.supply}</p></div>
    </article>)}</div>
    <div className="public-ac-context"><h4>{copy(language, 'AC Charging สำหรับมอเตอร์ไซค์ไฟฟ้า', 'AC charging for electric motorcycles')}</h4><p>{copy(language, 'AC เป็นอีกทางเลือกสำหรับมอเตอร์ไซค์ไฟฟ้ารุ่นที่รองรับ โดยต้องตรวจหัวต่อและระบบชาร์จของรถแต่ละรุ่นก่อน ไม่ได้จำกัดเฉพาะมอเตอร์ไซค์ รถยนต์ไฟฟ้าที่รองรับก็ใช้ AC ได้ ส่วนเครื่องที่แสดงในโครงการนี้เป็น DC 20/30 kW และ DC 120 kW', 'AC is another option for compatible electric motorcycles. Check each model’s connector and charging system first. AC also serves compatible electric cars; the project chargers shown here are DC 20/30 kW and DC 120 kW.')}</p></div>
    <div className="public-disclosure"><ShieldCheck size={22}/><p>{copy(language, 'รุ่นย่อย หัวต่อ และรายละเอียดอุปกรณ์ที่ส่งมอบให้ยึดตามใบเสนอราคาโครงการ กำลังติดตั้งจริงและขนาดหม้อแปลงต้องตรวจสอบจากพื้นที่โดยวิศวกรและการไฟฟ้า', 'The project quotation governs the exact configuration, connector, and delivered equipment. Engineers and the utility must confirm installed power and transformer sizing from the actual site.')}</p><a href="https://www.igreenplus.co.th/" target="_blank" rel="noreferrer">{copy(language, 'อ้างอิงข้อมูลผู้ผลิต iGreen', 'Reference iGreen manufacturer information')}<ExternalLink size={15}/></a></div>
  </section><ACChargerSpecifications language={language}/></>
}

export function InvestmentPage({ authenticated = false, accountName, onLogout }: { authenticated?: boolean; accountName?: string; onLogout?: () => void }) {
  const { language } = useI18n()
  const [selectedFormat, setSelectedFormat] = useState<(typeof chargerModels)[number] | (typeof standaloneStationModels)[number] | null>(null)
  const investmentLink = authenticated ? '/sites/new' : '/login?mode=register'
  useEffect(() => {
    if (!selectedFormat) return
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === 'Escape') setSelectedFormat(null) }
    window.addEventListener('keydown', closeOnEscape)
    return () => { document.body.style.overflow = previousOverflow; window.removeEventListener('keydown', closeOnEscape) }
  }, [selectedFormat])

  return <main className="landing-page public-site public-investment-page">
    <PublicHeader authenticated={authenticated} accountName={accountName} onLogout={onLogout} />
    <section id="investment" className="public-investment"><div className="public-investment-copy"><SectionTitle title={copy(language, 'ลงทุนอย่างมีข้อมูล ไม่ใช่จากการคาดเดา', 'Invest with evidence, not guesswork')} description={copy(language, 'RBC EV Station ช่วยจัดระเบียบข้อมูลทำเลให้เป็นภาพตัดสินใจที่ชัดเจน ก่อนเข้าสู่ขั้นตอนสำรวจและพิจารณาโครงการ', 'RBC EV Station turns a prospective location into a clear decision picture before site inspection and project review.')}/><div className="public-investment-actions"><Link className="landing-cta" to={investmentLink}>{copy(language, 'เริ่มประเมินทำเล', 'Start site assessment')}<ArrowRight size={18}/></Link><Link className="landing-outline" to="/#contact">{copy(language, 'ติดต่อสอบถามทาง LINE', 'Contact us on LINE')}<ChevronRight size={18}/></Link></div></div><ol className="public-process" aria-label={copy(language, 'ขั้นตอนการลงทุน', 'Investment process')}><li><MapPin size={22}/><div><strong>{copy(language, 'ส่งทำเล', 'Submit a site')}</strong><span>{copy(language, 'พิกัด ภาพ และข้อมูลพื้นที่', 'Coordinates, photos, and site details')}</span></div></li><li><BarChart3 size={22}/><div><strong>{copy(language, 'วิเคราะห์เบื้องต้น', 'Review the evidence')}</strong><span>{copy(language, 'ทำเล การเข้าถึง และข้อมูลประกอบ', 'Location, access, and supporting data')}</span></div></li><li><Building2 size={22}/><div><strong>{copy(language, 'พิจารณาโครงการ', 'Assess the project')}</strong><span>{copy(language, 'ระบบไฟฟ้า รูปแบบสถานี และความเหมาะสมเชิงธุรกิจ', 'Power, station format, and commercial fit')}</span></div></li><li><Zap size={22}/><div><strong>{copy(language, 'ติดตั้งและเปิดใช้บริการ', 'Install and open')}</strong><span>{copy(language, 'หลังอนุมัติโครงการ ใช้เวลาติดตั้งโดยประมาณ 3–6 เดือน ขึ้นอยู่กับหน้างานและการประสานงานที่เกี่ยวข้อง', 'After approval, installation typically takes around 3–6 months, depending on site conditions and required coordination.')}</span></div></li><li><ClipboardCheck size={22}/><div><strong>{copy(language, 'ติดตามผล', 'Track the outcome')}</strong><span>{copy(language, 'ทุกขั้นตอนในบัญชี RBC EV Station', 'Every step in your RBC EV Station account')}</span></div></li></ol></section>
    <PublicReviewDetails />
    <section id="chargers" className="public-chargers"><SectionTitle title={copy(language, 'Charging Solutions ของ RBC EV Station', 'RBC EV Station charging solutions')} description={copy(language, 'เลือกรูปแบบ S / M / L จากขนาดพื้นที่ เป้าหมายธุรกิจ และจำนวนจุดชาร์จที่ต้องการ โดยตัวเลขด้านล่างเป็นกรอบประมาณการเบื้องต้น', 'Choose an S / M / L format around site size, business goals, and charging capacity. The figures below are preliminary estimates.')}/><div className="public-model-grid">{chargerModels.map(model => <article key={model.code} className="public-model"><div className="public-model-top"><span className="public-format-code">{model.code}</span><span>{copy(language, 'รูปแบบโครงการ', 'Project format')}</span></div><button type="button" className="public-model-image-button" onClick={() => setSelectedFormat(model)} aria-label={copy(language, 'ดูภาพผัง ' + model.nameTh + ' ขนาดใหญ่', 'View the ' + model.nameEn + ' plan in full size')}><img src={language === 'en' ? model.imageEn ?? model.image : model.image} alt={copy(language, model.imageAlt.th, model.imageAlt.en)} loading="lazy"/><span><Maximize2 size={17}/>{copy(language, 'ดูภาพขนาดใหญ่', 'View full size')}</span></button><h3>{copy(language, model.nameTh, model.nameEn)}</h3><div className="public-model-features"><span>{copy(language, 'สิ่งที่มีในโครงการ', 'Included in this format')}</span><ul>{model.features.map(feature => <li key={feature.th}>{copy(language, feature.th, feature.en)}</li>)}</ul></div><dl><div><dt>{copy(language, 'พื้นที่แนะนำ', 'Suggested area')}</dt><dd>{copy(language, model.area.th, model.area.en)}</dd></div><div><dt>{copy(language, 'EV Charging', 'EV Charging')}</dt><dd>{copy(language, model.chargers.th, model.chargers.en)}</dd></div><div><dt>{copy(language, 'งบลงทุนโดยประมาณ', 'Indicative investment')}</dt><dd>{copy(language, model.investment.th, model.investment.en)}</dd></div><div><dt>{copy(language, 'ค่าแฟรนไชส์เริ่มต้น', 'Starting franchise fee')}</dt><dd>{copy(language, model.franchise.th, model.franchise.en)}</dd></div></dl></article>)}</div><p className="public-format-note">{copy(language, 'งบลงทุนและค่าแฟรนไชส์เป็นข้อมูลประมาณการจากรูปแบบโครงการ ตัวเลขจริงขึ้นอยู่กับแบบก่อสร้าง อุปกรณ์ งานระบบไฟฟ้า เงื่อนไขเชิงพาณิชย์ และใบเสนอราคาที่ RBC อนุมัติ', 'Investment and franchise figures are project-format estimates. Final amounts depend on construction design, equipment, electrical works, commercial terms, and the RBC-approved quotation.')}</p><ChargerSpecifications language={language}/></section>
    <section id="standalone-stations" className="public-standalone-stations"><SectionTitle title={copy(language, 'EV Charging Station แบบเดี่ยว', 'Standalone EV Charging Stations')} description={copy(language, 'รูปแบบสถานีชาร์จสำหรับพื้นที่ที่ต้องการให้บริการชาร์จ EV โดยเฉพาะ ไม่มีร้านกาแฟหรือธุรกิจบริการอื่นรวมอยู่ในโครงการ', 'Charging-station formats for sites focused solely on EV charging, with no café or other destination-service components.')}/><div className="public-model-grid">{standaloneStationModels.map(model => <article key={model.code} className="public-model public-standalone-model"><div className="public-model-top"><span className="public-format-code">{model.code}</span><span>{copy(language, 'สถานีชาร์จแบบเดี่ยว', 'Standalone station')}</span></div><button type="button" className="public-model-image-button" onClick={() => setSelectedFormat(model)} aria-label={copy(language, 'ดูภาพสถานี ' + model.code + ' ขนาดใหญ่', 'View the ' + model.code + ' station in full size')}><img src={language === 'en' ? model.imageEn ?? model.image : model.image} alt={copy(language, model.imageAlt.th, model.imageAlt.en)} loading="lazy"/><span><Maximize2 size={17}/>{copy(language, 'ดูภาพขนาดใหญ่', 'View full size')}</span></button><h3>{copy(language, model.nameTh, model.nameEn)}</h3><div className="public-model-features"><span>{copy(language, 'องค์ประกอบสถานี', 'Station configuration')}</span><ul><li>{copy(language, 'สถานีชาร์จ iGreen DC 120 kW', 'iGreen DC 120 kW charging station')}</li><li>{copy(language, model.chargers.th, model.chargers.en)}</li><li>{copy(language, model.parking.th, model.parking.en)}</li></ul></div><dl><div><dt>{copy(language, 'พื้นที่แนะนำ', 'Suggested area')}</dt><dd>{copy(language, model.area.th, model.area.en)}</dd></div><div><dt>{copy(language, 'EV Charging', 'EV Charging')}</dt><dd>{copy(language, model.chargers.th, model.chargers.en)}</dd></div><div><dt>{copy(language, 'ช่องจอด EV', 'EV parking bays')}</dt><dd>{copy(language, model.parking.th, model.parking.en)}</dd></div><div><dt>{copy(language, 'งานระบบ', 'Electrical works')}</dt><dd>{copy(language, model.electrical.th, model.electrical.en)}</dd></div></dl></article>)}</div><p className="public-format-note">{copy(language, 'ภาพและผังเป็นแนวคิดประกอบการนำเสนอ ขนาดจริง ตำแหน่งหม้อแปลง ระบบไฟฟ้า และระยะปลอดภัย ต้องยืนยันจากการสำรวจหน้างานและวิศวกรผู้ออกแบบ', 'The visuals and plans are presentation concepts. Final dimensions, transformer placement, electrical design, and safety clearances require site survey and engineer confirmation.')}</p></section>
    {selectedFormat ? <div className="public-image-modal" role="presentation" onMouseDown={() => setSelectedFormat(null)}><section className="public-image-modal-dialog" role="dialog" aria-modal="true" aria-labelledby="format-image-title" onMouseDown={event => event.stopPropagation()}><div className="public-image-modal-head"><div><span>{copy(language, 'รูปแบบโครงการ', 'Project format')} {selectedFormat.code}</span><h2 id="format-image-title">{copy(language, selectedFormat.nameTh, selectedFormat.nameEn)}</h2></div><button type="button" onClick={() => setSelectedFormat(null)} aria-label={copy(language, 'ปิดภาพขนาดใหญ่', 'Close full-size image')}><X size={24}/></button></div><img src={language === 'en' ? selectedFormat.imageEn ?? selectedFormat.image : selectedFormat.image} alt={copy(language, selectedFormat.imageAlt.th, selectedFormat.imageAlt.en)}/></section></div> : null}
  </main>
}
