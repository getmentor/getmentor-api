package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/stretchr/testify/assert"
)

type fakeLifecycle struct {
	deleteCalls  int
	restoreCalls int
}

func (f *fakeLifecycle) DeleteMentor(_ context.Context, _, _, _, _ string) error {
	f.deleteCalls++
	return nil
}

func (f *fakeLifecycle) RestoreMentor(_ context.Context, _, _ string) error {
	f.restoreCalls++
	return nil
}

func TestAdminMentorsService_DeleteRestoreRequireAdmin(t *testing.T) {
	lifecycle := &fakeLifecycle{}
	svc := services.NewAdminMentorsService(nil, nil, lifecycle, nil, nil, nil)
	session := &models.AdminSession{ModeratorID: "m1", Role: models.ModeratorRoleModerator}
	ctx := context.Background()

	_, err := svc.DeleteMentor(ctx, session, "id", "slug")
	assert.True(t, errors.Is(err, services.ErrAdminForbiddenAction))

	_, err = svc.RestoreMentor(ctx, session, "id")
	assert.True(t, errors.Is(err, services.ErrAdminForbiddenAction))

	assert.Equal(t, 0, lifecycle.deleteCalls)
	assert.Equal(t, 0, lifecycle.restoreCalls)
}

func TestAdminMentorsService_ModeratorCannotListDeleted(t *testing.T) {
	svc := services.NewAdminMentorsService(nil, nil, &fakeLifecycle{}, nil, nil, nil)
	session := &models.AdminSession{ModeratorID: "m1", Role: models.ModeratorRoleModerator}

	_, err := svc.ListMentors(context.Background(), session, models.MentorModerationFilterDeleted)
	assert.True(t, errors.Is(err, services.ErrAdminForbiddenAction))
}

func TestMentorModerationFilter_Deleted(t *testing.T) {
	assert.True(t, models.MentorModerationFilterDeleted.IsValid())
	assert.False(t, models.MentorModerationFilter("bogus").IsValid())
}
