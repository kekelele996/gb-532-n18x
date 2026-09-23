package dto

type DetectCoverageRequest struct {
	SurveyAreaID     uint    `json:"survey_area_id" binding:"required,gt=0"`
	SourceRunIDs     []uint  `json:"source_run_ids" binding:"required,min=1,dive,gt=0"`
	AlgorithmVersion string  `json:"algorithm_version" binding:"required,min=3,max=40"`
	ResolutionM      float64 `json:"resolution_m" binding:"omitempty,gt=0,lte=100"`
}

type GapTransitionRequest struct {
	TargetState     string `json:"target_state" binding:"required,oneof=reviewed accepted false_positive resurveyed closed"`
	ExpectedVersion uint   `json:"expected_version" binding:"required,gt=0"`
	ReviewNote      string `json:"review_note" binding:"required,min=8,max=600"`
}

type ResurveyPlanRequest struct {
	GapIDs []uint `json:"gap_ids" binding:"required,min=2,max=50,dive,gt=0"`
}

type ResurveyTaskView struct {
	Order                int     `json:"order"`
	GapID                uint    `json:"gap_id"`
	SurveyAreaID         uint    `json:"survey_area_id"`
	AreaCode             string  `json:"area_code"`
	Severity             string  `json:"severity"`
	AreaSquareM          float64 `json:"area_square_m"`
	RecommendedLineM     float64 `json:"recommended_line_m"`
	GapState             string  `json:"gap_state"`
	DetectedAt           string  `json:"detected_at"`
	AlgorithmVersion     string  `json:"algorithm_version"`
	SortRationale        string  `json:"sort_rationale"`
}

type ResurveyPlanView struct {
	SurveyAreaID         uint               `json:"survey_area_id"`
	AreaCode             string             `json:"area_code"`
	TaskCount            int                `json:"task_count"`
	TotalGapAreaSquareM  float64            `json:"total_gap_area_square_m"`
	TotalLineLengthM     float64            `json:"total_line_length_m"`
	SortingPolicy        string             `json:"sorting_policy"`
	PlanFingerprint      string             `json:"plan_fingerprint"`
	Tasks                []ResurveyTaskView `json:"tasks"`
}

type CoverageGapQuery struct {
	SurveyAreaID uint
	State        string
	Severity     string
	Page         int
	PageSize     int
}

type CoverageEvidence struct {
	InputHash            string  `json:"input_hash"`
	CoordinateSystem     string  `json:"coordinate_system"`
	AlgorithmVersion     string  `json:"algorithm_version"`
	SourceRunCount       int     `json:"source_run_count"`
	CoverageRatio        float64 `json:"coverage_ratio"`
	OverlapRatio         float64 `json:"overlap_ratio"`
	GapRatio             float64 `json:"gap_ratio"`
	FilteredFragments    int     `json:"filtered_fragments"`
	ProcessingMillis     int64   `json:"processing_millis"`
	DecisionBoundaryNote string  `json:"decision_boundary_note"`
}
