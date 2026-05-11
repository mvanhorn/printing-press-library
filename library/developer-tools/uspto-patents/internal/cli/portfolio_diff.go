package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/uspto-patents/internal/store"
	"github.com/spf13/cobra"
)

// portfolioDiffEntry represents a single change in the portfolio diff.
type portfolioDiffEntry struct {
	ChangeType        string `json:"change_type"`
	ApplicationNumber string `json:"applicationNumber"`
	Status            string `json:"status,omitempty"`
	PreviousStatus    string `json:"previousStatus,omitempty"`
	Assignee          string `json:"assignee,omitempty"`
	FilingDate        string `json:"filingDate,omitempty"`
}

func newPortfolioDiffCmd(flags *rootFlags) *cobra.Command {
	var since string

	cmd := &cobra.Command{
		Use:   "diff <assignee>",
		Short: "Show changes in an assignee's patent portfolio since last sync",
		Long: `Compares the current state of an assignee's patent portfolio from
the live API against previously synced data in the local store.
Shows new filings, status changes, and removed entries.`,
		Example: strings.Trim(`
  uspto-patents-pp-cli portfolio diff "Google LLC"
  uspto-patents-pp-cli portfolio diff "Apple Inc." --json
  uspto-patents-pp-cli portfolio diff "Microsoft" --since 2024-01-01`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			assignee := args[0]

			// Open local store for previous data
			dbPath := defaultDBPath("uspto-patents-pp-cli")
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'uspto-patents-pp-cli sync' first to populate the local database.", err)
			}
			defer db.Close()

			// Load previously synced applications for this assignee from store
			previousApps := loadLocalAssigneeApps(db, assignee)

			// Fetch current applications from API
			c, clientErr := flags.newClient()
			if clientErr != nil {
				return clientErr
			}

			var currentApps []map[string]interface{}
			offset := 0
			pageSize := 25
			for {
				body := map[string]interface{}{
					"q": fmt.Sprintf("applicationMetaData.firstApplicantName:\"%s\"", assignee),
					"pagination": map[string]interface{}{
						"offset": offset,
						"limit":  pageSize,
					},
				}
				data, _, postErr := c.Post("/api/v1/patent/applications/search", body)
				if postErr != nil {
					return classifyAPIError(postErr, flags)
				}
				apps := extractSearchApps(data)
				if len(apps) == 0 {
					break
				}
				currentApps = append(currentApps, apps...)
				offset += len(apps)
				if len(apps) < pageSize {
					break
				}
			}

			// Build current map keyed by application number
			currentMap := map[string]map[string]interface{}{}
			for _, app := range currentApps {
				num := extractStringField(app, "applicationNumberText", "applicationNumber")
				if num != "" {
					currentMap[num] = app
				}
			}

			// Compute diff
			var diffs []portfolioDiffEntry

			// New and changed
			for num, app := range currentMap {
				status := extractStringField(app, "applicationStatus", "status")
				filingDate := extractStringField(app, "filingDate", "applicationFilingDate")

				if since != "" && filingDate != "" && filingDate < since {
					continue
				}

				prev, existed := previousApps[num]
				if !existed {
					diffs = append(diffs, portfolioDiffEntry{
						ChangeType:        "new",
						ApplicationNumber: num,
						Status:            status,
						Assignee:          assignee,
						FilingDate:        filingDate,
					})
				} else {
					prevStatus := extractStringField(prev, "applicationStatus", "status")
					if prevStatus != status {
						diffs = append(diffs, portfolioDiffEntry{
							ChangeType:        "changed",
							ApplicationNumber: num,
							Status:            status,
							PreviousStatus:    prevStatus,
							Assignee:          assignee,
							FilingDate:        filingDate,
						})
					}
				}
			}

			// Removed
			for num, prev := range previousApps {
				if _, exists := currentMap[num]; !exists {
					filingDate := extractStringField(prev, "filingDate", "applicationFilingDate")
					if since != "" && filingDate != "" && filingDate < since {
						continue
					}
					diffs = append(diffs, portfolioDiffEntry{
						ChangeType:        "removed",
						ApplicationNumber: num,
						Status:            extractStringField(prev, "applicationStatus", "status"),
						Assignee:          assignee,
						FilingDate:        filingDate,
					})
				}
			}

			if len(diffs) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no changes found for assignee %q\n", assignee)
				return nil
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), diffs, flags)
			}

			// Table output
			headers := []string{"Change", "AppNumber", "Status", "PrevStatus", "FilingDate"}
			rows := make([][]string, len(diffs))
			for i, d := range diffs {
				rows[i] = []string{d.ChangeType, d.ApplicationNumber, d.Status, d.PreviousStatus, d.FilingDate}
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Filter changes after this date (YYYY-MM-DD)")
	return cmd
}

// loadLocalAssigneeApps loads previously synced applications for an assignee from the local store.
func loadLocalAssigneeApps(db *store.Store, assignee string) map[string]map[string]interface{} {
	result := map[string]map[string]interface{}{}

	rows, err := db.Query(
		`SELECT data FROM resources WHERE resource_type = 'patent'`,
	)
	if err != nil {
		return result
	}
	defer rows.Close()

	lowerAssignee := strings.ToLower(assignee)
	for rows.Next() {
		var dataStr string
		if rows.Scan(&dataStr) != nil {
			continue
		}
		var obj map[string]interface{}
		if json.Unmarshal([]byte(dataStr), &obj) != nil {
			continue
		}
		// Check if this application belongs to the assignee
		appAssignee := strings.ToLower(extractStringField(obj, "firstApplicantName", "applicantName", "assignee"))
		if !strings.Contains(appAssignee, lowerAssignee) {
			continue
		}
		num := extractStringField(obj, "applicationNumberText", "applicationNumber")
		if num != "" {
			result[num] = obj
		}
	}

	return result
}
