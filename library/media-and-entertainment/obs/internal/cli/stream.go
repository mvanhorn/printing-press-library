package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/client"
	"github.com/spf13/cobra"
)

var streamCmd = &cobra.Command{
	Use:   "stream",
	Short: "Control OBS streaming — start, stop, and check status",
}

var streamStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the OBS stream output",
	Example: `  obs-pp-cli stream start
  obs-pp-cli stream start --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun() {
			fmt.Println("[dry-run] Would start streaming.")
			return nil
		}

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		status, err := c.Stream.GetStreamStatus()
		if err != nil {
			return fmt.Errorf("failed to get stream status: %w", err)
		}
		if status.OutputActive {
			if currentFormat() == "json" {
				return printJSON(map[string]any{"ok": true, "message": "already streaming"})
			}
			fmt.Println("Stream is already running.")
			return nil
		}

		_, err = c.Stream.StartStream()
		if err != nil {
			return fmt.Errorf("failed to start stream: %w\nhint: check OBS stream settings (Settings → Stream)", err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{"ok": true, "message": "stream started"})
		}
		fmt.Println("✅ Stream started.")
		return nil
	},
}

var streamStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the OBS stream output",
	Example: `  obs-pp-cli stream stop
  obs-pp-cli stream stop --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun() {
			fmt.Println("[dry-run] Would stop streaming.")
			return nil
		}

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		status, err := c.Stream.GetStreamStatus()
		if err != nil {
			return fmt.Errorf("failed to get stream status: %w", err)
		}
		if !status.OutputActive {
			if currentFormat() == "json" {
				return printJSON(map[string]any{"ok": true, "message": "not streaming"})
			}
			fmt.Println("Stream is not running.")
			return nil
		}

		_, err = c.Stream.StopStream()
		if err != nil {
			return fmt.Errorf("failed to stop stream: %w", err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{"ok": true, "message": "stream stopped"})
		}
		fmt.Println("✅ Stream stopped.")
		return nil
	},
}

var streamStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current streaming status and output stats",
	Example: `  obs-pp-cli stream status
  obs-pp-cli stream status --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		status, err := c.Stream.GetStreamStatus()
		if err != nil {
			return fmt.Errorf("failed to get stream status: %w", err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{
				"active":          status.OutputActive,
				"duration_ms":     status.OutputDuration,
				"timecode":        status.OutputTimecode,
				"bytes_sent":      status.OutputBytes,
				"skipped_frames":  status.OutputSkippedFrames,
				"total_frames":    status.OutputTotalFrames,
			})
		}

		if status.OutputActive {
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "🔴 LIVE")
			fmt.Fprintf(w, "  Duration:\t%s\n", formatMs(status.OutputDuration))
			fmt.Fprintf(w, "  Timecode:\t%s\n", status.OutputTimecode)
			fmt.Fprintf(w, "  Bytes sent:\t%.0f\n", status.OutputBytes)
			fmt.Fprintf(w, "  Skipped frames:\t%.0f\n", status.OutputSkippedFrames)
			fmt.Fprintf(w, "  Total frames:\t%.0f\n", status.OutputTotalFrames)
			return w.Flush()
		}
		fmt.Println("⚫ Not streaming.")
		return nil
	},
}

func init() {
	streamCmd.AddCommand(streamStartCmd)
	streamCmd.AddCommand(streamStopCmd)
	streamCmd.AddCommand(streamStatusCmd)
}
