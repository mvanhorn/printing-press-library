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

  # Export a random 10 sensitive videos to a folder
  icloud-pp-cli photos download --sensitive --type video --limit 10 --output ~/Desktop/naked

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
			fmt.Fprintf(out, "Waiting for Photos.app to export (iCloud downloads may take a moment)...\n")

			results, batchErr := exportViaPhotos(assets, abs)
			if batchErr != nil {
				return fmt.Errorf("Photos.app export failed: %w", batchErr)
			}

			fmt.Fprintln(out)
			exported, failed := 0, 0
			for _, a := range assets {
				if err := results[a.UUID]; err != nil {
					fmt.Fprintf(out, "  %s %s: %v\n", red(f, out, "✗"), a.Filename, err)
					failed++
				} else {
					fmt.Fprintf(out, "  %s %s\n", green(f, out, "✓"), a.Filename)
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
	cmd.Flags().BoolVar(&sensitive, "sensitive", false, "Export items flagged as containing sensitive content")
	cmd.Flags().StringVar(&mediaType, "type", "all", "Media type when using --sensitive: all, photo, video")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max items to export when using --sensitive (0 = all)")

	return cmd
}

// exportViaPhotos exports assets to destDir via a single Photos.app AppleScript call.
// Photos.app downloads originals from iCloud automatically before exporting.
// Returns a per-UUID error map (nil value = success).
func exportViaPhotos(assets []Asset, destDir string) (map[string]error, error) {
	results := make(map[string]error, len(assets))

	var valid []Asset
	for _, a := range assets {
		if !uuidRE.MatchString(a.UUID) {
			results[a.UUID] = fmt.Errorf("invalid UUID %q", a.UUID)
		} else {
			valid = append(valid, a)
		}
	}
	if len(valid) == 0 {
		return results, nil
	}

	// Snapshot output directory before export so we can detect new files.
	before, _ := listDir(destDir)

	// Build AppleScript UUID list literal.
	quoted := make([]string, len(valid))
	for i, a := range valid {
		quoted[i] = `"` + a.UUID + `"`
	}

	// The export command is synchronous: Photos.app downloads from iCloud if needed,
	// then copies the original to destDir, before the tell block returns.
	script := fmt.Sprintf(`tell application "Photos"
	activate
	set destFolder to POSIX file %q
	set theIDs to {%s}
	set theItems to {}
	repeat with theID in theIDs
		try
			set found to (media items whose id starts with theID)
			if (count of found) > 0 then
				set theItems to theItems & found
			end if
		end try
	end repeat
	if (count of theItems) > 0 then
		export theItems to destFolder using originals true
	end if
end tell
`, destDir, strings.Join(quoted, ", "))

	raw, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = err.Error()
		}
		return results, fmt.Errorf("osascript: %s", msg)
	}

	// Give the filesystem a moment to flush writes.
	time.Sleep(300 * time.Millisecond)

	after, _ := listDir(destDir)

	// Build set of files that appeared during the export.
	newFiles := make(map[string]bool)
	for name := range after {
		if !before[name] {
			newFiles[name] = true
		}
	}

	// Match each UUID to a newly appeared file by filename (case-insensitive).
	for _, a := range valid {
		matched := false
		lower := strings.ToLower(a.Filename)
		for name := range newFiles {
			if strings.ToLower(name) == lower {
				matched = true
				break
			}
		}
		if matched {
			results[a.UUID] = nil
		} else {
			results[a.UUID] = fmt.Errorf("file not found after export — may still be downloading from iCloud")
		}
	}
	return results, nil
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
