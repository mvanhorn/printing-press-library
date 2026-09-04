// Package snapshot persists small per-tracker sets of catalog object ids so the
// new-since command can report what the public catalog added since the last
// run. It is deliberately file-based (one JSON file per tracker under the user
// config dir): the Creative Fabrica catalog has 20M+ items and cannot be synced
// in bulk, so the only local state worth keeping is the id set per tracked
// query/designer.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"syscall"
	"time"
)

// Snapshot is the persisted state for one tracker key.
type Snapshot struct {
	Key       string   `json:"key"`
	UpdatedAt int64    `json:"updated_at"`
	ObjectIDs []string `json:"object_ids"`
}

// Store is a directory of snapshot JSON files.
type Store struct{ dir string }

// DefaultDir returns the standard snapshot directory under the user config dir.
func DefaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "creativefabrica-pp-cli", "snapshots")
}

// Open returns a store rooted at dir (DefaultDir if empty).
func Open(dir string) *Store {
	if dir == "" {
		dir = DefaultDir()
	}
	return &Store{dir: dir}
}

func (s *Store) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])[:32]+".json")
}

// Get returns the stored snapshot for key, or ok=false if none exists.
func (s *Store) Get(key string) (Snapshot, bool) {
	data, err := os.ReadFile(s.path(key))
	if err != nil {
		return Snapshot{}, false
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return Snapshot{}, false
	}
	return snap, true
}

// Put writes the snapshot for key with the given object ids. The write is
// tmp+rename in the store directory so an interrupted or concurrent Put cannot
// leave truncated JSON at the final path.
func (s *Store) Put(key string, ids []string) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	data, err := json.MarshalIndent(Snapshot{Key: key, UpdatedAt: time.Now().Unix(), ObjectIDs: sorted}, "", "  ")
	if err != nil {
		return err
	}
	dest := s.path(key)
	tmp, err := os.CreateTemp(s.dir, ".snap-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return err
	}
	ok = true
	return syncDir(s.dir)
}

func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		if runtime.GOOS == "windows" || errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTSUP) {
			return nil
		}
		return err
	}
	return nil
}

// Diff returns the ids present in current but not in prior.
func Diff(prior []string, current []string) []string {
	seen := make(map[string]bool, len(prior))
	for _, id := range prior {
		seen[id] = true
	}
	var added []string
	for _, id := range current {
		if !seen[id] {
			added = append(added, id)
		}
	}
	return added
}
