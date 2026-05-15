package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/trainual/internal/store"
)

type contentAuditResult struct {
	SubjectID  string   `json:"subject_id"`
	Name       string   `json:"name"`
	TopicCount int      `json:"topic_count"`
	TestCount  int      `json:"test_count"`
	UserCount  int      `json:"user_count"`
	Flags      []string `json:"flags,omitempty"`
}

func newContentAuditCmd(flags *rootFlags) *cobra.Command {
	var showEmpty, showUntested, showOrphaned bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "content-audit",
		Short: "List all curriculums with course counts, test counts, and enrollment — flag quality issues",
		Example: strings.Trim(`
  trainual-pp-cli content-audit --show-empty --show-untested --json
  trainual-pp-cli content-audit --show-orphaned`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("trainual-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			results, err := runContentAudit(cmd.Context(), db, showEmpty, showUntested, showOrphaned)
			if err != nil {
				return err
			}
			jsonData, err := json.Marshal(results)
			if err != nil {
				return fmt.Errorf("marshaling results: %w", err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), jsonData, flags)
		},
	}
	cmd.Flags().BoolVar(&showEmpty, "show-empty", false, "Show only curriculums with 0 courses")
	cmd.Flags().BoolVar(&showUntested, "show-untested", false, "Show only curriculums with courses but 0 tests")
	cmd.Flags().BoolVar(&showOrphaned, "show-orphaned", false, "Show only curriculums with 0 enrolled users")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func runContentAudit(ctx context.Context, db *store.Store, showEmpty, showUntested, showOrphaned bool) ([]contentAuditResult, error) {
	subjects, err := db.List("subjects", 0)
	if err != nil {
		return nil, fmt.Errorf("querying subjects: %w", err)
	}
	topics, err := db.List("topics", 0)
	if err != nil {
		return nil, fmt.Errorf("querying topics: %w", err)
	}
	tests, err := db.List("tests", 0)
	if err != nil {
		return nil, fmt.Errorf("querying tests: %w", err)
	}

	topicCounts := map[string]int{}
	for _, t := range topics {
		var topic struct {
			SubjectID string `json:"subject_id"`
		}
		if err := json.Unmarshal(t, &topic); err != nil {
			continue
		}
		topicCounts[topic.SubjectID]++
	}

	testCounts := map[string]int{}
	for _, t := range tests {
		var test struct {
			SubjectID string `json:"subject_id"`
		}
		if err := json.Unmarshal(t, &test); err != nil {
			continue
		}
		testCounts[test.SubjectID]++
	}

	// Count enrolled users per subject from subject data
	var results []contentAuditResult
	for _, s := range subjects {
		var subj struct {
			ID            string            `json:"id"`
			Name          string            `json:"name"`
			AssignedUsers []json.RawMessage `json:"assigned_users"`
		}
		if err := json.Unmarshal(s, &subj); err != nil {
			continue
		}

		tc := topicCounts[subj.ID]
		tsc := testCounts[subj.ID]
		uc := len(subj.AssignedUsers)

		var f []string
		if tc == 0 {
			f = append(f, "EMPTY")
		}
		if tc > 0 && tsc == 0 {
			f = append(f, "UNTESTED")
		}
		if uc == 0 {
			f = append(f, "ORPHANED")
		}

		filtering := showEmpty || showUntested || showOrphaned
		if filtering {
			match := false
			for _, flag := range f {
				if (showEmpty && flag == "EMPTY") ||
					(showUntested && flag == "UNTESTED") ||
					(showOrphaned && flag == "ORPHANED") {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		results = append(results, contentAuditResult{
			SubjectID:  subj.ID,
			Name:       subj.Name,
			TopicCount: tc,
			TestCount:  tsc,
			UserCount:  uc,
			Flags:      f,
		})
	}
	return results, nil
}
