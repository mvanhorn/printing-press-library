// Hand-written novel feature: bounties triage.
// Group partner-submitted bounty proof by status, age, and bounty type.

package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type triageEntry struct {
	SubmissionID string `json:"submissionId"`
	BountyID     string `json:"bountyId"`
	PartnerID    string `json:"partnerId,omitempty"`
	BountyType   string `json:"bountyType,omitempty"`
	Status       string `json:"status"`
	SubmittedAt  string `json:"submittedAt,omitempty"`
	AgeDays      int    `json:"ageDays"`
}

func newBountiesTriageCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var status string
	var olderThan string

	cmd := &cobra.Command{
		Use:   "triage",
		Short: "Group partner-submitted bounty proof by status, age, and bounty type",
		Long: `Surface bounty submission backlog. Default behavior: list every submission
across every bounty, grouped by status × age. Filter with --status to focus on
the review queue, or --older-than to find rotting submissions.

Run ` + "`dub-pp-cli sync`" + ` first to populate the local store.`,
		Example: strings.Trim(`
  dub-pp-cli bounties triage --json
  dub-pp-cli bounties triage --status pending --older-than 7d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, COALESCE(bounties_id,''), COALESCE(synced_at,''), COALESCE(data,'{}')
				FROM submissions
			`)
			if err != nil {
				return fmt.Errorf("querying submissions: %w", err)
			}
			defer rows.Close()

			minAge := 0
			if olderThan != "" {
				if d, ok := parseDuration(olderThan); ok {
					minAge = int(d / (24 * time.Hour))
				}
			}

			var results []triageEntry
			seen := 0
			now := time.Now()
			for rows.Next() {
				var id, bountyID, syncedAt string
				var blob sql.NullString
				if err := rows.Scan(&id, &bountyID, &syncedAt, &blob); err != nil {
					return err
				}
				seen++
				blobBytes := []byte(blob.String)
				if !blob.Valid {
					blobBytes = []byte("{}")
				}
				submitted := extractFromData(blobBytes, "submittedAt")
				if submitted == "" {
					submitted = extractFromData(blobBytes, "createdAt")
				}
				ageDays := 0
				if t, err := time.Parse(time.RFC3339, submitted); err == nil {
					ageDays = int(now.Sub(t).Hours() / 24)
				} else if t, err := time.Parse(time.RFC3339, syncedAt); err == nil {
					ageDays = int(now.Sub(t).Hours() / 24)
				}
				st := extractFromData(blobBytes, "status")
				if st == "" {
					st = "unknown"
				}
				if status != "" && !strings.EqualFold(st, status) {
					continue
				}
				if ageDays < minAge {
					continue
				}
				results = append(results, triageEntry{
					SubmissionID: id,
					BountyID:     bountyID,
					PartnerID:    extractFromData(blobBytes, "partnerId"),
					BountyType:   extractFromData(blobBytes, "type"),
					Status:       st,
					SubmittedAt:  submitted,
					AgeDays:      ageDays,
				})
			}
			if seen == 0 {
				return hintEmptyStore("submissions")
			}
			// Sort oldest first.
			for i := 0; i < len(results); i++ {
				for j := i + 1; j < len(results); j++ {
					if results[j].AgeDays > results[i].AgeDays {
						results[i], results[j] = results[j], results[i]
					}
				}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No submissions matched the filter.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-12s %-12s %-22s %s\n", "STATUS", "AGE(D)", "TYPE", "PARTNER", "SUBMISSION")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-12d %-12s %-22s %s\n",
					r.Status, r.AgeDays, truncate(r.BountyType, 12), truncate(r.PartnerID, 22), r.SubmissionID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (pending, approved, rejected)")
	cmd.Flags().StringVar(&olderThan, "older-than", "", "Only include submissions older than this duration (e.g. 7d, 24h)")
	return cmd
}

// parseDuration accepts shorthand like 24h, 7d, 30d, 1w, 30m.
func parseDuration(s string) (time.Duration, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	var n int
	var unit string
	if _, err := fmt.Sscanf(s, "%d%s", &n, &unit); err != nil {
		return 0, false
	}
	switch strings.ToLower(unit) {
	case "h":
		return time.Duration(n) * time.Hour, true
	case "d":
		return time.Duration(n) * 24 * time.Hour, true
	case "w":
		return time.Duration(n) * 7 * 24 * time.Hour, true
	case "m":
		return time.Duration(n) * time.Minute, true
	}
	return 0, false
}
