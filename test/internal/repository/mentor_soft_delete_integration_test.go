package repository_test

// Integration tests for mentor soft delete against a real PostgreSQL.
// They run only when TEST_DATABASE_URL points to a database with all migrations applied.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

var mentorSeq int

func insertMentor(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	mentorSeq++
	slug := fmt.Sprintf("it-mentor-%d-%d", time.Now().UnixNano(), mentorSeq)
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO mentors (slug, name, status, email) VALUES ($1, 'Test Mentor', 'active', $2) RETURNING id`,
		slug, email,
	).Scan(&id)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM client_requests WHERE mentor_id = $1`, id) //nolint:errcheck
		_, _ = pool.Exec(ctx, `DELETE FROM mentors WHERE id = $1`, id)                //nolint:errcheck
	})
	return id
}

func newRequest(mentorID string) *models.ClientRequest {
	return &models.ClientRequest{MentorID: mentorID, Email: "mentee@example.com", Name: "Mentee", Level: "junior", Description: "help"}
}

func requestStatus(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var status string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT status FROM client_requests WHERE id = $1`, id).Scan(&status))
	return status
}

func TestSoftDelete_DeclinesRequestsAndBlocksNewOnes(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	mentors := repository.NewMentorRepository(pool, nil, nil, true)
	requests := repository.NewClientRequestRepository(pool)

	mentorID := insertMentor(t, pool, fmt.Sprintf("it-%d@example.com", time.Now().UnixNano()))
	reqID, err := requests.Create(ctx, newRequest(mentorID))
	require.NoError(t, err)

	_, declined, err := mentors.SoftDeleteMentor(ctx, mentorID, "deleted")
	require.NoError(t, err)
	assert.Equal(t, []string{reqID}, declined)
	assert.Equal(t, "declined", requestStatus(t, pool, reqID))

	_, err = requests.Create(ctx, newRequest(mentorID))
	assert.True(t, errors.Is(err, repository.ErrMentorUnavailable), "got %v", err)

	accessible, err := mentors.IsMentorAccessible(ctx, mentorID)
	require.NoError(t, err)
	assert.False(t, accessible)

	_, _, err = mentors.SoftDeleteMentor(ctx, mentorID, "deleted")
	assert.True(t, errors.Is(err, repository.ErrMentorAlreadyDeleted))
}

// A request that starts while a delete is in progress must not slip through.
func TestSoftDelete_CreateWaitsForInFlightDelete(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	requests := repository.NewClientRequestRepository(pool)
	mentorID := insertMentor(t, pool, fmt.Sprintf("it-%d@example.com", time.Now().UnixNano()))

	// Same locking as SoftDeleteMentor, held open
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `SELECT id FROM mentors WHERE id = $1 FOR UPDATE`, mentorID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE mentors SET deleted_at = NOW() WHERE id = $1`, mentorID)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, createErr := requests.Create(ctx, newRequest(mentorID))
		done <- createErr
	}()

	select {
	case err = <-done:
		t.Fatalf("Create did not wait for the delete: %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	require.NoError(t, tx.Commit(ctx))
	err = <-done
	assert.True(t, errors.Is(err, repository.ErrMentorUnavailable), "got %v", err)
}

// A request created just before the delete commits must be declined by the delete.
func TestSoftDelete_DeclinesRequestCommittedDuringDelete(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	mentors := repository.NewMentorRepository(pool, nil, nil, true)
	mentorID := insertMentor(t, pool, fmt.Sprintf("it-%d@example.com", time.Now().UnixNano()))

	// Same locking as ClientRequestRepository.Create, held open
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `SELECT id FROM mentors WHERE id = $1 AND deleted_at IS NULL FOR SHARE`, mentorID)
	require.NoError(t, err)
	var reqID string
	require.NoError(t, tx.QueryRow(ctx,
		`INSERT INTO client_requests (mentor_id, email, name, status) VALUES ($1, 'm@example.com', 'M', 'pending') RETURNING id`,
		mentorID,
	).Scan(&reqID))

	type result struct {
		declined []string
		err      error
	}
	done := make(chan result, 1)
	go func() {
		_, declined, deleteErr := mentors.SoftDeleteMentor(ctx, mentorID, "deleted")
		done <- result{declined, deleteErr}
	}()

	select {
	case res := <-done:
		t.Fatalf("delete did not wait for the request: %+v", res)
	case <-time.After(300 * time.Millisecond):
	}

	require.NoError(t, tx.Commit(ctx))
	res := <-done
	require.NoError(t, res.err)
	assert.Equal(t, []string{reqID}, res.declined)
	assert.Equal(t, "declined", requestStatus(t, pool, reqID))
}

func TestRestore_BlockedByProfileWithSameEmail(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	mentors := repository.NewMentorRepository(pool, nil, nil, true)
	email := fmt.Sprintf("it-%d@example.com", time.Now().UnixNano())

	oldID := insertMentor(t, pool, email)
	_, _, err := mentors.SoftDeleteMentor(ctx, oldID, "deleted")
	require.NoError(t, err)

	// The mentor registers again; email is CITEXT, so case must not matter
	newID := insertMentor(t, pool, "IT-UPPER-"+email)
	_, err = pool.Exec(ctx, `UPDATE mentors SET email = upper($1), status = 'pending' WHERE id = $2`, email, newID)
	require.NoError(t, err)

	err = mentors.RestoreMentor(ctx, oldID)
	var conflict *repository.MentorEmailConflictError
	require.True(t, errors.As(err, &conflict), "got %v", err)
	assert.Equal(t, newID, conflict.MentorID)
	assert.Equal(t, "pending", conflict.Status)

	accessible, err := mentors.IsMentorAccessible(ctx, oldID)
	require.NoError(t, err)
	assert.False(t, accessible, "failed restore must leave the profile deleted")

	// Once the new profile is deleted too, the old one can be restored
	_, _, err = mentors.SoftDeleteMentor(ctx, newID, "deleted")
	require.NoError(t, err)
	require.NoError(t, mentors.RestoreMentor(ctx, oldID))

	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM mentors WHERE id = $1`, oldID).Scan(&status))
	assert.Equal(t, "inactive", status)

	err = mentors.RestoreMentor(ctx, oldID)
	assert.True(t, errors.Is(err, repository.ErrMentorNotDeleted))
}
