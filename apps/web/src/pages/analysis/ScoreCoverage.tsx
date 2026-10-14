import { useI18n } from '../../i18n/I18nProvider'
import type { AnalysisRun } from '../../types/domain'

const names: Record<string, [string, string]> = {
  traffic: ['การจราจร', 'Traffic'], road_accessibility: ['ทางเข้า–ออก', 'Road access'],
  ev_demand: ['รถ EV ระดับจังหวัด', 'Provincial EV registrations'], population: ['ประชากร', 'Population'],
  poi: ['สถานที่รอบพื้นที่', 'Nearby places'], competition: ['คู่แข่ง', 'Competition'],
  flood: ['น้ำท่วม', 'Flood history'], electrical: ['ระบบไฟฟ้า', 'Electrical readiness'],
  site_requirements: ['ข้อมูลอินเทอร์เน็ตหน้างาน', 'On-site internet'],
}

export function ScoreCoverage({ scoring }: { scoring: AnalysisRun['scoring'] }) {
  const { language } = useI18n()
  if (!scoring) return null
  const th = language === 'th'
  const excluded = scoring.excludedMetrics ?? []
  return <div className="mt-5 border-t border-line pt-4 text-sm leading-relaxed">
    <h3 className="font-semibold">{th ? 'ข้อมูลที่ใช้คำนวณคะแนน' : 'Evidence used for this score'}</h3>
    <p className="mt-1">{th
      ? `${scoring.scoredMetricCount} จาก ${scoring.requiredMetricCount} ด้าน · ครอบคลุมน้ำหนักคะแนน ${scoring.coveragePercentage.toFixed(1)}%`
      : `${scoring.scoredMetricCount} of ${scoring.requiredMetricCount} factors · ${scoring.coveragePercentage.toFixed(1)}% of scoring weight covered`}</p>
    <p className="mt-2 text-muted">{th
      ? `ต้องมีข้อมูลอย่างน้อย ${scoring.minimumCoveragePercentage}% ของน้ำหนักคะแนนจึงแสดงคะแนนรวม เปอร์เซ็นต์นี้บอกความครบถ้วน ไม่ใช่ความแม่นยำหรือโอกาสสำเร็จของการลงทุน`
      : `An overall score requires at least ${scoring.minimumCoveragePercentage}% of scoring weight. Coverage measures completeness, not accuracy or investment success.`}</p>
    {excluded.length > 0 && <>
      <p className="mt-3 font-medium">{th ? 'ยังไม่รวมในคะแนน:' : 'Excluded from the score:'}</p>
      <ul className="mt-1 list-disc space-y-1 pl-5">{excluded.map(key => <li key={key}>{names[key]?.[th ? 0 : 1] ?? key}</li>)}</ul>
      <p className="mt-3 text-muted">{th
        ? 'ตรวจเหตุผลและแหล่งข้อมูลในตารางด้านล่าง ข้อมูลที่ขาดยังต้องตรวจเพิ่ม โดยเฉพาะน้ำท่วม ทางเข้า–ออก และไฟฟ้าหน้างาน ก่อนตัดสินใจลงทุน'
        : 'Review the reasons and sources below. Missing evidence needs follow-up, especially flooding, road access and on-site electricity, before an investment decision.'}</p>
    </>}
  </div>
}
