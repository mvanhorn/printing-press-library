package commands

import (
	"fmt"

	"github.com/andreykaipov/goobs/api/requests/scenes"
	"github.com/spf13/cobra"
)

var sceneCmd = &cobra.Command{
	Use:   "scene",
	Short: "Manage OBS scenes",
}

var sceneListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all scenes",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		resp, err := client.Scenes.GetSceneList()
		if err != nil {
			return fmt.Errorf("failed to get scenes: %w", err)
		}

		fmt.Printf("Current scene: %s\n\n", resp.CurrentProgramSceneName)
		fmt.Printf("All scenes (%d):\n", len(resp.Scenes))
		for _, s := range resp.Scenes {
			marker := "  "
			if s.SceneName == resp.CurrentProgramSceneName {
				marker = "▶ "
			}
			fmt.Printf("%s%s\n", marker, s.SceneName)
		}
		return nil
	},
}

var sceneSwitchCmd = &cobra.Command{
	Use:   "switch <name>",
	Short: "Switch to a scene",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		_, err = client.Scenes.SetCurrentProgramScene(
			scenes.NewSetCurrentProgramSceneParams().WithSceneName(args[0]),
		)
		if err != nil {
			return fmt.Errorf("failed to switch to scene %q: %w", args[0], err)
		}
		fmt.Printf("✅ Switched to scene: %s\n", args[0])
		return nil
	},
}

var sceneCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the current active scene",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		resp, err := client.Scenes.GetSceneList()
		if err != nil {
			return fmt.Errorf("failed to get current scene: %w", err)
		}
		fmt.Println(resp.CurrentProgramSceneName)
		return nil
	},
}

var sceneCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new empty scene",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		_, err = client.Scenes.CreateScene(
			scenes.NewCreateSceneParams().WithSceneName(args[0]),
		)
		if err != nil {
			return fmt.Errorf("failed to create scene %q: %w", args[0], err)
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
