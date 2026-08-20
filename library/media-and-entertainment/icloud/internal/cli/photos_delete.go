// Copyright 2026 Matias Sanchez Moises and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var uuidRE = regexp.MustCompile(`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`)

func newDeleteCmd(f *rootFlags) *cobra.Command {
	var confirm bool

	cmd := &cobra.Command{
		Use:   "delete <uuid> [uuid...]",
		Short: "Move photos or videos to Recently Deleted in Photos.app",
		Long: `Move one or more items to the "Recently Deleted" album in Photos.app.

Items are NOT immediately removed — they stay in Recently Deleted for 30 days
and can be recovered. To permanently free space, empty the Recently Deleted
album from within Photos.app after running this command.

Requires Photos.app to be running (it will be launched automatically).
Requires --confirm to actually delete.

Get UUIDs from:  icloud-pp-cli photos top --json | jq '.[].uuid'`,
		Example: `  # Dry run — see what would be deleted
  icloud-pp-cli photos delete 6799AE02-EE45-4469-8AC9-1443582A828E

  # Actually move to Recently Deleted
  icloud-pp-cli photos delete --confirm 6799AE02-EE45-4469-8AC9-1443582A828E

  # Delete multiple
  icloud-pp-cli photos delete --confirm UUID1 UUID2 UUID3

  # Pipe top 5 largest videos into delete
  icloud-pp-cli photos top --type video --limit 5 --json \
    | jq -r '.[].uuid' \
    | xargs icloud-pp-cli photos delete --confirm`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uuids := args

			// Preview what will be deleted
			db, err := openPhotosDB(f.libraryPath)
			if err != nil {
				return err
			}
			assets, err := queryByUUIDs(db, uuids)
			db.Close()
			if err != nil {
				return fmt.Errorf("lookup failed: %w", err)
			}

			// Show what was found / not found
			found := map[string]Asset{}
			for _, a := range assets {
				found[a.UUID] = a
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out)
			for _, uuid := range uuids {
				a, ok := found[uuid]
				if ok {
					fmt.Fprintf(out, "  %s %s  %s  %s GB\n",
						yellow(f, out, "→"),
						a.Date.Format("2006-01-02"),
						a.Filename,
						formatFloat(a.SizeGB()),
					)
				} else {
					short := uuid
					if len(uuid) > 8 {
						short = uuid[:8] + "..."
					}
					fmt.Fprintf(out, "  %s %s  (not found in library)\n",
						red(f, out, "✗"), short,
					)
				}
			}
			fmt.Fprintln(out)

			if !confirm {
				fmt.Fprintf(out, "Dry run — %d item(s) would be moved to Recently Deleted.\n", len(assets))
				fmt.Fprintf(out, "Add --confirm to proceed.\n")
				return nil
			}

			if len(assets) == 0 {
				fmt.Fprintln(out, "No matching items found.")
				return nil
			}

			// Delete via Photos.app scripting — single batched osascript call.
			results, batchErr := deleteViaPhotosBatch(assets)
			if batchErr != nil {
				return fmt.Errorf("Photos.app scripting failed: %w", batchErr)
			}
			deleted, errors := 0, 0
			for _, a := range assets {
				if err := results[a.UUID]; err != nil {
					fmt.Fprintf(out, "  %s %s: %v\n", red(f, out, "✗"), a.Filename, err)
					errors++
				} else {
					fmt.Fprintf(out, "  %s moved to Recently Deleted: %s\n",
						green(f, out, "✓"), a.Filename)
					deleted++
				}
			}

			fmt.Fprintln(out)
			fmt.Fprintf(out, "Done — %d moved, %d failed.\n", deleted, errors)
			if deleted > 0 {
				fmt.Fprintf(out, "Open Photos.app → Recently Deleted → Empty to permanently free space.\n")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually move items to Recently Deleted (default: dry run)")

	return cmd
}

// PATCH(applescript-delete): uses osascript because there is no public iCloud deletion API.
// deleteViaPhotosBatch moves all assets to Recently Deleted in a single osascript invocation.
// activate is called once and all UUIDs are iterated inside one tell block, avoiding the
// N-process-spawn and repeated focus-steal that occurred with the old per-item scalar function.
// Returns a per-UUID error map (nil value = success).
func deleteViaPhotosBatch(assets []Asset) (map[string]error, error) {
	results := make(map[string]error, len(assets))

	// Validate UUIDs and build the AppleScript list literal.
	// Pre-populate valid UUIDs with a sentinel error so that any UUID whose
	// result line is absent from the osascript output is reported as failed
	// rather than silently inheriting Go's nil zero-value (which the caller
	// reads as success).
	var valid []Asset
	for _, a := range assets {
		if !uuidRE.MatchString(a.UUID) {
			results[a.UUID] = fmt.Errorf("invalid UUID %q: must be RFC 4122 hex-and-dash format", a.UUID)
		} else {
			results[a.UUID] = fmt.Errorf("no result returned from Photos.app")
			valid = append(valid, a)
		}
	}
	if len(valid) == 0 {
		return results, nil
	}

	quoted := make([]string, len(valid))
	for i, a := range valid {
		quoted[i] = `"` + a.UUID + `"`
	}
	script := fmt.Sprintf(`tell application "Photos"
	activate
	set theIDs to {%s}
	set output to ""
	repeat with theID in theIDs
		try
			set theItems to (media items whose id starts with theID)
			if (count of theItems) is 0 then
				set output to output & "NOTFOUND" & tab & theID & linefeed
			else
				delete (item 1 of theItems)
				set output to output & "OK" & tab & theID & linefeed
			end if
		on error errMsg
			set output to output & "ERR" & tab & theID & tab & errMsg & linefeed
		end try
	end repeat
	return output
end tell
`, strings.Join(quoted, ", "))

	raw, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = err.Error()
		}
		return results, fmt.Errorf("osascript: %s", msg)
	}

	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		status, uuid := parts[0], parts[1]
		switch status {
		case "OK":
			results[uuid] = nil
		case "NOTFOUND":
			results[uuid] = fmt.Errorf("item not found in library")
		default: // "ERR"
			msg := "unknown error"
			if len(parts) == 3 && parts[2] != "" {
				msg = parts[2]
			}
			results[uuid] = fmt.Errorf("%s", msg)
		}
	}
	return results, nil
}
