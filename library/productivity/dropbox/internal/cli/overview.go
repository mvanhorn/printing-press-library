// pp:data-source auto
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type overviewTotals struct {
	Files   int   `json:"files"`
	Folders int   `json:"folders"`
	Bytes   int64 `json:"bytes"`
}
type overviewBucket struct {
	Name  string `json:"name"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
}
type overviewYear struct {
	Year  string `json:"year"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
}
type overviewLargeFile struct {
	PathDisplay string `json:"path_display"`
	Size        int64  `json:"size"`
}
type overviewDuplicates struct {
	Groups           int   `json:"groups"`
	Files            int   `json:"files"`
	ReclaimableBytes int64 `json:"reclaimable_bytes"`
}
type overviewIndex struct {
	Roots           int    `json:"roots"`
	IncompleteRoots int    `json:"incomplete_roots"`
	LastIndexedAt   string `json:"last_indexed_at"`
}
type overviewQuota struct {
	Used           int64  `json:"used"`
	Allocated      int64  `json:"allocated"`
	AllocationType string `json:"allocation_type"`
}
type overviewDevDir struct {
	Kind    string `json:"kind"`
	Files   int    `json:"files"`
	Bytes   int64  `json:"bytes"`
	Folders int    `json:"folders"`
}
type overviewDevFolder struct {
	Path  string `json:"path"`
	Kind  string `json:"kind"`
	Bytes int64  `json:"bytes"`
	Files int    `json:"files"`
}
type overviewResult struct {
	Quota            *overviewQuota      `json:"quota"`
	IndexMissing     bool                `json:"index_missing,omitempty"`
	Note             string              `json:"note,omitempty"`
	Totals           overviewTotals      `json:"totals"`
	TopFolders       []overviewBucket    `json:"top_folders"`
	ByType           []overviewBucket    `json:"by_type"`
	ByYear           []overviewYear      `json:"by_year"`
	LargestFiles     []overviewLargeFile `json:"largest_files"`
	Duplicates       overviewDuplicates  `json:"duplicates"`
	ConflictedCopies int                 `json:"conflicted_copies"`
	Index            overviewIndex       `json:"index"`
	DevDirs          []overviewDevDir    `json:"dev_dirs"`
	TopDevFolders    []overviewDevFolder `json:"top_dev_folders"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelOverviewCmd(flags))
	})
}

func newNovelOverviewCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var top int
	cmd := &cobra.Command{
		Use: "overview", Short: "Summarize quota and indexed Dropbox storage",
		Long: "Use this command for a one-screen account summary across the whole Dropbox. Do NOT use it to inspect one folder's structure; use 'tree' instead.",
		Example: strings.Trim(`
  dropbox-pp-cli overview --agent
  dropbox-pp-cli overview --top 20 --agent`, "\n"),
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "overview")
			}
			if top < 0 {
				return usageErr(fmt.Errorf("--top must be nonnegative"))
			}
			result := newOverviewResult()
			// The aggregates always come from the local index; quota is separate.
			flags.agentSource = "local"
			if flags.dataSource == "local" {
				result.Note = "quota unavailable: local data source"
			} else {
				client, err := flags.newClient()
				if err != nil {
					result.Note = "quota unavailable: " + err.Error()
				} else {
					ctx, cancel := boundCtx(cmd.Context(), flags)
					defer cancel()
					raw, _, err := client.PostQueryWithParamsAndHeaders(ctx, "/users/get_space_usage", nil, nil, nil)
					if err != nil {
						result.Note = "quota unavailable: " + err.Error()
					} else {
						var response struct {
							Used       int64 `json:"used"`
							Allocation struct {
								Tag        string `json:".tag"`
								Allocated  int64  `json:"allocated"`
								Individual struct {
									Allocated int64 `json:"allocated"`
								} `json:"individual"`
							} `json:"allocation"`
						}
						if err := json.Unmarshal(raw, &response); err != nil {
							result.Note = "quota unavailable: " + err.Error()
						} else {
							allocated := response.Allocation.Allocated
							if allocated == 0 {
								allocated = response.Allocation.Individual.Allocated
							}
							result.Quota = &overviewQuota{response.Used, allocated, response.Allocation.Tag}
						}
					}
				}
			}
			db, found, err := openIndex(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if !found {
				result.IndexMissing = true
				// Keep the quota failure reason so a null quota is never unexplained.
				if result.Note != "" {
					result.Note = missingIndexNote + "; " + result.Note
				} else {
					result.Note = missingIndexNote
				}
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			defer db.Close()
			quota, note := result.Quota, result.Note
			result, err = queryOverviewFast(cmd.Context(), db, top)
			if err != nil {
				return err
			}
			result.Quota, result.Note = quota, note
			indexRows, err := db.DB().QueryContext(cmd.Context(), `SELECT COALESCE(last_full_at,''),COALESCE(last_incremental_at,''),COALESCE(complete,0) FROM dbx_index_state`)
			if err != nil {
				return err
			}
			for indexRows.Next() {
				var full, incremental string
				var complete bool
				if err := indexRows.Scan(&full, &incremental, &complete); err != nil {
					_ = indexRows.Close()
					return err
				}
				result.Index.Roots++
				if !complete {
					result.Index.IncompleteRoots++
				}
				stamp := incremental
				if stamp == "" {
					stamp = full
				}
				if stamp > result.Index.LastIndexedAt {
					result.Index.LastIndexedAt = stamp
				}
			}
			err = indexRows.Err()
			_ = indexRows.Close()
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d files, %d folders, %d bytes\n", result.Totals.Files, result.Totals.Folders, result.Totals.Bytes)
			if result.Quota != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Quota: %d of %d bytes\n", result.Quota.Used, result.Quota.Allocated)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), result.Note)
			}
			for _, folder := range result.TopFolders {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d bytes\t%d files\n", folder.Name, folder.Bytes, folder.Files)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite index path")
	cmd.Flags().IntVar(&top, "top", 15, "Maximum folder and type buckets")
	return cmd
}

func newOverviewResult() overviewResult {
	return overviewResult{TopFolders: make([]overviewBucket, 0), ByType: make([]overviewBucket, 0), ByYear: make([]overviewYear, 0), LargestFiles: make([]overviewLargeFile, 0), DevDirs: make([]overviewDevDir, 0), TopDevFolders: make([]overviewDevFolder, 0)}
}
