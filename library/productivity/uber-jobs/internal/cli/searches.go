// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelSearchesCmd(flags))
	})
}

// savedSearchRow is a saved search with the id key every row carries.
type savedSearchRow struct {
	ID string `json:"id"`
	uberjobs.SavedSearch
}

// deletedSearchRow reports one deleted saved search.
type deletedSearchRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
}

func newNovelSearchesCmd(flags *rootFlags) *cobra.Command {
	var dbPath, deleteName string
	cmd := &cobra.Command{
		Use:   "searches",
		Short: "Show every saved search with its filters, baseline size and last advance, or delete one by name",
		Long: strings.Trim(`
List every saved search in the local store, or delete one with --delete <name>

Each row shows the filters new applies, baseline_size (how many postings the
search held at its last complete scan), and when new last advanced it. An empty
store lists nothing and exits 0. Deleting a name that does not exist exits
not-found (3); deleting removes the search and its baseline.`, "\n"),
		Example: strings.Trim(`
  uber-jobs-pp-cli searches --json
  uber-jobs-pp-cli searches --delete uk-strategy --json`, "\n"),
		Annotations: map[string]string{
			"mcp:local-write":    "true",
			"pp:data-source":     "local",
			"pp:live-happy-path": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("searches takes no arguments; to delete one use --delete %s", args[0]))
			}
			del := strings.TrimSpace(deleteName)
			if cmd.Flags().Changed("delete") && del == "" {
				return usageErr(fmt.Errorf("--delete needs a saved-search name"))
			}
			if dryRunOK(flags) {
				if del != "" {
					return writeDryRun(cmd.OutOrStdout(), flags, fmt.Sprintf("delete saved search %q and its baseline from the local store at %s", del, uberDBPath(dbPath)))
				}
				return writeDryRun(cmd.OutOrStdout(), flags, "list saved searches in the local store at "+uberDBPath(dbPath))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, db, err := openUberStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			if del != "" {
				ok, err := uberjobs.DeleteSearch(ctx, db, del)
				if err != nil {
					return fmt.Errorf("deleting saved search %q: %w", del, err)
				}
				if !ok {
					return notFoundErr(fmt.Errorf("no saved search named %q; run: uber-jobs-pp-cli searches", del))
				}
				env := uberjobs.NewEnvelope("searches", uberjobs.SourceLocal, []deletedSearchRow{{ID: del, Name: del, Deleted: true}}, 1)
				env.Hits = 1
				env.Meta.Complete = true
				return printEnvelope(cmd, flags, env, func(w io.Writer) error {
					_, err := fmt.Fprintf(w, "deleted saved search %s\n", del)
					return err
				})
			}
			list, err := uberjobs.ListSearches(ctx, db)
			if err != nil {
				return fmt.Errorf("listing saved searches: %w", err)
			}
			rows := make([]savedSearchRow, 0, len(list))
			for _, sv := range list {
				rows = append(rows, savedSearchRow{ID: sv.Name, SavedSearch: sv})
			}
			env := uberjobs.NewEnvelope("searches", uberjobs.SourceLocal, rows, len(rows))
			env.Hits = len(rows)
			env.Scanned = len(rows)
			env.Meta.Complete = true
			if len(rows) == 0 {
				env.Meta.Note = "no saved searches; create one with: uber-jobs-pp-cli save <name> --country GBR"
			}
			return printEnvelope(cmd, flags, env, func(w io.Writer) error {
				if len(rows) == 0 {
					_, err := fmt.Fprintln(w, env.Meta.Note)
					return err
				}
				tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
				fmt.Fprintln(tw, "NAME\tBASELINE\tLAST ADVANCED\tFILTERS")
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", r.Name, r.BaselineSize, deref(r.LastAdvancedAt, "never"), describeFilters(r.Filters))
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&deleteName, "delete", "", "Delete this saved search and its baseline")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI data dir)")
	return cmd
}
