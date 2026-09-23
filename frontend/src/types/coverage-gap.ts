import type { GeoJSONFeature } from './api'
import type { GapSeverity, GapState } from './enums/gap-severity'
import type { SurveyArea } from './survey-area'

export interface CoverageGap {
  id:number; survey_area_id:number; source_run_ids:number[]; gap_geojson:GeoJSONFeature; area_square_m:number; gap_ratio:number; severity:GapSeverity
  recommended_line_geojson:GeoJSONFeature; algorithm_version:string; input_hash:string; gap_state:GapState; explanation:string; coverage_ratio:number
  overlap_ratio:number; processing_millis:number; version:number; detected_at:string; updated_at:string; survey_area?:SurveyArea
}
export interface CoverageEvidence { input_hash:string; coordinate_system:string; algorithm_version:string; source_run_count:number; coverage_ratio:number; overlap_ratio:number; gap_ratio:number; filtered_fragments:number; processing_millis:number; decision_boundary_note:string }
export interface CoverageResult { gap:CoverageGap; evidence:CoverageEvidence; idempotent:boolean }
export interface DetectCoverage { survey_area_id:number; source_run_ids:number[]; algorithm_version:string; resolution_m:number }
export interface ResurveyTaskItem { gap_id:number; rank:number; severity:GapSeverity; gap_state:GapState; area_square_m:number; recommended_line_m:number; detected_at:string; rationale:string }
export interface ResurveyTaskSheet { id:number; survey_area_id:number; items:ResurveyTaskItem[]; task_count:number; total_gap_area_square_m:number; total_recommended_line_m:number; algorithm_version:string; input_hash:string; created_by:number; created_at:string }
export interface ResurveyScheduleResult { sheet:ResurveyTaskSheet; idempotent:boolean; sort_rule:string }
