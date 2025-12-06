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
)

func main() {
	// Create HTTP server with basic routing
	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("/", handleHome)
	mux.HandleFunc("/streamer/{id}", handleStreamerDetail)
	mux.HandleFunc("/login", handleLogin)
	mux.HandleFunc("/auth/callback", handleAuthCallback)
	mux.HandleFunc("/logout", handleLogout)

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

func handleHome(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Home page - Default week view with most viewed streamers")
}

func handleStreamerDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fmt.Fprintf(w, "Streamer detail page for ID: %s", id)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Login page - Google OAuth redirect")
}

func handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OAuth callback handler")
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Logout handler")
}

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
