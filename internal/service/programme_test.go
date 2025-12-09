package service

import (
	"context"
	"testing"
	"time"

	"who-live-when/internal/domain"
)

func TestProgrammeService_CreateCustomProgramme(t *testing.T) {
	ctx := context.Background()
	programmeRepo := newProgMockProgrammeRepo()
	streamerRepo := newProgMockStreamerRepo()
	followRepo := newProgMockFollowRepo()
	heatmapService := newProgMockHeatmapSvc()

	service := NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

	streamerIDs := []string{"streamer-1", "streamer-2"}
	programme, err := service.CreateCustomProgramme(ctx, "user-1", streamerIDs)
	if err != nil {
		t.Fatalf("CreateCustomProgramme failed: %v", err)
	}

	if programme.UserID != "user-1" {
		t.Errorf("Expected UserID 'user-1', got '%s'", programme.UserID)
	}

	if len(programme.StreamerIDs) != 2 {
		t.Errorf("Expected 2 streamer IDs, got %d", len(programme.StreamerIDs))
	}
}

func TestProgrammeService_CreateCustomProgramme_EmptyUserID(t *testing.T) {
	ctx := context.Background()
	programmeRepo := newProgMockProgrammeRepo()
	streamerRepo := newProgMockStreamerRepo()
	followRepo := newProgMockFollowRepo()
	heatmapService := newProgMockHeatmapSvc()

	service := NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

	_, err := service.CreateCustomProgramme(ctx, "", []string{"streamer-1"})
	if err == nil {
		t.Error("Expected error for empty user ID")
	}
}

func TestProgrammeService_GetCustomProgramme(t *testing.T) {
	ctx := context.Background()
	programmeRepo := newProgMockProgrammeRepo()
	streamerRepo := newProgMockStreamerRepo()
	followRepo := newProgMockFollowRepo()
	heatmapService := newProgMockHeatmapSvc()

	service := NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

	// Create a programme first
	_, err := service.CreateCustomProgramme(ctx, "user-1", []string{"streamer-1"})
	if err != nil {
		t.Fatalf("CreateCustomProgramme failed: %v", err)
	}

	// Retrieve it
	programme, err := service.GetCustomProgramme(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetCustomProgramme failed: %v", err)
	}

	if programme.UserID != "user-1" {
		t.Errorf("Expected UserID 'user-1', got '%s'", programme.UserID)
	}
}

func TestProgrammeService_GetCustomProgramme_NotFound(t *testing.T) {
	ctx := context.Background()
	programmeRepo := newProgMockProgrammeRepo()
	streamerRepo := newProgMockStreamerRepo()
	followRepo := newProgMockFollowRepo()
	heatmapService := newProgMockHeatmapSvc()

	service := NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

	_, err := service.GetCustomProgramme(ctx, "nonexistent-user")
	if err == nil {
		t.Error("Expected error for nonexistent programme")
	}
}

func TestProgrammeService_UpdateCustomProgramme(t *testing.T) {
	ctx := context.Background()
	programmeRepo := newProgMockProgrammeRepo()
	streamerRepo := newProgMockStreamerRepo()
	followRepo := newProgMockFollowRepo()
	heatmapService := newProgMockHeatmapSvc()

	service := NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

	// Create a programme first
	_, err := service.CreateCustomProgramme(ctx, "user-1", []string{"streamer-1"})
	if err != nil {
		t.Fatalf("CreateCustomProgramme failed: %v", err)
	}

	// Update it
	err = service.UpdateCustomProgramme(ctx, "user-1", []string{"streamer-2", "streamer-3"})
	if err != nil {
		t.Fatalf("UpdateCustomProgramme failed: %v", err)
	}

	// Verify update
	programme, err := service.GetCustomProgramme(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetCustomProgramme failed: %v", err)
	}

	if len(programme.StreamerIDs) != 2 {
		t.Errorf("Expected 2 streamer IDs, got %d", len(programme.StreamerIDs))
	}
}

func TestProgrammeService_DeleteCustomProgramme(t *testing.T) {
	ctx := context.Background()
	programmeRepo := newProgMockProgrammeRepo()
	streamerRepo := newProgMockStreamerRepo()
	followRepo := newProgMockFollowRepo()
	heatmapService := newProgMockHeatmapSvc()

	service := NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

	// Create a programme first
	_, err := service.CreateCustomProgramme(ctx, "user-1", []string{"streamer-1"})
	if err != nil {
		t.Fatalf("CreateCustomProgramme failed: %v", err)
	}

	// Delete it
	err = service.DeleteCustomProgramme(ctx, "user-1")
	if err != nil {
		t.Fatalf("DeleteCustomProgramme failed: %v", err)
	}

	// Verify deletion
	_, err = service.GetCustomProgramme(ctx, "user-1")
	if err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestProgrammeService_CreateGuestProgramme(t *testing.T) {
	programmeRepo := newProgMockProgrammeRepo()
	streamerRepo := newProgMockStreamerRepo()
	followRepo := newProgMockFollowRepo()
	heatmapService := newProgMockHeatmapSvc()

	service := NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

	streamerIDs := []string{"streamer-1", "streamer-2"}
	programme := service.CreateGuestProgramme(streamerIDs)

	if programme.UserID != "" {
		t.Errorf("Expected empty UserID for guest programme, got '%s'", programme.UserID)
	}

	if len(programme.StreamerIDs) != 2 {
		t.Errorf("Expected 2 streamer IDs, got %d", len(programme.StreamerIDs))
	}
}

func TestProgrammeService_GetProgrammeView_CustomProgramme(t *testing.T) {
	ctx := context.Background()
	programmeRepo := newProgMockProgrammeRepo()
	streamerRepo := newProgMockStreamerRepo()
	followRepo := newProgMockFollowRepo()
	heatmapService := newProgMockHeatmapSvc()

	// Add streamers
	streamer := &domain.Streamer{
		ID:        "streamer-1",
		Name:      "Test Streamer",
		Platforms: []string{"twitch"},
		Handles:   map[string]string{"twitch": "testhandle"},
	}
	streamerRepo.Create(ctx, streamer)

	service := NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

	// Create a custom programme
	_, err := service.CreateCustomProgramme(ctx, "user-1", []string{"streamer-1"})
	if err != nil {
		t.Fatalf("CreateCustomProgramme failed: %v", err)
	}

	// Get programme view
	view, err := service.GetProgrammeView(ctx, "user-1", time.Now())
	if err != nil {
		t.Fatalf("GetProgrammeView failed: %v", err)
	}

	if !view.IsCustom {
		t.Error("Expected IsCustom to be true")
	}
}

func TestProgrammeService_GetProgrammeView_FallbackToGlobal(t *testing.T) {
	ctx := context.Background()
	programmeRepo := newProgMockProgrammeRepo()
	streamerRepo := newProgMockStreamerRepo()
	followRepo := newProgMockFollowRepo()
	heatmapService := newProgMockHeatmapSvc()

	// Add streamers
	streamer := &domain.Streamer{
		ID:        "streamer-1",
		Name:      "Test Streamer",
		Platforms: []string{"twitch"},
		Handles:   map[string]string{"twitch": "testhandle"},
	}
	streamerRepo.Create(ctx, streamer)

	service := NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

	// Get programme view without custom programme
	view, err := service.GetProgrammeView(ctx, "user-1", time.Now())
	if err != nil {
		t.Fatalf("GetProgrammeView failed: %v", err)
	}

	if view.IsCustom {
		t.Error("Expected IsCustom to be false for global programme")
	}
}
