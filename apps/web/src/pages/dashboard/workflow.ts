import { Check, FileText, Phone, TrendingUp } from 'lucide-react'
import type { AnalysisRun, Site } from '../../types/domain'

export type WorkflowStage = 'submitted' | 'documents' | 'analysing' | 'ready'

export const WORKFLOW: Array<{
  id: WorkflowStage
  icon: typeof Check
  th: string
  en: string
  tone: string
  activeTone: string
}> = [
  { id: 'submitted', icon: Check, th: 'ส่งข้อมูลแล้ว', en: 'Submitted', tone: 'text-emerald-600', activeTone: 'bg-emerald-50 ring-emerald-200' },
  { id: 'documents', icon: FileText, th: 'ต้องตรวจเพิ่ม', en: 'Needs review', tone: 'text-orange-500', activeTone: 'bg-orange-50 ring-orange-200' },
  { id: 'analysing', icon: TrendingUp, th: 'กำลังวิเคราะห์', en: 'Analysing', tone: 'text-blue-600', activeTone: 'bg-blue-50 ring-blue-200' },
  { id: 'ready', icon: Phone, th: 'พร้อมติดตาม', en: 'Ready to follow up', tone: 'text-emerald-600', activeTone: 'bg-emerald-50 ring-emerald-200' },
]

export function stageFor(site: Site, run?: AnalysisRun | null): WorkflowStage {
  if (site.inputStatus === 'missing') return 'documents'
  if (!run) return 'submitted'
  if (run.status === 'pending' || run.status === 'running') return 'analysing'
  if (run.status === 'failed') return 'documents'
  return 'ready'
}

export function copy(language: 'th' | 'en', th: string, en: string) {
  return language === 'th' ? th : en
}

export function formatDate(value: string, language: 'th' | 'en') {
  return new Date(value).toLocaleDateString(language === 'th' ? 'th-TH' : 'en-GB', {
    day: 'numeric',
    month: 'short',
    year: language === 'th' ? 'numeric' : '2-digit',
  })
}

export function locationText(site: Site, language: 'th' | 'en') {
  if (site.address) return site.address
  if (site.latitude !== undefined && site.longitude !== undefined) return `${site.latitude.toFixed(5)}, ${site.longitude.toFixed(5)}`
  return copy(language, 'ยังไม่มีตำแหน่ง', 'Location pending')
}
