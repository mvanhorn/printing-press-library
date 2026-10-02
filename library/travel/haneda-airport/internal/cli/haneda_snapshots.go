// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/internal/haneda"
)

func hanedaSnapshotDir() (string, error) {
	p, err := cliutil.CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(p, "haneda-snapshots"), nil
}

func hanedaLatestPaths() ([]string, error) {
	root, err := hanedaSnapshotDir()
	if err != nil {
		return nil, err
	}
	// #nosec G304 -- root is the configured local CLI cache; the listing is capped and accepts only regular snapshot files.
	dir, err := os.Open(root)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(1001)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(entries) > 1000 {
		return nil, fmt.Errorf("default snapshot directory exceeds 1000 entries; choose an explicit --file or --before/--after path")
	}
	paths := []string{}
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasPrefix(e.Name(), "hnd-snapshot-") && strings.HasSuffix(e.Name(), ".json") {
			paths = append(paths, filepath.Join(root, e.Name()))
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	return paths, nil
}

// Find a compatible pair by validated content, including origin. A newest
// unpaired scope does not hide an older usable pair; byte work stays bounded.
func hanedaLatestCompatiblePair(paths []string) (before, after string, notes []string, err error) {
	type observation struct {
		path  string
		at    time.Time
		index int
	}
	seen := map[haneda.Coverage]observation{}
	notes = []string{}
	stopped := func(reason error) (string, string, []string, error) {
		if before == "" {
			return "", "", notes, reason
		}
		return before, after, []string{"Automatic cache selection stopped: " + reason.Error() + ". Comparing the newest compatible pair found in the examined files; a newer pair may remain unexamined. Choose explicit --before and --after to select a particular pair."}, nil
	}
	var bytes int64
	bestIndex := len(paths)
	for index, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return stopped(fmt.Errorf("inspect older cached snapshot %s: %w", filepath.Base(path), err))
		}
		bytes += info.Size()
		if bytes > 64<<20 {
			return stopped(fmt.Errorf("automatic snapshot pairing exceeds 64 MiB"))
		}
		s, err := haneda.LoadSnapshot(path)
		if err != nil {
			return stopped(fmt.Errorf("read older cached snapshot %s: %w", filepath.Base(path), err))
		}
		key := s.Board.Coverage
		key.QueryMode = "board" // Empty v1 mode and explicit board mode are compatible.
		at, _ := time.Parse(time.RFC3339, s.Board.ObservedAt)
		if previous, ok := seen[key]; ok {
			if previous.index < bestIndex {
				before, after = path, previous.path
				if at.After(previous.at) {
					before, after = previous.path, path
				}
				bestIndex = previous.index
			}
			if bestIndex == 0 {
				return before, after, notes, nil
			}
			continue
		}
		seen[key] = observation{path: path, at: at, index: index}
	}
	return before, after, notes, nil
}
