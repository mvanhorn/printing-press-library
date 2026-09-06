// Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mvanhorn/printing-press-library/library/marketing/bing-ads/internal/cliutil"
	"github.com/spf13/cobra"
)

func newAdInsightGetTextAssetSuggestionsByFinalUrlsCmd(flags *rootFlags) *cobra.Command {
	var bodyFinalUrls string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "get-text-asset-suggestions-by-final-urls",
		Short:       "get_text_asset_suggestions_by_final_urls",
		Example:     "  bing-ads-pp-cli ad-insight get-text-asset-suggestions-by-final-urls",
		Annotations: map[string]string{"pp:endpoint": "ad-insight.get-text-asset-suggestions-by-final-urls", "pp:method": "POST", "pp:path": "/AdInsight/v13/TextAssetSuggestions/queryByFinalUrls", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
			}
			path := "/AdInsight/v13/TextAssetSuggestions/queryByFinalUrls"
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			var body any
			if stdinBody {
				stdinData, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				var jsonBody map[string]any
				if err := json.Unmarshal(stdinData, &jsonBody); err != nil {
					return fmt.Errorf("parsing stdin JSON: %w", err)
				}
				body = jsonBody
			} else {
				bodyMap := map[string]any{}
				body = bodyMap
				if cmd.Flags().Changed("final-urls") {
					parsedFinalUrls, parseErr := cliutil.ParseStringList(bodyFinalUrls)
					if parseErr != nil {
						return fmt.Errorf("parsing --final-urls list: %w", parseErr)
					}
					bodyMap["FinalUrls"] = parsedFinalUrls
				}
			}
			data, statusCode, err := c.PostQueryWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if isDryRunResponse(c.IsDryRun(), data) {
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "dry-run"}, map[string]bool{"PartialErrors": true, "TextAssetSuggestions": true})
				}
				return nil
			}
			_ = statusCode
			outputData := data
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(outputData, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}
			formatData := data
			if flags.csv || flags.plain {
				formatData = outputData
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), formatData, flags, map[string]any{"source": "live"}, map[string]bool{"PartialErrors": true, "TextAssetSuggestions": true})
		},
	}
	cmd.Flags().StringVar(&bodyFinalUrls, "final-urls", "", "Final urls")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
