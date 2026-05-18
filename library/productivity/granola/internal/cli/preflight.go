// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

// PreflightDefaultRoot is the directory granola.py expects.
const PreflightDefaultRoot = "Documents/Dev/meeting-transcripts"

func newPreflightCmd(flags *rootFlags) *cobra.Command {
	var root string
	cmd := &cobra.Command{
		Use:   "preflight <id>",
		Short: "Validate a meeting is ready for extract (transcript + no duplicate)",
		Long: `Returns exit 0 when the meeting has at least 5 microphone-source and 5
system-source transcript segments AND no full_<id>.md or fuzzy
title-match file exists in --root. Exit 2 = transcript missing, exit 3 =
duplicate found.`,
		Example: `  # Pass-fail gate before running extract
  granola-pp-cli preflight ff1186df-593b-4ce5-bb1d-70e265f4a811

  # Use a non-default duplicate-scan root
  granola-pp-cli preflight ff1186df-593b-4ce5-bb1d-70e265f4a811 --root ~/work/meeting-notes

  # JSON for scripted pipelines
  granola-pp-cli preflight ff1186df-593b-4ce5-bb1d-70e265f4a811 --json`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			if root == "" {
				home, _ := os.UserHomeDir()
				root = filepath.Join(home, PreflightDefaultRoot)
			}
			c, err := openGranolaCache()
			if err != nil {
				return err
			}
			d := c.DocumentByID(id)
			if d == nil {
				return notFoundErr(fmt.Errorf("meeting %s not in cache", id))
			}
			segs := c.TranscriptByID(id)
			mic, sys := countSources(segs)
			transcriptOK := mic >= 5 && sys >= 5
			dupOf := findDuplicateFile(root, id, d.Title)

			res := map[string]any{
				"id":    id,
				"title": d.Title,
				"transcript": map[string]any{
					"present":      transcriptOK,
					"system_count": sys,
					"mic_count":    mic,
				},
				"duplicate_of": dupOf,
				"ok":           transcriptOK && dupOf == "",
			}
			_ = emitJSON(cmd, flags, res)
			if !transcriptOK {
				return &cliError{code: 2, err: fmt.Errorf("transcript missing or below 5/5 threshold for %s", id)}
			}
			if dupOf != "" {
				return &cliError{code: 3, err: fmt.Errorf("duplicate file %s already covers %s", dupOf, id)}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "Root directory to scan for duplicate files (default: ~/Documents/Dev/meeting-transcripts)")
	return cmd
}

func countSources(segs []granola.TranscriptSegment) (mic, sys int) {
	for _, s := range segs {
		switch strings.ToLower(s.Source) {
		case "microphone", "mic":
			mic++
		case "system", "speakers":
			sys++
		}
	}
	return
}

// findDuplicateFile checks --root for a file containing the id, OR a
// fuzzy title match (lowercased title substring).
func findDuplicateFile(root, id, title string) string {
	if root == "" {
		return ""
	}
	if _, err := os.Stat(root); err != nil {
		return ""
	}
	titleKey := strings.ToLower(strings.TrimSpace(title))
	var hit string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || hit != "" {
			return nil
		}
		name := info.Name()
		if strings.Contains(name, id) {
			hit = path
			return filepath.SkipDir
		}
		if titleKey != "" && strings.HasSuffix(name, ".md") {
			base := strings.ToLower(strings.TrimSuffix(name, ".md"))
			// Normalize both sides so underscores in filenames match spaces in titles.
			baseNorm := strings.ReplaceAll(base, "_", " ")
			if strings.Contains(baseNorm, titleKey) {
				hit = path
				return filepath.SkipDir
			}
		}
		return nil
	})
	return hit
}
