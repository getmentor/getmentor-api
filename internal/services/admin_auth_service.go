package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/httpclient"
	"github.com/getmentor/getmentor-api/pkg/jwt"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/trigger"
	"go.uber.org/zap"
)

var (
	ErrModeratorNotFound      = errors.New("moderator not found")
	ErrModeratorNotEligible   = errors.New("moderator not eligible for login")
	ErrAdminInvalidLoginToken = errors.New("invalid or expired admin login token")
	ErrAdminJWTSecretNotSet   = errors.New("JWT secret not configured")
	ErrAdminTokenGeneration   = errors.New("failed to generate admin login token")
)

// AdminAuthService handles moderator/admin one-time login flow.
type AdminAuthService struct {
	moderatorRepo *repository.ModeratorRepository
	config        *config.Config
	tokenManager  *jwt.TokenManager
	httpClient    httpclient.Client
}

func NewAdminAuthService(
	moderatorRepo *repository.ModeratorRepository,
	cfg *config.Config,
	httpClient httpclient.Client,
) *AdminAuthService {
	var tokenManager *jwt.TokenManager
	if cfg.MentorSession.JWTSecret != "" {
		tokenManager = jwt.NewTokenManager(
			cfg.MentorSession.JWTSecret,
			cfg.MentorSession.JWTIssuer,
			cfg.MentorSession.SessionTTLHours,
		)
	}

	return &AdminAuthService{
		moderatorRepo: moderatorRepo,
		config:        cfg,
		tokenManager:  tokenManager,
		httpClient:    httpClient,
	}
}

func (s *AdminAuthService) RequestLogin(ctx context.Context, email string) (*models.AdminRequestLoginResponse, error) {
	moderator, err := s.moderatorRepo.GetByEmail(ctx, email)
	if err != nil {
		logger.Warn("Admin login request for unknown email", zap.String("email", email), zap.Error(err))
		return nil, ErrModeratorNotFound
	}
	if !moderator.Role.IsValid() {
		logger.Warn("Admin login request with invalid role",
			zap.String("moderator_id", moderator.ID),
			zap.String("role", string(moderator.Role)))
		return nil, ErrModeratorNotEligible
	}

	token, err := generateAdminLoginToken()
	if err != nil {
		logger.Error("Failed to generate admin login token", zap.Error(err))
		return nil, ErrAdminTokenGeneration
	}

	expiration := time.Now().Add(time.Duration(s.config.MentorSession.LoginTokenTTLMinutes) * time.Minute)
	if err := s.moderatorRepo.SetLoginToken(ctx, moderator.ID, token, expiration); err != nil {
		return nil, fmt.Errorf("failed to store admin login token: %w", err)
	}

	loginURL := fmt.Sprintf("%s/admin/auth/callback?token=%s", s.config.Server.BaseURL, token)
	if s.config.EventTriggers.ModeratorLoginEmailTriggerURL != "" {
		payload := map[string]interface{}{
			"type":            "admin_login",
			"moderator_id":    moderator.ID,
			"moderator_name":  moderator.Name,
			"moderator_email": moderator.Email,
			"login_url":       loginURL,
		}
		trigger.CallAsyncWithPayload(s.config.EventTriggers.ModeratorLoginEmailTriggerURL, payload, s.httpClient)
	} else if s.config.IsDevelopment() {
		logger.Info("=== DEVELOPMENT ADMIN LOGIN URL ===",
			zap.String("moderator_email", moderator.Email),
			zap.String("moderator_name", moderator.Name),
			zap.String("login_url", loginURL))
	}

	return &models.AdminRequestLoginResponse{
		Success: true,
		Message: "Ссылка для входа отправлена на вашу почту",
	}, nil
}

func (s *AdminAuthService) VerifyLogin(ctx context.Context, token string) (*models.AdminSession, string, error) {
	if s.tokenManager == nil {
		return nil, "", ErrAdminJWTSecretNotSet
	}

	moderator, tokenExp, err := s.moderatorRepo.GetByLoginToken(ctx, token)
	if err != nil {
		return nil, "", ErrAdminInvalidLoginToken
	}
	if time.Now().After(tokenExp) {
		return nil, "", ErrAdminInvalidLoginToken
	}
	if !moderator.Role.IsValid() {
		return nil, "", ErrModeratorNotEligible
	}

	if err := s.moderatorRepo.ClearLoginToken(ctx, moderator.ID); err != nil {
		logger.Error("Failed to clear admin login token",
			zap.String("moderator_id", moderator.ID),
			zap.Error(err))
	}

	jwtToken, err := s.tokenManager.GenerateTokenWithRole(
		moderator.ID,
		0,
		moderator.Email,
		moderator.Name,
		string(moderator.Role),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate admin session token: %w", err)
	}

	now := time.Now()
	session := &models.AdminSession{
		ModeratorID: moderator.ID,
		Email:       moderator.Email,
		Name:        moderator.Name,
		Role:        moderator.Role,
		ExpiresAt:   now.Add(s.tokenManager.GetExpirationTime()).Unix(),
		IssuedAt:    now.Unix(),
	}

	return session, jwtToken, nil
}

func (s *AdminAuthService) GetSessionTTL() int {
	return s.config.MentorSession.SessionTTLHours * 3600
}

func (s *AdminAuthService) GetCookieDomain() string {
	return s.config.MentorSession.CookieDomain
}

func (s *AdminAuthService) GetCookieSecure() bool {
	return s.config.MentorSession.CookieSecure
}

func (s *AdminAuthService) GetTokenManager() *jwt.TokenManager {
	return s.tokenManager
}

func generateAdminLoginToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	timestamp := time.Now().Unix()
	return fmt.Sprintf("atk_%s_%d", hex.EncodeToString(bytes), timestamp), nil
}
