package task

import (
	"context"
	"log"
	"sync"
	"time"

	"who-live-when/internal/domain"
	"who-live-when/internal/repository"

	"github.com/google/uuid"
)

// ActivityTracker handles background activity tracking for streamers
type ActivityTracker struct {
	streamerRepo   repository.StreamerRepository
	activityRepo   repository.ActivityRecordRepository
	liveStatusSvc  domain.LiveStatusService
	checkInterval  time.Duration
	stopCh         chan struct{}
	wg             sync.WaitGroup
	mu             sync.RWMutex
	lastLiveStatus map[string]bool // tracks previous live status per streamer
}

// NewActivityTracker creates a new ActivityTracker instance
func NewActivityTracker(
	streamerRepo repository.StreamerRepository,
	activityRepo repository.ActivityRecordRepository,
	liveStatusSvc domain.LiveStatusService,
	checkInterval time.Duration,
) *ActivityTracker {
	return &ActivityTracker{
		streamerRepo:   streamerRepo,
		activityRepo:   activityRepo,
		liveStatusSvc:  liveStatusSvc,
		checkInterval:  checkInterval,
		stopCh:         make(chan struct{}),
		lastLiveStatus: make(map[string]bool),
	}
}

// Start begins the background activity tracking loop
func (t *ActivityTracker) Start(ctx context.Context) {
	t.wg.Add(1)
	go t.run(ctx)
}

// Stop gracefully stops the activity tracker
func (t *ActivityTracker) Stop() {
	close(t.stopCh)
	t.wg.Wait()
}

// run is the main loop that periodically checks live status
func (t *ActivityTracker) run(ctx context.Context) {
	defer t.wg.Done()

	ticker := time.NewTicker(t.checkInterval)
	defer ticker.Stop()

	// Run immediately on start
	t.checkAndRecordActivity(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopCh:
			return
		case <-ticker.C:
			t.checkAndRecordActivity(ctx)
		}
	}
}

// checkAndRecordActivity checks all streamers and records activity for those going live
func (t *ActivityTracker) checkAndRecordActivity(ctx context.Context) {
	streamers, err := t.streamerRepo.List(ctx, 1000)
	if err != nil {
		log.Printf("activity tracker: failed to list streamers: %v", err)
		return
	}

	for _, streamer := range streamers {
		status, err := t.liveStatusSvc.GetLiveStatus(ctx, streamer.ID)
		if err != nil {
			log.Printf("activity tracker: failed to get live status for %s: %v", streamer.ID, err)
			continue
		}

		t.processStreamerStatus(ctx, streamer.ID, status)
	}
}

// processStreamerStatus handles the live status transition for a single streamer
func (t *ActivityTracker) processStreamerStatus(ctx context.Context, streamerID string, status *domain.LiveStatus) {
	t.mu.Lock()
	wasLive := t.lastLiveStatus[streamerID]
	isLive := status != nil && status.IsLive
	t.lastLiveStatus[streamerID] = isLive
	t.mu.Unlock()

	// Record activity when streamer goes live (transition from offline to live)
	if isLive && !wasLive {
		if err := t.recordActivity(ctx, streamerID, status.Platform); err != nil {
			log.Printf("activity tracker: failed to record activity for %s: %v", streamerID, err)
		}
	}
}

// recordActivity creates an activity record for a streamer going live
func (t *ActivityTracker) recordActivity(ctx context.Context, streamerID, platform string) error {
	now := time.Now()
	record := &domain.ActivityRecord{
		ID:         uuid.New().String(),
		StreamerID: streamerID,
		StartTime:  now,
		EndTime:    now,
		Platform:   platform,
		CreatedAt:  now,
	}

	return t.activityRepo.Create(ctx, record)
}

// GetLastLiveStatus returns the last known live status for a streamer (for testing)
func (t *ActivityTracker) GetLastLiveStatus(streamerID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastLiveStatus[streamerID]
}

// SetLastLiveStatus sets the last known live status for a streamer (for testing)
func (t *ActivityTracker) SetLastLiveStatus(streamerID string, isLive bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastLiveStatus[streamerID] = isLive
}
