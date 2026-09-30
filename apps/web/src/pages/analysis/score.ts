export function getScoreGrade(score?: number | null) {
  if (score == null || !Number.isFinite(score)) return undefined
  if (score >= 80) return 'A'
  if (score >= 70) return 'B'
  if (score >= 60) return 'C'
  return undefined
}
