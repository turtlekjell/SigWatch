package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(s), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestLoadValid(t *testing.T) {
	p := write(t, `title: Test
listen: 127.0.0.1:8080
default_region: a
regions:
  a:
    widgets:
      - id: utc
        type: clock
        timezone: UTC
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Theme != "default" {
		t.Fatalf("theme=%q", c.Theme)
	}
	if c.Freshness.WarningAfter.String() != "15m0s" {
		t.Fatalf("warning=%s", c.Freshness.WarningAfter)
	}
}
func TestRejectRemoteListener(t *testing.T) {
	p := write(t, `listen: 0.0.0.0:8080
default_region: a
regions:
  a:
    widgets:
      - id: utc
        type: clock
        timezone: UTC
`)
	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("expected loopback error, got %v", err)
	}
}
func TestUnknownKeyFails(t *testing.T) {
	p := write(t, `default_region: a
mystery: true
regions:
  a:
    widgets:
      - id: utc
        type: clock
        timezone: UTC
`)
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected unknown key error")
	}
}

func TestRepositoryConfigLoads(t *testing.T) {
	if _, err := Load("../../config.yaml"); err != nil {
		t.Fatalf("repository config failed to load: %v", err)
	}
}

func TestEarthquakeDefaultsAndValidation(t *testing.T) {
	p := write(t, `default_region: a
regions:
  a:
    widgets:
      - id: quakes
        type: earthquake
        latitude: 34.05
        longitude: -118.25
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	w := c.Regions["a"].Widgets[0]
	if w.MaxRadiusKM != 300 || w.MinMagnitude == nil || *w.MinMagnitude != 1.5 || w.Hours != 24 || w.MaxEvents != 8 {
		t.Fatalf("unexpected earthquake defaults: %#v", w)
	}
}

func TestFireDefaultsAndValidation(t *testing.T) {
	p := write(t, `default_region: a
regions:
  a:
    widgets:
      - id: fires
        type: fire
        latitude: 33.66
        longitude: -117.99
        named_only: true
        min_acres: 5
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	w := c.Regions["a"].Widgets[0]
	if w.MaxRadiusKM != 300 || w.MaxEvents != 8 || w.Refresh.String() != "5m0s" {
		t.Fatalf("unexpected fire defaults: %#v", w)
	}
	if !w.NamedOnly || w.MinAcres != 5 {
		t.Fatalf("unexpected fire filters: %#v", w)
	}
}

func TestFireRejectsNegativeMinAcres(t *testing.T) {
	p := write(t, `default_region: a
regions:
  a:
    widgets:
      - id: fires
        type: fire
        latitude: 33.66
        longitude: -117.99
        min_acres: -1
`)
	if _, err := Load(p); err == nil || !strings.Contains(err.Error(), "min_acres") {
		t.Fatalf("expected min_acres validation error, got %v", err)
	}
}

func TestCameraYouTubeDefaultsAndURLForms(t *testing.T) {
	p := write(t, `default_region: a
regions:
  a:
    widgets:
      - id: cam
        type: camera
        title: Test Camera
        url: https://www.youtube.com/watch?v=M7lc1UVf-VE
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	w := c.Regions["a"].Widgets[0]
	if w.Autoplay == nil || !*w.Autoplay || w.Muted == nil || !*w.Muted || w.Controls == nil || !*w.Controls {
		t.Fatalf("unexpected camera defaults: %#v", w)
	}
	forms := []string{
		"https://www.youtube.com/watch?v=M7lc1UVf-VE",
		"https://youtu.be/M7lc1UVf-VE",
		"https://www.youtube.com/live/M7lc1UVf-VE",
		"https://www.youtube.com/embed/M7lc1UVf-VE",
	}
	for _, raw := range forms {
		id, err := YouTubeVideoID(raw)
		if err != nil || id != "M7lc1UVf-VE" {
			t.Fatalf("YouTubeVideoID(%q) = %q, %v", raw, id, err)
		}
	}
}

func TestCameraRejectsNonYouTubeURL(t *testing.T) {
	p := write(t, `default_region: a
regions:
  a:
    widgets:
      - id: cam
        type: camera
        url: https://example.com/camera
`)
	if _, err := Load(p); err == nil || !strings.Contains(err.Error(), "youtube.com") {
		t.Fatalf("expected YouTube validation error, got %v", err)
	}
}

func TestCoastalDefaultsAndValidation(t *testing.T) {
	p := write(t, `default_region: a
regions:
  a:
    widgets:
      - id: coast
        type: coastal
        latitude: 33.66
        longitude: -117.99
        timezone: America/Los_Angeles
        tide_station: "9410580"
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	w := c.Regions["a"].Widgets[0]
	if w.ForecastDays != 7 || w.Refresh.String() != "30m0s" || w.TideStation != "9410580" {
		t.Fatalf("unexpected coastal defaults: %#v", w)
	}
	if w.Freshness == nil || w.Freshness.WarningAfter.String() != "1h30m0s" || w.Freshness.ExpireAfter.String() != "6h0m0s" {
		t.Fatalf("unexpected coastal freshness defaults: %#v", w.Freshness)
	}
}

func TestCoastalRequiresStationAndTimezone(t *testing.T) {
	p := write(t, `default_region: a
regions:
  a:
    widgets:
      - id: coast
        type: coastal
        latitude: 33.66
        longitude: -117.99
`)
	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "tide_station") || !strings.Contains(err.Error(), "timezone") {
		t.Fatalf("expected coastal station/timezone validation error, got %v", err)
	}
}

func TestViewportRegionLayout(t *testing.T) {
	p := write(t, `default_region: local
regions:
  local:
    label: Local
    layout: viewport
    columns: 4
    rows: 2
    widgets:
      - id: utc
        type: clock
        timezone: UTC
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	r := c.Regions["local"]
	if r.Layout != "viewport" || r.Columns != 4 || r.Rows != 2 {
		t.Fatalf("unexpected viewport layout: %#v", r)
	}
}

func TestViewportRegionLayoutDefaults(t *testing.T) {
	p := write(t, `default_region: local
regions:
  local:
    layout: viewport
    widgets:
      - id: utc
        type: clock
        timezone: UTC
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	r := c.Regions["local"]
	if r.Columns != 4 || r.Rows != 2 {
		t.Fatalf("unexpected viewport defaults: %#v", r)
	}
}

func TestViewportRegionRejectsBadDimensions(t *testing.T) {
	p := write(t, `default_region: local
regions:
  local:
    layout: viewport
    columns: 0
    rows: 20
    widgets:
      - id: utc
        type: clock
        timezone: UTC
`)
	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "rows") {
		t.Fatalf("expected viewport dimension error, got %v", err)
	}
}
