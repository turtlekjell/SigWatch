package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sigwatch/internal/config"
	"sigwatch/internal/provider"
)

type fakeProvider struct {
	results []provider.Result
	errs    []error
	calls   int
}

func (f *fakeProvider) Fetch(context.Context, config.Widget) (provider.Result, error) {
	i := f.calls
	f.calls++
	if i < len(f.errs) && f.errs[i] != nil {
		return provider.Result{}, f.errs[i]
	}
	if i < len(f.results) {
		return f.results[i], nil
	}
	return provider.Result{}, nil
}

func testConfig() *config.Config {
	lat, long := 33.0, -118.0
	return &config.Config{
		Title: "Test", Listen: "127.0.0.1:0", Theme: "default", DefaultRegion: "a",
		Freshness: config.Freshness{WarningAfter: config.Duration{Duration: 15 * time.Minute}, ExpireAfter: config.Duration{Duration: time.Hour}},
		Regions: map[string]config.Region{"a": {Label: "A", Widgets: []config.Widget{
			{ID: "c", Type: "clock", Title: "UTC", Timezone: "UTC", Width: 1, Height: 1, Refresh: config.Duration{Duration: time.Minute}},
			{ID: "w", Type: "weather", Title: "W", Latitude: &lat, Longitude: &long, Units: "imperial", Width: 1, Height: 1, Refresh: config.Duration{Duration: time.Minute}},
		}}},
	}
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestDashboardDoesNotExposeWeatherCoordinatesOrURL(t *testing.T) {
	s, err := New(testConfig(), testLogger())
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/api/dashboard", nil))
	body := rr.Body.String()
	if strings.Contains(body, "33.0") || strings.Contains(body, "-118") || strings.Contains(body, "api.open-meteo") {
		t.Fatalf("dashboard leaks provider details: %s", body)
	}
}

func TestSecurityHeaders(t *testing.T) {
	s, _ := New(testConfig(), testLogger())
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing CSP")
	}
}

func TestLastKnownGoodSurvivesRefreshFailure(t *testing.T) {
	s, _ := New(testConfig(), testLogger())
	f := &fakeProvider{
		results: []provider.Result{{Data: map[string]any{"temperature": 72.0}, Source: "test"}},
		errs:    []error{nil, errors.New("upstream down")},
	}
	s.provider = f
	w, _ := s.widgetConfig("a", "w")
	s.refreshOne(context.Background(), "a", w)
	first := s.states[key("a", "w")].LastSuccess
	s.refreshOne(context.Background(), "a", w)
	st := s.states[key("a", "w")]
	if !st.LastSuccess.Equal(first) {
		t.Fatal("failed refresh changed last-success timestamp")
	}
	if st.Data == nil {
		t.Fatal("failed refresh discarded last-known-good data")
	}
	if st.Err == "" {
		t.Fatal("failed refresh did not retain error state")
	}
}

func TestExpiredWidgetHidesPrimaryData(t *testing.T) {
	s, _ := New(testConfig(), testLogger())
	st := s.states[key("a", "w")]
	st.LastSuccess = time.Now().Add(-2 * time.Hour)
	st.Data = map[string]any{"temperature": 72.0}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/api/widget/a/w", nil))
	body := rr.Body.String()
	if !strings.Contains(body, `"status":"expired"`) {
		t.Fatalf("expected expired state: %s", body)
	}
	if strings.Contains(body, "72") {
		t.Fatalf("expired primary data leaked into response: %s", body)
	}
	if !strings.Contains(body, "last_success") {
		t.Fatalf("expired response lacks last-success timestamp: %s", body)
	}
}

func TestMissingThemeFailsStartup(t *testing.T) {
	cfg := testConfig()
	cfg.Theme = "does-not-exist"
	if _, err := New(cfg, testLogger()); err == nil {
		t.Fatal("expected missing theme error")
	}
}
