// Hand-authored: novel feature `prune` — cross-workspace status-filtered
// batch delete with dry-run by default. Created during Phase 3 build;
// survives regen because it lives in its own file.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/orgo/internal/store"

	"github.com/spf13/cobra"
)

func newPruneCmd(flags *rootFlags) *cobra.Command {
	var statusList string
	var olderThan string

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Cross-workspace status-filtered batch delete with dry-run by default. Pass --yes to actually delete.",
		Long: `Walk the local computers store, select rows whose status matches
--status and whose created_at is older than --older-than, and delete
them via the live API.

DRY-RUN BY DEFAULT. The command refuses to mutate without --yes. Pair
with --agent for non-interactive scripts; the agent expansion sets
--yes for you.`,
		Example: strings.Trim(`
  # Preview: every suspended/error computer older than 7 days
  orgo-pp-cli prune --status suspended,error --older-than 7d

  # Same query, but actually delete (yes is required to proceed)
  orgo-pp-cli prune --status suspended,error --older-than 7d --yes

  # Default --status of "suspended,error", default --older-than of 7d
  orgo-pp-cli prune
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			// Brief: dry-run by default; only --yes (or --agent which sets
			// --yes) flips it. Persistent --dry-run never triggers a real
			// delete regardless of --yes.
			effectiveDryRun := !flags.yes || flags.dryRun

			cutoff, err := parseSinceDuration(olderThan)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --older-than value %q: %w", olderThan, err))
			}

			statuses := map[string]bool{}
			for _, s := range strings.Split(statusList, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					statuses[s] = true
				}
			}
			if len(statuses) == 0 {
				return usageErr(fmt.Errorf("--status must be a non-empty comma-separated list"))
			}

			dbPath := defaultDBPath("orgo-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			computerRows, source, err := loadFleetComputers(flags, db)
			if err != nil {
				return fmt.Errorf("loading computers: %w", err)
			}
			if len(computerRows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No computers found locally or via the live API.")
				return notFoundErr(fmt.Errorf("no computers found"))
			}
			_ = source // provenance label, unused for now

			var matches []pruneRow
			for _, raw := range computerRows {
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}
				status := stringField(obj, "status")
				if !statuses[status] {
					continue
				}
				createdStr := stringField(obj, "created_at")
				if createdStr == "" {
					continue
				}
				created, terr := time.Parse(time.RFC3339, createdStr)
				if terr != nil {
					// Try without nano precision
					created, terr = time.Parse(time.RFC3339Nano, createdStr)
					if terr != nil {
						continue
					}
				}
				if created.After(cutoff) {
					continue
				}
				matches = append(matches, pruneRow{
					ID:        stringField(obj, "id"),
					Name:      stringField(obj, "name"),
					Status:    status,
					CreatedAt: createdStr,
					Workspace: stringField(obj, "workspace_id"),
				})
			}

			sort.SliceStable(matches, func(i, j int) bool { return matches[i].CreatedAt < matches[j].CreatedAt })

			if len(matches) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No computers match the prune filter.")
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), []pruneRow{}, flags)
				}
				return nil
			}

			if effectiveDryRun {
				for i := range matches {
					matches[i].Action = "would-delete"
				}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), matches, flags)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "DRY-RUN — re-run with --yes to delete %d computer(s).\n", len(matches))
				return renderPruneTable(cmd.OutOrStdout(), matches)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			for i := range matches {
				path := replacePathParam("/computers/{id}", "id", matches[i].ID)
				_, _, derr := c.Delete(path)
				if derr != nil {
					matches[i].Action = "failed"
					matches[i].Error = derr.Error()
				} else {
					matches[i].Action = "deleted"
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), matches, flags)
			}
			return renderPruneTable(cmd.OutOrStdout(), matches)
		},
	}

	cmd.Flags().StringVar(&statusList, "status", "suspended,error", "Comma-separated status values to prune")
	cmd.Flags().StringVar(&olderThan, "older-than", "7d", "Only prune computers created before this duration ago (e.g. 7d, 24h)")

	return cmd
}

type pruneRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	Action    string `json:"action"`
	Workspace string `json:"workspace_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

func renderPruneTable(w io.Writer, rows []pruneRow) error {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, bold("ID")+"\t"+bold("NAME")+"\t"+bold("STATUS")+"\t"+bold("CREATED_AT")+"\t"+bold("ACTION"))
	for _, r := range rows {
		action := r.Action
		if r.Error != "" {
			action = action + ": " + truncate(r.Error, 60)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", truncate(r.ID, 12), r.Name, r.Status, r.CreatedAt, action)
	}
	return tw.Flush()
}
