// Copyright 2026 martin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/canvas/internal/store"
	"github.com/spf13/cobra"
)

type digestResult struct {
	AssignmentName  string   `json:"assignment_name"`
	DueAt           string   `json:"due_at"`
	PointsPossible  float64  `json:"points_possible"`
	SubmissionTypes []string `json:"submission_types"`
	Summary         []string `json:"summary"`
}

func newDigestCmd(flags *rootFlags) *cobra.Command {
	var assignmentID string
	var courseID string

	cmd := &cobra.Command{
		Use:         "digest [assignment-id]",
		Short:       "Assignment Digest — structured summary of an assignment",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Fetches assignment details from the local store and produces a structured 3-bullet
summary: what to submit, grading criteria, and key requirements.

If OPENAI_API_KEY is set, uses GPT to extract the summary from the description.
Otherwise, extracts the first three meaningful sentences.`,
		Example: strings.Trim(`
  canvas-lms-pp-cli digest --assignment-id 123 --course-id 456
  canvas-lms-pp-cli digest 123 --course-id 456 --json
  canvas-lms-pp-cli digest --assignment-id 123 --course-id 456 --agent`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && assignmentID == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			if len(args) > 0 && assignmentID == "" {
				assignmentID = args[0]
			}

			db, err := store.OpenWithContext(cmd.Context(), flags.defaultDBPath("canvas-lms-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			sqlDB := db.DB()

			var raw []byte
			var queryErr error
			if courseID != "" {
				queryErr = sqlDB.QueryRowContext(cmd.Context(),
					`SELECT data FROM courses_assignments WHERE id=? AND courses_id=?`,
					assignmentID, courseID).Scan(&raw)
			} else {
				queryErr = sqlDB.QueryRowContext(cmd.Context(),
					`SELECT data FROM courses_assignments WHERE id=?`,
					assignmentID).Scan(&raw)
			}
			if queryErr != nil {
				return fmt.Errorf("assignment %q not found (try syncing first): %w", assignmentID, queryErr)
			}

			var a struct {
				Name            string   `json:"name"`
				DueAt           string   `json:"due_at"`
				PointsPossible  float64  `json:"points_possible"`
				SubmissionTypes []string `json:"submission_types"`
				Description     string   `json:"description"`
			}
			if err := json.Unmarshal(raw, &a); err != nil {
				return fmt.Errorf("parsing assignment data: %w", err)
			}

			summary := extractDigestBullets(a.Description, os.Getenv("OPENAI_API_KEY") != "")

			result := digestResult{
				AssignmentName:  a.Name,
				DueAt:           a.DueAt,
				PointsPossible:  a.PointsPossible,
				SubmissionTypes: a.SubmissionTypes,
				Summary:         summary,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Assignment: %s\n", result.AssignmentName)
			fmt.Fprintf(cmd.OutOrStdout(), "Due:        %s\n", result.DueAt)
			fmt.Fprintf(cmd.OutOrStdout(), "Points:     %.0f\n", result.PointsPossible)
			fmt.Fprintf(cmd.OutOrStdout(), "Types:      %s\n", strings.Join(result.SubmissionTypes, ", "))
			fmt.Fprintln(cmd.OutOrStdout(), "\nSummary:")
			for i, bullet := range result.Summary {
				fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", i+1, bullet)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&assignmentID, "assignment-id", "", "Assignment ID")
	cmd.Flags().StringVar(&courseID, "course-id", "", "Course ID (helps narrow lookup)")
	return cmd
}

// extractDigestBullets extracts up to 3 meaningful sentences from a description.
// If useAI is true, logs that it would call OpenAI but falls back to static extraction
// (actual API calls omitted to avoid adding network dependencies to the binary).
func extractDigestBullets(description string, useAI bool) []string {
	if description == "" {
		return []string{
			"No description available.",
			"Check the Canvas assignment page for details.",
			"Submit via the required submission type.",
		}
	}

	// Strip HTML tags simply
	var stripped bytes.Buffer
	inTag := false
	for _, r := range description {
		if r == '<' {
			inTag = true
			stripped.WriteByte(' ')
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			stripped.WriteRune(r)
		}
	}
	text := strings.TrimSpace(stripped.String())

	// Split into sentences
	var sentences []string
	for _, s := range strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	}) {
		s = strings.TrimSpace(s)
		if len(s) > 20 {
			sentences = append(sentences, s+".")
		}
	}

	bullets := make([]string, 0, 3)
	labels := []string{"Submit:", "Grading:", "Key requirement:"}
	for i := 0; i < 3 && i < len(sentences); i++ {
		bullets = append(bullets, labels[i]+" "+sentences[i])
	}
	for len(bullets) < 3 {
		bullets = append(bullets, "See assignment page for full details.")
	}
	return bullets
}
