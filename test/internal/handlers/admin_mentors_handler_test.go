package handlers_test

import (
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
	"github.com/stretchr/testify/require"
)

// fakeAdminMentorsService implements only RestoreMentor; other methods are not called.
type fakeAdminMentorsService struct {
	services.AdminMentorsServiceInterface
	restoreErr error
}

func (f *fakeAdminMentorsService) RestoreMentor(_ context.Context, _ *models.AdminSession, mentorID string) (*models.AdminMentorDetails, error) {
	if f.restoreErr != nil {
		return nil, f.restoreErr
	}
	return &models.AdminMentorDetails{MentorID: mentorID, Status: "inactive"}, nil
}

func runRestore(t *testing.T, svc services.AdminMentorsServiceInterface) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(middleware.AdminSessionContextKey, &models.AdminSession{ModeratorID: "a1", Role: models.ModeratorRoleAdmin})
	})
	h := handlers.NewAdminMentorsHandler(svc)
	router.POST("/admin/mentors/:id/restore", h.RestoreMentor)

	req := httptest.NewRequest(http.MethodPost, "/admin/mentors/old-id/restore", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestAdminRestoreMentor_EmailConflict(t *testing.T) {
	w := runRestore(t, &fakeAdminMentorsService{restoreErr: &repository.MentorEmailConflictError{
		MentorID: "new-id", Name: "Ivan", Slug: "ivan-2", Status: "pending",
	}})

	assert.Equal(t, http.StatusConflict, w.Code)
	var body struct {
		Error   string                                 `json:"error"`
		Details models.AdminMentorEmailConflictDetails `json:"details"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "email_conflict", body.Details.Code)
	assert.Equal(t, "new-id", body.Details.MentorID)
	assert.Equal(t, "ivan-2", body.Details.Slug)
	assert.Equal(t, "pending", body.Details.Status)
}

func TestAdminRestoreMentor_Success(t *testing.T) {
	w := runRestore(t, &fakeAdminMentorsService{})
	assert.Equal(t, http.StatusOK, w.Code)
}
