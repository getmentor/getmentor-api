package models_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strp(s string) *string { return &s }

func visibleMentorRow(status string) models.MenteeRequestRow {
	return models.MenteeRequestRow{
		ID:                   "11111111-1111-1111-1111-111111111111",
		Status:               status,
		Description:          strp("help me"),
		CreatedAt:            time.Now(),
		MentorID:             strp("m-1"),
		MentorName:           strp("Анна"),
		MentorSlug:           strp("anna"),
		MentorCalendarURL:    strp("https://calendly.com/anna"),
		MentorStatus:         strp("active"),
		MentorHasTelegramBot: true,
		DeclineReason:        strp("no_time"),
		DeclineComment:       strp("busy"),
	}
}

func TestMenteeGroupForStatus(t *testing.T) {
	cases := map[string]models.MenteeRequestGroup{
		"pending":     models.MenteeGroupPending,
		"contacted":   models.MenteeGroupActive,
		"working":     models.MenteeGroupActive,
		"reschedule":  models.MenteeGroupActive,
		"done":        models.MenteeGroupDone,
		"declined":    models.MenteeGroupClosed,
		"unavailable": models.MenteeGroupClosed,
	}
	for status, group := range cases {
		assert.Equal(t, group, models.MenteeGroupForStatus(status), status)
	}
}

func TestDashboard_CalendarURLOnlyForPendingAndActive(t *testing.T) {
	for _, status := range []string{"pending", "contacted", "working", "reschedule"} {
		row := visibleMentorRow(status)
		card := row.ToDashboardRequest()
		require.NotNil(t, card.Mentor)
		require.NotNil(t, card.Mentor.CalendarURL, status)
		assert.Equal(t, "https://calendly.com/anna", *card.Mentor.CalendarURL)
	}
	for _, status := range []string{"done", "declined", "unavailable"} {
		row := visibleMentorRow(status)
		card := row.ToDashboardRequest()
		require.NotNil(t, card.Mentor)
		assert.Nil(t, card.Mentor.CalendarURL, status)
	}
}

func TestSafeCalendarURL(t *testing.T) {
	assert.Nil(t, models.SafeCalendarURL(nil))
	assert.Nil(t, models.SafeCalendarURL(strp("")))
	assert.Nil(t, models.SafeCalendarURL(strp("   ")))
	assert.Nil(t, models.SafeCalendarURL(strp("javascript:alert(1)")))
	assert.Nil(t, models.SafeCalendarURL(strp("data:text/html,hi")))
	assert.Nil(t, models.SafeCalendarURL(strp("calendly.com/anna")))
	assert.Nil(t, models.SafeCalendarURL(strp("//calendly.com/anna")))
	assert.Equal(t, "http://cal.example/x", *models.SafeCalendarURL(strp(" http://cal.example/x ")))
	assert.Equal(t, "https://cal.example/x", *models.SafeCalendarURL(strp("https://cal.example/x")))
}

func TestDashboard_SlugOnlyWhilePubliclyVisible(t *testing.T) {
	row := visibleMentorRow("pending")
	assert.Equal(t, "anna", *row.ToDashboardRequest().Mentor.Slug)

	inactive := visibleMentorRow("pending")
	inactive.MentorStatus = strp("inactive")
	card := inactive.ToDashboardRequest()
	assert.Equal(t, "Анна", card.Mentor.Name)
	assert.Nil(t, card.Mentor.Slug)

	noBot := visibleMentorRow("pending")
	noBot.MentorHasTelegramBot = false
	assert.Nil(t, noBot.ToDashboardRequest().Mentor.Slug)
}

func TestDashboard_DeletedMentorIsAnonymous(t *testing.T) {
	deleted := visibleMentorRow("done")
	now := time.Now()
	deleted.MentorDeletedAt = &now
	card := deleted.ToDashboardRequest()
	assert.Nil(t, card.Mentor)
	assert.False(t, card.CanReview)

	// mentor row gone entirely (client_requests.mentor_id ON DELETE SET NULL)
	gone := models.MenteeRequestRow{ID: "x", Status: "pending", CreatedAt: now}
	card = gone.ToDashboardRequest()
	assert.Nil(t, card.Mentor)
	assert.False(t, card.CanReview)
}

func TestDashboard_CanReview(t *testing.T) {
	done := visibleMentorRow("done")
	assert.True(t, done.ToDashboardRequest().CanReview)

	reviewed := visibleMentorRow("done")
	reviewed.HasReview = true
	reviewed.ReviewMentorReview = strp("great mentor")
	card := reviewed.ToDashboardRequest()
	assert.False(t, card.CanReview)
	require.NotNil(t, card.Review)
	assert.Equal(t, "great mentor", card.Review.MentorReview)

	for _, status := range []string{"pending", "contacted", "working", "reschedule", "declined", "unavailable"} {
		row := visibleMentorRow(status)
		assert.False(t, row.ToDashboardRequest().CanReview, status)
	}
}

func TestDashboard_DeclineInfoOnlyForClosed(t *testing.T) {
	closed := visibleMentorRow("declined")
	card := closed.ToDashboardRequest()
	assert.Equal(t, "no_time", *card.DeclineReason)
	assert.Equal(t, "busy", *card.DeclineComment)

	active := visibleMentorRow("working")
	card = active.ToDashboardRequest()
	assert.Nil(t, card.DeclineReason)
	assert.Nil(t, card.DeclineComment)
}

// The API response has no field that could carry a mentor's email or messenger handle.
func TestDashboard_JSONHasNoMentorContactFields(t *testing.T) {
	row := visibleMentorRow("pending")
	body, err := json.Marshal(row.ToDashboardRequest())
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &decoded))
	mentor, ok := decoded["mentor"].(map[string]interface{})
	require.True(t, ok)
	keys := []string{}
	for k := range mentor {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, []string{"name", "slug", "calendarUrl"}, keys)
	for _, banned := range []string{"email", "telegram", "Email", "Telegram"} {
		assert.NotContains(t, string(body), banned)
	}
}
