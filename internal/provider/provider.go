package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"sigwatch/internal/config"
)

type Result struct {
	Data        any
	Image       []byte
	ContentType string
	Source      string
}

type Provider interface {
	Fetch(context.Context, config.Widget) (Result, error)
}

type HTTP struct {
	Client                 *http.Client
	earthquakeFeedOverride string
	fireFeedOverride       string
	coastalTideOverride    string
	coastalPointsOverride  string
}

func NewHTTP(timeout time.Duration) *HTTP {
	return &HTTP{Client: &http.Client{Timeout: timeout}}
}

func (h *HTTP) Fetch(ctx context.Context, w config.Widget) (Result, error) {
	switch w.Type {
	case "image":
		return h.fetchImage(ctx, w)
	case "weather":
		return h.fetchWeather(ctx, w)
	case "earthquake":
		return h.fetchEarthquakes(ctx, w)
	case "fire":
		return h.fetchFires(ctx, w)
	case "coastal":
		return h.fetchCoastal(ctx, w)
	case "system":
		return fetchSystem(), nil
	default:
		return Result{}, fmt.Errorf("widget type %q does not use a provider", w.Type)
	}
}

func (h *HTTP) fetchImage(ctx context.Context, w config.Widget) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.URL, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", "SigWatch/0.1 (+local situational-awareness dashboard)")
	resp, err := h.Client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Result{}, fmt.Errorf("upstream returned %s", resp.Status)
	}
	const max = 20 << 20
	b, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return Result{}, err
	}
	if len(b) > max {
		return Result{}, fmt.Errorf("image exceeds 20 MiB limit")
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = http.DetectContentType(b)
	}
	if len(ct) < 6 || ct[:6] != "image/" {
		return Result{}, fmt.Errorf("upstream content type %q is not an image", ct)
	}
	source := w.Attribution
	if source == "" {
		source = "Configured image source"
	}
	return Result{Image: b, ContentType: ct, Source: source}, nil
}

type openMeteoResponse struct {
	Current struct {
		Time          string  `json:"time"`
		Temperature   float64 `json:"temperature_2m"`
		Apparent      float64 `json:"apparent_temperature"`
		Humidity      float64 `json:"relative_humidity_2m"`
		Precipitation float64 `json:"precipitation"`
		WeatherCode   int     `json:"weather_code"`
		WindSpeed     float64 `json:"wind_speed_10m"`
		WindDirection float64 `json:"wind_direction_10m"`
	} `json:"current"`
	CurrentUnits map[string]string `json:"current_units"`
}

func (h *HTTP) fetchWeather(ctx context.Context, w config.Widget) (Result, error) {
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(*w.Latitude, 'f', 6, 64))
	q.Set("longitude", strconv.FormatFloat(*w.Longitude, 'f', 6, 64))
	q.Set("current", "temperature_2m,relative_humidity_2m,apparent_temperature,precipitation,weather_code,wind_speed_10m,wind_direction_10m")
	q.Set("timezone", "auto")
	if w.Units == "imperial" {
		q.Set("temperature_unit", "fahrenheit")
		q.Set("wind_speed_unit", "mph")
		q.Set("precipitation_unit", "inch")
	}
	endpoint := "https://api.open-meteo.com/v1/forecast?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", "SigWatch/0.1")
	resp, err := h.Client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Result{}, fmt.Errorf("weather upstream returned %s", resp.Status)
	}
	var v openMeteoResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&v); err != nil {
		return Result{}, fmt.Errorf("decode weather: %w", err)
	}
	data := map[string]any{
		"time": v.Current.Time, "temperature": v.Current.Temperature, "temperature_unit": v.CurrentUnits["temperature_2m"],
		"apparent_temperature": v.Current.Apparent, "humidity": v.Current.Humidity, "precipitation": v.Current.Precipitation,
		"precipitation_unit": v.CurrentUnits["precipitation"], "wind_speed": v.Current.WindSpeed, "wind_speed_unit": v.CurrentUnits["wind_speed_10m"],
		"wind_direction": v.Current.WindDirection, "weather_code": v.Current.WeatherCode, "summary": weatherSummary(v.Current.WeatherCode),
	}
	return Result{Data: data, Source: "Open-Meteo"}, nil
}

type noaaTideResponse struct {
	Predictions []struct {
		Time  string `json:"t"`
		Value string `json:"v"`
		Type  string `json:"type"`
	} `json:"predictions"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type nwsPointsResponse struct {
	Properties struct {
		Forecast string `json:"forecast"`
	} `json:"properties"`
}

type nwsForecastResponse struct {
	Properties struct {
		Updated     string `json:"updated"`
		GeneratedAt string `json:"generatedAt"`
		Periods     []struct {
			Name                       string `json:"name"`
			StartTime                  string `json:"startTime"`
			IsDaytime                  bool   `json:"isDaytime"`
			Temperature                int    `json:"temperature"`
			TemperatureUnit            string `json:"temperatureUnit"`
			ShortForecast              string `json:"shortForecast"`
			ProbabilityOfPrecipitation struct {
				Value *int `json:"value"`
			} `json:"probabilityOfPrecipitation"`
		} `json:"periods"`
	} `json:"properties"`
}

func (h *HTTP) fetchCoastal(ctx context.Context, w config.Widget) (Result, error) {
	var wg sync.WaitGroup
	var tides map[string]any
	var forecast map[string]any
	var tideErr, forecastErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		tides, tideErr = h.fetchTides(ctx, w)
	}()
	go func() {
		defer wg.Done()
		forecast, forecastErr = h.fetchNWSForecast(ctx, w)
	}()
	wg.Wait()

	if tideErr != nil && forecastErr != nil {
		return Result{}, fmt.Errorf("coastal sources unavailable: tides: %v; forecast: %v", tideErr, forecastErr)
	}
	data := map[string]any{}
	if tides != nil {
		data["tides"] = tides
	}
	if forecast != nil {
		data["forecast"] = forecast
	}
	if tideErr != nil {
		data["tides_error"] = tideErr.Error()
	}
	if forecastErr != nil {
		data["forecast_error"] = forecastErr.Error()
	}
	return Result{Data: data, Source: "NOAA CO-OPS + National Weather Service"}, nil
}

func (h *HTTP) fetchTides(ctx context.Context, w config.Widget) (map[string]any, error) {
	endpoint := h.coastalTideOverride
	if endpoint == "" {
		endpoint = "https://api.tidesandcurrents.noaa.gov/api/prod/datagetter"
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	q := u.Query()
	q.Set("begin_date", now.Format("20060102"))
	q.Set("end_date", now.Add(48*time.Hour).Format("20060102"))
	q.Set("station", w.TideStation)
	q.Set("product", "predictions")
	q.Set("datum", "MLLW")
	q.Set("time_zone", "gmt")
	q.Set("interval", "hilo")
	if w.Units == "metric" {
		q.Set("units", "metric")
	} else {
		q.Set("units", "english")
	}
	q.Set("application", "SigWatch")
	q.Set("format", "json")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SigWatch/0.1 (+local situational-awareness dashboard)")
	resp, err := h.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("NOAA tide upstream returned %s", resp.Status)
	}
	var v noaaTideResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&v); err != nil {
		return nil, fmt.Errorf("decode NOAA tides: %w", err)
	}
	if v.Error != nil && strings.TrimSpace(v.Error.Message) != "" {
		return nil, fmt.Errorf("NOAA tides: %s", strings.TrimSpace(v.Error.Message))
	}
	loc, err := time.LoadLocation(w.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load coastal timezone: %w", err)
	}
	events := make([]map[string]any, 0, 6)
	for _, prediction := range v.Predictions {
		tm, err := time.ParseInLocation("2006-01-02 15:04", prediction.Time, time.UTC)
		if err != nil || tm.Before(now.Add(-5*time.Minute)) {
			continue
		}
		height, err := strconv.ParseFloat(prediction.Value, 64)
		if err != nil {
			continue
		}
		kind := strings.ToUpper(strings.TrimSpace(prediction.Type))
		if kind != "H" && kind != "L" {
			continue
		}
		events = append(events, map[string]any{
			"time": tm.Format(time.RFC3339), "local_time": tm.In(loc).Format("2006-01-02T15:04:05Z07:00"),
			"height": height, "type": kind,
		})
		if len(events) >= 6 {
			break
		}
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("NOAA tides returned no upcoming high/low predictions")
	}
	unit := "ft"
	if w.Units == "metric" {
		unit = "m"
	}
	return map[string]any{
		"events": events, "station": w.TideStation, "datum": "MLLW", "height_unit": unit,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (h *HTTP) fetchNWSForecast(ctx context.Context, w config.Widget) (map[string]any, error) {
	pointsURL := h.coastalPointsOverride
	if pointsURL == "" {
		pointsURL = fmt.Sprintf("https://api.weather.gov/points/%.4f,%.4f", *w.Latitude, *w.Longitude)
	}
	var points nwsPointsResponse
	if err := h.getNWSJSON(ctx, pointsURL, &points); err != nil {
		return nil, fmt.Errorf("NWS point lookup: %w", err)
	}
	forecastURL := strings.TrimSpace(points.Properties.Forecast)
	if forecastURL == "" {
		return nil, fmt.Errorf("NWS point lookup did not return a forecast URL")
	}
	var forecast nwsForecastResponse
	if err := h.getNWSJSON(ctx, forecastURL, &forecast); err != nil {
		return nil, fmt.Errorf("NWS forecast: %w", err)
	}
	loc, err := time.LoadLocation(w.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load coastal timezone: %w", err)
	}

	order := make([]string, 0, 8)
	byDate := map[string]map[string]any{}
	for _, period := range forecast.Properties.Periods {
		start, err := time.Parse(time.RFC3339, period.StartTime)
		if err != nil {
			continue
		}
		localStart := start.In(loc)
		key := localStart.Format("2006-01-02")
		day, ok := byDate[key]
		if !ok {
			day = map[string]any{"date": key, "day_name": localStart.Format("Mon"), "precip_probability": 0}
			byDate[key] = day
			order = append(order, key)
		}
		temp := float64(period.Temperature)
		unit := strings.ToUpper(period.TemperatureUnit)
		if w.Units == "metric" && unit == "F" {
			temp = (temp - 32) * 5 / 9
			unit = "C"
		}
		if period.IsDaytime {
			day["high"] = math.Round(temp)
			day["summary"] = period.ShortForecast
		} else {
			day["low"] = math.Round(temp)
			if _, ok := day["summary"]; !ok {
				day["summary"] = period.ShortForecast
			}
		}
		day["temperature_unit"] = unit
		if period.ProbabilityOfPrecipitation.Value != nil {
			current, _ := day["precip_probability"].(int)
			if *period.ProbabilityOfPrecipitation.Value > current {
				day["precip_probability"] = *period.ProbabilityOfPrecipitation.Value
			}
		}
	}

	days := make([]map[string]any, 0, w.ForecastDays)
	for _, key := range order {
		if len(days) >= w.ForecastDays {
			break
		}
		days = append(days, byDate[key])
	}
	if len(days) == 0 {
		return nil, fmt.Errorf("NWS forecast returned no usable periods")
	}
	updated := forecast.Properties.Updated
	if updated == "" {
		updated = forecast.Properties.GeneratedAt
	}
	if updated == "" {
		updated = time.Now().UTC().Format(time.RFC3339)
	}
	return map[string]any{"days": days, "updated_at": updated}, nil
}

func (h *HTTP) getNWSJSON(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "SigWatch/0.1 (local situational-awareness dashboard)")
	req.Header.Set("Accept", "application/geo+json, application/ld+json, application/json")
	resp, err := h.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("upstream returned %s", resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

type usgsEarthquakeResponse struct {
	Metadata struct {
		Generated int64 `json:"generated"`
		Count     int   `json:"count"`
	} `json:"metadata"`
	Features []struct {
		ID         string `json:"id"`
		Properties struct {
			Magnitude *float64 `json:"mag"`
			Place     string   `json:"place"`
			Time      int64    `json:"time"`
			Updated   int64    `json:"updated"`
			URL       string   `json:"url"`
			Status    string   `json:"status"`
			Type      string   `json:"type"`
		} `json:"properties"`
		Geometry struct {
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
	} `json:"features"`
}

func (h *HTTP) fetchEarthquakes(ctx context.Context, w config.Widget) (Result, error) {
	endpoint := h.earthquakeFeedOverride
	if endpoint == "" {
		endpoint = "https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/all_day.geojson"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", "SigWatch/0.1 (+local situational-awareness dashboard)")
	resp, err := h.Client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Result{}, fmt.Errorf("USGS earthquake upstream returned %s", resp.Status)
	}

	var v usgsEarthquakeResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&v); err != nil {
		return Result{}, fmt.Errorf("decode USGS earthquakes: %w", err)
	}

	cutoff := time.Now().UTC().Add(-time.Duration(w.Hours) * time.Hour)
	type selectedEvent struct {
		time  time.Time
		value map[string]any
	}
	selected := make([]selectedEvent, 0, len(v.Features))
	for _, feature := range v.Features {
		if feature.Properties.Magnitude == nil || *feature.Properties.Magnitude < *w.MinMagnitude {
			continue
		}
		if feature.Properties.Type != "" && feature.Properties.Type != "earthquake" {
			continue
		}
		eventTime := time.UnixMilli(feature.Properties.Time).UTC()
		if eventTime.Before(cutoff) {
			continue
		}
		if len(feature.Geometry.Coordinates) < 3 {
			continue
		}
		longitude := feature.Geometry.Coordinates[0]
		latitude := feature.Geometry.Coordinates[1]
		depth := feature.Geometry.Coordinates[2]
		if haversineKM(*w.Latitude, *w.Longitude, latitude, longitude) > w.MaxRadiusKM {
			continue
		}

		event := map[string]any{
			"id":        feature.ID,
			"magnitude": *feature.Properties.Magnitude,
			"place":     feature.Properties.Place,
			"time":      eventTime.Format(time.RFC3339),
			"updated":   time.UnixMilli(feature.Properties.Updated).UTC().Format(time.RFC3339),
			"status":    feature.Properties.Status,
			"longitude": longitude,
			"latitude":  latitude,
			"depth_km":  depth,
		}
		if u, err := url.Parse(feature.Properties.URL); err == nil && u.Scheme == "https" && u.Host == "earthquake.usgs.gov" {
			event["url"] = u.String()
		}
		selected = append(selected, selectedEvent{time: eventTime, value: event})
	}

	sort.Slice(selected, func(i, j int) bool { return selected[i].time.After(selected[j].time) })
	if len(selected) > w.MaxEvents {
		selected = selected[:w.MaxEvents]
	}
	events := make([]map[string]any, 0, len(selected))
	for _, event := range selected {
		events = append(events, event.value)
	}

	data := map[string]any{
		"events":        events,
		"count":         len(events),
		"window_hours":  w.Hours,
		"min_magnitude": *w.MinMagnitude,
		"radius_km":     w.MaxRadiusKM,
	}
	return Result{Data: data, Source: "USGS Earthquake Hazards Program"}, nil
}

type wfigsFireResponse struct {
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Features []struct {
		Attributes struct {
			IncidentName             string   `json:"IncidentName"`
			IncidentShortDescription string   `json:"IncidentShortDescription"`
			LocalIncidentIdentifier  string   `json:"LocalIncidentIdentifier"`
			IncidentSize             *float64 `json:"IncidentSize"`
			InitialResponseAcres     *float64 `json:"InitialResponseAcres"`
			PercentContained         *float64 `json:"PercentContained"`
			POOCity                  string   `json:"POOCity"`
			POOCounty                string   `json:"POOCounty"`
			POOState                 string   `json:"POOState"`
			FireDiscoveryDateTime    *int64   `json:"FireDiscoveryDateTime"`
			ModifiedOnDateTime       *int64   `json:"ModifiedOnDateTime_dt"`
			InitialLatitude          *float64 `json:"InitialLatitude"`
			InitialLongitude         *float64 `json:"InitialLongitude"`
			UniqueFireIdentifier     string   `json:"UniqueFireIdentifier"`
			IrwinID                  string   `json:"IrwinID"`
			IncidentTypeCategory     string   `json:"IncidentTypeCategory"`
		} `json:"attributes"`
		Geometry struct {
			X *float64 `json:"x"`
			Y *float64 `json:"y"`
		} `json:"geometry"`
	} `json:"features"`
}

func (h *HTTP) fetchFires(ctx context.Context, w config.Widget) (Result, error) {
	endpoint := h.fireFeedOverride
	if endpoint == "" {
		endpoint = "https://services3.arcgis.com/T4QMspbfLg3qTGWY/arcgis/rest/services/WFIGS_Incident_Locations_Current/FeatureServer/0/query"
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return Result{}, err
	}
	q := u.Query()
	q.Set("where", "IncidentTypeCategory = 'WF' AND FireOutDateTime IS NULL")
	q.Set("geometry", strconv.FormatFloat(*w.Longitude, 'f', 6, 64)+","+strconv.FormatFloat(*w.Latitude, 'f', 6, 64))
	q.Set("geometryType", "esriGeometryPoint")
	q.Set("inSR", "4326")
	q.Set("spatialRel", "esriSpatialRelIntersects")
	q.Set("distance", strconv.FormatFloat(w.MaxRadiusKM, 'f', 3, 64))
	q.Set("units", "esriSRUnit_Kilometer")
	q.Set("outFields", "IncidentName,IncidentShortDescription,LocalIncidentIdentifier,IncidentSize,InitialResponseAcres,PercentContained,POOCity,POOCounty,POOState,FireDiscoveryDateTime,ModifiedOnDateTime_dt,InitialLatitude,InitialLongitude,UniqueFireIdentifier,IrwinID,IncidentTypeCategory")
	q.Set("returnGeometry", "true")
	q.Set("outSR", "4326")
	q.Set("resultRecordCount", "200")
	q.Set("f", "json")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", "SigWatch/0.1 (+local situational-awareness dashboard)")
	resp, err := h.Client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Result{}, fmt.Errorf("WFIGS fire upstream returned %s", resp.Status)
	}

	var v wfigsFireResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&v); err != nil {
		return Result{}, fmt.Errorf("decode WFIGS fires: %w", err)
	}
	if v.Error != nil {
		return Result{}, fmt.Errorf("WFIGS fire query failed (%d): %s", v.Error.Code, v.Error.Message)
	}

	type selectedFire struct {
		distance float64
		updated  time.Time
		value    map[string]any
	}
	selected := make([]selectedFire, 0, len(v.Features))
	for _, feature := range v.Features {
		if feature.Attributes.IncidentTypeCategory != "" && feature.Attributes.IncidentTypeCategory != "WF" {
			continue
		}
		lat, lon, ok := fireCoordinates(feature.Geometry.X, feature.Geometry.Y, feature.Attributes.InitialLatitude, feature.Attributes.InitialLongitude)
		if !ok {
			continue
		}
		distance := haversineKM(*w.Latitude, *w.Longitude, lat, lon)
		if distance > w.MaxRadiusKM {
			continue
		}
		name, named, incidentCode := fireDisplayName(
			feature.Attributes.IncidentName,
			feature.Attributes.IncidentShortDescription,
			feature.Attributes.LocalIncidentIdentifier,
		)
		if w.NamedOnly && !named {
			continue
		}

		acres := feature.Attributes.IncidentSize
		if acres == nil {
			acres = feature.Attributes.InitialResponseAcres
		}
		if w.MinAcres > 0 && (acres == nil || *acres < w.MinAcres) {
			continue
		}

		fire := map[string]any{
			"name":        name,
			"named":       named,
			"latitude":    lat,
			"longitude":   lon,
			"distance_km": distance,
			"city":        strings.TrimSpace(feature.Attributes.POOCity),
			"county":      strings.TrimSpace(feature.Attributes.POOCounty),
			"state":       strings.TrimSpace(feature.Attributes.POOState),
		}
		if incidentCode != "" {
			fire["incident_id"] = incidentCode
		}
		if feature.Attributes.UniqueFireIdentifier != "" {
			fire["unique_fire_identifier"] = feature.Attributes.UniqueFireIdentifier
		}
		if feature.Attributes.IrwinID != "" {
			fire["irwin_id"] = feature.Attributes.IrwinID
		}
		if named {
			fire["details_url"] = inciWebSearchURL(name)
		}
		if acres != nil {
			fire["acres"] = *acres
		}
		if feature.Attributes.PercentContained != nil {
			fire["percent_contained"] = *feature.Attributes.PercentContained
		}
		if feature.Attributes.FireDiscoveryDateTime != nil && *feature.Attributes.FireDiscoveryDateTime > 0 {
			fire["discovered"] = time.UnixMilli(*feature.Attributes.FireDiscoveryDateTime).UTC().Format(time.RFC3339)
		}
		var updated time.Time
		if feature.Attributes.ModifiedOnDateTime != nil && *feature.Attributes.ModifiedOnDateTime > 0 {
			updated = time.UnixMilli(*feature.Attributes.ModifiedOnDateTime).UTC()
			fire["updated"] = updated.Format(time.RFC3339)
		}
		selected = append(selected, selectedFire{distance: distance, updated: updated, value: fire})
	}

	sort.Slice(selected, func(i, j int) bool {
		if math.Abs(selected[i].distance-selected[j].distance) > 0.01 {
			return selected[i].distance < selected[j].distance
		}
		return selected[i].updated.After(selected[j].updated)
	})
	if len(selected) > w.MaxEvents {
		selected = selected[:w.MaxEvents]
	}
	fires := make([]map[string]any, 0, len(selected))
	for _, fire := range selected {
		fires = append(fires, fire.value)
	}
	data := map[string]any{
		"incidents":        fires,
		"count":            len(fires),
		"radius_km":        w.MaxRadiusKM,
		"named_only":       w.NamedOnly,
		"min_acres":        w.MinAcres,
		"official_map_url": "https://egp.wildfire.gov/maps",
	}
	return Result{Data: data, Source: "NIFC WFIGS current wildfire incidents"}, nil
}

var codeLikeFireName = regexp.MustCompile(`^[A-Z0-9_]{2,12}-[0-9]{4,}$`)

func fireDisplayName(rawName, shortDescription, localID string) (string, bool, string) {
	rawName = strings.TrimSpace(rawName)
	shortDescription = strings.TrimSpace(shortDescription)
	localID = strings.TrimSpace(localID)

	if rawName != "" && !looksCodeLikeFireName(rawName) {
		return rawName, true, localID
	}
	if shortDescription != "" && !looksCodeLikeFireName(shortDescription) {
		code := rawName
		if code == "" {
			code = localID
		}
		return shortDescription, true, code
	}
	code := rawName
	if code == "" {
		code = localID
	}
	return "Unnamed wildfire", false, code
}

func looksCodeLikeFireName(name string) bool {
	name = strings.TrimSpace(name)
	return codeLikeFireName.MatchString(name)
}

func inciWebSearchURL(name string) string {
	u := &url.URL{Scheme: "https", Host: "inciweb.wildfire.gov", Path: "/accessible-view"}
	q := u.Query()
	q.Set("combine", name)
	u.RawQuery = q.Encode()
	return u.String()
}

func fireCoordinates(x, y, initialLat, initialLon *float64) (float64, float64, bool) {
	if x != nil && y != nil {
		return *y, *x, true
	}
	if initialLat != nil && initialLon != nil {
		return *initialLat, *initialLon, true
	}
	return 0, 0, false
}

func haversineKM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKM = 6371.0088
	toRad := math.Pi / 180
	phi1, phi2 := lat1*toRad, lat2*toRad
	dPhi := (lat2 - lat1) * toRad
	dLambda := (lon2 - lon1) * toRad
	a := math.Sin(dPhi/2)*math.Sin(dPhi/2) + math.Cos(phi1)*math.Cos(phi2)*math.Sin(dLambda/2)*math.Sin(dLambda/2)
	return earthRadiusKM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

var processStarted = time.Now()

func fetchSystem() Result {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	host, _ := os.Hostname()
	return Result{Source: "Local system", Data: map[string]any{
		"hostname": host, "go_version": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH, "goroutines": runtime.NumGoroutine(),
		"memory_alloc_mb": float64(ms.Alloc) / (1024 * 1024), "process_uptime_seconds": int64(time.Since(processStarted).Seconds()),
	}}
}

func weatherSummary(code int) string {
	switch code {
	case 0:
		return "Clear sky"
	case 1:
		return "Mainly clear"
	case 2:
		return "Partly cloudy"
	case 3:
		return "Overcast"
	case 45, 48:
		return "Fog"
	case 51, 53, 55, 56, 57:
		return "Drizzle"
	case 61, 63, 65, 66, 67:
		return "Rain"
	case 71, 73, 75, 77:
		return "Snow"
	case 80, 81, 82:
		return "Rain showers"
	case 85, 86:
		return "Snow showers"
	case 95, 96, 99:
		return "Thunderstorm"
	default:
		return "Unknown"
	}
}
