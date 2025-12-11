package service

import (
	"context"
	"fmt"
	"time"

	"who-live-when/internal/domain"
	"who-live-when/internal/repository"

	"github.com/google/uuid"
)

// userService implements domain.UserService
type userService struct {
	userRepo      repository.UserRepository
	followRepo    repository.FollowRepository
	activityRepo  repository.ActivityRecordRepository
	streamerRepo  repository.StreamerRepository
	programmeRepo repository.CustomProgrammeRepository
}

// NewUserService creates a new UserService
func NewUserService(
	userRepo repository.UserRepository,
	followRepo repository.FollowRepository,
	activityRepo repository.ActivityRecordRepository,
	streamerRepo repository.StreamerRepository,
	programmeRepo repository.CustomProgrammeRepository,
) domain.UserService {
	return &userService{
		userRepo:      userRepo,
		followRepo:    followRepo,
		activityRepo:  activityRepo,
		streamerRepo:  streamerRepo,
		programmeRepo: programmeRepo,
	}
}

// GetUser retrieves a user by ID
func (s *userService) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// CreateUser creates a new user with Google OAuth data
func (s *userService) CreateUser(ctx context.Context, googleID string, email string) (*domain.User, error) {
	if googleID == "" {
		return nil, fmt.Errorf("google ID cannot be empty")
	}
	if email == "" {
		return nil, fmt.Errorf("email cannot be empty")
	}

	// Check if user already exists
	existingUser, err := s.userRepo.GetByGoogleID(ctx, googleID)
	if err == nil {
		// User already exists, return it
		return existingUser, nil
	}

	// Create new user
	now := time.Now()
	user := &domain.User{
		ID:        uuid.New().String(),
		GoogleID:  googleID,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GetUserFollows retrieves all streamers followed by a user
func (s *userService) GetUserFollows(ctx context.Context, userID string) ([]*domain.Streamer, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	streamers, err := s.followRepo.GetFollowedStreamers(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user follows: %w", err)
	}

	return streamers, nil
}

// FollowStreamer creates a follow relationship between user and streamer
func (s *userService) FollowStreamer(ctx context.Context, userID, streamerID string) error {
	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}
	if streamerID == "" {
		return fmt.Errorf("streamer ID cannot be empty")
	}

	// Check if already following (idempotent operation)
	isFollowing, err := s.followRepo.IsFollowing(ctx, userID, streamerID)
	if err != nil {
		return fmt.Errorf("failed to check follow status: %w", err)
	}

	if isFollowing {
		// Already following, this is idempotent
		return nil
	}

	// Create follow relationship
	if err := s.followRepo.Create(ctx, userID, streamerID); err != nil {
		return fmt.Errorf("failed to follow streamer: %w", err)
	}

	return nil
}

// UnfollowStreamer removes a follow relationship between user and streamer
func (s *userService) UnfollowStreamer(ctx context.Context, userID, streamerID string) error {
	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}
	if streamerID == "" {
		return fmt.Errorf("streamer ID cannot be empty")
	}

	if err := s.followRepo.Delete(ctx, userID, streamerID); err != nil {
		return fmt.Errorf("failed to unfollow streamer: %w", err)
	}

	return nil
}

// GetStreamersByIDs retrieves streamers by their IDs (used for guest follows)
func (s *userService) GetStreamersByIDs(ctx context.Context, streamerIDs []string) ([]*domain.Streamer, error) {
	if len(streamerIDs) == 0 {
		return []*domain.Streamer{}, nil
	}

	streamers, err := s.streamerRepo.GetByIDs(ctx, streamerIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get streamers by IDs: %w", err)
	}

	return streamers, nil
}

// MigrateGuestData migrates guest session data to persistent storage for a registered user
func (s *userService) MigrateGuestData(ctx context.Context, userID string, guestFollows []string, guestProgramme *domain.CustomProgramme) error {
	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}

	// Migrate follows
	for _, streamerID := range guestFollows {
		if err := s.FollowStreamer(ctx, userID, streamerID); err != nil {
			return fmt.Errorf("failed to migrate follow for streamer %s: %w", streamerID, err)
		}
	}

	// Migrate custom programme if exists
	if guestProgramme != nil && len(guestProgramme.StreamerIDs) > 0 {
		now := time.Now()
		programme := &domain.CustomProgramme{
			ID:          uuid.New().String(),
			UserID:      userID,
			StreamerIDs: guestProgramme.StreamerIDs,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := s.programmeRepo.Create(ctx, programme); err != nil {
			return fmt.Errorf("failed to migrate custom programme: %w", err)
		}
	}

	return nil
}
