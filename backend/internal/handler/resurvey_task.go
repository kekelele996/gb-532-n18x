package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"sonar-survey-coverage-planner/backend/internal/dto"
	"sonar-survey-coverage-planner/backend/internal/service"
	"sonar-survey-coverage-planner/backend/pkg/api"
)

type ResurveyTaskHandler struct{ service *service.ResurveyTaskService }

func NewResurveyTaskHandler(service *service.ResurveyTaskService) *ResurveyTaskHandler {
	return &ResurveyTaskHandler{service: service}
}

func (h *ResurveyTaskHandler) Schedule(c *gin.Context) {
	var request dto.ScheduleResurveyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		api.BindError(c, err)
		return
	}
	view, err := h.service.Schedule(request, strings.TrimSpace(c.GetHeader("Idempotency-Key")), actorFrom(c))
	if err != nil {
		writeServiceError(c, err, "补测任务单")
		return
	}
	status := http.StatusCreated
	if view.Idempotent {
		status = http.StatusOK
	}
	api.Success(c, status, view)
}
