package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"who-live-when/internal/domain"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// progMockProgrammeRepo is a mock implementation for programme property tests
type progMockProgrammeRepo struct {
	programmes map[string]*domain.CustomProgramme
}

func newProgMockProgrammeRepo() *progMockProgrammeRepo {
	return &progMockProgrammeRepo{
		programmes: make(map[string]*domain.CustomProgramme),
	}
}

func (m *progMockProgrammeRepo) Create(ctx context.Context, programme *domain.CustomProgramme) error {
	m.programmes[programme.UserID] = programme
	return nil
}

func (m *progMockProgrammeRepo) GetByUserID(ctx context.Context, userID string) (*domain.CustomProgramme, error) {
	if p, ok := m.programmes[userID]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *progMockProgrammeRepo) Update(ctx context.Context, programme *domain.CustomProgramme) error {
	m.programmes[programme.UserID] = programme
	return nil
}

func (m *progMockProgrammeRepo) Delete(ctx context.Context, userID string) error {
	delete(m.programmes, userID)
	return nil
}

// progMockStreamerRepo is a mock implementation for programme property tests
type progMockStreamerRepo struct {
	streamers map[string]*domain.Streamer
}

func newProgMockStreamerRepo() *progMockStreamerRepo {
	return &progMockStreamerRepo{
		streamers: make(map[string]*domain.Streamer),
	}
}

func (m *progMockStreamerRepo) Create(ctx context.Context, streamer *domain.Streamer) error {
	m.streamers[streamer.ID] = streamer
	return nil
}

func (m *progMockStreamerRepo) GetByID(ctx context.Context, id string) (*domain.Streamer, error) {
	if s, ok := m.streamers[id]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *progMockStreamerRepo) List(ctx context.Context, limit int) ([]*domain.Streamer, error) {
	var result []*domain.Streamer
	for _, s := range m.streamers {
		result = append(result, s)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (m *progMockStreamerRepo) Update(ctx context.Context, streamer *domain.Streamer) error {
	m.streamers[streamer.ID] = streamer
	return nil
}

func (m *progMockStreamerRepo) Delete(ctx context.Context, id string) error {
	delete(m.streamers, id)
	return nil
}

func (m *progMockStreamerRepo) GetByPlatform(ctx context.Context, platform string) ([]*domain.Streamer, error) {
	var result []*domain.Streamer
	for _, s := range m.streamers {
		for _, p := range s.Platforms {
			if p == platform {
				result = append(result, s)
				break
			}
		}
	}
	return result, nil
}

func (m *progMockStreamerRepo) GetByIDs(ctx context.Context, ids []string) ([]*domain.Streamer, error) {
	result := make([]*domain.Streamer, 0, len(ids))
	for _, id := range ids {
		if s, ok := m.streamers[id]; ok {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *progMockStreamerRepo) GetByPlatformHandle(ctx context.Context, platform, handle string) (*domain.Streamer, error) {
	for _, s := range m.streamers {
		if h, ok := s.Handles[platform]; ok && h == handle {
			return s, nil
		}
	}
	return nil, nil
}

// progMockFollowRepo is a mock implementation for programme property tests
type progMockFollowRepo struct {
	follows map[string]map[string]bool
}

func newProgMockFollowRepo() *progMockFollowRepo {
	return &progMockFollowRepo{
		follows: make(map[string]map[string]bool),
	}
}

func (m *progMockFollowRepo) Create(ctx context.Context, userID, streamerID string) error {
	if m.follows[userID] == nil {
		m.follows[userID] = make(map[string]bool)
	}
	m.follows[userID][streamerID] = true
	return nil
}

func (m *progMockFollowRepo) Delete(ctx context.Context, userID, streamerID string) error {
	if m.follows[userID] != nil {
		delete(m.follows[userID], streamerID)
	}
	return nil
}

func (m *progMockFollowRepo) GetFollowedStreamers(ctx context.Context, userID string) ([]*domain.Streamer, error) {
	return nil, nil
}

func (m *progMockFollowRepo) IsFollowing(ctx context.Context, userID, streamerID string) (bool, error) {
	if m.follows[userID] != nil {
		return m.follows[userID][streamerID], nil
	}
	return false, nil
}

func (m *progMockFollowRepo) GetFollowerCount(ctx context.Context, streamerID string) (int, error) {
	count := 0
	for _, follows := range m.follows {
		if follows[streamerID] {
			count++
		}
	}
	return count, nil
}

// progMockHeatmapSvc is a mock implementation for programme property tests
type progMockHeatmapSvc struct {
	heatmaps map[string]*domain.Heatmap
}

func newProgMockHeatmapSvc() *progMockHeatmapSvc {
	return &progMockHeatmapSvc{
		heatmaps: make(map[string]*domain.Heatmap),
	}
}

func (m *progMockHeatmapSvc) GenerateHeatmap(ctx context.Context, streamerID string) (*domain.Heatmap, error) {
	if h, ok := m.heatmaps[streamerID]; ok {
		return h, nil
	}
	heatmap := &domain.Heatmap{StreamerID: streamerID, DataPoints: 10}
	for i := 0; i < 7; i++ {
		heatmap.DaysOfWeek[i] = 0.3
	}
	for i := 0; i < 24; i++ {
		heatmap.Hours[i] = 0.2
	}
	return heatmap, nil
}

func (m *progMockHeatmapSvc) RecordActivity(ctx context.Context, streamerID string, timestamp time.Time) error {
	return nil
}

func (m *progMockHeatmapSvc) GetActivityStats(ctx context.Context, streamerID string) (*domain.ActivityStats, error) {
	return nil, nil
}

// **Feature: user-experience-enhancements, Property 8: Custom Programme Calendar Filtering**
// **Validates: Requirements 3.3, 9.2**
func TestProperty_CustomProgrammeCalendarFiltering(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("calendar only contains entries for streamers in the programme", prop.ForAll(
		func(numStreamers int, numInProgramme int, weekTimestamp int64) bool {
			ctx := context.Background()
			week := time.Unix(weekTimestamp, 0)

			if numInProgramme > numStreamers {
				numInProgramme = numStreamers
			}

			streamerRepo := newProgMockStreamerRepo()
			followRepo := newProgMockFollowRepo()
			programmeRepo := newProgMockProgrammeRepo()
			heatmapService := newProgMockHeatmapSvc()

			var allStreamerIDs []string
			for i := 0; i < numStreamers; i++ {
				streamerID := fmt.Sprintf("streamer-%d", i)
				allStreamerIDs = append(allStreamerIDs, streamerID)
				streamer := &domain.Streamer{
					ID:        streamerID,
					Name:      fmt.Sprintf("Streamer %d", i),
					Platforms: []string{"twitch"},
					Handles:   map[string]string{"twitch": fmt.Sprintf("handle%d", i)},
				}
				streamerRepo.Create(ctx, streamer)

				heatmap := &domain.Heatmap{StreamerID: streamerID, DataPoints: 10}
				for d := 0; d < 7; d++ {
					heatmap.DaysOfWeek[d] = 0.3
				}
				for h := 0; h < 24; h++ {
					heatmap.Hours[h] = 0.3
				}
				heatmapService.heatmaps[streamerID] = heatmap
			}

			programmeStreamerIDs := allStreamerIDs[:numInProgramme]
			programme := &domain.CustomProgramme{
				ID:          "prog-1",
				UserID:      "user-1",
				StreamerIDs: programmeStreamerIDs,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			service := NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)
			calendarView, err := service.GenerateCalendarFromProgramme(ctx, programme, week)
			if err != nil {
				return false
			}

			programmeSet := make(map[string]bool)
			for _, id := range programmeStreamerIDs {
				programmeSet[id] = true
			}

			for _, entry := range calendarView.Entries {
				if !programmeSet[entry.StreamerID] {
					return false
				}
			}

			for _, streamer := range calendarView.Streamers {
				if !programmeSet[streamer.ID] {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 20),
		gen.IntRange(0, 10),
		gen.Int64Range(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC).Unix()),
	))

	properties.TestingRun(t)
}

// **Feature: user-experience-enhancements, Property 9: Streamer Removal from Programme**
// **Validates: Requirements 3.4, 9.3**
func TestProperty_StreamerRemovalFromProgramme(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("removing a streamer excludes them from the calendar", prop.ForAll(
		func(numStreamers int, removeIndex int, weekTimestamp int64) bool {
			ctx := context.Background()
			week := time.Unix(weekTimestamp, 0)

			if numStreamers < 2 {
				numStreamers = 2
			}
			removeIndex = removeIndex % numStreamers

			streamerRepo := newProgMockStreamerRepo()
			followRepo := newProgMockFollowRepo()
			programmeRepo := newProgMockProgrammeRepo()
			heatmapService := newProgMockHeatmapSvc()

			var allStreamerIDs []string
			for i := 0; i < numStreamers; i++ {
				streamerID := fmt.Sprintf("streamer-%d", i)
				allStreamerIDs = append(allStreamerIDs, streamerID)
				streamer := &domain.Streamer{
					ID:        streamerID,
					Name:      fmt.Sprintf("Streamer %d", i),
					Platforms: []string{"twitch"},
					Handles:   map[string]string{"twitch": fmt.Sprintf("handle%d", i)},
				}
				streamerRepo.Create(ctx, streamer)

				heatmap := &domain.Heatmap{StreamerID: streamerID, DataPoints: 10}
				for d := 0; d < 7; d++ {
					heatmap.DaysOfWeek[d] = 0.3
				}
				for h := 0; h < 24; h++ {
					heatmap.Hours[h] = 0.3
				}
				heatmapService.heatmaps[streamerID] = heatmap
			}

			userID := "user-1"
			programme := &domain.CustomProgramme{
				ID:          "prog-1",
				UserID:      userID,
				StreamerIDs: allStreamerIDs,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			programmeRepo.Create(ctx, programme)

			service := NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)
			streamerToRemove := allStreamerIDs[removeIndex]

			err := service.RemoveStreamerFromProgramme(ctx, userID, streamerToRemove)
			if err != nil {
				return false
			}

			updatedProgramme, err := service.GetCustomProgramme(ctx, userID)
			if err != nil {
				return false
			}

			calendarView, err := service.GenerateCalendarFromProgramme(ctx, updatedProgramme, week)
			if err != nil {
				return false
			}

			for _, entry := range calendarView.Entries {
				if entry.StreamerID == streamerToRemove {
					return false
				}
			}

			for _, streamer := range calendarView.Streamers {
				if streamer.ID == streamerToRemove {
					return false
				}
			}

			for _, id := range updatedProgramme.StreamerIDs {
				if id == streamerToRemove {
					return false
				}
			}

			return true
		},
		gen.IntRange(2, 15),
		gen.IntRange(0, 100),
		gen.Int64Range(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC).Unix()),
	))

	properties.TestingRun(t)
}

// **Feature: user-experience-enhancements, Property 10: Global Programme Ranking**
// **Validates: Requirements 4.2**
func TestProperty_GlobalProgrammeRanking(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("global programme orders streamers by follower count descending", prop.ForAll(
		func(numStreamers int, followerCounts []int, weekTimestamp int64) bool {
			ctx := context.Background()
			week := time.Unix(weekTimestamp, 0)

			if len(followerCounts) < numStreamers {
				for len(followerCounts) < numStreamers {
					followerCounts = append(followerCounts, 0)
				}
			}

			streamerRepo := newProgMockStreamerRepo()
			followRepo := newProgMockFollowRepo()
			programmeRepo := newProgMockProgrammeRepo()
			heatmapService := newProgMockHeatmapSvc()

			for i := 0; i < numStreamers; i++ {
				streamerID := fmt.Sprintf("streamer-%d", i)
				streamer := &domain.Streamer{
					ID:        streamerID,
					Name:      fmt.Sprintf("Streamer %d", i),
					Platforms: []string{"twitch"},
					Handles:   map[string]string{"twitch": fmt.Sprintf("handle%d", i)},
				}
				streamerRepo.Create(ctx, streamer)

				followerCount := followerCounts[i]
				for j := 0; j < followerCount; j++ {
					userID := fmt.Sprintf("user-%d-%d", i, j)
					followRepo.Create(ctx, userID, streamerID)
				}

				heatmap := &domain.Heatmap{StreamerID: streamerID, DataPoints: 10}
				for d := 0; d < 7; d++ {
					heatmap.DaysOfWeek[d] = 0.3
				}
				for h := 0; h < 24; h++ {
					heatmap.Hours[h] = 0.3
				}
				heatmapService.heatmaps[streamerID] = heatmap
			}

			service := NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

			rankedStreamers, err := service.GetStreamersRankedByFollowers(ctx, numStreamers)
			if err != nil {
				return false
			}

			for i := 1; i < len(rankedStreamers); i++ {
				if rankedStreamers[i].FollowerCount > rankedStreamers[i-1].FollowerCount {
					return false
				}
			}

			globalView, err := service.GenerateGlobalProgramme(ctx, week, numStreamers)
			if err != nil {
				return false
			}

			if len(globalView.Streamers) > 0 && len(rankedStreamers) > 0 {
				topRankedCount := rankedStreamers[0].FollowerCount
				firstGlobalID := globalView.Streamers[0].ID
				firstGlobalCount, _ := followRepo.GetFollowerCount(ctx, firstGlobalID)

				if firstGlobalCount < topRankedCount {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 10),
		gen.SliceOfN(10, gen.IntRange(0, 50)),
		gen.Int64Range(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC).Unix()),
	))

	properties.TestingRun(t)
}
