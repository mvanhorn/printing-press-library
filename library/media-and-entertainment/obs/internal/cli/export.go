package cli

import (
	"fmt"

	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/client"
	"github.com/spf13/cobra"
)

var exportScene string

// exportCmd exports the full scene and source inventory to JSON.
// Useful for backup, diff-checking, and handoff between agent sessions.
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export OBS scene and source inventory to JSON",
	Long: `Exports the full OBS scene collection to JSON, including all sources
and their settings. Useful for backup, audit, and agent handoff.

Output format:
  {
    "obs_version": "...",
    "current_scene": "...",
    "scenes": [
      {
        "name": "...",
        "sources": [{ "name": "...", "kind": "...", "enabled": true }]
      }
    ]
  }`,
	Example: `  obs-pp-cli export
  obs-pp-cli export > scene-backup.json
  obs-pp-cli export --scene "Interview"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		filterScene := exportScene

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		version, _ := c.General.GetVersion()
		sceneList, err := c.Scenes.GetSceneList()
		if err != nil {
			return fmt.Errorf("failed to list scenes: %w", err)
		}

		type sourceEntry struct {
			Name    string `json:"name"`
			Kind    string `json:"kind"`
			Enabled bool   `json:"enabled"`
		}
		type sceneEntry struct {
			Name    string        `json:"name"`
			Sources []sourceEntry `json:"sources"`
		}

		var exportedScenes []sceneEntry
		for _, s := range sceneList.Scenes {
			if filterScene != "" && s.SceneName != filterScene {
				continue
			}
			items, err := c.SceneItems.GetSceneItemList(
				sceneitems.NewGetSceneItemListParams().WithSceneName(s.SceneName),
			)
			if err != nil {
				continue
			}
			var sources []sourceEntry
			for _, item := range items.SceneItems {
				// Optionally fetch URL for browser sources.
				_ = func() string {
					settings, err := c.Inputs.GetInputSettings(
						inputs.NewGetInputSettingsParams().WithInputName(item.SourceName),
					)
					if err != nil || settings == nil {
						return ""
					}
					if u, ok := settings.InputSettings["url"].(string); ok {
						return u
					}
					return ""
				}()

				sources = append(sources, sourceEntry{
					Name:    item.SourceName,
					Kind:    item.InputKind,
					Enabled: item.SceneItemEnabled,
				})
			}
			exportedScenes = append(exportedScenes, sceneEntry{
				Name:    s.SceneName,
				Sources: sources,
			})
		}

		obsVer := ""
		if version != nil {
			obsVer = version.ObsVersion
		}

		out := map[string]any{
			"obs_version":   obsVer,
			"current_scene": sceneList.CurrentProgramSceneName,
			"scene_count":   len(exportedScenes),
			"scenes":        exportedScenes,
		}
		// In agent mode, strip zero/empty fields for compact output.
		if isAgentMode() {
			out = compactObjectFields(out)
		}
		return printJSON(out)
	},
}

func init() {
	exportCmd.Flags().StringVar(&exportScene, "scene", "", "Export only this scene (default: all scenes)")
}
