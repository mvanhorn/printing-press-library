// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
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

type reachDestinationAPI struct {
	Airport  string `json:"airport"`
	Economy  *int   `json:"economy"`
	Premium  *int   `json:"premium"`
	Business *int   `json:"business"`
	First    *int   `json:"first"`
}
type reachAPIResponse struct {
	Success      bool                  `json:"success"`
	Origin       string                `json:"origin_airport"`
	Destinations []reachDestinationAPI `json:"destinations"`
}
type reachEvidence struct {
	Rows       int     `json:"rows"`
	NextDate   string  `json:"next_date,omitempty"`
	MinMiles   float64 `json:"min_miles,omitempty"`
	DirectRows int     `json:"direct_rows"`
	LastSynced string  `json:"last_synced,omitempty"`
}
type reachLiveCheck struct {
	NextDate string  `json:"next_date,omitempty"`
	Miles    float64 `json:"miles,omitempty"`
}
type reachResult struct {
	Airport        string          `json:"airport"`
	Miles          int             `json:"miles"`
	LocalEvidence  *reachEvidence  `json:"local_evidence"`
	LiveCheck      *reachLiveCheck `json:"live_check"`
	LiveCheckError string          `json:"live_check_error,omitempty"`
}
type reachEnvelope struct {
	Origin       string        `json:"origin"`
	Cabin        string        `json:"cabin"`
	MaxMileage   int           `json:"max_mileage"`
	Source       string        `json:"source"`
	Destinations []reachResult `json:"destinations"`
	Warnings     []string      `json:"warnings,omitempty"`
}

func newNovelReachCmd(flags *rootFlags) *cobra.Command {
	var origin, cabin, flagDB string
	var maxMileage, top int
	var confirmLive bool
	cmd := &cobra.Command{
		Use: "reach", Short: "Discover where your miles can take you nonstop from one airport, ranked by cost and cross-checked against real dated seats.",
		Long:        "Use this command to discover which destinations are reachable nonstop from one origin airport, ranked by mileage cost. At most 50 destinations are returned, and --confirm-live checks at most 10 destinations without local evidence (zero during dogfood runs). Mileage values are Seats.aero's own cross-program floor per cabin from /destinations, passed through unmodified; an implausibly low figure (for example under 1,000 miles transatlantic) is upstream data, so cross-check with --confirm-live or 'awards' before trusting it. Do NOT use this to filter already-synced availability for a route you already know; use 'direct-scan' or 'calendar' instead.",
		Example:     "  seats-aero-pp-cli reach --origin JFK --cabin business --max-mileage 90000 --top 10 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--origin=JFK;--cabin=business;--top=3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reach")
			}
			origin = strings.ToUpper(strings.TrimSpace(origin))
			cabin = strings.ToLower(strings.TrimSpace(cabin))
			if origin == "" {
				return novelUsageError(cmd, flags, fmt.Errorf("--origin is required"))
			}
			prefixes := map[string]string{"economy": "y", "premium": "w", "business": "j", "first": "f"}
			prefix, ok := prefixes[cabin]
			if !ok {
				return novelUsageError(cmd, flags, fmt.Errorf("--cabin must be one of economy, premium, business, first"))
			}
			if maxMileage < 0 {
				return novelUsageError(cmd, flags, fmt.Errorf("--max-mileage must be zero or greater"))
			}
			if top <= 0 {
				return novelUsageError(cmd, flags, fmt.Errorf("--top must be greater than zero"))
			}
			if top > 50 {
				return novelUsageError(cmd, flags, fmt.Errorf("--top must be 50 or less"))
			}
			if flags.dataSource == "local" {
				return novelUsageError(cmd, flags, fmt.Errorf("reach has no local equivalent; use --data-source auto or live"))
			}
			if cliutil.IsDogfoodEnv() && top > 3 {
				top = 3
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			path := resolveNovelDBPath(flagDB)
			var db *store.Store
			var c *client.Client
			candidates := make([]reachDestinationAPI, 0)
			{
				var err error
				c, err = flags.newClient()
				if err != nil {
					return err
				}
				data, err := c.Get(ctx, "/destinations", map[string]string{"origin_airport": origin})
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				var response reachAPIResponse
				if err = json.Unmarshal(data, &response); err != nil {
					return fmt.Errorf("decode destinations: %w", err)
				}
				candidates = response.Destinations
				if flags.dataSource != "live" {
					db, err = openNovelStoreAt(ctx, path)
					if err != nil {
						return err
					}
					if db != nil {
						defer db.Close()
						if !hintIfUnsynced(cmd, db, "availability") {
							hintIfStale(cmd, db, "availability", flags.maxAge)
						}
					}
				}
			}
			results := make([]reachResult, 0)
			for _, d := range candidates {
				cost := reachCabinCost(d, cabin)
				if cost == nil || (maxMileage > 0 && *cost > maxMileage) {
					continue
				}
				results = append(results, reachResult{Airport: strings.ToUpper(d.Airport), Miles: *cost})
			}
			sort.Slice(results, func(i, j int) bool {
				if results[i].Miles == results[j].Miles {
					return results[i].Airport < results[j].Airport
				}
				return results[i].Miles < results[j].Miles
			})
			if len(results) > top {
				results = results[:top]
			}
			if db != nil {
				for i := range results {
					ev, err := readReachEvidence(ctx, db, origin, results[i].Airport, prefix)
					if err != nil {
						return err
					}
					results[i].LocalEvidence = ev
				}
			}
			warnings := make([]string, 0)
			if confirmLive && !cliutil.IsDogfoodEnv() {
				if c == nil {
					var err error
					c, err = flags.newClient()
					if err != nil {
						return err
					}
				}
				checks := 0
				for i := range results {
					if results[i].LocalEvidence == nil {
						if checks >= 10 {
							break
						}
						check, err := checkReachLive(ctx, c, origin, results[i].Airport, cabin)
						if err != nil {
							results[i].LiveCheckError = err.Error()
							warnings = append(warnings, fmt.Sprintf("live check for %s failed: %v", results[i].Airport, err))
							break
						}
						results[i].LiveCheck = check
						checks++
					}
				}
			}
			env := reachEnvelope{Origin: origin, Cabin: cabin, MaxMileage: maxMileage, Source: "live", Destinations: results, Warnings: warnings}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				raw, err := json.Marshal(env)
				if err != nil {
					return err
				}
				meta := novelLocalMeta(db)
				meta["source"] = "live"
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, meta)
			}
			return printReachTable(cmd, results)
		},
	}
	cmd.Flags().StringVar(&origin, "origin", "", "Origin IATA airport code (required).")
	cmd.Flags().StringVar(&cabin, "cabin", "business", "Cabin class: economy, premium, business, or first.")
	cmd.Flags().IntVar(&maxMileage, "max-mileage", 0, "Maximum mileage cost (0 means no limit).")
	cmd.Flags().IntVar(&top, "top", 10, "Maximum destinations to return (hard cap: 50).")
	cmd.Flags().BoolVar(&confirmLive, "confirm-live", false, "Check one live dated result for at most 10 destinations without local evidence; disabled during dogfood runs.")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite store (default: ~/.local/share/seats-aero-pp-cli/data.db)")
	return cmd
}

func reachCabinCost(d reachDestinationAPI, c string) *int {
	switch c {
	case "economy":
		return d.Economy
	case "premium":
		return d.Premium
	case "business":
		return d.Business
	default:
		return d.First
	}
}
func readReachEvidence(ctx context.Context, db *store.Store, o, d, p string) (*reachEvidence, error) {
	q, args := buildReachEvidenceQuery(o, d, p, time.Now().UTC())
	var rows, direct sql.NullInt64
	var date, last sql.NullString
	var miles sql.NullFloat64
	err := db.DB().QueryRowContext(ctx, q, args...).Scan(&rows, &date, &miles, &direct, &last)
	if err != nil {
		return nil, err
	}
	if !rows.Valid || rows.Int64 == 0 {
		return nil, nil
	}
	return &reachEvidence{Rows: int(rows.Int64), NextDate: date.String, MinMiles: miles.Float64, DirectRows: int(direct.Int64), LastSynced: last.String}, nil
}

func buildReachEvidenceQuery(o, d, p string, now time.Time) (string, []any) {
	q := fmt.Sprintf(`SELECT COUNT(*),MIN(substr(date,1,10)),MIN(NULLIF(%s_mileage_cost_raw,0)),SUM(CASE WHEN %s_direct=1 THEN 1 ELSE 0 END),MAX(synced_at) FROM availability_all WHERE json_extract(data,'$.Route.OriginAirport')=? AND json_extract(data,'$.Route.DestinationAirport')=? AND %s_available=1 AND date >= ?`, p, p, p)
	return q, []any{o, d, now.UTC().Format("2006-01-02")}
}
func checkReachLive(ctx context.Context, c *client.Client, o, d, cabin string) (*reachLiveCheck, error) {
	data, err := c.Get(ctx, "/search", map[string]string{"origin_airport": o, "destination_airport": d, "cabins": cabin, "take": "1", "order_by": "lowest_mileage"})
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if json.Unmarshal(data, &raw) != nil {
		return nil, nil
	}
	items, _ := raw["data"].([]any)
	if len(items) == 0 {
		items, _ = raw["results"].([]any)
	}
	if len(items) == 0 {
		return nil, nil
	}
	m, _ := items[0].(map[string]any)
	check := &reachLiveCheck{}
	if v, ok := m["Date"].(string); ok {
		check.NextDate = v
		if len(check.NextDate) > 10 {
			check.NextDate = check.NextDate[:10]
		}
	}
	field := map[string]string{"economy": "YMileageCostRaw", "premium": "WMileageCostRaw", "business": "JMileageCostRaw", "first": "FMileageCostRaw"}[cabin]
	if v, ok := m[field].(float64); ok {
		check.Miles = v
	}
	return check, nil
}
func printReachTable(cmd *cobra.Command, rs []reachResult) error {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "AIRPORT\tMILES\tLOCAL ROWS\tNEXT DATE\tMIN MILES")
	for _, r := range rs {
		rows, next, min := "-", "-", "-"
		if r.LocalEvidence != nil {
			rows = fmt.Sprint(r.LocalEvidence.Rows)
			if r.LocalEvidence.NextDate != "" {
				next = r.LocalEvidence.NextDate
			}
			if r.LocalEvidence.MinMiles > 0 {
				min = fmt.Sprintf("%g", r.LocalEvidence.MinMiles)
			}
		}
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n", r.Airport, r.Miles, rows, next, min)
	}
	return tw.Flush()
}
