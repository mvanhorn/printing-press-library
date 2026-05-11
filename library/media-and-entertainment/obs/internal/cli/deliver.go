package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// deliverSession represents a completed production session ready for delivery.
type deliverSession struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Notes       string    `json:"notes,omitempty"`
	RecordedAt  time.Time `json:"recorded_at"`
	OutputPath  string    `json:"output_path,omitempty"`
	Status      string    `json:"status"`
}

// deliverStorePath returns the path to the deliver log file.
func deliverStorePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".obs-pp", "deliver.json")
}

// loadDeliveries reads the deliver log from disk.
func loadDeliveries() ([]deliverSession, error) {
	path := deliverStorePath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sessions []deliverSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("deliver log corrupt: %w\nhint: delete %s to reset", err, path)
	}
	return sessions, nil
}

// saveDeliveries writes the deliver log to disk.
func saveDeliveries(sessions []deliverSession) error {
	path := deliverStorePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

var (
	deliverTitle      string
	deliverNotes      string
	deliverOutputPath string
)

// deliverCmd is the top-level command for post-production delivery management.
var deliverCmd = &cobra.Command{
	Use:   "deliver",
	Short: "Log completed productions and manage delivery status",
	Long: `Track post-production deliveries for each recorded session.
After recording, use 'deliver' to log the session with title, notes, and output path.

Delivery records are stored in ~/.obs-pp/deliver.json.`,
}

var deliverCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Log a completed production session for delivery",
	Long: `Record a completed production with title, notes, and optional output file path.
Useful for tracking which recordings have been processed, exported, or handed off.`,
	Args:  cobra.ExactArgs(1),
	Example: `  obs-pp-cli deliver create "Episode 5"
  obs-pp-cli deliver create "Episode 5" --notes "Great energy, check audio at 12:30"
  obs-pp-cli deliver create "Interview" --output /path/to/recording.mp4
  obs-pp-cli deliver create "Podcast EP10" --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]
		notes := deliverNotes
		outputPath := deliverOutputPath

		if isDryRun() {
			fmt.Printf("[dry-run] Would log delivery: %s\n", title)
			return nil
		}

		sessions, err := loadDeliveries()
		if err != nil {
			return err
		}

		id := fmt.Sprintf("deliver-%d", time.Now().UnixMilli())
		session := deliverSession{
			ID:         id,
			Title:      title,
			Notes:      notes,
			RecordedAt: time.Now().UTC(),
			OutputPath: outputPath,
			Status:     "pending",
		}
		sessions = append(sessions, session)

		if err := saveDeliveries(sessions); err != nil {
			return fmt.Errorf("failed to save delivery log: %w", err)
		}

		if currentFormat() == "json" {
			return printJSON(session)
		}
		fmt.Printf("✅ Delivery logged: %s\n", title)
		fmt.Printf("   ID:     %s\n", id)
		fmt.Printf("   Status: pending\n")
		if outputPath != "" {
			fmt.Printf("   File:   %s\n", outputPath)
		}
		return nil
	},
}

var deliverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all logged production deliveries",
	Example: `  obs-pp-cli deliver list
  obs-pp-cli deliver list --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessions, err := loadDeliveries()
		if err != nil {
			return err
		}

		if currentFormat() == "json" {
			if sessions == nil {
				sessions = []deliverSession{}
			}
			return printJSON(map[string]any{"count": len(sessions), "sessions": sessions})
		}

		if len(sessions) == 0 {
			fmt.Println("No deliveries logged yet.")
			fmt.Println("Use: obs-pp-cli deliver create --title \"Episode 1\"")
			return nil
		}

		w := newTabWriter(cmd.OutOrStdout())
		fmt.Fprintf(w, "ID\tTITLE\tSTATUS\tDATE\n")
		for _, s := range sessions {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				s.ID[:14],
				truncate(s.Title, 30),
				s.Status,
				s.RecordedAt.Format("2006-01-02 15:04"),
			)
		}
		return w.Flush()
	},
}

func init() {
	deliverCreateCmd.Flags().StringVar(&deliverNotes, "deliver-notes", "", "Production notes for delivery")
	deliverCreateCmd.Flags().StringVar(&deliverOutputPath, "output", "", "Path to the recording file")

	deliverCmd.AddCommand(deliverCreateCmd)
	deliverCmd.AddCommand(deliverListCmd)
}
