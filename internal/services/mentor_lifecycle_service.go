package services

import (
	"context"
	"errors"
	"strings"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/analytics"
	"github.com/getmentor/getmentor-api/pkg/httpclient"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/trigger"
	"go.uber.org/zap"
)

const (
	MentorLifecycleInitiatorMentor = "mentor"
	MentorLifecycleInitiatorAdmin  = "admin"

	mentorLifecycleActionDeleted  = "deleted"
	mentorLifecycleActionRestored = "restored"

	// Shown to mentees in the "request declined" email for requests closed by a profile delete.
	deletedMentorDeclineComment = "Профиль ментора удалён с платформы, поэтому заявка закрыта. Пожалуйста, выберите другого ментора на getmentor.dev."
)

var (
	// ErrDeleteConfirmationMismatch is returned when the confirmation slug does not match the mentor's slug.
	ErrDeleteConfirmationMismatch = errors.New("confirmation does not match mentor slug")
	// ErrMentorDeleted is returned for any action on a soft-deleted mentor.
	ErrMentorDeleted = errors.New("mentor is deleted")
)

// MentorLifecycleService soft-deletes and restores mentor profiles.
// Both the mentor (self-service) and admins go through it, so the side effects are the same.
type MentorLifecycleService struct {
	mentorRepo *repository.MentorRepository
	config     *config.Config
	httpClient httpclient.Client
	tracker    analytics.Tracker
}

func NewMentorLifecycleService(
	mentorRepo *repository.MentorRepository,
	cfg *config.Config,
	httpClient httpclient.Client,
	tracker analytics.Tracker,
) *MentorLifecycleService {

	if tracker == nil {
		tracker = analytics.NoopTracker{}
	}
	return &MentorLifecycleService{
		mentorRepo: mentorRepo,
		config:     cfg,
		httpClient: httpClient,
		tracker:    tracker,
	}
}

// DeleteMentor soft-deletes a mentor after checking that confirmSlug matches the current slug.
// Active requests are declined (mentees are notified), the mentor leaves the public cache,
// and the lifecycle function notifies the mentor by email and Telegram.
func (s *MentorLifecycleService) DeleteMentor(
	ctx context.Context,
	mentorID string,
	confirmSlug string,
	initiator string,
	moderatorID string,
) error {

	distinctID := s.distinctID(mentorID, initiator, moderatorID)

	mentor, err := s.mentorRepo.GetForModerationByID(ctx, mentorID)
	if err != nil {
		s.track(ctx, analytics.EventMentorProfileDeleted, distinctID, mentorID, initiator, moderatorID, "mentor_not_found", nil)
		return err
	}
	if mentor.DeletedAt != nil {
		s.track(ctx, analytics.EventMentorProfileDeleted, distinctID, mentorID, initiator, moderatorID, "already_deleted", nil)
		return repository.ErrMentorAlreadyDeleted
	}
	if strings.TrimSpace(confirmSlug) != mentor.Slug {
		s.track(ctx, analytics.EventMentorProfileDeleted, distinctID, mentorID, initiator, moderatorID, "confirmation_mismatch", nil)
		return ErrDeleteConfirmationMismatch
	}

	slug, declinedIDs, err := s.mentorRepo.SoftDeleteMentor(ctx, mentorID, deletedMentorDeclineComment)
	if err != nil {
		s.track(ctx, analytics.EventMentorProfileDeleted, distinctID, mentorID, initiator, moderatorID, "delete_failed", nil)
		return err
	}

	if cacheErr := s.mentorRepo.RemoveMentorFromCache(slug); cacheErr != nil {
		logger.Warn("Failed to remove deleted mentor from cache",
			zap.String("mentor_id", mentorID),
			zap.String("slug", slug),
			zap.Error(cacheErr))
	}

	// Mentees of declined requests get the regular "request declined" email
	for _, requestID := range declinedIDs {
		trigger.CallAsync(s.config.EventTriggers.RequestProcessFinishedTriggerURL, requestID, s.httpClient)
	}

	trigger.CallAsyncWithPayload(s.config.EventTriggers.MentorLifecycleTriggerURL, models.MentorLifecycleTriggerPayload{
		Type:                  "mentor_lifecycle",
		Action:                mentorLifecycleActionDeleted,
		MentorID:              mentorID,
		Initiator:             initiator,
		ModeratorID:           moderatorID,
		DeclinedRequestsCount: len(declinedIDs),
	}, s.httpClient)

	s.track(ctx, analytics.EventMentorProfileDeleted, distinctID, mentorID, initiator, moderatorID, "success", map[string]interface{}{
		"from_status":             mentor.Status,
		"declined_requests_count": len(declinedIDs),
	})
	logger.Info("Mentor profile deleted",
		zap.String("mentor_id", mentorID),
		zap.String("initiator", initiator),
		zap.String("moderator_id", moderatorID),
		zap.Int("declined_requests", len(declinedIDs)))

	return nil
}

// RestoreMentor clears deleted_at, sets status to inactive and notifies the mentor.
func (s *MentorLifecycleService) RestoreMentor(ctx context.Context, mentorID string, moderatorID string) error {
	distinctID := analytics.ModeratorDistinctID(moderatorID)

	if err := s.mentorRepo.RestoreMentor(ctx, mentorID); err != nil {
		outcome := "restore_failed"
		var conflict *repository.MentorEmailConflictError
		if errors.As(err, &conflict) {
			outcome = "email_conflict"
		} else if errors.Is(err, repository.ErrMentorNotDeleted) {
			outcome = "not_deleted"
		} else if strings.Contains(strings.ToLower(err.Error()), "not found") {
			outcome = "mentor_not_found"
		}
		s.track(ctx, analytics.EventMentorProfileRestored, distinctID, mentorID, MentorLifecycleInitiatorAdmin, moderatorID, outcome, nil)
		return err
	}

	trigger.CallAsyncWithPayload(s.config.EventTriggers.MentorLifecycleTriggerURL, models.MentorLifecycleTriggerPayload{
		Type:        "mentor_lifecycle",
		Action:      mentorLifecycleActionRestored,
		MentorID:    mentorID,
		Initiator:   MentorLifecycleInitiatorAdmin,
		ModeratorID: moderatorID,
	}, s.httpClient)

	s.track(ctx, analytics.EventMentorProfileRestored, distinctID, mentorID, MentorLifecycleInitiatorAdmin, moderatorID, "success", nil)
	logger.Info("Mentor profile restored",
		zap.String("mentor_id", mentorID),
		zap.String("moderator_id", moderatorID))

	return nil
}

func (s *MentorLifecycleService) distinctID(mentorID, initiator, moderatorID string) string {
	if initiator == MentorLifecycleInitiatorAdmin {
		return analytics.ModeratorDistinctID(moderatorID)
	}
	return analytics.MentorDistinctID(mentorID)
}

func (s *MentorLifecycleService) track(
	ctx context.Context,
	event string,
	distinctID string,
	mentorID string,
	initiator string,
	moderatorID string,
	outcome string,
	extra map[string]interface{},
) {

	properties := map[string]interface{}{
		"mentor_id": mentorID,
		"initiator": initiator,
		"outcome":   outcome,
	}
	if moderatorID != "" {
		properties["moderator_id"] = moderatorID
	}
	for key, value := range extra {
		properties[key] = value
	}
	s.tracker.Track(ctx, event, distinctID, properties)
}
