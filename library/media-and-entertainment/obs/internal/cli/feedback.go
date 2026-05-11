package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// feedbackEntry records qualitative feedback about a production session.
type feedbackEntry struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id,omitempty"`
	Rating    int       `json:"rating"`
	Notes     string    `json:"notes,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// feedbackStorePath returns the path to the feedback log file.
func feedbackStorePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".obs-pp", "feedback.json")
}

// loadFeedback reads the feedback log from disk.
func loadFeedback() ([]feedbackEntry, error) {
	path := feedbackStorePath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []feedbackEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("feedback log corrupt: %w\nhint: delete %s to reset", err, path)
	}
	return entries, nil
}

// saveFeedback writes the feedback log to disk.
func saveFeedback(entries []feedbackEntry) error {
	path := feedbackStorePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

var (
	feedbackRating    int
	feedbackNotes     string
	feedbackSession   string
	feedbackTags      []string
)

// feedbackCmd manages production quality feedback.
// Feedback records help track improvements across episodes over time.
var feedbackCmd = &cobra.Command{
	Use:   "feedback",
	Short: "Log and review feedback for completed production sessions",
	Long: `Record qualitative feedback and ratings for OBS production sessions.
Useful for tracking quality improvements across episodes, identifying recurring issues,
and building a history of production notes.

Feedback records are stored in ~/.obs-pp/feedback.json.`,
}

var feedbackAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add feedback for a production session",
	Long: `Log a rating and optional notes for a completed production.
Rating is 1–5 (1 = major issues, 5 = perfect).`,
	Example: `  obs-pp-cli feedback add --rating 4 --notes "Audio dipped at 23:00, otherwise great"
  obs-pp-cli feedback add --rating 5 --tags "interview,live"
  obs-pp-cli feedback add --rating 3 --session deliver-1234567 --notes "Guest audio was too low"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rating := feedbackRating
		notes := feedbackNotes
		sessionID := feedbackSession
		tags := feedbackTags

		if rating < 1 || rating > 5 {
			return fmt.Errorf("rating must be between 1 and 5\nhint: 1=poor, 3=ok, 5=excellent")
		}

		if isDryRun() {
			fmt.Printf("[dry-run] Would log feedback: rating=%d\n", rating)
			return nil
		}

		entries, err := loadFeedback()
		if err != nil {
			return err
		}

		entry := feedbackEntry{
			ID:        fmt.Sprintf("feedback-%d", time.Now().UnixMilli()),
			SessionID: sessionID,
			Rating:    rating,
			Notes:     notes,
			Tags:      tags,
			CreatedAt: time.Now().UTC(),
		}
		entries = append(entries, entry)

		if err := saveFeedback(entries); err != nil {
			return fmt.Errorf("failed to save feedback: %w", err)
		}

		if currentFormat() == "json" {
			return printJSON(entry)
		}
		stars := ""
		for i := 0; i < rating; i++ {
			stars += "★"
		}
		for i := rating; i < 5; i++ {
			stars += "☆"
		}
		fmt.Printf("✅ Feedback logged: %s (%d/5)\n", stars, rating)
		if notes != "" {
			fmt.Printf("   Notes: %s\n", notes)
		}
		return nil
	},
}

var feedbackListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all production feedback entries",
	Example: `  obs-pp-cli feedback list
  obs-pp-cli feedback list --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := loadFeedback()
		if err != nil {
			return err
		}

		if currentFormat() == "json" {
			if entries == nil {
				entries = []feedbackEntry{}
			}
			// Calculate average rating if entries exist
			avg := 0.0
			for _, e := range entries {
				avg += float64(e.Rating)
			}
			if len(entries) > 0 {
				avg /= float64(len(entries))
			}
			return printJSON(map[string]any{
				"count":          len(entries),
				"average_rating": fmt.Sprintf("%.1f", avg),
				"entries":        entries,
			})
		}

		if len(entries) == 0 {
			fmt.Println("No feedback logged yet.")
			fmt.Println("Use: obs-pp-cli feedback add --rating 5")
			return nil
		}

		// Calculate average rating
		total := 0
		for _, e := range entries {
			total += e.Rating
		}
		avg := float64(total) / float64(len(entries))

		w := newTabWriter(cmd.OutOrStdout())
		fmt.Fprintf(w, "Feedback History (avg: %.1f/5, %d entries)\n\n", avg, len(entries))
		fmt.Fprintf(w, "RATING\tNOTES\tDATE\n")
		for _, e := range entries {
			stars := ""
			for i := 0; i < e.Rating; i++ {
				stars += "★"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				stars,
				truncate(e.Notes, 40),
				e.CreatedAt.Format("2006-01-02 15:04"),
			)
		}
		return w.Flush()
	},
}

func init() {
	feedbackAddCmd.Flags().IntVar(&feedbackRating, "rating", 0, "Session rating from 1 (poor) to 5 (excellent)")
	feedbackAddCmd.Flags().StringVar(&feedbackNotes, "notes", "", "Production notes or issues observed")
	feedbackAddCmd.Flags().StringVar(&feedbackSession, "session", "", "Session or delivery ID to associate feedback with")
	feedbackAddCmd.Flags().StringSliceVar(&feedbackTags, "tags", nil, "Comma-separated tags (e.g., interview,live,audio-issue)")
	_ = feedbackAddCmd.MarkFlagRequired("rating")

	feedbackCmd.AddCommand(feedbackAddCmd)
	feedbackCmd.AddCommand(feedbackListCmd)
}
