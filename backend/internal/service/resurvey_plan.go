package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"sonar-survey-coverage-planner/backend/internal/constants"
	"sonar-survey-coverage-planner/backend/internal/dto"
	"sonar-survey-coverage-planner/backend/internal/geometry"
	"sonar-survey-coverage-planner/backend/internal/model"
	"sonar-survey-coverage-planner/backend/pkg/api"
)

const resurveyPlanVersion = "resurvey-plan-v1"

var (
	severityRank  = map[constants.GapSeverity]int{constants.SeverityCritical: 3, constants.SeverityMajor: 2, constants.SeverityMinor: 1}
	severityLabel = map[constants.GapSeverity]string{constants.SeverityCritical: "严重", constants.SeverityMajor: "主要", constants.SeverityMinor: "轻微"}
)

type resurveyCandidate struct {
	gap              model.CoverageGap
	recommendedLineM float64
}

// PlanResurvey 为同一测区内多条待复核/已复核缺口快照生成确定性补测任务顺序。
// 该操作只读：不会迁移任何快照状态，也不写审计或数据库。
func (s *CoverageGapService) PlanResurvey(request dto.ResurveyPlanRequest) (dto.ResurveyPlanView, error) {
	if duplicates := duplicateUint(request.GapIDs); len(duplicates) > 0 {
		return dto.ResurveyPlanView{}, api.Unprocessable("RESURVEY_SELECTION_DUPLICATE",
			fmt.Sprintf("选择包含重复快照：#%s，请整次重新勾选", joinUint(duplicates)), nil)
	}
	gaps, err := s.repository.ByIDs(uniqueIDs(request.GapIDs))
	if err != nil {
		return dto.ResurveyPlanView{}, err
	}
	byID := make(map[uint]model.CoverageGap, len(gaps))
	for _, gap := range gaps {
		byID[gap.ID] = gap
	}
	var missing []uint
	for _, id := range uniqueIDs(request.GapIDs) {
		if _, ok := byID[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return dto.ResurveyPlanView{}, api.Unprocessable("RESURVEY_SELECTION_NOT_FOUND",
			fmt.Sprintf("快照不存在：#%s，整次拒绝且原快照保持不变", joinUint(missing)), nil)
	}
	areaID := byID[uniqueIDs(request.GapIDs)[0]].SurveyAreaID
	var otherAreas, ineligible []uint
	for _, gap := range gaps {
		if gap.SurveyAreaID != areaID {
			otherAreas = append(otherAreas, gap.ID)
		}
		if !canResurvey(gap.GapState) {
			ineligible = append(ineligible, gap.ID)
		}
	}
	if len(otherAreas) > 0 {
		return dto.ResurveyPlanView{}, api.Unprocessable("RESURVEY_SELECTION_MULTI_AREA",
			fmt.Sprintf("只能选择同一测区的快照，跨测区快照：#%s，整次拒绝", joinUint(sortedUint(otherAreas))), nil)
	}
	if len(ineligible) > 0 {
		return dto.ResurveyPlanView{}, api.Unprocessable("RESURVEY_SELECTION_STATE_INVALID",
			fmt.Sprintf("仅待复核或已复核快照可排入补测，已关闭等不合规快照：#%s，整次拒绝", joinUint(sortedUint(ineligible))), nil)
	}
	candidates := make([]resurveyCandidate, 0, len(gaps))
	for _, gap := range gaps {
		lineLength, lineErr := geometry.FeatureLineLength(gap.RecommendedLineGeoJSON)
		if lineErr != nil {
			return dto.ResurveyPlanView{}, fmt.Errorf("measure recommended line for gap %d: %w", gap.ID, lineErr)
		}
		candidates = append(candidates, resurveyCandidate{gap: gap, recommendedLineM: lineLength})
	}
	plan := buildResurveyPlan(candidates)
	return plan, nil
}

// buildResurveyPlan 按 严重度降序 -> 缺口面积降序 -> 发现时间升序 -> ID 升序 稳定排序，
// 汇总缺口面积与建议线长度，并生成与输入集合一一对应的确定性指纹。
func buildResurveyPlan(candidates []resurveyCandidate) dto.ResurveyPlanView {
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i].gap, candidates[j].gap
		leftRank, rightRank := severityRank[constants.GapSeverity(left.Severity)], severityRank[constants.GapSeverity(right.Severity)]
		if leftRank != rightRank {
			return leftRank > rightRank
		}
		if left.AreaSquareM != right.AreaSquareM {
			return left.AreaSquareM > right.AreaSquareM
		}
		if !left.DetectedAt.Equal(right.DetectedAt) {
			return left.DetectedAt.Before(right.DetectedAt)
		}
		return left.ID < right.ID
	})
	tasks := make([]dto.ResurveyTaskView, 0, len(candidates))
	totalArea, totalLine := 0.0, 0.0
	orderedIDs := make([]uint, 0, len(candidates))
	for index, candidate := range candidates {
		gap := candidate.gap
		totalArea += gap.AreaSquareM
		totalLine += candidate.recommendedLineM
		orderedIDs = append(orderedIDs, gap.ID)
		detected := gap.DetectedAt.UTC()
		taskAreaCode := ""
		if gap.SurveyArea != nil {
			taskAreaCode = gap.SurveyArea.AreaCode
		}
		tasks = append(tasks, dto.ResurveyTaskView{
			Order:            index + 1,
			GapID:            gap.ID,
			SurveyAreaID:     gap.SurveyAreaID,
			AreaCode:         taskAreaCode,
			Severity:         gap.Severity,
			AreaSquareM:      gap.AreaSquareM,
			RecommendedLineM: candidate.recommendedLineM,
			GapState:         gap.GapState,
			DetectedAt:       detected.Format(time.RFC3339),
			AlgorithmVersion: gap.AlgorithmVersion,
			SortRationale:    resurveRationale(candidate, candidates, index),
		})
	}
	areaCode := ""
	if len(candidates) > 0 && candidates[0].gap.SurveyArea != nil {
		areaCode = candidates[0].gap.SurveyArea.AreaCode
	}
	return dto.ResurveyPlanView{
		SurveyAreaID:        candidates[0].gap.SurveyAreaID,
		AreaCode:            areaCode,
		TaskCount:           len(candidates),
		TotalGapAreaSquareM: totalArea,
		TotalLineLengthM:    totalLine,
		SortingPolicy:       "排序规则：严重度降序（严重 > 主要 > 轻微）；同级按缺口面积降序；面积相同按发现时间升序（早发现先补测）；仍相同按快照 ID 升序。任务单为只读规划，不改变任何原快照状态。",
		PlanFingerprint:     resurveFingerprint(orderedIDs),
		Tasks:               tasks,
	}
}

func resurveRationale(candidate resurveyCandidate, ordered []resurveyCandidate, index int) string {
	gap := candidate.gap
	base := fmt.Sprintf("严重度=%s（%d/3）", severityLabel[constants.GapSeverity(gap.Severity)], severityRank[constants.GapSeverity(gap.Severity)])
	if index == len(ordered)-1 {
		return fmt.Sprintf("第 %d 位：%s，缺口面积 %.1f m²，%s 发现；已是排序末位。",
			index+1, base, gap.AreaSquareM, gap.DetectedAt.UTC().Format("2006-01-02 15:04 UTC"))
	}
	next := ordered[index+1].gap
	switch {
	case severityRank[constants.GapSeverity(gap.Severity)] > severityRank[constants.GapSeverity(next.Severity)]:
		return fmt.Sprintf("第 %d 位：%s，严重度高于下一条（%s），优先补测。",
			index+1, base, severityLabel[constants.GapSeverity(next.Severity)])
	case gap.AreaSquareM > next.AreaSquareM:
		return fmt.Sprintf("第 %d 位：%s，同级严重度下缺口面积 %.1f m² 大于下一条 %.1f m²，优先补测。",
			index+1, base, gap.AreaSquareM, next.AreaSquareM)
	case gap.AreaSquareM < next.AreaSquareM:
		return fmt.Sprintf("第 %d 位：%s，缺口面积 %.1f m² 小于下一条 %.1f m²。",
			index+1, base, gap.AreaSquareM, next.AreaSquareM)
	case gap.DetectedAt.Before(next.DetectedAt):
		return fmt.Sprintf("第 %d 位：%s，面积相同（%.1f m²），%s 早于下一条 %s 发现，先补测。",
			index+1, base, gap.AreaSquareM,
			gap.DetectedAt.UTC().Format("01-02 15:04"), next.DetectedAt.UTC().Format("01-02 15:04"))
	default:
		return fmt.Sprintf("第 %d 位：%s，面积与发现时间相同，快照 #%d 先于 #%d 排入。",
			index+1, base, gap.ID, next.ID)
	}
}

func resurveFingerprint(orderedIDs []uint) string {
	payload, _ := json.Marshal(struct {
		Version string `json:"version"`
		GapIDs  []uint `json:"gap_ids"`
	}{Version: resurveyPlanVersion, GapIDs: orderedIDs})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:16])
}

func canResurvey(state string) bool {
	return constants.GapState(state) == constants.GapDetected || constants.GapState(state) == constants.GapReviewed
}

func duplicateUint(values []uint) []uint {
	seen := map[uint]int{}
	var duplicates []uint
	marked := map[uint]struct{}{}
	for _, value := range values {
		seen[value]++
		if seen[value] > 1 {
			if _, ok := marked[value]; !ok {
				marked[value] = struct{}{}
				duplicates = append(duplicates, value)
			}
		}
	}
	sort.Slice(duplicates, func(i, j int) bool { return duplicates[i] < duplicates[j] })
	return duplicates
}

func sortedUint(values []uint) []uint {
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values
}

func joinUint(values []uint) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("%d", value))
	}
	return strings.Join(parts, ", #")
}
