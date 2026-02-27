package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/analytics"
	"github.com/getmentor/getmentor-api/pkg/httpclient"
	"github.com/getmentor/getmentor-api/pkg/trigger"
)

var (
	ErrAdminForbiddenAction = errors.New("forbidden action for current role")
)

type AdminMentorsService struct {
	mentorRepo     *repository.MentorRepository
	profileService ProfileServiceInterface
	config         *config.Config
	httpClient     httpclient.Client
	tracker        analytics.Tracker
}

func NewAdminMentorsService(
	mentorRepo *repository.MentorRepository,
	profileService ProfileServiceInterface,
	cfg *config.Config,
	httpClient httpclient.Client,
	tracker analytics.Tracker,
) *AdminMentorsService {
	if tracker == nil {
		tracker = analytics.NoopTracker{}
	}

	return &AdminMentorsService{
		mentorRepo:     mentorRepo,
		profileService: profileService,
		config:         cfg,
		httpClient:     httpClient,
		tracker:        tracker,
	}
}

func (s *AdminMentorsService) ListMentors(
	ctx context.Context,
	session *models.AdminSession,
	filter models.MentorModerationFilter,
) ([]models.AdminMentorListItem, error) {
	statuses, err := resolveStatuses(filter, session.Role)
	if err != nil {
		return nil, err
	}

	mentors, err := s.mentorRepo.ListForModeration(ctx, statuses)
	if err != nil {
		return nil, err
	}

	return mentors, nil
}

func (s *AdminMentorsService) GetMentor(
	ctx context.Context,
	session *models.AdminSession,
	mentorID string,
) (*models.AdminMentorDetails, error) {
	mentor, err := s.mentorRepo.GetForModerationByID(ctx, mentorID)
	if err != nil {
		return nil, err
	}
	if session.Role == models.ModeratorRoleModerator && mentor.Status != "pending" {
		return nil, ErrAdminForbiddenAction
	}
	return mentor, nil
}

func (s *AdminMentorsService) UpdateMentorProfile(
	ctx context.Context,
	session *models.AdminSession,
	mentorID string,
	req *models.AdminMentorProfileUpdateRequest,
) (*models.AdminMentorDetails, error) {
	mentor, err := s.GetMentor(ctx, session, mentorID)
	if err != nil {
		s.tracker.Track(ctx, analytics.EventAdminMentorProfileUpdated, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
			"moderator_id":     session.ModeratorID,
			"moderator_role":   string(session.Role),
			"target_mentor_id": mentorID,
			"outcome":          "mentor_not_found_or_forbidden",
		})
		return nil, err
	}

	if session.Role == models.ModeratorRoleModerator && mentor.Status != "pending" {
		s.tracker.Track(ctx, analytics.EventAdminMentorProfileUpdated, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
			"moderator_id":     session.ModeratorID,
			"moderator_role":   string(session.Role),
			"target_mentor_id": mentorID,
			"outcome":          "forbidden",
		})
		return nil, ErrAdminForbiddenAction
	}
	if session.Role != models.ModeratorRoleAdmin && (req.Slug != nil || req.TelegramChatID != nil) {
		s.tracker.Track(ctx, analytics.EventAdminMentorProfileUpdated, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
			"moderator_id":     session.ModeratorID,
			"moderator_role":   string(session.Role),
			"target_mentor_id": mentorID,
			"outcome":          "forbidden",
		})
		return nil, ErrAdminForbiddenAction
	}

	telegram := strings.TrimSpace(req.Telegram)
	telegram = strings.TrimPrefix(telegram, "@")
	telegram = strings.TrimPrefix(telegram, "https://t.me/")
	telegram = strings.TrimPrefix(telegram, "t.me/")

	tagIDs := make([]string, 0, len(req.Tags))
	for _, tagName := range req.Tags {
		tagID, tagErr := s.mentorRepo.GetTagIDByName(ctx, tagName)
		if tagErr == nil && tagID != "" {
			tagIDs = append(tagIDs, tagID)
		}
	}
	if len(tagIDs) == 0 {
		s.tracker.Track(ctx, analytics.EventAdminMentorProfileUpdated, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
			"moderator_id":     session.ModeratorID,
			"moderator_role":   string(session.Role),
			"target_mentor_id": mentorID,
			"outcome":          "invalid_tags",
		})
		return nil, fmt.Errorf("at least one valid tag is required")
	}

	updates := map[string]interface{}{
		"name":         req.Name,
		"email":        req.Email,
		"telegram":     telegram,
		"job_title":    req.Job,
		"workplace":    req.Workplace,
		"experience":   req.Experience,
		"price":        req.Price,
		"details":      req.Description,
		"about":        req.About,
		"competencies": req.Competencies,
		"calendar_url": req.CalendarURL,
	}
	if session.Role == models.ModeratorRoleAdmin {
		if req.Slug != nil {
			slug := strings.TrimSpace(*req.Slug)
			if slug == "" {
				return nil, fmt.Errorf("slug cannot be empty")
			}
			updates["slug"] = slug
		}
		if req.TelegramChatID != nil {
			rawTelegramChatID := strings.TrimSpace(*req.TelegramChatID)
			if rawTelegramChatID == "" {
				updates["telegram_chat_id"] = nil
			} else {
				telegramChatID, parseErr := strconv.ParseInt(rawTelegramChatID, 10, 64)
				if parseErr != nil {
					return nil, fmt.Errorf("telegramChatId must be an integer")
				}
				updates["telegram_chat_id"] = telegramChatID
			}
		}
	}

	if err := s.mentorRepo.Update(ctx, mentorID, updates); err != nil {
		s.tracker.Track(ctx, analytics.EventAdminMentorProfileUpdated, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
			"moderator_id":     session.ModeratorID,
			"moderator_role":   string(session.Role),
			"target_mentor_id": mentorID,
			"outcome":          "update_failed",
		})
		return nil, err
	}
	if err := s.mentorRepo.UpdateMentorTags(ctx, mentorID, tagIDs); err != nil {
		s.tracker.Track(ctx, analytics.EventAdminMentorProfileUpdated, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
			"moderator_id":     session.ModeratorID,
			"moderator_role":   string(session.Role),
			"target_mentor_id": mentorID,
			"outcome":          "tags_update_failed",
		})
		return nil, err
	}

	s.tracker.Track(ctx, analytics.EventAdminMentorProfileUpdated, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
		"moderator_id":     session.ModeratorID,
		"moderator_role":   string(session.Role),
		"target_mentor_id": mentorID,
		"tags_count":       len(tagIDs),
		"outcome":          "success",
	})
	return s.mentorRepo.GetForModerationByID(ctx, mentorID)
}

func (s *AdminMentorsService) ApproveMentor(
	ctx context.Context,
	session *models.AdminSession,
	mentorID string,
) (*models.AdminMentorDetails, error) {
	mentor, err := s.GetMentor(ctx, session, mentorID)
	if err != nil {
		s.trackModerationAction(ctx, session, mentorID, "approve", "mentor_not_found_or_forbidden")
		return nil, err
	}
	if session.Role == models.ModeratorRoleModerator && mentor.Status != "pending" {
		s.trackModerationAction(ctx, session, mentorID, "approve", "forbidden")
		return nil, ErrAdminForbiddenAction
	}

	if err := s.mentorRepo.SetMentorStatus(ctx, mentorID, "active"); err != nil {
		s.trackModerationAction(ctx, session, mentorID, "approve", "update_failed")
		return nil, err
	}
	s.trackModerationAction(ctx, session, mentorID, "approve", "success")
	s.triggerModerationAction("approve", session, mentorID)

	return s.mentorRepo.GetForModerationByID(ctx, mentorID)
}

func (s *AdminMentorsService) DeclineMentor(
	ctx context.Context,
	session *models.AdminSession,
	mentorID string,
) (*models.AdminMentorDetails, error) {
	mentor, err := s.GetMentor(ctx, session, mentorID)
	if err != nil {
		s.trackModerationAction(ctx, session, mentorID, "decline", "mentor_not_found_or_forbidden")
		return nil, err
	}
	if session.Role == models.ModeratorRoleModerator && mentor.Status != "pending" {
		s.trackModerationAction(ctx, session, mentorID, "decline", "forbidden")
		return nil, ErrAdminForbiddenAction
	}

	if err := s.mentorRepo.SetMentorStatus(ctx, mentorID, "declined"); err != nil {
		s.trackModerationAction(ctx, session, mentorID, "decline", "update_failed")
		return nil, err
	}
	s.trackModerationAction(ctx, session, mentorID, "decline", "success")
	s.triggerModerationAction("decline", session, mentorID)

	return s.mentorRepo.GetForModerationByID(ctx, mentorID)
}

func (s *AdminMentorsService) UpdateMentorStatus(
	ctx context.Context,
	session *models.AdminSession,
	mentorID string,
	status string,
) (*models.AdminMentorDetails, error) {
	if session.Role != models.ModeratorRoleAdmin {
		s.tracker.Track(ctx, analytics.EventAdminMentorStatusUpdated, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
			"moderator_id":     session.ModeratorID,
			"moderator_role":   string(session.Role),
			"target_mentor_id": mentorID,
			"requested_status": status,
			"outcome":          "forbidden",
		})
		return nil, ErrAdminForbiddenAction
	}
	if status != "active" && status != "inactive" {
		s.tracker.Track(ctx, analytics.EventAdminMentorStatusUpdated, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
			"moderator_id":     session.ModeratorID,
			"moderator_role":   string(session.Role),
			"target_mentor_id": mentorID,
			"requested_status": status,
			"outcome":          "unsupported_status",
		})
		return nil, fmt.Errorf("unsupported status: %s", status)
	}

	mentor, err := s.mentorRepo.GetForModerationByID(ctx, mentorID)
	if err != nil {
		s.tracker.Track(ctx, analytics.EventAdminMentorStatusUpdated, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
			"moderator_id":     session.ModeratorID,
			"moderator_role":   string(session.Role),
			"target_mentor_id": mentorID,
			"requested_status": status,
			"outcome":          "mentor_not_found",
		})
		return nil, err
	}
	if mentor.Status != "active" && mentor.Status != "inactive" {
		s.tracker.Track(ctx, analytics.EventAdminMentorStatusUpdated, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
			"moderator_id":     session.ModeratorID,
			"moderator_role":   string(session.Role),
			"target_mentor_id": mentorID,
			"from_status":      mentor.Status,
			"requested_status": status,
			"outcome":          "invalid_transition",
		})
		return nil, fmt.Errorf("status toggle is available only for approved mentors")
	}

	if err := s.mentorRepo.SetMentorStatus(ctx, mentorID, status); err != nil {
		s.tracker.Track(ctx, analytics.EventAdminMentorStatusUpdated, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
			"moderator_id":     session.ModeratorID,
			"moderator_role":   string(session.Role),
			"target_mentor_id": mentorID,
			"from_status":      mentor.Status,
			"requested_status": status,
			"outcome":          "update_failed",
		})
		return nil, err
	}
	s.tracker.Track(ctx, analytics.EventAdminMentorStatusUpdated, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
		"moderator_id":     session.ModeratorID,
		"moderator_role":   string(session.Role),
		"target_mentor_id": mentorID,
		"from_status":      mentor.Status,
		"requested_status": status,
		"outcome":          "success",
	})
	return s.mentorRepo.GetForModerationByID(ctx, mentorID)
}

func (s *AdminMentorsService) UploadMentorPicture(
	ctx context.Context,
	session *models.AdminSession,
	mentorID string,
	req *models.UploadProfilePictureRequest,
) (string, error) {
	if session.Role != models.ModeratorRoleAdmin {
		s.tracker.Track(ctx, analytics.EventAdminMentorPictureUploaded, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
			"moderator_id":     session.ModeratorID,
			"moderator_role":   string(session.Role),
			"target_mentor_id": mentorID,
			"outcome":          "forbidden",
		})
		return "", ErrAdminForbiddenAction
	}

	mentor, err := s.mentorRepo.GetForModerationByID(ctx, mentorID)
	if err != nil {
		s.tracker.Track(ctx, analytics.EventAdminMentorPictureUploaded, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
			"moderator_id":     session.ModeratorID,
			"moderator_role":   string(session.Role),
			"target_mentor_id": mentorID,
			"outcome":          "mentor_not_found",
		})
		return "", err
	}
	uploadURL, err := s.profileService.UploadPictureByMentorId(ctx, mentorID, mentor.Slug, req)
	if err != nil {
		s.tracker.Track(ctx, analytics.EventAdminMentorPictureUploaded, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
			"moderator_id":     session.ModeratorID,
			"moderator_role":   string(session.Role),
			"target_mentor_id": mentorID,
			"outcome":          "upload_failed",
		})
		return "", err
	}
	s.tracker.Track(ctx, analytics.EventAdminMentorPictureUploaded, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
		"moderator_id":     session.ModeratorID,
		"moderator_role":   string(session.Role),
		"target_mentor_id": mentorID,
		"url_returned":     strings.TrimSpace(uploadURL) != "",
		"outcome":          "success",
	})

	return uploadURL, nil
}

func (s *AdminMentorsService) triggerModerationAction(action string, session *models.AdminSession, mentorID string) {
	payload := models.AdminModerationTriggerPayload{
		Type:        "mentor_moderation",
		MentorID:    mentorID,
		Action:      action,
		ModeratorID: session.ModeratorID,
		Role:        string(session.Role),
	}
	trigger.CallAsyncWithPayload(s.config.EventTriggers.MentorModerationTriggerURL, payload, s.httpClient)
}

func (s *AdminMentorsService) trackModerationAction(
	ctx context.Context,
	session *models.AdminSession,
	mentorID string,
	action string,
	outcome string,
) {
	s.tracker.Track(ctx, analytics.EventAdminMentorModerationAction, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
		"moderator_id":     session.ModeratorID,
		"moderator_role":   string(session.Role),
		"target_mentor_id": mentorID,
		"action":           action,
		"outcome":          outcome,
	})
}

func resolveStatuses(filter models.MentorModerationFilter, role models.ModeratorRole) ([]string, error) {
	if role == models.ModeratorRoleModerator {
		if filter != models.MentorModerationFilterPending {
			return nil, ErrAdminForbiddenAction
		}
		return []string{"pending"}, nil
	}

	switch filter {
	case models.MentorModerationFilterPending:
		return []string{"pending"}, nil
	case models.MentorModerationFilterApproved:
		return []string{"active", "inactive"}, nil
	case models.MentorModerationFilterDeclined:
		return []string{"declined"}, nil
	default:
		return nil, fmt.Errorf("unsupported filter: %s", filter)
	}
}
