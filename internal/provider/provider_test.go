package provider

import (
	"context"
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
