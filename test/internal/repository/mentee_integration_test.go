package repository_test

// Integration tests for the mentee dashboard against a real PostgreSQL.
// They run only when TEST_DATABASE_URL points to a database with all migrations applied.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%d@Example.com", prefix, time.Now().UnixNano())
}

// insertRequest inserts a client request directly (status and email under test control).
func insertRequest(t *testing.T, pool *pgxpool.Pool, mentorID *string, email, status string) string {
	t.Helper()
	var id string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO client_requests (mentor_id, email, name, description, level, status)
		 VALUES ($1, $2, 'Mentee', 'please help', 'junior', $3) RETURNING id`,
		mentorID, email, status,
	).Scan(&id))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM client_requests WHERE id = $1`, id) //nolint:errcheck
	})
	return id
}

func cleanupMentee(t *testing.T, pool *pgxpool.Pool, email string) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM mentees WHERE email = $1`, email) //nolint:errcheck
	})
}

func menteeCount(t *testing.T, pool *pgxpool.Pool, email string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM mentees WHERE email = $1`, email).Scan(&n))
	return n
}

func TestMentee_NoRowForUnknownEmail(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewMenteeRepository(pool)
	email := uniqueEmail("unknown")
	cleanupMentee(t, pool, email)

	_, err := repo.SetLoginToken(context.Background(), email, "hash-unknown", time.Now().Add(time.Hour))
	assert.ErrorIs(t, err, repository.ErrMenteeNotFound)
	assert.Equal(t, 0, menteeCount(t, pool, email))
}

func TestMentee_EmailMatchingIsCaseInsensitive(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	repo := repository.NewMenteeRepository(pool)
	mentorID := insertMentor(t, pool, uniqueEmail("mentor"))
	email := uniqueEmail("MiXeD")
	cleanupMentee(t, pool, email)
	insertRequest(t, pool, &mentorID, email, "pending")

	id1, err := repo.SetLoginToken(ctx, email, "hash-a", time.Now().Add(time.Hour)) // mixed case, as stored on the request
	require.NoError(t, err)
	id2, err := repo.SetLoginToken(ctx, toLower(email), "hash-b", time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, id1, id2, "same mentee row for any letter case")
	assert.Equal(t, 1, menteeCount(t, pool, email))

	// The second login replaced the first token.
	_, err = repo.ConsumeLoginToken(ctx, "hash-a")
	assert.ErrorIs(t, err, repository.ErrMenteeNotFound)

	rows, err := repo.ListRequests(ctx, id1)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func TestMentee_LoginTokenSingleUseUnderConcurrency(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	repo := repository.NewMenteeRepository(pool)
	mentorID := insertMentor(t, pool, uniqueEmail("mentor"))
	email := uniqueEmail("race")
	cleanupMentee(t, pool, email)
	insertRequest(t, pool, &mentorID, email, "pending")

	hash := fmt.Sprintf("hash-race-%d", time.Now().UnixNano())
	_, err := repo.SetLoginToken(ctx, email, hash, time.Now().Add(time.Hour))
	require.NoError(t, err)

	const callers = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes, notFound, other := 0, 0, 0
	start := make(chan struct{})
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := repo.ConsumeLoginToken(ctx, hash)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				successes++
			case errors.Is(err, repository.ErrMenteeNotFound):
				notFound++
			default:
				other++
			}
		}()
	}
	close(start)
	wg.Wait()
	assert.Equal(t, 1, successes)
	assert.Equal(t, callers-1, notFound)
	assert.Equal(t, 0, other)
}

func TestMentee_ExpiredTokenRefused(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	repo := repository.NewMenteeRepository(pool)
	mentorID := insertMentor(t, pool, uniqueEmail("mentor"))
	email := uniqueEmail("expired")
	cleanupMentee(t, pool, email)
	insertRequest(t, pool, &mentorID, email, "pending")

	hash := fmt.Sprintf("hash-exp-%d", time.Now().UnixNano())
	_, err := repo.SetLoginToken(ctx, email, hash, time.Now().Add(-time.Second))
	require.NoError(t, err)
	_, err = repo.ConsumeLoginToken(ctx, hash)
	assert.ErrorIs(t, err, repository.ErrMenteeNotFound)
}

func TestMentee_RevokeSessionsBumpsVersion(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	repo := repository.NewMenteeRepository(pool)
	mentorID := insertMentor(t, pool, uniqueEmail("mentor"))
	email := uniqueEmail("logout")
	cleanupMentee(t, pool, email)
	insertRequest(t, pool, &mentorID, email, "pending")

	hash := fmt.Sprintf("hash-logout-%d", time.Now().UnixNano())
	_, err := repo.SetLoginToken(ctx, email, hash, time.Now().Add(time.Hour))
	require.NoError(t, err)
	mentee, err := repo.ConsumeLoginToken(ctx, hash)
	require.NoError(t, err)
	assert.Equal(t, 1, mentee.SessionVersion)

	require.NoError(t, repo.RevokeSessions(ctx, mentee.ID))
	v, err := repo.GetSessionVersion(ctx, mentee.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, v, "a session token carrying sv=1 no longer matches")

	_, err = repo.GetSessionVersion(ctx, "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, repository.ErrMenteeNotFound)
}

// loginMentee creates the mentee row for email and returns its id.
func loginMentee(t *testing.T, repo *repository.MenteeRepository, email string) string {
	t.Helper()
	ctx := context.Background()
	hash := fmt.Sprintf("hash-%s-%d", email, time.Now().UnixNano())
	_, err := repo.SetLoginToken(ctx, email, hash, time.Now().Add(time.Hour))
	require.NoError(t, err)
	m, err := repo.ConsumeLoginToken(ctx, hash)
	require.NoError(t, err)
	return m.ID
}

func TestMentee_SeesOnlyOwnRequestsAndDeletedMentorsAreAnonymous(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	repo := repository.NewMenteeRepository(pool)
	mentors := repository.NewMentorRepository(pool, nil, nil, true)

	visible := insertMentor(t, pool, uniqueEmail("visible"))
	_, err := pool.Exec(ctx, `UPDATE mentors SET telegram_chat_id = 42, calendar_url = 'https://cal.example/a', telegram = 'secret_handle' WHERE id = $1`, visible)
	require.NoError(t, err)
	deleted := insertMentor(t, pool, uniqueEmail("deleted"))
	_, _, err = mentors.SoftDeleteMentor(ctx, deleted, "bye")
	require.NoError(t, err)

	me := uniqueEmail("me")
	other := uniqueEmail("other")
	cleanupMentee(t, pool, me)
	cleanupMentee(t, pool, other)
	mine1 := insertRequest(t, pool, &visible, me, "pending")
	mine2 := insertRequest(t, pool, &deleted, me, "done")
	theirs := insertRequest(t, pool, &visible, other, "pending")

	rows, err := repo.ListRequests(ctx, loginMentee(t, repo, me))
	require.NoError(t, err)
	ids := map[string]bool{}
	for i := range rows {
		ids[rows[i].ID] = true
		card := rows[i].ToDashboardRequest()
		switch rows[i].ID {
		case mine1:
			require.NotNil(t, card.Mentor)
			assert.Equal(t, "Test Mentor", card.Mentor.Name)
			require.NotNil(t, card.Mentor.Slug)
			require.NotNil(t, card.Mentor.CalendarURL)
		case mine2:
			assert.Nil(t, card.Mentor, "deleted mentor is anonymous")
			assert.False(t, card.CanReview)
		}
	}
	assert.True(t, ids[mine1])
	assert.True(t, ids[mine2])
	assert.False(t, ids[theirs], "another mentee's request is never listed")
	assert.Len(t, rows, 2)
}

func TestMentee_ListReadsEveryNullableColumn(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	repo := repository.NewMenteeRepository(pool)
	email := uniqueEmail("nulls")
	cleanupMentee(t, pool, email)

	// Every nullable client_requests column NULL, including mentor_id (mentor row deleted).
	var bareID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO client_requests (email, status) VALUES ($1, 'declined') RETURNING id`, email,
	).Scan(&bareID))
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM client_requests WHERE id = $1`, bareID) }) //nolint:errcheck

	// A done request with a mentor whose optional columns are NULL and a review whose texts are NULL.
	mentorID := insertMentor(t, pool, uniqueEmail("nullmentor"))
	reviewedID := insertRequest(t, pool, &mentorID, email, "done")
	_, err := pool.Exec(ctx, `UPDATE client_requests SET description = NULL, name = NULL, level = NULL WHERE id = $1`, reviewedID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO reviews (client_request_id) VALUES ($1)`, reviewedID)
	require.NoError(t, err)

	rows, err := repo.ListRequests(ctx, loginMentee(t, repo, email))
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for i := range rows {
		card := rows[i].ToDashboardRequest()
		if rows[i].ID == bareID {
			assert.Nil(t, card.Mentor)
			assert.Equal(t, "", card.Details)
		} else {
			require.NotNil(t, card.Review)
			assert.Equal(t, "", card.Review.MentorReview)
			assert.False(t, card.CanReview)
		}
	}
}

func TestMentee_DashboardReviewOwnerOnlySingleUseAndConsumesEmailLink(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	repo := repository.NewMenteeRepository(pool)
	reviews := repository.NewReviewRepository(pool)
	mentorID := insertMentor(t, pool, uniqueEmail("mentor"))

	me, other := uniqueEmail("rv-me"), uniqueEmail("rv-other")
	cleanupMentee(t, pool, me)
	cleanupMentee(t, pool, other)
	mine := insertRequest(t, pool, &mentorID, me, "done")
	theirs := insertRequest(t, pool, &mentorID, other, "done")
	myID := loginMentee(t, repo, me)
	loginMentee(t, repo, other)

	// Before: the emailed link (request id) would accept a review.
	check, err := reviews.CheckCanSubmitReview(ctx, mine)
	require.NoError(t, err)
	require.True(t, check.CanSubmit)

	// Owner-only: another mentee's request and a missing one look the same.
	_, err = repo.CreateReview(ctx, myID, theirs, "great mentor!", "", "")
	assert.ErrorIs(t, err, repository.ErrMenteeRequestNotFound)
	_, err = repo.CreateReview(ctx, myID, "00000000-0000-0000-0000-000000000000", "great mentor!", "", "")
	assert.ErrorIs(t, err, repository.ErrMenteeRequestNotFound)
	_, err = repo.CreateReview(ctx, myID, "not-a-uuid", "great mentor!", "", "")
	assert.ErrorIs(t, err, repository.ErrMenteeRequestNotFound)

	// Single-use under concurrency.
	const callers = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes, exists, other2 := 0, 0, 0
	start := make(chan struct{})
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, createErr := repo.CreateReview(ctx, myID, mine, "great mentor!", "platform", "nothing")
			mu.Lock()
			defer mu.Unlock()
			switch {
			case createErr == nil:
				successes++
			case errors.Is(createErr, repository.ErrMenteeReviewExists):
				exists++
			default:
				other2++
				t.Logf("unexpected error: %v", createErr)
			}
		}()
	}
	close(start)
	wg.Wait()
	assert.Equal(t, 1, successes)
	assert.Equal(t, callers-1, exists)
	assert.Equal(t, 0, other2)

	// After: the emailed link no longer accepts a review.
	check, err = reviews.CheckCanSubmitReview(ctx, mine)
	require.NoError(t, err)
	assert.False(t, check.CanSubmit)

	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reviews WHERE client_request_id = $1`, mine).Scan(&n))
	assert.Equal(t, 1, n)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM reviews WHERE client_request_id = $1`, mine) }) //nolint:errcheck
}

func TestMentee_NoReviewBeforeDoneOrForDeletedMentor(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	repo := repository.NewMenteeRepository(pool)
	mentors := repository.NewMentorRepository(pool, nil, nil, true)
	mentorID := insertMentor(t, pool, uniqueEmail("mentor"))
	email := uniqueEmail("early")
	cleanupMentee(t, pool, email)

	ids := map[string]string{}
	for _, status := range []string{"pending", "contacted", "working", "reschedule", "declined", "unavailable"} {
		ids[status] = insertRequest(t, pool, &mentorID, email, status)
	}
	menteeID := loginMentee(t, repo, email)
	for status, id := range ids {
		_, err := repo.CreateReview(ctx, menteeID, id, "great mentor!", "", "")
		assert.ErrorIs(t, err, repository.ErrMenteeReviewNotAllowed, status)
	}

	gone := insertMentor(t, pool, uniqueEmail("gone"))
	doneWithGone := insertRequest(t, pool, &gone, email, "done")
	_, _, err := mentors.SoftDeleteMentor(ctx, gone, "bye")
	require.NoError(t, err)
	_, err = repo.CreateReview(ctx, menteeID, doneWithGone, "great mentor!", "", "")
	assert.ErrorIs(t, err, repository.ErrMenteeReviewNotAllowed)
}
