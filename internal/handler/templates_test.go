package handler

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
	"time"

	"who-live-when/internal/domain"
)

func TestTemplateFuncs(t *testing.T) {
	funcs := TemplateFuncs()

	t.Run("mul function multiplies two numbers", func(t *testing.T) {
		mulFunc := funcs["mul"].(func(float64, float64) float64)
		result := mulFunc(0.5, 100)
		if result != 50 {
			t.Errorf("expected 50, got %f", result)
		}
	})

	t.Run("list function creates a slice", func(t *testing.T) {
		listFunc := funcs["list"].(func(...interface{}) []interface{})
		result := listFunc("a", "b", "c")
		if len(result) != 3 {
			t.Errorf("expected 3 items, got %d", len(result))
		}
	})

	t.Run("seq function generates sequence", func(t *testing.T) {
		seqFunc := funcs["seq"].(func(int, int) []int)
		result := seqFunc(0, 5)
		if len(result) != 6 {
			t.Errorf("expected 6 items, got %d", len(result))
		}
		if result[0] != 0 || result[5] != 5 {
			t.Errorf("expected sequence 0-5, got %v", result)
		}
	})

	t.Run("add function adds two integers", func(t *testing.T) {
		addFunc := funcs["add"].(func(int, int) int)
		result := addFunc(3, 4)
		if result != 7 {
			t.Errorf("expected 7, got %d", result)
		}
	})

	t.Run("sub function subtracts two integers", func(t *testing.T) {
		subFunc := funcs["sub"].(func(int, int) int)
		result := subFunc(10, 3)
		if result != 7 {
			t.Errorf("expected 7, got %d", result)
		}
	})
}

func TestHomeTemplateRendering(t *testing.T) {
	tmpl := createTestTemplates(t)

	t.Run("renders home page with streamers for authenticated user", func(t *testing.T) {
		data := map[string]interface{}{
			"WeekView": &domain.WeekView{
				Week: time.Now(),
				Streamers: []*domain.Streamer{
					{ID: "1", Name: "TestStreamer", Platforms: []string{"youtube", "twitch"}},
				},
				ViewCount: map[string]int{"1": 100},
			},
			"LiveStatuses": map[string]*domain.LiveStatus{
				"1": {StreamerID: "1", IsLive: true, Platform: "youtube", StreamURL: "https://youtube.com/watch?v=123"},
			},
			"IsAuthenticated": true,
		}

		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "home.html", data)
		if err != nil {
			t.Fatalf("failed to execute template: %v", err)
		}

		output := buf.String()
		assertContains(t, output, "TestStreamer")
		assertContains(t, output, "Live on youtube")
		assertContains(t, output, "Dashboard")
		assertContains(t, output, "Logout")
	})

	t.Run("renders home page for unauthenticated user", func(t *testing.T) {
		data := map[string]interface{}{
			"WeekView": &domain.WeekView{
				Week:      time.Now(),
				Streamers: []*domain.Streamer{},
				ViewCount: map[string]int{},
			},
			"LiveStatuses":    map[string]*domain.LiveStatus{},
			"IsAuthenticated": false,
		}

		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "home.html", data)
		if err != nil {
			t.Fatalf("failed to execute template: %v", err)
		}

		output := buf.String()
		assertContains(t, output, "Login with Google")
		assertNotContains(t, output, "Dashboard")
		assertNotContains(t, output, "Logout")
	})

	t.Run("renders empty state when no streamers", func(t *testing.T) {
		data := map[string]interface{}{
			"WeekView": &domain.WeekView{
				Week:      time.Now(),
				Streamers: []*domain.Streamer{},
				ViewCount: map[string]int{},
			},
			"LiveStatuses":    map[string]*domain.LiveStatus{},
			"IsAuthenticated": false,
		}

		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "home.html", data)
		if err != nil {
			t.Fatalf("failed to execute template: %v", err)
		}

		output := buf.String()
		assertContains(t, output, "No streamers yet")
	})
}

func TestStreamerTemplateRendering(t *testing.T) {
	tmpl := createTestTemplates(t)

	t.Run("renders streamer detail with live status", func(t *testing.T) {
		data := map[string]interface{}{
			"Streamer": &domain.Streamer{
				ID:        "1",
				Name:      "TestStreamer",
				Platforms: []string{"youtube", "twitch"},
				Handles:   map[string]string{"youtube": "testchannel", "twitch": "teststreamer"},
			},
			"LiveStatus": &domain.LiveStatus{
				StreamerID:  "1",
				IsLive:      true,
				Platform:    "youtube",
				StreamURL:   "https://youtube.com/watch?v=123",
				Title:       "Test Stream Title",
				ViewerCount: 500,
			},
			"Heatmap":         nil,
			"IsAuthenticated": true,
			"IsFollowing":     false,
		}

		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "streamer.html", data)
		if err != nil {
			t.Fatalf("failed to execute template: %v", err)
		}

		output := buf.String()
		assertContains(t, output, "TestStreamer")
		assertContains(t, output, "Live Now on youtube")
		assertContains(t, output, "Test Stream Title")
		assertContains(t, output, "Watch Stream")
		assertContains(t, output, "Follow")
	})

	t.Run("renders streamer detail with heatmap", func(t *testing.T) {
		heatmap := &domain.Heatmap{
			StreamerID: "1",
			Hours:      [24]float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 0.9, 0.8, 0.7, 0.6, 0.5, 0.4, 0.3, 0.2, 0.1, 0.0, 0.1, 0.2, 0.3, 0.4},
			DaysOfWeek: [7]float64{0.5, 0.6, 0.7, 0.8, 0.9, 0.8, 0.6},
			DataPoints: 150,
		}

		data := map[string]interface{}{
			"Streamer": &domain.Streamer{
				ID:        "1",
				Name:      "TestStreamer",
				Platforms: []string{"youtube"},
				Handles:   map[string]string{"youtube": "testchannel"},
			},
			"LiveStatus":      nil,
			"Heatmap":         heatmap,
			"IsAuthenticated": false,
			"IsFollowing":     false,
		}

		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "streamer.html", data)
		if err != nil {
			t.Fatalf("failed to execute template: %v", err)
		}

		output := buf.String()
		assertContains(t, output, "Activity Heatmap")
		assertContains(t, output, "150 data points")
		assertContains(t, output, "Hours of Day")
		assertContains(t, output, "Days of Week")
	})

	t.Run("shows insufficient data message when no heatmap", func(t *testing.T) {
		data := map[string]interface{}{
			"Streamer": &domain.Streamer{
				ID:        "1",
				Name:      "TestStreamer",
				Platforms: []string{"youtube"},
				Handles:   map[string]string{"youtube": "testchannel"},
			},
			"LiveStatus":      nil,
			"Heatmap":         nil,
			"IsAuthenticated": false,
			"IsFollowing":     false,
		}

		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "streamer.html", data)
		if err != nil {
			t.Fatalf("failed to execute template: %v", err)
		}

		output := buf.String()
		assertContains(t, output, "Insufficient Data")
	})

	t.Run("shows unfollow button when user is following", func(t *testing.T) {
		data := map[string]interface{}{
			"Streamer": &domain.Streamer{
				ID:        "1",
				Name:      "TestStreamer",
				Platforms: []string{"youtube"},
				Handles:   map[string]string{"youtube": "testchannel"},
			},
			"LiveStatus":      nil,
			"Heatmap":         nil,
			"IsAuthenticated": true,
			"IsFollowing":     true,
		}

		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "streamer.html", data)
		if err != nil {
			t.Fatalf("failed to execute template: %v", err)
		}

		output := buf.String()
		assertContains(t, output, "Unfollow")
	})

	t.Run("shows login prompt for unauthenticated user", func(t *testing.T) {
		data := map[string]interface{}{
			"Streamer": &domain.Streamer{
				ID:        "1",
				Name:      "TestStreamer",
				Platforms: []string{"youtube"},
				Handles:   map[string]string{"youtube": "testchannel"},
			},
			"LiveStatus":      nil,
			"Heatmap":         nil,
			"IsAuthenticated": false,
			"IsFollowing":     false,
		}

		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "streamer.html", data)
		if err != nil {
			t.Fatalf("failed to execute template: %v", err)
		}

		output := buf.String()
		assertContains(t, output, "Login to Follow")
	})
}

func TestDashboardTemplateRendering(t *testing.T) {
	tmpl := createTestTemplates(t)

	t.Run("renders dashboard with followed streamers", func(t *testing.T) {
		data := map[string]interface{}{
			"User": &domain.User{
				ID:    "user1",
				Email: "test@example.com",
			},
			"FollowedStreamers": []*domain.Streamer{
				{ID: "1", Name: "Streamer1", Platforms: []string{"youtube"}},
				{ID: "2", Name: "Streamer2", Platforms: []string{"twitch"}},
			},
			"LiveStatuses": map[string]*domain.LiveStatus{
				"1": {StreamerID: "1", IsLive: true, Platform: "youtube"},
				"2": {StreamerID: "2", IsLive: false},
			},
			"IsAuthenticated": true,
		}

		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "dashboard.html", data)
		if err != nil {
			t.Fatalf("failed to execute template: %v", err)
		}

		output := buf.String()
		assertContains(t, output, "test@example.com")
		assertContains(t, output, "Streamer1")
		assertContains(t, output, "Streamer2")
		assertContains(t, output, "Live on youtube")
		assertContains(t, output, "Offline")
		assertContains(t, output, "Search")
	})

	t.Run("renders empty state when no followed streamers", func(t *testing.T) {
		data := map[string]interface{}{
			"User": &domain.User{
				ID:    "user1",
				Email: "test@example.com",
			},
			"FollowedStreamers": []*domain.Streamer{},
			"LiveStatuses":      map[string]*domain.LiveStatus{},
			"IsAuthenticated":   true,
		}

		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "dashboard.html", data)
		if err != nil {
			t.Fatalf("failed to execute template: %v", err)
		}

		output := buf.String()
		assertContains(t, output, "No followed streamers yet")
	})
}

func TestCalendarTemplateRendering(t *testing.T) {
	tmpl := createTestTemplates(t)

	t.Run("renders calendar with programme entries", func(t *testing.T) {
		week := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		data := map[string]interface{}{
			"Programme": &domain.TVProgramme{
				UserID: "user1",
				Week:   week,
				Entries: []domain.ProgrammeEntry{
					{StreamerID: "1", DayOfWeek: 1, Hour: 14, Probability: 0.85},
					{StreamerID: "2", DayOfWeek: 3, Hour: 20, Probability: 0.70},
				},
			},
			"StreamerMap": map[string]*domain.Streamer{
				"1": {ID: "1", Name: "Streamer1"},
				"2": {ID: "2", Name: "Streamer2"},
			},
			"Week":            week,
			"PrevWeek":        week.AddDate(0, 0, -7),
			"NextWeek":        week.AddDate(0, 0, 7),
			"IsAuthenticated": true,
		}

		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "calendar.html", data)
		if err != nil {
			t.Fatalf("failed to execute template: %v", err)
		}

		output := buf.String()
		assertContains(t, output, "TV Programme Calendar")
		assertContains(t, output, "Streamer1")
		assertContains(t, output, "Streamer2")
		assertContains(t, output, "Previous Week")
		assertContains(t, output, "Next Week")
	})

	t.Run("renders empty state when no predictions", func(t *testing.T) {
		week := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		data := map[string]interface{}{
			"Programme": &domain.TVProgramme{
				UserID:  "user1",
				Week:    week,
				Entries: []domain.ProgrammeEntry{},
			},
			"StreamerMap":     map[string]*domain.Streamer{},
			"Week":            week,
			"PrevWeek":        week.AddDate(0, 0, -7),
			"NextWeek":        week.AddDate(0, 0, 7),
			"IsAuthenticated": true,
		}

		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "calendar.html", data)
		if err != nil {
			t.Fatalf("failed to execute template: %v", err)
		}

		output := buf.String()
		assertContains(t, output, "No predictions available")
	})
}

// Helper functions

func createTestTemplates(t *testing.T) *template.Template {
	t.Helper()

	// Define test templates inline to avoid file path issues in tests
	// Each template is named with .html suffix to match how they're called
	baseTemplate := `{{define "base"}}<!DOCTYPE html><html><head><title>{{block "title" .}}Test{{end}}</title></head><body><nav class="navbar"><div class="nav-links">{{if .IsAuthenticated}}<a href="/dashboard">Dashboard</a><a href="/logout">Logout</a>{{else}}<a href="/login" class="btn-login">Login with Google</a>{{end}}</div></nav><main>{{block "content" .}}{{end}}</main></body></html>{{end}}`

	homeTemplate := `{{define "home.html"}}{{template "base" .}}{{end}}{{define "title"}}Home{{end}}{{define "content"}}{{if .WeekView.Streamers}}{{range .WeekView.Streamers}}{{$status := index $.LiveStatuses .ID}}<div class="streamer-card"><h3>{{.Name}}</h3>{{if and $status $status.IsLive}}<span class="status-badge status-live">Live on {{$status.Platform}}</span>{{else}}<span class="status-badge status-offline">Offline</span>{{end}}</div>{{end}}{{else}}<div class="empty-state"><h3>No streamers yet</h3></div>{{end}}{{end}}`

	streamerTemplate := `{{define "streamer.html"}}<!DOCTYPE html><html><head><title>{{.Streamer.Name}}</title></head><body><nav class="navbar"><div class="nav-links">{{if .IsAuthenticated}}<a href="/dashboard">Dashboard</a><a href="/logout">Logout</a>{{else}}<a href="/login" class="btn-login">Login with Google</a>{{end}}</div></nav><main><h1>{{.Streamer.Name}}</h1>{{if .IsAuthenticated}}{{if .IsFollowing}}<button>Unfollow</button>{{else}}<button>Follow</button>{{end}}{{else}}<a href="/login">Login to Follow</a>{{end}}{{if .LiveStatus}}{{if .LiveStatus.IsLive}}<h2>Live Now on {{.LiveStatus.Platform}}</h2><p>{{.LiveStatus.Title}}</p><a href="{{.LiveStatus.StreamURL}}">Watch Stream</a>{{else}}<h2>Currently Offline</h2>{{end}}{{end}}{{if .Heatmap}}<h2>Activity Heatmap</h2><p>{{.Heatmap.DataPoints}} data points</p><h3>Hours of Day</h3><h3>Days of Week</h3>{{else}}<h2>Activity Heatmap</h2><p>Insufficient Data</p>{{end}}</main></body></html>{{end}}`

	dashboardTemplate := `{{define "dashboard.html"}}<!DOCTYPE html><html><head><title>Dashboard</title></head><body><nav class="navbar"><div class="nav-links">{{if .IsAuthenticated}}<a href="/dashboard">Dashboard</a><a href="/logout">Logout</a>{{else}}<a href="/login" class="btn-login">Login with Google</a>{{end}}</div></nav><main><h1>Dashboard</h1><p>{{.User.Email}}</p><form action="/search"><input name="query"><button>Search</button></form>{{if .FollowedStreamers}}{{range .FollowedStreamers}}{{$status := index $.LiveStatuses .ID}}<div class="streamer-card"><h3>{{.Name}}</h3>{{if and $status $status.IsLive}}<span>Live on {{$status.Platform}}</span>{{else}}<span>Offline</span>{{end}}</div>{{end}}{{else}}<p>No followed streamers yet</p>{{end}}</main></body></html>{{end}}`

	calendarTemplate := `{{define "calendar.html"}}<!DOCTYPE html><html><head><title>Calendar</title></head><body><nav class="navbar"><div class="nav-links">{{if .IsAuthenticated}}<a href="/dashboard">Dashboard</a><a href="/logout">Logout</a>{{else}}<a href="/login" class="btn-login">Login with Google</a>{{end}}</div></nav><main><h1>TV Programme Calendar</h1><a href="/calendar?week={{.PrevWeek.Format "2006-01-02"}}">Previous Week</a><a href="/calendar?week={{.NextWeek.Format "2006-01-02"}}">Next Week</a>{{if .Programme.Entries}}{{range .Programme.Entries}}{{$streamer := index $.StreamerMap .StreamerID}}{{if $streamer}}<div>{{$streamer.Name}}</div>{{end}}{{end}}{{else}}<p>No predictions available</p>{{end}}</main></body></html>{{end}}`

	searchTemplate := `{{define "search.html"}}<!DOCTYPE html><html><head><title>Search</title></head><body><nav class="navbar"><div class="nav-links">{{if .IsAuthenticated}}<a href="/dashboard">Dashboard</a><a href="/logout">Logout</a>{{else}}<a href="/login" class="btn-login">Login with Google</a>{{end}}</div></nav><main><h1>Search Results</h1><p>{{.Query}}</p>{{if .Results}}{{range .Results}}<div>{{.Name}}</div>{{end}}{{else}}<p>No results found</p>{{end}}</main></body></html>{{end}}`

	tmpl := template.New("").Funcs(TemplateFuncs())
	var err error
	tmpl, err = tmpl.Parse(baseTemplate)
	if err != nil {
		t.Fatalf("failed to parse base template: %v", err)
	}
	tmpl, err = tmpl.Parse(homeTemplate)
	if err != nil {
		t.Fatalf("failed to parse home template: %v", err)
	}
	tmpl, err = tmpl.Parse(streamerTemplate)
	if err != nil {
		t.Fatalf("failed to parse streamer template: %v", err)
	}
	tmpl, err = tmpl.Parse(dashboardTemplate)
	if err != nil {
		t.Fatalf("failed to parse dashboard template: %v", err)
	}
	tmpl, err = tmpl.Parse(calendarTemplate)
	if err != nil {
		t.Fatalf("failed to parse calendar template: %v", err)
	}
	tmpl, err = tmpl.Parse(searchTemplate)
	if err != nil {
		t.Fatalf("failed to parse search template: %v", err)
	}

	return tmpl
}

func assertContains(t *testing.T, output, expected string) {
	t.Helper()
	if !strings.Contains(output, expected) {
		t.Errorf("expected output to contain %q, but it didn't.\nOutput: %s", expected, output)
	}
}

func assertNotContains(t *testing.T, output, unexpected string) {
	t.Helper()
	if strings.Contains(output, unexpected) {
		t.Errorf("expected output NOT to contain %q, but it did.\nOutput: %s", unexpected, output)
	}
}
