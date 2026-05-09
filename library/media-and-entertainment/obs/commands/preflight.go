package commands

import (
	"fmt"
	"os"

	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/spf13/cobra"
)

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Pre-show readiness check",
	Long: `Checks OBS is ready before going live:
  - Connection
  - Current scene has visible sources
  - Microphones not muted
  - Stream/record not already running

Exits with code 1 if any check fails (useful for scripted automation).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		allGood := true
		check := func(ok bool, label, detail string) {
			icon := "✅"
			if !ok {
				icon = "❌"
				allGood = false
			}
			if detail != "" {
				fmt.Printf("%s  %-22s %s\n", icon, label, detail)
			} else {
				fmt.Printf("%s  %s\n", icon, label)
			}
		}

		fmt.Println("OBS Preflight Check")
		fmt.Println("─────────────────────────────────────")

		// Connection
		version, err := client.General.GetVersion()
		if err != nil {
			check(false, "OBS connection", "unreachable")
			goto summary
		}
		check(true, "OBS connection", fmt.Sprintf("OBS %s / WS %s", version.ObsVersion, version.ObsWebSocketVersion))

		// Current scene + visible sources
		{
			sceneList, err := client.Scenes.GetSceneList()
			if err != nil {
				check(false, "Scene list", "failed")
				goto summary
			}
			check(true, "Current scene", sceneList.CurrentProgramSceneName)

			items, err := client.SceneItems.GetSceneItemList(
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

		// Microphones muted?
		{
			allInputs, err := client.Inputs.GetInputList()
			if err == nil {
				muted := []string{}
				for _, input := range allInputs.Inputs {
					kind := fmt.Sprintf("%v", input.InputKind)
					if kind == "coreaudio_input_capture" || kind == "pulse_input_capture" || kind == "wasapi_input_capture" {
						muteResp, err := client.Inputs.GetInputMute(
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

		// Stream already live?
		{
			streamStatus, _ := client.Stream.GetStreamStatus()
			if streamStatus != nil && streamStatus.OutputActive {
				check(false, "Stream", "already live")
			} else {
				check(true, "Stream", "ready")
			}
		}

		// Already recording?
		{
			recStatus, _ := client.Record.GetRecordStatus()
			if recStatus != nil && recStatus.OutputActive {
				check(false, "Recording", "already running")
			} else {
				check(true, "Recording", "ready")
			}
		}

	summary:
		fmt.Println("─────────────────────────────────────")
		if allGood {
			fmt.Println("✅ All checks passed. You're good to go.")
			return nil
		}
		fmt.Println("⚠️  Issues found. Review above before going live.")
		os.Exit(1)
		return nil
	},
}
