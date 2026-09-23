package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"

	"sonar-survey-coverage-planner/backend/internal/constants"
	"sonar-survey-coverage-planner/backend/internal/dto"
	"sonar-survey-coverage-planner/backend/internal/geometry"
	"sonar-survey-coverage-planner/backend/internal/model"
	"sonar-survey-coverage-planner/backend/internal/repository"
	"sonar-survey-coverage-planner/backend/pkg/api"
)

const (
	ResurveyScheduleAlgorithm = "resurvey-schedule-v1.0.0"
	ResurveySortRule          = "严重度（critical>major>minor）→ 缺口面积降序 → 发现时间升序 → 快照 ID 升序"
)

type ResurveyScheduleView struct {
	Sheet      model.ResurveyTaskSheet `json:"sheet"`
	Idempotent bool                    `json:"idempotent"`
	SortRule   string                  `json:"sort_rule"`
}

type ResurveyTaskService struct {
	repository *repository.ResurveyTaskRepository
	gaps       *repository.CoverageGapRepository
	audit      *AuditService
}

func NewResurveyTaskService(repository *repository.ResurveyTaskRepository, gaps *repository.CoverageGapRepository, audit *AuditService) *ResurveyTaskService {
	return &ResurveyTaskService{repository: repository, gaps: gaps, audit: audit}
}

// Schedule validates the selected snapshots, orders them deterministically and
// freezes a task sheet. Source gaps are never mutated; an invalid member
// rejects the whole batch, and the same snapshot set replays idempotently.
func (s *ResurveyTaskService) Schedule(request dto.ScheduleResurveyRequest, idempotencyKey string, actor Actor) (ResurveyScheduleView, error) {
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 120 {
		return ResurveyScheduleView{}, api.BadRequest("IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key 必须为 8 到 120 个字符", nil)
	}
	unique := uniqueIDs(request.GapIDs)
	if len(unique) != len(request.GapIDs) {
		return ResurveyScheduleView{}, api.Unprocessable("RESURVEY_GAP_DUPLICATED", "任务单包含重复快照，整次拒绝", nil)
	}
	if len(unique) < 2 {
		return ResurveyScheduleView{}, api.Unprocessable("RESURVEY_BATCH_TOO_SMALL", "补测调度至少需要两条快照", nil)
	}
	gaps, err := s.gaps.ByIDs(unique)
	if err != nil {
		return ResurveyScheduleView{}, err
	}
	if len(gaps) != len(unique) {
		return ResurveyScheduleView{}, api.Unprocessable("GAP_SET_INCOMPLETE", "部分覆盖缺口不存在，整次拒绝", nil)
	}
	areaID := gaps[0].SurveyAreaID
	ineligible := make([]uint, 0, len(gaps))
	for _, gap := range gaps {
		if gap.SurveyAreaID != areaID {
			return ResurveyScheduleView{}, api.Unprocessable("RESURVEY_AREA_MISMATCH", "仅同一测区的缺口快照可合并调度，整次拒绝", nil)
		}
		state, parseErr := constants.ParseGapState(gap.GapState)
		if parseErr != nil || !state.Schedulable() {
			ineligible = append(ineligible, gap.ID)
		}
	}
	if len(ineligible) > 0 {
		rejection := api.Unprocessable("RESURVEY_STATE_INVALID", "仅待复核或已复核的快照可调度；含已关闭或其他状态时整次拒绝", nil)
		rejection.Details = map[string]any{"gap_ids": ineligible}
		return ResurveyScheduleView{}, rejection
	}
	hash := geometry.StableScheduleHash(ResurveyScheduleAlgorithm, unique)
	if existing, lookupErr := s.repository.ByInputHash(hash); lookupErr == nil {
		return ResurveyScheduleView{Sheet: existing, Idempotent: true, SortRule: ResurveySortRule}, nil
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return ResurveyScheduleView{}, lookupErr
	}
	lineLengths := make(map[uint]float64, len(gaps))
	totalLine, totalArea := 0.0, 0.0
	for _, gap := range gaps {
		lines, parseErr := geometry.ParseLines(gap.RecommendedLineGeoJSON)
		if parseErr != nil {
			return ResurveyScheduleView{}, api.Unprocessable("GEOJSON_INVALID", "补测线建议无法解析", parseErr)
		}
		lineLengths[gap.ID] = geometry.TrackLength(lines)
		totalLine += lineLengths[gap.ID]
		totalArea += gap.AreaSquareM
	}
	items := orderResurveyItems(gaps, lineLengths)
	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return ResurveyScheduleView{}, fmt.Errorf("encode resurvey task items: %w", err)
	}
	sheet := model.ResurveyTaskSheet{SurveyAreaID: areaID, Items: itemsJSON, TaskCount: len(items), TotalGapAreaSquareM: totalArea, TotalRecommendedLineM: totalLine, AlgorithmVersion: ResurveyScheduleAlgorithm, InputHash: hash, CreatedBy: actor.UserID, CreatedAt: time.Now().UTC()}
	if err := s.repository.Create(&sheet); err != nil {
		return ResurveyScheduleView{}, mapDatabaseError(err, "补测任务单")
	}
	if err := s.audit.Record(actor, "coverage.resurvey_schedule", "resurvey_task_sheet", sheet.ID, nil, sheet, map[string]any{"input_checksum": hash, "idempotency_key": idempotencyKey, "algorithm_version": ResurveyScheduleAlgorithm, "gap_ids": unique, "task_count": len(items), "total_gap_area_square_m": totalArea, "total_recommended_line_m": totalLine, "sort_rule": ResurveySortRule}); err != nil {
		return ResurveyScheduleView{}, err
	}
	return ResurveyScheduleView{Sheet: sheet, SortRule: ResurveySortRule}, nil
}

// orderResurveyItems sorts snapshots by severity, gap area and detection time
// with the gap ID as a stable final key, then explains each position.
func orderResurveyItems(gaps []model.CoverageGap, lineLengths map[uint]float64) []dto.ResurveyTaskItem {
	ordered := append([]model.CoverageGap(nil), gaps...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := ordered[i], ordered[j]
		leftRank, rightRank := constants.SeverityRank(constants.GapSeverity(left.Severity)), constants.SeverityRank(constants.GapSeverity(right.Severity))
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if left.AreaSquareM != right.AreaSquareM {
			return left.AreaSquareM > right.AreaSquareM
		}
		if !left.DetectedAt.Equal(right.DetectedAt) {
			return left.DetectedAt.Before(right.DetectedAt)
		}
		return left.ID < right.ID
	})
	items := make([]dto.ResurveyTaskItem, 0, len(ordered))
	for index, gap := range ordered {
		rank := index + 1
		rationale := fmt.Sprintf("第 %d 位：严重度 %s 优先（critical>major>minor）；缺口面积 %.2f m² 按降序；发现时间 %s 按升序；快照 ID %d 保证次序稳定。", rank, gap.Severity, gap.AreaSquareM, gap.DetectedAt.UTC().Format(time.RFC3339), gap.ID)
		items = append(items, dto.ResurveyTaskItem{GapID: gap.ID, Rank: rank, Severity: gap.Severity, GapState: gap.GapState, AreaSquareM: gap.AreaSquareM, RecommendedLineM: lineLengths[gap.ID], DetectedAt: gap.DetectedAt, Rationale: rationale})
	}
	return items
}
