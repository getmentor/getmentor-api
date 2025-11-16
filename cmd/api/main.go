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
	"github.com/getmentor/getmentor-api/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
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

	// Initialize distributed tracing
	tracerShutdown, err := tracing.InitTracer(
		cfg.Observability.ServiceName,
		cfg.Observability.ServiceVersion,
		cfg.Server.AppEnv,
		cfg.Observability.AlloyEndpoint,
	)
	if err != nil {
		logger.Fatal("Failed to initialize tracer", zap.Error(err))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracerShutdown(ctx); err != nil {
			logger.Error("Failed to shutdown tracer", zap.Error(err))
		}
	}()

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
	logsHandler := handlers.NewLogsHandler(cfg.Logging.Dir)

	// Set up Gin router
	gin.SetMode(cfg.Server.GinMode)
	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(otelgin.Middleware(cfg.Observability.ServiceName)) // OpenTelemetry tracing
	router.Use(middleware.ObservabilityMiddleware())
	router.Use(middleware.SecurityHeadersMiddleware())

	// CORS configuration - SECURITY: Only allow specific origins
	allowedOrigins := []string{
		"https://гетментор.рф",
		"https://www.гетментор.рф",
	}
	// Allow localhost in development
	if cfg.Server.AppEnv == "development" {
		allowedOrigins = append(allowedOrigins, "http://localhost:3000", "http://127.0.0.1:3000")
	}

	router.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "mentors_api_auth_token", "x-internal-mentors-api-auth-token", "X-Webhook-Secret", "X-Mentor-ID", "X-Auth-Token", "X-CSRF-Token"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// SECURITY: Rate limiters to prevent abuse and DoS attacks
	// Different limits for different endpoint types
	generalRateLimiter := middleware.NewRateLimiter(100, 200) // 100 req/sec, burst of 200
	contactRateLimiter := middleware.NewRateLimiter(5, 10)    // 5 req/sec, burst of 10 (prevent spam)
	profileRateLimiter := middleware.NewRateLimiter(10, 20)   // 10 req/sec, burst of 20
	webhookRateLimiter := middleware.NewRateLimiter(10, 20)   // 10 req/sec, burst of 20

	// API routes
	// SECURITY: Apply body size limits to prevent DoS attacks
	api := router.Group("/api")
	api.Use(middleware.BodySizeLimitMiddleware(1 * 1024 * 1024)) // Default 1 MB limit
	{
		// Utility endpoints
		api.GET("/healthcheck", generalRateLimiter.Middleware(), healthHandler.Healthcheck)
		api.GET("/metrics", generalRateLimiter.Middleware(), gin.WrapH(promhttp.Handler()))

		// Public mentor endpoints
		publicTokens := []string{
			cfg.Auth.MentorsAPIToken,
			cfg.Auth.MentorsAPITokenInno,
			cfg.Auth.MentorsAPITokenAIKB,
		}
		api.GET("/mentors", generalRateLimiter.Middleware(), middleware.TokenAuthMiddleware(publicTokens...), mentorHandler.GetPublicMentors)
		api.GET("/mentor/:id", generalRateLimiter.Middleware(), middleware.TokenAuthMiddleware(cfg.Auth.MentorsAPIToken, cfg.Auth.MentorsAPITokenInno), mentorHandler.GetPublicMentorByID)

		// Internal mentor endpoint
		api.POST("/internal/mentors", generalRateLimiter.Middleware(), middleware.InternalAPIAuthMiddleware(cfg.Auth.InternalMentorsAPI), mentorHandler.GetInternalMentors)

		// Contact endpoint (smaller limit for forms + stricter rate limit)
		api.POST("/contact-mentor", contactRateLimiter.Middleware(), middleware.BodySizeLimitMiddleware(100*1024), contactHandler.ContactMentor)

		// Profile endpoints (moderate rate limit)
		api.POST("/save-profile", profileRateLimiter.Middleware(), profileHandler.SaveProfile)
		// Profile picture upload needs larger limit for base64-encoded images (10 MB)
		api.POST("/upload-profile-picture", profileRateLimiter.Middleware(), middleware.BodySizeLimitMiddleware(10*1024*1024), profileHandler.UploadProfilePicture)

		// Logs endpoint - receive logs from frontend for centralized collection (moderate rate limit, 1 MB max)
		api.POST("/logs", generalRateLimiter.Middleware(), middleware.BodySizeLimitMiddleware(1*1024*1024), logsHandler.ReceiveFrontendLogs)

		// Webhook endpoint (moderate rate limit)
		api.POST("/webhooks/airtable", webhookRateLimiter.Middleware(), middleware.WebhookAuthMiddleware(cfg.Auth.WebhookSecret), webhookHandler.HandleAirtableWebhook)

		// Revalidate Next.js endpoint
		api.POST("/revalidate-nextjs", webhookRateLimiter.Middleware(), webhookHandler.RevalidateNextJS)
	}

	// Create HTTP server
	// SECURITY: Bind to all interfaces for Docker Compose networking
	// Network isolation is enforced by Docker Compose (backend has no public ports)
	// In Docker Compose, frontend container needs to access backend via service name
	srv := &http.Server{
		Addr:              "0.0.0.0:" + cfg.Server.Port,
		Handler:           router,
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // SECURITY: 1 MB max header size
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
