import type { CoverageGap } from '../types/coverage-gap'
import { GAP_SCHEDULABLE_STATES } from '../types/enums/gap-severity'

export type ResurveySelectionIssue = 'too_few' | 'duplicated' | 'cross_area' | 'state_invalid'

export const RESURVEY_ISSUE_LABEL: Record<ResurveySelectionIssue,string> = {
  too_few: '至少勾选两条快照',
  duplicated: '选择包含重复快照',
  cross_area: '选择跨越多个测区',
  state_invalid: '仅待复核（已检测）或已复核的快照可调度'
}

export function resurveyIdempotencyKey(gapIDs:number[]):string{
  return `resurvey-${[...gapIDs].sort((a,b)=>a-b).join('-')}`
}

export function validateResurveySelection(gaps:CoverageGap[], selectedIDs:number[]):ResurveySelectionIssue[]{
  const issues=new Set<ResurveySelectionIssue>()
  if(selectedIDs.length<2)issues.add('too_few')
  if(new Set(selectedIDs).size!==selectedIDs.length)issues.add('duplicated')
  const selected=gaps.filter(gap=>selectedIDs.includes(gap.id))
  if(new Set(selected.map(gap=>gap.survey_area_id)).size>1)issues.add('cross_area')
  if(selected.some(gap=>!GAP_SCHEDULABLE_STATES.includes(gap.gap_state)))issues.add('state_invalid')
  return [...issues]
}
