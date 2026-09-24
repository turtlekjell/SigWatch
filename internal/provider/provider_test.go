package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sigwatch/internal/config"
)

func TestImageFetch(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}))
	defer ts.Close()
	h := NewHTTP(time.Second)
	got, err := h.Fetch(context.Background(), config.Widget{Type: "image", URL: ts.URL, Attribution: "Test source"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Image) != string(png) {
		t.Fatal("image bytes changed")
	}
	if got.ContentType != "image/png" {
		t.Fatalf("content type = %q", got.ContentType)
	}
	if got.Source != "Test source" {
		t.Fatalf("source = %q", got.Source)
	}
}

func TestImageRejectsNonImage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("not an image"))
	}))
	defer ts.Close()
	h := NewHTTP(time.Second)
	_, err := h.Fetch(context.Background(), config.Widget{Type: "image", URL: ts.URL})
	if err == nil || !strings.Contains(err.Error(), "not an image") {
		t.Fatalf("expected image validation error, got %v", err)
	}
}

func TestEarthquakeFetch(t *testing.T) {
	now := time.Now().UTC()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{
  "metadata":{"generated":%d,"count":3},
  "features":[
    {"id":"ci123","properties":{"mag":3.2,"place":"8 km NE of Testville, CA","time":%d,"updated":%d,"url":"https://earthquake.usgs.gov/earthquakes/eventpage/ci123","status":"reviewed","type":"earthquake"},"geometry":{"coordinates":[-117.9,33.8,7.2]}},
    {"id":"far","properties":{"mag":5.0,"place":"Far Away","time":%d,"updated":%d,"url":"https://earthquake.usgs.gov/earthquakes/eventpage/far","status":"reviewed","type":"earthquake"},"geometry":{"coordinates":[-122.4,37.8,4.0]}},
    {"id":"small","properties":{"mag":0.8,"place":"Too Small","time":%d,"updated":%d,"url":"https://earthquake.usgs.gov/earthquakes/eventpage/small","status":"automatic","type":"earthquake"},"geometry":{"coordinates":[-118.1,34.0,2.0]}}
  ]
}`, now.UnixMilli(), now.Add(-10*time.Minute).UnixMilli(), now.UnixMilli(), now.Add(-5*time.Minute).UnixMilli(), now.UnixMilli(), now.Add(-3*time.Minute).UnixMilli(), now.UnixMilli())))
	}))
	defer ts.Close()

	lat, lon, minMag := 34.05, -118.25, 1.5
	p := NewHTTP(time.Second)
	p.earthquakeFeedOverride = ts.URL
	got, err := p.Fetch(context.Background(), config.Widget{
		Type: "earthquake", Latitude: &lat, Longitude: &lon, MaxRadiusKM: 300,
		MinMagnitude: &minMag, Hours: 24, MaxEvents: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "USGS Earthquake Hazards Program" {
		t.Fatalf("source = %q", got.Source)
	}
	data, ok := got.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type %T", got.Data)
	}
	events, ok := data["events"].([]map[string]any)
	if !ok || len(events) != 1 {
		t.Fatalf("events = %#v", data["events"])
	}
	if events[0]["place"] != "8 km NE of Testville, CA" || events[0]["depth_km"] != 7.2 {
		t.Fatalf("unexpected event: %#v", events[0])
	}
}

func TestFireFetch(t *testing.T) {
	now := time.Now().UTC()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("geometryType"); got != "esriGeometryPoint" {
			t.Errorf("geometryType = %q", got)
		}
		if got := r.URL.Query().Get("units"); got != "esriSRUnit_Kilometer" {
			t.Errorf("units = %q", got)
		}
		if !strings.Contains(r.URL.Query().Get("where"), "IncidentTypeCategory") {
			t.Errorf("where = %q", r.URL.Query().Get("where"))
		}
		outFields := r.URL.Query().Get("outFields")
		if !strings.Contains(outFields, "IncidentShortDescription") || !strings.Contains(outFields, "LocalIncidentIdentifier") {
			t.Errorf("outFields missing cleanup fields: %q", outFields)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{
  "features":[
    {"attributes":{"IncidentName":"Test Fire","IncidentShortDescription":"","LocalIncidentIdentifier":"T-1","IncidentSize":125.5,"PercentContained":40,"POOCity":"Testville","POOCounty":"Orange","POOState":"US-CA","FireDiscoveryDateTime":%d,"ModifiedOnDateTime_dt":%d,"UniqueFireIdentifier":"2026-TEST","IrwinID":"irwin-test","IncidentTypeCategory":"WF"},"geometry":{"x":-117.95,"y":33.72}},
    {"attributes":{"IncidentName":"LAC-342856","IncidentShortDescription":"Brush fire near Test Canyon","LocalIncidentIdentifier":"342856","InitialResponseAcres":12,"PercentContained":20,"POOCity":"Canyon","POOCounty":"Los Angeles","POOState":"US-CA","ModifiedOnDateTime_dt":%d,"UniqueFireIdentifier":"2026-DESC","IncidentTypeCategory":"WF"},"geometry":{"x":-118.05,"y":33.80}},
    {"attributes":{"IncidentName":"LAC-341314","IncidentShortDescription":"","LocalIncidentIdentifier":"341314","IncidentSize":1,"PercentContained":0,"POOCity":"Smallville","POOCounty":"Los Angeles","POOState":"US-CA","ModifiedOnDateTime_dt":%d,"UniqueFireIdentifier":"2026-SMALL","IncidentTypeCategory":"WF"},"geometry":{"x":-118.02,"y":33.78}},
    {"attributes":{"IncidentName":"Far Fire","IncidentSize":900,"PercentContained":5,"POOCity":"Farville","POOCounty":"Far","POOState":"US-CA","ModifiedOnDateTime_dt":%d,"UniqueFireIdentifier":"2026-FAR","IncidentTypeCategory":"WF"},"geometry":{"x":-122.4,"y":37.8}}
  ]
}`, now.Add(-2*time.Hour).UnixMilli(), now.Add(-5*time.Minute).UnixMilli(), now.Add(-10*time.Minute).UnixMilli(), now.Add(-15*time.Minute).UnixMilli(), now.UnixMilli())))
	}))
	defer ts.Close()

	lat, lon := 33.66, -117.99
	p := NewHTTP(time.Second)
	p.fireFeedOverride = ts.URL
	got, err := p.Fetch(context.Background(), config.Widget{
		Type: "fire", Latitude: &lat, Longitude: &lon, MaxRadiusKM: 160, MaxEvents: 8, MinAcres: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "NIFC WFIGS current wildfire incidents" {
		t.Fatalf("source = %q", got.Source)
	}
	data, ok := got.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type %T", got.Data)
	}
	fires, ok := data["incidents"].([]map[string]any)
	if !ok || len(fires) != 2 {
		t.Fatalf("incidents = %#v", data["incidents"])
	}
	byName := map[string]map[string]any{}
	for _, fire := range fires {
		byName[fire["name"].(string)] = fire
	}
	testFire := byName["Test Fire"]
	if testFire == nil || testFire["acres"] != 125.5 || testFire["percent_contained"] != 40.0 {
		t.Fatalf("unexpected named fire: %#v", testFire)
	}
	if url, _ := testFire["details_url"].(string); !strings.Contains(url, "inciweb.wildfire.gov") || !strings.Contains(url, "Test+Fire") {
		t.Fatalf("unexpected details url: %q", url)
	}
	described := byName["Brush fire near Test Canyon"]
	if described == nil || described["incident_id"] != "LAC-342856" || described["acres"] != 12.0 || described["named"] != true {
		t.Fatalf("unexpected described fire: %#v", described)
	}
	if _, exists := byName["LAC-341314"]; exists {
		t.Fatal("small code-only fire should have been filtered by min_acres")
	}
	for _, fire := range fires {
		if d, ok := fire["distance_km"].(float64); !ok || d <= 0 || d >= 160 {
			t.Fatalf("unexpected distance: %#v", fire["distance_km"])
		}
	}
}

func TestFireNamedOnlyDropsCodeOnlyIncident(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "features":[
    {"attributes":{"IncidentName":"LAC-342856","IncidentShortDescription":"","LocalIncidentIdentifier":"342856","IncidentSize":25,"IncidentTypeCategory":"WF"},"geometry":{"x":-117.95,"y":33.72}},
    {"attributes":{"IncidentName":"LAC-999999","IncidentShortDescription":"Canyon brush fire","LocalIncidentIdentifier":"999999","IncidentSize":30,"IncidentTypeCategory":"WF"},"geometry":{"x":-117.96,"y":33.73}}
  ]
}`))
	}))
	defer ts.Close()

	lat, lon := 33.66, -117.99
	p := NewHTTP(time.Second)
	p.fireFeedOverride = ts.URL
	got, err := p.Fetch(context.Background(), config.Widget{
		Type: "fire", Latitude: &lat, Longitude: &lon, MaxRadiusKM: 160, MaxEvents: 8, NamedOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	data := got.Data.(map[string]any)
	fires := data["incidents"].([]map[string]any)
	if len(fires) != 1 || fires[0]["name"] != "Canyon brush fire" {
		t.Fatalf("unexpected named-only fires: %#v", fires)
	}
}

func TestCoastalFetchCombinesTidesAndForecast(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tides":
			if r.URL.Query().Get("product") != "predictions" || r.URL.Query().Get("interval") != "hilo" || r.URL.Query().Get("station") != "9410580" {
				t.Errorf("unexpected tide query: %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"predictions":[{"t":%q,"v":"4.8","type":"H"},{"t":%q,"v":"0.7","type":"L"}]}`,
				now.Add(2*time.Hour).Format("2006-01-02 15:04"), now.Add(8*time.Hour).Format("2006-01-02 15:04"))
		case "/points":
			if !strings.Contains(r.Header.Get("User-Agent"), "SigWatch") {
				t.Errorf("missing SigWatch user agent")
			}
			w.Header().Set("Content-Type", "application/geo+json")
			fmt.Fprintf(w, `{"properties":{"forecast":%q}}`, serverURL+"/forecast")
		case "/forecast":
			w.Header().Set("Content-Type", "application/geo+json")
			_, _ = w.Write([]byte(`{"properties":{"updated":"2026-09-23T19:00:00+00:00","periods":[
{"name":"Wednesday","startTime":"2026-09-23T12:00:00-07:00","isDaytime":true,"temperature":74,"temperatureUnit":"F","shortForecast":"Sunny","probabilityOfPrecipitation":{"value":5}},
{"name":"Wednesday Night","startTime":"2026-09-23T18:00:00-07:00","isDaytime":false,"temperature":61,"temperatureUnit":"F","shortForecast":"Mostly Clear","probabilityOfPrecipitation":{"value":2}},
{"name":"Thursday","startTime":"2026-09-24T06:00:00-07:00","isDaytime":true,"temperature":75,"temperatureUnit":"F","shortForecast":"Partly Sunny","probabilityOfPrecipitation":{"value":10}},
{"name":"Thursday Night","startTime":"2026-09-24T18:00:00-07:00","isDaytime":false,"temperature":60,"temperatureUnit":"F","shortForecast":"Mostly Cloudy","probabilityOfPrecipitation":{"value":15}}
]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	serverURL = ts.URL

	lat, lon := 33.66, -117.99
	p := NewHTTP(2 * time.Second)
	p.coastalTideOverride = ts.URL + "/tides"
	p.coastalPointsOverride = ts.URL + "/points"
	got, err := p.Fetch(context.Background(), config.Widget{
		Type: "coastal", Latitude: &lat, Longitude: &lon, Timezone: "America/Los_Angeles",
		TideStation: "9410580", ForecastDays: 2, Units: "imperial",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "NOAA CO-OPS + National Weather Service" {
		t.Fatalf("source = %q", got.Source)
	}
	data := got.Data.(map[string]any)
	tides := data["tides"].(map[string]any)
	if len(tides["events"].([]map[string]any)) != 2 {
		t.Fatalf("unexpected tides: %#v", tides)
	}
	forecast := data["forecast"].(map[string]any)
	days := forecast["days"].([]map[string]any)
	if len(days) != 2 || days[0]["high"] != float64(74) || days[0]["low"] != float64(61) {
		t.Fatalf("unexpected forecast: %#v", days)
	}
	if days[1]["precip_probability"] != 15 {
		t.Fatalf("unexpected precip: %#v", days[1])
	}
}

func TestCoastalFetchKeepsForecastWhenTidesFail(t *testing.T) {
	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tides":
			http.Error(w, "no tides", http.StatusServiceUnavailable)
		case "/points":
			fmt.Fprintf(w, `{"properties":{"forecast":%q}}`, serverURL+"/forecast")
		case "/forecast":
			_, _ = w.Write([]byte(`{"properties":{"periods":[{"name":"Today","startTime":"2026-09-23T12:00:00-07:00","isDaytime":true,"temperature":74,"temperatureUnit":"F","shortForecast":"Sunny","probabilityOfPrecipitation":{"value":0}}]}}`))
		}
	}))
	defer ts.Close()
	serverURL = ts.URL

	lat, lon := 33.66, -117.99
	p := NewHTTP(2 * time.Second)
	p.coastalTideOverride = ts.URL + "/tides"
	p.coastalPointsOverride = ts.URL + "/points"
	got, err := p.Fetch(context.Background(), config.Widget{
		Type: "coastal", Latitude: &lat, Longitude: &lon, Timezone: "America/Los_Angeles",
		TideStation: "9410580", ForecastDays: 1, Units: "imperial",
	})
	if err != nil {
		t.Fatal(err)
	}
	data := got.Data.(map[string]any)
	if data["forecast"] == nil || data["tides_error"] == nil {
		t.Fatalf("expected partial coastal result, got %#v", data)
	}
}
