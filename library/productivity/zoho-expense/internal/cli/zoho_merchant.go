package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/internal/zohotools"
)

func newMerchantCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merchant",
		Short: "Manage the local merchant memory (auto-tag training set)",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newMerchantListCmd(flags))
	cmd.AddCommand(newMerchantMapCmd(flags))
	cmd.AddCommand(newMerchantDeduceCmd(flags))
	return cmd
}

func newMerchantListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "Synthesize merchants from local expenses; show which have explicit mappings",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  zoho-expense-pp-cli merchant list
  zoho-expense-pp-cli merchant list --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openZohoStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			out, err := zohotools.SynthesizeMerchants(s.DB())
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no merchants in local store; run 'sync expenses' first")
				return nil
			}
			rows := make([][]string, 0, len(out))
			for _, m := range out {
				mapped := m.MappedCategory
				if mapped == "" {
					mapped = "(none)"
				}
				rows = append(rows, []string{
					m.MerchantName,
					fmt.Sprintf("%d", m.ExpenseCount),
					m.LastCategory,
					mapped,
					m.MappedProject,
				})
			}
			return flags.printTable(cmd, []string{"merchant", "expenses", "last_category", "mapped_category", "mapped_project"}, rows)
		},
	}
}

func newMerchantMapCmd(flags *rootFlags) *cobra.Command {
	var category, project string
	var tags []string
	cmd := &cobra.Command{
		Use:   "map <merchant_name>",
		Short: "Persist a category/project (and optional tags) for a merchant; used by --auto-tag on receipt upload",
		Example: strings.Trim(`
  zoho-expense-pp-cli merchant map "Uber" --category Travel --project Acme
  zoho-expense-pp-cli merchant map "Swiggy" --category "Meals & Entertainment"
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			merchant := strings.TrimSpace(args[0])
			if merchant == "" {
				return usageErr(fmt.Errorf("merchant_name required"))
			}
			s, err := openZohoStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			m := &zohotools.MerchantMapping{MerchantName: merchant}
			if category != "" {
				id, name := resolveExpenseCategory(s.DB(), category)
				m.CategoryID = id
				m.CategoryName = name
			}
			if project != "" {
				id, name := resolveProject(s.DB(), project)
				m.ProjectID = id
				m.ProjectName = name
			}
			for _, t := range tags {
				k, v, ok := strings.Cut(t, "=")
				if !ok || k == "" {
					return usageErr(fmt.Errorf("--tag must be tag_name=option, got %q", t))
				}
				m.Tags = append(m.Tags, zohotools.TagMapping{TagName: k, OptionName: v})
			}
			if err := zohotools.PutMerchant(s.DB(), m); err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), m, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "mapped %q → category=%s project=%s\n", merchant, m.CategoryID, m.ProjectID)
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Category id or name")
	cmd.Flags().StringVar(&project, "project", "", "Project id or name")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "Repeatable: tag_name=option")
	return cmd
}

func newMerchantDeduceCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "deduce",
		Short: "Populate merchant_memory from past expense history (most-common-category-per-merchant)",
		Example: strings.Trim(`
  zoho-expense-pp-cli merchant deduce
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openZohoStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if err := zohotools.DeduceFromHistory(s.DB()); err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"status": "ok"}, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "merchant memory populated from history")
			return nil
		},
	}
}
