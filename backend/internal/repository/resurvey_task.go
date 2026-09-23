package repository

import (
	"fmt"

	"gorm.io/gorm"
	"sonar-survey-coverage-planner/backend/internal/model"
)

type ResurveyTaskRepository struct{ db *gorm.DB }

func NewResurveyTaskRepository(db *gorm.DB) *ResurveyTaskRepository {
	return &ResurveyTaskRepository{db: db}
}

func (r *ResurveyTaskRepository) Create(item *model.ResurveyTaskSheet) error {
	if err := r.db.Create(item).Error; err != nil {
		return fmt.Errorf("create resurvey task sheet: %w", err)
	}
	return nil
}

func (r *ResurveyTaskRepository) ByInputHash(hash string) (model.ResurveyTaskSheet, error) {
	var item model.ResurveyTaskSheet
	if err := r.db.Preload("SurveyArea").Where("input_hash = ?", hash).First(&item).Error; err != nil {
		return item, fmt.Errorf("find resurvey task input hash: %w", err)
	}
	return item, nil
}
