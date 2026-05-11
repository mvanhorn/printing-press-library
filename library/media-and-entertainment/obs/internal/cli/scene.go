package cli

import (
	"fmt"

	"github.com/andreykaipov/goobs/api/requests/scenes"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/client"
	"github.com/spf13/cobra"
)

var sceneCmd = &cobra.Command{
	Use:   "scene",
	Short: "Manage OBS scenes — list, switch, create, and inspect",
}

var sceneListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all OBS scenes with active scene highlighted",
	Example: `  # List all scenes
  obs-pp-cli scene list

  # JSON output for agent use
  obs-pp-cli scene list --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		resp, err := c.Scenes.GetSceneList()
		if err != nil {
			return fmt.Errorf("failed to get scenes: %w\nhint: run 'obs-pp-cli doctor' to check your connection", err)
		}

		format := currentFormat()
		if format == "json" || format == "select" || format == "csv" {
			type sceneEntry struct {
				Name    string `json:"name"`
				Current bool   `json:"current"`
			}
			list := make([]sceneEntry, 0, len(resp.Scenes))
			for _, s := range resp.Scenes {
				list = append(list, sceneEntry{
					Name:    s.SceneName,
					Current: s.SceneName == resp.CurrentProgramSceneName,
				})
			}
			data := map[string]any{
				"current": resp.CurrentProgramSceneName,
				"scenes":  list,
			}
			if format == "select" {
				sel, _ := cmd.Root().PersistentFlags().GetString("select")
				fields := parseSelectFields(sel)
				filtered, _ := filterFields(data, fields)
				return printJSON(filtered)
			}
			return printJSON(data)
		}

		w := newTabWriter(cmd.OutOrStdout())
		fmt.Fprintf(w, "ACTIVE\tSCENE\n")
		for _, s := range resp.Scenes {
			marker := " "
			if s.SceneName == resp.CurrentProgramSceneName {
				marker = "▶"
			}
			fmt.Fprintf(w, "%s\t%s\n", marker, s.SceneName)
		}
		return w.Flush()
	},
}

var sceneSwitchCmd = &cobra.Command{
	Use:   "switch <name>",
	Short: "Switch the active OBS scene by name",
	Args:  cobra.ExactArgs(1),
	Example: `  obs-pp-cli scene switch "Interview"
  obs-pp-cli scene switch "BRB" --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun() {
			fmt.Printf("[dry-run] Would switch to scene: %s\n", args[0])
			return nil
		}

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		_, err = c.Scenes.SetCurrentProgramScene(
			scenes.NewSetCurrentProgramSceneParams().WithSceneName(args[0]),
		)
		if err != nil {
			return fmt.Errorf("failed to switch to scene %q: %w\nhint: use 'obs-pp-cli scene list' to see available scenes", args[0], err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{"scene": args[0], "ok": true})
		}
		fmt.Printf("✅ Switched to: %s\n", args[0])
		return nil
	},
}

var sceneCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the name of the currently active OBS scene",
	Example: `  obs-pp-cli scene current
  obs-pp-cli scene current --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		resp, err := c.Scenes.GetSceneList()
		if err != nil {
			return fmt.Errorf("failed to get current scene: %w", err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{"scene": resp.CurrentProgramSceneName})
		}
		fmt.Println(resp.CurrentProgramSceneName)
		return nil
	},
}

var sceneCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new empty scene with the given name",
	Args:  cobra.ExactArgs(1),
	Example: `  obs-pp-cli scene create "My Scene"
  obs-pp-cli scene create "Backup" --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun() {
			fmt.Printf("[dry-run] Would create scene: %s\n", args[0])
			return nil
		}

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		_, err = c.Scenes.CreateScene(
			scenes.NewCreateSceneParams().WithSceneName(args[0]),
		)
		if err != nil {
			return fmt.Errorf("failed to create scene %q: %w", args[0], err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{"scene": args[0], "created": true})
		}
		fmt.Printf("✅ Created scene: %s\n", args[0])
		return nil
	},
}

func init() {
	sceneCmd.AddCommand(sceneListCmd)
	sceneCmd.AddCommand(sceneSwitchCmd)
	sceneCmd.AddCommand(sceneCurrentCmd)
	sceneCmd.AddCommand(sceneCreateCmd)
}
