// Copyright 2026 HenryBranchAdams and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/namethatui"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local
// Impact compares only locally synced mirrors and their immutable snapshots.
func newNovelImpactCmd(flags *rootFlags) *cobra.Command {
	var flagSince, dbPath string
	cmd := &cobra.Command{
		Use:         "impact <path>",
		Short:       "Show which project files may be affected by component or style guidance changes since a prior snapshot.",
		Example:     "name-that-ui-pp-cli impact . --since 2026-07-13 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				path, mirror := "", componentDBPath(dbPath)
				if len(args) > 0 {
					path = args[0]
				}
				return componentPrint(cmd, flags, impactResponse{Path: path, DBPath: mirror, Since: "", DryRun: true, Walked: false, SQLiteOpened: false, Changes: []impactChange{}, Files: []impactFile{}, Provenance: impactProvenance{DataSource: "local"}})
			}
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return usageErr(fmt.Errorf("%s requires exactly one <path>", cmd.CommandPath()))
			}
			if strings.TrimSpace(flagSince) == "" {
				return usageErr(fmt.Errorf("%s requires --since <duration-or-YYYY-MM-DD>", cmd.CommandPath()))
			}
			since, err := impactSince(flagSince, time.Now().UTC())
			if err != nil {
				return usageErr(err)
			}
			return runImpact(cmd, flags, args[0], componentDBPath(dbPath), since)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Baseline duration (for example 7d) or UTC date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

type impactResponse struct {
	Path         string           `json:"path"`
	DBPath       string           `json:"db_path"`
	DataSource   string           `json:"data_source,omitempty"`
	Since        string           `json:"since"`
	Reason       string           `json:"reason,omitempty"`
	DryRun       bool             `json:"dry_run,omitempty"`
	Walked       bool             `json:"walked"`
	SQLiteOpened bool             `json:"sqlite_opened"`
	Changes      []impactChange   `json:"changes"`
	Files        []impactFile     `json:"files"`
	Provenance   impactProvenance `json:"provenance"`
}

type impactProvenance struct {
	DataSource string `json:"data_source"`
	Baseline   string `json:"baseline,omitempty"`
	Current    string `json:"current,omitempty"`
}

type impactChange struct {
	EntityType         string         `json:"entity_type"`
	EntityID           string         `json:"entity_id"`
	Status             string         `json:"status"`
	SourceURL          string         `json:"source_url"`
	BaselineSnapshotAt string         `json:"baseline_snapshot_at,omitempty"`
	CurrentSnapshotAt  string         `json:"current_snapshot_at,omitempty"`
	Evidence           impactEvidence `json:"evidence"`
}

type impactEvidence struct {
	BaselineContentHash string   `json:"baseline_content_hash,omitempty"`
	CurrentContentHash  string   `json:"current_content_hash,omitempty"`
	Terms               []string `json:"terms"`
}

type impactFile struct {
	Path    string        `json:"path"`
	Matches []impactMatch `json:"matches"`
}

type impactMatch struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Symbol     string `json:"symbol"`
	Kind       string `json:"kind"`
	SourceURL  string `json:"source_url"`
	Line       int    `json:"line"`
}

type impactSnapshot struct {
	EntityID    string          `json:"entity_id"`
	SnapshotAt  string          `json:"snapshot_at"`
	ContentHash string          `json:"content_hash"`
	Data        json.RawMessage `json:"data"`
	SourceURL   string          `json:"source_url"`
}

type impactEntity struct {
	Type      string
	ID        string
	Data      json.RawMessage
	SourceURL string
}

func impactSince(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if duration, err := time.ParseDuration(value); err == nil {
		if duration <= 0 {
			return time.Time{}, fmt.Errorf("--since duration must be positive")
		}
		return now.UTC().Add(-duration), nil
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err == nil && days > 0 {
			return now.UTC().Add(-time.Duration(days) * 24 * time.Hour), nil
		}
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("--since must be a positive duration or YYYY-MM-DD")
	}
	return date.UTC(), nil
}

func runImpact(cmd *cobra.Command, flags *rootFlags, root, path string, since time.Time) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("NameThatUI snapshot mirror is unavailable; run 'name-that-ui-pp-cli sync --resources catalog,styles' first: %w", err)
	}
	db, err := store.OpenReadOnlyContext(cmd.Context(), path)
	if err != nil {
		return fmt.Errorf("opening NameThatUI snapshot mirror; run 'name-that-ui-pp-cli sync --resources catalog,styles' first: %w", err)
	}
	defer db.Close()
	snapshots, err := loadImpactSnapshots(db)
	if err != nil {
		return err
	}
	if len(snapshots["component"]) == 0 && len(snapshots["style"]) == 0 {
		return fmt.Errorf("NameThatUI snapshots are unavailable; run 'name-that-ui-pp-cli sync --resources catalog,styles' first")
	}
	current, err := loadImpactCurrent(db)
	if err != nil {
		return err
	}
	if len(current) == 0 {
		return fmt.Errorf("NameThatUI current mirror is unavailable; run 'name-that-ui-pp-cli sync --resources catalog,styles' first")
	}
	if err := validateImpactCurrentSnapshots(snapshots, current); err != nil {
		return err
	}
	changes, baselineFound := impactChanges(snapshots, current, since)
	response := impactResponse{Path: root, DBPath: path, DataSource: "local", Since: since.UTC().Format(time.RFC3339), Walked: false, SQLiteOpened: true, Changes: changes, Files: []impactFile{}, Provenance: impactProvenance{DataSource: "local", Baseline: since.UTC().Format(time.RFC3339), Current: "current local mirror"}}
	if !baselineFound {
		response.Reason = "no snapshot exists at or before --since; no changes were inferred"
		return componentPrint(cmd, flags, response)
	}
	terms := impactTerms(changes, snapshots, current, since)
	files, err := impactFiles(root, terms)
	if err != nil {
		return err
	}
	response.Walked, response.Files = true, files
	return componentPrint(cmd, flags, response)
}

func loadImpactSnapshots(db *store.Store) (map[string]map[string][]impactSnapshot, error) {
	out := map[string]map[string][]impactSnapshot{"component": {}, "style": {}}
	for _, spec := range []struct{ kind, resource string }{{"component", "component_snapshots"}, {"style", "style_snapshots"}} {
		raw, err := db.List(spec.resource, 0)
		if err != nil {
			return nil, err
		}
		for _, row := range raw {
			var snapshot impactSnapshot
			if json.Unmarshal(row, &snapshot) != nil || snapshot.EntityID == "" || snapshot.SnapshotAt == "" || snapshot.ContentHash == "" {
				continue
			}
			out[spec.kind][snapshot.EntityID] = append(out[spec.kind][snapshot.EntityID], snapshot)
		}
		for _, rows := range out[spec.kind] {
			sort.Slice(rows, func(i, j int) bool { return impactSnapshotTime(rows[i]).Before(impactSnapshotTime(rows[j])) })
		}
	}
	return out, nil
}

func loadImpactCurrent(db *store.Store) (map[string]impactEntity, error) {
	out := map[string]impactEntity{}
	for _, spec := range []struct{ kind, resource string }{{"component", "components"}, {"style", "style_details"}} {
		raw, err := db.List(spec.resource, 0)
		if err != nil {
			return nil, err
		}
		for _, row := range raw {
			var identity struct {
				ID        string `json:"id"`
				SourceURL string `json:"source_url"`
			}
			if json.Unmarshal(row, &identity) == nil && identity.ID != "" {
				out[spec.kind+":"+identity.ID] = impactEntity{Type: spec.kind, ID: identity.ID, Data: row, SourceURL: identity.SourceURL}
			}
		}
	}
	return out, nil
}

func validateImpactCurrentSnapshots(snapshots map[string]map[string][]impactSnapshot, current map[string]impactEntity) error {
	for _, entity := range current {
		if latestSnapshotForHash(snapshots[entity.Type][entity.ID], impactHash(entity.Data)) == nil {
			return fmt.Errorf("NameThatUI %s snapshot is unavailable for %s; run 'name-that-ui-pp-cli sync --resources catalog,styles' first", entity.Type, entity.ID)
		}
	}
	return nil
}

func impactChanges(snapshots map[string]map[string][]impactSnapshot, current map[string]impactEntity, since time.Time) ([]impactChange, bool) {
	changes := []impactChange{}
	baselineFound := false
	for _, kind := range []string{"component", "style"} {
		for _, rows := range snapshots[kind] {
			if latestSnapshotAtOrBefore(rows, since) != nil {
				baselineFound = true
				break
			}
		}
	}
	if !baselineFound {
		return changes, false
	}
	for _, kind := range []string{"component", "style"} {
		ids := map[string]bool{}
		for id := range snapshots[kind] {
			ids[id] = true
		}
		for key, entity := range current {
			if entity.Type == kind {
				ids[strings.TrimPrefix(key, kind+":")] = true
			}
		}
		ordered := make([]string, 0, len(ids))
		for id := range ids {
			ordered = append(ordered, id)
		}
		sort.Strings(ordered)
		for _, id := range ordered {
			baseline := latestSnapshotAtOrBefore(snapshots[kind][id], since)
			entity, exists := current[kind+":"+id]
			if baseline == nil {
				if exists && len(snapshots[kind][id]) > 0 {
					currentHash := impactHash(entity.Data)
					latest := latestSnapshotForHash(snapshots[kind][id], currentHash)
					currentAt := ""
					if latest != nil {
						currentAt = latest.SnapshotAt
					}
					changes = append(changes, impactChange{EntityType: kind, EntityID: id, Status: "added", SourceURL: entity.SourceURL, CurrentSnapshotAt: currentAt, Evidence: impactEvidence{CurrentContentHash: currentHash, Terms: []string{}}})
				}
				continue
			}
			if !exists {
				changes = append(changes, impactChange{EntityType: kind, EntityID: id, Status: "removed", SourceURL: baseline.SourceURL, BaselineSnapshotAt: baseline.SnapshotAt, Evidence: impactEvidence{BaselineContentHash: baseline.ContentHash, Terms: []string{}}})
				continue
			}
			currentHash := impactHash(entity.Data)
			latest := latestSnapshotForHash(snapshots[kind][id], currentHash)
			currentAt := ""
			if latest != nil {
				currentAt = latest.SnapshotAt
			}
			if baseline.ContentHash != currentHash {
				changes = append(changes, impactChange{EntityType: kind, EntityID: id, Status: "changed", SourceURL: entity.SourceURL, BaselineSnapshotAt: baseline.SnapshotAt, CurrentSnapshotAt: currentAt, Evidence: impactEvidence{BaselineContentHash: baseline.ContentHash, CurrentContentHash: currentHash, Terms: []string{}}})
			}
		}
	}
	return changes, baselineFound
}

func latestSnapshotAtOrBefore(rows []impactSnapshot, since time.Time) *impactSnapshot {
	var latest *impactSnapshot
	for i := range rows {
		at := impactSnapshotTime(rows[i])
		if !at.IsZero() && !at.After(since) && (latest == nil || at.After(impactSnapshotTime(*latest))) {
			latest = &rows[i]
		}
	}
	return latest
}

func latestSnapshotForHash(rows []impactSnapshot, hash string) *impactSnapshot {
	var latest *impactSnapshot
	for i := range rows {
		if rows[i].ContentHash == hash && (latest == nil || impactSnapshotTime(rows[i]).After(impactSnapshotTime(*latest))) {
			latest = &rows[i]
		}
	}
	return latest
}

func impactSnapshotTime(snapshot impactSnapshot) time.Time {
	at, err := time.Parse(time.RFC3339Nano, snapshot.SnapshotAt)
	if err != nil {
		return time.Time{}
	}
	return at
}

func impactHash(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

type impactTerm struct{ EntityType, EntityID, Phrase, Kind, SourceURL string }

func impactTerms(changes []impactChange, snapshots map[string]map[string][]impactSnapshot, current map[string]impactEntity, since time.Time) []impactTerm {
	terms := []impactTerm{}
	for i := range changes {
		change := &changes[i]
		var records []json.RawMessage
		if entity, ok := current[change.EntityType+":"+change.EntityID]; ok {
			records = append(records, entity.Data)
		}
		if baseline := latestSnapshotAtOrBefore(snapshots[change.EntityType][change.EntityID], since); baseline != nil {
			records = append(records, baseline.Data)
		}
		seen := map[string]bool{}
		for _, record := range records {
			if change.EntityType == "component" {
				var component namethatui.Component
				if json.Unmarshal(record, &component) == nil {
					for _, term := range componentTerms([]namethatui.Component{component}, false, true) {
						key := term.Kind + ":" + term.Phrase
						if !seen[key] {
							terms = append(terms, impactTerm{EntityType: change.EntityType, EntityID: change.EntityID, Phrase: term.Phrase, Kind: term.Kind, SourceURL: change.SourceURL})
							seen[key] = true
						}
					}
				}
			} else {
				var style namethatui.Style
				if json.Unmarshal(record, &style) == nil {
					for _, phrase := range []string{style.ID, style.Slug, style.Name} {
						if meaningfulPhrase(phrase) && !seen["style:"+phrase] {
							terms = append(terms, impactTerm{EntityType: change.EntityType, EntityID: change.EntityID, Phrase: phrase, Kind: "style", SourceURL: change.SourceURL})
							seen["style:"+phrase] = true
						}
					}
				}
			}
		}
		change.Evidence.Terms = []string{}
		for _, term := range terms {
			if term.EntityType == change.EntityType && term.EntityID == change.EntityID {
				change.Evidence.Terms = append(change.Evidence.Terms, term.Phrase)
			}
		}
		sort.Strings(change.Evidence.Terms)
	}
	sort.Slice(terms, func(i, j int) bool {
		if terms[i].EntityType == terms[j].EntityType {
			if terms[i].EntityID == terms[j].EntityID {
				return terms[i].Phrase < terms[j].Phrase
			}
			return terms[i].EntityID < terms[j].EntityID
		}
		return terms[i].EntityType < terms[j].EntityType
	})
	return terms
}

func impactFiles(root string, terms []impactTerm) ([]impactFile, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", root, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return []impactFile{}, fmt.Errorf("%s is a symlink; impact does not follow symlinks", root)
	}
	if !info.IsDir() && !inventoryExtensions[strings.ToLower(filepath.Ext(root))] {
		return []impactFile{}, fmt.Errorf("%s is not an allowed source or text file", root)
	}
	files := []impactFile{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if path != root && inventorySkippedDirectories[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || !inventoryExtensions[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if fileInfo.Size() > novelMaxFileBytes {
			return fmt.Errorf("%s exceeds the 2 MiB limit", path)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !utf8.Valid(body) || containsNUL(body) {
			return fmt.Errorf("%s is binary; impact accepts text only", path)
		}
		matches := impactMatches(body, terms)
		if len(matches) > 0 {
			files = append(files, impactFile{Path: novelRelativePath(root, path), Matches: matches})
		}
		return nil
	})
	if err != nil {
		return []impactFile{}, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func impactMatches(body []byte, terms []impactTerm) []impactMatch {
	grouped := map[string]impactMatch{}
	text := string(body)
	for _, term := range terms {
		caseSensitive := term.Kind == "api" || term.Kind == "part_api"
		for _, offset := range boundedPhraseOffsets(text, term.Phrase, caseSensitive) {
			line, _ := offsetLineColumn(body, offset)
			key := fmt.Sprintf("%s:%s:%d:%s", term.EntityType, term.EntityID, line, term.Phrase)
			grouped[key] = impactMatch{EntityType: term.EntityType, EntityID: term.EntityID, Symbol: term.Phrase, Kind: term.Kind, SourceURL: term.SourceURL, Line: line}
		}
	}
	matches := make([]impactMatch, 0, len(grouped))
	for _, match := range grouped {
		matches = append(matches, match)
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Line == matches[j].Line {
			if matches[i].EntityType == matches[j].EntityType {
				return matches[i].Symbol < matches[j].Symbol
			}
			return matches[i].EntityType < matches[j].EntityType
		}
		return matches[i].Line < matches[j].Line
	})
	return matches
}
