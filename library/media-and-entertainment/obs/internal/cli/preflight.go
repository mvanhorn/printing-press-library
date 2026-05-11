package cli

import (
	"fmt"
	"os"

	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/client"
	"github.com/spf13/cobra"
)

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Run pre-show readiness checks before going live",
	Long: `Runs a comprehensive pre-show checklist:

  ✅ OBS connection
  ✅ Current scene has visible sources
  ✅ Microphones are not muted
  ✅ Stream/record not already running

Exits with code 0 if all checks pass.
Exits with code 5 if any check fails (code: 5).
Use this in automated workflows to gate going live.`,
	Example: `  obs-pp-cli preflight
  obs-pp-cli preflight --format=json
  obs-pp-cli preflight && obs-pp-cli stream start`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		allGood := true

		type checkResult struct {
			Name   string `json:"name"`
			OK     bool   `json:"ok"`
			Detail string `json:"detail,omitempty"`
		}
		var results []checkResult

		check := func(ok bool, label, detail string) {
			icon := "✅"
			if !ok {
				icon = "❌"
				allGood = false
			}
			if currentFormat() != "json" {
				if detail != "" {
					fmt.Printf("%s  %-24s %s\n", icon, label, detail)
				} else {
					fmt.Printf("%s  %s\n", icon, label)
				}
			}
			results = append(results, checkResult{Name: label, OK: ok, Detail: detail})
		}

		if currentFormat() != "json" {
			fmt.Println("OBS Preflight Check")
			fmt.Println("─────────────────────────────────────")
		}

		// OBS version + connection
		version, vErr := c.General.GetVersion()
		if vErr != nil {
			check(false, "OBS connection", "unreachable")
			goto summary
		}
		check(true, "OBS connection",
			fmt.Sprintf("OBS %s / WS %s", version.ObsVersion, version.ObsWebSocketVersion))

		// Current scene + visible sources
		{
			sceneList, err := c.Scenes.GetSceneList()
			if err != nil {
				check(false, "Scene list", "failed")
				goto summary
			}
			check(true, "Current scene", sceneList.CurrentProgramSceneName)

			items, err := c.SceneItems.GetSceneItemList(
				sceneitems.NewGetSceneItemListParams().WithSceneName(sceneList.CurrentProgramSceneName),
			)
			if err == nil {
				visible := 0
				for _, item := range items.SceneItems {
					if item.SceneItemEnabled {
						visible++
					}
				}
				check(visible > 0, "Visible sources",
					fmt.Sprintf("%d visible / %d total", visible, len(items.SceneItems)))
			}
		}

		// Microphone mute check
		{
			allInputs, err := c.Inputs.GetInputList()
			if err == nil {
				muted := []string{}
				for _, input := range allInputs.Inputs {
					kind := fmt.Sprintf("%v", input.InputKind)
					if kind == "coreaudio_input_capture" || kind == "pulse_input_capture" || kind == "wasapi_input_capture" {
						muteResp, err := c.Inputs.GetInputMute(
							inputs.NewGetInputMuteParams().WithInputName(input.InputName),
						)
						if err == nil && muteResp.InputMuted {
							muted = append(muted, input.InputName)
						}
					}
				}
				if len(muted) > 0 {
					check(false, "Microphone(s)", fmt.Sprintf("MUTED: %v", muted))
				} else {
					check(true, "Microphone(s)", "not muted")
				}
			}
		}

		// Stream status
		{
			streamStatus, _ := c.Stream.GetStreamStatus()
			if streamStatus != nil && streamStatus.OutputActive {
				check(false, "Stream", "already live")
			} else {
				check(true, "Stream", "ready")
			}
		}

		// Record status
		{
			recStatus, _ := c.Record.GetRecordStatus()
			if recStatus != nil && recStatus.OutputActive {
				check(false, "Recording", "already running")
			} else {
				check(true, "Recording", "ready")
			}
		}

	summary:
		if currentFormat() == "json" {
			return printJSON(map[string]any{"ok": allGood, "checks": results})
		}

		fmt.Println("─────────────────────────────────────")
		if allGood {
			fmt.Println("✅ All checks passed. You're good to go.")
			return nil
		}
		fmt.Println("⚠️  Issues found. Review above before going live.")
		os.Exit(ExitPreflight) // code: 5
		return nil
	},
}
