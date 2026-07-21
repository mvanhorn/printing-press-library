// Copyright 2026 HenryBranchAdams and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/namethatui"
	"github.com/spf13/cobra"
)

// pp:data-source local
// Inventory deliberately scans local source and the synced component mirror only.
func newNovelInventoryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "inventory <path>",
		Short:       "Map UI API symbols in a source tree to canonical components and source references.",
		Example:     "name-that-ui-pp-cli inventory . --agent --select files.path,files.matches.symbol,files.matches.component,files.matches.source_url",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return usageErr(fmt.Errorf("%s requires exactly one <path>", cmd.CommandPath()))
			}
			root, mirror := args[0], componentDBPath(dbPath)
			if dryRunOK(flags) {
				return componentPrint(cmd, flags, inventoryResponse{Path: root, DBPath: mirror, DryRun: true, Walked: false, SQLiteOpened: false, Files: []inventoryFile{}, Totals: inventoryTotals{}})
			}
			items, meta, err := componentLoad(cmd, mirror)
			if err != nil {
				return err
			}
			files, totals, err := inventoryFiles(root, items)
			if err != nil {
				return err
			}
			return componentPrint(cmd, flags, inventoryResponse{Path: root, DBPath: mirror, DataSource: meta.DataSource, Walked: true, SQLiteOpened: true, Files: files, Totals: totals})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

type inventoryResponse struct {
	Path         string          `json:"path"`
	DBPath       string          `json:"db_path"`
	DataSource   string          `json:"data_source,omitempty"`
	DryRun       bool            `json:"dry_run,omitempty"`
	Walked       bool            `json:"walked"`
	SQLiteOpened bool            `json:"sqlite_opened"`
	Files        []inventoryFile `json:"files"`
	Totals       inventoryTotals `json:"totals"`
}

type inventoryFile struct {
	Path    string           `json:"path"`
	Matches []inventoryMatch `json:"matches"`
}

type inventoryMatch struct {
	Symbol              string               `json:"symbol"`
	Framework           string               `json:"framework"`
	Component           string               `json:"component"`
	Part                string               `json:"part,omitempty"`
	SourceURL           string               `json:"source_url"`
	Line                int                  `json:"line"`
	Ambiguous           bool                 `json:"ambiguous"`
	CanonicalCandidates []canonicalCandidate `json:"canonical_candidates"`
}

type inventoryTotals struct {
	FilesScanned     int `json:"files_scanned"`
	FilesWithMatches int `json:"files_with_matches"`
	Matches          int `json:"matches"`
}

var inventoryExtensions = map[string]bool{
	".swift": true, ".m": true, ".mm": true, ".h": true, ".ts": true, ".tsx": true,
	".js": true, ".jsx": true, ".html": true, ".vue": true, ".svelte": true, ".kt": true,
	".java": true, ".dart": true, ".rs": true, ".go": true, ".py": true, ".rb": true, ".cs": true,
}

var inventorySkippedDirectories = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "build": true, "dist": true, ".next": true,
}

func inventoryFiles(root string, items []namethatui.Component) ([]inventoryFile, inventoryTotals, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, inventoryTotals{}, fmt.Errorf("reading %s: %w", root, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return []inventoryFile{}, inventoryTotals{}, fmt.Errorf("%s is a symlink; inventory does not follow symlinks", root)
	}
	if !info.IsDir() && !inventoryExtensions[strings.ToLower(filepath.Ext(root))] {
		return []inventoryFile{}, inventoryTotals{}, fmt.Errorf("%s is not an allowed source or text file", root)
	}
	terms := componentTerms(items, false, true)
	files, totals := []inventoryFile{}, inventoryTotals{}
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
			return fmt.Errorf("%s is binary; inventory accepts text only", path)
		}
		totals.FilesScanned++
		matches := inventoryMatches(body, terms)
		files = append(files, inventoryFile{Path: novelRelativePath(root, path), Matches: matches})
		if len(matches) > 0 {
			totals.FilesWithMatches++
			totals.Matches += len(matches)
		}
		return nil
	})
	if err != nil {
		return []inventoryFile{}, inventoryTotals{}, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, totals, nil
}

func inventoryMatches(body []byte, terms []novelTerm) []inventoryMatch {
	grouped := map[string]*inventoryMatch{}
	text := string(body)
	for _, term := range terms {
		caseSensitive := term.Kind == "api" || term.Kind == "part_api"
		for _, at := range boundedPhraseOffsets(text, term.Phrase, caseSensitive) {
			line, _ := offsetLineColumn(body, at)
			key := fmt.Sprintf("%d:%s", line, strings.ToLower(term.Phrase))
			match := grouped[key]
			if match == nil {
				match = &inventoryMatch{Symbol: term.Phrase, Framework: term.Candidate.Framework, Component: term.Candidate.Component, Part: term.Candidate.Part, SourceURL: term.Candidate.SourceURL, Line: line, CanonicalCandidates: []canonicalCandidate{}}
				grouped[key] = match
			}
			if term.Kind == "api" || term.Kind == "part_api" {
				match.Framework = term.Candidate.Framework
			}
			appendInventoryCandidate(match, term.Candidate)
		}
	}
	matches := make([]inventoryMatch, 0, len(grouped))
	for _, match := range grouped {
		sort.Slice(match.CanonicalCandidates, func(i, j int) bool {
			left, right := match.CanonicalCandidates[i], match.CanonicalCandidates[j]
			if left.Component == right.Component {
				return left.Part < right.Part
			}
			return left.Component < right.Component
		})
		match.Ambiguous = len(match.CanonicalCandidates) > 1
		matches = append(matches, *match)
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Line == matches[j].Line {
			return matches[i].Symbol < matches[j].Symbol
		}
		return matches[i].Line < matches[j].Line
	})
	return matches
}

func appendInventoryCandidate(match *inventoryMatch, candidate canonicalCandidate) {
	for i, existing := range match.CanonicalCandidates {
		if existing.Component == candidate.Component && existing.Part == candidate.Part && existing.SourceURL == candidate.SourceURL {
			if existing.Symbol == "" && candidate.Symbol != "" {
				match.CanonicalCandidates[i] = candidate
			}
			return
		}
	}
	match.CanonicalCandidates = append(match.CanonicalCandidates, candidate)
}
