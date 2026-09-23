package dto

import "time"

type ScheduleResurveyRequest struct {
	GapIDs []uint `json:"gap_ids" binding:"required,min=2,max=64,dive,gt=0"`
}

// ResurveyTaskItem is one ordered entry of a resurvey task sheet with the
// human-readable rationale for its position.
type ResurveyTaskItem struct {
	GapID             uint      `json:"gap_id"`
	Rank              int       `json:"rank"`
	Severity          string    `json:"severity"`
	GapState          string    `json:"gap_state"`
	AreaSquareM       float64   `json:"area_square_m"`
	RecommendedLineM  float64   `json:"recommended_line_m"`
	DetectedAt        time.Time `json:"detected_at"`
	Rationale         string    `json:"rationale"`
}
