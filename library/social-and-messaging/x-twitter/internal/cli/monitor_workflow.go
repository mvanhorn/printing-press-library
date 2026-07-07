// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/store"
	"github.com/spf13/cobra"
)

type monitorDefinition struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Query     string `json:"query"`
	SourceURL string `json:"source_url,omitempty"`
	Account   string `json:"account,omitempty"`
	Watermark string `json:"watermark_id,omitempty"`
	LastRunAt string `json:"last_run_at,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type monitorRunResult struct {
	Monitor              monitorDefinition        `json:"monitor"`
	RunID                string                   `json:"run_id"`
	Preview              bool                     `json:"preview"`
	NewResults           int                      `json:"new_results"`
	SkippedDuplicates    int                      `json:"skipped_duplicates"`
	WatermarkBefore      string                   `json:"watermark_before,omitempty"`
	WatermarkAfter       string                   `json:"watermark_after,omitempty"`
	Results              []collectionItemSnapshot `json:"results,omitempty"`
	Collection           string                   `json:"collection,omitempty"`
	CollectionAdded      int                      `json:"collection_added,omitempty"`
	CollectionDuplicates int                      `json:"collection_duplicates,omitempty"`
}

func newNovelMonitorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Create and run durable watermarked X monitors",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelMonitorCreateCmd(flags))
	cmd.AddCommand(newNovelMonitorRunCmd(flags))
	cmd.AddCommand(newNovelMonitorListCmd(flags))
	return cmd
}

func newNovelMonitorCreateCmd(flags *rootFlags) *cobra.Command {
	var dbPath, query, sourceURL, account string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create or update a named local X monitor",
		Example: `  x-twitter-pp-cli monitor create ai-labs --query 'from:openai OR from:anthropic' --agent
  x-twitter-pp-cli monitor create product-mentions --url https://example.com --agent
  x-twitter-pp-cli monitor create founder --account @sama --agent`,
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			def, err := buildMonitorDefinition(args[0], query, sourceURL, account)
			if err != nil {
				return err
			}
			db, err := openWorkflowDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := upsertMonitorDefinition(cmd, db, def); err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return flags.printTable(cmd, []string{"NAME", "KIND", "QUERY"}, [][]string{{def.Name, def.Kind, def.Query}})
			}
			return printJSONFiltered(cmd.OutOrStdout(), workflowEnvelope{
				Meta:    workflowMeta("monitor create", "local"),
				Results: def,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local database")
	cmd.Flags().StringVar(&query, "query", "", "Recent-search query to monitor")
	cmd.Flags().StringVar(&sourceURL, "url", "", "URL or domain to monitor with X URL search")
	cmd.Flags().StringVar(&account, "account", "", "Account username to monitor with from:<username>")
	return cmd
}

func newNovelMonitorRunCmd(flags *rootFlags) *cobra.Command {
	var dbPath, since, collection string
	var limit int
	var preview bool
	cmd := &cobra.Command{
		Use:   "run <name>",
		Short: "Run a monitor and emit only new deduped results by default",
		Example: `  x-twitter-pp-cli monitor run ai-labs --since last --agent
  x-twitter-pp-cli monitor run product-mentions --since 24h --preview --agent`,
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openWorkflowDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			def, err := getMonitorDefinition(cmd, db, args[0])
			if err != nil {
				return err
			}
			result, err := runMonitor(cmd, flags, db, def, since, limit, preview, collection)
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				rows := make([][]string, 0, len(result.Results))
				for _, item := range result.Results {
					rows = append(rows, []string{item.TweetID, item.URL, authorDisplay(item.Author), truncatePlain(item.Text, 88)})
				}
				return flags.printTable(cmd, []string{"ID", "URL", "AUTHOR", "TEXT"}, rows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), workflowEnvelope{
				Meta:    workflowMeta("monitor run", "live"),
				Results: result,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local database")
	cmd.Flags().StringVar(&since, "since", "last", "Window to query: last, 24h, 7d, RFC3339, or YYYY-MM-DD")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum search results to inspect")
	cmd.Flags().BoolVar(&preview, "preview", false, "Fetch results without updating watermark or dedupe state")
	cmd.Flags().StringVar(&collection, "collection", "", "Also save new results into a local collection")
	return cmd
}

func newNovelMonitorListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List configured local X monitors",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openWorkflowDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			monitors, err := listMonitorDefinitions(cmd, db)
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				rows := make([][]string, 0, len(monitors))
				for _, m := range monitors {
					rows = append(rows, []string{m.Name, m.Kind, m.Watermark, m.UpdatedAt})
				}
				return flags.printTable(cmd, []string{"NAME", "KIND", "WATERMARK", "UPDATED"}, rows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), workflowEnvelope{
				Meta:    workflowMeta("monitor list", "local"),
				Results: monitors,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local database")
	return cmd
}

func buildMonitorDefinition(name, query, sourceURL, account string) (monitorDefinition, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return monitorDefinition{}, usageErr(fmt.Errorf("monitor name required"))
	}
	def := monitorDefinition{Name: name}
	set := 0
	if strings.TrimSpace(query) != "" {
		def.Kind = "query"
		def.Query = strings.TrimSpace(query)
		set++
	}
	if strings.TrimSpace(sourceURL) != "" {
		q, target, err := normalizeURLMentionQuery(sourceURL)
		if err != nil {
			return monitorDefinition{}, err
		}
		def.Kind = "url"
		def.Query = q
		def.SourceURL = target
		set++
	}
	if strings.TrimSpace(account) != "" {
		acct, _ := normalizeAccountInput(account)
		if acct == "" {
			return monitorDefinition{}, usageErr(fmt.Errorf("invalid --account %q", account))
		}
		def.Kind = "account"
		def.Query = "from:" + acct
		def.Account = acct
		set++
	}
	if set != 1 {
		return monitorDefinition{}, usageErr(fmt.Errorf("set exactly one of --query, --url, or --account"))
	}
	return def, nil
}

func upsertMonitorDefinition(cmd *cobra.Command, db *store.Store, def monitorDefinition) error {
	now := generatedAt()
	_, err := db.DB().ExecContext(cmd.Context(),
		`INSERT INTO workflow_monitors(name, kind, query, source_url, account, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET
		   kind = excluded.kind,
		   query = excluded.query,
		   source_url = excluded.source_url,
		   account = excluded.account,
		   updated_at = excluded.updated_at`,
		def.Name, def.Kind, def.Query, def.SourceURL, def.Account, now, now)
	return err
}

func getMonitorDefinition(cmd *cobra.Command, db *store.Store, name string) (monitorDefinition, error) {
	var def monitorDefinition
	var sourceURL, account, watermark, lastRun sql.NullString
	err := db.DB().QueryRowContext(cmd.Context(),
		`SELECT name, kind, query, source_url, account, watermark_id, last_run_at, created_at, updated_at
		 FROM workflow_monitors WHERE name = ?`, name).
		Scan(&def.Name, &def.Kind, &def.Query, &sourceURL, &account, &watermark, &lastRun, &def.CreatedAt, &def.UpdatedAt)
	if err == sql.ErrNoRows {
		return def, notFoundErr(fmt.Errorf("monitor %q not found", name))
	}
	if err != nil {
		return def, err
	}
	def.SourceURL = sourceURL.String
	def.Account = account.String
	def.Watermark = watermark.String
	def.LastRunAt = lastRun.String
	return def, nil
}

func listMonitorDefinitions(cmd *cobra.Command, db *store.Store) ([]monitorDefinition, error) {
	rows, err := db.DB().QueryContext(cmd.Context(),
		`SELECT name, kind, query, source_url, account, watermark_id, last_run_at, created_at, updated_at
		 FROM workflow_monitors ORDER BY updated_at DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []monitorDefinition
	for rows.Next() {
		var def monitorDefinition
		var sourceURL, account, watermark, lastRun sql.NullString
		if err := rows.Scan(&def.Name, &def.Kind, &def.Query, &sourceURL, &account, &watermark, &lastRun, &def.CreatedAt, &def.UpdatedAt); err != nil {
			return nil, err
		}
		def.SourceURL = sourceURL.String
		def.Account = account.String
		def.Watermark = watermark.String
		def.LastRunAt = lastRun.String
		out = append(out, def)
	}
	return out, rows.Err()
}

func runMonitor(cmd *cobra.Command, flags *rootFlags, db *store.Store, def monitorDefinition, since string, limit int, preview bool, collection string) (monitorRunResult, error) {
	watermarkBefore := ""
	if strings.TrimSpace(strings.ToLower(since)) == "last" {
		watermarkBefore = def.Watermark
	}
	records, err := recentSearchRecords(cmd, flags, def.Query, limit, since, watermarkBefore, parseIncludeSet("author,media,links,refs,metrics"))
	if err != nil {
		return monitorRunResult{}, classifyAPIError(err, flags)
	}
	runID := generatedAt()
	result := monitorRunResult{
		Monitor:         def,
		RunID:           runID,
		Preview:         preview,
		WatermarkBefore: watermarkBefore,
		Collection:      collection,
	}
	maxID := watermarkBefore
	var collectionRecords []*resolvedPostRecord
	for _, rec := range records {
		if rec == nil {
			continue
		}
		item := collectionItemFromPost(rec, runID)
		if preview {
			exists, err := monitorResultExists(cmd, db, def.Name, rec.TweetID)
			if err != nil {
				return result, err
			}
			if exists {
				result.SkippedDuplicates++
			} else {
				result.Results = append(result.Results, item)
				result.NewResults++
			}
		} else {
			added, err := saveMonitorResult(cmd, db, def.Name, rec, runID)
			if err != nil {
				return result, err
			}
			if added {
				result.Results = append(result.Results, item)
				result.NewResults++
				if collection != "" {
					collectionRecords = append(collectionRecords, rec)
				}
			} else {
				result.SkippedDuplicates++
			}
		}
		if compareTweetID(rec.TweetID, maxID) > 0 {
			maxID = rec.TweetID
		}
	}
	result.WatermarkAfter = maxID
	if !preview {
		if collection != "" && len(collectionRecords) > 0 {
			added, duplicates, _, err := saveCollectionItems(cmd, db, collection, collectionRecords, "saved from monitor "+def.Name, nil)
			if err != nil {
				return result, err
			}
			result.CollectionAdded = added
			result.CollectionDuplicates = duplicates
		}
		if err := updateMonitorWatermark(cmd, db, def.Name, maxID, runID); err != nil {
			return result, err
		}
	}
	return result, nil
}

func monitorResultExists(cmd *cobra.Command, db *store.Store, monitor, tweetID string) (bool, error) {
	var one int
	err := db.DB().QueryRowContext(cmd.Context(),
		`SELECT 1 FROM workflow_monitor_results WHERE monitor_name = ? AND tweet_id = ?`,
		monitor, tweetID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func saveMonitorResult(cmd *cobra.Command, db *store.Store, monitor string, rec *resolvedPostRecord, runID string) (bool, error) {
	raw, err := json.Marshal(rec)
	if err != nil {
		return false, err
	}
	result, err := db.DB().ExecContext(cmd.Context(),
		`INSERT INTO workflow_monitor_results(monitor_name, tweet_id, tweet_json, source_url, seen_at, run_id)
		 VALUES(?, ?, ?, ?, ?, ?)
		 ON CONFLICT(monitor_name, tweet_id) DO NOTHING`,
		monitor, rec.TweetID, string(raw), rec.URL, runID, runID)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func updateMonitorWatermark(cmd *cobra.Command, db *store.Store, name, watermark, now string) error {
	_, err := db.DB().ExecContext(cmd.Context(),
		`UPDATE workflow_monitors SET watermark_id = ?, last_run_at = ?, updated_at = ? WHERE name = ?`,
		watermark, now, now, name)
	return err
}

func compareTweetID(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) != len(b) {
		if len(a) > len(b) {
			return 1
		}
		return -1
	}
	if a > b {
		return 1
	}
	return -1
}
