package commands

import (
	"fmt"
	"strings"

	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/spf13/cobra"
)

var guestCmd = &cobra.Command{
	Use:   "guest",
	Short: "Manage VDO.Ninja guest sources",
	Long: `Add and remove VDO.Ninja browser sources in OBS scenes.

VDO.Ninja is free browser-based WebRTC for remote guests. No account required.
Get a room at: https://vdo.ninja/
Use the view URL (not push URL) as the source URL here.`,
}

var guestAddCmd = &cobra.Command{
	Use:   "add <source-name> <vdo-ninja-url>",
	Short: "Add or update a VDO.Ninja guest source",
	Long: `Creates a browser source with the given VDO.Ninja view URL.
If a source with that name already exists, updates its URL.

Example:
  obs-pp-cli guest add "Jason" "https://vdo.ninja/?view=abc123&cleanoutput&transparent"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceName := args[0]
		url := args[1]
		scene, _ := cmd.Flags().GetString("scene")

		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		if scene == "" {
			resp, err := client.Scenes.GetSceneList()
			if err != nil {
				return fmt.Errorf("failed to get current scene: %w", err)
			}
			scene = resp.CurrentProgramSceneName
		}

		// Try to update existing source first
		existing, _ := client.Inputs.GetInputSettings(
			inputs.NewGetInputSettingsParams().WithInputName(sourceName),
		)
		if existing != nil {
			_, err = client.Inputs.SetInputSettings(
				inputs.NewSetInputSettingsParams().
					WithInputName(sourceName).
					WithInputSettings(map[string]any{"url": url}),
			)
			if err != nil {
				return fmt.Errorf("failed to update source URL: %w", err)
			}
			fmt.Printf("✅ Updated %q → %s\n", sourceName, url)
			return nil
		}

		// Create new browser source
		_, err = client.Inputs.CreateInput(
			inputs.NewCreateInputParams().
				WithSceneName(scene).
				WithInputName(sourceName).
				WithInputKind("browser_source").
				WithInputSettings(map[string]any{
					"url":    url,
					"width":  960,
					"height": 1080,
					"fps":    30,
				}).
				WithSceneItemEnabled(true),
		)
		if err != nil {
			return fmt.Errorf("failed to create guest source: %w", err)
		}

		fmt.Printf("✅ Added %q in scene %q\n", sourceName, scene)
		fmt.Printf("   URL: %s\n", url)
		return nil
	},
}

var guestRemoveCmd = &cobra.Command{
	Use:   "remove <source-name>",
	Short: "Remove a guest source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		_, err = client.Inputs.RemoveInput(
			inputs.NewRemoveInputParams().WithInputName(args[0]),
		)
		if err != nil {
			return fmt.Errorf("failed to remove %q: %w", args[0], err)
		}
		fmt.Printf("✅ Removed source: %s\n", args[0])
		return nil
	},
}

var guestListCmd = &cobra.Command{
	Use:   "list",
	Short: "List browser sources in the current scene",
	RunE: func(cmd *cobra.Command, args []string) error {
		scene, _ := cmd.Flags().GetString("scene")

		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		if scene == "" {
			resp, err := client.Scenes.GetSceneList()
			if err != nil {
				return fmt.Errorf("failed to get current scene: %w", err)
			}
			scene = resp.CurrentProgramSceneName
		}

		items, err := client.SceneItems.GetSceneItemList(
			sceneitems.NewGetSceneItemListParams().WithSceneName(scene),
		)
		if err != nil {
			return fmt.Errorf("failed to list scene items: %w", err)
		}

		fmt.Printf("Scene: %s\n\n", scene)
		count := 0
		for _, item := range items.SceneItems {
			if item.InputKind != "browser_source" {
				continue
			}
			count++
			visibility := "👁  visible"
			if !item.SceneItemEnabled {
				visibility = "   hidden"
			}
			settings, _ := client.Inputs.GetInputSettings(
				inputs.NewGetInputSettingsParams().WithInputName(item.SourceName),
			)
			url := "(no URL)"
			if settings != nil {
				if u, ok := settings.InputSettings["url"].(string); ok {
					url = truncate(u, 64)
				}
			}
			fmt.Printf("%s  %s\n     %s\n\n", visibility, item.SourceName, url)
		}
		if count == 0 {
			fmt.Println("No browser sources in this scene.")
		}
		return nil
	},
}

func init() {
	guestAddCmd.Flags().String("scene", "", "Target scene (defaults to current)")
	guestListCmd.Flags().String("scene", "", "Scene to inspect (defaults to current)")

	guestCmd.AddCommand(guestAddCmd)
	guestCmd.AddCommand(guestRemoveCmd)
	guestCmd.AddCommand(guestListCmd)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// containsCI is available for future use
var _ = func(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
