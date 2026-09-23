import { describe, expect, it } from 'vitest'
import type { CoverageGap } from '../types/coverage-gap'
import { resurveyIdempotencyKey, validateResurveySelection } from './resurvey'

function gap(id:number, area:number, state:CoverageGap['gap_state']):CoverageGap{
  return{id:id,survey_area_id:area,gap_state:state} as CoverageGap
}

describe('resurvey selection rules',()=>{
  const gaps=[gap(1,7,'detected'),gap(2,7,'reviewed'),gap(3,7,'closed'),gap(4,9,'detected')]
  it('accepts two or more same-area snapshots awaiting or past review',()=>{
    expect(validateResurveySelection(gaps,[1,2])).toEqual([])
    expect(validateResurveySelection(gaps,[2,1])).toEqual([])
  })
  it('requires at least two snapshots',()=>{
    expect(validateResurveySelection(gaps,[1])).toContain('too_few')
    expect(validateResurveySelection(gaps,[])).toContain('too_few')
  })
  it('flags duplicates, foreign areas and closed snapshots',()=>{
    expect(validateResurveySelection(gaps,[1,1,2])).toContain('duplicated')
    expect(validateResurveySelection(gaps,[1,4])).toContain('cross_area')
    expect(validateResurveySelection(gaps,[1,3])).toContain('state_invalid')
  })
  it('produces an order-independent idempotency key',()=>{
    expect(resurveyIdempotencyKey([3,1,2])).toBe(resurveyIdempotencyKey([2,3,1]))
    expect(resurveyIdempotencyKey([1,2])).not.toBe(resurveyIdempotencyKey([1,3]))
  })
})
