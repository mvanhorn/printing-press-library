// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
	"github.com/spf13/cobra"
)

type indexRootResult struct {
	Root     string `json:"root"`
	Mode     string `json:"mode"`
	Pages    int    `json:"pages"`
	Upserted int    `json:"upserted"`
	Deleted  int    `json:"deleted"`
	Complete bool   `json:"complete"`
}

type indexResult struct {
	Roots         []indexRootResult `json:"roots"`
	TotalUpserted int               `json:"total_upserted"`
	TotalDeleted  int               `json:"total_deleted"`
	DurationMS    int64             `json:"duration_ms"`
	AccountType   string            `json:"account_type"`
	Curtailed     bool              `json:"curtailed,omitempty"`
	SkippedRoots  *int              `json:"skipped_roots,omitempty"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newIndexCmd(flags))
	})
}

func newIndexCmd(flags *rootFlags) *cobra.Command {
	var dbPath, selectedRoot string
	var full, rebind, yes bool
	var maxPages int
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index Dropbox metadata in a local SQLite database",
		Long:  "Crawl Dropbox metadata into a local SQLite index. Subsequent runs follow saved cursors to apply changes, including deletions.",
		Example: strings.Trim(`
  dropbox-pp-cli index --agent
  dropbox-pp-cli index --root "/Camera Uploads" --full --agent`, "\n"),
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				roots := []string{""}
				if selectedRoot != "" {
					roots = []string{strings.ToLower(selectedRoot)}
				}
				return printJSONFiltered(cmd.OutOrStdout(), struct {
					DryRun bool     `json:"dry_run"`
					Roots  []string `json:"roots"`
					Would  string   `json:"would"`
				}{true, roots, "crawl the listed roots and any top-level folders discovered at the Dropbox root"}, flags)
			}
			if maxPages < 0 {
				return usageErr(fmt.Errorf("--max-pages must be nonnegative"))
			}
			if rebind {
				if !isatty.IsTerminal(os.Stdin.Fd()) {
					return usageErr(fmt.Errorf("index --rebind clears the local index; run it yourself in a terminal"))
				}
				if cliutil.IsAnyHarness() {
					return usageErr(fmt.Errorf("--rebind is unavailable in a harness"))
				}
				if !yes {
					return usageErr(fmt.Errorf("--rebind requires --yes to clear the index"))
				}
			}
			if err := liveSourceOnly(flags); err != nil {
				return err
			}
			if selectedRoot != "" && (selectedRoot[0] != '/' || strings.Count(strings.Trim(selectedRoot, "/"), "/") != 0 || selectedRoot == "/") {
				return usageErr(fmt.Errorf("--root must be a top-level Dropbox folder path"))
			}
			if cliutil.IsDogfoodEnv() {
				maxPages = 1
			}
			if dbPath == "" {
				dbPath = defaultDBPath("dropbox-pp-cli")
			}
			ctx := cmd.Context()
			cancel := func() {}
			if cmd.Flags().Changed("timeout") {
				ctx, cancel = boundCtx(ctx, flags)
			}
			defer cancel()
			started := time.Now()
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.EnsureDropboxSchema(ctx); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			info, err := fetchDropboxAccount(ctx, c)
			if err != nil {
				return err
			}
			if err := bindDropboxIndexAccount(ctx, db, info, rebind); err != nil {
				return err
			}
			headers := pathRootHeaders(info)
			result := indexResult{Roots: make([]indexRootResult, 0), AccountType: info.AccountType}
			if cliutil.IsDogfoodEnv() {
				zero := 0
				result.Curtailed = true
				result.SkippedRoots = &zero
			}
			if selectedRoot == "" {
				oldFolders, err := db.ListDropboxTopFolders(ctx)
				if err != nil {
					return err
				}
				rootResult, err := crawlDropboxRoot(ctx, cmd, c, db, "", "", full, maxPages, headers)
				if err != nil {
					return err
				}
				result.Roots = append(result.Roots, rootResult)
				folders, err := db.ListDropboxTopFolders(ctx)
				if err != nil {
					return err
				}
				if rootResult.Complete {
					current := make(map[string]bool, len(folders))
					for _, p := range folders {
						current[p] = true
					}
					for _, p := range oldFolders {
						if !current[p] {
							if _, err := db.DeleteDropboxPathPrefix(ctx, p); err != nil {
								return err
							}
							if err := db.DeleteDropboxIndexState(ctx, p); err != nil {
								return err
							}
						}
					}
				}
				if cliutil.IsDogfoodEnv() {
					sort.Strings(folders)
					if len(folders) > 3 {
						*result.SkippedRoots = len(folders) - 3
						folders = folders[:3]
					}
				}
				for _, folder := range folders {
					r, err := crawlDropboxRoot(ctx, cmd, c, db, folder, folder, full, maxPages, headers)
					if err != nil {
						return err
					}
					result.Roots = append(result.Roots, r)
				}
			} else {
				root := strings.ToLower(strings.TrimSuffix(selectedRoot, "/"))
				r, err := crawlDropboxRoot(ctx, cmd, c, db, root, selectedRoot, full, maxPages, headers)
				if err != nil {
					return err
				}
				result.Roots = append(result.Roots, r)
			}
			for _, r := range result.Roots {
				result.TotalUpserted += r.Upserted
				result.TotalDeleted += r.Deleted
			}
			result.DurationMS = time.Since(started).Milliseconds()
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			for _, r := range result.Roots {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s, %d pages, %d upserted, %d deleted, complete=%t\n", r.Root, r.Mode, r.Pages, r.Upserted, r.Deleted, r.Complete)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite index path")
	cmd.Flags().StringVar(&selectedRoot, "root", "", "Index only one top-level folder")
	cmd.Flags().BoolVar(&full, "full", false, "Ignore saved cursors and recrawl")
	cmd.Flags().BoolVar(&rebind, "rebind", false, "Clear indexed files and roots, then bind this account while retaining journals")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm clearing the index with --rebind")
	cmd.Flags().IntVar(&maxPages, "max-pages", 0, "Maximum pages per root (0 means unlimited)")
	return cmd
}

func crawlDropboxRoot(ctx context.Context, cmd *cobra.Command, c *client.Client, db *store.Store, root, apiPath string, forceFull bool, maxPages int, headers map[string]string) (indexRootResult, error) {
	state, found, err := db.GetDropboxIndexState(ctx, root)
	if err != nil {
		return indexRootResult{}, err
	}
	mode := "incremental"
	if forceFull || !found || state.Cursor == "" {
		mode = "full"
	}
	if found && !state.Complete && state.Cursor != "" && state.LastIncrementalAt == "" {
		mode = "full"
	}
	if mode == "full" && (forceFull || state.Cursor == "") {
		if err := db.ClearDropboxRoot(ctx, root); err != nil {
			return indexRootResult{}, err
		}
		state = store.DropboxIndexState{Root: root}
		if err := db.SetDropboxIndexState(ctx, state); err != nil {
			return indexRootResult{}, err
		}
	}
	result := indexRootResult{Root: root, Mode: mode}
	resetTried := false
	fmt.Fprintf(cmd.ErrOrStderr(), "index root %q (%s)\n", root, mode)
	for maxPages == 0 || result.Pages < maxPages {
		var raw json.RawMessage
		if state.Cursor == "" {
			body := map[string]any{"path": apiPath, "recursive": root != "", "limit": 2000, "include_deleted": false, "include_non_downloadable_files": true}
			raw, _, err = c.PostQueryWithParamsAndHeaders(ctx, "/files/list_folder", nil, body, headers)
		} else {
			raw, _, err = c.PostQueryWithParamsAndHeaders(ctx, "/files/list_folder/continue", nil, map[string]string{"cursor": state.Cursor}, headers)
		}
		if err != nil {
			if state.Cursor != "" && !resetTried && dropbox.HasSummaryPrefix(err, "reset") {
				resetTried = true
				if err := db.ClearDropboxRoot(ctx, root); err != nil {
					return result, err
				}
				state = store.DropboxIndexState{Root: root}
				if err := db.SetDropboxIndexState(ctx, state); err != nil {
					return result, err
				}
				result.Mode = "full"
				result.Pages = 0
				result.Upserted = 0
				result.Deleted = 0
				continue
			}
			return result, err
		}
		var page struct {
			Cursor  string `json:"cursor"`
			HasMore bool   `json:"has_more"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return result, err
		}
		if page.Cursor == "" {
			return result, fmt.Errorf("Dropbox list_folder response lacks cursor")
		}
		entries, err := dropbox.ParseEntries(raw)
		if err != nil {
			return result, err
		}
		rows := make([]store.DropboxRow, 0, len(entries))
		deleted := make([]string, 0)
		for _, e := range entries {
			if e.Tag == "deleted" {
				deleted = append(deleted, e.PathLower)
				continue
			}
			if e.Tag != "file" && e.Tag != "folder" {
				continue
			}
			rows = append(rows, dropboxEntryRow(e))
		}
		state.Root = root
		state.Cursor = page.Cursor
		state.Complete = !page.HasMore
		stamp := time.Now().UTC().Format(time.RFC3339)
		if result.Mode == "full" {
			state.LastFullAt = stamp
			state.LastIncrementalAt = ""
		} else {
			state.LastIncrementalAt = stamp
		}
		state.Entries += len(rows)
		n, err := db.ApplyDropboxPage(ctx, root, rows, deleted, state)
		if err != nil {
			return result, err
		}
		result.Pages++
		result.Upserted += len(rows)
		result.Deleted += int(n)
		result.Complete = !page.HasMore
		fmt.Fprintf(cmd.ErrOrStderr(), "index root %q page %d: %d entries\n", root, result.Pages, len(entries))
		if !page.HasMore {
			break
		}
	}
	return result, nil
}

func dropboxEntryRow(e dropbox.Entry) store.DropboxRow {
	row := store.DropboxRow{PathLower: e.PathLower, ID: e.ID, Tag: e.Tag, Name: e.Name, PathDisplay: e.PathDisplay, ParentLower: e.ParentLower, Rev: e.Rev, Size: e.Size, ContentHash: e.ContentHash, SharedFolderID: e.SharedFolderID, ParentSharedFolderID: e.ParentSharedFolderID, IsDownloadable: e.IsDownloadable}
	if !e.ClientModified.IsZero() {
		row.ClientModified = e.ClientModified.UTC().Format(time.RFC3339)
	}
	if !e.ServerModified.IsZero() {
		row.ServerModified = e.ServerModified.UTC().Format(time.RFC3339)
	}
	return row
}
