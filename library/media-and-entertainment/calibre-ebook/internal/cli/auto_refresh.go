package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/calibre-ebook/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/calibre-ebook/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/calibre-ebook/internal/store"
)

func autoRefreshIfStale(flags *rootFlags, resourceTypes []string) error {
	if flags.noCache {
		return nil
	}
	dbPath := defaultDBPath("calibre-ebook-pp-cli")
	if _, err := os.Stat(dbPath); err != nil {
		return nil
	}
	s, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return nil
	}
	defer s.Close()

	for _, rt := range resourceTypes {
		ts := s.GetLastSyncedAt(rt)
		if ts == "" {
			continue
		}
		lastSynced, parseErr := time.Parse(time.RFC3339, ts)
		if parseErr != nil {
			continue
		}
		if time.Since(lastSynced) > 6*time.Hour {
			fmt.Fprintf(os.Stderr, "  auto-refresh: %s is stale (%s old)\n", rt, time.Since(lastSynced).Round(time.Minute))
			if freshErr := cliutil.EnsureFresh(dbPath, 6*time.Hour); freshErr != nil {
				fmt.Fprintf(os.Stderr, "  auto-refresh: triggering live query\n")
				cfg, _ := config.Load(flags.configPath)
				if flags.libraryPath != "" && cfg != nil {
					cfg.LibraryPath = flags.libraryPath
				}
				c, clientErr := flags.newClient()
				if clientErr == nil {
					if _, _, runErr := c.RunCalibredb("list", "--for-machine", "--limit", "1"); runErr == nil {
						fmt.Fprintf(os.Stderr, "  auto-refresh: done\n")
					} else {
						fmt.Fprintf(os.Stderr, "  auto-refresh failed: %v\n", runErr)
					}
				}
			}
			return nil
		}
	}
	return nil
}
