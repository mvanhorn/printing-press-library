package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Control OBS recording",
}

var recordStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start recording",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		status, err := client.Record.GetRecordStatus()
		if err != nil {
			return fmt.Errorf("failed to get record status: %w", err)
		}
		if status.OutputActive {
			fmt.Println("Recording is already running.")
			return nil
		}

		_, err = client.Record.StartRecord()
		if err != nil {
			return fmt.Errorf("failed to start recording: %w", err)
		}
		fmt.Println("✅ Recording started.")
		return nil
	},
}

var recordStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop recording",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		status, err := client.Record.GetRecordStatus()
		if err != nil {
			return fmt.Errorf("failed to get record status: %w", err)
		}
		if !status.OutputActive {
			fmt.Println("Recording is not running.")
			return nil
		}

		resp, err := client.Record.StopRecord()
		if err != nil {
			return fmt.Errorf("failed to stop recording: %w", err)
		}
		fmt.Printf("✅ Recording stopped. Saved to: %s\n", resp.OutputPath)
		return nil
	},
}

var recordPauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause or resume recording",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		_, err = client.Record.ToggleRecordPause()
		if err != nil {
			return fmt.Errorf("failed to toggle record pause: %w", err)
		}

		status, err := client.Record.GetRecordStatus()
		if err != nil {
			return fmt.Errorf("failed to get record status: %w", err)
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
	Short: "Show recording status",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		status, err := client.Record.GetRecordStatus()
		if err != nil {
			return fmt.Errorf("failed to get record status: %w", err)
		}

		if status.OutputActive {
			state := "🔴 RECORDING"
			if status.OutputPaused {
				state = "⏸  PAUSED"
			}
			fmt.Printf("%s\n", state)
			fmt.Printf("   Duration:  %s\n", formatMs(status.OutputDuration))
			fmt.Printf("   Timecode:  %s\n", status.OutputTimecode)
			fmt.Printf("   Bytes:     %d\n", status.OutputBytes)
		} else {
			fmt.Println("⚫ Not recording.")
		}
		return nil
	},
}

func init() {
	recordCmd.AddCommand(recordStartCmd)
	recordCmd.AddCommand(recordStopCmd)
	recordCmd.AddCommand(recordPauseCmd)
	recordCmd.AddCommand(recordStatusCmd)
}
