// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelEpisodeImportCmd(flags *rootFlags) *cobra.Command {
	var flagScribeFolder string
	var flagImagesFolder string
	var flagOut string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Pull a whole episode's assets from two separate Google Drive folders -- Scribe's session output and the images folder",
		Long: "The user's own stated workflow, in one command: point this at the Scribe output folder (containing a " +
			"recap_script.json -- a Scribe RadioPlayScript) and the separate images folder, and it drafts the same " +
			"prompt queue that script draft-prompts produces.\n\n" +
			"Both folder flags take local filesystem paths, most commonly folders under a mounted Google Drive for " +
			"Desktop volume (e.g. \"~/Library/CloudStorage/GoogleDrive-<email>/My Drive/<folder>\" on macOS). A bare " +
			"Drive folder ID cannot be resolved this way -- see drive import --help for why.",
		Example: "  flow-pp-cli episode import --scribe-folder ~/gdrive/session12-scribe --images-folder ~/gdrive/episode12-images --out episode12-queue.json",
		// pp:happy-args points the live dogfood matrix at real shipped
		// fixture folders instead of its generic Drive-ID-shaped
		// placeholders (which this command correctly rejects). Pure local
		// file work -- no live API dependency -- so this is a genuine full
		// fix, not a partial improvement. See
		// .printing-press-patches/2026-08-30-happy-args-dogfood-fixtures.md.
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:happy-args":       "--scribe-folder=testdata/dogfood-fixtures/scribe;--images-folder=testdata/dogfood-fixtures/images",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagScribeFolder == "" && flagImagesFolder == "" && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if flagScribeFolder == "" {
				return usageErr(fmt.Errorf("--scribe-folder is required; run %q for usage", cmd.CommandPath()+" --help"))
			}
			if flagImagesFolder == "" {
				return usageErr(fmt.Errorf("--images-folder is required; run %q for usage", cmd.CommandPath()+" --help"))
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, fmt.Sprintf("import episode from scribe-folder=%s images-folder=%s", flagScribeFolder, flagImagesFolder))
			}

			scribeDir, err := resolveDriveFolder(flagScribeFolder)
			if err != nil {
				return usageErr(err)
			}
			imagesDir, err := resolveDriveFolder(flagImagesFolder)
			if err != nil {
				return usageErr(err)
			}

			scriptPath, err := findRecapScript(scribeDir)
			if err != nil {
				return usageErr(err)
			}

			out := flagOut
			if out == "" {
				out = "episode-queue.json"
			}

			queue, err := draftPromptQueue(scriptPath, imagesDir)
			if err != nil {
				return usageErr(err)
			}

			marshaled, err := json.MarshalIndent(queue, "", "  ")
			if err != nil {
				return err
			}
			if err := os.WriteFile(out, marshaled, 0o600); err != nil {
				return usageErr(fmt.Errorf("writing queue file: %w", err))
			}
			unmatched := 0
			for _, s := range queue.Shots {
				if s.SeedImage == "" {
					unmatched++
				}
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "drafted %d shots from %s to %s (%d without a matched seed image)\n", len(queue.Shots), scriptPath, out, unmatched)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"scribe_folder": scribeDir, "images_folder": imagesDir, "recap_script": scriptPath, "out": out, "shots": len(queue.Shots), "unmatched_images": unmatched}, flags)
		},
	}
	cmd.Flags().StringVar(&flagScribeFolder, "scribe-folder", "", "Local path to the Scribe session output folder containing recap_script.json (see --help for details)")
	cmd.Flags().StringVar(&flagImagesFolder, "images-folder", "", "Local path to the separate images folder (see --help for details)")
	cmd.Flags().StringVar(&flagOut, "out", "", "Write the drafted queue to this file (default: episode-queue.json)")
	return cmd
}

// findRecapScript looks for Scribe's recap_script.json (a RadioPlayScript,
// per Scribe's own ADR-011) directly inside scribeDir. There is no
// documented fixed filename -- Scribe's ADR only fixes the .json extension
// and content model -- so this matches any *.json file whose name contains
// "recap" and "script", case-insensitively, picking the most recently
// modified match if more than one exists.
func findRecapScript(scribeDir string) (string, error) {
	entries, err := os.ReadDir(scribeDir)
	if err != nil {
		return "", fmt.Errorf("reading scribe folder: %w", err)
	}
	type candidate struct {
		path    string
		modTime int64
	}
	var candidates []candidate
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if !strings.HasSuffix(name, ".json") || !strings.Contains(name, "recap") || !strings.Contains(name, "script") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{path: filepath.Join(scribeDir, e.Name()), modTime: info.ModTime().Unix()})
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no recap_script.json-style file (*recap*script*.json) found in %s", scribeDir)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].modTime > candidates[j].modTime })
	return candidates[0].path, nil
}
