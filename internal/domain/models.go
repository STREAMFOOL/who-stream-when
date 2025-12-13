package domain

import "time"

// Streamer represents a content creator who broadcasts on streaming platforms
type Streamer struct {
	ID        string            // Unique identifier
	Name      string            // Display name
	Handles   map[string]string // Platform -> handle mapping
	Platforms []string          // List of supported platforms
	CreatedAt time.Time
	UpdatedAt time.Time
}

// LiveStatus represents the current streaming state of a streamer
type LiveStatus struct {
	StreamerID  string
	IsLive      bool
	Platform    string
	StreamURL   string
	Title       string
	Thumbnail   string
	ViewerCount int
	UpdatedAt   time.Time
}

// Heatmap represents activity patterns for a streamer
type Heatmap struct {
	StreamerID  string
	Hours       [24]float64 // Probability 0-1 for each hour
	DaysOfWeek  [7]float64  // Probability 0-1 for each day
	DataPoints  int         // Number of historical records
	GeneratedAt time.Time
}

// User represents a registered user account
type User struct {
	ID        string
	GoogleID  string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ActivityRecord represents a historical streaming session
type ActivityRecord struct {
	ID         string
	StreamerID string
	StartTime  time.Time
	EndTime    time.Time
	Platform   string
	CreatedAt  time.Time
}

// TVProgramme represents a weekly schedule of predicted live times
type TVProgramme struct {
	UserID      string
	Week        time.Time
	Entries     []ProgrammeEntry
	GeneratedAt time.Time
}

// ProgrammeEntry represents a single predicted streaming slot
type ProgrammeEntry struct {
	StreamerID  string
	DayOfWeek   int
	Hour        int
	Probability float64
}

// WeekView represents the default week view for the home page
type WeekView struct {
	Week      time.Time
	Streamers []*Streamer
	Entries   []ProgrammeEntry
	ViewCount map[string]int // Streamer ID -> view count
}

// ActivityStats represents statistical data about streamer activity
type ActivityStats struct {
	StreamerID             string
	TotalSessions          int
	AverageSessionDuration time.Duration
	LastActive             time.Time
	MostActiveHour         int
	MostActiveDay          int
}

// PlatformLiveStatus represents live status from a specific platform
type PlatformLiveStatus struct {
	IsLive      bool
	StreamURL   string
	Title       string
	Thumbnail   string
	ViewerCount int
}

// PlatformStreamer represents a streamer found via platform search
type PlatformStreamer struct {
	Handle    string
	Name      string
	Platform  string
	Thumbnail string
}

// PlatformChannelInfo represents detailed channel information from a platform
type PlatformChannelInfo struct {
	Handle      string
	Name        string
	Description string
	Thumbnail   string
	Platform    string
}

// PredictedTime represents a predicted streaming time slot
type PredictedTime struct {
	DayOfWeek   int
	Hour        int
	Probability float64
}

// CustomProgramme represents a user's personalized weekly schedule.
// Registered users have custom programmes persisted in the database.
// Guest users have custom programmes stored in session cookies.
// When a guest registers, their custom programme is migrated to the database.
type CustomProgramme struct {
	ID          string    // Unique identifier
	UserID      string    // User ID (empty for guest programmes)
	StreamerIDs []string  // List of streamer IDs in the programme
	CreatedAt   time.Time // Creation timestamp
	UpdatedAt   time.Time // Last update timestamp
}
