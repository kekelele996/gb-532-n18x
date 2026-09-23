export type GapSeverity = 'minor' | 'major' | 'critical'
export type GapState = 'detected' | 'reviewed' | 'accepted' | 'false_positive' | 'resurveyed' | 'closed'
export const GAP_SEVERITY_LABEL: Record<GapSeverity,string> = { minor: '轻微', major: '主要', critical: '严重' }
export const GAP_STATE_LABEL: Record<GapState,string> = { detected:'待复核', reviewed:'已复核', accepted:'接受补测', false_positive:'误报', resurveyed:'已补测', closed:'已关闭' }
/** 可参与多快照补测调度的缺口状态：待复核或已复核。 */
export const RESURVEY_SELECTABLE_STATES: GapState[] = ['detected', 'reviewed']
export const isResurveySelectable = (state: GapState): boolean => RESURVEY_SELECTABLE_STATES.includes(state)
export const GAP_SEVERITY_ORDER: Record<GapSeverity,number> = { critical: 3, major: 2, minor: 1 }
