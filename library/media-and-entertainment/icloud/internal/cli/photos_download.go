// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newDownloadCmd(f *rootFlags) *cobra.Command {
	var outputDir string
	var sensitive bool
	var confirm bool
	var mediaType string
	var limit int

	cmd := &cobra.Command{
		Use:   "download [uuid...]",
		Short: "Export originals from iCloud to a local folder",
		Long: `Export original photos or videos from iCloud Photos to a local folder.

Photos.app is used to perform the export. If a file is not stored locally
(iCloud optimized storage), Photos.app downloads the original from iCloud
automatically before copying it to the output directory.

Pass UUIDs explicitly, or use --sensitive to target items Apple's on-device
ML engine has flagged as containing nudity.

Get UUIDs from any read command:
  icloud-pp-cli photos top --json | jq -r '.[].uuid'
  icloud-pp-cli photos videos --json | jq -r '.[].uuid'`,
		Example: `  # Export a specific item by UUID
  icloud-pp-cli photos download --output ~/Desktop 6799AE02-EE45-4469-8AC9-1443582A828E

  # Export a random 10 sensitive videos to a folder (requires --confirm)
  icloud-pp-cli photos download --sensitive --confirm --type video --limit 10 --output ~/Desktop/export

  # Pipe the 5 largest videos into download
  icloud-pp-cli photos top --type video --limit 5 --json \
    | jq -r '.[].uuid' \
    | xargs icloud-pp-cli photos download --output ~/Desktop/big`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !sensitive {
				return usageErr(fmt.Errorf("provide at least one UUID or use --sensitive"))
			}
			if len(args) > 0 && sensitive {
				return usageErr(fmt.Errorf("--sensitive cannot be combined with explicit UUIDs"))
			}
			if mediaType != "all" && mediaType != "photo" && mediaType != "video" {
				return usageErr(fmt.Errorf("--type must be one of: all, photo, video"))
			}
			if sensitive && !confirm {
				return usageErr(fmt.Errorf(
					"--sensitive requires --confirm\n\n" +
						"This flag targets content Apple's on-device ML has flagged as nudity.\n" +
						"Add --confirm to acknowledge and proceed with the export."))
			}

			out := cmd.OutOrStdout()

			// Resolve and create output directory.
			abs, err := filepath.Abs(outputDir)
			if err != nil {
				return fmt.Errorf("invalid output path: %w", err)
			}
			if err := os.MkdirAll(abs, 0o755); err != nil {
				return fmt.Errorf("cannot create output directory: %w", err)
			}

			db, err := openPhotosDB(f.libraryPath)
			if err != nil {
				return err
			}

			var assets []Asset
			if sensitive {
				assets, err = querySensitiveAssets(db, limit, mediaType)
			} else {
				assets, err = queryByUUIDs(db, args)
			}
			db.Close()
			if err != nil {
				return fmt.Errorf("lookup failed: %w", err)
			}

			if len(assets) == 0 {
				fmt.Fprintln(out, "No matching items found.")
				return nil
			}

			fmt.Fprintln(out)
			fmt.Fprintf(out, "Exporting %d item(s) to %s\n\n", len(assets), abs)
			for _, a := range assets {
				fmt.Fprintf(out, "  %s  %s  %s\n",
					yellow(f, out, "→"),
					a.Filename,
					formatSize(f, out, a.SizeGB()),
				)
			}
			fmt.Fprintln(out)
			exported, failed := 0, 0
			for i, a := range assets {
				fmt.Fprintf(out, "  [%d/%d] %s … ", i+1, len(assets), a.Filename)
				uuidName, err := exportOne(a, abs)
				if err != nil {
					fmt.Fprintf(out, "%s %v\n", red(f, out, "✗"), err)
					failed++
				} else {
					fmt.Fprintf(out, "%s → %s\n", green(f, out, "✓"), uuidName)
					exported++
				}
			}

			fmt.Fprintln(out)
			fmt.Fprintf(out, "Done — %d exported, %d failed.\n", exported, failed)
			if exported > 0 {
				fmt.Fprintf(out, "Files saved to: %s\n", abs)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Destination folder for exported files")
	cmd.Flags().BoolVar(&sensitive, "sensitive", false, "Export items flagged as containing sensitive content (requires --confirm)")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Required when using --sensitive: acknowledge export of nudity-flagged content")
	cmd.Flags().StringVar(&mediaType, "type", "all", "Media type when using --sensitive: all, photo, video")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max items to export when using --sensitive (0 = all)")

	return cmd
}

// exportOne exports a single asset to destDir via Photos.app AppleScript.
// Photos.app downloads the original from iCloud if needed before exporting.
// Returns the final UUID-based filename on success.
func exportOne(a Asset, destDir string) (string, error) {
	if !uuidRE.MatchString(a.UUID) {
		return "", fmt.Errorf("invalid UUID %q", a.UUID)
	}

	// Snapshot directory before so we can detect the new file.
	before, _ := listDir(destDir)

	// Pass destDir and UUID as separate -e arguments to avoid any quoting issues
	// inside the AppleScript string literal (e.g. paths with double quotes).
	script := fmt.Sprintf(`tell application "Photos"
	activate
	set found to (media items whose id starts with "%s")
	if (count of found) is 0 then
		error "item not found: %s"
	end if
	export {item 1 of found} to destArg using originals true
end tell`, a.UUID, a.UUID)

	raw, err := exec.Command("osascript",
		"-e", fmt.Sprintf(`set destArg to POSIX file %q`, destDir),
		"-e", script,
	).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}

	// Give the filesystem a moment to flush.
	time.Sleep(200 * time.Millisecond)

	after, _ := listDir(destDir)

	// Find the newly appeared file by exact (case-insensitive) filename match.
	// We intentionally avoid extension-only fallbacks: if two assets share the same
	// extension we would silently attribute the wrong file to the wrong UUID.
	matchedName := ""
	lower := strings.ToLower(a.Filename)
	for name := range after {
		if before[name] {
			continue
		}
		if strings.ToLower(name) == lower {
			matchedName = name
			break
		}
	}
	if matchedName == "" {
		return "", fmt.Errorf("file not found after export — iCloud download may have timed out")
	}

	// Rename to <UUID>.<lowercased-ext>.
	ext := strings.ToLower(filepath.Ext(matchedName))
	uuidName := a.UUID + ext
	oldPath := filepath.Join(destDir, matchedName)
	newPath := filepath.Join(destDir, uuidName)
	if oldPath != newPath {
		if err := os.Rename(oldPath, newPath); err != nil {
			return "", fmt.Errorf("exported but rename failed: %w", err)
		}
	}
	return uuidName, nil
}

// listDir returns a set of filenames (not full paths) in dir.
func listDir(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(entries))
	for _, e := range entries {
		m[e.Name()] = true
	}
	return m, nil
}
