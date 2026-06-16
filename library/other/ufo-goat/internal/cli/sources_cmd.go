// Sources command — list the manifest sources the CLI can sync from.
//
// The source is configurable so the CLI is not locked to one community mirror.
// It also lays groundwork for a future sanctioned, direct-from-war.gov feed:
// that becomes a new entry in the registry (manifest.Sources) selectable via
// `--source wargov`, with no other command changes.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ufo-goat/internal/manifest"

	"github.com/spf13/cobra"
)

func newSourcesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "List the manifest sources the CLI can sync from",
		Long: `List the known manifest sources. Each source is an origin for the UAP file
manifest. Select one for any sync with --source <name>, override with a custom
URL via --manifest-url, or set the UFO_SOURCE / UFO_MANIFEST_URL environment
variables.

Resolution precedence (highest first): --manifest-url, --source,
UFO_MANIFEST_URL, UFO_SOURCE, built-in default (community).`,
		Example: `  # List all sources
  ufo-goat-pp-cli sources

  # Sync from a specific source
  ufo-goat-pp-cli sync --source legacy`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			sources := manifest.SortedSources()

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(map[string]any{
					"default": manifest.DefaultSourceName,
					"sources": sources,
				})
				filtered := json.RawMessage(data)
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				}
				return printOutput(cmd.OutOrStdout(), filtered, true)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, strings.Join([]string{
				bold("SOURCE"), bold("STATUS"), bold("DESCRIPTION"),
			}, "\t"))
			for _, s := range sources {
				status := "available"
				if !s.Available {
					status = "planned"
				}
				if s.Name == manifest.DefaultSourceName {
					status += " (default)"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", s.Name, status, s.Description)
			}
			tw.Flush()
			fmt.Fprintf(cmd.OutOrStdout(), "\nSelect with: ufo-goat-pp-cli sync --source <name>   (or --manifest-url <url>)\n")
			return nil
		},
	}
	return cmd
}
