package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Duration struct{ time.Duration }

type Freshness struct {
	WarningAfter Duration `json:"warning_after"`
	ExpireAfter  Duration `json:"expire_after"`
}

type Config struct {
	Title         string
	Listen        string
	Theme         string
	DefaultRegion string
	Freshness     Freshness
	Regions       map[string]Region
}

type Region struct {
	Label   string
	Layout  string
	Columns int
	Rows    int
	Widgets []Widget
}

type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type Widget struct {
	ID           string
	Type         string
	Title        string
	URL          string
	Attribution  string
	Refresh      Duration
	Width        int
	Height       int
	Timezone     string
	Links        []Link
	Latitude     *float64
	Longitude    *float64
	Units        string
	MaxRadiusKM  float64
	MinMagnitude *float64
	Hours        int
	MaxEvents    int
	NamedOnly    bool
	MinAcres     float64
	Autoplay     *bool
	Muted        *bool
	Controls     *bool
	TideStation  string
	ForecastDays int
	Freshness    *Freshness
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	root, err := parseYAML(os.ExpandEnv(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	c, err := decodeConfig(root)
	if err != nil {
		return nil, err
	}
	defaults(c)
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func decodeConfig(m map[string]any) (*Config, error) {
	allowed := set("title", "listen", "theme", "default_region", "freshness", "regions")
	if err := unknownKeys("root", m, allowed); err != nil {
		return nil, err
	}
	c := &Config{Regions: map[string]Region{}}
	var err error
	if c.Title, err = optString(m, "title"); err != nil {
		return nil, fieldErr("title", err)
	}
	if c.Listen, err = optString(m, "listen"); err != nil {
		return nil, fieldErr("listen", err)
	}
	if c.Theme, err = optString(m, "theme"); err != nil {
		return nil, fieldErr("theme", err)
	}
	if c.DefaultRegion, err = optString(m, "default_region"); err != nil {
		return nil, fieldErr("default_region", err)
	}
	if v, ok := m["freshness"]; ok && v != nil {
		fm, ok := v.(map[string]any)
		if !ok {
			return nil, errors.New("freshness: must be a mapping")
		}
		f, err := decodeFreshness("freshness", fm)
		if err != nil {
			return nil, err
		}
		c.Freshness = f
	}
	rawRegions, ok := m["regions"]
	if !ok || rawRegions == nil {
		return c, nil
	}
	rm, ok := rawRegions.(map[string]any)
	if !ok {
		return nil, errors.New("regions: must be a mapping")
	}
	for name, raw := range rm {
		regionMap, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("regions.%s: must be a mapping", name)
		}
		r, err := decodeRegion(name, regionMap)
		if err != nil {
			return nil, err
		}
		c.Regions[name] = r
	}
	return c, nil
}

func decodeRegion(name string, m map[string]any) (Region, error) {
	if err := unknownKeys("regions."+name, m, set("label", "layout", "columns", "rows", "widgets")); err != nil {
		return Region{}, err
	}
	r := Region{}
	var err error
	if r.Label, err = optString(m, "label"); err != nil {
		return r, fieldErr("regions."+name+".label", err)
	}
	if r.Layout, err = optString(m, "layout"); err != nil {
		return r, fieldErr("regions."+name+".layout", err)
	}
	if raw, ok := m["columns"]; ok && raw != nil {
		r.Columns, err = intValue(raw)
		if err != nil {
			return r, fieldErr("regions."+name+".columns", err)
		}
	}
	if raw, ok := m["rows"]; ok && raw != nil {
		r.Rows, err = intValue(raw)
		if err != nil {
			return r, fieldErr("regions."+name+".rows", err)
		}
	}
	v, ok := m["widgets"]
	if !ok || v == nil {
		return r, nil
	}
	list, ok := v.([]any)
	if !ok {
		return r, fmt.Errorf("regions.%s.widgets: must be a list", name)
	}
	for i, raw := range list {
		wm, ok := raw.(map[string]any)
		if !ok {
			return r, fmt.Errorf("regions.%s.widgets[%d]: must be a mapping", name, i)
		}
		w, err := decodeWidget(fmt.Sprintf("regions.%s.widgets[%d]", name, i), wm)
		if err != nil {
			return r, err
		}
		r.Widgets = append(r.Widgets, w)
	}
	return r, nil
}

func decodeWidget(path string, m map[string]any) (Widget, error) {
	if err := unknownKeys(path, m, set("id", "type", "title", "url", "attribution", "refresh", "width", "height", "timezone", "links", "latitude", "longitude", "units", "max_radius_km", "min_magnitude", "hours", "max_events", "named_only", "min_acres", "autoplay", "muted", "controls", "tide_station", "forecast_days", "freshness")); err != nil {
		return Widget{}, err
	}
	w := Widget{}
	var err error
	for key, target := range map[string]*string{"id": &w.ID, "type": &w.Type, "title": &w.Title, "url": &w.URL, "attribution": &w.Attribution, "timezone": &w.Timezone, "units": &w.Units, "tide_station": &w.TideStation} {
		*target, err = optString(m, key)
		if err != nil {
			return w, fieldErr(path+"."+key, err)
		}
	}
	if raw, ok := m["refresh"]; ok && raw != nil {
		w.Refresh, err = parseDurationValue(raw)
		if err != nil {
			return w, fieldErr(path+".refresh", err)
		}
	}
	if raw, ok := m["width"]; ok && raw != nil {
		w.Width, err = intValue(raw)
		if err != nil {
			return w, fieldErr(path+".width", err)
		}
	}
	if raw, ok := m["height"]; ok && raw != nil {
		w.Height, err = intValue(raw)
		if err != nil {
			return w, fieldErr(path+".height", err)
		}
	}
	if raw, ok := m["latitude"]; ok && raw != nil {
		f, e := floatValue(raw)
		if e != nil {
			return w, fieldErr(path+".latitude", e)
		}
		w.Latitude = &f
	}
	if raw, ok := m["longitude"]; ok && raw != nil {
		f, e := floatValue(raw)
		if e != nil {
			return w, fieldErr(path+".longitude", e)
		}
		w.Longitude = &f
	}

	if raw, ok := m["max_radius_km"]; ok && raw != nil {
		w.MaxRadiusKM, err = floatValue(raw)
		if err != nil {
			return w, fieldErr(path+".max_radius_km", err)
		}
	}
	if raw, ok := m["min_magnitude"]; ok && raw != nil {
		f, e := floatValue(raw)
		if e != nil {
			return w, fieldErr(path+".min_magnitude", e)
		}
		w.MinMagnitude = &f
	}
	if raw, ok := m["hours"]; ok && raw != nil {
		w.Hours, err = intValue(raw)
		if err != nil {
			return w, fieldErr(path+".hours", err)
		}
	}
	if raw, ok := m["max_events"]; ok && raw != nil {
		w.MaxEvents, err = intValue(raw)
		if err != nil {
			return w, fieldErr(path+".max_events", err)
		}
	}
	if raw, ok := m["forecast_days"]; ok && raw != nil {
		w.ForecastDays, err = intValue(raw)
		if err != nil {
			return w, fieldErr(path+".forecast_days", err)
		}
	}
	if raw, ok := m["named_only"]; ok && raw != nil {
		w.NamedOnly, err = boolValue(raw)
		if err != nil {
			return w, fieldErr(path+".named_only", err)
		}
	}
	if raw, ok := m["min_acres"]; ok && raw != nil {
		w.MinAcres, err = floatValue(raw)
		if err != nil {
			return w, fieldErr(path+".min_acres", err)
		}
	}
	for key, target := range map[string]**bool{"autoplay": &w.Autoplay, "muted": &w.Muted, "controls": &w.Controls} {
		if raw, ok := m[key]; ok && raw != nil {
			v, e := boolValue(raw)
			if e != nil {
				return w, fieldErr(path+"."+key, e)
			}
			*target = &v
		}
	}
	if raw, ok := m["freshness"]; ok && raw != nil {
		fm, ok := raw.(map[string]any)
		if !ok {
			return w, fmt.Errorf("%s.freshness: must be a mapping", path)
		}
		f, e := decodeFreshness(path+".freshness", fm)
		if e != nil {
			return w, e
		}
		w.Freshness = &f
	}
	if raw, ok := m["links"]; ok && raw != nil {
		list, ok := raw.([]any)
		if !ok {
			return w, fmt.Errorf("%s.links: must be a list", path)
		}
		for i, item := range list {
			lm, ok := item.(map[string]any)
			if !ok {
				return w, fmt.Errorf("%s.links[%d]: must be a mapping", path, i)
			}
			if err := unknownKeys(fmt.Sprintf("%s.links[%d]", path, i), lm, set("label", "url")); err != nil {
				return w, err
			}
			label, e := optString(lm, "label")
			if e != nil {
				return w, fieldErr(fmt.Sprintf("%s.links[%d].label", path, i), e)
			}
			u, e := optString(lm, "url")
			if e != nil {
				return w, fieldErr(fmt.Sprintf("%s.links[%d].url", path, i), e)
			}
			w.Links = append(w.Links, Link{Label: label, URL: u})
		}
	}
	return w, nil
}

func decodeFreshness(path string, m map[string]any) (Freshness, error) {
	if err := unknownKeys(path, m, set("warning_after", "expire_after")); err != nil {
		return Freshness{}, err
	}
	var f Freshness
	var err error
	if raw, ok := m["warning_after"]; ok && raw != nil {
		f.WarningAfter, err = parseDurationValue(raw)
		if err != nil {
			return f, fieldErr(path+".warning_after", err)
		}
	}
	if raw, ok := m["expire_after"]; ok && raw != nil {
		f.ExpireAfter, err = parseDurationValue(raw)
		if err != nil {
			return f, fieldErr(path+".expire_after", err)
		}
	}
	return f, nil
}

func defaults(c *Config) {
	if c.Title == "" {
		c.Title = "SigWatch"
	}
	if c.Listen == "" {
		c.Listen = "127.0.0.1:8080"
	}
	if c.Theme == "" {
		c.Theme = "default"
	}
	if c.Freshness.WarningAfter.Duration == 0 {
		c.Freshness.WarningAfter.Duration = 15 * time.Minute
	}
	if c.Freshness.ExpireAfter.Duration == 0 {
		c.Freshness.ExpireAfter.Duration = time.Hour
	}
	for name, r := range c.Regions {
		if r.Label == "" {
			r.Label = name
		}
		if r.Layout == "viewport" {
			if r.Columns == 0 {
				r.Columns = 4
			}
			if r.Rows == 0 {
				r.Rows = 2
			}
		}
		for i := range r.Widgets {
			w := &r.Widgets[i]
			if w.Title == "" {
				w.Title = w.ID
			}
			if w.Width <= 0 {
				w.Width = 1
			}
			if w.Height <= 0 {
				w.Height = 1
			}
			if w.Refresh.Duration == 0 {
				switch w.Type {
				case "image":
					w.Refresh.Duration = 2 * time.Minute
				case "weather":
					w.Refresh.Duration = 10 * time.Minute
				case "earthquake":
					w.Refresh.Duration = 2 * time.Minute
				case "fire":
					w.Refresh.Duration = 5 * time.Minute
				case "coastal":
					w.Refresh.Duration = 30 * time.Minute
				case "system":
					w.Refresh.Duration = 10 * time.Second
				default:
					w.Refresh.Duration = time.Minute
				}
			}
			if w.Units == "" {
				w.Units = "imperial"
			}
			if w.Type == "earthquake" {
				if w.MaxRadiusKM == 0 {
					w.MaxRadiusKM = 300
				}
				if w.MinMagnitude == nil {
					v := 1.5
					w.MinMagnitude = &v
				}
				if w.Hours == 0 {
					w.Hours = 24
				}
				if w.MaxEvents == 0 {
					w.MaxEvents = 8
				}
			}
			if w.Type == "fire" {
				if w.MaxRadiusKM == 0 {
					w.MaxRadiusKM = 300
				}
				if w.MaxEvents == 0 {
					w.MaxEvents = 8
				}
			}
			if w.Type == "coastal" {
				if w.ForecastDays == 0 {
					w.ForecastDays = 7
				}
				if w.Freshness == nil {
					w.Freshness = &Freshness{WarningAfter: Duration{Duration: 90 * time.Minute}, ExpireAfter: Duration{Duration: 6 * time.Hour}}
				}
			}
			if w.Type == "camera" {
				if w.Autoplay == nil {
					v := true
					w.Autoplay = &v
				}
				if w.Muted == nil {
					v := true
					w.Muted = &v
				}
				if w.Controls == nil {
					v := true
					w.Controls = &v
				}
			}
			if w.Freshness != nil {
				if w.Freshness.WarningAfter.Duration == 0 {
					w.Freshness.WarningAfter = c.Freshness.WarningAfter
				}
				if w.Freshness.ExpireAfter.Duration == 0 {
					w.Freshness.ExpireAfter = c.Freshness.ExpireAfter
				}
			}
		}
		c.Regions[name] = r
	}
}

func (c *Config) Validate() error {
	var problems []string
	if !safeName(c.Theme) {
		problems = append(problems, "theme: must contain only letters, numbers, dash, and underscore")
	}
	host, _, err := net.SplitHostPort(c.Listen)
	if err != nil {
		problems = append(problems, "listen: must be host:port (for example 127.0.0.1:8080)")
	} else if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		problems = append(problems, "listen: MVP requires a loopback address such as 127.0.0.1 or [::1]")
	}
	if c.Freshness.WarningAfter.Duration <= 0 {
		problems = append(problems, "freshness.warning_after: must be greater than zero")
	}
	if c.Freshness.ExpireAfter.Duration <= 0 {
		problems = append(problems, "freshness.expire_after: must be greater than zero")
	}
	if c.Freshness.WarningAfter.Duration >= c.Freshness.ExpireAfter.Duration {
		problems = append(problems, "freshness: warning_after must be less than expire_after")
	}
	if len(c.Regions) == 0 {
		problems = append(problems, "regions: at least one region is required")
	}
	if c.DefaultRegion == "" {
		problems = append(problems, "default_region: is required")
	} else if _, ok := c.Regions[c.DefaultRegion]; !ok {
		problems = append(problems, "default_region: does not match a configured region")
	}
	names := make([]string, 0, len(c.Regions))
	for n := range c.Regions {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, regionName := range names {
		if !safeName(regionName) {
			problems = append(problems, fmt.Sprintf("regions.%s: region key must contain only letters, numbers, dash, and underscore", regionName))
		}
		r := c.Regions[regionName]
		if r.Layout != "" && r.Layout != "flow" && r.Layout != "viewport" {
			problems = append(problems, fmt.Sprintf("regions.%s.layout: must be flow or viewport", regionName))
		}
		if r.Layout == "viewport" {
			if r.Columns < 1 || r.Columns > 12 {
				problems = append(problems, fmt.Sprintf("regions.%s.columns: must be between 1 and 12 for viewport layout", regionName))
			}
			if r.Rows < 1 || r.Rows > 8 {
				problems = append(problems, fmt.Sprintf("regions.%s.rows: must be between 1 and 8 for viewport layout", regionName))
			}
		}
		seen := map[string]bool{}
		if len(r.Widgets) == 0 {
			problems = append(problems, fmt.Sprintf("regions.%s.widgets: at least one widget is required", regionName))
		}
		for i, w := range r.Widgets {
			p := fmt.Sprintf("regions.%s.widgets[%d]", regionName, i)
			if strings.TrimSpace(w.ID) == "" {
				problems = append(problems, p+".id: is required")
			}
			if seen[w.ID] {
				problems = append(problems, p+".id: duplicate widget id "+w.ID)
			}
			seen[w.ID] = true
			if strings.ContainsAny(w.ID, "/\\") {
				problems = append(problems, p+".id: must not contain slash characters")
			}
			switch w.Type {
			case "image":
				if err := validateHTTPURL(w.URL); err != nil {
					problems = append(problems, p+".url: "+err.Error())
				}
			case "clock":
				if w.Timezone == "" {
					problems = append(problems, p+".timezone: is required for clock widgets")
				} else if _, err := time.LoadLocation(w.Timezone); err != nil {
					problems = append(problems, p+".timezone: unknown time zone")
				}
			case "links":
				if len(w.Links) == 0 {
					problems = append(problems, p+".links: at least one link is required")
				}
				for j, l := range w.Links {
					if l.Label == "" {
						problems = append(problems, fmt.Sprintf("%s.links[%d].label: is required", p, j))
					}
					if err := validateHTTPURL(l.URL); err != nil {
						problems = append(problems, fmt.Sprintf("%s.links[%d].url: %s", p, j, err))
					}
				}
			case "weather":
				if w.Latitude == nil || w.Longitude == nil {
					problems = append(problems, p+": weather requires latitude and longitude")
				} else if *w.Latitude < -90 || *w.Latitude > 90 || *w.Longitude < -180 || *w.Longitude > 180 {
					problems = append(problems, p+": latitude/longitude out of range")
				}
				if w.Units != "imperial" && w.Units != "metric" {
					problems = append(problems, p+".units: must be imperial or metric")
				}
			case "earthquake":
				if w.Latitude == nil || w.Longitude == nil {
					problems = append(problems, p+": earthquake requires latitude and longitude")
				} else if *w.Latitude < -90 || *w.Latitude > 90 || *w.Longitude < -180 || *w.Longitude > 180 {
					problems = append(problems, p+": latitude/longitude out of range")
				}
				if w.MaxRadiusKM <= 0 || w.MaxRadiusKM > 20001.6 {
					problems = append(problems, p+".max_radius_km: must be greater than 0 and no more than 20001.6")
				}
				if w.MinMagnitude == nil {
					problems = append(problems, p+".min_magnitude: is required after defaults")
				}
				if w.Hours < 1 || w.Hours > 24 {
					problems = append(problems, p+".hours: must be between 1 and 24")
				}
				if w.MaxEvents < 1 || w.MaxEvents > 50 {
					problems = append(problems, p+".max_events: must be between 1 and 50")
				}
			case "fire":
				if w.Latitude == nil || w.Longitude == nil {
					problems = append(problems, p+": fire requires latitude and longitude")
				} else if *w.Latitude < -90 || *w.Latitude > 90 || *w.Longitude < -180 || *w.Longitude > 180 {
					problems = append(problems, p+": latitude/longitude out of range")
				}
				if w.MaxRadiusKM <= 0 || w.MaxRadiusKM > 20001.6 {
					problems = append(problems, p+".max_radius_km: must be greater than 0 and no more than 20001.6")
				}
				if w.MaxEvents < 1 || w.MaxEvents > 50 {
					problems = append(problems, p+".max_events: must be between 1 and 50")
				}
				if w.MinAcres < 0 {
					problems = append(problems, p+".min_acres: must be zero or greater")
				}
			case "camera":
				if _, err := YouTubeVideoID(w.URL); err != nil {
					problems = append(problems, p+".url: "+err.Error())
				}
			case "coastal":
				if w.Latitude == nil || w.Longitude == nil {
					problems = append(problems, p+": coastal requires latitude and longitude")
				} else if *w.Latitude < -90 || *w.Latitude > 90 || *w.Longitude < -180 || *w.Longitude > 180 {
					problems = append(problems, p+": latitude/longitude out of range")
				}
				if ok, _ := regexp.MatchString(`^[A-Za-z0-9-]{3,20}$`, w.TideStation); !ok {
					problems = append(problems, p+".tide_station: is required and must contain only letters, numbers, or dash")
				}
				if w.Timezone == "" {
					problems = append(problems, p+".timezone: is required for coastal widgets")
				} else if _, err := time.LoadLocation(w.Timezone); err != nil {
					problems = append(problems, p+".timezone: unknown time zone")
				}
				if w.ForecastDays < 1 || w.ForecastDays > 7 {
					problems = append(problems, p+".forecast_days: must be between 1 and 7")
				}
				if w.Units != "imperial" && w.Units != "metric" {
					problems = append(problems, p+".units: must be imperial or metric")
				}
			case "system":
			default:
				problems = append(problems, p+".type: unsupported type "+w.Type)
			}
			if w.Freshness != nil && w.Freshness.WarningAfter.Duration >= w.Freshness.ExpireAfter.Duration {
				problems = append(problems, p+".freshness: warning_after must be less than expire_after")
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("configuration invalid:\n - %s", strings.Join(problems, "\n - "))
	}
	return nil
}

func YouTubeVideoID(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return "", errors.New("must be an HTTPS YouTube URL")
	}
	host := strings.ToLower(u.Hostname())
	var id string
	switch host {
	case "youtube.com", "www.youtube.com", "m.youtube.com":
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if u.Path == "/watch" {
			id = u.Query().Get("v")
		} else if len(parts) == 2 && (parts[0] == "live" || parts[0] == "embed" || parts[0] == "shorts") {
			id = parts[1]
		}
	case "youtu.be", "www.youtu.be":
		id = strings.Trim(u.Path, "/")
	default:
		return "", errors.New("camera currently supports youtube.com or youtu.be URLs")
	}
	if ok, _ := regexp.MatchString(`^[A-Za-z0-9_-]{11}$`, id); !ok {
		return "", errors.New("could not find a valid YouTube video ID in URL")
	}
	return id, nil
}

func parseDurationValue(v any) (Duration, error) {
	s, ok := v.(string)
	if !ok {
		return Duration{}, errors.New("must be a duration string such as 30s, 2m, or 1h")
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return Duration{}, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	if d <= 0 {
		return Duration{}, errors.New("must be greater than zero")
	}
	return Duration{Duration: d}, nil
}
func optString(m map[string]any, key string) (string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", errors.New("must be a string")
	}
	return s, nil
}
func intValue(v any) (int, error) {
	switch x := v.(type) {
	case int64:
		return int(x), nil
	case float64:
		if x == float64(int(x)) {
			return int(x), nil
		}
	}
	return 0, errors.New("must be an integer")
}
func floatValue(v any) (float64, error) {
	switch x := v.(type) {
	case int64:
		return float64(x), nil
	case float64:
		return x, nil
	}
	return 0, errors.New("must be a number")
}
func boolValue(v any) (bool, error) {
	b, ok := v.(bool)
	if !ok {
		return false, errors.New("must be true or false")
	}
	return b, nil
}
func fieldErr(path string, err error) error { return fmt.Errorf("%s: %w", path, err) }
func set(keys ...string) map[string]struct{} {
	m := map[string]struct{}{}
	for _, k := range keys {
		m[k] = struct{}{}
	}
	return m
}
func unknownKeys(path string, m map[string]any, allowed map[string]struct{}) error {
	var bad []string
	for k := range m {
		if _, ok := allowed[k]; !ok {
			bad = append(bad, k)
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return fmt.Errorf("%s: unknown configuration key(s): %s", path, strings.Join(bad, ", "))
	}
	return nil
}
func safeName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func validateHTTPURL(s string) error {
	if s == "" {
		return errors.New("is required")
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("must be an http(s) URL")
	}
	return nil
}
