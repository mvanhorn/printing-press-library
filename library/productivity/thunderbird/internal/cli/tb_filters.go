// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if parent := tbFindChild(root, "filters"); parent != nil {
			addNovelCommandIfAbsent(parent, newTBFiltersListCmd(flags))
		}
	})
}

// tbFilterDoc is the stored JSON shape of a filter rule.
type tbFilterDoc struct {
	ID          string                   `json:"id"`
	Account     string                   `json:"account"`
	AccountName string                   `json:"account_name"`
	Index       int                      `json:"index"`
	Name        string                   `json:"name"`
	Enabled     bool                     `json:"enabled"`
	Type        string                   `json:"type"`
	Actions     []tbprofile.FilterAction `json:"actions"`
	Action      string                   `json:"action"`
	ActionValue string                   `json:"action_value"`
	Conditions  []tbprofile.FilterTerm   `json:"conditions"`
	Condition   string                   `json:"condition"`
	MatchType   string                   `json:"match_type"`
}

type tbFilterRow struct {
	ID             string                   `json:"id"`
	Account        string                   `json:"account"`
	AccountName    string                   `json:"account_name"`
	Index          int                      `json:"index"`
	Name           string                   `json:"name"`
	Enabled        bool                     `json:"enabled"`
	Action         string                   `json:"action"`
	Target         string                   `json:"target"`
	Actions        []tbprofile.FilterAction `json:"actions"`
	MatchType      string                   `json:"match_type"`
	Summary        string                   `json:"condition_summary"`
	Condition      string                   `json:"condition"`
	Terms          int                      `json:"terms"`
	SupportedTerms int                      `json:"supported_terms"`
}

// tbConditionSummary renders terms as "field op value" joined by AND/OR.
func tbConditionSummary(matchType string, terms []tbprofile.FilterTerm) string {
	if strings.EqualFold(matchType, "ALL") || len(terms) == 0 {
		return "all messages"
	}
	parts := make([]string, 0, len(terms))
	for _, t := range terms {
		parts = append(parts, strings.TrimSpace(t.Field+" "+t.Op+" "+t.Value))
	}
	return strings.Join(parts, " "+strings.ToUpper(matchType)+" ")
}

func tbFilterRowFromDoc(d tbFilterDoc) tbFilterRow {
	r := tbFilterRow{
		ID: d.ID, Account: d.Account, AccountName: d.AccountName, Index: d.Index, Name: d.Name, Enabled: d.Enabled,
		Action: d.Action, Target: d.ActionValue, Actions: d.Actions, MatchType: d.MatchType, Condition: d.Condition,
		Summary: tbConditionSummary(d.MatchType, d.Conditions), Terms: len(d.Conditions),
	}
	for _, a := range d.Actions {
		if a.Value != "" {
			r.Action, r.Target = a.Type, a.Value
			break
		}
	}
	if r.Actions == nil {
		r.Actions = []tbprofile.FilterAction{}
	}
	for _, t := range d.Conditions {
		if t.Supported {
			r.SupportedTerms++
		}
	}
	return r
}

func newTBFiltersListCmd(flags *rootFlags) *cobra.Command {
	var account string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List filter rules with action, target folder and condition summary",
		Long: `List every message filter rule of every account in execution order with
whether it is enabled, its first action with a target (e.g. Move to folder
and the folder URI), a readable condition summary, and how many of its terms
this CLI can evaluate locally (supported_terms, used by filters audit).`,
		Example: strings.Trim(`
  thunderbird-pp-cli filters list
  thunderbird-pp-cli filters list --account account1 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "filters list")
			}
			db, err := tbStoreFor(cmd, flags, "filters")
			if err != nil || db == nil {
				return err
			}
			docs, err := tbLoadDocs[tbFilterDoc](db, "filters")
			_ = db.Close()
			if err != nil {
				return err
			}
			rows := make([]tbFilterRow, 0, len(docs))
			for _, d := range docs {
				if tbAccountMatches(account, d.Account, d.AccountName) {
					rows = append(rows, tbFilterRowFromDoc(d))
				}
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Account != rows[j].Account {
					return rows[i].Account < rows[j].Account
				}
				return rows[i].Index < rows[j].Index
			})
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(tbHumanOut(cmd))
			fmt.Fprintln(tw, "ACCOUNT\t#\tNAME\tENABLED\tACTION\tTARGET\tCONDITION")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%d\t%s\t%v\t%s\t%s\t%s\n", r.AccountName, r.Index, r.Name, r.Enabled, r.Action, r.Target, tbTrunc(r.Summary, 70))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Only filters of this account (key like account1 or account name)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum filters to show (0 = all)")
	return cmd
}
