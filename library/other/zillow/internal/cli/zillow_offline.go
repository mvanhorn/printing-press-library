// Copyright 2026 Hunter Veltri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/zillow/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/zillow/internal/zillowdata"
	"github.com/spf13/cobra"
)

var bridgeSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func init() {
	registerNovelCommand(addZillowOfflineCommands)
}

func addZillowOfflineCommands(root *cobra.Command, flags *rootFlags) {
	if syncCmd, _, err := root.Find([]string{"sync"}); err == nil && syncCmd != nil {
		wrapSyncForPipelineVerify(syncCmd)
		marketSync := newMarketSyncCmd(flags)
		makeMarketDryRunSafe(marketSync, flags)
		syncCmd.AddCommand(marketSync)
	}
	commands := []*cobra.Command{
		newRankCmd(flags),
		newExportMarketCmd(flags),
		newSQLCmd(flags),
		newWatchCmd(flags),
		newBridgeCmd(flags),
	}
	for _, cmd := range commands {
		makeMarketDryRunSafe(cmd, flags)
	}
	root.AddCommand(commands...)
}

func wrapSyncForPipelineVerify(syncCmd *cobra.Command) {
	run := syncCmd.RunE
	syncCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if cliutil.IsVerifyEnv() {
			dbPath, _ := cmd.Flags().GetString("db")
			if dbPath != "" {
				db, err := zillowdata.OpenDatabase(dbPath)
				if err != nil {
					return err
				}
				// ponytail: one verifier-only sentinel proves sync -> SQLite wiring;
				// real rows remain owned by `sync market`.
				_, err = db.ExecContext(cmd.Context(), `CREATE TABLE IF NOT EXISTS verify_fixture (id INTEGER PRIMARY KEY, source TEXT NOT NULL)`)
				if err == nil {
					_, err = db.ExecContext(cmd.Context(), `INSERT OR IGNORE INTO verify_fixture(id, source) VALUES (1, 'zillow-research')`)
				}
				closeErr := db.Close()
				if err != nil {
					return err
				}
				if closeErr != nil {
					return closeErr
				}
			}
		}
		if run == nil {
			return nil
		}
		return run(cmd, args)
	}
}

func marketDBPath(override string) (string, error) {
	if override != "" {
		if !filepath.IsAbs(override) {
			return "", fmt.Errorf("--db must be an absolute path")
		}
		return filepath.Clean(override), nil
	}
	dir, err := cliutil.DataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "market.db"), nil
}

func newMarketSyncCmd(flags *rootFlags) *cobra.Command {
	var metrics string
	var dbOverride string
	cmd := &cobra.Command{
		Use: "market", Short: "Sync normalized Zillow Research observations into market.db",
		Example:     `  zillow-pp-cli sync market --metrics zhvi,zori --agent`,
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var keys []string
			if strings.EqualFold(strings.TrimSpace(metrics), "all") {
				for _, dataset := range zillowdata.Datasets() {
					keys = append(keys, dataset.Key)
				}
			} else {
				keys = splitList(metrics)
			}
			if len(keys) == 0 {
				return usageErr(fmt.Errorf("no metrics selected"))
			}
			dbPath, err := marketDBPath(dbOverride)
			if err != nil {
				return err
			}
			db, err := zillowdata.OpenDatabase(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			loader, err := marketLoader(flags)
			if err != nil {
				return err
			}
			type result struct {
				Metric       string `json:"metric"`
				Regions      int    `json:"regions"`
				Observations int    `json:"observations"`
				SHA256       string `json:"sha256"`
			}
			var results []result
			for _, key := range keys {
				table, loadErr := loader.Load(cmd.Context(), key, "live")
				if loadErr != nil {
					return loadErr
				}
				if err := zillowdata.SaveTable(cmd.Context(), db, table); err != nil {
					return fmt.Errorf("saving %s: %w", key, err)
				}
				count := 0
				for _, row := range table.Rows {
					count += len(row.Values)
				}
				results = append(results, result{Metric: table.Dataset.Key, Regions: len(table.Rows), Observations: count, SHA256: table.SHA256})
			}
			return emitMarket(cmd, flags, map[string]any{"database": dbPath, "synced": results}, map[string]any{"source": "live"})
		},
	}
	cmd.Flags().StringVar(&metrics, "metrics", "all", "Comma-separated metrics or all")
	cmd.Flags().StringVar(&dbOverride, "db", "", "Absolute market database path")
	return cmd
}

func newRankCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var ascending bool
	cmd := &cobra.Command{
		Use: "rank <metric>", Args: cobra.ExactArgs(1), Short: "Rank all covered regions by latest metric value",
		Example:     `  zillow-pp-cli rank zhvi --limit 10 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<metric>=zhvi"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit <= 0 || limit > 10000 {
				return usageErr(fmt.Errorf("--limit must be 1..10000"))
			}
			loader, err := marketLoader(flags)
			if err != nil {
				return err
			}
			table, err := loader.Load(cmd.Context(), args[0], flags.dataSource)
			if err != nil {
				return err
			}
			var rows []metricValue
			for _, region := range table.Rows {
				value, valueErr := latestMetric(table, region)
				if valueErr == nil {
					rows = append(rows, value)
				}
			}
			sort.Slice(rows, func(i, j int) bool {
				if ascending {
					return rows[i].Value < rows[j].Value
				}
				return rows[i].Value > rows[j].Value
			})
			if len(rows) > limit {
				rows = rows[:limit]
			}
			return emitMarket(cmd, flags, rows, map[string]any{"metric": table.Dataset.Key})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum ranked regions")
	cmd.Flags().BoolVar(&ascending, "ascending", false, "Lowest values first")
	return cmd
}

func newExportMarketCmd(flags *rootFlags) *cobra.Command {
	var region string
	var months int
	cmd := &cobra.Command{
		Use: "export <metric>", Args: cobra.ExactArgs(1), Short: "Export normalized long-form observations",
		Example:     `  zillow-pp-cli export zhvi --region "Austin, TX" --months 12 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			loader, err := marketLoader(flags)
			if err != nil {
				return err
			}
			table, err := loader.Load(cmd.Context(), args[0], flags.dataSource)
			if err != nil {
				return err
			}
			rows := table.Rows
			if region != "" {
				resolved, resolveErr := table.ResolveRegion(region)
				if resolveErr != nil {
					return resolveErr
				}
				rows = []zillowdata.Row{resolved}
			}
			var out []map[string]any
			for _, row := range rows {
				dates := row.SortedDates()
				if months > 0 && len(dates) > months {
					dates = dates[len(dates)-months:]
				}
				for _, date := range dates {
					out = append(out, map[string]any{
						"metric": table.Dataset.Key, "region_id": row.RegionID,
						"region": row.DisplayName(), "region_type": row.RegionType, "state": row.StateName,
						"date": date.Format("2006-01-02"), "value": row.Values[date], "unit": table.Dataset.Unit,
					})
				}
			}
			return emitMarket(cmd, flags, out, map[string]any{"evidence": evidence(table)})
		},
	}
	cmd.Flags().StringVar(&region, "region", "", "Optional region name or ID")
	cmd.Flags().IntVar(&months, "months", 0, "Most recent observations per region; 0 means all")
	return cmd
}

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbOverride string
	var maxRows int
	cmd := &cobra.Command{
		Use: "sql <query>", Args: cobra.ExactArgs(1), Short: "Run read-only SQL against normalized market.db",
		Example:     `  zillow-pp-cli sql "SELECT metric, COUNT(*) AS observations FROM observations GROUP BY metric" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath, err := marketDBPath(dbOverride)
			if err != nil {
				return err
			}
			db, err := zillowdata.OpenReadOnlyDatabase(dbPath)
			if err != nil {
				return fmt.Errorf("opening %s: %w; run 'sync market' first", dbPath, err)
			}
			defer db.Close()
			rows, err := zillowdata.QueryReadOnly(cmd.Context(), db, args[0], maxRows)
			if err != nil {
				return err
			}
			if !flags.asJSON && !flags.csv && !flags.plain && !flags.quiet && !flags.agent && flags.selectFields == "" {
				return emitSQLRowsDefault(cmd, rows)
			}
			return emitMarket(cmd, flags, rows, map[string]any{"database": dbPath, "read_only": true})
		},
	}
	cmd.Flags().StringVar(&dbOverride, "db", "", "Absolute market database path")
	cmd.Flags().IntVar(&maxRows, "limit", 1000, "Maximum returned rows")
	return cmd
}

func emitSQLRowsDefault(cmd *cobra.Command, rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}
	columns := make([]string, 0, len(rows[0]))
	for column := range rows[0] {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	writer := newTabWriter(cmd.OutOrStdout())
	if _, err := fmt.Fprintln(writer, strings.Join(columns, "\t")); err != nil {
		return err
	}
	for _, row := range rows {
		values := make([]string, len(columns))
		for i, column := range columns {
			values[i] = fmt.Sprint(row[column])
		}
		if _, err := fmt.Fprintln(writer, strings.Join(values, "\t")); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	group := &cobra.Command{Use: "watch", Short: "Track saved regional metric snapshots locally"}
	var addRegion, addMetrics, dbOverride string
	add := &cobra.Command{
		Use: "add <name>", Args: cobra.ExactArgs(1), Short: "Create or replace a local watch",
		Example:     `  zillow-pp-cli watch add austin --region "Austin, TX" --metrics zhvi,zori --agent`,
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			keys := splitList(addMetrics)
			if addRegion == "" || len(keys) == 0 {
				return usageErr(fmt.Errorf("--region and --metrics are required"))
			}
			for _, key := range keys {
				if _, ok := zillowdata.DatasetByKey(key); !ok {
					return usageErr(fmt.Errorf("unknown metric %q", key))
				}
			}
			dbPath, err := marketDBPath(dbOverride)
			if err != nil {
				return err
			}
			db, err := zillowdata.OpenDatabase(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			watch := zillowdata.Watch{Name: args[0], Region: addRegion, Metrics: keys, UpdatedAt: time.Now().UTC()}
			if err := zillowdata.SaveWatch(cmd.Context(), db, watch); err != nil {
				return err
			}
			return emitMarket(cmd, flags, watch, map[string]any{"database": dbPath})
		},
	}
	add.Flags().StringVar(&addRegion, "region", "", "Region name or ID")
	add.Flags().StringVar(&addMetrics, "metrics", "zhvi,zori,inventory", "Comma-separated metrics")
	add.Flags().StringVar(&dbOverride, "db", "", "Absolute market database path")

	var listDB string
	list := &cobra.Command{
		Use: "list", Short: "List local watches",
		Example:     `  zillow-pp-cli watch list --agent`,
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath, err := marketDBPath(listDB)
			if err != nil {
				return err
			}
			db, err := zillowdata.OpenDatabase(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			watches, err := zillowdata.ListWatches(cmd.Context(), db)
			if err != nil {
				return err
			}
			return emitMarket(cmd, flags, watches, map[string]any{"database": dbPath})
		},
	}
	list.Flags().StringVar(&listDB, "db", "", "Absolute market database path")

	var runDB string
	run := &cobra.Command{
		Use: "run <name>", Args: cobra.ExactArgs(1), Short: "Fetch a watch and diff it against its previous snapshot",
		Example:     `  zillow-pp-cli watch run austin --agent`,
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath, err := marketDBPath(runDB)
			if err != nil {
				return err
			}
			db, err := zillowdata.OpenDatabase(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			watch, err := zillowdata.LoadWatch(cmd.Context(), db, args[0])
			if err != nil {
				return err
			}
			current := map[string]float64{}
			changes := map[string]map[string]float64{}
			for _, metric := range watch.Metrics {
				table, row, loadErr := loadRegion(cmd.Context(), flags, metric, watch.Region)
				if loadErr != nil {
					return loadErr
				}
				_, value, ok := row.Latest()
				if !ok {
					continue
				}
				current[metric] = value
				if previous, found := watch.LastSnapshot[metric]; found {
					changes[metric] = map[string]float64{"previous": previous, "current": value, "change": value - previous}
					if previous != 0 {
						changes[metric]["change_percent"] = (value/previous - 1) * 100
					}
				}
				_ = table
			}
			watch.LastSnapshot = current
			watch.UpdatedAt = time.Now().UTC()
			if err := zillowdata.SaveWatch(cmd.Context(), db, watch); err != nil {
				return err
			}
			return emitMarket(cmd, flags, map[string]any{"watch": watch, "changes": changes}, map[string]any{"database": dbPath})
		},
	}
	run.Flags().StringVar(&runDB, "db", "", "Absolute market database path")
	group.AddCommand(add, list, run)
	return group
}

func newBridgeCmd(flags *rootFlags) *cobra.Command {
	group := &cobra.Command{
		Use: "bridge", Short: "Query separately authorized Bridge datasets without caching",
		Long: "Bridge access is optional and permissioned. This CLI never stores Bridge responses. Set BRIDGE_ACCESS_TOKEN only after Zillow/MLS approval.",
	}
	status := &cobra.Command{
		Use: "status", Short: "Show Bridge authorization readiness",
		Example:     `  zillow-pp-cli bridge status --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			_, present := os.LookupEnv("BRIDGE_ACCESS_TOKEN")
			return emitMarket(cmd, flags, map[string]any{
				"token_configured": present, "response_caching": false,
				"access": "Dataset access requires separate Zillow Group or MLS approval.",
			}, nil)
		},
	}
	var dataset, resource string
	var params []string
	var top int
	request := &cobra.Command{
		Use: "request", Short: "Issue one authorized Bridge Web API GET",
		Example:     `  zillow-pp-cli bridge request --dataset <approved-dataset> --resource Property --top 10 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:requires-tier": "bridge-approved"},
		RunE: func(cmd *cobra.Command, args []string) error {
			token := strings.TrimSpace(os.Getenv("BRIDGE_ACCESS_TOKEN"))
			if token == "" {
				return fmt.Errorf("BRIDGE_ACCESS_TOKEN is not configured")
			}
			if !bridgeSegmentPattern.MatchString(dataset) || !bridgeSegmentPattern.MatchString(resource) {
				return usageErr(fmt.Errorf("--dataset and --resource must contain only letters, numbers, underscore, or hyphen"))
			}
			if top <= 0 || top > 200 {
				return usageErr(fmt.Errorf("--top must be 1..200"))
			}
			values := url.Values{}
			values.Set("limit", strconv.Itoa(top))
			for _, param := range params {
				key, value, ok := strings.Cut(param, "=")
				if !ok || strings.TrimSpace(key) == "" {
					return usageErr(fmt.Errorf("invalid --param %q; expected key=value", param))
				}
				if strings.EqualFold(strings.TrimSpace(key), "access_token") {
					return usageErr(fmt.Errorf("access_token query parameters are forbidden; token is sent in Authorization header"))
				}
				values.Set(strings.TrimSpace(key), value)
			}
			target := "https://api.bridgedataoutput.com/api/v2/" + dataset + "/" + resource + "?" + values.Encode()
			req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, target, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Accept", "application/json")
			client := &http.Client{Timeout: flags.timeout}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			if err != nil {
				return err
			}
			if resp.StatusCode >= 400 {
				message := strings.TrimSpace(string(body))
				if len(message) > 2048 {
					message = message[:2048] + "..."
				}
				return fmt.Errorf("Bridge API HTTP %d: %s", resp.StatusCode, message)
			}
			var payload any
			if err := json.Unmarshal(body, &payload); err != nil {
				return fmt.Errorf("decoding Bridge response: %w", err)
			}
			return emitMarket(cmd, flags, map[string]any{
				"dataset": dataset, "resource": resource, "results": payload,
				"cached": false, "attribution": "Bridge Interactive / authorized data provider",
			}, map[string]any{"source": "live"})
		},
	}
	request.Flags().StringVar(&dataset, "dataset", "", "Approved Bridge dataset ID")
	request.Flags().StringVar(&resource, "resource", "", "Bridge resource name")
	request.Flags().StringSliceVar(&params, "param", nil, "Query parameter key=value; repeatable")
	request.Flags().IntVar(&top, "top", 10, "Maximum results; Bridge maximum 200")
	_ = request.MarkFlagRequired("dataset")
	_ = request.MarkFlagRequired("resource")
	group.AddCommand(status, request)
	return group
}
