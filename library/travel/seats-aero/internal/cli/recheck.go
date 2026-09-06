// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// pp:method GET is deliberate: the default invocation only reads local rows and probes quota; POST /refresh happens solely behind --apply, which refuses under any harness.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
	"github.com/spf13/cobra"
)

type recheckRow struct {
	ID          string  `json:"id"`
	Date        string  `json:"date"`
	Source      string  `json:"source"`
	Origin      string  `json:"origin"`
	Destination string  `json:"destination"`
	Miles       float64 `json:"miles"`
	SyncedAt    string  `json:"synced_at"`
	Age         string  `json:"age"`
}
type recheckPlan struct {
	Mode            string        `json:"mode"`
	Cabin           string        `json:"cabin"`
	OlderThan       string        `json:"older_than"`
	Shortlist       []recheckRow  `json:"shortlist"`
	WouldRefresh    int           `json:"would_refresh"`
	Quota           *client.Quota `json:"quota"`
	Warnings        []string      `json:"warnings"`
	RefreshFailures []struct {
		AvailabilityID string `json:"availability_id"`
		Status         string `json:"status"`
	} `json:"refresh_failures,omitempty"`
	RefreshResponse json.RawMessage `json:"refresh_response,omitempty"`
}

type recheckRefreshResponse struct {
	Items []struct {
		AvailabilityID string `json:"availability_id"`
		Status         string `json:"status"`
		UpdatedAt      string `json:"updated_at"`
	} `json:"items"`
	Queued   int            `json:"queued"`
	Refunded int            `json:"refunded"`
	Counts   map[string]int `json:"counts"`
	Quota    map[string]any `json:"quota"`
}

func newNovelRecheckCmd(flags *rootFlags) *cobra.Command {
	var origin, destination, cabin, sources, olderThan, flagDB string
	var maxMileage, limit int
	var apply, ignoreQuota bool
	cmd := &cobra.Command{
		Use: "recheck", Short: "Re-verify aging award rows are still live right before booking, without blowing the daily refresh quota.",
		Long:        "Use this command to re-verify specific already-known Availability rows are still live before booking, with a quota guard. Recheck always performs one live quota probe (1 daily call), except with --data-source local. The POST itself is one daily call; refresh credits are reported only by its response, and per-item insufficient_quota statuses are surfaced after the call. Only --apply spends refresh credits. Do NOT use this to discover newly appeared availability across a route; use 'new-since' instead.",
		Example:     "  seats-aero-pp-cli recheck --origin JFK --destination NRT --cabin business --max-mileage 90000 --agent",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:method": "GET", "pp:typed-exit-codes": "0,3", "pp:happy-args": "--origin=JFK;--destination=NRT;--cabin=business;--older-than=1h"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "recheck")
			}
			if apply && flags.dataSource == "local" {
				return novelUsageError(cmd, flags, fmt.Errorf("--apply requires live data source"))
			}
			cabin = strings.ToLower(strings.TrimSpace(cabin))
			prefix, ok := map[string]string{"economy": "y", "premium": "w", "business": "j", "first": "f"}[cabin]
			if !ok {
				return novelUsageError(cmd, flags, fmt.Errorf("--cabin must be one of economy, premium, business, first"))
			}
			if maxMileage < 0 {
				return novelUsageError(cmd, flags, fmt.Errorf("--max-mileage must be zero or greater"))
			}
			if limit <= 0 || limit > 250 {
				return novelUsageError(cmd, flags, fmt.Errorf("--limit must be between 1 and 250"))
			}
			age, err := cliutil.ParseDurationLoose(olderThan)
			if err != nil || age < 0 {
				return novelUsageError(cmd, flags, fmt.Errorf("invalid --older-than %q", olderThan))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			path := resolveNovelDBPath(flagDB)
			db, err := openNovelStoreAt(ctx, path)
			if err != nil {
				return err
			}
			shortlist := make([]recheckRow, 0)
			if db == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\n%s\n", path, novelStoreMissingHint(path))
			} else {
				defer db.Close()
				if !hintIfUnsynced(cmd, db, "availability") {
					hintIfStale(cmd, db, "availability", flags.maxAge)
				}
				warnUnknownSources(ctx, cmd, db, strings.Split(sources, ","))
				shortlist, err = readRecheckRows(ctx, db, prefix, origin, destination, sources, maxMileage, time.Now().UTC().Add(-age), limit)
				if err != nil {
					return err
				}
			}
			plan := recheckPlan{Mode: "plan", Cabin: cabin, OlderThan: olderThan, Shortlist: shortlist, WouldRefresh: len(shortlist), Warnings: make([]string, 0)}
			var c *client.Client
			var qerr error
			if flags.dataSource == "local" {
				plan.Warnings = append(plan.Warnings, "quota probe skipped (--data-source local)")
			} else {
				c, err = flags.newClient()
				if err != nil {
					return err
				}
				q, probeErr := c.ProbeQuota(ctx)
				qerr = probeErr
				if qerr != nil {
					plan.Warnings = append(plan.Warnings, fmt.Sprintf("quota probe failed: %v", qerr))
				} else if q.Observed {
					plan.Quota = &q
				} else {
					plan.Warnings = append(plan.Warnings, "quota probe returned no quota observation")
				}
			}
			if !apply {
				return printRecheckPlan(cmd, flags, plan, db)
			}
			if cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "refresh availability via POST /refresh")
			}
			if len(shortlist) == 0 {
				return printRecheckPlan(cmd, flags, plan, db)
			}
			if !ignoreQuota && (qerr != nil || plan.Quota == nil || !plan.Quota.Observed) {
				return fmt.Errorf("quota unknown; refusing to spend refresh credits — pass --ignore-quota to override")
			}
			if plan.Quota != nil && plan.Quota.Observed && plan.Quota.Remaining < 2 {
				return fmt.Errorf("daily calls quota nearly exhausted (remaining=%d); refusing to call refresh", plan.Quota.Remaining)
			}
			ids := make([]string, 0, len(shortlist))
			for _, row := range shortlist {
				ids = append(ids, row.ID)
			}
			data, status, err := c.Post(ctx, "/refresh", map[string]any{"availability_ids": ids})
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if status < 200 || status >= 300 {
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("refresh availability returned HTTP %d", status), flags)
			}
			plan.RefreshResponse = json.RawMessage(data)
			var response recheckRefreshResponse
			if err := json.Unmarshal(data, &response); err != nil {
				return fmt.Errorf("decode refresh response: %w", err)
			}
			okStatuses := map[string]bool{"queued": true, "succeeded": true, "fresh": true, "processing": true}
			failureCounts := make(map[string]int)
			for _, item := range response.Items {
				status := strings.ToLower(strings.TrimSpace(item.Status))
				if okStatuses[status] {
					continue
				}
				plan.RefreshFailures = append(plan.RefreshFailures, struct {
					AvailabilityID string `json:"availability_id"`
					Status         string `json:"status"`
				}{AvailabilityID: item.AvailabilityID, Status: item.Status})
				failureCounts[status]++
			}
			plan.Mode = "applied"
			if len(plan.RefreshFailures) > 0 {
				plan.Mode = "partial"
				statuses := make([]string, 0, len(failureCounts))
				for status := range failureCounts {
					statuses = append(statuses, status)
				}
				sort.Strings(statuses)
				parts := make([]string, 0, len(statuses))
				for _, status := range statuses {
					parts = append(parts, fmt.Sprintf("%s: %d", status, failureCounts[status]))
				}
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("%d of %d rows were not refreshed (%s)", len(plan.RefreshFailures), len(response.Items), strings.Join(parts, ", ")))
			}
			if err := printRecheckPlan(cmd, flags, plan, db); err != nil {
				return err
			}
			if len(response.Items) > 0 && len(plan.RefreshFailures) == len(response.Items) {
				return notFoundErr(fmt.Errorf("all %d refresh items failed", len(response.Items)))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&origin, "origin", "", "Optional origin IATA airport code filter.")
	cmd.Flags().StringVar(&destination, "destination", "", "Optional destination IATA airport code filter.")
	cmd.Flags().StringVar(&cabin, "cabin", "business", "Cabin class: economy, premium, business, or first.")
	cmd.Flags().IntVar(&maxMileage, "max-mileage", 0, "Maximum mileage cost (0 means no limit).")
	cmd.Flags().StringVar(&sources, "sources", "", "Optional comma-separated mileage program sources.")
	cmd.Flags().StringVar(&olderThan, "older-than", "6h", "Only include rows synced before this loose duration (for example 6h or 2d).")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum rows to shortlist (hard maximum 250).")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually spend refresh credits by calling POST /refresh.")
	cmd.Flags().BoolVar(&ignoreQuota, "ignore-quota", false, "Allow --apply when the live quota cannot be observed.")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite store (default: ~/.local/share/seats-aero-pp-cli/data.db)")
	return cmd
}

func readRecheckRows(ctx context.Context, db *store.Store, prefix, origin, destination, sources string, maxMileage int, cutoff time.Time, limit int) ([]recheckRow, error) {
	query, args := buildRecheckQuery(prefix, origin, destination, sources, maxMileage, cutoff, limit)
	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	result := make([]recheckRow, 0)
	for rows.Next() {
		var r recheckRow
		var date, source, o, d sql.NullString
		var miles sql.NullFloat64
		var synced sql.NullString
		if err := rows.Scan(&r.ID, &date, &source, &o, &d, &miles, &synced); err != nil {
			_ = rows.Close()
			return nil, err
		}
		r.Date, r.Source, r.Origin, r.Destination, r.Miles = date.String, source.String, o.String, d.String, miles.Float64
		if synced.Valid {
			r.SyncedAt = synced.String
			if syncedTime, err := parseRecheckTime(synced.String); err == nil {
				r.Age = humanRecheckAge(time.Since(syncedTime))
			}
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return result, nil
}

func buildRecheckQuery(prefix, origin, destination, sources string, maxMileage int, cutoff time.Time, limit int) (string, []any) {
	query := fmt.Sprintf(`SELECT id,substr(date,1,10),source,json_extract(data,'$.Route.OriginAirport'),json_extract(data,'$.Route.DestinationAirport'),%s_mileage_cost_raw,CAST(synced_at AS TEXT) FROM availability_all WHERE %s_available=1 AND datetime(synced_at)<=datetime(?)`, prefix, prefix)
	args := []any{cutoff.UTC().Format("2006-01-02 15:04:05")}
	origin, destination = strings.ToUpper(strings.TrimSpace(origin)), strings.ToUpper(strings.TrimSpace(destination))
	if origin != "" {
		query += ` AND json_extract(data,'$.Route.OriginAirport')=?`
		args = append(args, origin)
	}
	if destination != "" {
		query += ` AND json_extract(data,'$.Route.DestinationAirport')=?`
		args = append(args, destination)
	}
	if maxMileage > 0 {
		query += fmt.Sprintf(` AND %s_mileage_cost_raw<=?`, prefix)
		args = append(args, maxMileage)
	}
	parts := make([]string, 0)
	for _, s := range strings.Split(sources, ",") {
		if s = strings.TrimSpace(s); s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) > 0 {
		query += ` AND source IN (` + strings.TrimRight(strings.Repeat("?,", len(parts)), ",") + `)`
		for _, s := range parts {
			args = append(args, s)
		}
	}
	query += ` ORDER BY datetime(synced_at) ASC, id ASC LIMIT ?`
	args = append(args, limit)
	return query, args
}

func parseRecheckTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid synced_at %q", value)
}

func humanRecheckAge(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
func printRecheckPlan(cmd *cobra.Command, flags *rootFlags, plan recheckPlan, db *store.Store) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printNovelJSON(cmd.OutOrStdout(), plan, flags, db)
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDATE\tSOURCE\tROUTE\tMILES\tAGE")
	for _, r := range plan.Shortlist {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s-%s\t%.0f\t%s\n", r.ID, r.Date, r.Source, r.Origin, r.Destination, r.Miles, r.Age)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if plan.Quota == nil {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Would refresh %d rows. Daily calls remaining: unknown. Add --apply to spend refresh credits.\n", plan.WouldRefresh)
		return err
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "Would refresh %d rows. Daily calls remaining: %d/%d (resets in %ds). Add --apply to spend refresh credits.\n", plan.WouldRefresh, plan.Quota.Remaining, plan.Quota.Limit, plan.Quota.ResetSeconds)
	return err
}
