package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"who-live-when/internal/domain"
)

func TestCustomProgrammeRepository_Create(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewCustomProgrammeRepository(db)
	ctx := context.Background()

	// Create test user
	userID := uuid.New().String()
	user := &domain.User{
		ID:        userID,
		GoogleID:  "test-google-id",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	userRepo := NewUserRepository(db)
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create test streamers
	streamerIDs := []string{uuid.New().String(), uuid.New().String(), uuid.New().String()}
	streamerRepo := NewStreamerRepository(db)
	for _, streamerID := range streamerIDs {
		streamer := &domain.Streamer{
			ID:   streamerID,
			Name: "Test Streamer " + streamerID,
			Handles: map[string]string{
				"youtube": "handle-" + streamerID,
			},
			Platforms: []string{"youtube"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := streamerRepo.Create(ctx, streamer); err != nil {
			t.Fatalf("failed to create test streamer: %v", err)
		}
	}

	// Create custom programme
	programme := &domain.CustomProgramme{
		ID:          uuid.New().String(),
		UserID:      userID,
		StreamerIDs: streamerIDs,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := repo.Create(ctx, programme); err != nil {
		t.Fatalf("failed to create custom programme: %v", err)
	}

	// Retrieve and verify
	retrieved, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("failed to retrieve custom programme: %v", err)
	}

	if retrieved.UserID != userID {
		t.Errorf("expected user ID %s, got %s", userID, retrieved.UserID)
	}

	if len(retrieved.StreamerIDs) != len(streamerIDs) {
		t.Errorf("expected %d streamer IDs, got %d", len(streamerIDs), len(retrieved.StreamerIDs))
	}

	for i, id := range streamerIDs {
		if retrieved.StreamerIDs[i] != id {
			t.Errorf("expected streamer ID %s at position %d, got %s", id, i, retrieved.StreamerIDs[i])
		}
	}
}

func TestCustomProgrammeRepository_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewCustomProgrammeRepository(db)
	ctx := context.Background()

	// Create test user
	userID := uuid.New().String()
	user := &domain.User{
		ID:        userID,
		GoogleID:  "test-google-id",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	userRepo := NewUserRepository(db)
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create test streamers
	streamerIDs := []string{uuid.New().String(), uuid.New().String(), uuid.New().String()}
	streamerRepo := NewStreamerRepository(db)
	for _, streamerID := range streamerIDs {
		streamer := &domain.Streamer{
			ID:   streamerID,
			Name: "Test Streamer " + streamerID,
			Handles: map[string]string{
				"youtube": "handle-" + streamerID,
			},
			Platforms: []string{"youtube"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := streamerRepo.Create(ctx, streamer); err != nil {
			t.Fatalf("failed to create test streamer: %v", err)
		}
	}

	// Create initial programme
	programmeID := uuid.New().String()
	programme := &domain.CustomProgramme{
		ID:          programmeID,
		UserID:      userID,
		StreamerIDs: streamerIDs[:2],
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := repo.Create(ctx, programme); err != nil {
		t.Fatalf("failed to create custom programme: %v", err)
	}

	// Update programme with all streamers
	programme.StreamerIDs = streamerIDs
	programme.UpdatedAt = time.Now()

	if err := repo.Update(ctx, programme); err != nil {
		t.Fatalf("failed to update custom programme: %v", err)
	}

	// Retrieve and verify
	retrieved, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("failed to retrieve custom programme: %v", err)
	}

	if len(retrieved.StreamerIDs) != len(streamerIDs) {
		t.Errorf("expected %d streamer IDs, got %d", len(streamerIDs), len(retrieved.StreamerIDs))
	}

	for i, id := range streamerIDs {
		if retrieved.StreamerIDs[i] != id {
			t.Errorf("expected streamer ID %s at position %d, got %s", id, i, retrieved.StreamerIDs[i])
		}
	}
}

func TestCustomProgrammeRepository_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewCustomProgrammeRepository(db)
	ctx := context.Background()

	// Create test user
	userID := uuid.New().String()
	user := &domain.User{
		ID:        userID,
		GoogleID:  "test-google-id",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	userRepo := NewUserRepository(db)
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create test streamer
	streamerID := uuid.New().String()
	streamer := &domain.Streamer{
		ID:   streamerID,
		Name: "Test Streamer",
		Handles: map[string]string{
			"youtube": "handle",
		},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	streamerRepo := NewStreamerRepository(db)
	if err := streamerRepo.Create(ctx, streamer); err != nil {
		t.Fatalf("failed to create test streamer: %v", err)
	}

	// Create custom programme
	programme := &domain.CustomProgramme{
		ID:          uuid.New().String(),
		UserID:      userID,
		StreamerIDs: []string{streamerID},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := repo.Create(ctx, programme); err != nil {
		t.Fatalf("failed to create custom programme: %v", err)
	}

	// Delete programme
	if err := repo.Delete(ctx, userID); err != nil {
		t.Fatalf("failed to delete custom programme: %v", err)
	}

	// Verify deletion
	_, err := repo.GetByUserID(ctx, userID)
	if err == nil {
		t.Error("expected error when retrieving deleted programme, got nil")
	}
}

func TestCustomProgrammeRepository_UserIsolation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewCustomProgrammeRepository(db)
	ctx := context.Background()

	// Create two test users
	userID1 := uuid.New().String()
	userID2 := uuid.New().String()

	userRepo := NewUserRepository(db)
	for i, userID := range []string{userID1, userID2} {
		user := &domain.User{
			ID:        userID,
			GoogleID:  "test-google-id-" + string(rune(i)),
			Email:     "test" + string(rune(i)) + "@example.com",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := userRepo.Create(ctx, user); err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}
	}

	// Create test streamers
	streamerIDs1 := []string{uuid.New().String(), uuid.New().String()}
	streamerIDs2 := []string{uuid.New().String(), uuid.New().String()}

	streamerRepo := NewStreamerRepository(db)
	for _, streamerID := range append(streamerIDs1, streamerIDs2...) {
		streamer := &domain.Streamer{
			ID:   streamerID,
			Name: "Test Streamer " + streamerID,
			Handles: map[string]string{
				"youtube": "handle-" + streamerID,
			},
			Platforms: []string{"youtube"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := streamerRepo.Create(ctx, streamer); err != nil {
			t.Fatalf("failed to create test streamer: %v", err)
		}
	}

	// Create programmes for both users
	programme1 := &domain.CustomProgramme{
		ID:          uuid.New().String(),
		UserID:      userID1,
		StreamerIDs: streamerIDs1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	programme2 := &domain.CustomProgramme{
		ID:          uuid.New().String(),
		UserID:      userID2,
		StreamerIDs: streamerIDs2,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := repo.Create(ctx, programme1); err != nil {
		t.Fatalf("failed to create programme 1: %v", err)
	}

	if err := repo.Create(ctx, programme2); err != nil {
		t.Fatalf("failed to create programme 2: %v", err)
	}

	// Verify user 1 can only access their programme
	retrieved1, err := repo.GetByUserID(ctx, userID1)
	if err != nil {
		t.Fatalf("failed to retrieve programme 1: %v", err)
	}

	if len(retrieved1.StreamerIDs) != len(streamerIDs1) {
		t.Errorf("user 1: expected %d streamer IDs, got %d", len(streamerIDs1), len(retrieved1.StreamerIDs))
	}

	for i, id := range streamerIDs1 {
		if retrieved1.StreamerIDs[i] != id {
			t.Errorf("user 1: expected streamer ID %s at position %d, got %s", id, i, retrieved1.StreamerIDs[i])
		}
	}

	// Verify user 2 can only access their programme
	retrieved2, err := repo.GetByUserID(ctx, userID2)
	if err != nil {
		t.Fatalf("failed to retrieve programme 2: %v", err)
	}

	if len(retrieved2.StreamerIDs) != len(streamerIDs2) {
		t.Errorf("user 2: expected %d streamer IDs, got %d", len(streamerIDs2), len(retrieved2.StreamerIDs))
	}

	for i, id := range streamerIDs2 {
		if retrieved2.StreamerIDs[i] != id {
			t.Errorf("user 2: expected streamer ID %s at position %d, got %s", id, i, retrieved2.StreamerIDs[i])
		}
	}
}

func TestCustomProgrammeRepository_ConcurrentUpdates(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewCustomProgrammeRepository(db)
	ctx := context.Background()

	// Create test user
	userID := uuid.New().String()
	user := &domain.User{
		ID:        userID,
		GoogleID:  "test-google-id",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	userRepo := NewUserRepository(db)
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create test streamers
	streamerIDs := make([]string, 10)
	streamerRepo := NewStreamerRepository(db)
	for i := range streamerIDs {
		streamerIDs[i] = uuid.New().String()
		streamer := &domain.Streamer{
			ID:   streamerIDs[i],
			Name: "Test Streamer " + streamerIDs[i],
			Handles: map[string]string{
				"youtube": "handle-" + streamerIDs[i],
			},
			Platforms: []string{"youtube"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := streamerRepo.Create(ctx, streamer); err != nil {
			t.Fatalf("failed to create test streamer: %v", err)
		}
	}

	// Create initial programme
	programmeID := uuid.New().String()
	programme := &domain.CustomProgramme{
		ID:          programmeID,
		UserID:      userID,
		StreamerIDs: streamerIDs[:5],
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := repo.Create(ctx, programme); err != nil {
		t.Fatalf("failed to create custom programme: %v", err)
	}

	// Perform concurrent updates
	done := make(chan bool, 2)

	go func() {
		programme.StreamerIDs = streamerIDs[:7]
		programme.UpdatedAt = time.Now()
		if err := repo.Update(ctx, programme); err != nil {
			t.Logf("concurrent update 1 failed: %v", err)
		}
		done <- true
	}()

	go func() {
		programme.StreamerIDs = streamerIDs[3:]
		programme.UpdatedAt = time.Now()
		if err := repo.Update(ctx, programme); err != nil {
			t.Logf("concurrent update 2 failed: %v", err)
		}
		done <- true
	}()

	<-done
	<-done

	// Verify programme still exists and is valid
	retrieved, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("failed to retrieve programme after concurrent updates: %v", err)
	}

	if retrieved.UserID != userID {
		t.Errorf("expected user ID %s, got %s", userID, retrieved.UserID)
	}

	// Verify all streamer IDs are valid
	for _, id := range retrieved.StreamerIDs {
		found := false
		for _, validID := range streamerIDs {
			if id == validID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("invalid streamer ID in programme: %s", id)
		}
	}
}
