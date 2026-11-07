package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmentor/getmentor-api/internal/middleware"
	"github.com/getmentor/getmentor-api/pkg/jwt"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAccessChecker struct {
	accessible bool
	err        error
	calledWith string
}

func (f *fakeAccessChecker) IsMentorAccessible(_ context.Context, mentorID string) (bool, error) {
	f.calledWith = mentorID
	return f.accessible, f.err
}

func runMentorSession(t *testing.T, checker middleware.MentorAccessChecker) *httptest.ResponseRecorder {
	t.Helper()
	tm := jwt.NewTokenManager("test-secret-test-secret-test-secret", "test", 1)
	token, err := tm.GenerateToken("mentor-uuid", 42, "", "Mentor")
	require.NoError(t, err)

	router := gin.New()
	router.Use(middleware.MentorSessionMiddleware(tm, "", false, checker))
	router.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.AddCookie(&http.Cookie{Name: middleware.MentorSessionCookieName, Value: token})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestMentorSessionMiddleware_AllowsAccessibleMentor(t *testing.T) {
	checker := &fakeAccessChecker{accessible: true}
	w := runMentorSession(t, checker)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "mentor-uuid", checker.calledWith)
}

func TestMentorSessionMiddleware_RejectsDeletedMentor(t *testing.T) {
	w := runMentorSession(t, &fakeAccessChecker{accessible: false})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	// Session cookie is cleared
	assert.Contains(t, w.Header().Get("Set-Cookie"), middleware.MentorSessionCookieName+"=;")
}

func TestMentorSessionMiddleware_CheckerError(t *testing.T) {
	w := runMentorSession(t, &fakeAccessChecker{err: errors.New("db down")})
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestMentorSessionMiddleware_NilCheckerSkipsCheck(t *testing.T) {
	w := runMentorSession(t, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}
