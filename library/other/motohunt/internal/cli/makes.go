// Copyright 2026 richardadonnell. Licensed under Apache-2.0. See LICENSE.
// Hand-written: `makes` and `models` enumeration commands.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/motohunt/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/motohunt/internal/motohunt"

	"github.com/spf13/cobra"
)

func newMakesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "makes",
		Short: "Enumerate valid makes for the active --site (drives precise searches)",
		Long: `List the canonical make slugs for the active marketplace, parsed from the
search page's make dropdown. Use the slugs with 'search --make' and 'models --make'.`,
		Example:     "  motohunt-pp-cli makes --agent\n  motohunt-pp-cli --site atv makes --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			site, err := siteConfigFor(flags)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would GET %s%s (parse make dropdown)\n", site.Base, site.SearchPath)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), make([]motohunt.Make, 0), flags)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			doc, ferr := scrapeClient(flags).Fetch(ctx, site.Base+site.SearchPath)
			if ferr != nil {
				return apiErr(ferr)
			}
			makes := motohunt.ParseMakes(doc)
			if len(makes) == 0 {
				makes = make([]motohunt.Make, 0)
			}
			return printDomainJSON(cmd.OutOrStdout(), makes, flags)
		},
	}
	return cmd
}

func newModelsCmd(flags *rootFlags) *cobra.Command {
	var mk string
	cmd := &cobra.Command{
		Use:   "models --make <X>",
		Short: "Enumerate models for a make on the active --site",
		Long: `List the models for a make, parsed from the /model-selector cascade fragment
(button[data-name]). Each model carries its slug (vehicle-model id), name, and
the style/category section it sits under.`,
		Example:     "  motohunt-pp-cli models --make Harley-Davidson --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			site, err := siteConfigFor(flags)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				m := mk
				if m == "" {
					m = "<make>"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would GET %s\n", site.ModelSelectorURL(m))
				return nil
			}
			if mk == "" {
				return usageErr(fmt.Errorf("--make is required: motohunt-pp-cli models --make Harley-Davidson"))
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), make([]motohunt.Model, 0), flags)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			doc, ferr := scrapeClient(flags).Fetch(ctx, site.ModelSelectorURL(mk))
			if ferr != nil {
				return apiErr(ferr)
			}
			models := motohunt.ParseModels(doc)
			if len(models) == 0 {
				models = make([]motohunt.Model, 0)
			}
			return printDomainJSON(cmd.OutOrStdout(), models, flags)
		},
	}
	cmd.Flags().StringVar(&mk, "make", "", "Make slug to enumerate models for (see 'makes')")
	return cmd
}
