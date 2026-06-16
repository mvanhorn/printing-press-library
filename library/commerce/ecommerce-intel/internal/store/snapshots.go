package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/internal/intelcli"
)

const SnapshotSchemaVersion = "ecommerce-intel.snapshot/v1"

type Snapshot struct {
	SchemaVersion         string            `json:"schema_version"`
	Profile               string            `json:"profile"`
	SnapshotDate          string            `json:"snapshot_date"`
	CapturedAt            time.Time         `json:"captured_at"`
	Source                string            `json:"source"`
	DateRange             DateRange         `json:"date_range"`
	SourceCommandVersions map[string]string `json:"source_command_versions,omitempty"`
	InputHashes           map[string]string `json:"input_hashes,omitempty"`
	Data                  DataSet           `json:"data"`
}

type SnapshotRef struct {
	Path       string
	Name       string
	CapturedAt time.Time
	Snapshot   Snapshot
}

func (s *Store) snapshotDir(profile string) string {
	return filepath.Join(s.Dir, "snapshots", safe(profile))
}

func (s *Store) LearningsPath(profile string) string {
	return filepath.Join(s.Dir, "learnings", safe(profile)+".md")
}

func (s *Store) EnsureLearnings(profile string) error {
	if strings.TrimSpace(profile) == "" {
		profile = "default"
	}
	path := s.LearningsPath(profile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	body := fmt.Sprintf("# Learnings for %s\n\nAppend-only notes from movers and outcome checks.\n", profile)
	return os.WriteFile(path, []byte(body), 0o644)
}

func (s *Store) AppendLearning(profile, body string) error {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	if err := s.EnsureLearnings(profile); err != nil {
		return err
	}
	f, err := os.OpenFile(s.LearningsPath(profile), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n## %s\n\n%s\n", time.Now().UTC().Format(time.RFC3339), strings.TrimSpace(body))
	return err
}

func (s *Store) SaveSnapshot(d DataSet) error {
	if strings.TrimSpace(d.Profile) == "" {
		d.Profile = "default"
	}
	if d.SyncedAt.IsZero() {
		d.SyncedAt = time.Now().UTC()
	}
	dir := s.snapshotDir(d.Profile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	snap := Snapshot{
		SchemaVersion:         SnapshotSchemaVersion,
		Profile:               d.Profile,
		SnapshotDate:          d.SyncedAt.UTC().Format("2006-01-02"),
		CapturedAt:            d.SyncedAt.UTC(),
		Source:                d.Source,
		DateRange:             d.Provenance.DateRange,
		SourceCommandVersions: intelcli.CloneStringMap(d.Provenance.SourceCommandVersions),
		InputHashes:           intelcli.CloneStringMap(d.Provenance.InputHashes),
		Data:                  d,
	}
	path := intelcli.UniqueSnapshotPath(dir, snap.SnapshotDate)
	if err := writeJSON(path, snap); err != nil {
		return err
	}
	return s.CompactSnapshots(d.Profile, time.Now().UTC())
}

func (s *Store) ListSnapshots(profile string) ([]SnapshotRef, error) {
	dir := s.snapshotDir(profile)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	refs := []SnapshotRef{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		var snap Snapshot
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &snap); err != nil {
			return nil, err
		}
		captured := snap.CapturedAt
		if captured.IsZero() {
			if info, err := entry.Info(); err == nil {
				captured = info.ModTime().UTC()
			}
		}
		refs = append(refs, SnapshotRef{Path: path, Name: entry.Name(), CapturedAt: captured, Snapshot: snap})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].CapturedAt.Before(refs[j].CapturedAt) })
	return refs, nil
}

func (s *Store) LatestSnapshots(profile string, n int) ([]Snapshot, error) {
	refs, err := s.ListSnapshots(profile)
	if err != nil {
		return nil, err
	}
	if n <= 0 || n > len(refs) {
		n = len(refs)
	}
	out := make([]Snapshot, 0, n)
	for i := len(refs) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, refs[i].Snapshot)
	}
	return out, nil
}

func (s *Store) CompactSnapshots(profile string, now time.Time) error {
	refs, err := s.ListSnapshots(profile)
	if err != nil {
		return err
	}
	files := make([]intelcli.SnapshotFile, 0, len(refs))
	for _, ref := range refs {
		files = append(files, intelcli.SnapshotFile{Path: ref.Path, Name: ref.Name, CapturedAt: ref.CapturedAt})
	}
	return intelcli.CompactSnapshotFiles(files, now)
}
