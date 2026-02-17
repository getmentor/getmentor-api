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
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/getmentor/getmentor-api/pkg/db"
	"github.com/getmentor/getmentor-api/pkg/httpclient"
	"github.com/getmentor/getmentor-api/pkg/jwt"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/getmentor/getmentor-api/pkg/tracing"
	"github.com/getmentor/getmentor-api/pkg/yandex"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
)

// registerAPIRoutes registers common API routes for a given router group
func registerAPIRoutes(
	group *gin.RouterGroup,
	cfg *config.Config,
	generalRateLimiter, contactRateLimiter, registrationRateLimiter *middleware.RateLimiter,
	mentorHandler *handlers.MentorHandler,
	contactHandler *handlers.ContactHandler,
	logsHandler *handlers.LogsHandler,
	registrationHandler *handlers.RegistrationHandler,
	reviewHandler *handlers.ReviewHandler,
) {

	publicTokens := []string{
		cfg.Auth.MentorsAPIToken,
		cfg.Auth.MentorsAPITokenInno,
		cfg.Auth.MentorsAPITokenAIKB,
	}
	group.GET("/mentors", generalRateLimiter.Middleware(), middleware.TokenAuthMiddleware(publicTokens...), mentorHandler.GetPublicMentors)
	group.GET("/mentor/:id", generalRateLimiter.Middleware(), middleware.TokenAuthMiddleware(cfg.Auth.MentorsAPIToken, cfg.Auth.MentorsAPITokenInno), mentorHandler.GetPublicMentorByID)
	group.POST("/internal/mentors", generalRateLimiter.Middleware(), middleware.InternalAPIAuthMiddleware(cfg.Auth.InternalMentorsAPI), mentorHandler.GetInternalMentors)
	group.POST("/contact-mentor", contactRateLimiter.Middleware(), middleware.BodySizeLimitMiddleware(100*1024), contactHandler.ContactMentor)
	group.POST("/register-mentor", registrationRateLimiter.Middleware(), middleware.BodySizeLimitMiddleware(10*1024*1024), registrationHandler.RegisterMentor)
	group.POST("/logs", generalRateLimiter.Middleware(), middleware.BodySizeLimitMiddleware(1*1024*1024), logsHandler.ReceiveFrontendLogs)

	// Review routes (public - uses captcha for protection)
	group.GET("/reviews/:requestId/check", generalRateLimiter.Middleware(), reviewHandler.CheckReview)
	group.POST("/reviews/:requestId", contactRateLimiter.Middleware(), middleware.BodySizeLimitMiddleware(100*1024), reviewHandler.SubmitReview)
}

// registerMentorAdminRoutes registers mentor admin routes for authentication, request management, and profile
func registerMentorAdminRoutes(
	router *gin.Engine,
	cfg *config.Config,
	authRateLimiter *middleware.RateLimiter,
	profileRateLimiter *middleware.RateLimiter,
	mentorAuthHandler *handlers.MentorAuthHandler,
	mentorRequestsHandler *handlers.MentorRequestsHandler,
	mentorProfileHandler *handlers.MentorProfileHandler,
	tokenManager *jwt.TokenManager,
) {
	// Skip mentor admin routes if JWT is not configured
	if tokenManager == nil {
		logger.Warn("Mentor admin routes disabled: JWT_SECRET not configured")
		return
	}

	// Authentication routes (public)
	auth := router.Group("/api/v1/auth/mentor")
	auth.POST("/request-login", authRateLimiter.Middleware(), mentorAuthHandler.RequestLogin)
	auth.POST("/verify", mentorAuthHandler.VerifyLogin)
	auth.POST("/logout", mentorAuthHandler.Logout)
	auth.GET("/session", middleware.MentorSessionMiddleware(tokenManager, cfg.MentorSession.CookieDomain, cfg.MentorSession.CookieSecure), mentorAuthHandler.GetSession)

	// Mentor admin routes (protected)
	mentor := router.Group("/api/v1/mentor")
	mentor.Use(middleware.MentorSessionMiddleware(tokenManager, cfg.MentorSession.CookieDomain, cfg.MentorSession.CookieSecure))

	// Request management routes
	mentor.GET("/requests", mentorRequestsHandler.GetRequests)
	mentor.GET("/requests/:id", mentorRequestsHandler.GetRequestByID)
	mentor.POST("/requests/:id/status", mentorRequestsHandler.UpdateStatus)
	mentor.POST("/requests/:id/decline", mentorRequestsHandler.DeclineRequest)

	// Profile routes
	mentor.GET("/profile", mentorProfileHandler.GetProfile)
	mentor.POST("/profile", profileRateLimiter.Middleware(), mentorProfileHandler.UpdateProfile)
	mentor.POST("/profile/picture", profileRateLimiter.Middleware(), middleware.BodySizeLimitMiddleware(10*1024*1024), mentorProfileHandler.UploadPicture)
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	err = logger.Initialize(logger.Config{
		Level:       cfg.Logging.Level,
		LogDir:      cfg.Logging.Dir,
		Environment: cfg.Server.AppEnv,
		ServiceName: cfg.Observability.ServiceName,
	})
	if err != nil {
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
		cfg.Observability.ServiceNamespace,
		cfg.Observability.ServiceVersion,
		cfg.Observability.ServiceInstanceID,
		cfg.Server.AppEnv,
		cfg.Observability.AlloyEndpoint,
	)
	if err != nil {
		logger.Fatal("Failed to initialize tracer", zap.Error(err))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := tracerShutdown(ctx); shutdownErr != nil {
			logger.Error("Failed to shutdown tracer", zap.Error(shutdownErr))
		}
	}()

	// Initialize metrics with service name from config
	metrics.Init(cfg.Observability.ServiceName)

	// Start infrastructure metrics collection
	metrics.RecordInfrastructureMetrics()

	// Initialize PostgreSQL connection pool
	pool, err := db.NewPool(context.Background(), cfg.Database)
	if err != nil {
		logger.Fatal("Failed to initialize database connection pool", zap.Error(err))
	}
	defer pool.Close()

	// NOTE: Database migrations are now run separately via the migrate command
	// Run migrations before starting the app: ./migrate or docker-compose run migrate

	// Initialize Yandex Object Storage client
	var yandexClient *yandex.StorageClient
	if cfg.YandexStorage.AccessKeyID != "" && cfg.YandexStorage.SecretAccessKey != "" {
		yandexClient, err = yandex.NewStorageClient(
			cfg.YandexStorage.AccessKeyID,
			cfg.YandexStorage.SecretAccessKey,
			cfg.YandexStorage.BucketName,
			cfg.YandexStorage.Endpoint,
			cfg.YandexStorage.Region,
		)
		if err != nil {
			logger.Fatal("Failed to initialize Yandex Storage client", zap.Error(err))
		}
	}

	// Initialize repositories (needed for cache fetchers)
	// First create caches with dummy fetchers, then update with real fetchers
	mentorCache := cache.NewMentorCache(
		func(ctx context.Context) ([]*models.Mentor, error) {
			// This fetcher will be replaced after repository is fully initialized
			return []*models.Mentor{}, nil
		},
		func(ctx context.Context, slug string) (*models.Mentor, error) {
			// This fetcher will be replaced after repository is fully initialized
			return &models.Mentor{}, nil
		},
		cfg.Cache.MentorTTLSeconds,
	)
	tagsCache := cache.NewTagsCache(
		func(ctx context.Context) (map[string]string, error) {
			// This fetcher will be replaced after repository is fully initialized
			return make(map[string]string), nil
		},
	)

	// Initialize repositories with pool and caches
	mentorRepo := repository.NewMentorRepository(pool, mentorCache, tagsCache, cfg.Cache.DisableMentorsCache)
	clientRequestRepo := repository.NewClientRequestRepository(pool)

	// Now update cache with actual fetcher functions from repository
	mentorCache = cache.NewMentorCache(
		mentorRepo.FetchAllMentorsFromDB,
		mentorRepo.FetchSingleMentorFromDB,
		cfg.Cache.MentorTTLSeconds,
	)
	tagsCache = cache.NewTagsCache(mentorRepo.FetchAllTagsFromDB)

	// Re-initialize repository with updated caches
	mentorRepo = repository.NewMentorRepository(pool, mentorCache, tagsCache, cfg.Cache.DisableMentorsCache)

	// Initialize mentor cache synchronously before accepting requests
	// This ensures the cache is populated before the container is marked as healthy
	if cfg.Cache.DisableMentorsCache {
		logger.Warn("Mentor cache is DISABLED - reading from database on every request (experimental feature)")
	} else {
		if err := mentorCache.Initialize(); err != nil {
			logger.Fatal("Failed to initialize mentor cache", zap.Error(err))
		}
	}

	// Initialize tags cache synchronously
	if err := tagsCache.Initialize(); err != nil {
		logger.Fatal("Failed to initialize tags cache", zap.Error(err))
	}

	// Initialize HTTP client for external API calls
	httpClient := httpclient.NewStandardClient()

	// Initialize repositories for reviews
	reviewRepo := repository.NewReviewRepository(pool)

	// Initialize services
	mentorService := services.NewMentorService(mentorRepo, cfg)
	contactService := services.NewContactService(clientRequestRepo, mentorRepo, cfg, httpClient)
	profileService := services.NewProfileService(mentorRepo, yandexClient, cfg, httpClient)
	registrationService := services.NewRegistrationService(mentorRepo, yandexClient, cfg, httpClient)
	mcpService := services.NewMCPService(mentorRepo, cfg.Server.BaseURL)
	mentorAuthService := services.NewMentorAuthService(mentorRepo, cfg, httpClient)
	mentorRequestsService := services.NewMentorRequestsService(clientRequestRepo, cfg, httpClient)
	reviewService := services.NewReviewService(reviewRepo, cfg, httpClient)

	// Initialize handlers
	mentorHandler := handlers.NewMentorHandler(mentorService, cfg.Server.BaseURL)
	contactHandler := handlers.NewContactHandler(contactService)
	registrationHandler := handlers.NewRegistrationHandler(registrationService)
	reviewHandler := handlers.NewReviewHandler(reviewService)
	mcpHandler := handlers.NewMCPHandler(mcpService)
	// Health check: If cache is disabled, always return true for cache readiness
	cacheReadyFunc := mentorCache.IsReady
	if cfg.Cache.DisableMentorsCache {
		cacheReadyFunc = func() bool { return true }
	}
	healthHandler := handlers.NewHealthHandler(pool, cacheReadyFunc)
	logsHandler := handlers.NewLogsHandler(cfg.Logging.Dir)
	mentorAuthHandler := handlers.NewMentorAuthHandler(mentorAuthService)
	mentorRequestsHandler := handlers.NewMentorRequestsHandler(mentorRequestsService)
	mentorProfileHandler := handlers.NewMentorProfileHandler(mentorService, profileService)

	// Set up Gin router
	gin.SetMode(cfg.Server.GinMode)
	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(otelgin.Middleware(cfg.Observability.ServiceName)) // OpenTelemetry tracing
	router.Use(middleware.ObservabilityMiddleware())
	router.Use(middleware.SecurityHeadersMiddleware())

	// CORS configuration - SECURITY: Only allow specific origins
	allowedOrigins := cfg.Server.AllowedOrigins
	// Allow localhost in development
	if cfg.IsDevelopment() {
		allowedOrigins = append(allowedOrigins, "http://localhost:3000", "http://127.0.0.1:3000")
	}

	router.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "mentors_api_auth_token", "x-internal-mentors-api-auth-token", "X-Webhook-Secret", "X-Mentor-ID", "X-Auth-Token", "X-CSRF-Token", "traceparent", "tracestate"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true, // Required for mentor session cookies
		MaxAge:           12 * time.Hour,
	}))

	// SECURITY: Rate limiters to prevent abuse and DoS attacks
	// Different limits for different endpoint types
	generalRateLimiter := middleware.NewRateLimiter(100, 200)        // 100 req/sec, burst of 200
	contactRateLimiter := middleware.NewRateLimiter(5, 10)           // 5 req/sec, burst of 10 (prevent spam)
	profileRateLimiter := middleware.NewRateLimiter(10, 20)          // 10 req/sec, burst of 20
	registrationRateLimiter := middleware.NewRateLimiter(0.00667, 3) // 2 req/5min (0.00667 req/sec), burst of 3
	mcpRateLimiter := middleware.NewRateLimiter(20, 40)              // 20 req/sec, burst of 40 (for AI tool usage)
	mentorAuthRateLimiter := middleware.NewRateLimiter(0.00667, 2)   // 2 req/5min (0.00667 req/sec), burst of 2 (login abuse prevention)

	// API routes
	api := router.Group("/api")
	// Utility endpoints (not versioned - operational endpoints)
	api.GET("/healthcheck", generalRateLimiter.Middleware(), healthHandler.Healthcheck)
	api.GET("/metrics", generalRateLimiter.Middleware(), gin.WrapH(promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})))
	// MCP endpoint (for AI tools to search mentors)
	api.POST("/internal/mcp", mcpRateLimiter.Middleware(), middleware.MCPServerAuthMiddleware(cfg.Auth.MCPAuthToken, cfg.Auth.MCPAllowAll), mcpHandler.HandleMCPRequest)

	// API v1 routes
	// SECURITY: Apply body size limits to prevent DoS attacks
	v1 := router.Group("/api/v1")
	registerAPIRoutes(v1, cfg, generalRateLimiter, contactRateLimiter, registrationRateLimiter,
		mentorHandler, contactHandler, logsHandler, registrationHandler, reviewHandler)

	// Mentor admin routes (authentication, request management, and profile)
	registerMentorAdminRoutes(router, cfg, mentorAuthRateLimiter, profileRateLimiter, mentorAuthHandler, mentorRequestsHandler, mentorProfileHandler, mentorAuthService.GetTokenManager())

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
