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
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/getmentor/getmentor-api/pkg/trigger"
	"go.uber.org/zap"
)

var (
	ErrMentorNotFound      = errors.New("mentor not found")
	ErrMentorNotEligible   = errors.New("mentor not eligible for login")
	ErrInvalidLoginToken   = errors.New("invalid or expired login token")
	ErrJWTSecretNotSet     = errors.New("JWT secret not configured")
	ErrTokenGenerationFail = errors.New("failed to generate login token")
)

// MentorAuthService handles mentor authentication
type MentorAuthService struct {
	mentorRepo   *repository.MentorRepository
	config       *config.Config
	tokenManager *jwt.TokenManager
	httpClient   httpclient.Client
}

// NewMentorAuthService creates a new MentorAuthService
func NewMentorAuthService(mentorRepo *repository.MentorRepository, cfg *config.Config, httpClient httpclient.Client) *MentorAuthService {
	var tokenManager *jwt.TokenManager
	if cfg.MentorSession.JWTSecret != "" {
		tokenManager = jwt.NewTokenManager(
			cfg.MentorSession.JWTSecret,
			cfg.MentorSession.JWTIssuer,
			cfg.MentorSession.SessionTTLHours,
		)
	}

	return &MentorAuthService{
		mentorRepo:   mentorRepo,
		config:       cfg,
		tokenManager: tokenManager,
		httpClient:   httpClient,
	}
}

// RequestLogin generates a login token and triggers email sending
func (s *MentorAuthService) RequestLogin(ctx context.Context, email string) (*models.RequestLoginResponse, error) {
	start := time.Now()

	// Find mentor by email
	mentor, err := s.mentorRepo.GetByEmail(ctx, email)
	if err != nil {
		logger.Warn("Login request for unknown email",
			zap.String("email", email),
			zap.Error(err))
		metrics.MentorAuthLoginRequests.WithLabelValues("mentor_not_found").Inc()
		return nil, ErrMentorNotFound
	}

	// Check if mentor is eligible for login (only active or inactive status allowed)
	if mentor.Status != "active" && mentor.Status != "inactive" {
		logger.Warn("Login request for mentor with ineligible status",
			zap.String("email", email),
			zap.String("mentor_id", mentor.AirtableID),
			zap.String("status", mentor.Status))
		metrics.MentorAuthLoginRequests.WithLabelValues("not_eligible").Inc()
		return nil, ErrMentorNotEligible
	}

	// Generate login token
	token, err := generateLoginToken()
	if err != nil {
		logger.Error("Failed to generate login token", zap.Error(err))
		metrics.MentorAuthLoginRequests.WithLabelValues("token_generation_failed").Inc()
		return nil, ErrTokenGenerationFail
	}

	// Calculate expiration
	expiration := time.Now().Add(time.Duration(s.config.MentorSession.LoginTokenTTLMinutes) * time.Minute)

	// Store token in Airtable
	if err := s.mentorRepo.SetLoginToken(ctx, mentor.AirtableID, token, expiration); err != nil {
		logger.Error("Failed to store login token",
			zap.String("mentor_id", mentor.AirtableID),
			zap.Error(err))
		metrics.MentorAuthLoginRequests.WithLabelValues("storage_failed").Inc()
		return nil, fmt.Errorf("failed to store login token: %w", err)
	}

	// Build login URL
	loginURL := fmt.Sprintf("%s/mentor/auth/callback?token=%s", s.config.Server.BaseURL, token)

	// Trigger email sending via webhook
	if s.config.EventTriggers.MentorLoginEmailTriggerURL != "" {
		payload := map[string]interface{}{
			"type": "mentor_login",
			"mentor": map[string]string{
				"email": email,
				"name":  mentor.Name,
			},
			"login_url": loginURL,
		}
		trigger.CallAsyncWithPayload(s.config.EventTriggers.MentorLoginEmailTriggerURL, payload, s.httpClient)
	} else if s.config.Server.AppEnv == "development" {
		// In development mode without email trigger, log the login URL to console
		logger.Info("=== DEVELOPMENT LOGIN URL ===",
			zap.String("mentor_email", email),
			zap.String("mentor_name", mentor.Name),
			zap.String("login_url", loginURL))
	}

	duration := metrics.MeasureDuration(start)
	metrics.MentorAuthLoginDuration.Observe(duration)
	metrics.MentorAuthLoginRequests.WithLabelValues("success").Inc()

	logger.Info("Login token generated",
		zap.String("mentor_id", mentor.AirtableID),
		zap.Duration("duration", time.Since(start)))

	return &models.RequestLoginResponse{
		Success: true,
		Message: "Ссылка для входа отправлена на вашу почту",
	}, nil
}

// VerifyLogin verifies a login token and creates a session
func (s *MentorAuthService) VerifyLogin(ctx context.Context, token string) (*models.MentorSession, string, error) {
	start := time.Now()

	if s.tokenManager == nil {
		logger.Error("JWT secret not configured")
		metrics.MentorAuthVerifyRequests.WithLabelValues("not_configured").Inc()
		return nil, "", ErrJWTSecretNotSet
	}

	// Find mentor by login token
	mentor, storedToken, tokenExp, err := s.mentorRepo.GetByLoginToken(ctx, token)
	if err != nil {
		logger.Warn("Login verification with invalid token", zap.Error(err))
		metrics.MentorAuthVerifyRequests.WithLabelValues("invalid_token").Inc()
		return nil, "", ErrInvalidLoginToken
	}

	// Verify token matches (timing-safe comparison)
	if !jwt.TimingSafeCompare(token, storedToken) {
		logger.Warn("Login token mismatch",
			zap.String("mentor_id", mentor.AirtableID))
		metrics.MentorAuthVerifyRequests.WithLabelValues("token_mismatch").Inc()
		return nil, "", ErrInvalidLoginToken
	}

	// Check expiration
	if time.Now().After(tokenExp) {
		logger.Warn("Login token expired",
			zap.String("mentor_id", mentor.AirtableID),
			zap.Time("expired_at", tokenExp))
		metrics.MentorAuthVerifyRequests.WithLabelValues("expired").Inc()
		return nil, "", ErrInvalidLoginToken
	}

	// Clear the login token (single-use)
	if clearErr := s.mentorRepo.ClearLoginToken(ctx, mentor.AirtableID); clearErr != nil {
		logger.Error("Failed to clear login token",
			zap.String("mentor_id", mentor.AirtableID),
			zap.Error(clearErr))
		// Continue with login even if clearing fails
	}

	// Generate JWT session token
	jwtToken, err := s.tokenManager.GenerateToken(mentor.ID, mentor.AirtableID, "", mentor.Name)
	if err != nil {
		logger.Error("Failed to generate JWT",
			zap.String("mentor_id", mentor.AirtableID),
			zap.Error(err))
		metrics.MentorAuthVerifyRequests.WithLabelValues("jwt_failed").Inc()
		return nil, "", fmt.Errorf("failed to generate session: %w", err)
	}

	now := time.Now()
	session := &models.MentorSession{
		MentorID:   mentor.ID,
		AirtableID: mentor.AirtableID,
		Email:      "",
		Name:       mentor.Name,
		ExpiresAt:  now.Add(s.tokenManager.GetExpirationTime()).Unix(),
		IssuedAt:   now.Unix(),
	}

	duration := metrics.MeasureDuration(start)
	metrics.MentorAuthVerifyDuration.Observe(duration)
	metrics.MentorAuthVerifyRequests.WithLabelValues("success").Inc()

	logger.Info("Login successful",
		zap.String("mentor_id", mentor.AirtableID),
		zap.Duration("duration", time.Since(start)))

	return session, jwtToken, nil
}

// GetSessionTTL returns the session TTL in seconds
func (s *MentorAuthService) GetSessionTTL() int {
	return s.config.MentorSession.SessionTTLHours * 3600
}

// GetCookieDomain returns the cookie domain
func (s *MentorAuthService) GetCookieDomain() string {
	return s.config.MentorSession.CookieDomain
}

// GetCookieSecure returns whether cookies should be secure
func (s *MentorAuthService) GetCookieSecure() bool {
	return s.config.MentorSession.CookieSecure
}

// GetTokenManager returns the JWT token manager
func (s *MentorAuthService) GetTokenManager() *jwt.TokenManager {
	return s.tokenManager
}

// generateLoginToken creates a secure random login token
func generateLoginToken() (string, error) {
	// Generate 32 random bytes (256 bits)
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Format: mtk_{random_hex}_{timestamp}
	timestamp := time.Now().Unix()
	return fmt.Sprintf("mtk_%s_%d", hex.EncodeToString(bytes), timestamp), nil
}
