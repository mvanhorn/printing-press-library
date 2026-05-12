package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appsflyer/internal/channels"
	"github.com/mvanhorn/printing-press-library/library/marketing/appsflyer/internal/client"

	"github.com/spf13/cobra"
)

// standupWindow is one of the three time windows the standup pivot shows.
type standupWindow struct {
	Label    string  `json:"label"`
	From     string  `json:"from"`
	To       string  `json:"to"`
	Installs int     `json:"installs"`
	Cost     float64 `json:"cost"`
	Revenue  float64 `json:"revenue"`
	ROAS     float64 `json:"roas"`
}

type standupApp struct {
	AppID     string                   `json:"app_id"`
	Yesterday standupWindow            `json:"yesterday"`
	WTD       standupWindow            `json:"wtd"`
	MTD       standupWindow            `json:"mtd"`
	ByGroup   map[string]standupWindow `json:"by_channel_group,omitempty"`
}

type standupReport struct {
	GeneratedAt     string       `json:"generated_at"`
	Timezone        string       `json:"timezone"`
	BudgetUsed      int          `json:"budget_used"`
	BudgetRemaining int          `json:"budget_remaining"`
	Apps            []standupApp `json:"apps"`
	Notes           []string     `json:"notes,omitempty"`
}

type standupArgs struct {
	appIDs       []string
	channelGroup string
	timezone     string
}

func newStandupCmd(flags *rootFlags) *cobra.Command {
	a := &standupArgs{}
	cmd := &cobra.Command{
		Use:   "standup",
		Short: "Morning pivot: yesterday vs week-to-date vs month-to-date ROAS, spend, installs across apps",
		Long: `standup is the morning command: for each app you pass via --app-id
(repeatable), it pulls the partners report for three time windows
(yesterday, week-to-date, month-to-date) and reports a side-by-side
pivot of installs, cost, revenue, and ROAS.

Optionally pass --channel-group to roll up rows by the named channel
group (social, programmatic, OEM, rewarded). The CLI tracks budget
consumption: each app × window combination is one API call, so a 2-app
standup costs 6 calls.`,
		Example: strings.Trim(`
  appsflyer-pp-cli standup --app-id id123456 --app-id id654321 --json
  appsflyer-pp-cli standup --app-id id123456 --channel-group social --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(a.appIDs) == 0 {
				if !dryRunOK(flags) {
					return fmt.Errorf("standup requires at least one --app-id")
				}
			}
			if dryRunOK(flags) && len(a.appIDs) == 0 {
				return nil
			}
			return runStandup(cmd, flags, a)
		},
	}
	cmd.Flags().StringSliceVar(&a.appIDs, "app-id", nil, "AppsFlyer app id (repeatable, required)")
	cmd.Flags().StringVar(&a.channelGroup, "channel-group", "", "Optional rollup group (social, programmatic, oem, rewarded, video)")
	cmd.Flags().StringVar(&a.timezone, "timezone", "", "Override account timezone")
	return cmd
}

func runStandup(cmd *cobra.Command, flags *rootFlags, a *standupArgs) error {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	yest := yesterday.Format("2006-01-02")
	// WTD window: Monday of the current week through yesterday inclusive.
	// On Mondays the current-week Monday lies *after* yesterday (Sunday), which
	// produces an invalid from>to window — shift back one week so WTD always
	// covers Mon→Sun of the most recent complete or in-progress week.
	weekStart := startOfWeek(now)
	if weekStart.After(yesterday) {
		weekStart = weekStart.AddDate(0, 0, -7)
	}
	weekStartStr := weekStart.Format("2006-01-02")
	// MTD window: first-of-month → yesterday. Same Monday edge case for the
	// 1st of the month: if monthStart > yesterday (i.e. today is the 1st),
	// roll back to the previous month's 1st so MTD covers the full prior month.
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	if monthStart.After(yesterday) {
		monthStart = monthStart.AddDate(0, -1, 0)
	}
	monthStartStr := monthStart.Format("2006-01-02")

	var groupMembers []string
	if a.channelGroup != "" {
		grp, err := channels.Load()
		if err != nil {
			return err
		}
		members, err := grp.Resolve(a.channelGroup)
		if err != nil {
			return err
		}
		groupMembers = members
	}

	c, err := flags.newClient()
	if err != nil {
		return err
	}

	var notes []string
	if c.Budget() != nil {
		needed := len(a.appIDs) * 3
		if remaining := c.Budget().Remaining(); needed > remaining {
			notes = append(notes, fmt.Sprintf("budget warning: standup needs %d calls but only %d remaining today", needed, remaining))
		}
	}

	apps := make([]standupApp, 0, len(a.appIDs))
	for _, appID := range a.appIDs {
		sa := standupApp{AppID: appID}
		var werr error
		sa.Yesterday, werr = aggregateWindow(c, appID, "yesterday", yest, yest, a.timezone, groupMembers)
		if werr != nil {
			notes = append(notes, fmt.Sprintf("%s yesterday failed: %v", appID, werr))
		}
		sa.WTD, werr = aggregateWindow(c, appID, "wtd", weekStartStr, yest, a.timezone, groupMembers)
		if werr != nil {
			notes = append(notes, fmt.Sprintf("%s wtd failed: %v", appID, werr))
		}
		sa.MTD, werr = aggregateWindow(c, appID, "mtd", monthStartStr, yest, a.timezone, groupMembers)
		if werr != nil {
			notes = append(notes, fmt.Sprintf("%s mtd failed: %v", appID, werr))
		}
		apps = append(apps, sa)
	}

	report := standupReport{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Timezone:    a.timezone,
		Apps:        apps,
		Notes:       notes,
	}
	if t := c.Budget(); t != nil {
		s := t.Snapshot()
		report.BudgetUsed = s.Used
		report.BudgetRemaining = t.Remaining()
	}

	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), report, flags)
	}

	tableRows := make([][]string, 0, len(apps))
	for _, sa := range apps {
		tableRows = append(tableRows, []string{
			truncStr(sa.AppID, 24),
			renderWindow(sa.Yesterday),
			renderWindow(sa.WTD),
			renderWindow(sa.MTD),
		})
	}
	if err := flags.printTable(cmd, []string{"APP_ID", "YESTERDAY", "WTD", "MTD"}, tableRows); err != nil {
		return err
	}
	if t := c.Budget(); t != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "\nbudget: %d used / %d remaining today\n", t.Snapshot().Used, t.Remaining())
	}
	for _, n := range notes {
		fmt.Fprintln(cmd.OutOrStdout(), "note:", n)
	}
	return nil
}

func renderWindow(w standupWindow) string {
	if w.Installs == 0 && w.Cost == 0 && w.Revenue == 0 {
		return "—"
	}
	return fmt.Sprintf("%s installs / $%s cost / $%s rev / %s%% ROAS",
		itoa(w.Installs), ftoa(w.Cost, 0), ftoa(w.Revenue, 0), ftoa(w.ROAS*100, 1))
}

func aggregateWindow(c *client.Client, appID, label, from, to, tz string, groupMembers []string) (standupWindow, error) {
	w := standupWindow{Label: label, From: from, To: to}
	path := fmt.Sprintf("/api/agg-data/export/app/%s/partners_report/v5", appID)
	params := map[string]string{"from": from, "to": to}
	if tz != "" {
		params["timezone"] = tz
	}
	raw, err := c.Get(path, params)
	if err != nil {
		return w, err
	}
	if c.DryRun {
		// In dry-run mode, the client returns {"dry_run": true} — no aggregation possible.
		return w, nil
	}
	rows, err := parsePullResponse(raw)
	if err != nil {
		return w, err
	}
	if len(groupMembers) > 0 {
		rows = filterPullRows(rows, "", groupMembers, "")
	}
	var totCost, totRev float64
	var totInstalls int
	for _, r := range rows {
		totInstalls += r.Installs
		totCost += r.Cost
		totRev += r.Revenue
	}
	w.Installs = totInstalls
	w.Cost = totCost
	w.Revenue = totRev
	if totCost > 0 {
		w.ROAS = totRev / totCost
	}
	return w, nil
}

// startOfWeek returns Monday of the current week, in the same location as t.
func startOfWeek(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	delta := weekday - 1
	d := t.AddDate(0, 0, -delta)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, t.Location())
}

// ensure printer for empty-by-group map encoding stays well-typed for codegen.
var _ = json.Marshal
