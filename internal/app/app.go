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
	"strings"
	"sync"
	"time"

	"sigwatch/internal/config"
	"sigwatch/internal/freshness"
	"sigwatch/internal/provider"
)

//go:embed web/* web/static/* web/themes/default/*
var assets embed.FS

type widgetState struct {
	mu          sync.RWMutex
	LastSuccess time.Time
	LastAttempt time.Time
	Err         string
	Data        any
	Image       []byte
	ContentType string
	Source      string
	Version     string
}

type Server struct {
	cfg      *config.Config
	logger   *slog.Logger
	provider provider.Provider
	states   map[string]*widgetState
	started  time.Time
	tmpl     *template.Template
}

func New(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	if _, err := fs.Stat(assets, "web/themes/"+cfg.Theme+"/theme.css"); err != nil {
		return nil, fmt.Errorf("theme %q is not installed: %w", cfg.Theme, err)
	}
	t, err := template.ParseFS(assets, "web/index.html")
	if err != nil {
		return nil, err
	}
	s := &Server{cfg: cfg, logger: logger, provider: provider.NewHTTP(12 * time.Second), states: map[string]*widgetState{}, started: time.Now(), tmpl: t}
	for rn, r := range cfg.Regions {
		for _, w := range r.Widgets {
			if isExternal(w.Type) {
				s.states[key(rn, w.ID)] = &widgetState{}
			}
		}
	}
	return s, nil
}

func key(region, id string) string { return region + "/" + id }
func isExternal(t string) bool     { return t == "image" || t == "weather" || t == "system" }

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
	attempt := time.Now()
	result, err := s.provider.Fetch(ctx, w)
	st.mu.Lock()
	defer st.mu.Unlock()
	st.LastAttempt = attempt
	if err != nil {
		st.Err = err.Error()
		s.logger.Warn("widget refresh failed", "region", region, "widget", w.ID, "error", err)
		return
	}
	st.LastSuccess = time.Now()
	st.Err = ""
	st.Data = result.Data
	st.Source = result.Source
	if result.Image != nil {
		st.Image = result.Image
		st.ContentType = result.ContentType
		h := sha256.Sum256(result.Image)
		st.Version = hex.EncodeToString(h[:8])
	}
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
	mux.HandleFunc("/healthz", s.handleHealth)
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
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
}
type safeRegion struct {
	Label   string       `json:"label"`
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
	}
	return json.Marshal(alias{w.ID, w.Type, w.Title, w.Timezone, w.Width, w.Height, w.RefreshMS, w.Links})
}
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	regions := map[string]safeRegion{}
	for rn, rr := range s.cfg.Regions {
		sr := safeRegion{Label: rr.Label}
		for _, x := range rr.Widgets {
			sr.Widgets = append(sr.Widgets, safeWidget{ID: x.ID, Type: x.Type, Title: x.Title, Timezone: x.Timezone, Width: x.Width, Height: x.Height, RefreshMS: x.Refresh.Milliseconds(), Links: x.Links})
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
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"status": "ok", "uptime_seconds": int64(time.Since(s.started).Seconds())})
}
