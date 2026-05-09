package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var streamCmd = &cobra.Command{
	Use:   "stream",
	Short: "Control OBS streaming",
}

var streamStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start streaming",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		status, err := client.Stream.GetStreamStatus()
		if err != nil {
			return fmt.Errorf("failed to get stream status: %w", err)
		}
		if status.OutputActive {
			fmt.Println("Stream is already running.")
			return nil
		}

		_, err = client.Stream.StartStream()
		if err != nil {
			return fmt.Errorf("failed to start stream: %w", err)
		}
		fmt.Println("✅ Stream started.")
		return nil
	},
}

var streamStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop streaming",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		status, err := client.Stream.GetStreamStatus()
		if err != nil {
			return fmt.Errorf("failed to get stream status: %w", err)
		}
		if !status.OutputActive {
			fmt.Println("Stream is not running.")
			return nil
		}

		_, err = client.Stream.StopStream()
		if err != nil {
			return fmt.Errorf("failed to stop stream: %w", err)
		}
		fmt.Println("✅ Stream stopped.")
		return nil
	},
}

var streamStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show streaming status",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		status, err := client.Stream.GetStreamStatus()
		if err != nil {
			return fmt.Errorf("failed to get stream status: %w", err)
		}

		if status.OutputActive {
			fmt.Printf("🔴 LIVE\n")
			fmt.Printf("   Duration:      %s\n", formatMs(status.OutputDuration))
			fmt.Printf("   Timecode:      %s\n", status.OutputTimecode)
			fmt.Printf("   Bytes sent:    %d\n", status.OutputBytes)
			fmt.Printf("   Skipped frames: %d\n", status.OutputSkippedFrames)
			fmt.Printf("   Total frames:   %d\n", status.OutputTotalFrames)
		} else {
			fmt.Println("⚫ Not streaming.")
		}
		return nil
	},
}

func init() {
	streamCmd.AddCommand(streamStartCmd)
	streamCmd.AddCommand(streamStopCmd)
	streamCmd.AddCommand(streamStatusCmd)
}

func formatMs(ms float64) string {
	total := int(ms / 1000)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
