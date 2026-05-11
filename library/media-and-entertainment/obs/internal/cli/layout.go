package cli

import (
	"fmt"

	goobs "github.com/andreykaipov/goobs"
	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/andreykaipov/goobs/api/requests/scenes"
	"github.com/andreykaipov/goobs/api/typedefs"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/client"
	"github.com/spf13/cobra"
)

const canvasW = 1920.0
const canvasH = 1080.0

var (
	layoutInterviewName   string
	layoutSoloName        string
	layoutScreenshareName string
	layoutBRBName         string
)

var layoutCmd = &cobra.Command{
	Use:   "layout",
	Short: "Build production scene layouts — interview, solo, screenshare, brb",
	Long: `Create named OBS scenes with pre-positioned browser sources.
Creates or updates scenes without touching your existing ones.

VDO.Ninja (https://vdo.ninja) is used for remote guest video feeds.
Layouts assume a 1920x1080 canvas.`,
}

var layoutInterviewCmd = &cobra.Command{
	Use:   "interview",
	Short: "Two-person side-by-side interview layout at 1920x1080",
	Long: `Creates an "Interview" scene with two browser source slots.

  Host Camera  (left):  960x1080 at position 0,0
  Guest Camera (right): 960x1080 at position 960,0

After creating: update guest URL with 'obs-pp-cli guest add "Guest Camera" <url>'`,
	Example: `  obs-pp-cli layout interview
  obs-pp-cli layout interview --name "Live Interview"
  obs-pp-cli layout interview --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun() {
			fmt.Printf("[dry-run] Would create interview layout: %s\n", layoutInterviewName)
			return nil
		}

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		if err := ensureScene(c.Client, layoutInterviewName); err != nil {
			return err
		}
		if err := addBrowserSource(c.Client, layoutInterviewName, "Host Camera",
			"https://vdo.ninja/?push=host&cleanoutput&transparent",
			960, 1080, 0, 0); err != nil {
			return fmt.Errorf("host source: %w", err)
		}
		if err := addBrowserSource(c.Client, layoutInterviewName, "Guest Camera",
			"https://vdo.ninja/?view=guest&cleanoutput&transparent",
			960, 1080, 960, 0); err != nil {
			return fmt.Errorf("guest source: %w", err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{
				"scene":   layoutInterviewName,
				"sources": []string{"Host Camera", "Guest Camera"},
			})
		}
		fmt.Printf("✅ Layout created: %s\n", layoutInterviewName)
		fmt.Printf("   Host Camera  — left  (960x1080 @ 0,0)\n")
		fmt.Printf("   Guest Camera — right (960x1080 @ 960,0)\n")
		fmt.Printf("\nNext: obs-pp-cli guest add \"Guest Camera\" <vdo-ninja-url>\n")
		fmt.Printf("      obs-pp-cli scene switch \"%s\"\n", layoutInterviewName)
		return nil
	},
}

var layoutSoloCmd = &cobra.Command{
	Use:   "solo",
	Short: "Single host fullscreen layout at 1920x1080",
	Example: `  obs-pp-cli layout solo
  obs-pp-cli layout solo --name "Solo Stream"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun() {
			fmt.Printf("[dry-run] Would create solo layout: %s\n", layoutSoloName)
			return nil
		}

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		if err := ensureScene(c.Client, layoutSoloName); err != nil {
			return err
		}
		if err := addBrowserSource(c.Client, layoutSoloName, "Host Camera",
			"https://vdo.ninja/?push=host&cleanoutput&transparent",
			canvasW, canvasH, 0, 0); err != nil {
			return fmt.Errorf("host source: %w", err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{"scene": layoutSoloName, "sources": []string{"Host Camera"}})
		}
		fmt.Printf("✅ Layout created: %s\n", layoutSoloName)
		fmt.Printf("   Host Camera — fullscreen (1920x1080)\n")
		return nil
	},
}

var layoutScreenshareCmd = &cobra.Command{
	Use:   "screenshare",
	Short: "Guest screen share fullscreen with host camera picture-in-picture",
	Long: `Creates a screenshare layout:
  Guest Screen — fullscreen (1920x1080)
  Host Camera  — PiP bottom-right (320x180 at 1580,880)`,
	Example: `  obs-pp-cli layout screenshare
  obs-pp-cli layout screenshare --name "Demo"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun() {
			fmt.Printf("[dry-run] Would create screenshare layout: %s\n", layoutScreenshareName)
			return nil
		}

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		if err := ensureScene(c.Client, layoutScreenshareName); err != nil {
			return err
		}
		if err := addBrowserSource(c.Client, layoutScreenshareName, "Guest Screen",
			"https://vdo.ninja/?view=guestscreen&cleanoutput&transparent",
			canvasW, canvasH, 0, 0); err != nil {
			return fmt.Errorf("screen source: %w", err)
		}
		pipW, pipH := 320.0, 180.0
		if err := addBrowserSource(c.Client, layoutScreenshareName, "Host Camera",
			"https://vdo.ninja/?push=host&cleanoutput&transparent",
			pipW, pipH, canvasW-pipW-20, canvasH-pipH-20); err != nil {
			return fmt.Errorf("host PiP source: %w", err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{
				"scene":   layoutScreenshareName,
				"sources": []string{"Guest Screen", "Host Camera"},
			})
		}
		fmt.Printf("✅ Layout created: %s\n", layoutScreenshareName)
		fmt.Printf("   Guest Screen — fullscreen\n")
		fmt.Printf("   Host Camera  — PiP bottom-right (320x180)\n")
		return nil
	},
}

var layoutBRBCmd = &cobra.Command{
	Use:   "brb",
	Short: "Be Right Back empty scene for transition breaks",
	Example: `  obs-pp-cli layout brb
  obs-pp-cli layout brb --name "Break"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun() {
			fmt.Printf("[dry-run] Would create BRB scene: %s\n", layoutBRBName)
			return nil
		}

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		if err := ensureScene(c.Client, layoutBRBName); err != nil {
			return err
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{"scene": layoutBRBName, "created": true})
		}
		fmt.Printf("✅ Scene created: %s\n", layoutBRBName)
		fmt.Printf("   Add your BRB graphic as a source in OBS.\n")
		fmt.Printf("   Switch to it: obs-pp-cli scene switch \"%s\"\n", layoutBRBName)
		return nil
	},
}

func init() {
	layoutInterviewCmd.Flags().StringVar(&layoutInterviewName, "name", "Interview", "Scene name to create or update")
	layoutSoloCmd.Flags().StringVar(&layoutSoloName, "name", "Solo", "Scene name to create or update")
	layoutScreenshareCmd.Flags().StringVar(&layoutScreenshareName, "name", "Screen Share", "Scene name to create or update")
	layoutBRBCmd.Flags().StringVar(&layoutBRBName, "name", "BRB", "Scene name to create or update")

	layoutCmd.AddCommand(layoutInterviewCmd)
	layoutCmd.AddCommand(layoutSoloCmd)
	layoutCmd.AddCommand(layoutScreenshareCmd)
	layoutCmd.AddCommand(layoutBRBCmd)
}

// ensureScene creates the named scene if it does not already exist.
func ensureScene(c *goobs.Client, name string) error {
	list, err := c.Scenes.GetSceneList()
	if err != nil {
		return fmt.Errorf("failed to list scenes: %w", err)
	}
	for _, s := range list.Scenes {
		if s.SceneName == name {
			fmt.Printf("Scene %q already exists — updating sources.\n", name)
			return nil
		}
	}
	_, err = c.Scenes.CreateScene(scenes.NewCreateSceneParams().WithSceneName(name))
	return err
}

// addBrowserSource creates or updates a browser_source at the given canvas position.
func addBrowserSource(c *goobs.Client, sceneName, sourceName, url string, w, h, x, y float64) error {
	_, err := c.Inputs.CreateInput(
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
		// Source may already exist — proceed to transform update.
		fmt.Printf("   Note: source %q exists, updating transform.\n", sourceName)
	}

	itemsResp, err := c.SceneItems.GetSceneItemList(
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

	_, err = c.SceneItems.SetSceneItemTransform(
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
