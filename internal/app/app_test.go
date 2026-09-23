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
			{ID: "cam", Type: "camera", Title: "Cam", URL: "https://www.youtube.com/watch?v=M7lc1UVf-VE", Autoplay: boolPtr(true), Muted: boolPtr(true), Controls: boolPtr(true), Width: 2, Height: 2},
			{ID: "w", Type: "weather", Title: "W", Latitude: &lat, Longitude: &long, Units: "imperial", Width: 1, Height: 1, Refresh: config.Duration{Duration: time.Minute}},
		}}},
	}
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
func boolPtr(v bool) *bool     { return &v }

func TestDashboardDoesNotExposeWeatherCoordinatesOrURL(t *testing.T) {
	s, err := New(testConfig(), testLogger(), WithCacheDir(t.TempDir()))
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
	s, _ := New(testConfig(), testLogger(), WithCacheDir(t.TempDir()))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("missing CSP")
	}
	if !strings.Contains(csp, "frame-src https://www.youtube.com") {
		t.Fatalf("CSP does not permit the configured YouTube embed: %s", csp)
	}
}

func TestDashboardExposesOnlySanitizedCameraEmbed(t *testing.T) {
	s, err := New(testConfig(), testLogger(), WithCacheDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/api/dashboard", nil))
	body := rr.Body.String()
	if !strings.Contains(body, "https://www.youtube.com/embed/M7lc1UVf-VE") {
		t.Fatalf("dashboard missing sanitized camera embed: %s", body)
	}
	if !strings.Contains(body, "https://www.youtube.com/watch?v=M7lc1UVf-VE") {
		t.Fatalf("dashboard missing canonical camera source link: %s", body)
	}
}

func TestLastKnownGoodSurvivesRefreshFailure(t *testing.T) {
	s, _ := New(testConfig(), testLogger(), WithCacheDir(t.TempDir()))
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
	s, _ := New(testConfig(), testLogger(), WithCacheDir(t.TempDir()))
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
	if _, err := New(cfg, testLogger(), WithCacheDir(t.TempDir())); err == nil {
		t.Fatal("expected missing theme error")
	}
}

func TestPersistentCacheSurvivesServerRestartAndRefreshFailure(t *testing.T) {
	cacheDir := t.TempDir()
	cfg := testConfig()

	first, err := New(cfg, testLogger(), WithCacheDir(cacheDir))
	if err != nil {
		t.Fatal(err)
	}
	first.provider = &fakeProvider{results: []provider.Result{{Data: map[string]any{"temperature": 72.0}, Source: "test"}}}
	w, _ := first.widgetConfig("a", "w")
	first.refreshOne(context.Background(), "a", w)
	firstSuccess := first.states[key("a", "w")].LastSuccess
	if firstSuccess.IsZero() {
		t.Fatal("first server never recorded a successful refresh")
	}

	second, err := New(cfg, testLogger(), WithCacheDir(cacheDir))
	if err != nil {
		t.Fatal(err)
	}
	loaded := second.states[key("a", "w")]
	if !loaded.LastSuccess.Equal(firstSuccess) {
		t.Fatalf("cached last-success mismatch: got %v want %v", loaded.LastSuccess, firstSuccess)
	}
	if loaded.Data == nil {
		t.Fatal("second server did not load persisted last-known-good data")
	}

	second.provider = &fakeProvider{errs: []error{errors.New("upstream still down")}}
	second.refreshOne(context.Background(), "a", w)
	if loaded.Data == nil {
		t.Fatal("failed post-restart refresh discarded persisted last-known-good data")
	}
	if !loaded.LastSuccess.Equal(firstSuccess) {
		t.Fatal("failed post-restart refresh changed persisted last-success timestamp")
	}
	if loaded.Err == "" {
		t.Fatal("failed post-restart refresh did not report the live upstream error")
	}
}

func TestPersistentCacheIgnoredWhenWidgetSourceChanges(t *testing.T) {
	cacheDir := t.TempDir()
	cfg := testConfig()
	first, err := New(cfg, testLogger(), WithCacheDir(cacheDir))
	if err != nil {
		t.Fatal(err)
	}
	first.provider = &fakeProvider{results: []provider.Result{{Data: map[string]any{"temperature": 72.0}, Source: "test"}}}
	w, _ := first.widgetConfig("a", "w")
	first.refreshOne(context.Background(), "a", w)

	changed := testConfig()
	newLat := 34.0
	region := changed.Regions["a"]
	for i := range region.Widgets {
		if region.Widgets[i].ID == "w" {
			region.Widgets[i].Latitude = &newLat
		}
	}
	changed.Regions["a"] = region

	second, err := New(changed, testLogger(), WithCacheDir(cacheDir))
	if err != nil {
		t.Fatal(err)
	}
	st := second.states[key("a", "w")]
	if !st.LastSuccess.IsZero() || st.Data != nil {
		t.Fatal("cache from previous widget source/configuration was reused")
	}
}

func TestRefreshRegionEndpointRequiresActionHeader(t *testing.T) {
	s, _ := New(testConfig(), testLogger(), WithCacheDir(t.TempDir()))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/region/a/refresh", nil)
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 403 {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
}

func TestRefreshRegionEndpointRefreshesExternalWidgets(t *testing.T) {
	s, _ := New(testConfig(), testLogger(), WithCacheDir(t.TempDir()))
	f := &fakeProvider{results: []provider.Result{{Data: map[string]any{"temperature": 73.0}, Source: "manual-test"}}}
	s.provider = f
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/region/a/refresh", nil)
	req.Header.Set("X-SigWatch-Action", "refresh-region")
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if f.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", f.calls)
	}
	if s.states[key("a", "w")].LastSuccess.IsZero() {
		t.Fatal("manual refresh did not update widget state")
	}
}
