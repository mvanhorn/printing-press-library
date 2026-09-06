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

func newAdInsightGetKeywordLocationsCmd(flags *rootFlags) *cobra.Command {
	var bodyDevice string
	var bodyKeywords string
	var bodyLanguage string
	var bodyLevel int
	var bodyMaxLocations int
	var bodyParentCountry string
	var bodyPublisherCountry string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "get-keyword-locations",
		Short:       "get_keyword_locations",
		Example:     "  bing-ads-pp-cli ad-insight get-keyword-locations",
		Annotations: map[string]string{"pp:endpoint": "ad-insight.get-keyword-locations", "pp:method": "POST", "pp:path": "/AdInsight/v13/KeywordLocations/query", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
			}
			path := "/AdInsight/v13/KeywordLocations/query"
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
				if cmd.Flags().Changed("device") {
					parsedDevice, parseErr := cliutil.ParseStringList(bodyDevice)
					if parseErr != nil {
						return fmt.Errorf("parsing --device list: %w", parseErr)
					}
					bodyMap["Device"] = parsedDevice
				}
				if cmd.Flags().Changed("keywords") {
					parsedKeywords, parseErr := cliutil.ParseStringList(bodyKeywords)
					if parseErr != nil {
						return fmt.Errorf("parsing --keywords list: %w", parseErr)
					}
					bodyMap["Keywords"] = parsedKeywords
				}
				if cmd.Flags().Changed("language") || bodyLanguage != "" {
					bodyMap["Language"] = bodyLanguage
				}
				if cmd.Flags().Changed("level") || bodyLevel != 0 {
					bodyMap["Level"] = bodyLevel
				}
				if cmd.Flags().Changed("max-locations") || bodyMaxLocations != 0 {
					bodyMap["MaxLocations"] = bodyMaxLocations
				}
				if cmd.Flags().Changed("parent-country") || bodyParentCountry != "" {
					bodyMap["ParentCountry"] = bodyParentCountry
				}
				if cmd.Flags().Changed("publisher-country") || bodyPublisherCountry != "" {
					bodyMap["PublisherCountry"] = bodyPublisherCountry
				}
			}
			data, statusCode, err := c.PostQueryWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if isDryRunResponse(c.IsDryRun(), data) {
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "dry-run"}, map[string]bool{"Keyword": true, "KeywordLocations": true})
				}
				return nil
			}
			_ = statusCode
			if !flags.dryRun {
				data = applyResponsePath(data, "KeywordLocationResult")
			}
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
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), formatData, flags, map[string]any{"source": "live"}, map[string]bool{"Keyword": true, "KeywordLocations": true})
		},
	}
	cmd.Flags().StringVar(&bodyDevice, "device", "", "Device")
	cmd.Flags().StringVar(&bodyKeywords, "keywords", "", "Keywords")
	cmd.Flags().StringVar(&bodyLanguage, "language", "", "Language")
	cmd.Flags().IntVar(&bodyLevel, "level", 0, "Level")
	cmd.Flags().IntVar(&bodyMaxLocations, "max-locations", 0, "Max locations")
	cmd.Flags().StringVar(&bodyParentCountry, "parent-country", "", "Parent country")
	cmd.Flags().StringVar(&bodyPublisherCountry, "publisher-country", "", "Publisher country")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
