package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/user/who-live-when/internal/adapter"
	"github.com/user/who-live-when/internal/auth"
	"github.com/user/who-live-when/internal/domain"
	"github.com/user/who-live-when/internal/handler"
	"github.com/user/who-live-when/internal/repository/sqlite"
	"github.com/user/who-live-when/internal/service"
)

func main() {
	// Initialize database
	db, err := sqlite.NewDB("./data/who-live-when.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := sqlite.Migrate(db.DB); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repositories
	streamerRepo := sqlite.NewStreamerRepository(db)
	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	liveStatusRepo := sqlite.NewLiveStatusRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)

	// Initialize platform adapters
	platformAdapters := map[string]domain.PlatformAdapter{
		"youtube": adapter.NewYouTubeAdapter(""),
		"kick":    adapter.NewKickAdapter(),
		"twitch":  adapter.NewTwitchAdapter("", ""),
	}

	// Initialize services
	streamerService := service.NewStreamerService(streamerRepo)
	heatmapService := service.NewHeatmapService(activityRepo, heatmapRepo)
	liveStatusService := service.NewLiveStatusService(streamerRepo, liveStatusRepo, platformAdapters)
	userService := service.NewUserService(userRepo, followRepo, activityRepo)
	tvProgrammeService := service.NewTVProgrammeService(heatmapService, userRepo, followRepo, streamerRepo, activityRepo)

	// Initialize OAuth configuration
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	if redirectURL == "" {
		redirectURL = "http://localhost:8080/auth/callback"
	}

	oauthConfig := auth.NewGoogleOAuthConfig(clientID, clientSecret, redirectURL)
	sessionManager := auth.NewSessionManager("session", false, 86400*7) // 7 days
	stateStore := auth.NewStateStore()

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

	// Create HTTP server with basic routing
	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("/", publicHandler.HandleHome)
	mux.HandleFunc("/streamer/{id}", publicHandler.HandleStreamerDetail)
	mux.HandleFunc("/login", publicHandler.HandleLogin)
	mux.HandleFunc("/auth/callback", publicHandler.HandleAuthCallback)
	mux.HandleFunc("/logout", publicHandler.HandleLogout)

	// Authenticated routes (will be protected by middleware later)
	mux.HandleFunc("/dashboard", handleDashboard)
	mux.HandleFunc("/search", handleSearch)
	mux.HandleFunc("/follow/{id}", handleFollow)
	mux.HandleFunc("/unfollow/{id}", handleUnfollow)
	mux.HandleFunc("/calendar", handleCalendar)

	// Static files (for CSS, JS, images)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Configure server
	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// Placeholder handlers - will be implemented in later tasks

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "User dashboard")
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	fmt.Fprintf(w, "Search handler")
}

func handleFollow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	fmt.Fprintf(w, "Follow streamer: %s", id)
}

func handleUnfollow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	fmt.Fprintf(w, "Unfollow streamer: %s", id)
}

func handleCalendar(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Calendar view")
}
