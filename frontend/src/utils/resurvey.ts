import type { CoverageGap } from '../types/coverage-gap'
import { GAP_SEVERITY_ORDER, isResurveySelectable } from '../types/enums/gap-severity'

export interface ResurveySelectionIssue {
  code: 'TOO_FEW' | 'MULTI_AREA' | 'STATE_INVALID'
  message: string
  gapIds: number[]
}

/**
 * 多快照补测调度的前端预校验。最终裁决仍以后端为准：
 * 至少 2 条、同一测区、全部为待复核或已复核。
 */
export function validateResurveySelection(gaps: CoverageGap[]): ResurveySelectionIssue | null {
  if (gaps.length < 2) {
    return { code: 'TOO_FEW', message: '请至少勾选同一测区的 2 条待复核或已复核快照。', gapIds: [] }
  }
  const areaIds = Array.from(new Set(gaps.map(gap => gap.survey_area_id)))
  if (areaIds.length > 1) {
    return { code: 'MULTI_AREA', message: '只能选择同一测区的快照，含其他测区时整次拒绝。', gapIds: areaIds }
  }
  const invalidIds = gaps.filter(gap => !isResurveySelectable(gap.gap_state)).map(gap => gap.id)
  if (invalidIds.length > 0) {
    return { code: 'STATE_INVALID', message: `仅待复核或已复核快照可排入补测，已关闭等不合规快照：#${invalidIds.join('、#')}，整次拒绝。`, gapIds: invalidIds }
  }
  return null
}

export interface ResurveySortKey {
  gap: CoverageGap
  severityRank: number
}

/** 与后端一致的确定性排序：严重度降序 -> 缺口面积降序 -> 发现时间升序 -> ID 升序。 */
export function orderGapsForResurvey(gaps: CoverageGap[]): CoverageGap[] {
  return [...gaps].sort((left, right) => {
    const rankDiff = GAP_SEVERITY_ORDER[right.severity] - GAP_SEVERITY_ORDER[left.severity]
    if (rankDiff !== 0) return rankDiff
    if (left.area_square_m !== right.area_square_m) return right.area_square_m - left.area_square_m
    const timeDiff = new Date(left.detected_at).getTime() - new Date(right.detected_at).getTime()
    if (timeDiff !== 0) return timeDiff
    return left.id - right.id
  })
}

export const formatMetres = (value: number): string =>
  value >= 1000 ? `${(value / 1000).toFixed(2)} km` : `${value.toFixed(1)} m`

export const formatSquareMetres = (value: number): string =>
  value >= 10000 ? `${(value / 10000).toFixed(2)} ha` : `${value.toFixed(1)} m²`
