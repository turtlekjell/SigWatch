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
