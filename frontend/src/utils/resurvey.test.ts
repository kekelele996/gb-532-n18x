import { describe, expect, it } from 'vitest'
import type { Position } from '../types/api'
import type { CoverageGap } from '../types/coverage-gap'
import { formatMetres, formatSquareMetres, orderGapsForResurvey, validateResurveySelection } from './resurvey'

const ring: Position[][] = [[[0, 0], [1, 0], [1, 1], [0, 1], [0, 0]]]
const polygonFeature = () => ({
  type: 'Feature' as const,
  properties: {},
  geometry: { type: 'Polygon' as const, coordinates: ring },
})

function gap(id:number, overrides:Partial<CoverageGap>={}):CoverageGap{
  return {
    id, survey_area_id: 1, source_run_ids: [1], gap_geojson: polygonFeature(),
    area_square_m: 100, gap_ratio: 0.1, severity: 'major', recommended_line_geojson: polygonFeature(),
    algorithm_version: 'grid-cover-v1.0.0', input_hash: `hash-${id}`, gap_state: 'detected',
    explanation: '', coverage_ratio: 0.9, overlap_ratio: 0, processing_millis: 1, version: 1,
    detected_at: '2026-09-01T08:00:00Z', updated_at: '2026-09-01T08:00:00Z', ...overrides,
  }
}

describe('多快照补测调度选择校验', () => {
  it('少于两条快照时拒绝', () => {
    expect(validateResurveySelection([gap(1)])?.code).toBe('TOO_FEW')
    expect(validateResurveySelection([])?.code).toBe('TOO_FEW')
  })
  it('跨测区选择整次拒绝', () => {
    const issue = validateResurveySelection([gap(1), gap(2, { survey_area_id: 2 })])
    expect(issue?.code).toBe('MULTI_AREA')
  })
  it('含已关闭或其他非待复核/已复核状态时整次拒绝', () => {
    expect(validateResurveySelection([gap(1), gap(2, { gap_state: 'closed' })])?.code).toBe('STATE_INVALID')
    expect(validateResurveySelection([gap(1, { gap_state: 'accepted' }), gap(2, { gap_state: 'reviewed' })])?.code).toBe('STATE_INVALID')
  })
  it('同一测区两条待复核/已复核快照通过', () => {
    expect(validateResurveySelection([gap(1), gap(2, { gap_state: 'reviewed' })])).toBeNull()
  })
})

describe('补测任务确定性排序', () => {
  it('按严重度、缺口面积、发现时间排序', () => {
    const ordered = orderGapsForResurvey([
      gap(1, { severity: 'minor', area_square_m: 500, detected_at: '2026-09-01T06:00:00Z' }),
      gap(2, { severity: 'critical', area_square_m: 300, detected_at: '2026-09-03T06:00:00Z' }),
      gap(3, { severity: 'major', area_square_m: 300, detected_at: '2026-09-02T06:00:00Z' }),
      gap(4, { severity: 'critical', area_square_m: 900, detected_at: '2026-09-02T06:00:00Z' }),
    ]).map(item => item.id)
    expect(ordered).toEqual([4, 2, 3, 1])
  })
  it('不修改入参且重复排序结果一致', () => {
    const input = [gap(1, { severity: 'minor' }), gap(2, { severity: 'critical' })]
    const first = orderGapsForResurvey(input).map(item => item.id)
    const second = orderGapsForResurvey([...input].reverse()).map(item => item.id)
    expect(first).toEqual(second)
    expect(input.map(item => item.id)).toEqual([1, 2])
  })
  it('完整并列时按快照 ID 升序', () => {
    const ordered = orderGapsForResurvey([gap(9), gap(3), gap(5)]).map(item => item.id)
    expect(ordered).toEqual([3, 5, 9])
  })
})

describe('米制汇总格式化', () => {
  it('长度和面积使用米/千米与平方米/公顷', () => {
    expect(formatMetres(820)).toBe('820.0 m')
    expect(formatMetres(1500)).toBe('1.50 km')
    expect(formatSquareMetres(400)).toBe('400.0 m²')
    expect(formatSquareMetres(25000)).toBe('2.50 ha')
  })
})
