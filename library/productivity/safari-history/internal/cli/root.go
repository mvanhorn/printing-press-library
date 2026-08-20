package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/output"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/store"
	"github.com/spf13/cobra"
)

const (
	CLIVersion          = "0.1.0"
	ExitOK              = 0
	ExitUsage           = 2
	ExitNoSnapshot      = 3
	ExitSourceDBMissing = 4
)

var (
	ErrNoSnapshot      = errors.New("run sync first")
	ErrSourceDBMissing = errors.New("safari db not found")
	ErrUsage           = errors.New("usage error")
)

type RootOptions struct {
	Output     output.Flags
	Profile    string
	SourceName string
	Source     source.Source
	Device     string
}

func NewRootCmd() *cobra.Command {
	opts := &RootOptions{}
	root := &cobra.Command{
		Use:           "safari-history-pp-cli",
		Short:         "Local-first Safari history CLI",
		Version:       CLIVersion,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().BoolVar(&opts.Output.JSON, "json", false, "JSON output")
	root.PersistentFlags().StringVar(&opts.Output.Select, "select", "", "select dotted paths")
	root.PersistentFlags().BoolVar(&opts.Output.Compact, "compact", false, "compact output")
	root.PersistentFlags().BoolVar(&opts.Output.CSV, "csv", false, "CSV output")
	root.PersistentFlags().BoolVar(&opts.Output.Quiet, "quiet", false, "suppress output")
	root.PersistentFlags().IntVar(&opts.Output.Limit, "limit", 20, "row limit")
	root.PersistentFlags().StringVar(&opts.Profile, "profile", "Default", "Safari profile (ignored; Safari uses default history DB)")
	root.PersistentFlags().StringVar(&opts.SourceName, "browser", "safari", "history source")
	root.PersistentFlags().StringVar(&opts.Device, "device", "all", "visit origin filter: all|this|synced|device-N")
	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return fmt.Errorf("%w: %v", ErrUsage, err)
	})
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		opts.Output.Command = cmd.Name()
		s, err := resolveSource(opts.SourceName)
		if err != nil {
			return err
		}
		opts.Source = s
		return nil
	}

	root.AddCommand(
		newSyncCmd(opts),
		newDoctorCmd(opts),
		newSearchCmd(opts),
		newSQLCmd(opts),
		newListCmd(opts),
		newDomainsCmd(opts),
		newSearchesCmd(opts),
		newDownloadsCmd(opts),
		newDevicesCmd(opts),
		newICloudTabsCmd(opts),
		newVisitedCmd(opts),
		newReportCmd(opts),
		newHeatmapCmd(opts),
		newJourneysCmd(opts),
		newTimelineCmd(opts),
		newRabbitholesCmd(opts),
		newDwellCmd(opts),
		newGraphCmd(opts),
		newProfileCmd(opts),
		newTopicCmd(opts),
		newArchiveCmd(opts),
		newAgentContextCmd(opts),
		newVersionCmd(),
		newMCPCmd(),
	)
	return root
}

func newSyncCmd(opts *RootOptions) *cobra.Command {
	var accumulate bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Snapshot Safari history and build FTS",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := opts.Source.LocateHistoryDB(opts.Profile); err != nil {
				return fmt.Errorf("%w: %v. Cannot refresh live Safari history right now; this does not mean there is no data. If a cached snapshot/archive exists, query it directly with search, sql, domains, list, or report (cached reads do not require sync). Grant this terminal Full Disk Access to refresh from live Safari", ErrSourceDBMissing, err)
			}
			snapshot, err := snapshotPath()
			if err != nil {
				return err
			}
			si, err := opts.Source.Snapshot(filepath.Dir(snapshot), opts.Profile)
			if err != nil {
				return err
			}
			meta, err := store.BuildSnapshotIndexWithVersions(si.SnapshotPath, opts.Profile, int64(si.Version), int64(si.LastCompatibleVersion), opts.Source)
			if err != nil {
				_ = os.Remove(si.SnapshotPath)
				return err
			}
			if err := os.Rename(si.SnapshotPath, snapshot); err != nil {
				_ = os.Remove(si.SnapshotPath)
				return err
			}
			archiveEnabled, err := store.IsArchiveEnabled()
			if err != nil {
				return err
			}
			rows := []map[string]any{{
				"snapshot":                       snapshot,
				"profile":                        meta.Profile,
				"synced_at":                      meta.SyncedAt,
				"pages_count":                    meta.PagesCount,
				"visits_count":                   meta.VisitsCount,
				"terms_count":                    meta.TermsCount,
				"source_schema_version":          meta.SourceSchemaVersion,
				"source_last_compatible_version": meta.SourceLastCompatibleVersion,
			}}
			if accumulate || archiveEnabled {
				archivePath, err := store.ArchivePath()
				if err != nil {
					return err
				}
				counts, err := store.AccumulateFromSource(archivePath, snapshot, time.Now().UTC())
				if err != nil {
					return err
				}
				rows[0]["archive"] = archivePath
				rows[0]["archive_accumulated"] = true
				rows[0]["archive_before"] = counts.Before
				rows[0]["archive_after"] = counts.After
				rows[0]["archive_inserted"] = counts.Inserted
			}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, rows)
		},
	}
	cmd.Flags().BoolVar(&accumulate, "accumulate", false, "append snapshot rows into the accumulating archive")
	return cmd
}

func newDoctorCmd(opts *RootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check safari db and snapshot health",
		RunE: func(cmd *cobra.Command, args []string) error {
			src, srcErr := opts.Source.LocateHistoryDB(opts.Profile)
			snapshot, err := snapshotPath()
			if err != nil {
				return err
			}
			status := map[string]any{"profile": opts.Profile}
			if srcErr != nil {
				status["source_db"] = "missing"
				status["source_db_error"] = srcErr.Error()
			} else {
				status["source_db"] = src
			}
			st, err := os.Stat(snapshot)
			if err != nil {
				if os.IsNotExist(err) {
					status["snapshot"] = "missing"
				} else {
					status["snapshot"] = "error"
					status["snapshot_error"] = err.Error()
				}
			} else {
				status["snapshot"] = snapshot
				status["snapshot_age"] = time.Since(st.ModTime()).String()
				stStore, openErr := store.OpenExisting(snapshot)
				if openErr == nil {
					defer stStore.Close()
					status["fts_ready"] = stStore.IsFTSReady()
					status["pages_count"] = stStore.RowCount("history_items")
					status["visits_count"] = stStore.RowCount("history_visits")
					status["history_fts_count"] = stStore.RowCount("history_fts")
					meta, _ := stStore.GetSyncMeta()
					status["meta_pp"] = meta
					status["source_schema_version"] = meta.SourceSchemaVersion
					status["source_last_compatible_version"] = meta.SourceLastCompatibleVersion
					if meta.SourceSchemaVersion > 0 {
						if meta.SourceSchemaVersion < int64(opts.Source.MinSupportedVersion()) {
							status["warning"] = fmt.Sprintf("older than supported schema (detected v%d, min supported v%d)", meta.SourceSchemaVersion, opts.Source.MinSupportedVersion())
						} else if meta.SourceSchemaVersion > int64(opts.Source.TestedVersion()) {
							status["warning"] = fmt.Sprintf("newer than tested v%d (detected v%d) - some commands may need updates", opts.Source.TestedVersion(), meta.SourceSchemaVersion)
						}
					} else {
						status["schema_version"] = "unknown"
					}
					devs, derr := opts.Source.Devices(stStore.DB())
					if derr == nil {
						breakdown := map[string]int64{}
						for _, d := range devs {
							if d.Kind == "this" || d.Kind == "synced" {
								breakdown[d.ID] = d.Visits
							}
						}
						status["device_breakdown"] = breakdown
					}
				}
			}
			archiveStatus, archiveErr := store.ReadArchiveStatus()
			if archiveErr != nil {
				status["archive_error"] = archiveErr.Error()
			} else {
				status["archive"] = archiveStatus.Path
				status["archive_enabled"] = archiveStatus.Enabled
				status["archive_url_count"] = archiveStatus.URLCount
				status["archive_visit_count"] = archiveStatus.VisitCount
				status["archive_size_bytes"] = archiveStatus.SizeBytes
			}
			status["cached_store"] = cachedStoreStatus(status)
			if source := cachedStoreSource(status); source != "" {
				status["cached_store_source"] = source
			}
			if status["source_db"] == "missing" && status["cached_store"] == "queryable" {
				status["note"] = "Live Safari history cannot be refreshed right now, but cached history is queryable offline with search/sql/domains/list/report; no sync required for reads."
			}
			status["healthy"] = status["source_db"] != "missing" && status["cached_store"] == "queryable"
			rows := []map[string]any{status}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, rows)
		},
	}
	return cmd
}

func cachedStoreStatus(status map[string]any) string {
	visits, ok := status["visits_count"].(int64)
	if ok && visits > 0 {
		return "queryable"
	}
	pages, ok := status["pages_count"].(int64)
	if ok && pages > 0 {
		return "queryable"
	}
	archiveVisits, ok := status["archive_visit_count"].(int64)
	archiveEnabled, _ := status["archive_enabled"].(bool)
	if ok && archiveEnabled && archiveVisits > 0 {
		return "queryable"
	}
	if _, ok := status["visits_count"]; ok {
		return "empty"
	}
	if _, ok := status["archive_visit_count"]; ok {
		return "empty"
	}
	return "missing"
}

func cachedStoreSource(status map[string]any) string {
	if visits, ok := status["archive_visit_count"].(int64); ok && visits > 0 {
		if enabled, _ := status["archive_enabled"].(bool); enabled {
			return "archive"
		}
	}
	if visits, ok := status["visits_count"].(int64); ok && visits > 0 {
		return "snapshot"
	}
	if pages, ok := status["pages_count"].(int64); ok && pages > 0 {
		return "snapshot"
	}
	return ""
}

func newSearchCmd(opts *RootOptions) *cobra.Command {
	var domain string
	var since string
	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Full-text (FTS5) search over visited URLs and page titles; filter by --domain and --since, ranked by relevance",
		Args:        usageMinArgs(1),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Time{}
			if strings.TrimSpace(since) != "" {
				var err error
				start, _, err = sourceTimeWindow(since, "", 30*24*time.Hour)
				if err != nil {
					return errors.Join(ErrUsage, err)
				}
			}
			st, _, err := openCoreHistoryStore(opts.Device)
			if err != nil {
				return err
			}
			defer st.Close()
			q := strings.Join(args, " ")
			rows, err := opts.Source.FullTextSearch(st.DB(), q, source.VisitFilter{Limit: opts.Output.Limit, Domain: domain, Since: start, Device: opts.Device})
			if err != nil {
				return err
			}
			recent, _ := opts.Source.RecentVisits(st.DB(), source.VisitFilter{Since: start, Until: time.Now().UTC(), Limit: 500000, Domain: domain, Device: opts.Device})
			byURL := map[string]string{}
			for _, v := range recent {
				if _, ok := byURL[v.URL]; !ok {
					byURL[v.URL] = v.Origin
				}
			}
			out := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				out = append(out, map[string]any{
					"url": r.URL, "title": r.Title, "visit_count": r.VisitCount,
					"last_visit_time": r.LastVisit, "rank": r.Rank, "origin": byURL[r.URL],
				})
			}
			maybePrintEmptyWindowHint(st.DB(), since, len(out) == 0)
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, out)
		},
	}
	cmd.Flags().StringVar(&domain, "domain", "", "filter by domain")
	cmd.Flags().StringVar(&since, "since", "", "time window")
	return cmd
}

func newSQLCmd(opts *RootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sql <SELECT...>",
		Short:       "Run a read-only SELECT query against the snapshot's history tables (non-SELECT statements are rejected)",
		Args:        usageMinArgs(1),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			q := strings.Join(args, " ")
			if !store.IsSelectOnly(q) {
				return fmt.Errorf("%w: only SELECT statements are allowed", ErrUsage)
			}
			st, _, err := openCoreHistoryStore(opts.Device)
			if err != nil {
				return err
			}
			defer st.Close()
			rows, err := st.RunSelect(q, opts.Output.Limit)
			if err != nil {
				return err
			}
			opts2 := opts.Output
			if !opts2.CSV {
				opts2.JSON = true
			}
			output.DefaultToJSONIfNotTTY(&opts2)
			return output.Render(opts2, rows)
		},
	}
	return cmd
}

func ExitCodeForError(err error) int {
	if err == nil {
		return ExitOK
	}
	if errors.Is(err, ErrNoSnapshot) {
		return ExitNoSnapshot
	}
	if errors.Is(err, ErrSourceDBMissing) {
		return ExitSourceDBMissing
	}
	if errors.Is(err, ErrUsage) {
		return ExitUsage
	}
	if strings.Contains(err.Error(), "accepts") || strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "unknown flag") {
		return ExitUsage
	}
	return 1
}

func usageMinArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < n {
			return fmt.Errorf("%w: accepts at least %d arg(s), received %d", ErrUsage, n, len(args))
		}
		return nil
	}
}

func usageExactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return fmt.Errorf("%w: accepts %d arg(s), received %d", ErrUsage, n, len(args))
		}
		return nil
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the safari-history CLI version string",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := io.WriteString(cmd.OutOrStdout(), CLIVersion+"\n")
			return err
		},
	}
}
