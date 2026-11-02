package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/getmentor/getmentor-api/internal/middleware"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/gin-gonic/gin"
)

type AdminRequestsHandler struct {
	service services.AdminRequestsServiceInterface
}

func NewAdminRequestsHandler(service services.AdminRequestsServiceInterface) *AdminRequestsHandler {
	return &AdminRequestsHandler{service: service}
}

// ListRequests returns one page of client requests.
// Query params: page, pageSize, mentorId, mentorEmail, mentorName, menteeEmail, status.
func (h *AdminRequestsHandler) ListRequests(c *gin.Context) {
	session, err := middleware.GetAdminSession(c)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	page, err := parseOptionalPositiveInt(c.Query("page"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "Invalid page", err)
		return
	}
	pageSize, err := parseOptionalPositiveInt(c.Query("pageSize"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "Invalid pageSize", err)
		return
	}

	filter := &models.AdminRequestsFilter{
		MentorID:    c.Query("mentorId"),
		MentorEmail: c.Query("mentorEmail"),
		MentorName:  c.Query("mentorName"),
		MenteeEmail: c.Query("menteeEmail"),
		Status:      models.RequestStatus(strings.TrimSpace(c.Query("status"))),
		Page:        page,
		PageSize:    pageSize,
	}

	resp, err := h.service.ListRequests(c.Request.Context(), session, filter)
	if err != nil {
		h.respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AdminRequestsHandler) GetRequest(c *gin.Context) {
	session, requestID, ok := adminSessionAndRequestID(c)
	if !ok {
		return
	}

	request, err := h.service.GetRequest(c.Request.Context(), session, requestID)
	if err != nil {
		h.respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.AdminRequestResponse{Request: request})
}

func (h *AdminRequestsHandler) UpdateRequestStatus(c *gin.Context) {
	session, requestID, ok := adminSessionAndRequestID(c)
	if !ok {
		return
	}

	var req models.AdminRequestStatusUpdateRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		respondErrorWithDetails(c, http.StatusBadRequest, "Invalid request body", gin.H{"message": bindErr.Error()}, bindErr)
		return
	}

	request, err := h.service.UpdateRequestStatus(c.Request.Context(), session, requestID, req.Status)
	if err != nil {
		h.respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.AdminRequestResponse{Request: request})
}

// adminSessionAndRequestID reads the admin session and :id route param,
// writing an error response and returning ok=false if either is missing.
func adminSessionAndRequestID(c *gin.Context) (*models.AdminSession, string, bool) {
	session, err := middleware.GetAdminSession(c)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "Unauthorized", err)
		return nil, "", false
	}

	requestID := c.Param("id")
	if requestID == "" {
		respondError(c, http.StatusBadRequest, "Invalid request ID", errors.New("missing route param: id"))
		return nil, "", false
	}
	return session, requestID, true
}

func (h *AdminRequestsHandler) respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrAdminForbiddenAction):
		respondError(c, http.StatusForbidden, "Access denied", err)
	case errors.Is(err, repository.ErrClientRequestNotFound):
		respondError(c, http.StatusNotFound, "Request not found", err)
	case errors.Is(err, repository.ErrInvalidMentorIDFilter):
		respondError(c, http.StatusBadRequest, "Invalid mentorId filter", err)
	case strings.Contains(strings.ToLower(err.Error()), "unsupported"):
		respondError(c, http.StatusBadRequest, "Invalid request", err)
	default:
		respondError(c, http.StatusInternalServerError, "Internal server error", err)
	}
}

func parseOptionalPositiveInt(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, errors.New("must be a positive integer")
	}
	return n, nil
}
