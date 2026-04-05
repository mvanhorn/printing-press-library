package cli

import (
	"context"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture-pp-cli/internal/capture"
	"github.com/spf13/cobra"
)

var findCmd = &cobra.Command{
	Use:   "find <query>",
	Short: "Fuzzy search window titles to find the right capture target",
	Long: `Search across all window titles and app names for a match.
Returns the best match with window ID, app name, and bounds.
Agents use this when they know what they're looking for but not the window ID.`,
	Example: `  # Find a window by title
  agent-capture find "pull request"

  # Find by app name
  agent-capture find "preview"

  # Find with JSON output for agent consumption
  agent-capture find "terminal" --json`,
	Args: cobra.ExactArgs(1),
	RunE: runFind,
}

func runFind(cmd *cobra.Command, args []string) error {
	query := args[0]
	ctx := context.Background()

	windows, err := capture.ListWindows(ctx)
	if err != nil {
		return err
	}

	// Score and rank matches
	type match struct {
		Window capture.Window `json:"window"`
		Score  int            `json:"score"`
		Reason string         `json:"reason"`
	}

	var matches []match
	queryLower := strings.ToLower(query)

	for _, w := range windows {
		titleLower := strings.ToLower(w.Title)
		appLower := strings.ToLower(w.AppName)
		score := 0
		reason := ""

		// Exact title match
		if titleLower == queryLower {
			score = 100
			reason = "exact title match"
		} else if appLower == queryLower {
			score = 90
			reason = "exact app name match"
		} else if strings.Contains(titleLower, queryLower) {
			score = 70
			reason = "title contains query"
		} else if strings.Contains(appLower, queryLower) {
			score = 60
			reason = "app name contains query"
		} else {
			// Check individual words
			words := strings.Fields(queryLower)
			matched := 0
			for _, word := range words {
				if strings.Contains(titleLower, word) || strings.Contains(appLower, word) {
					matched++
				}
			}
			if matched > 0 {
				score = (matched * 40) / len(words)
				reason = "partial word match"
			}
		}

		if score > 0 {
			matches = append(matches, match{Window: w, Score: score, Reason: reason})
		}
	}

	// Sort by score descending
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Score > matches[i].Score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	if len(matches) == 0 {
		return errorf("no windows matching %q found", query)
	}

	if jsonOutput {
		return printJSON(matches)
	}

	// Show top matches
	infof("Found %d match(es) for %q:", len(matches), query)
	for i, m := range matches {
		if i >= 5 {
			break
		}
		infof("  [%d] %s - %s (window %d, %dx%d) - %s",
			m.Score, m.Window.AppName, m.Window.Title,
			m.Window.ID, m.Window.Width, m.Window.Height, m.Reason)
	}
	return nil
}
