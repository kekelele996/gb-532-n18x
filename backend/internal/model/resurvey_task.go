package model

import (
	"time"

	"gorm.io/datatypes"
)

// ResurveyTaskSheet is a frozen, idempotent scheduling artifact over a set of
// coverage gap snapshots. Generating a sheet never mutates the source gaps.
type ResurveyTaskSheet struct {
	ID                    uint           `json:"id" gorm:"primaryKey"`
	SurveyAreaID          uint           `json:"survey_area_id" gorm:"not null;index"`
	Items                 datatypes.JSON `json:"items" gorm:"column:items;type:jsonb;not null"`
	TaskCount             int            `json:"task_count" gorm:"not null"`
	TotalGapAreaSquareM   float64        `json:"total_gap_area_square_m" gorm:"not null"`
	TotalRecommendedLineM float64        `json:"total_recommended_line_m" gorm:"not null"`
	AlgorithmVersion      string         `json:"algorithm_version" gorm:"size:40;not null"`
	InputHash             string         `json:"input_hash" gorm:"size:64;not null;uniqueIndex:idx_resurvey_idempotency"`
	CreatedBy             uint           `json:"created_by" gorm:"not null"`
	CreatedAt             time.Time      `json:"created_at" gorm:"not null"`
	SurveyArea            *SurveyArea    `json:"survey_area,omitempty" gorm:"foreignKey:SurveyAreaID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

func (ResurveyTaskSheet) TableName() string { return "resurvey_task_sheets" }
