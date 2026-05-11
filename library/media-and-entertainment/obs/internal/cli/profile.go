package cli

import (
	"fmt"

	"github.com/andreykaipov/goobs/api/requests/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/client"
	"github.com/spf13/cobra"
)

// profileCmd manages OBS Studio profiles.
// Profiles store encoder settings, output configuration, and scene collections.
// Switching profiles is useful for transitioning between show types (streaming vs recording vs podcasting).
var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage OBS Studio profiles — list, switch, and inspect",
	Long: `OBS Studio profiles store encoder settings, output paths, and stream keys.
Use profiles to switch between different show configurations without reconfiguring manually.

Examples:
  obs-pp-cli profile list          List all available profiles
  obs-pp-cli profile current       Show the active profile
  obs-pp-cli profile switch Podcast Switch to the Podcast profile`,
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available OBS profiles",
	Example: `  obs-pp-cli profile list
  obs-pp-cli profile list --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		resp, err := c.Config.GetProfileList()
		if err != nil {
			return fmt.Errorf("failed to get profile list: %w", err)
		}

		if currentFormat() == "json" {
			type profileEntry struct {
				Name    string `json:"name"`
				Current bool   `json:"current"`
			}
			entries := make([]profileEntry, 0, len(resp.Profiles))
			for _, p := range resp.Profiles {
				entries = append(entries, profileEntry{
					Name:    p,
					Current: p == resp.CurrentProfileName,
				})
			}
			return printJSON(map[string]any{
				"current":  resp.CurrentProfileName,
				"profiles": entries,
			})
		}

		w := newTabWriter(cmd.OutOrStdout())
		fmt.Fprintf(w, "ACTIVE\tPROFILE\n")
		for _, p := range resp.Profiles {
			marker := " "
			if p == resp.CurrentProfileName {
				marker = "▶"
			}
			fmt.Fprintf(w, "%s\t%s\n", marker, p)
		}
		return w.Flush()
	},
}

var profileCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the name of the currently active OBS profile",
	Example: `  obs-pp-cli profile current
  obs-pp-cli profile current --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		resp, err := c.Config.GetProfileList()
		if err != nil {
			return fmt.Errorf("failed to get profile list: %w", err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{"profile": resp.CurrentProfileName})
		}
		fmt.Println(resp.CurrentProfileName)
		return nil
	},
}

var profileSwitchCmd = &cobra.Command{
	Use:   "switch <name>",
	Short: "Switch the active OBS profile by name",
	Args:  cobra.ExactArgs(1),
	Example: `  obs-pp-cli profile switch "Streaming"
  obs-pp-cli profile switch "Podcast" --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun() {
			fmt.Printf("[dry-run] Would switch to profile: %s\n", args[0])
			return nil
		}

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		_, err = c.Config.SetCurrentProfile(
			config.NewSetCurrentProfileParams().WithProfileName(args[0]),
		)
		if err != nil {
			return fmt.Errorf("failed to switch to profile %q: %w\nhint: use 'obs-pp-cli profile list' to see available profiles", args[0], err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{"profile": args[0], "ok": true})
		}
		fmt.Printf("✅ Switched to profile: %s\n", args[0])
		return nil
	},
}

func init() {
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileCurrentCmd)
	profileCmd.AddCommand(profileSwitchCmd)
}
