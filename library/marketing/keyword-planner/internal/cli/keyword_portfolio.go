// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/portfolio"
	"github.com/spf13/cobra"
)

const (
	plannerDBEnv        = "KEYWORD_PLANNER_DB"
	plannerDBDefaultDir = ".local/share/keyword-planner"
	plannerDBFilename   = "snapshots.db"
)

type plannerPortfolioQueryFlags struct {
	dbPath            string
	snapshot          string
	keyword           string
	language          string
	geoTargets        []string
	endpoint          string
	start             string
	end               string
	includeIncomplete bool
	availableOnly     bool
	limit             int
}

func bindPlannerPortfolioQueryFlags(cmd *cobra.Command, flags *plannerPortfolioQueryFlags) {
	if cmd == nil || flags == nil {
		return
	}
	cmd.Flags().StringVar(&flags.dbPath, "db", "", "Portfolio SQLite path (env KEYWORD_PLANNER_DB is used when omitted)")
	cmd.Flags().StringVar(&flags.snapshot, "snapshot", "", "Snapshot ID or alias latest/previous")
	cmd.Flags().StringVar(&flags.keyword, "keyword", "", "Case-insensitive keyword filter")
	cmd.Flags().StringVar(&flags.language, "language", "", "Language resource filter, for example languageConstants/1000")
	cmd.Flags().StringArrayVar(&flags.geoTargets, "geo", nil, "Exact geo target resource filter; repeat for multiple targets")
	cmd.Flags().StringVar(&flags.endpoint, "endpoint", "", "Collection endpoint filter: ideas or historical")
	cmd.Flags().StringVar(&flags.start, "start", "", "First month filter, YYYY-MM")
	cmd.Flags().StringVar(&flags.end, "end", "", "Last month filter, YYYY-MM")
	cmd.Flags().BoolVar(&flags.includeIncomplete, "include-incomplete", false, "Include snapshots that were not collected completely")
	cmd.Flags().BoolVar(&flags.availableOnly, "available-only", false, "Return only rows with an available monthly value")
	cmd.Flags().IntVar(&flags.limit, "limit", 0, "Maximum rows or snapshots to render; zero means all")
}

func plannerRootHome(flags *rootFlags) string {
	if flags == nil {
		return ""
	}
	home := strings.TrimSpace(flags.homePath)
	// Root pre-run has already installed the override. Expand against the
	// process home before calling the shared cleaner, whose own tilde resolver
	// would otherwise expand against that installed override a second time.
	if home == "~" || strings.HasPrefix(home, "~/") {
		if processHome, err := os.UserHomeDir(); err == nil {
			if home == "~" {
				home = processHome
			} else {
				home = filepath.Join(processHome, strings.TrimPrefix(home, "~/"))
			}
		}
	}
	if clean, ok := cliutil.CleanPathOverride(home); ok {
		return clean
	}
	// Root pre-run rejects invalid nonempty overrides before command work.
	return home
}

func plannerPortfolioDBPath(explicit string, homeOverrides ...string) string {
	if path := strings.TrimSpace(explicit); path != "" {
		return path
	}
	if path := strings.TrimSpace(os.Getenv(plannerDBEnv)); path != "" {
		return path
	}
	if len(homeOverrides) > 0 {
		if home := strings.TrimSpace(homeOverrides[0]); home != "" {
			return filepath.Join(home, plannerDBDefaultDir, plannerDBFilename)
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(plannerDBDefaultDir, plannerDBFilename)
	}
	return filepath.Join(home, plannerDBDefaultDir, plannerDBFilename)
}

func plannerPortfolioOptions(flags plannerPortfolioQueryFlags) (portfolio.QueryOptions, error) {
	if flags.limit < 0 {
		return portfolio.QueryOptions{}, fmt.Errorf("limit cannot be negative; got %d", flags.limit)
	}
	options := portfolio.QueryOptions{
		SnapshotID:        strings.TrimSpace(flags.snapshot),
		Keyword:           strings.TrimSpace(flags.keyword),
		Language:          strings.TrimSpace(flags.language),
		Endpoint:          strings.TrimSpace(flags.endpoint),
		StartMonth:        strings.TrimSpace(flags.start),
		EndMonth:          strings.TrimSpace(flags.end),
		IncludeIncomplete: flags.includeIncomplete,
		AvailableOnly:     flags.availableOnly,
		Limit:             flags.limit,
	}
	if options.Language != "" {
		language, err := validatePlannerResourceName("language", options.Language, "languageConstants")
		if err != nil {
			return portfolio.QueryOptions{}, err
		}
		options.Language = language
	}
	if len(flags.geoTargets) > 0 {
		geos, _, err := validatePlannerGeoTargets(flags.geoTargets, false)
		if err != nil {
			return portfolio.QueryOptions{}, err
		}
		options.GeoTargets = geos
	}
	if options.Endpoint != "" {
		endpoint, err := portfolio.NormalizeEndpoint(options.Endpoint)
		if err != nil {
			return portfolio.QueryOptions{}, err
		}
		if endpoint != portfolio.EndpointIdeas && endpoint != portfolio.EndpointHistorical {
			return portfolio.QueryOptions{}, fmt.Errorf("endpoint %q is not a Planner collection endpoint", options.Endpoint)
		}
		options.Endpoint = endpoint
	}
	if options.StartMonth != "" || options.EndMonth != "" {
		if options.StartMonth == "" || options.EndMonth == "" {
			return portfolio.QueryOptions{}, errors.New("--start and --end must be supplied together")
		}
		from, err := parsePlannerMonth(options.StartMonth)
		if err != nil {
			return portfolio.QueryOptions{}, err
		}
		to, err := parsePlannerMonth(options.EndMonth)
		if err != nil {
			return portfolio.QueryOptions{}, err
		}
		if from.after(to) {
			return portfolio.QueryOptions{}, fmt.Errorf("start month %q is after end month %q", options.StartMonth, options.EndMonth)
		}
		options.StartMonth, options.EndMonth = from.String(), to.String()
	}
	return options, nil
}

func rejectPlannerEvidenceBypass(flags *rootFlags) error {
	if flags == nil {
		return nil
	}
	if flags.noCache {
		return errors.New("--no-cache is not supported for immutable portfolio reads")
	}
	if flags.allowPartialFailure {
		return errors.New("--allow-partial-failure is not supported for immutable portfolio reads")
	}
	return nil
}

func openPlannerPortfolioReadOnly(ctx context.Context, path string) (*portfolio.Store, bool, error) {
	store, err := portfolio.OpenReadOnly(ctx, path)
	if errors.Is(err, portfolio.ErrNotFound) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	return store, false, nil
}

func writePlannerLocalOutput(cmd *cobra.Command, flags *rootFlags, value any, documented ...map[string]bool) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode local portfolio output: %w", err)
	}
	return printPlannerEvidenceOutput(cmd.OutOrStdout(), data, flags, map[string]any{"source": "local"}, documented...)
}

func writePlannerLocalRaw(cmd *cobra.Command, flags *rootFlags, data []byte, documented ...map[string]bool) error {
	if len(data) == 0 {
		data = []byte("[]")
	}
	return printPlannerEvidenceOutput(cmd.OutOrStdout(), data, flags, map[string]any{"source": "local"}, documented...)
}

func newPlannerPortfolioListCmd(flags *rootFlags) *cobra.Command {
	var query plannerPortfolioQueryFlags
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List local Planner snapshots, including incomplete attempts",
		Example:     "  keyword-planner portfolio list --snapshot latest --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usageErr(fmt.Errorf("unexpected argument %q", args[0]))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "portfolio list")
			}
			if err := rejectPlannerEvidenceBypass(flags); err != nil {
				return usageErr(err)
			}
			options, err := plannerPortfolioOptions(query)
			if err != nil {
				return usageErr(err)
			}
			store, missing, err := openPlannerPortfolioReadOnly(cmd.Context(), plannerPortfolioDBPath(query.dbPath, plannerRootHome(flags)))
			if err != nil {
				return configErr(err)
			}
			if missing {
				if plannerWantsHumanTerminal(cmd.OutOrStdout(), flags) {
					return writePlannerSnapshotsHuman(cmd.OutOrStdout(), []portfolio.Snapshot{})
				}
				return writePlannerLocalOutput(cmd, flags, []portfolio.Snapshot{}, plannerSnapshotFields)
			}
			defer store.Close()
			items, err := store.ListSnapshots(cmd.Context(), options)
			if err != nil {
				return configErr(err)
			}
			if items == nil {
				items = []portfolio.Snapshot{}
			}
			if plannerWantsHumanTerminal(cmd.OutOrStdout(), flags) {
				return writePlannerSnapshotsHuman(cmd.OutOrStdout(), items)
			}
			return writePlannerLocalOutput(cmd, flags, items, plannerSnapshotFields)
		},
	}
	bindPlannerPortfolioQueryFlags(cmd, &query)
	return cmd
}

func newPlannerPortfolioShowCmd(flags *rootFlags) *cobra.Command {
	var query struct {
		dbPath   string
		snapshot string
	}
	cmd := &cobra.Command{
		Use:         "show [snapshot-id]",
		Short:       "Show one local snapshot with raw receipts and normalized rows",
		Example:     "  keyword-planner portfolio show latest --agent --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return usageErr(fmt.Errorf("show accepts at most one snapshot ID or alias"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "portfolio show")
			}
			if err := rejectPlannerEvidenceBypass(flags); err != nil {
				return usageErr(err)
			}
			alias := strings.TrimSpace(query.snapshot)
			if len(args) == 1 {
				if alias != "" && alias != strings.TrimSpace(args[0]) {
					return usageErr(errors.New("snapshot may be supplied as an argument or --snapshot, not both"))
				}
				alias = strings.TrimSpace(args[0])
			}
			if alias == "" {
				return usageErr(errors.New("provide a snapshot ID or alias latest/previous"))
			}
			store, missing, err := openPlannerPortfolioReadOnly(cmd.Context(), plannerPortfolioDBPath(query.dbPath, plannerRootHome(flags)))
			if err != nil {
				return configErr(err)
			}
			if missing {
				return notFoundErr(portfolio.ErrNotFound)
			}
			defer store.Close()
			view, err := store.ShowSnapshot(cmd.Context(), alias)
			if errors.Is(err, portfolio.ErrNotFound) {
				return notFoundErr(err)
			}
			if err != nil {
				return configErr(err)
			}
			return writePlannerLocalOutput(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&query.dbPath, "db", "", "Portfolio SQLite path (env KEYWORD_PLANNER_DB is used when omitted)")
	cmd.Flags().StringVar(&query.snapshot, "snapshot", "", "Snapshot ID or alias latest/previous")
	return cmd
}

func newPlannerPortfolioSearchCmd(flags *rootFlags) *cobra.Command {
	var query plannerPortfolioQueryFlags
	var term string
	cmd := &cobra.Command{
		Use:         "search [term]",
		Short:       "Search saved Planner rows offline by keyword, endpoint, snapshot, month, geo, or availability",
		Example:     "  keyword-planner portfolio search beef --snapshot latest --limit 10 --agent --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return usageErr(errors.New("search accepts one term"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "portfolio search")
			}
			if err := rejectPlannerEvidenceBypass(flags); err != nil {
				return usageErr(err)
			}
			if len(args) == 1 {
				if strings.TrimSpace(term) != "" && strings.TrimSpace(term) != strings.TrimSpace(args[0]) {
					return usageErr(errors.New("search term may be supplied as an argument or --query, not both"))
				}
				term = args[0]
			}
			term = strings.TrimSpace(term)
			if term == "" {
				return usageErr(errors.New("provide a search term"))
			}
			options, err := plannerPortfolioOptions(query)
			if err != nil {
				return usageErr(err)
			}
			store, missing, err := openPlannerPortfolioReadOnly(cmd.Context(), plannerPortfolioDBPath(query.dbPath, plannerRootHome(flags)))
			if err != nil {
				return configErr(err)
			}
			if missing {
				if plannerWantsHumanTerminal(cmd.OutOrStdout(), flags) {
					return writePlannerRowsHuman(cmd.OutOrStdout(), []portfolio.Row{}, "Monthly rows")
				}
				return writePlannerLocalOutput(cmd, flags, []portfolio.Row{}, plannerRowFields)
			}
			defer store.Close()
			rows, err := store.Search(cmd.Context(), term, options)
			if err != nil {
				return configErr(err)
			}
			if rows == nil {
				rows = []portfolio.Row{}
			}
			if plannerWantsHumanTerminal(cmd.OutOrStdout(), flags) {
				return writePlannerRowsHuman(cmd.OutOrStdout(), rows, "Monthly rows")
			}
			return writePlannerLocalOutput(cmd, flags, rows, plannerRowFields)
		},
	}
	bindPlannerPortfolioQueryFlags(cmd, &query)
	cmd.Flags().StringVar(&term, "query", "", "Search term when no positional term is used")
	return cmd
}

func newPlannerPortfolioExportCmd(flags *rootFlags) *cobra.Command {
	var query plannerPortfolioQueryFlags
	var format string
	cmd := &cobra.Command{
		Use:         "export",
		Short:       "Export saved Planner rows offline as JSON or CSV with keyword, endpoint, snapshot, month, geo, and field filters",
		Example:     "  keyword-planner portfolio export --snapshot latest --agent --select keyword,month,monthly_searches",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usageErr(fmt.Errorf("unexpected argument %q", args[0]))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "portfolio export")
			}
			if err := rejectPlannerEvidenceBypass(flags); err != nil {
				return usageErr(err)
			}
			options, err := plannerPortfolioOptions(query)
			if err != nil {
				return usageErr(err)
			}
			format = strings.ToLower(strings.TrimSpace(format))
			if flags != nil && flags.csv {
				format = "csv"
			}
			if format != string(portfolio.ExportJSON) && format != string(portfolio.ExportCSV) {
				return usageErr(fmt.Errorf("format must be json or csv; got %q", format))
			}
			selectedFields := ""
			if flags != nil {
				selectedFields = flags.selectFields
			}
			store, missing, err := openPlannerPortfolioReadOnly(cmd.Context(), plannerPortfolioDBPath(query.dbPath, plannerRootHome(flags)))
			if err != nil {
				return configErr(err)
			}
			if missing {
				if format == string(portfolio.ExportCSV) {
					data, projectionErr := projectPlannerPortfolioCSV([]byte(plannerPortfolioCSVHeader+"\n"), selectedFields)
					if projectionErr != nil {
						return usageErr(projectionErr)
					}
					_, err := ioWrite(cmd, data)
					return err
				}
				if plannerWantsHumanTerminal(cmd.OutOrStdout(), flags) {
					return writePlannerRowsHuman(cmd.OutOrStdout(), []portfolio.Row{}, "Monthly export")
				}
				return writePlannerLocalRaw(cmd, flags, []byte("[]"), plannerRowFields)
			}
			defer store.Close()
			data, err := store.Export(cmd.Context(), options, portfolio.ExportFormat(format))
			if err != nil {
				return configErr(err)
			}
			if format == string(portfolio.ExportCSV) {
				data, err = projectPlannerPortfolioCSV(data, selectedFields)
				if err != nil {
					return usageErr(err)
				}
				_, err = ioWrite(cmd, data)
				return err
			}
			if plannerWantsHumanTerminal(cmd.OutOrStdout(), flags) {
				return writePlannerRowsHumanJSON(cmd.OutOrStdout(), data, "Monthly export")
			}
			return writePlannerLocalRaw(cmd, flags, data, plannerRowFields)
		},
	}
	bindPlannerPortfolioQueryFlags(cmd, &query)
	cmd.Flags().StringVar(&format, "format", string(portfolio.ExportJSON), "Export format: json or csv")
	return cmd
}

const plannerPortfolioCSVHeader = "keyword,geo,language_code,month,monthly_searches,low_bid_micros,high_bid_micros,competition,fetched_at,snapshot_id,endpoint,variant_group,geo_set_id,geo_targets_json,language_resource,network,currency,requested_start,requested_end,status,complete,flags,value_state"

var plannerPortfolioCSVFields = strings.Split(plannerPortfolioCSVHeader, ",")

func projectPlannerPortfolioCSV(data []byte, selectFields string) ([]byte, error) {
	if strings.TrimSpace(selectFields) == "" {
		return data, nil
	}
	requested := make([]string, 0)
	for _, raw := range strings.Split(selectFields, ",") {
		field := strings.TrimSpace(raw)
		if field == "" {
			return nil, errors.New("--select contains an empty CSV column")
		}
		requested = append(requested, field)
	}
	if len(requested) == 0 {
		return data, nil
	}
	indexes := make(map[string]int, len(plannerPortfolioCSVFields))
	for i, field := range plannerPortfolioCSVFields {
		indexes[field] = i
	}
	selectedIndexes := make([]int, len(requested))
	for i, field := range requested {
		index, ok := indexes[field]
		if !ok {
			return nil, fmt.Errorf("--select column %q is not available in Planner CSV export; valid columns: %s", field, strings.Join(plannerPortfolioCSVFields, ", "))
		}
		selectedIndexes[i] = index
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read Planner CSV header: %w", err)
	}
	headerIndexes := make(map[string]int, len(header))
	for i, field := range header {
		headerIndexes[field] = i
	}
	for i, field := range requested {
		if _, ok := headerIndexes[field]; !ok {
			return nil, fmt.Errorf("Planner CSV source is missing selected column %q", field)
		}
		// Use the source header's position rather than the fixed schema index so
		// this remains safe if the store's stable column order evolves.
		selectedIndexes[i] = headerIndexes[field]
	}

	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	if err := writer.Write(requested); err != nil {
		return nil, fmt.Errorf("write selected Planner CSV header: %w", err)
	}
	for {
		record, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("read Planner CSV row: %w", readErr)
		}
		selected := make([]string, len(selectedIndexes))
		for i, index := range selectedIndexes {
			if index < len(record) {
				selected[i] = record[index]
			}
		}
		if err := writer.Write(selected); err != nil {
			return nil, fmt.Errorf("write selected Planner CSV row: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush selected Planner CSV: %w", err)
	}
	return output.Bytes(), nil
}

var plannerSnapshotFields = map[string]bool{
	"id": true, "run_id": true, "endpoint": true, "customer_id": true, "language": true,
	"network": true, "geo_target_constants": true, "requested_start": true, "requested_end": true,
	"status": true, "complete": true, "fetched_at": true,
}

var plannerRowFields = map[string]bool{
	"snapshot_id": true, "receipt_id": true, "keyword": true, "submitted_keyword": true,
	"language": true, "network": true, "currency": true, "month": true, "monthly_searches": true,
	"low_bid_micros": true, "high_bid_micros": true, "average_cpc_micros": true,
	"competition": true, "fetched_at": true, "endpoint": true, "variant_group": true,
	"geo_set_id": true, "requested_start": true, "requested_end": true, "status": true,
	"complete": true, "flags": true, "value_state": true,
}

func ioWrite(cmd *cobra.Command, data []byte) (int, error) {
	if cmd == nil {
		return 0, errors.New("command is nil")
	}
	return cmd.OutOrStdout().Write(data)
}
