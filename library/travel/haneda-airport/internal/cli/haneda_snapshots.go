// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/internal/cliutil"
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
