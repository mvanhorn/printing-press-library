package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/vault"
)

// SyncStats records what happened during a sync pass.
type SyncStats struct {
	NotesIndexed   int      `json:"notes_indexed"`
	NotesUpdated   int      `json:"notes_updated"`
	NotesUnchanged int      `json:"notes_unchanged"`
	NotesDeleted   int      `json:"notes_deleted"`
	FactsIndexed   int      `json:"facts_indexed"`
	LinksIndexed   int      `json:"links_indexed"`
	TagsIndexed    int      `json:"tags_indexed"`
	Errors         []string `json:"errors,omitempty"`
}

// Sync walks the vault and reconciles the store. When incremental is true,
// notes whose on-disk mtime matches the indexed mtime are skipped.
func Sync(ctx context.Context, st *Store, v *vault.Vault, incremental bool) (*SyncStats, error) {
	stats := &SyncStats{}
	// Track the set of paths seen during the walk so we can prune deletions.
	seen := map[string]bool{}

	// Build a map of indexed mtimes so we can short-circuit unchanged notes.
	existingMtimes := map[string]int64{}
	if incremental {
		rows, err := st.db.QueryContext(ctx, `SELECT path, mtime FROM notes`)
		if err != nil {
			return stats, fmt.Errorf("load existing: %w", err)
		}
		for rows.Next() {
			var p string
			var m int64
			if err := rows.Scan(&p, &m); err != nil {
				rows.Close()
				return stats, err
			}
			existingMtimes[p] = m
		}
		rows.Close()
	}

	// Resolution map for wikilinks: title-or-stem -> path.
	resolution, err := buildResolutionIndex(v)
	if err != nil {
		return stats, fmt.Errorf("build resolution index: %w", err)
	}

	if err := v.Walk(func(n *vault.Note) error {
		seen[n.Path] = true
		if n.FMError != "" {
			stats.Errors = append(stats.Errors, fmt.Sprintf("%s: %s (indexed without frontmatter)", n.Path, n.FMError))
		}
		info, err := os.Stat(n.AbsPath)
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("stat %s: %v", n.Path, err))
			return nil
		}
		mtime := info.ModTime().Unix()
		if incremental {
			if prev, ok := existingMtimes[n.Path]; ok && prev == mtime {
				stats.NotesUnchanged++
				return nil
			}
		}
		// Compute body hash.
		h := sha256.Sum256([]byte(n.Body))
		bodyHash := hex.EncodeToString(h[:])

		layer := vault.Layers[n.Frontmatter.Type]
		title := titleFromPath(n.Path)

		// Stash unknown frontmatter keys.
		extras := map[string]string{}
		for k, v := range n.Frontmatter.Extra {
			extras[k] = fmt.Sprintf("%v", v)
		}

		// Tags: frontmatter list + inline #tags.
		var tags []TagEntry
		for _, t := range n.Frontmatter.Tags {
			tags = append(tags, TagEntry{Tag: t, Source: "frontmatter"})
		}
		for _, t := range vault.ExtractInlineTags(n.Body) {
			tags = append(tags, TagEntry{Tag: t, Source: "inline"})
		}

		// Links.
		var links []LinkEntry
		for _, target := range vault.ExtractWikilinks(n.Body) {
			resolved := resolution[normalizeLinkKey(target)]
			links = append(links, LinkEntry{Target: target, ResolvedPath: resolved})
		}

		// Facts: inline + TOML sidecar.
		var facts []FactEntry
		for _, f := range n.Frontmatter.Facts {
			facts = append(facts, factEntry(f, "inline"))
		}
		if n.Frontmatter.FactsFile != "" {
			tomlFacts, err := v.LoadFactsTOML(n.AbsPath, n.Frontmatter.FactsFile)
			if err != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("facts toml %s: %v", n.Path, err))
			}
			for _, f := range tomlFacts {
				facts = append(facts, factEntry(f, "toml"))
			}
		}

		if err := st.UpsertNote(ctx, n, mtime, info.Size(), layer, bodyHash, extras, tags, links, facts, title); err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("upsert %s: %v", n.Path, err))
			return nil
		}
		stats.NotesIndexed++
		if _, ok := existingMtimes[n.Path]; ok {
			stats.NotesUpdated++
		}
		stats.FactsIndexed += len(facts)
		stats.LinksIndexed += len(links)
		stats.TagsIndexed += len(tags)
		return nil
	}); err != nil {
		return stats, err
	}

	// Prune deleted notes.
	indexed, err := st.AllPaths(ctx)
	if err != nil {
		return stats, err
	}
	for _, p := range indexed {
		if !seen[p] {
			if err := st.DeletePath(ctx, p); err != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("delete %s: %v", p, err))
				continue
			}
			stats.NotesDeleted++
		}
	}
	if err := st.SetMeta(ctx, "last_sync_unix", fmt.Sprintf("%d", nowUnix())); err != nil {
		return stats, err
	}
	if err := st.SetMeta(ctx, "vault_root", v.Root); err != nil {
		return stats, err
	}
	return stats, nil
}

func factEntry(f vault.Fact, storage string) FactEntry {
	return FactEntry{
		ID: f.ID, Fact: f.Fact, Category: f.Category, Timestamp: f.Timestamp,
		Status: f.Status, Source: f.Source, DecisionTraceID: f.DecisionTraceID,
		Storage: storage,
	}
}

// titleFromPath returns the title-cased basename (stem) of a vault-relative path.
func titleFromPath(p string) string {
	base := filepath.Base(p)
	return strings.TrimSuffix(base, ".md")
}

// buildResolutionIndex returns a map of normalized link keys -> vault-relative paths.
// Both filename-stem (e.g. "Jeff Smith") and full path (e.g. "People/Jeff Smith") resolve.
func buildResolutionIndex(v *vault.Vault) (map[string]string, error) {
	out := map[string]string{}
	err := v.Walk(func(n *vault.Note) error {
		stem := strings.TrimSuffix(filepath.Base(n.Path), ".md")
		out[normalizeLinkKey(stem)] = n.Path
		out[normalizeLinkKey(strings.TrimSuffix(n.Path, ".md"))] = n.Path
		return nil
	})
	return out, err
}

func normalizeLinkKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func nowUnix() int64 {
	return nowFunc()
}

// nowFunc is overridable in tests.
var nowFunc = func() int64 {
	return timeNow().Unix()
}
