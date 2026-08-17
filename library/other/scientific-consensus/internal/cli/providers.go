// Hand-authored command: secondary data-source (provider) management for the
// aggregator pattern. Not generated. Named `providers` to avoid colliding with
// the OpenAlex `sources` (journals) entity command.
package cli

import (
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/source"
	"github.com/spf13/cobra"
)

type providerInfo struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	AuthRequired bool   `json:"auth_required"`
	OptionalKey  string `json:"optional_key_env,omitempty"`
}

func newProvidersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "providers",
		Short:       "List and query the secondary scholarly data sources (PubMed, Crossref, Europe PMC, Semantic Scholar)",
		Long:        "OpenAlex is the primary source (used by search and the engine commands). These\nsecondary providers add coverage and enrichment. List them or query one directly.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newProvidersListCmd(flags))
	cmd.AddCommand(newProvidersSyncCmd(flags))
	return cmd
}

func newProvidersListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List registered secondary data sources",
		Example:     "  scientific-consensus-pp-cli providers list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			infos := make([]providerInfo, 0)
			for _, s := range source.All() {
				infos = append(infos, providerInfo{
					Name: s.Name(), Description: s.Description(),
					AuthRequired: s.AuthRequired(), OptionalKey: s.OptionalKeyEnv(),
				})
			}
			return emit(cmd, flags, infos, func(w io.Writer) {
				fmt.Fprintln(w, "Secondary data sources (OpenAlex is the primary source):")
				for _, p := range infos {
					key := p.OptionalKey
					if key == "" {
						key = "—"
					}
					fmt.Fprintf(w, "  %-16s keyless=%v  optional-key=%s\n      %s\n", p.Name, !p.AuthRequired, key, p.Description)
				}
			})
		},
	}
	return cmd
}

func newProvidersSyncCmd(flags *rootFlags) *cobra.Command {
	var sourceName, query string
	var limit int

	cmd := &cobra.Command{
		Use:         "sync",
		Short:       "Query a secondary source (or all) for a topic and return normalized works",
		Example:     "  scientific-consensus-pp-cli providers sync --source pubmed --query \"vitamin d\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query secondary source(s)")
				return nil
			}
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--query is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var sources []source.Source
			if sourceName != "" {
				s, ok := source.Lookup(sourceName)
				if !ok {
					return usageErr(fmt.Errorf("unknown source %q; run 'providers list'", sourceName))
				}
				sources = []source.Source{s}
			} else {
				sources = source.All()
			}

			type providerResult struct {
				Source string        `json:"source"`
				Works  []source.Work `json:"works"`
				Error  string        `json:"error,omitempty"`
			}
			results := make([]providerResult, 0, len(sources))
			for _, s := range sources {
				works, err := s.Sync(ctx, source.SyncOptions{Query: query, Limit: limit})
				pr := providerResult{Source: s.Name(), Works: works}
				if err != nil {
					pr.Error = err.Error()
				}
				results = append(results, pr)
			}
			return emit(cmd, flags, results, func(w io.Writer) {
				for _, r := range results {
					fmt.Fprintf(w, "\n== %s ==\n", r.Source)
					if r.Error != "" {
						fmt.Fprintf(w, "  error: %s\n", r.Error)
						continue
					}
					for _, wk := range r.Works {
						fmt.Fprintf(w, "  [%d] %s\n", wk.Year, truncate(wk.Title, 80))
					}
				}
			})
		},
	}
	cmd.Flags().StringVar(&sourceName, "source", "", "single source to query (default: all). One of: pubmed, crossref, europepmc, semanticscholar")
	cmd.Flags().StringVar(&query, "query", "", "search query (required)")
	cmd.Flags().IntVar(&limit, "limit", 15, "results per source")
	return cmd
}
