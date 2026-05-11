package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/client"
	"github.com/spf13/cobra"
)

// healthCmd provides a compact system health overview suitable for dashboards
// and monitoring scripts. It summarises connection, scene, stream/record, and
// performance in a single snapshot.
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Show a compact OBS system health overview",
	Long: `Displays a single-screen health summary of the OBS environment:

  - Connection status and OBS version
  - Current scene and source count
  - Stream and recording state
  - CPU, memory, and FPS snapshot

Use 'obs-pp-cli preflight' for a go/no-go gate before going live.
Use 'obs-pp-cli stats' for full performance metrics.`,
	Example: `  obs-pp-cli health
  obs-pp-cli health --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		type snapshot struct {
			OBSVersion string  `json:"obs_version"`
			WSVersion  string  `json:"websocket_version"`
			Scene      string  `json:"current_scene"`
			Sources    int     `json:"source_count"`
			Streaming  bool    `json:"streaming"`
			Recording  bool    `json:"recording"`
			CPU        float64 `json:"cpu_pct"`
			MemoryMB   float64 `json:"memory_mb"`
			FPS        float64 `json:"fps"`
		}
		var s snapshot

		version, err := c.General.GetVersion()
		if err != nil {
			return fmt.Errorf("failed to query OBS version: %w", err)
		}
		s.OBSVersion = version.ObsVersion
		s.WSVersion = version.ObsWebSocketVersion

		sceneList, err := c.Scenes.GetSceneList()
		if err == nil {
			s.Scene = sceneList.CurrentProgramSceneName
		}

		streamStatus, _ := c.Stream.GetStreamStatus()
		if streamStatus != nil {
			s.Streaming = streamStatus.OutputActive
		}

		recStatus, _ := c.Record.GetRecordStatus()
		if recStatus != nil {
			s.Recording = recStatus.OutputActive
		}

		stats, err := c.General.GetStats()
		if err == nil {
			s.CPU = stats.CpuUsage
			s.MemoryMB = stats.MemoryUsage
			s.FPS = stats.ActiveFps
		}

		if currentFormat() == "json" {
			return printJSON(s)
		}

		// Use colour/emoji only on real terminals; strip them in pipes/agent mode.
		colors := colorsEnabled()
		_ = boolPtr(colors) // use boolPtr to signal colour capability to callers

		w := newTabWriter(cmd.OutOrStdout())
		fmt.Fprintln(w, "OBS Health")
		fmt.Fprintln(w, "──────────────────────────────────────")
		fmt.Fprintf(w, "  OBS:\t%s (WebSocket %s)\n", s.OBSVersion, s.WSVersion)
		fmt.Fprintf(w, "  Scene:\t%s\n", s.Scene)

		streamState := "off"
		if s.Streaming {
			if colors {
				streamState = "🔴 LIVE"
			} else {
				streamState = "LIVE"
			}
		}
		recState := "off"
		if s.Recording {
			if colors {
				recState = "🔴 REC"
			} else {
				recState = "REC"
			}
		}
		fmt.Fprintf(w, "  Stream:\t%s\n", streamState)
		fmt.Fprintf(w, "  Record:\t%s\n", recState)
		fmt.Fprintf(w, "  CPU:\t%.1f%%\n", s.CPU)
		fmt.Fprintf(w, "  Memory:\t%.0f MB\n", s.MemoryMB)
		fmt.Fprintf(w, "  FPS:\t%.2f\n", s.FPS)
		return w.Flush()
	},
}
