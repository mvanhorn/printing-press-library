package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/client"
	"github.com/spf13/cobra"
)

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Control OBS recording — start, stop, pause, and check status",
}

var recordStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start OBS recording to the configured output path",
	Example: `  obs-pp-cli record start
  obs-pp-cli record start --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun() {
			fmt.Println("[dry-run] Would start recording.")
			return nil
		}

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		status, err := c.Record.GetRecordStatus()
		if err != nil {
			return fmt.Errorf("failed to get record status: %w", err)
		}
		if status.OutputActive {
			if currentFormat() == "json" {
				return printJSON(map[string]any{"ok": true, "message": "already recording"})
			}
			fmt.Println("Recording is already running.")
			return nil
		}

		_, err = c.Record.StartRecord()
		if err != nil {
			return fmt.Errorf("failed to start recording: %w\nhint: check OBS output settings (Settings → Output)", err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{"ok": true, "message": "recording started"})
		}
		fmt.Println("✅ Recording started.")
		return nil
	},
}

var recordStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop OBS recording and return the output file path",
	Example: `  obs-pp-cli record stop
  obs-pp-cli record stop --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun() {
			fmt.Println("[dry-run] Would stop recording.")
			return nil
		}

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		status, err := c.Record.GetRecordStatus()
		if err != nil {
			return fmt.Errorf("failed to get record status: %w", err)
		}
		if !status.OutputActive {
			if currentFormat() == "json" {
				return printJSON(map[string]any{"ok": true, "message": "not recording"})
			}
			fmt.Println("Recording is not running.")
			return nil
		}

		resp, err := c.Record.StopRecord()
		if err != nil {
			return fmt.Errorf("failed to stop recording: %w", err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{"ok": true, "output_path": resp.OutputPath})
		}
		fmt.Printf("✅ Recording stopped. Saved to: %s\n", resp.OutputPath)
		return nil
	},
}

var recordPauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Toggle recording pause — pauses if active, resumes if paused",
	Example: `  obs-pp-cli record pause
  obs-pp-cli record pause --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun() {
			fmt.Println("[dry-run] Would toggle recording pause.")
			return nil
		}

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		_, err = c.Record.ToggleRecordPause()
		if err != nil {
			return fmt.Errorf("failed to toggle record pause: %w", err)
		}

		status, err := c.Record.GetRecordStatus()
		if err != nil {
			return fmt.Errorf("failed to get record status: %w", err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{"paused": status.OutputPaused})
		}
		if status.OutputPaused {
			fmt.Println("⏸  Recording paused.")
		} else {
			fmt.Println("▶  Recording resumed.")
		}
		return nil
	},
}

var recordStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current recording status, duration, and output path",
	Example: `  obs-pp-cli record status
  obs-pp-cli record status --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		status, err := c.Record.GetRecordStatus()
		if err != nil {
			return fmt.Errorf("failed to get record status: %w", err)
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{
				"active":   status.OutputActive,
				"paused":   status.OutputPaused,
				"duration": status.OutputDuration,
				"timecode": status.OutputTimecode,
				"bytes":    status.OutputBytes,
			})
		}

		if status.OutputActive {
			state := "🔴 RECORDING"
			if status.OutputPaused {
				state = "⏸  PAUSED"
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, state)
			fmt.Fprintf(w, "  Duration:\t%s\n", formatMs(status.OutputDuration))
			fmt.Fprintf(w, "  Timecode:\t%s\n", status.OutputTimecode)
			fmt.Fprintf(w, "  Bytes:\t%.0f\n", status.OutputBytes)
			return w.Flush()
		}
		fmt.Println("⚫ Not recording.")
		return nil
	},
}

func init() {
	recordCmd.AddCommand(recordStartCmd)
	recordCmd.AddCommand(recordStopCmd)
	recordCmd.AddCommand(recordPauseCmd)
	recordCmd.AddCommand(recordStatusCmd)
}
