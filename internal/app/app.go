package app

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"sigwatch/internal/cache"
	"sigwatch/internal/config"
	"sigwatch/internal/freshness"
	"sigwatch/internal/provider"
)

//go:embed web/* web/static/* web/themes/default/*
var assets embed.FS

type widgetState struct {
	refreshMu   sync.Mutex
	mu          sync.RWMutex
	LastSuccess time.Time
	LastAttempt time.Time
	Err         string
	Data        any
	Image       []byte
	ContentType string
	Source      string
	Version     string
	LastPersist time.Time
}

type Server struct {
	cfg      *config.Config
	logger   *slog.Logger
	provider provider.Provider
	cache    *cache.Store
	states   map[string]*widgetState
	started  time.Time
	tmpl     *template.Template
}

type options struct {
	cacheDir string
}

type Option func(*options)

// WithCacheDir overrides the platform-default persistent cache directory.
// An empty directory leaves default cache-directory selection in place.
func WithCacheDir(dir string) Option {
	return func(o *options) {
		if dir != "" {
			o.cacheDir = dir
		}
	}
}

func New(cfg *config.Config, logger *slog.Logger, opts ...Option) (*Server, error) {
	if _, err := fs.Stat(assets, "web/themes/"+cfg.Theme+"/theme.css"); err != nil {
		return nil, fmt.Errorf("theme %q is not installed: %w", cfg.Theme, err)
	}
	t, err := template.ParseFS(assets, "web/index.html")
	if err != nil {
		return nil, err
	}

	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	cacheDir := o.cacheDir
	if cacheDir == "" {
		cacheDir, err = cache.DefaultDir()
		if err != nil {
			logger.Warn("persistent cache disabled", "error", err)
		}
	}
	var diskCache *cache.Store
	if cacheDir != "" {
		diskCache, err = cache.New(cacheDir)
		if err != nil {
			logger.Warn("persistent cache disabled", "dir", cacheDir, "error", err)
		} else {
			logger.Info("persistent cache enabled", "dir", cacheDir)
		}
	}

	s := &Server{cfg: cfg, logger: logger, provider: provider.NewHTTP(12 * time.Second), cache: diskCache, states: map[string]*widgetState{}, started: time.Now(), tmpl: t}
	for rn, r := range cfg.Regions {
		for _, w := range r.Widgets {
			if !isExternal(w.Type) {
				continue
			}
			st := &widgetState{}
			s.states[key(rn, w.ID)] = st
			if cacheable(w.Type) {
				s.loadCachedState(rn, w, st)
			}
		}
	}
	return s, nil
}

func key(region, id string) string { return region + "/" + id }
func isExternal(t string) bool {
	return t == "image" || t == "weather" || t == "earthquake" || t == "fire" || t == "coastal" || t == "system"
}
func cacheable(t string) bool {
	return t == "image" || t == "weather" || t == "earthquake" || t == "fire" || t == "coastal"
}

func widgetFingerprint(w config.Widget) string {
	payload := struct {
		Type         string   `json:"type"`
		URL          string   `json:"url,omitempty"`
		Latitude     *float64 `json:"latitude,omitempty"`
		Longitude    *float64 `json:"longitude,omitempty"`
		Units        string   `json:"units,omitempty"`
		MaxRadiusKM  float64  `json:"max_radius_km,omitempty"`
		MinMagnitude *float64 `json:"min_magnitude,omitempty"`
		Hours        int      `json:"hours,omitempty"`
		MaxEvents    int      `json:"max_events,omitempty"`
		NamedOnly    bool     `json:"named_only,omitempty"`
		MinAcres     float64  `json:"min_acres,omitempty"`
		TideStation  string   `json:"tide_station,omitempty"`
		ForecastDays int      `json:"forecast_days,omitempty"`
		Timezone     string   `json:"timezone,omitempty"`
	}{w.Type, w.URL, w.Latitude, w.Longitude, w.Units, w.MaxRadiusKM, w.MinMagnitude, w.Hours, w.MaxEvents, w.NamedOnly, w.MinAcres, w.TideStation, w.ForecastDays, w.Timezone}
	b, _ := json.Marshal(payload)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:16])
}

func (s *Server) loadCachedState(region string, w config.Widget, st *widgetState) {
	if s.cache == nil {
		return
	}
	e, err := s.cache.Load(region, w.ID)
	if err != nil {
		if !os.IsNotExist(err) {
			s.logger.Warn("persistent cache load failed", "region", region, "widget", w.ID, "error", err)
		}
		return
	}
	if e.Fingerprint != widgetFingerprint(w) {
		s.logger.Info("ignoring cache after widget source change", "region", region, "widget", w.ID)
		return
	}
	st.LastSuccess = e.LastSuccess
	st.LastPersist = e.LastSuccess
	st.Data = e.Data
	st.Image = e.Image
	st.ContentType = e.ContentType
	st.Source = e.Source
	st.Version = e.Version
	s.logger.Info("loaded persisted widget cache", "region", region, "widget", w.ID, "last_success", e.LastSuccess)
}

func (s *Server) StartRefreshers(ctx context.Context) {
	for rn, r := range s.cfg.Regions {
		for _, w := range r.Widgets {
			if !isExternal(w.Type) {
				continue
			}
			rn, w := rn, w
			go s.refreshLoop(ctx, rn, w)
		}
	}
}
func (s *Server) refreshLoop(ctx context.Context, region string, w config.Widget) {
	s.refreshOne(ctx, region, w)
	ticker := time.NewTicker(w.Refresh.Duration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshOne(ctx, region, w)
		}
	}
}
func (s *Server) refreshOne(ctx context.Context, region string, w config.Widget) {
	st := s.states[key(region, w.ID)]
	st.refreshMu.Lock()
	defer st.refreshMu.Unlock()
	attempt := time.Now()
	result, err := s.provider.Fetch(ctx, w)
	st.mu.Lock()
	st.LastAttempt = attempt
	if err != nil {
		st.Err = err.Error()
		st.mu.Unlock()
		s.logger.Warn("widget refresh failed", "region", region, "widget", w.ID, "error", err)
		return
	}

	now := time.Now()
	st.LastSuccess = now
	st.Err = ""
	if w.Type == "coastal" {
		st.Data = mergeCoastalData(st.Data, result.Data)
	} else {
		st.Data = result.Data
	}
	st.Source = result.Source
	if result.Image != nil {
		st.Image = result.Image
		st.ContentType = result.ContentType
		h := sha256.Sum256(result.Image)
		st.Version = hex.EncodeToString(h[:8])
	}

	shouldPersist := false
	var entry cache.Entry
	if s.cache != nil && cacheable(w.Type) && (st.LastPersist.IsZero() || now.Sub(st.LastPersist) >= s.persistInterval(w)) {
		shouldPersist = true
		st.LastPersist = now
		entry = cache.Entry{
			Region: region, WidgetID: w.ID, Fingerprint: widgetFingerprint(w), LastSuccess: st.LastSuccess,
			Data: st.Data, Image: append([]byte(nil), st.Image...), ContentType: st.ContentType,
			Source: st.Source, Version: st.Version,
		}
	}
	st.mu.Unlock()

	if shouldPersist {
		if err := s.cache.Save(entry); err != nil {
			s.logger.Warn("persistent cache save failed", "region", region, "widget", w.ID, "error", err)
			st.mu.Lock()
			if st.LastPersist.Equal(now) {
				st.LastPersist = time.Time{}
			}
			st.mu.Unlock()
		}
	}
}

func mergeCoastalData(previous, current any) any {
	newMap, ok := current.(map[string]any)
	if !ok {
		return current
	}
	merged := make(map[string]any, len(newMap)+2)
	for k, v := range newMap {
		merged[k] = v
	}
	oldMap, ok := previous.(map[string]any)
	if !ok {
		return merged
	}
	for _, key := range []string{"tides", "forecast"} {
		if _, exists := merged[key]; !exists {
			if oldValue, exists := oldMap[key]; exists {
				merged[key] = oldValue
			}
		}
	}
	return merged
}

func (s *Server) persistInterval(w config.Widget) time.Duration {
	_, expireAfter := s.thresholds(w)
	d := expireAfter / 2
	if d > 30*time.Minute {
		d = 30 * time.Minute
	}
	if d < time.Minute {
		d = time.Minute
	}
	return d
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	staticFS, _ := fs.Sub(assets, "web/static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	themeFS, _ := fs.Sub(assets, "web/themes")
	mux.Handle("/themes/", http.StripPrefix("/themes/", http.FileServer(http.FS(themeFS))))
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/dashboard", s.handleDashboard)
	mux.HandleFunc("/api/widget/", s.handleWidget)
	mux.HandleFunc("/api/region/", s.handleRegionAction)
	mux.HandleFunc("/healthz", s.handleHealth)
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; frame-src https://www.youtube.com; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.tmpl.ExecuteTemplate(w, "index.html", map[string]any{"Title": s.cfg.Title, "Theme": s.cfg.Theme})
}

type safeWidget struct {
	ID, Type, Title, Timezone string
	Width, Height             int
	RefreshMS                 int64
	Links                     []config.Link
	EmbedURL                  string
	SourceURL                 string
}
type safeRegion struct {
	Label   string       `json:"label"`
	Layout  string       `json:"layout,omitempty"`
	Columns int          `json:"columns,omitempty"`
	Rows    int          `json:"rows,omitempty"`
	Widgets []safeWidget `json:"widgets"`
}

func (w safeWidget) MarshalJSON() ([]byte, error) {
	type alias struct {
		ID        string        `json:"id"`
		Type      string        `json:"type"`
		Title     string        `json:"title"`
		Timezone  string        `json:"timezone,omitempty"`
		Width     int           `json:"width"`
		Height    int           `json:"height"`
		RefreshMS int64         `json:"refresh_ms"`
		Links     []config.Link `json:"links,omitempty"`
		EmbedURL  string        `json:"embed_url,omitempty"`
		SourceURL string        `json:"source_url,omitempty"`
	}
	return json.Marshal(alias{w.ID, w.Type, w.Title, w.Timezone, w.Width, w.Height, w.RefreshMS, w.Links, w.EmbedURL, w.SourceURL})
}
func cameraSourceURL(w config.Widget) string {
	id, err := config.YouTubeVideoID(w.URL)
	if err != nil {
		return ""
	}
	return "https://www.youtube.com/watch?v=" + id
}

func cameraEmbedURL(w config.Widget) string {
	id, err := config.YouTubeVideoID(w.URL)
	if err != nil {
		return ""
	}
	q := url.Values{}
	q.Set("playsinline", "1")
	q.Set("rel", "0")
	if w.Autoplay != nil && *w.Autoplay {
		q.Set("autoplay", "1")
	}
	if w.Muted != nil && *w.Muted {
		q.Set("mute", "1")
	}
	if w.Controls != nil && !*w.Controls {
		q.Set("controls", "0")
	}
	return "https://www.youtube.com/embed/" + id + "?" + q.Encode()
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	regions := map[string]safeRegion{}
	for rn, rr := range s.cfg.Regions {
		sr := safeRegion{Label: rr.Label, Layout: rr.Layout, Columns: rr.Columns, Rows: rr.Rows}
		for _, x := range rr.Widgets {
			sw := safeWidget{ID: x.ID, Type: x.Type, Title: x.Title, Timezone: x.Timezone, Width: x.Width, Height: x.Height, RefreshMS: x.Refresh.Milliseconds(), Links: x.Links}
			if x.Type == "camera" {
				sw.EmbedURL = cameraEmbedURL(x)
				sw.SourceURL = cameraSourceURL(x)
			}
			sr.Widgets = append(sr.Widgets, sw)
		}
		regions[rn] = sr
	}
	writeJSON(w, map[string]any{"title": s.cfg.Title, "theme": s.cfg.Theme, "default_region": s.cfg.DefaultRegion, "regions": regions})
}
func (s *Server) widgetConfig(region, id string) (config.Widget, bool) {
	rr, ok := s.cfg.Regions[region]
	if !ok {
		return config.Widget{}, false
	}
	for _, x := range rr.Widgets {
		if x.ID == id {
			return x, true
		}
	}
	return config.Widget{}, false
}
func (s *Server) thresholds(w config.Widget) (time.Duration, time.Duration) {
	f := s.cfg.Freshness
	if w.Freshness != nil {
		f = *w.Freshness
	}
	return f.WarningAfter.Duration, f.ExpireAfter.Duration
}
func (s *Server) handleWidget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/widget/")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || len(parts) > 3 {
		http.NotFound(w, r)
		return
	}
	region, id := parts[0], parts[1]
	wc, ok := s.widgetConfig(region, id)
	if !ok || !isExternal(wc.Type) {
		http.NotFound(w, r)
		return
	}
	st := s.states[key(region, id)]
	warn, exp := s.thresholds(wc)
	st.mu.RLock()
	defer st.mu.RUnlock()
	status := freshness.Evaluate(time.Now(), st.LastSuccess, warn, exp)
	if len(parts) == 3 && parts[2] == "image" {
		if wc.Type != "image" {
			http.NotFound(w, r)
			return
		}
		if status == freshness.Expired || status == freshness.Unavailable || len(st.Image) == 0 {
			http.Error(w, "no current image", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", st.ContentType)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(st.Image)
		return
	}
	if len(parts) == 3 {
		http.NotFound(w, r)
		return
	}
	data := st.Data
	if status == freshness.Expired || status == freshness.Unavailable {
		data = nil
	}
	resp := map[string]any{"id": id, "status": status, "last_success": timeOrNil(st.LastSuccess), "last_attempt": timeOrNil(st.LastAttempt), "source": st.Source, "error": st.Err, "data": data}
	if wc.Type == "image" && status != freshness.Expired && status != freshness.Unavailable && len(st.Image) > 0 {
		resp["image_url"] = fmt.Sprintf("/api/widget/%s/%s/image?v=%s", region, id, st.Version)
	}
	writeJSON(w, resp)
}
func timeOrNil(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.Format(time.RFC3339)
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleRegionAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("X-SigWatch-Action") != "refresh-region" {
		http.Error(w, "missing refresh action header", http.StatusForbidden)
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/region/"), "/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[1] != "refresh" {
		http.NotFound(w, r)
		return
	}
	regionName := parts[0]
	region, ok := s.cfg.Regions[regionName]
	if !ok {
		http.NotFound(w, r)
		return
	}

	var wg sync.WaitGroup
	count := 0
	for _, widget := range region.Widgets {
		if !isExternal(widget.Type) {
			continue
		}
		count++
		widget := widget
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.refreshOne(r.Context(), regionName, widget)
		}()
	}
	wg.Wait()
	writeJSON(w, map[string]any{"status": "ok", "region": regionName, "refreshed": count})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"status": "ok", "uptime_seconds": int64(time.Since(s.started).Seconds())})
}
