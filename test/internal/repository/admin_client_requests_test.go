package repository_test

import (
	"errors"
	"testing"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAdminRequestsWhere_Empty(t *testing.T) {
	where, args, err := repository.BuildAdminRequestsWhere(&models.AdminRequestsFilter{})
	require.NoError(t, err)
	assert.Equal(t, "", where)
	assert.Empty(t, args)
}

func TestBuildAdminRequestsWhere_AllFilters(t *testing.T) {
	where, args, err := repository.BuildAdminRequestsWhere(&models.AdminRequestsFilter{
		MentorID:    "  123 ",
		MentorEmail: "mentor@",
		MentorName:  "Ivan",
		MenteeEmail: "50%_off",
		Status:      models.StatusDone,
	})
	require.NoError(t, err)
	assert.Equal(t,
		"WHERE m.legacy_id = $1 AND m.email::text ILIKE '%' || $2 || '%' AND m.name ILIKE '%' || $3 || '%' AND cr.email::text ILIKE '%' || $4 || '%' AND cr.status = $5",
		where)
	assert.Equal(t, []interface{}{int64(123), "mentor@", "Ivan", `50\%\_off`, "done"}, args)
}

func TestBuildAdminRequestsWhere_MentorUUID(t *testing.T) {
	where, args, err := repository.BuildAdminRequestsWhere(&models.AdminRequestsFilter{
		MentorID: "A1B2C3D4-0000-1111-2222-333344445555",
	})
	require.NoError(t, err)
	assert.Equal(t, "WHERE cr.mentor_id = $1", where)
	assert.Equal(t, []interface{}{"a1b2c3d4-0000-1111-2222-333344445555"}, args)
}

func TestBuildAdminRequestsWhere_InvalidMentorID(t *testing.T) {
	_, _, err := repository.BuildAdminRequestsWhere(&models.AdminRequestsFilter{MentorID: "not-an-id"})
	assert.True(t, errors.Is(err, repository.ErrInvalidMentorIDFilter))
}
