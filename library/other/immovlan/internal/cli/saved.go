// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/store"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSavedCmd(flags))
	})
}

func newSavedCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "saved",
		Short: "Manage saved searches (local, no Immovlan account): add, list, remove",
		Example: strings.Trim(`
  immovlan-pp-cli saved add sch-fg --type maison,appartement --deal sale --postcode 1030,1210 --epc F,G
  immovlan-pp-cli saved list --agent
  immovlan-pp-cli saved remove sch-fg`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE:        func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	cmd.PersistentFlags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.PersistentFlags().MarkHidden("db")

	var cf critFlags
	add := &cobra.Command{
		Use:     "add <name>",
		Example: "  immovlan-pp-cli saved add sch-fg --type maison,appartement --deal sale --postcode 1030,1210 --epc F,G",
		Short:   "Save a search under a name (same flags as find)",
		Annotations: map[string]string{"pp:data-source": "local", "mcp:local-write": "true",
			"pp:happy-args": "name=dogfood-saved;--type=maison;--deal=sale;--postcode=1030"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "save a search")
			}
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return usageErr(fmt.Errorf("give the search a name: saved add <name> --type ... --deal ... --postcode ..."))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openVlanStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			crit, err := cf.build(ctx, db)
			if err != nil {
				return usageErr(err)
			}
			if err := crit.Validate(); err != nil {
				return usageErr(err)
			}
			if err := db.SaveSearch(ctx, args[0], crit); err != nil {
				return err
			}
			ss, err := db.GetSearch(ctx, args[0])
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printView(cmd.OutOrStdout(), ss, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved %q. Run: immovlan-pp-cli watch %s\n", ss.Name, ss.Name)
			return nil
		},
	}
	addCritFlags(add, &cf, true)

	list := &cobra.Command{
		Use:         "list",
		Short:       "List saved searches with their deal, property types, postcodes, PEB filter and last watch run",
		Example:     "  immovlan-pp-cli saved list --agent",
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list saved searches")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openVlanStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			ss, err := db.ListSearches(ctx)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				note := ""
				if len(ss) == 0 {
					note = "no saved searches; add one with: immovlan-pp-cli saved add <name> ..."
				}
				return printView(cmd.OutOrStdout(), struct {
					Results []store.SavedSearch `json:"results"`
					Note    string              `json:"note,omitempty"`
				}{ss, note}, flags)
			}
			if len(ss) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No saved searches. Add one with: immovlan-pp-cli saved add <name> --type maison --deal sale --postcode 1030")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "NAME\tDEAL\tTYPES\tPOSTCODES\tEPC\tLAST RUN")
			for _, s := range ss {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", s.Name, s.Criteria.Deal, strings.Join(s.Criteria.Types, ","), strings.Join(append(s.Criteria.PostalCodes, s.Criteria.Towns...), ","), strings.Join(s.Criteria.EPC, ","), shortDate(s.LastRunAt))
			}
			return tw.Flush()
		},
	}

	remove := &cobra.Command{
		Use:         "remove <name>",
		Short:       "Delete a saved search and its watch history",
		Example:     "  immovlan-pp-cli saved remove sch-fg",
		Annotations: map[string]string{"pp:data-source": "local", "mcp:local-write": "true", "pp:happy-args": "name=dogfood-saved"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "remove a saved search")
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("give the saved search name"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openVlanStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			ok, err := db.DeleteSearch(ctx, args[0])
			if err != nil {
				return err
			}
			if !ok {
				return notFoundErr(fmt.Errorf("no saved search named %q", args[0]))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printView(cmd.OutOrStdout(), map[string]any{"removed": args[0]}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %q.\n", args[0])
			return nil
		},
	}
	cmd.AddCommand(add, list, remove)
	return cmd
}

func shortDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	if s == "" {
		return "never"
	}
	return s
}
