package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
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

type HTTP struct{ Client *http.Client }

func NewHTTP(timeout time.Duration) *HTTP { return &HTTP{Client: &http.Client{Timeout: timeout}} }

func (h *HTTP) Fetch(ctx context.Context, w config.Widget) (Result, error) {
	switch w.Type {
	case "image":
		return h.fetchImage(ctx, w)
	case "weather":
		return h.fetchWeather(ctx, w)
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
