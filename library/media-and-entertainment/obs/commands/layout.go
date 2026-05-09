package commands

import (
	"fmt"

	goobs "github.com/andreykaipov/goobs"
	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/andreykaipov/goobs/api/requests/scenes"
	"github.com/andreykaipov/goobs/api/typedefs"
	"github.com/spf13/cobra"
)

const canvasW = 1920.0
const canvasH = 1080.0

var layoutCmd = &cobra.Command{
	Use:   "layout",
	Short: "Build production scene layouts",
	Long:  `Create named OBS scenes with pre-positioned browser sources. Creates or updates scenes without touching your existing ones.`,
}

var layoutInterviewCmd = &cobra.Command{
	Use:   "interview",
	Short: "Two-person side-by-side interview layout",
	Long: `Creates an "Interview" scene with two browser source slots.
  Host Camera  (left):  960x1080 at position 0,0
  Guest Camera (right): 960x1080 at position 960,0

Update guest URL after: obs-pp-cli guest add "Guest Camera" <vdo-ninja-url>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		if err := ensureScene(client, name); err != nil {
			return err
		}
		if err := addBrowserSource(client, name, "Host Camera",
			"https://vdo.ninja/?push=host&cleanoutput&transparent",
			960, 1080, 0, 0); err != nil {
			return fmt.Errorf("host source: %w", err)
		}
		if err := addBrowserSource(client, name, "Guest Camera",
			"https://vdo.ninja/?view=guest&cleanoutput&transparent",
			960, 1080, 960, 0); err != nil {
			return fmt.Errorf("guest source: %w", err)
		}

		fmt.Printf("✅ Layout created: %s\n", name)
		fmt.Printf("   Host Camera  — left  (960x1080 @ 0,0)\n")
		fmt.Printf("   Guest Camera — right (960x1080 @ 960,0)\n")
		fmt.Printf("\nNext: obs-pp-cli guest add \"Guest Camera\" <vdo-ninja-url>\n")
		fmt.Printf("      obs-pp-cli scene switch \"%s\"\n", name)
		return nil
	},
}

var layoutSoloCmd = &cobra.Command{
	Use:   "solo",
	Short: "Single host fullscreen layout",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		if err := ensureScene(client, name); err != nil {
			return err
		}
		if err := addBrowserSource(client, name, "Host Camera",
			"https://vdo.ninja/?push=host&cleanoutput&transparent",
			canvasW, canvasH, 0, 0); err != nil {
			return fmt.Errorf("host source: %w", err)
		}

		fmt.Printf("✅ Layout created: %s\n", name)
		fmt.Printf("   Host Camera — fullscreen (1920x1080)\n")
		return nil
	},
}

var layoutScreenshareCmd = &cobra.Command{
	Use:   "screenshare",
	Short: "Guest screen share fullscreen + host camera PiP (bottom-right)",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		if err := ensureScene(client, name); err != nil {
			return err
		}
		if err := addBrowserSource(client, name, "Guest Screen",
			"https://vdo.ninja/?view=guestscreen&cleanoutput&transparent",
			canvasW, canvasH, 0, 0); err != nil {
			return fmt.Errorf("screen source: %w", err)
		}
		pipW, pipH := 320.0, 180.0
		if err := addBrowserSource(client, name, "Host Camera",
			"https://vdo.ninja/?push=host&cleanoutput&transparent",
			pipW, pipH, canvasW-pipW-20, canvasH-pipH-20); err != nil {
			return fmt.Errorf("host PiP source: %w", err)
		}

		fmt.Printf("✅ Layout created: %s\n", name)
		fmt.Printf("   Guest Screen — fullscreen\n")
		fmt.Printf("   Host Camera  — PiP bottom-right (320x180)\n")
		return nil
	},
}

var layoutBRBCmd = &cobra.Command{
	Use:   "brb",
	Short: "Be Right Back empty scene",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		if err := ensureScene(client, name); err != nil {
			return err
		}
		fmt.Printf("✅ Scene created: %s\n", name)
		fmt.Printf("   Add your BRB graphic as a source in OBS.\n")
		fmt.Printf("   Switch to it: obs-pp-cli scene switch \"%s\"\n", name)
		return nil
	},
}

func init() {
	layoutInterviewCmd.Flags().String("name", "Interview", "Scene name to create")
	layoutSoloCmd.Flags().String("name", "Solo", "Scene name to create")
	layoutScreenshareCmd.Flags().String("name", "Screen Share", "Scene name to create")
	layoutBRBCmd.Flags().String("name", "BRB", "Scene name to create")

	layoutCmd.AddCommand(layoutInterviewCmd)
	layoutCmd.AddCommand(layoutSoloCmd)
	layoutCmd.AddCommand(layoutScreenshareCmd)
	layoutCmd.AddCommand(layoutBRBCmd)
}

func ensureScene(client *goobs.Client, name string) error {
	list, err := client.Scenes.GetSceneList()
	if err != nil {
		return fmt.Errorf("failed to list scenes: %w", err)
	}
	for _, s := range list.Scenes {
		if s.SceneName == name {
			fmt.Printf("Scene %q already exists — updating sources.\n", name)
			return nil
		}
	}
	_, err = client.Scenes.CreateScene(
		scenes.NewCreateSceneParams().WithSceneName(name),
	)
	return err
}

func addBrowserSource(client *goobs.Client, sceneName, sourceName, url string, w, h, x, y float64) error {
	_, err := client.Inputs.CreateInput(
		inputs.NewCreateInputParams().
			WithSceneName(sceneName).
			WithInputName(sourceName).
			WithInputKind("browser_source").
			WithInputSettings(map[string]any{
				"url":    url,
				"width":  int(w),
				"height": int(h),
				"fps":    30,
			}).
			WithSceneItemEnabled(true),
	)
	if err != nil {
		// Source likely already exists — proceed to transform update
		fmt.Printf("   Note: source %q exists, updating transform.\n", sourceName)
	}

	itemsResp, err := client.SceneItems.GetSceneItemList(
		sceneitems.NewGetSceneItemListParams().WithSceneName(sceneName),
	)
	if err != nil {
		return fmt.Errorf("failed to list scene items: %w", err)
	}

	var itemID int
	for _, item := range itemsResp.SceneItems {
		if item.SourceName == sourceName {
			itemID = item.SceneItemID
			break
		}
	}
	if itemID == 0 {
		return fmt.Errorf("could not find scene item %q after creation", sourceName)
	}

	_, err = client.SceneItems.SetSceneItemTransform(
		sceneitems.NewSetSceneItemTransformParams().
			WithSceneName(sceneName).
			WithSceneItemId(itemID).
			WithSceneItemTransform(&typedefs.SceneItemTransform{
				PositionX:    x,
				PositionY:    y,
				BoundsType:   "OBS_BOUNDS_STRETCH",
				BoundsWidth:  w,
				BoundsHeight: h,
			}),
	)
	return err
}

func boolPtr(b bool) *bool { return &b }
