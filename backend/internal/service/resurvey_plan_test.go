package service

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paulmach/orb"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sonar-survey-coverage-planner/backend/internal/constants"
	"sonar-survey-coverage-planner/backend/internal/dto"
	"sonar-survey-coverage-planner/backend/internal/geometry"
	"sonar-survey-coverage-planner/backend/internal/model"
	"sonar-survey-coverage-planner/backend/internal/repository"
	"sonar-survey-coverage-planner/backend/pkg/api"
)

func lineFeatureJSON(lengthM float64) datatypes.JSON {
	raw, _ := geometry.MarshalFeature(orb.LineString{{0, 0}, {lengthM, 0}})
	return datatypes.JSON(raw)
}

func gapFeatureJSON() datatypes.JSON {
	return datatypes.JSON([]byte(`{"type":"Feature","properties":{},"geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1],[0,0]]]}}`))
}

func fixtureGap(id, area uint, severity, state string, areaM2 float64, detected time.Time, lineM float64) model.CoverageGap {
	return model.CoverageGap{
		ID: id, SurveyAreaID: area, SourceRunIDs: datatypes.JSON([]byte(`[1]`)),
		GapGeoJSON: gapFeatureJSON(), AreaSquareM: areaM2, GapRatio: 0.1, Severity: severity,
		RecommendedLineGeoJSON: lineFeatureJSON(lineM), AlgorithmVersion: "grid-cover-v1.0.0",
		InputHash: fmt.Sprintf("hash-gap-%d-%d", area, id),
		GapState: state, Explanation: "fixture", DetectedAt: detected, Version: 1,
		SurveyArea: &model.SurveyArea{ID: area, AreaCode: "A-01"},
	}
}

func candidateFromFixture(t *testing.T, gap model.CoverageGap) resurveyCandidate {
	t.Helper()
	length, err := geometry.FeatureLineLength(gap.RecommendedLineGeoJSON)
	if err != nil {
		t.Fatalf("measure recommended line: %v", err)
	}
	return resurveyCandidate{gap: gap, recommendedLineM: length}
}

func candidatesFromFixtures(t *testing.T, gaps ...model.CoverageGap) []resurveyCandidate {
	t.Helper()
	candidates := make([]resurveyCandidate, 0, len(gaps))
	for _, gap := range gaps {
		candidates = append(candidates, candidateFromFixture(t, gap))
	}
	return candidates
}

func TestBuildResurveyPlanOrdersBySeverityAreaTime(t *testing.T) {
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	candidates := candidatesFromFixtures(t,
		fixtureGap(1, 7, string(constants.SeverityMinor), string(constants.GapDetected), 100, base.Add(2*time.Hour), 10),
		fixtureGap(2, 7, string(constants.SeverityCritical), string(constants.GapReviewed), 300, base.Add(3*time.Hour), 30),
		fixtureGap(3, 7, string(constants.SeverityMajor), string(constants.GapDetected), 200, base, 20),
		fixtureGap(4, 7, string(constants.SeverityCritical), string(constants.GapDetected), 500, base.Add(1*time.Hour), 40),
	)
	plan := buildResurveyPlan(candidates)
	gotIDs := []uint{}
	for _, task := range plan.Tasks {
		gotIDs = append(gotIDs, task.GapID)
	}
	wantIDs := []uint{4, 2, 3, 1}
	for index := range wantIDs {
		if gotIDs[index] != wantIDs[index] {
			t.Fatalf("order = %v, want %v", gotIDs, wantIDs)
		}
	}
	if plan.Tasks[0].Order != 1 || plan.Tasks[3].Order != 4 {
		t.Fatalf("task order positions inconsistent: %+v", plan.Tasks)
	}
	if plan.TotalGapAreaSquareM != 1100 {
		t.Fatalf("total gap area = %v, want 1100", plan.TotalGapAreaSquareM)
	}
	if plan.TotalLineLengthM != 100 {
		t.Fatalf("total recommended line = %v, want 100", plan.TotalLineLengthM)
	}
	if plan.TaskCount != 4 || plan.SurveyAreaID != 7 || plan.AreaCode != "A-01" {
		t.Fatalf("plan header inconsistent: %+v", plan)
	}
	for _, task := range plan.Tasks {
		if task.SortRationale == "" {
			t.Fatalf("task #%d missing sort rationale", task.GapID)
		}
	}
	if plan.Tasks[0].RecommendedLineM != 40 {
		t.Fatalf("recommended line length for first task = %v, want 40", plan.Tasks[0].RecommendedLineM)
	}
}

func TestBuildResurveyPlanSameSeverityOrdersByAreaThenTime(t *testing.T) {
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	candidates := candidatesFromFixtures(t,
		fixtureGap(11, 1, string(constants.SeverityMajor), string(constants.GapDetected), 200, base.Add(2*time.Hour), 5),
		fixtureGap(12, 1, string(constants.SeverityMajor), string(constants.GapDetected), 200, base, 5),
		fixtureGap(13, 1, string(constants.SeverityMajor), string(constants.GapDetected), 250, base.Add(4*time.Hour), 5),
	)
	plan := buildResurveyPlan(candidates)
	gotIDs := []uint{plan.Tasks[0].GapID, plan.Tasks[1].GapID, plan.Tasks[2].GapID}
	wantIDs := []uint{13, 12, 11}
	for index := range wantIDs {
		if gotIDs[index] != wantIDs[index] {
			t.Fatalf("order = %v, want %v", gotIDs, wantIDs)
		}
	}
}

func TestBuildResurveyPlanFingerprintDeterministic(t *testing.T) {
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	makeCandidates := func() []resurveyCandidate {
		return []resurveyCandidate{
			{gap: fixtureGap(2, 1, string(constants.SeverityMajor), string(constants.GapDetected), 200, base.Add(2 * time.Hour), 20), recommendedLineM: 20},
			{gap: fixtureGap(1, 1, string(constants.SeverityCritical), string(constants.GapReviewed), 300, base, 30), recommendedLineM: 30},
		}
	}
	first := buildResurveyPlan(makeCandidates())
	second := buildResurveyPlan(makeCandidates())
	if first.PlanFingerprint == "" || first.PlanFingerprint != second.PlanFingerprint {
		t.Fatalf("fingerprint must be stable across regenerations: %q vs %q", first.PlanFingerprint, second.PlanFingerprint)
	}
	other := buildResurveyPlan([]resurveyCandidate{
		{gap: fixtureGap(3, 1, string(constants.SeverityCritical), string(constants.GapDetected), 300, base, 30), recommendedLineM: 30},
		{gap: fixtureGap(1, 1, string(constants.SeverityCritical), string(constants.GapReviewed), 300, base, 30), recommendedLineM: 30},
	})
	if other.PlanFingerprint == first.PlanFingerprint {
		t.Fatal("different snapshot sets must produce different fingerprints")
	}
}

func newResurveyService(t *testing.T, gaps ...model.CoverageGap) *CoverageGapService {
	t.Helper()
	dsn := fmt.Sprintf("file:resurvey-test-%d?mode=memory&cache=shared", resurveyDBSerial.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SurveyArea{}, &model.CoverageGap{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	areas := map[uint]model.SurveyArea{}
	for _, gap := range gaps {
		if _, ok := areas[gap.SurveyAreaID]; ok {
			continue
		}
		area := model.SurveyArea{
			ID: gap.SurveyAreaID, AreaCode: fmt.Sprintf("A-%02d", gap.SurveyAreaID), Name: "fixture area",
			BoundaryGeoJSON: gapFeatureJSON(), TargetResolutionM: 20, CoordinateSystem: "EPSG:32651",
			DefaultSwathM: 100, OwnerTeam: "fixture", Status: "active",
		}
		if err := db.Create(&area).Error; err != nil {
			t.Fatalf("seed area: %v", err)
		}
		areas[gap.SurveyAreaID] = area
	}
	for index := range gaps {
		gaps[index].SurveyArea = nil
		if err := db.Create(&gaps[index]).Error; err != nil {
			t.Fatalf("seed gap: %v", err)
		}
	}
	return NewCoverageGapService(repository.NewCoverageGapRepository(db), repository.NewSurveyAreaRepository(db), nil, nil)
}

var resurveyDBSerial atomic.Int64

func assertAppErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %s, got nil", code)
	}
	appErr, ok := err.(*api.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T: %v", err, err)
	}
	if appErr.Code != code {
		t.Fatalf("error code = %s, want %s", appErr.Code, code)
	}
}

func TestPlanResurveyRejectsInvalidSelections(t *testing.T) {
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	gaps := []model.CoverageGap{
		fixtureGap(101, 1, string(constants.SeverityMajor), string(constants.GapDetected), 100, base, 10),
		fixtureGap(102, 1, string(constants.SeverityMajor), string(constants.GapReviewed), 100, base, 10),
		fixtureGap(103, 1, string(constants.SeverityMajor), string(constants.GapClosed), 100, base, 10),
		fixtureGap(104, 1, string(constants.SeverityMajor), string(constants.GapAccepted), 100, base, 10),
		fixtureGap(201, 2, string(constants.SeverityMajor), string(constants.GapDetected), 100, base, 10),
	}
	gaps[0].SurveyArea = nil
	gaps[0].SurveyAreaID = 1
	gaps[2].SurveyAreaID = 1
	gaps[3].SurveyAreaID = 1
	gaps[4].SurveyAreaID = 2
	gaps[4].SurveyArea = &model.SurveyArea{ID: 2, AreaCode: "A-02"}
	service := newResurveyService(t, gaps...)

	// 重复快照整次拒绝
	_, err := service.PlanResurvey(dto.ResurveyPlanRequest{GapIDs: []uint{101, 101}})
	assertAppErrorCode(t, err, "RESURVEY_SELECTION_DUPLICATE")

	// 快照不存在整次拒绝
	_, err = service.PlanResurvey(dto.ResurveyPlanRequest{GapIDs: []uint{101, 999}})
	assertAppErrorCode(t, err, "RESURVEY_SELECTION_NOT_FOUND")

	// 跨测区整次拒绝
	_, err = service.PlanResurvey(dto.ResurveyPlanRequest{GapIDs: []uint{101, 201}})
	assertAppErrorCode(t, err, "RESURVEY_SELECTION_MULTI_AREA")

	// 含已关闭/非待复核状态整次拒绝
	_, err = service.PlanResurvey(dto.ResurveyPlanRequest{GapIDs: []uint{101, 103}})
	assertAppErrorCode(t, err, "RESURVEY_SELECTION_STATE_INVALID")
	_, err = service.PlanResurvey(dto.ResurveyPlanRequest{GapIDs: []uint{101, 104}})
	assertAppErrorCode(t, err, "RESURVEY_SELECTION_STATE_INVALID")
}

func TestPlanResurveyLeavesSnapshotsUnchangedAndIsRepeatable(t *testing.T) {
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	gaps := []model.CoverageGap{
		fixtureGap(301, 1, string(constants.SeverityCritical), string(constants.GapDetected), 400, base, 40),
		fixtureGap(302, 1, string(constants.SeverityMinor), string(constants.GapReviewed), 80, base.Add(2 * time.Hour), 8),
	}
	for index := range gaps {
		gaps[index].SurveyArea = nil
	}
	svc := newResurveyService(t, gaps...)
	request := dto.ResurveyPlanRequest{GapIDs: []uint{302, 301}}

	first, err := svc.PlanResurvey(request)
	if err != nil {
		t.Fatalf("plan resurvey: %v", err)
	}
	second, err := svc.PlanResurvey(request)
	if err != nil {
		t.Fatalf("replan resurvey: %v", err)
	}
	if first.PlanFingerprint != second.PlanFingerprint || first.TotalGapAreaSquareM != second.TotalGapAreaSquareM || first.TotalLineLengthM != second.TotalLineLengthM {
		t.Fatal("repeated generation must produce identical results")
	}
	if len(first.Tasks) != 2 || first.Tasks[0].GapID != 301 || first.Tasks[1].GapID != 302 {
		t.Fatalf("unexpected task ordering: %+v", first.Tasks)
	}

	for _, id := range []uint{301, 302} {
		stored, getErr := svc.Get(id)
		if getErr != nil {
			t.Fatalf("reload gap %d: %v", id, getErr)
		}
		if stored.Version != 1 {
			t.Fatalf("gap %d version changed to %d, snapshots must remain unchanged", id, stored.Version)
		}
		original := gaps[0]
		if id == 302 {
			original = gaps[1]
		}
		if stored.GapState != original.GapState {
			t.Fatalf("gap %d state mutated: %s -> %s", id, original.GapState, stored.GapState)
		}
	}
}
