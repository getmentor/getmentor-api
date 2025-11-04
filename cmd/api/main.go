package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/cache"
	"github.com/getmentor/getmentor-api/internal/handlers"
	"github.com/getmentor/getmentor-api/internal/middleware"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/getmentor/getmentor-api/pkg/airtable"
	"github.com/getmentor/getmentor-api/pkg/azure"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := logger.Initialize(logger.Config{
		Level:       cfg.Logging.Level,
		LogDir:      cfg.Logging.Dir,
		Environment: cfg.Server.AppEnv,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting GetMentor API",
		zap.String("version", "1.0.0"),
		zap.String("environment", cfg.Server.AppEnv),
	)

	// Start infrastructure metrics collection
	metrics.RecordInfrastructureMetrics()

	// Initialize Airtable client
	airtableClient, err := airtable.NewClient(
		cfg.Airtable.APIKey,
		cfg.Airtable.BaseID,
		cfg.Airtable.WorkOffline,
	)
	if err != nil {
		logger.Fatal("Failed to initialize Airtable client", zap.Error(err))
	}

	// Initialize Azure Storage client
	var azureClient *azure.StorageClient
	if cfg.Azure.ConnectionString != "" {
		azureClient, err = azure.NewStorageClient(
			cfg.Azure.ConnectionString,
			cfg.Azure.ContainerName,
			cfg.Azure.StorageDomain,
		)
		if err != nil {
			logger.Fatal("Failed to initialize Azure Storage client", zap.Error(err))
		}
	}

	// Initialize caches
	mentorCache := cache.NewMentorCache(airtableClient)
	tagsCache := cache.NewTagsCache(airtableClient)

	// Initialize repositories
	mentorRepo := repository.NewMentorRepository(airtableClient, mentorCache, tagsCache)
	clientRequestRepo := repository.NewClientRequestRepository(airtableClient)

	// Initialize services
	mentorService := services.NewMentorService(mentorRepo, cfg)
	contactService := services.NewContactService(clientRequestRepo, mentorRepo, cfg)
	profileService := services.NewProfileService(mentorRepo, azureClient, cfg)
	webhookService := services.NewWebhookService(mentorRepo, cfg)

	// Initialize handlers
	mentorHandler := handlers.NewMentorHandler(mentorService)
	contactHandler := handlers.NewContactHandler(contactService)
	profileHandler := handlers.NewProfileHandler(profileService)
	webhookHandler := handlers.NewWebhookHandler(webhookService)
	healthHandler := handlers.NewHealthHandler()

	// Set up Gin router
	gin.SetMode(cfg.Server.GinMode)
	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.ObservabilityMiddleware())

	// CORS configuration
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "mentors_api_auth_token", "x-internal-mentors-api-auth-token", "X-Webhook-Secret"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// API routes
	api := router.Group("/api")
	{
		// Utility endpoints
		api.GET("/healthcheck", healthHandler.Healthcheck)
		api.GET("/metrics", gin.WrapH(promhttp.Handler()))

		// Public mentor endpoints
		publicTokens := []string{
			cfg.Auth.MentorsAPIToken,
			cfg.Auth.MentorsAPITokenInno,
			cfg.Auth.MentorsAPITokenAIKB,
		}
		api.GET("/mentors", middleware.TokenAuthMiddleware(publicTokens...), mentorHandler.GetPublicMentors)
		api.GET("/mentor/:id", middleware.TokenAuthMiddleware(cfg.Auth.MentorsAPIToken, cfg.Auth.MentorsAPITokenInno), mentorHandler.GetPublicMentorByID)

		// Internal mentor endpoint
		api.POST("/internal/mentors", middleware.InternalAPIAuthMiddleware(cfg.Auth.InternalMentorsAPI), mentorHandler.GetInternalMentors)

		// Contact endpoint
		api.POST("/contact-mentor", contactHandler.ContactMentor)

		// Profile endpoints
		api.POST("/save-profile", profileHandler.SaveProfile)
		api.POST("/upload-profile-picture", profileHandler.UploadProfilePicture)

		// Webhook endpoint
		api.POST("/webhooks/airtable", middleware.WebhookAuthMiddleware(cfg.Auth.WebhookSecret), webhookHandler.HandleAirtableWebhook)

		// Revalidate Next.js endpoint
		api.POST("/revalidate-nextjs", webhookHandler.RevalidateNextJS)
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           router,
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Server started", zap.String("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}
