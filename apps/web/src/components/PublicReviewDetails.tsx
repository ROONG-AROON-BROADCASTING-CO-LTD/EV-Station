import { ArrowDownRight, BarChart3, ClipboardList, MapPinned, ShieldCheck, Zap } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useI18n } from '../i18n/I18nProvider'

const copy = (language: 'th' | 'en', th: string, en: string) => language === 'th' ? th : en

export function PublicReviewDetails() {
  const { language } = useI18n()
  const topics = [
    {
      icon: MapPinned,
      title: copy(language, 'ทำเลและการเดินทาง', 'Location and access'),
      description: copy(language, 'ตรวจบริบทถนนและทางเข้าออก สถานที่สำคัญ ชุมชน และสถานีชาร์จที่พบในแผนที่ พร้อมข้อมูลจราจรจากแหล่งที่มีข้อมูลใกล้พื้นที่', 'Reviews mapped roads and access, nearby places and communities, mapped charging stations, and traffic evidence where a source covers the area.'),
      note: copy(language, 'จำนวนสถานที่บนแผนที่ไม่ใช่จำนวนผู้ใช้จริง และข้อมูลจราจรบางพื้นที่เป็นศักยภาพจากประเภทถนน ไม่ใช่จำนวนรถที่นับได้', 'Mapped place counts are not actual customers. Some traffic results describe road potential rather than a measured vehicle count.'),
    },
    {
      icon: BarChart3,
      title: copy(language, 'โอกาสทางธุรกิจ', 'Business opportunity'),
      description: copy(language, 'พิจารณาคะแนนและหลักฐานด้านจราจร รถ BEV จดทะเบียนระดับจังหวัด ประชากร สถานที่สำคัญ และคู่แข่ง EV เพื่ออธิบายจุดแข็งกับประเด็นที่ควรตรวจเพิ่ม', 'Uses scores and available evidence on traffic, provincial BEV registrations, population, nearby destinations, and EV competitors to describe strengths and follow-up questions.'),
      note: copy(language, 'รถ BEV และประชากรเป็นข้อมูลภาพรวม ไม่ได้แปลว่ามีผู้ใช้หรือผู้ชาร์จเท่ากับตัวเลขนั้นในทำเลนี้', 'Provincial BEV and population data are context, not a count of customers or charging sessions at this site.'),
    },
    {
      icon: Zap,
      title: copy(language, 'ระบบไฟฟ้าและสภาพพื้นที่', 'Power and visible site conditions'),
      description: copy(language, 'แสดงหลักฐานแนวสายไฟหรือสถานีไฟฟ้าจากข้อมูลที่เผยแพร่ และให้ AI ช่วยอ่านสภาพพื้นผิว น้ำขังที่มองเห็น และสิ่งกีดขวางจากรูปที่ส่งมา', 'Shows published grid and substation evidence, while AI reviews visible ground surface, signs of ponding, and obstructions in submitted images.'),
      note: copy(language, 'ข้อมูลแผนที่ไฟฟ้าไม่ยืนยันกำลังไฟคงเหลือ ส่วนภาพถ่ายไม่สามารถตรวจดินใต้ผิวดินหรือรับรองแนวเขตที่ดินได้', 'Published electricity maps do not confirm spare capacity. Photos cannot verify underground soil conditions or legal parcel boundaries.'),
    },
    {
      icon: ClipboardList,
      title: copy(language, 'ขนาดโครงการและจำนวนตู้ DC', 'Project format and DC cabinet count'),
      description: copy(language, 'รูปแบบ S / M / L แสดงเพดานการวางผังตามขนาดที่ดิน ส่วนคำแนะนำจำนวนตู้ DC ใช้หลักฐานด้านทำเลและความต้องการร่วมกับเพดานผังนั้น', 'S / M / L describes the layout ceiling based on land area. The DC cabinet recommendation then considers location and demand evidence within that ceiling.'),
      note: copy(language, 'จำนวนที่แนะนำยังต้องตรวจผังจอดรถ การไหลของรถ กำลังไฟ จุดเชื่อมต่อ หม้อแปลง และแบบโดยวิศวกรกับ PEA/MEA', 'The proposed count still needs confirmation against parking layout, vehicle flow, supply capacity, connection point, transformer, and engineering review with PEA/MEA.'),
    },
  ]

  return <section className="public-review-details" aria-labelledby="review-details-title">
    <div className="public-review-details-head">
      <div>
        <h2 id="review-details-title">{copy(language, 'รายงานทำเลช่วยตอบอะไรได้บ้าง', 'What the site report helps answer')}</h2>
        <p>{copy(language, 'ระบบรวบรวมหลักฐานหลายด้าน เพื่อให้ทีมและเจ้าของพื้นที่เห็นภาพเดียวกันก่อนนัดสำรวจและวางแผนโครงการ', 'The report brings several evidence sources together so the owner and RBC team can discuss the same picture before a site visit and project plan.')}</p>
      </div>
      <ShieldCheck size={32} aria-hidden="true" />
    </div>
    <div className="public-review-details-list">
      {topics.map(({ icon: Icon, title, description, note }) => <article key={title}>
        <span className="public-review-details-icon"><Icon size={22} /></span>
        <div><h3>{title}</h3><p>{description}</p><small>{note}</small></div>
      </article>)}
    </div>
    <div className="public-review-details-foot"><p><strong>{copy(language, 'ผลคัดกรองใช้ประกอบการพิจารณา ไม่ใช่การรับรองรายได้หรือผลตอบแทน', 'Screening supports a decision; it does not guarantee revenue or returns.')}</strong> {copy(language, 'คะแนนอาจเปลี่ยนตามแหล่งข้อมูล วันที่ข้อมูล และหลักฐานที่ลูกค้าส่งเพิ่ม', 'Scores can change with source coverage, data dates, and additional customer evidence.')}</p><Link to="/guide/customer">{copy(language, 'ดูคู่มือเตรียมข้อมูลและส่งทำเล', 'Read the site-submission guide')}<ArrowDownRight size={17}/></Link></div>
  </section>
}
