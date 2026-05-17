// Copyright 2026 matysanchez. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

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
				return configErr(err)
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
						yellow(f, "→"),
						a.Date.Format("2006-01-02"),
						a.Filename,
						formatFloat(a.SizeGB()),
					)
				} else {
					fmt.Fprintf(out, "  %s %s  (not found in library)\n",
						red(f, "✗"), uuid[:8]+"...",
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

			// Delete via Photos.app scripting
			deleted, errors := 0, 0
			for _, a := range assets {
				if err := deleteViaPhotos(a.UUID); err != nil {
					fmt.Fprintf(out, "  %s %s: %v\n", red(f, "✗"), a.Filename, err)
					errors++
				} else {
					fmt.Fprintf(out, "  %s moved to Recently Deleted: %s\n",
						green(f, "✓"), a.Filename)
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

// deleteViaPhotos calls Photos.app via osascript to move an item to Recently Deleted.
func deleteViaPhotos(uuid string) error {
	script := fmt.Sprintf(`
tell application "Photos"
	activate
	set theItems to (media items whose id is "%s")
	if (count of theItems) is 0 then
		error "Item not found: %s"
	end if
	delete (item 1 of theItems)
end tell
`, uuid, uuid)

	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}
