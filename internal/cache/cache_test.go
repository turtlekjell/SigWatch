package cache

import (
	"os"
	"testing"
	"time"
)

func TestRoundTripDataAndImage(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	when := time.Now().UTC().Truncate(time.Second)
	want := Entry{
		Region: "maps", WidgetID: "radar", Fingerprint: "fp1", LastSuccess: when,
		Data: map[string]any{"value": 42.0}, Image: []byte("image-bytes"),
		ContentType: "image/gif", Source: "test", Version: "abc123",
	}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("maps", "radar")
	if err != nil {
		t.Fatal(err)
	}
	if !got.LastSuccess.Equal(when) || got.Fingerprint != want.Fingerprint || got.ContentType != want.ContentType || got.Source != want.Source || got.Version != want.Version {
		t.Fatalf("metadata mismatch: %#v", got)
	}
	if string(got.Image) != string(want.Image) {
		t.Fatalf("image mismatch: %q", got.Image)
	}
	m, ok := got.Data.(map[string]any)
	if !ok || m["value"] != 42.0 {
		t.Fatalf("data mismatch: %#v", got.Data)
	}
}

func TestMissingEntry(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Load("missing", "widget")
	if !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got %v", err)
	}
}
