package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmentor/getmentor-api/internal/handlers"
	"github.com/getmentor/getmentor-api/internal/middleware"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAdminRequestsService struct {
	mock.Mock
}

func (m *MockAdminRequestsService) ListRequests(ctx context.Context, session *models.AdminSession, filter *models.AdminRequestsFilter) (*models.AdminRequestsListResponse, error) {
	args := m.Called(ctx, session, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AdminRequestsListResponse), args.Error(1)
}

func (m *MockAdminRequestsService) GetRequest(ctx context.Context, session *models.AdminSession, requestID string) (*models.AdminRequestDetails, error) {
	args := m.Called(ctx, session, requestID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AdminRequestDetails), args.Error(1)
}

func (m *MockAdminRequestsService) UpdateRequestStatus(ctx context.Context, session *models.AdminSession, requestID string, status models.RequestStatus) (*models.AdminRequestDetails, error) {
	args := m.Called(ctx, session, requestID, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AdminRequestDetails), args.Error(1)
}

var testAdminSession = &models.AdminSession{ModeratorID: "admin-1", Role: models.ModeratorRoleAdmin}

func newAdminRequestsRouter(svc *MockAdminRequestsService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := handlers.NewAdminRequestsHandler(svc)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(middleware.AdminSessionContextKey, testAdminSession)
		c.Next()
	})
	router.GET("/admin/requests", h.ListRequests)
	router.GET("/admin/requests/:id", h.GetRequest)
	router.POST("/admin/requests/:id/status", h.UpdateRequestStatus)
	return router
}

func TestAdminRequestsHandler_ListRequests_PassesFilters(t *testing.T) {
	svc := new(MockAdminRequestsService)
	expected := &models.AdminRequestsFilter{
		MentorID:    "42",
		MentorEmail: "m@x.io",
		MentorName:  "Ivan",
		MenteeEmail: "c@x.io",
		Status:      models.StatusDone,
		Page:        2,
		PageSize:    10,
	}
	svc.On("ListRequests", mock.Anything, testAdminSession, expected).Return(&models.AdminRequestsListResponse{
		Requests: []models.AdminRequestListItem{}, Total: 11, Page: 2, PageSize: 10, TotalPages: 2,
	}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/requests?page=2&pageSize=10&mentorId=42&mentorEmail=m@x.io&mentorName=Ivan&menteeEmail=c@x.io&status=done", http.NoBody)
	newAdminRequestsRouter(svc).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body models.AdminRequestsListResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, 11, body.Total)
	assert.Equal(t, 2, body.TotalPages)
	svc.AssertExpectations(t)
}

func TestAdminRequestsHandler_ListRequests_InvalidPage(t *testing.T) {
	svc := new(MockAdminRequestsService)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/requests?page=-1", http.NoBody)
	newAdminRequestsRouter(svc).ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminRequestsHandler_ListRequests_InvalidMentorID(t *testing.T) {
	svc := new(MockAdminRequestsService)
	svc.On("ListRequests", mock.Anything, mock.Anything, mock.Anything).Return(nil, repository.ErrInvalidMentorIDFilter)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/requests?mentorId=abc", http.NoBody)
	newAdminRequestsRouter(svc).ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminRequestsHandler_GetRequest_NotFound(t *testing.T) {
	svc := new(MockAdminRequestsService)
	svc.On("GetRequest", mock.Anything, testAdminSession, "missing").Return(nil, repository.ErrClientRequestNotFound)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/requests/missing", http.NoBody)
	newAdminRequestsRouter(svc).ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAdminRequestsHandler_GetRequest_Forbidden(t *testing.T) {
	svc := new(MockAdminRequestsService)
	svc.On("GetRequest", mock.Anything, testAdminSession, "r1").Return(nil, services.ErrAdminForbiddenAction)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/requests/r1", http.NoBody)
	newAdminRequestsRouter(svc).ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminRequestsHandler_UpdateStatus_AnyToAny(t *testing.T) {
	svc := new(MockAdminRequestsService)
	svc.On("UpdateRequestStatus", mock.Anything, testAdminSession, "r1", models.StatusPending).
		Return(&models.AdminRequestDetails{ID: "r1", Status: models.StatusPending}, nil)

	body, _ := json.Marshal(map[string]string{"status": "pending"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/requests/r1/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	newAdminRequestsRouter(svc).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.AdminRequestResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, models.StatusPending, resp.Request.Status)
	svc.AssertExpectations(t)
}

func TestAdminRequestsHandler_UpdateStatus_InvalidBody(t *testing.T) {
	svc := new(MockAdminRequestsService)
	body, _ := json.Marshal(map[string]string{"status": "reschedule"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/requests/r1/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	newAdminRequestsRouter(svc).ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
