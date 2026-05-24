// Commands: lists (create/list/show/add/annotate/kill/export), watch (add/list/remove/run)
package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/source/rdap"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/store"
)

func newListsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lists",
		Short: "Manage candidate shortlists (saved domain sets with notes and tags).",
	}
	cmd.AddCommand(newListsCreateCmd(flags))
	cmd.AddCommand(newListsListCmd(flags))
	cmd.AddCommand(newListsShowCmd(flags))
	cmd.AddCommand(newListsAddCmd(flags))
	cmd.AddCommand(newListsAnnotateCmd(flags))
	cmd.AddCommand(newListsKillCmd(flags))
	return cmd
}

func newListsCreateCmd(flags *rootFlags) *cobra.Command {
	var description string
	cmd := &cobra.Command{
		Use:         "create <name>",
		Short:       "Create a new candidate shortlist.",
		Example:     `  domain-goat-pp-cli lists create ai-startup --description "AI-startup-name candidates"`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			name := args[0]
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "name": name})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.CreateList(cmd.Context(), name, description); err != nil {
				return apiErr(err)
			}
			return emitJSON(cmd, flags, map[string]any{"created": name, "description": description})
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "Human description")
	return cmd
}

func newListsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all shortlists with sizes.",
		Example: `  domain-goat-pp-cli lists list
  domain-goat-pp-cli lists list --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.ListLists(cmd.Context())
			if err != nil {
				return apiErr(err)
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, rows)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "NAME\tSIZE\tDESCRIPTION")
			for _, l := range rows {
				fmt.Fprintf(tw, "%s\t%d\t%s\n", l.Name, l.Size, l.Description)
			}
			return tw.Flush()
		},
	}
	return cmd
}

func newListsShowCmd(flags *rootFlags) *cobra.Command {
	var includeKilled bool
	cmd := &cobra.Command{
		Use:         "show <name>",
		Short:       "Show all candidates in one shortlist.",
		Example:     `  domain-goat-pp-cli lists show ai-startup --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "list": args[0]})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.ListCandidates(cmd.Context(), args[0], includeKilled)
			if err != nil {
				return apiErr(err)
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, rows)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DOMAIN\tKILLED\tTAGS\tNOTES")
			for _, c := range rows {
				fmt.Fprintf(tw, "%s\t%v\t%s\t%s\n", c.FQDN, c.Killed, c.Tags, truncate(c.Notes, 60))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&includeKilled, "include-killed", false, "Include killed candidates")
	return cmd
}

func newListsAddCmd(flags *rootFlags) *cobra.Command {
	var notes, tags string
	cmd := &cobra.Command{
		Use:   "add <list-name> <domain...>",
		Short: "Add one or more domains to a shortlist.",
		Example: `  domain-goat-pp-cli lists add ai-startup kindred.io lumen.ai novella.studio
  domain-goat-pp-cli lists add brand-sprint kindred.io --notes "founder favorite" --tags founder,short`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			listName := args[0]
			fqdns, err := normalizeAll(args[1:])
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "list": listName, "fqdns": fqdns})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			added := 0
			for _, f := range fqdns {
				if err := s.AddCandidate(cmd.Context(), store.CandidateRow{
					ListName: listName, FQDN: f, Notes: notes, Tags: tags,
				}); err == nil {
					added++
				}
			}
			return emitJSON(cmd, flags, map[string]any{"list": listName, "added": added})
		},
	}
	cmd.Flags().StringVar(&notes, "notes", "", "Notes for these candidates")
	cmd.Flags().StringVar(&tags, "tags", "", "Comma-separated tags")
	return cmd
}

func newListsAnnotateCmd(flags *rootFlags) *cobra.Command {
	var notes, tags, list string
	cmd := &cobra.Command{
		Use:         "annotate <domain>",
		Short:       "Update notes/tags on an existing candidate.",
		Example:     `  domain-goat-pp-cli lists annotate kindred.io --list ai-startup --notes "trademark check pending" --tags review`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || list == "" {
				return usageErr(fmt.Errorf("usage: annotate <domain> --list <list-name>"))
			}
			fqdn := strings.ToLower(args[0])
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "fqdn": fqdn})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.AddCandidate(cmd.Context(), store.CandidateRow{
				ListName: list, FQDN: fqdn, Notes: notes, Tags: tags,
			}); err != nil {
				return apiErr(err)
			}
			return emitJSON(cmd, flags, map[string]any{"annotated": fqdn, "list": list})
		},
	}
	cmd.Flags().StringVar(&list, "list", "", "List name (required)")
	cmd.Flags().StringVar(&notes, "notes", "", "Notes to set")
	cmd.Flags().StringVar(&tags, "tags", "", "Tags to set (comma-separated)")
	return cmd
}

func newListsKillCmd(flags *rootFlags) *cobra.Command {
	var reason, list string
	cmd := &cobra.Command{
		Use:         "kill <domain>",
		Short:       "Mark a candidate as killed (with reason) — keeps it in the list for `why-killed` audit.",
		Example:     `  domain-goat-pp-cli lists kill kindred.studio --list brand-sprint --reason "trademark conflict"`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || list == "" {
				return usageErr(fmt.Errorf("usage: kill <domain> --list <list-name> --reason <text>"))
			}
			fqdn := strings.ToLower(args[0])
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "fqdn": fqdn})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.AddCandidate(cmd.Context(), store.CandidateRow{
				ListName: list, FQDN: fqdn, Killed: true, KillReason: reason,
			}); err != nil {
				return apiErr(err)
			}
			return emitJSON(cmd, flags, map[string]any{"killed": fqdn, "reason": reason})
		},
	}
	cmd.Flags().StringVar(&list, "list", "", "List name (required)")
	cmd.Flags().StringVar(&reason, "reason", "", "Kill reason (recorded for why-killed audit)")
	return cmd
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Drop / expiry watch — periodically re-check domains and persist status.",
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchRemoveCmd(flags))
	cmd.AddCommand(newWatchRunCmd(flags))
	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var cadence int
	cmd := &cobra.Command{
		Use:         "add <domain>",
		Short:       "Add a domain to the watch list.",
		Example:     `  domain-goat-pp-cli watch add expiring.io --cadence 12`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			fqdns, err := normalizeAll(args)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "fqdn": fqdns[0]})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			added := 0
			for _, f := range fqdns {
				if err := s.AddWatch(cmd.Context(), store.WatchRow{FQDN: f, CadenceHours: cadence}); err == nil {
					added++
				}
			}
			return emitJSON(cmd, flags, map[string]any{"added": added})
		},
	}
	cmd.Flags().IntVar(&cadence, "cadence", 24, "Re-check cadence in hours")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all watched domains.",
		Example: `  domain-goat-pp-cli watch list
  domain-goat-pp-cli watch list --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.ListWatches(cmd.Context())
			if err != nil {
				return apiErr(err)
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, rows)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DOMAIN\tCADENCE_H\tLAST_RUN\tLAST_STATUS")
			for _, w := range rows {
				fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", w.FQDN, w.CadenceHours, w.LastRunAt, w.LastStatus)
			}
			return tw.Flush()
		},
	}
	return cmd
}

func newWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "remove <domain>",
		Short:       "Remove a domain from the watch list.",
		Example:     `  domain-goat-pp-cli watch remove expiring.io`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "fqdn": args[0]})
			}
			fqdn := strings.ToLower(args[0])
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.RemoveWatch(cmd.Context(), fqdn); err != nil {
				return apiErr(err)
			}
			return emitJSON(cmd, flags, map[string]any{"removed": fqdn})
		},
	}
	return cmd
}

// PATCH(watch-cadence-enforcement): default to Store.ListDueWatches (cadence-gated WHERE clause); --force falls back to ListWatches. Without this, watch run iterated every domain on every cron tick regardless of cadence_hours, making the cadence configuration a no-op for cron users.
func newWatchRunCmd(flags *rootFlags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Re-check watched domains whose cadence has elapsed (use --force to re-check all).",
		Long: `Re-checks each watched domain via RDAP and updates last_status.

By default only domains whose last_run_at + cadence_hours has elapsed
are re-checked, so wiring 'watch run' into a cron tick honours the
per-domain cadence configured via 'watch add --cadence N'. Pass --force
to re-check every watched domain regardless of cadence (useful for
ad-hoc manual runs or initial backfill).`,
		Example: `  domain-goat-pp-cli watch run
  domain-goat-pp-cli watch run --force --json`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "force": force})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			var watches []store.WatchRow
			if force {
				watches, err = s.ListWatches(cmd.Context())
			} else {
				watches, err = s.ListDueWatches(cmd.Context())
			}
			if err != nil {
				return apiErr(err)
			}
			type Result struct {
				FQDN   string `json:"fqdn"`
				Status string `json:"status"`
				Error  string `json:"error,omitempty"`
			}
			out := make([]Result, 0, len(watches))
			for _, w := range watches {
				ctx, cancel := context.WithTimeout(cmd.Context(), 12*time.Second)
				res, err := rdap.Lookup(ctx, w.FQDN)
				cancel()
				r := Result{FQDN: w.FQDN}
				if err != nil && res == nil {
					r.Error = err.Error()
					r.Status = "error"
				} else if res != nil {
					r.Status = res.StatusText
					if res.Available {
						r.Status = "available"
					}
					_ = s.SaveRDAPRecord(cmd.Context(), w.FQDN, string(res.Raw), r.Status, res.EventsJSON())
				}
				_ = s.UpdateWatchResult(cmd.Context(), w.FQDN, r.Status)
				out = append(out, r)
			}
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Re-check every watched domain regardless of cadence_hours")
	return cmd
}
