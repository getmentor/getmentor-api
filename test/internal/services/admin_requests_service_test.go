package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeAdminRequestsFilter(t *testing.T) {
	f := &models.AdminRequestsFilter{}
	services.NormalizeAdminRequestsFilter(f)
	assert.Equal(t, 1, f.Page)
	assert.Equal(t, models.AdminRequestsDefaultPageSize, f.PageSize)
	assert.Equal(t, 0, f.Offset())

	f = &models.AdminRequestsFilter{Page: 3, PageSize: 1000}
	services.NormalizeAdminRequestsFilter(f)
	assert.Equal(t, models.AdminRequestsMaxPageSize, f.PageSize)
	assert.Equal(t, 2*models.AdminRequestsMaxPageSize, f.Offset())
}

func TestAdminRequestsService_ModeratorForbidden(t *testing.T) {
	svc := services.NewAdminRequestsService(nil, nil)
	session := &models.AdminSession{ModeratorID: "m1", Role: models.ModeratorRoleModerator}
	ctx := context.Background()

	_, err := svc.ListRequests(ctx, session, &models.AdminRequestsFilter{})
	assert.True(t, errors.Is(err, services.ErrAdminForbiddenAction))

	_, err = svc.GetRequest(ctx, session, "id")
	assert.True(t, errors.Is(err, services.ErrAdminForbiddenAction))

	_, err = svc.UpdateRequestStatus(ctx, session, "id", models.StatusDone)
	assert.True(t, errors.Is(err, services.ErrAdminForbiddenAction))
}

func TestAdminRequestsService_RejectsUnknownStatus(t *testing.T) {
	svc := services.NewAdminRequestsService(nil, nil)
	session := &models.AdminSession{ModeratorID: "a1", Role: models.ModeratorRoleAdmin}

	_, err := svc.ListRequests(context.Background(), session, &models.AdminRequestsFilter{Status: "bogus"})
	assert.Error(t, err)

	_, err = svc.UpdateRequestStatus(context.Background(), session, "id", "bogus")
	assert.Error(t, err)
}

func TestIsValidAdminRequestStatus(t *testing.T) {
	for _, s := range models.AdminRequestStatuses {
		assert.True(t, models.IsValidAdminRequestStatus(s))
	}
	assert.False(t, models.IsValidAdminRequestStatus("reschedule"))
}
