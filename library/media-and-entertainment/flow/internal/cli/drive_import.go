// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type importedImage struct {
	File     string `json:"file"`
	Dest     string `json:"dest"`
	Tag      string `json:"tag,omitempty"`
	Untagged bool   `json:"untagged,omitempty"`
}

func newNovelDriveImportCmd(flags *rootFlags) *cobra.Command {
	var flagFolderId string
	var flagTagScene bool
	var flagDest string
	var flagProject string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Pull seed images straight out of Google Drive into a Flow project",
		Long: "Copies image files out of a Google Drive folder into a local staging directory. Flow's own UI has no " +
			"Drive picker at all -- \"Upload media\" is a bare native file dialog -- so this replaces the manual " +
			"download-then-reupload dance.\n\n" +
			"--folder-id takes a local filesystem path, most commonly a folder under a mounted Google Drive for " +
			"Desktop volume (e.g. \"~/Library/CloudStorage/GoogleDrive-<email>/My Drive/<folder>\" on macOS). A bare " +
			"Drive folder ID (from a drive.google.com URL) cannot be resolved this way -- that would require the " +
			"separate Google Drive API with its own OAuth2 setup, which this CLI does not implement.\n\n" +
			"--tag-scene tags each imported image with a character name matched from its filename, using character " +
			"names synced from --project. There is no confirmed Flow or Scribe filename convention for scene tags, " +
			"so this is a best-effort substring match, not a documented contract.",
		Example: "  flow-pp-cli drive import --folder-id ~/gdrive/episode3-images --tag-scene --project a1b2c3d4-e5f6-47a8-9b0c-1d2e3f4a5b6c",
		// pp:happy-args gives the live dogfood matrix a real local fixture
		// path instead of its generic Drive-ID-shaped placeholder (which
		// this command correctly rejects). --project is a well-formed but
		// non-real ID -- there is no universally real project ID this
		// shipped annotation could reference across accounts, so this shot
		// is a genuine improvement (real 404 instead of a client-side
		// validation error) without claiming a guaranteed full pass. See
		// .printing-press-patches/2026-08-30-happy-args-dogfood-fixtures.md.
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:happy-args":       "--folder-id=testdata/dogfood-fixtures/images;--tag-scene;--project=a1b2c3d4-e5f6-47a8-9b0c-1d2e3f4a5b6c",
			"pp:typed-exit-codes": "0,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagFolderId == "" && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if flagFolderId == "" {
				return usageErr(fmt.Errorf("--folder-id is required; run %q for usage", cmd.CommandPath()+" --help"))
			}
			if flagTagScene && flagProject == "" {
				return usageErr(fmt.Errorf("--tag-scene requires --project (to match filenames against synced character names)"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, fmt.Sprintf("import images from %s", flagFolderId))
			}

			source, err := resolveDriveFolder(flagFolderId)
			if err != nil {
				return usageErr(err)
			}
			entries, err := os.ReadDir(source)
			if err != nil {
				return usageErr(fmt.Errorf("reading folder: %w", err))
			}

			dest := flagDest
			if dest == "" {
				dest = filepath.Join(".", "flow-import", filepath.Base(source))
			}
			if err := os.MkdirAll(dest, 0o750); err != nil {
				return usageErr(fmt.Errorf("creating destination directory: %w", err))
			}

			var characterNames []string
			if flagTagScene {
				contents, err := fetchProjectContents(cmd, flags, flagProject)
				if err != nil {
					return err
				}
				characterNames, err = characterDisplayNames(contents)
				if err != nil {
					return apiErr(err)
				}
			}

			var imported []importedImage
			for _, e := range entries {
				if e.IsDir() || !isImageFile(e.Name()) {
					continue
				}
				srcPath := filepath.Join(source, e.Name())
				destPath := filepath.Join(dest, e.Name())
				if err := copyFile(srcPath, destPath); err != nil {
					return apiErr(fmt.Errorf("copying %s: %w", e.Name(), err))
				}
				img := importedImage{File: e.Name(), Dest: destPath}
				if flagTagScene {
					if tag := matchCharacterTag(e.Name(), characterNames); tag != "" {
						img.Tag = tag
					} else {
						img.Untagged = true
					}
				}
				imported = append(imported, img)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "imported %d image(s) from %s -> %s\n", len(imported), source, dest)
				for _, img := range imported {
					if img.Tag != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s (tagged: %s)\n", img.File, img.Tag)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", img.File)
					}
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"source": source, "dest": dest, "count": len(imported), "images": imported}, flags)
		},
	}
	cmd.Flags().StringVar(&flagFolderId, "folder-id", "", "Local path to the Drive folder to import from (see --help for details)")
	cmd.Flags().BoolVar(&flagTagScene, "tag-scene", false, "Tag each imported image with a character name matched from its filename (requires --project)")
	cmd.Flags().StringVar(&flagDest, "dest", "", "Local destination directory (default: ./flow-import/<source folder name>)")
	cmd.Flags().StringVar(&flagProject, "project", "", "Flow project ID to source character names from, for --tag-scene")
	return cmd
}

func copyFile(src, dst string) error {
	in, err := os.Open(filepath.Clean(src)) // #nosec G304 -- src is enumerated from the user-specified --folder-id directory, this command's documented purpose.
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(filepath.Clean(dst)) // #nosec G304 -- dst is derived from the user-specified --dest directory, this command's documented purpose.
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// matchCharacterTag is a best-effort filename match, not a documented
// convention -- see the command's --help text.
func matchCharacterTag(filename string, characterNames []string) string {
	needle := normalizeForMatch(filename)
	for _, name := range characterNames {
		if name == "" {
			continue
		}
		if strings.Contains(needle, normalizeForMatch(name)) {
			return name
		}
	}
	return ""
}
