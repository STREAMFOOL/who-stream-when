package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"who-live-when/internal/adapter"
	"who-live-when/internal/auth"
	"who-live-when/internal/config"
	"who-live-when/internal/domain"
	"who-live-when/internal/handler"
	"who-live-when/internal/repository/sqlite"
	"who-live-when/internal/service"
)

func main() {
	// Load configuration from environment variables
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Log configuration (excluding secrets)
	cfg.LogConfiguration()

	// Initialize SQLite database with WAL mode and connection pooling
	db, err := sqlite.NewDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Run database migrations to ensure schema is up to date
	if err := sqlite.Migrate(db.DB); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize data access layer (repositories)
	streamerRepo := sqlite.NewStreamerRepository(db)
	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	liveStatusRepo := sqlite.NewLiveStatusRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)

	// Initialize platform adapters for external streaming APIs
	platformAdapters := map[string]domain.PlatformAdapter{
		"youtube": adapter.NewYouTubeAdapter(cfg.YouTubeAPIKey),
		"kick":    adapter.NewKickAdapter(),
		"twitch":  adapter.NewTwitchAdapter(cfg.TwitchClientID, cfg.TwitchSecret),
	}

	// Initialize business logic layer (services)
	// Services implement domain logic and orchestrate between repositories and adapters
	streamerService := service.NewStreamerService(streamerRepo)
	heatmapService := service.NewHeatmapService(activityRepo, heatmapRepo)
	liveStatusService := service.NewLiveStatusService(streamerRepo, liveStatusRepo, platformAdapters)
	userService := service.NewUserService(userRepo, followRepo, activityRepo)
	tvProgrammeService := service.NewTVProgrammeService(heatmapService, userRepo, followRepo, streamerRepo, activityRepo)

	// Initialize Google OAuth for authentication
	oauthConfig := auth.NewGoogleOAuthConfig(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
	sessionManager := auth.NewSessionManager(cfg.SessionSecret, false, cfg.SessionDuration)
	stateStore := auth.NewStateStore()

	// Initialize multi-platform search service
	searchService := service.NewSearchService(
		platformAdapters["youtube"],
		platformAdapters["kick"],
		platformAdapters["twitch"],
	)

	// Initialize handlers
	publicHandler := handler.NewPublicHandler(
		tvProgrammeService,
		streamerService,
		liveStatusService,
		heatmapService,
		userService,
		oauthConfig,
		sessionManager,
		stateStore,
	)

	authenticatedHandler := handler.NewAuthenticatedHandler(
		tvProgrammeService,
		streamerService,
		liveStatusService,
		heatmapService,
		userService,
		searchService,
		sessionManager,
	)

	// Set up HTTP routing
	mux := http.NewServeMux()

	// Public routes (accessible without authentication)
	mux.HandleFunc("/", publicHandler.HandleHome)
	mux.HandleFunc("/streamer/{id}", publicHandler.HandleStreamerDetail)
	mux.HandleFunc("/login", publicHandler.HandleLogin)
	mux.HandleFunc("/auth/callback", publicHandler.HandleAuthCallback)
	mux.HandleFunc("/logout", publicHandler.HandleLogout)

	// Authenticated routes (require valid session)
	mux.HandleFunc("/dashboard", authenticatedHandler.RequireAuth(authenticatedHandler.HandleDashboard))
	mux.HandleFunc("/search", authenticatedHandler.RequireAuth(authenticatedHandler.HandleSearch))
	mux.HandleFunc("/follow/{id}", authenticatedHandler.RequireAuth(authenticatedHandler.HandleFollow))
	mux.HandleFunc("/unfollow/{id}", authenticatedHandler.RequireAuth(authenticatedHandler.HandleUnfollow))
	mux.HandleFunc("/calendar", authenticatedHandler.RequireAuth(authenticatedHandler.HandleCalendar))

	// API routes (JSON responses, also require authentication)
	mux.HandleFunc("/api/search", authenticatedHandler.RequireAuth(authenticatedHandler.HandleSearchAPI))

	// Static file serving for CSS, JavaScript, and images
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Configure HTTP server with timeouts to prevent resource exhaustion
	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      mux,
		ReadTimeout:  15 * time.Second, // Max time to read request
		WriteTimeout: 15 * time.Second, // Max time to write response
		IdleTimeout:  60 * time.Second, // Max time for keep-alive connections
	}

	// Start server in background goroutine
	go func() {
		log.Printf("Starting server on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Gracefully shutdown server with 30-second timeout
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
