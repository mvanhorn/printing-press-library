// pp:data-source local

package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/store"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSavedCmd(flags))
	})
}

var savedNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

func newSavedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "saved",
		Short: "Named local searches used by watch and triage (no Immoweb account needed)",
		Example: strings.Trim(`
  immoweb-pp-cli saved add ixelles-2bed --type apartment --deal rent --commune ixelles --max-price 1500 --min-bedrooms 2
  immoweb-pp-cli saved list
  immoweb-pp-cli saved remove ixelles-2bed`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newSavedAddCmd(flags), newSavedListCmd(flags), newSavedRemoveCmd(flags))
	return cmd
}

func newSavedAddCmd(flags *rootFlags) *cobra.Command {
	var cf critFlags
	var dbPath string
	cmd := &cobra.Command{
		Use:   "add [name]",
		Short: "Save a search under a name (same filters as find, or --url)",
		Example: strings.Trim(`
  immoweb-pp-cli saved add ixelles-2bed --type apartment --deal rent --commune ixelles --max-price 1500 --min-bedrooms 2
  immoweb-pp-cli saved add liege-invest --type apartment --deal sale --commune liege --max-price 180000
  immoweb-pp-cli saved add from-site --url "https://www.immoweb.be/fr/recherche/maison/a-vendre/namur/5000?maxPrice=350000"`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "local",
			"pp:happy-args":  "name=ixelles-2bed;--type=apartment;--deal=rent;--commune=ixelles;--max-price=1500;--min-bedrooms=2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "save a named search")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search name is required (e.g. ixelles-2bed)"))
			}
			name := strings.ToLower(strings.TrimSpace(args[0]))
			if !savedNameRe.MatchString(name) {
				return usageErr(fmt.Errorf("search name %q must be lowercase letters, digits, '.', '_' or '-'", args[0]))
			}
			crit, err := cf.build()
			if err != nil {
				return usageErr(err)
			}
			if err := crit.Validate(); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			db, err := openImmoStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.SaveSearch(cmd.Context(), name, crit); err != nil {
				return err
			}
			ss, err := db.GetSearch(cmd.Context(), name)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), ss, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved %q. Next: immoweb-pp-cli watch %s\n", name, name)
			return nil
		},
	}
	addCritFlags(cmd, &cf, true)
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db") // not an MCP tool argument: agents must not point the store at other files
	return cmd
}

func newSavedListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved searches with their criteria and last run",
		Example:     "  immoweb-pp-cli saved list --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list saved searches")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			list := make([]store.SavedSearch, 0)
			if path, ok := localStoreExists(dbPath); ok {
				db, err := openImmoStore(cmd.Context(), path)
				if err != nil {
					return err
				}
				defer db.Close()
				if list, err = db.ListSearches(cmd.Context()); err != nil {
					return err
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), list, flags)
			}
			if len(list) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No saved searches. Create one with: immoweb-pp-cli saved add <name> --type ... --deal ...")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "NAME\tDEAL\tTYPE\tWHERE\tPRICE\tLAST RUN")
			for _, s := range list {
				where := strings.Join(append(append([]string{}, s.Criteria.Communes...), s.Criteria.PostalCodes...), ",")
				if where == "" {
					where = strings.Join(append(s.Criteria.Provinces, s.Criteria.Districts...), ",")
				}
				price := ""
				if s.Criteria.MinPrice > 0 || s.Criteria.MaxPrice > 0 {
					price = fmt.Sprintf("%d-%d", s.Criteria.MinPrice, s.Criteria.MaxPrice)
				}
				last := s.LastRunAt
				if last == "" {
					last = "never"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", s.Name, strings.ToLower(s.Criteria.Deal), strings.ToLower(strings.Join(s.Criteria.Types, ",")), where, price, last)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db") // not an MCP tool argument: agents must not point the store at other files
	return cmd
}

func newSavedRemoveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:     "remove [name]",
		Aliases: []string{"rm", "delete"},
		Short:   "Delete a saved search and its watch history",
		Example: "  immoweb-pp-cli saved remove ixelles-2bed",
		Annotations: map[string]string{
			"pp:data-source":      "local",
			"pp:happy-args":       "name=ixelles-2bed",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "remove a saved search")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search name is required"))
			}
			db, err := openImmoStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			ok, err := db.DeleteSearch(cmd.Context(), strings.ToLower(args[0]))
			if err != nil {
				return err
			}
			if !ok {
				return notFoundErr(fmt.Errorf("no saved search named %q (see: immoweb-pp-cli saved list)", args[0]))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"removed": args[0]}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %q\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db") // not an MCP tool argument: agents must not point the store at other files
	return cmd
}

// loadSaved fetches a saved search, mapping a miss to a not-found error.
func loadSaved(cmd *cobra.Command, db *store.Store, name string) (store.SavedSearch, error) {
	ss, err := db.GetSearch(cmd.Context(), strings.ToLower(name))
	if errors.Is(err, sql.ErrNoRows) {
		return ss, notFoundErr(fmt.Errorf("no saved search named %q; create it with: immoweb-pp-cli saved add %s --type ... --deal ...", name, name))
	}
	return ss, err
}
