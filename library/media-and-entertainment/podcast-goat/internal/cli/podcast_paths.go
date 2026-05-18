// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 podcast-goat config + cache paths and store opener.

package cli

import (
	"context"
	"os"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/store"
)

func podcastConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "podcast-goat")
}

func podcastCacheDir() string {
	return filepath.Join(podcastConfigDir(), "cache")
}

func podcastDBPath() string {
	return filepath.Join(podcastConfigDir(), "podcast-goat.db")
}

func podcastMagicDir() string {
	return filepath.Join(podcastConfigDir(), "magic")
}

// openPodcastStore opens the SQLite store and ensures v0.1 tables exist.
func openPodcastStore(ctx context.Context) (*store.PodcastStore, error) {
	dbPath := podcastDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	return store.NewPodcastStore(ctx, s)
}
