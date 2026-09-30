import type { AnalysisRun, Site, UserRole } from '../../types/domain'

export function getSiteInspectorModel(site: Site, run: AnalysisRun | null | undefined, role: UserRole) {
  const score = Math.round(run?.overallScore ?? 0)
  return {
    score,
    destination: run ? `/analysis/${run.id}` : `/sites/${site.id}`,
    canEdit: role === 'super_admin' || role === 'admin' || role === 'sales',
    canDelete: role === 'super_admin',
    readinessTone: score >= 60 ? '#009b68' : score >= 45 ? '#f1b63d' : '#ff7a59',
    checklist: [
      { done: Boolean(site.contactName && site.contactPhone), th: 'ข้อมูลผู้ติดต่อ', en: 'Contact details' },
      { done: site.latitude !== undefined && site.longitude !== undefined, th: 'ตำแหน่งพื้นที่', en: 'Site location' },
      { done: run?.status === 'completed', th: 'ผลคัดกรองเบื้องต้น', en: 'Screening result' },
    ],
  }
}
