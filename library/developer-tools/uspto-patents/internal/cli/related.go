package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/uspto-patents/internal/store"
	"github.com/spf13/cobra"
)

// relatedApp represents a locally-cached application that shares dimensions with the target.
type relatedApp struct {
	ApplicationNumber string `json:"applicationNumber"`
	Assignee          string `json:"assignee,omitempty"`
	ArtUnit           string `json:"artUnit,omitempty"`
	Inventor          string `json:"inventor,omitempty"`
	MatchDimensions   string `json:"matchDimensions"`
	OverlapScore      int    `json:"overlapScore"`
}

func newRelatedCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "related <applicationNumber>",
		Short: "Find related patents in local cache by shared dimensions",
		Long: `Searches the local SQLite store for patents that share the same
art unit, assignee, or first inventor as the target application.
Results are ranked by number of matching dimensions (3 = highest).`,
		Example: strings.Trim(`
  uspto-patents-pp-cli related 14412875
  uspto-patents-pp-cli related 14412875 --json
  uspto-patents-pp-cli related 14412875 --limit 20`, "\n"),
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

			appNum := args[0]

			// Open local store read-only
			dbPath := defaultDBPath("uspto-patents-pp-cli")
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'uspto-patents-pp-cli sync' first.", err)
			}
			defer db.Close()

			// Get the target application's dimensions
			targetData, err := db.Get("patent", appNum)
			if err != nil || targetData == nil {
				// Try via resources table
				targetData, err = getResourceByAppNum(db, appNum)
				if err != nil || targetData == nil {
					return fmt.Errorf("application %s not found in local store. Run 'uspto-patents-pp-cli sync' first", appNum)
				}
			}

			var targetObj map[string]interface{}
			if json.Unmarshal(targetData, &targetObj) != nil {
				return fmt.Errorf("invalid data for application %s in local store", appNum)
			}

			targetArtUnit := strings.ToLower(extractStringField(targetObj, "groupArtUnitNumber", "artUnit", "groupArtUnit"))
			targetAssignee := strings.ToLower(extractStringField(targetObj, "firstApplicantName", "applicantName", "assignee"))
			targetInventor := strings.ToLower(extractStringField(targetObj, "firstInventorName", "inventorName", "inventor"))

			if targetArtUnit == "" && targetAssignee == "" && targetInventor == "" {
				return fmt.Errorf("application %s has no art unit, assignee, or inventor data to match against", appNum)
			}

			// Query all patent resources and find matches
			rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = 'patent'`)
			if err != nil {
				return fmt.Errorf("querying local store: %w", err)
			}
			defer rows.Close()

			var related []relatedApp
			for rows.Next() {
				var dataStr string
				if rows.Scan(&dataStr) != nil {
					continue
				}
				var obj map[string]interface{}
				if json.Unmarshal([]byte(dataStr), &obj) != nil {
					continue
				}

				num := extractStringField(obj, "applicationNumberText", "applicationNumber")
				if num == "" || num == appNum {
					continue
				}

				score := 0
				var dims []string

				if targetArtUnit != "" {
					au := strings.ToLower(extractStringField(obj, "groupArtUnitNumber", "artUnit", "groupArtUnit"))
					if au != "" && au == targetArtUnit {
						score++
						dims = append(dims, "artUnit")
					}
				}
				if targetAssignee != "" {
					a := strings.ToLower(extractStringField(obj, "firstApplicantName", "applicantName", "assignee"))
					if a != "" && strings.Contains(a, targetAssignee) {
						score++
						dims = append(dims, "assignee")
					}
				}
				if targetInventor != "" {
					inv := strings.ToLower(extractStringField(obj, "firstInventorName", "inventorName", "inventor"))
					if inv != "" && inv == targetInventor {
						score++
						dims = append(dims, "inventor")
					}
				}

				if score > 0 {
					related = append(related, relatedApp{
						ApplicationNumber: num,
						Assignee:          extractStringField(obj, "firstApplicantName", "applicantName", "assignee"),
						ArtUnit:           extractStringField(obj, "groupArtUnitNumber", "artUnit", "groupArtUnit"),
						Inventor:          extractStringField(obj, "firstInventorName", "inventorName", "inventor"),
						MatchDimensions:   strings.Join(dims, ","),
						OverlapScore:      score,
					})
				}
			}

			// Sort by score descending
			sort.Slice(related, func(i, j int) bool {
				return related[i].OverlapScore > related[j].OverlapScore
			})

			if limit > 0 && len(related) > limit {
				related = related[:limit]
			}

			if len(related) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no related patents found in local store for %s\n", appNum)
				return nil
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), related, flags)
			}

			// Table output
			headers := []string{"AppNumber", "Assignee", "ArtUnit", "Inventor", "MatchDimensions", "Score"}
			rows2 := make([][]string, len(related))
			for i, r := range related {
				rows2[i] = []string{
					r.ApplicationNumber,
					truncate(r.Assignee, 30),
					r.ArtUnit,
					truncate(r.Inventor, 20),
					r.MatchDimensions,
					fmt.Sprintf("%d", r.OverlapScore),
				}
			}
			return flags.printTable(cmd, headers, rows2)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of related patents to return")
	return cmd
}

// getResourceByAppNum searches the resources table for a patent by application number.
func getResourceByAppNum(db *store.Store, appNum string) (json.RawMessage, error) {
	rows, err := db.Query(
		`SELECT data FROM resources WHERE resource_type = 'patent' AND (
			id = ? OR
			json_extract(data, '$.applicationNumberText') = ? OR
			json_extract(data, '$.applicationNumber') = ?
		) LIMIT 1`,
		appNum, appNum, appNum,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		var dataStr string
		if rows.Scan(&dataStr) == nil {
			return json.RawMessage(dataStr), nil
		}
	}
	return nil, nil
}
