package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const formatVersion = 1

type Entry struct {
	Region      string
	WidgetID    string
	Fingerprint string
	LastSuccess time.Time
	Data        any
	Image       []byte
	ContentType string
	Source      string
	Version     string
}

type metadata struct {
	FormatVersion int             `json:"format_version"`
	Region        string          `json:"region"`
	WidgetID      string          `json:"widget_id"`
	Fingerprint   string          `json:"fingerprint"`
	LastSuccess   time.Time       `json:"last_success"`
	Data          json.RawMessage `json:"data,omitempty"`
	HasImage      bool            `json:"has_image,omitempty"`
	ContentType   string          `json:"content_type,omitempty"`
	Source        string          `json:"source,omitempty"`
	Version       string          `json:"version,omitempty"`
}

type Store struct {
	dir string
}

func New(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("cache directory is empty")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}
	return &Store{dir: dir}, nil
}

func DefaultDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("determine user cache directory: %w", err)
	}
	return filepath.Join(base, "sigwatch"), nil
}

func (s *Store) Load(region, widgetID string) (Entry, error) {
	base := s.base(region, widgetID)
	metaBytes, err := os.ReadFile(base + ".json")
	if err != nil {
		return Entry{}, err
	}
	var m metadata
	if err := json.Unmarshal(metaBytes, &m); err != nil {
		return Entry{}, fmt.Errorf("decode cache metadata: %w", err)
	}
	if m.FormatVersion != formatVersion {
		return Entry{}, fmt.Errorf("unsupported cache format version %d", m.FormatVersion)
	}
	if m.Region != region || m.WidgetID != widgetID {
		return Entry{}, errors.New("cache identity mismatch")
	}
	entry := Entry{
		Region: region, WidgetID: widgetID, Fingerprint: m.Fingerprint, LastSuccess: m.LastSuccess,
		ContentType: m.ContentType, Source: m.Source, Version: m.Version,
	}
	if len(m.Data) > 0 && string(m.Data) != "null" {
		var data any
		if err := json.Unmarshal(m.Data, &data); err != nil {
			return Entry{}, fmt.Errorf("decode cached data: %w", err)
		}
		entry.Data = data
	}
	if m.HasImage {
		entry.Image, err = os.ReadFile(base + ".img")
		if err != nil {
			return Entry{}, fmt.Errorf("read cached image: %w", err)
		}
	}
	return entry, nil
}

func (s *Store) Save(e Entry) error {
	if e.LastSuccess.IsZero() {
		return errors.New("cannot cache entry without last-success time")
	}
	base := s.base(e.Region, e.WidgetID)
	m := metadata{
		FormatVersion: formatVersion,
		Region:        e.Region, WidgetID: e.WidgetID, Fingerprint: e.Fingerprint, LastSuccess: e.LastSuccess,
		HasImage: len(e.Image) > 0, ContentType: e.ContentType, Source: e.Source, Version: e.Version,
	}
	if e.Data != nil {
		b, err := json.Marshal(e.Data)
		if err != nil {
			return fmt.Errorf("encode cached data: %w", err)
		}
		m.Data = b
	}

	// Write the potentially large image first. Metadata is renamed last, so a
	// completed metadata file always points at a completed image file.
	if len(e.Image) > 0 {
		writeImage := true
		if old, err := s.readMetadata(base + ".json"); err == nil && old.Version != "" && old.Version == e.Version {
			if _, err := os.Stat(base + ".img"); err == nil {
				writeImage = false
			}
		}
		if writeImage {
			if err := atomicWrite(base+".img", e.Image); err != nil {
				return fmt.Errorf("write cached image: %w", err)
			}
		}
	} else {
		_ = os.Remove(base + ".img")
	}

	metaBytes, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encode cache metadata: %w", err)
	}
	if err := atomicWrite(base+".json", metaBytes); err != nil {
		return fmt.Errorf("write cache metadata: %w", err)
	}
	return nil
}

func (s *Store) readMetadata(path string) (metadata, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return metadata{}, err
	}
	var m metadata
	if err := json.Unmarshal(b, &m); err != nil {
		return metadata{}, err
	}
	return m, nil
}

func (s *Store) base(region, widgetID string) string {
	sum := sha256.Sum256([]byte(region + "\x00" + widgetID))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:16]))
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
