package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"sonar-survey-coverage-planner/backend/internal/config"
	"sonar-survey-coverage-planner/backend/internal/constants"
	"sonar-survey-coverage-planner/backend/internal/dto"
	"sonar-survey-coverage-planner/backend/internal/model"
	"sonar-survey-coverage-planner/backend/internal/repository"
	"sonar-survey-coverage-planner/backend/pkg/api"
)

const resurveyTestKey = "resurvey-test-key-0001"

func newResurveyTestService(t *testing.T) (*ResurveyTaskService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "-", " ", "-").Replace(t.Name()))
	db, err := config.OpenDatabase(config.Config{DBDriver: "sqlite", DBDSN: dsn, JWTSecret: "test-secret-with-more-than-thirty-two-characters", AutoMigrate: true, SeedData: false})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	support := repository.NewSupportRepository(db)
	return NewResurveyTaskService(repository.NewResurveyTaskRepository(db), repository.NewCoverageGapRepository(db), NewAuditService(support)), db
}

func seedResurveyFixture(t *testing.T, db *gorm.DB) (areaID, otherAreaID uint, gaps []model.CoverageGap) {
	t.Helper()
	boundary := []byte(`{"type":"Feature","properties":{},"geometry":{"type":"Polygon","coordinates":[[[0,0],[100,0],[100,100],[0,100],[0,0]]]}}`)
	area := model.SurveyArea{AreaCode: "T-A01", Name: "调度测试测区", BoundaryGeoJSON: boundary, TargetResolutionM: 20, CoordinateSystem: "EPSG:32650", DefaultSwathM: 180, OwnerTeam: "测试组", Status: constants.AreaActive, Version: 1}
	other := model.SurveyArea{AreaCode: "T-B01", Name: "另一测区", BoundaryGeoJSON: boundary, TargetResolutionM: 20, CoordinateSystem: "EPSG:32650", DefaultSwathM: 180, OwnerTeam: "测试组", Status: constants.AreaActive, Version: 1}
	if err := db.Create(&area).Error; err != nil {
		t.Fatalf("create area: %v", err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create other area: %v", err)
	}
	line := func(length float64) []byte {
		return []byte(fmt.Sprintf(`{"type":"Feature","properties":{},"geometry":{"type":"LineString","coordinates":[[0,0],[%v,0]]}}`, length))
	}
	base := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	fixtures := []model.CoverageGap{
		{SurveyAreaID: area.ID, SourceRunIDs: []byte(`[1]`), GapGeoJSON: boundary, AreaSquareM: 500, GapRatio: 0.2, Severity: string(constants.SeverityCritical), RecommendedLineGeoJSON: line(120), AlgorithmVersion: "grid-cover-v1.0.0", InputHash: "hash-critical-large", GapState: string(constants.GapDetected), Explanation: "夹具", Version: 1, DetectedAt: base.Add(2 * time.Hour)},
		{SurveyAreaID: area.ID, SourceRunIDs: []byte(`[1]`), GapGeoJSON: boundary, AreaSquareM: 300, GapRatio: 0.15, Severity: string(constants.SeverityCritical), RecommendedLineGeoJSON: line(60), AlgorithmVersion: "grid-cover-v1.0.0", InputHash: "hash-critical-small", GapState: string(constants.GapReviewed), Explanation: "夹具", Version: 1, DetectedAt: base.Add(time.Hour)},
		{SurveyAreaID: area.ID, SourceRunIDs: []byte(`[1]`), GapGeoJSON: boundary, AreaSquareM: 900, GapRatio: 0.08, Severity: string(constants.SeverityMajor), RecommendedLineGeoJSON: line(30), AlgorithmVersion: "grid-cover-v1.0.0", InputHash: "hash-major-early", GapState: string(constants.GapReviewed), Explanation: "夹具", Version: 1, DetectedAt: base},
		{SurveyAreaID: area.ID, SourceRunIDs: []byte(`[1]`), GapGeoJSON: boundary, AreaSquareM: 900, GapRatio: 0.08, Severity: string(constants.SeverityMajor), RecommendedLineGeoJSON: line(40), AlgorithmVersion: "grid-cover-v1.0.0", InputHash: "hash-major-late", GapState: string(constants.GapDetected), Explanation: "夹具", Version: 1, DetectedAt: base.Add(3 * time.Hour)},
		{SurveyAreaID: area.ID, SourceRunIDs: []byte(`[1]`), GapGeoJSON: boundary, AreaSquareM: 100, GapRatio: 0.01, Severity: string(constants.SeverityMinor), RecommendedLineGeoJSON: line(10), AlgorithmVersion: "grid-cover-v1.0.0", InputHash: "hash-closed", GapState: string(constants.GapClosed), Explanation: "夹具", Version: 1, DetectedAt: base},
		{SurveyAreaID: other.ID, SourceRunIDs: []byte(`[1]`), GapGeoJSON: boundary, AreaSquareM: 200, GapRatio: 0.06, Severity: string(constants.SeverityMajor), RecommendedLineGeoJSON: line(20), AlgorithmVersion: "grid-cover-v1.0.0", InputHash: "hash-other-area", GapState: string(constants.GapDetected), Explanation: "夹具", Version: 1, DetectedAt: base},
	}
	for index := range fixtures {
		if err := db.Create(&fixtures[index]).Error; err != nil {
			t.Fatalf("create gap %d: %v", index, err)
		}
	}
	return area.ID, other.ID, fixtures
}

func scheduleActor() Actor {
	return Actor{RequestID: "req-resurvey-test", UserID: 1, Username: "reviewer", Role: constants.RoleReviewer}
}

func appErrorCode(t *testing.T, err error) string {
	t.Helper()
	var appErr *api.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %v", err)
	}
	return appErr.Code
}

func TestScheduleOrdersAggregatesAndStaysIdempotent(t *testing.T) {
	service, db := newResurveyTestService(t)
	areaID, _, gaps := seedResurveyFixture(t, db)
	ids := []uint{gaps[3].ID, gaps[2].ID, gaps[1].ID, gaps[0].ID}
	view, err := service.Schedule(dto.ScheduleResurveyRequest{GapIDs: ids}, resurveyTestKey, scheduleActor())
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}
	if view.Idempotent || view.SortRule != ResurveySortRule {
		t.Fatalf("first generation must be a fresh sheet with sort rule, got %+v", view)
	}
	sheet := view.Sheet
	if sheet.SurveyAreaID != areaID || sheet.TaskCount != 4 {
		t.Fatalf("sheet area=%d count=%d, want area %d count 4", sheet.SurveyAreaID, sheet.TaskCount, areaID)
	}
	var items []dto.ResurveyTaskItem
	if err := json.Unmarshal(sheet.Items, &items); err != nil {
		t.Fatalf("decode sheet items: %v", err)
	}
	wantOrder := []uint{gaps[0].ID, gaps[1].ID, gaps[2].ID, gaps[3].ID}
	if len(items) != len(wantOrder) {
		t.Fatalf("items = %d, want %d", len(items), len(wantOrder))
	}
	for index, want := range wantOrder {
		if items[index].GapID != want || items[index].Rank != index+1 {
			t.Fatalf("position %d: gap=%d rank=%d, want gap %d rank %d", index, items[index].GapID, items[index].Rank, want, index+1)
		}
		if items[index].Rationale == "" {
			t.Fatalf("position %d misses ordering rationale", index)
		}
	}
	if math.Abs(sheet.TotalGapAreaSquareM-2600) > 0.001 {
		t.Fatalf("total gap area = %.2f, want 2600", sheet.TotalGapAreaSquareM)
	}
	if math.Abs(sheet.TotalRecommendedLineM-250) > 0.001 {
		t.Fatalf("total recommended line = %.2f, want 250", sheet.TotalRecommendedLineM)
	}
	replay, err := service.Schedule(dto.ScheduleResurveyRequest{GapIDs: []uint{ids[2], ids[0], ids[3], ids[1]}}, resurveyTestKey, scheduleActor())
	if err != nil {
		t.Fatalf("replay schedule: %v", err)
	}
	if !replay.Idempotent || replay.Sheet.ID != sheet.ID {
		t.Fatalf("replay must return sheet %d idempotently, got %+v", sheet.ID, replay.Sheet)
	}
	var sheetCount int64
	if err := db.Model(&model.ResurveyTaskSheet{}).Count(&sheetCount).Error; err != nil {
		t.Fatal(err)
	}
	if sheetCount != 1 {
		t.Fatalf("sheets = %d, want exactly 1 after replay", sheetCount)
	}
	for _, gap := range gaps {
		var reloaded model.CoverageGap
		if err := db.First(&reloaded, gap.ID).Error; err != nil {
			t.Fatal(err)
		}
		if reloaded.GapState != gap.GapState || reloaded.Version != gap.Version {
			t.Fatalf("gap %d mutated: state %s->%s version %d->%d", gap.ID, gap.GapState, reloaded.GapState, gap.Version, reloaded.Version)
		}
	}
	var audits int64
	if err := db.Model(&model.AuditEvent{}).Where("entity_type = ?", "resurvey_task_sheet").Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if audits != 1 {
		t.Fatalf("resurvey audit events = %d, want 1", audits)
	}
}

func TestScheduleRejectsInvalidBatches(t *testing.T) {
	service, db := newResurveyTestService(t)
	_, _, gaps := seedResurveyFixture(t, db)
	cases := []struct {
		name string
		ids  []uint
		code string
	}{
		{"duplicate snapshots", []uint{gaps[0].ID, gaps[1].ID, gaps[1].ID}, "RESURVEY_GAP_DUPLICATED"},
		{"cross area", []uint{gaps[0].ID, gaps[5].ID}, "RESURVEY_AREA_MISMATCH"},
		{"closed snapshot", []uint{gaps[0].ID, gaps[4].ID}, "RESURVEY_STATE_INVALID"},
		{"missing snapshot", []uint{gaps[0].ID, 99999}, "GAP_SET_INCOMPLETE"},
		{"single snapshot", []uint{gaps[0].ID}, "RESURVEY_BATCH_TOO_SMALL"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := service.Schedule(dto.ScheduleResurveyRequest{GapIDs: testCase.ids}, resurveyTestKey, scheduleActor())
			if err == nil {
				t.Fatalf("expected rejection with %s", testCase.code)
			}
			if code := appErrorCode(t, err); code != testCase.code {
				t.Fatalf("code = %s, want %s", code, testCase.code)
			}
		})
	}
	if _, err := service.Schedule(dto.ScheduleResurveyRequest{GapIDs: []uint{gaps[0].ID, gaps[1].ID}}, "short", scheduleActor()); err == nil {
		t.Fatal("short idempotency key must be rejected")
	} else if code := appErrorCode(t, err); code != "IDEMPOTENCY_KEY_REQUIRED" {
		t.Fatalf("code = %s, want IDEMPOTENCY_KEY_REQUIRED", code)
	}
	var sheetCount int64
	if err := db.Model(&model.ResurveyTaskSheet{}).Count(&sheetCount).Error; err != nil {
		t.Fatal(err)
	}
	if sheetCount != 0 {
		t.Fatalf("rejected batches must not persist sheets, found %d", sheetCount)
	}
	for _, gap := range gaps {
		var reloaded model.CoverageGap
		if err := db.First(&reloaded, gap.ID).Error; err != nil {
			t.Fatal(err)
		}
		if reloaded.GapState != gap.GapState || reloaded.Version != gap.Version {
			t.Fatalf("rejected batch mutated gap %d", gap.ID)
		}
	}
}
