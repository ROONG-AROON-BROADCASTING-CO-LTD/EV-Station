import type { FranchisePlan, Site } from '../../types/domain'

export function toSquareWah(value?: number, unit?: string) {
  if (value === undefined) return undefined
  if (unit === 'sqm') return value / 4
  if (unit === 'rai') return value * 400
  if (unit === 'ngan') return value * 100
  return unit === 'sqwah' ? value : undefined
}

export function selectProjectFormat(plans: FranchisePlan[] | undefined, site: Site | undefined) {
  const area = toSquareWah(site?.landSize, site?.landSizeUnit)
  const frontage = site?.frontageMeters
  const minimumFrontageMeters = 7
  const widthMissing = frontage === undefined
  const widthInsufficient = frontage !== undefined && frontage < minimumFrontageMeters
  const selected = (plans ?? []).filter(plan => area !== undefined && area >= plan.minimumAreaSqWah).sort((first, second) => second.minimumAreaSqWah - first.minimumAreaSqWah)[0]
  return { area, frontage, minimumFrontageMeters, widthMissing, widthInsufficient, selected, canRecommend: Boolean(selected) }
}
