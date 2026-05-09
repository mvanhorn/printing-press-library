package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface-pp-cli/internal/hfx"
)

type localResponse struct {
	hfx.Envelope
	CacheDirs []string      `json:"cache_dirs"`
	Models    []localModel  `json:"models"`
	Total     int           `json:"total"`
	Explain   string        `json:"explain,omitempty"`
}

type localModel struct {
	ID            string   `json:"id"`
	Org           string   `json:"org"`
	Name          string   `json:"name"`
	CacheDir      string   `json:"cache_dir"`
	SizeBytes     int64    `json:"size_bytes"`
	SizeGB        string   `json:"size_gb"`
	Snapshots     []string `json:"snapshots,omitempty"`
	LastModified  string   `json:"last_modified"`
	HasGGUF       bool     `json:"has_gguf,omitempty"`
}

func newHFLocalCmd(flags *rootFlags) *cobra.Command {
	var cacheDirsFlag string
	cmd := &cobra.Command{
		Use:   "local",
		Short: "List models already on disk (HF hub cache + custom dirs).",
		Long: `local walks ~/.cache/huggingface/hub/models--<org>--<name>/snapshots/<sha>/
plus any --cache-dirs you pass and maps each to its HF id. Returns size,
snapshot list, and last-modified time. Stops accidental re-downloads.

Org names with "-" in them are recovered correctly: the splitter splits on
the LAST "--" in the directory name (the canonical hub layout convention).`,
		Example: `  huggingface-pp-cli local
  huggingface-pp-cli local --json
  huggingface-pp-cli local --cache-dirs ~/.cache/huggingface/hub,/Volumes/models`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			dirs := defaultCacheDirs()
			if cacheDirsFlag != "" {
				dirs = nil
				for _, d := range strings.Split(cacheDirsFlag, ",") {
					if t := strings.TrimSpace(d); t != "" {
						dirs = append(dirs, expandHome(t))
					}
				}
			}

			seen := map[string]*localModel{}
			for _, dir := range dirs {
				walkLocalCache(dir, seen)
			}

			models := make([]localModel, 0, len(seen))
			for _, m := range seen {
				models = append(models, *m)
			}
			sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
			if flags.limit > 0 && len(models) > flags.limit {
				models = models[:flags.limit]
			}

			resp := localResponse{
				Envelope:  hfx.NewEnvelope("local"),
				CacheDirs: dirs,
				Models:    models,
				Total:     len(seen),
			}
			if flags.explain {
				resp.Explain = fmt.Sprintf("explain: scanned %d cache dirs, found %d unique models. Pass --cache-dirs to add more roots; results capped at --limit (%d).",
					len(dirs), len(seen), flags.limit)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "local: %d models in %d cache dirs\n\n", len(seen), len(dirs))
			fmt.Fprintf(cmd.OutOrStdout(), "  %-50s  %-8s  %-10s  %s\n", "ID", "SIZE", "GGUF", "LAST_MODIFIED")
			for _, m := range models {
				gg := " "
				if m.HasGGUF {
					gg = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-50s  %-8s  %-10s  %s\n", m.ID, m.SizeGB, gg, m.LastModified)
			}
			if resp.Explain != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", resp.Explain)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cacheDirsFlag, "cache-dirs", "", "Comma-separated cache dirs to walk (default: ~/.cache/huggingface/hub)")
	return cmd
}

func defaultCacheDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{filepath.Join(home, ".cache", "huggingface", "hub")}
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// walkLocalCache enumerates models--<org>--<name>/ entries inside dir.
// Recovers org/name by splitting on the LAST "--" (canonical hub layout —
// org names may contain "-" but never "--", per HF naming rules). Sums
// sibling sizes via the blobs/ subdirectory and dereferences snapshots/<sha>/
// symlinks to count real size.
func walkLocalCache(dir string, out map[string]*localModel) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "models--") {
			continue
		}
		stripped := strings.TrimPrefix(name, "models--")
		// Split on the LAST "--" (org names may include "-" but never "--").
		idx := strings.LastIndex(stripped, "--")
		if idx < 0 {
			continue
		}
		org := stripped[:idx]
		modelName := stripped[idx+2:]
		id := org + "/" + modelName

		modelDir := filepath.Join(dir, name)
		size := dirSize(filepath.Join(modelDir, "blobs"))
		mod := dirNewest(modelDir)
		hasGGUF := dirHasGGUF(filepath.Join(modelDir, "snapshots"))

		// Snapshot SHAs
		var shas []string
		if snaps, err := os.ReadDir(filepath.Join(modelDir, "snapshots")); err == nil {
			for _, s := range snaps {
				if s.IsDir() {
					shas = append(shas, s.Name())
				}
			}
		}

		out[id] = &localModel{
			ID:           id,
			Org:          org,
			Name:         modelName,
			CacheDir:     modelDir,
			SizeBytes:    size,
			SizeGB:       hfHumanGB(size),
			Snapshots:    shas,
			LastModified: mod.UTC().Format(time.RFC3339),
			HasGGUF:      hasGGUF,
		}
	}
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func dirNewest(dir string) time.Time {
	var newest time.Time
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			if info.ModTime().After(newest) {
				newest = info.ModTime()
			}
		}
		return nil
	})
	return newest
}

func dirHasGGUF(snapshotsDir string) bool {
	found := false
	_ = filepath.WalkDir(snapshotsDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if hfx.IsGGUF(p) {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
