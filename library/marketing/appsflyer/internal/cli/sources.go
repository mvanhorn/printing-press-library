package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/appsflyer/internal/channels"
	"github.com/mvanhorn/printing-press-library/library/marketing/appsflyer/internal/sources"

	"github.com/spf13/cobra"
)

func newSourcesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Browse the AppsFlyer media-source registry (canonical _int IDs ↔ display names)",
		Long: `Browse and resolve the AppsFlyer media-source registry.

The V2 API expects canonical media_source IDs ending in '_int' (e.g.
facebook_int, googleadwords_int). This command lets you look up the
canonical ID for a brand name, list the full catalog, or list the
channel groups (social, programmatic, OEM, rewarded) the CLI applies
when you pass --channel-group to other commands.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newSourcesListCmd(flags))
	cmd.AddCommand(newSourcesResolveCmd(flags))
	cmd.AddCommand(newSourcesGroupsCmd(flags))
	return cmd
}

type sourceRow struct {
	Canonical string   `json:"canonical"`
	Display   string   `json:"display"`
	Kind      string   `json:"kind"`
	Aliases   []string `json:"aliases,omitempty"`
}

func newSourcesListCmd(flags *rootFlags) *cobra.Command {
	var query string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List media sources, optionally filtered by --query",
		Example: strings.Trim(`
  appsflyer-pp-cli sources list
  appsflyer-pp-cli sources list --query tiktok
  appsflyer-pp-cli sources list --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			matches := sources.Search(query)
			rows := make([]sourceRow, 0, len(matches))
			for _, s := range matches {
				rows = append(rows, sourceRow{Canonical: s.Canonical, Display: s.Display, Kind: s.Kind, Aliases: s.Aliases})
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				tableRows = append(tableRows, []string{r.Display, r.Canonical, r.Kind})
			}
			return flags.printTable(cmd, []string{"DISPLAY", "CANONICAL", "KIND"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Filter by substring across display name, canonical, or aliases")
	return cmd
}

func newSourcesResolveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve [input]",
		Short: "Resolve a brand name or alias to its canonical media_source ID",
		Long: `Resolve a brand name, canonical ID, or alias to the canonical
'_int' ID the AppsFlyer V2 API expects. Useful when piping into other
commands that take --source.`,
		Example: strings.Trim(`
  appsflyer-pp-cli sources resolve tiktok
  appsflyer-pp-cli sources resolve "Meta ads"
  appsflyer-pp-cli sources resolve facebook --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			input := strings.Join(args, " ")
			canonical, ok := sources.Resolve(input)
			if !ok {
				return fmt.Errorf("no media source matches %q (try 'appsflyer-pp-cli sources list --query %s' to browse)", input, input)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]string{"input": input, "canonical": canonical}, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), canonical)
			return nil
		},
	}
	return cmd
}

func newSourcesGroupsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "groups",
		Short: "List channel groups and their member media sources",
		Long: `List the channel groups (social, programmatic, OEM, rewarded)
and the canonical media-source IDs in each. Default mapping is shipped
with the CLI and overridable at ~/.config/appsflyer-pp-cli/channels.yaml.`,
		Example: strings.Trim(`
  appsflyer-pp-cli sources groups
  appsflyer-pp-cli sources groups --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			grp, err := channels.Load()
			if err != nil {
				return err
			}
			if flags.asJSON {
				type groupRow struct {
					Group   string   `json:"group"`
					Sources []string `json:"sources"`
				}
				rows := make([]groupRow, 0, len(grp.Names()))
				for _, name := range grp.Names() {
					rows = append(rows, groupRow{Group: name, Sources: grp[name]})
				}
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tableRows := make([][]string, 0, len(grp))
			for _, name := range grp.Names() {
				tableRows = append(tableRows, []string{name, fmt.Sprintf("%d", len(grp[name])), strings.Join(grp[name], ", ")})
			}
			return flags.printTable(cmd, []string{"GROUP", "COUNT", "MEMBERS"}, tableRows)
		},
	}
	return cmd
}
