package cli

import (
	"fmt"

	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/client"
	"github.com/spf13/cobra"
)

var (
	guestAddSource   string
	guestAddURL      string
	guestAddScene    string
	guestRemSource   string
	guestListScene   string
)

var guestCmd = &cobra.Command{
	Use:   "guest",
	Short: "Manage VDO.Ninja guest video sources in OBS scenes",
	Long: `Add and remove VDO.Ninja browser sources in OBS scenes.

VDO.Ninja is free browser-based WebRTC for remote guests. No account required.
Send guests a push URL:  https://vdo.ninja/?push=ROOMID
Add to OBS as a view URL: https://vdo.ninja/?view=ROOMID&cleanoutput&transparent`,
}

var guestAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add or update a VDO.Ninja guest source in the current scene",
	Long: `Creates a browser source with the given VDO.Ninja view URL.
If a source with that name already exists, updates its URL.`,
	Example: `  obs-pp-cli guest add --source "Jason" --url "https://vdo.ninja/?view=abc123&cleanoutput&transparent"
  obs-pp-cli guest add --source "Jason" --url "https://vdo.ninja/?view=abc123" --scene "Interview"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun() {
			fmt.Printf("[dry-run] Would add guest source %q → %s\n", guestAddSource, guestAddURL)
			return nil
		}

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		scene := guestAddScene
		if scene == "" {
			resp, err := c.Scenes.GetSceneList()
			if err != nil {
				return fmt.Errorf("failed to get current scene: %w", err)
			}
			scene = resp.CurrentProgramSceneName
		}

		// Try to update existing source URL first.
		existing, _ := c.Inputs.GetInputSettings(
			inputs.NewGetInputSettingsParams().WithInputName(guestAddSource),
		)
		if existing != nil {
			_, err = c.Inputs.SetInputSettings(
				inputs.NewSetInputSettingsParams().
					WithInputName(guestAddSource).
					WithInputSettings(map[string]any{"url": guestAddURL}),
			)
			if err != nil {
				return fmt.Errorf("failed to update source URL: %w", err)
			}
			if currentFormat() == "json" {
				return printJSON(map[string]any{"source": guestAddSource, "url": guestAddURL, "updated": true})
			}
			fmt.Printf("✅ Updated %q → %s\n", guestAddSource, guestAddURL)
			return nil
		}

		// Create new browser source in the target scene.
		_, err = c.Inputs.CreateInput(
			inputs.NewCreateInputParams().
				WithSceneName(scene).
				WithInputName(guestAddSource).
				WithInputKind("browser_source").
				WithInputSettings(map[string]any{
					"url":    guestAddURL,
					"width":  960,
					"height": 1080,
					"fps":    30,
				}).
				WithSceneItemEnabled(true),
		)
		if err != nil {
			return hint(fmt.Errorf("failed to create guest source: %w", err),
				"ensure the scene exists with 'obs-pp-cli scene list'")
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{"source": guestAddSource, "scene": scene, "url": guestAddURL, "created": true})
		}
		fmt.Printf("✅ Added %q in scene %q\n", guestAddSource, scene)
		fmt.Printf("   URL: %s\n", guestAddURL)
		return nil
	},
}

var guestRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a VDO.Ninja guest source by name",
	Example: `  obs-pp-cli guest remove --source "Jason"
  obs-pp-cli guest remove --source "Jason" --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun() {
			fmt.Printf("[dry-run] Would remove source: %s\n", guestRemSource)
			return nil
		}

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		_, err = c.Inputs.RemoveInput(
			inputs.NewRemoveInputParams().WithInputName(guestRemSource),
		)
		if err != nil {
			return fmt.Errorf("failed to remove %q: %w\nhint: use 'obs-pp-cli guest list' to see available sources", guestRemSource, err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{"source": guestRemSource, "removed": true})
		}
		fmt.Printf("✅ Removed source: %s\n", guestRemSource)
		return nil
	},
}

var guestListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all browser sources (guest feeds) in the current scene",
	Example: `  obs-pp-cli guest list
  obs-pp-cli guest list --scene "Interview"
  obs-pp-cli guest list --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		scene := guestListScene

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		if scene == "" {
			resp, err := c.Scenes.GetSceneList()
			if err != nil {
				return fmt.Errorf("failed to get current scene: %w", err)
			}
			scene = resp.CurrentProgramSceneName
		}

		items, err := c.SceneItems.GetSceneItemList(
			sceneitems.NewGetSceneItemListParams().WithSceneName(scene),
		)
		if err != nil {
			return fmt.Errorf("failed to list scene items: %w", err)
		}

		type guestEntry struct {
			Name    string `json:"name"`
			URL     string `json:"url"`
			Visible bool   `json:"visible"`
		}
		var guests []guestEntry

		for _, item := range items.SceneItems {
			if item.InputKind != "browser_source" {
				continue
			}
			settings, _ := c.Inputs.GetInputSettings(
				inputs.NewGetInputSettingsParams().WithInputName(item.SourceName),
			)
			url := "(no URL)"
			if settings != nil {
				if u, ok := settings.InputSettings["url"].(string); ok {
					url = u
				}
			}
			guests = append(guests, guestEntry{
				Name:    item.SourceName,
				URL:     url,
				Visible: item.SceneItemEnabled,
			})
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{"scene": scene, "guests": guests})
		}

		fmt.Printf("Scene: %s\n\n", scene)
		if len(guests) == 0 {
			fmt.Println("No browser sources in this scene.")
			return nil
		}
		w := newTabWriter(cmd.OutOrStdout())
		fmt.Fprintf(w, "VISIBLE\tNAME\tURL\n")
		for _, g := range guests {
			vis := "👁 yes"
			if !g.Visible {
				vis = "   no"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", vis, g.Name, truncate(g.URL, 60))
		}
		return w.Flush()
	},
}

func init() {
	guestAddCmd.Flags().StringVar(&guestAddSource, "source", "", "Source name for the guest feed")
	guestAddCmd.Flags().StringVar(&guestAddURL, "url", "", "VDO.Ninja view URL for this guest")
	guestAddCmd.Flags().StringVar(&guestAddScene, "scene", "", "Target scene (defaults to current active scene)")
	_ = guestAddCmd.MarkFlagRequired("source")
	_ = guestAddCmd.MarkFlagRequired("url")

	guestRemoveCmd.Flags().StringVar(&guestRemSource, "source", "", "Name of the source to remove")
	_ = guestRemoveCmd.MarkFlagRequired("source")

	guestListCmd.Flags().StringVar(&guestListScene, "scene", "", "Scene to inspect (defaults to current active scene)")

	guestCmd.AddCommand(guestAddCmd)
	guestCmd.AddCommand(guestRemoveCmd)
	guestCmd.AddCommand(guestListCmd)
}
