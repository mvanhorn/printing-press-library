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

func newCampaignManagementGetSupportedClipchampAudioCmd(flags *rootFlags) *cobra.Command {
	var bodyAudioFilterAudioNames string
	var bodyAudioFilterCategories string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "get-supported-clipchamp-audio",
		Short:       "get_supported_clipchamp_audio",
		Example:     "  bing-ads-pp-cli campaign-management get-supported-clipchamp-audio",
		Annotations: map[string]string{"pp:endpoint": "campaign-management.get-supported-clipchamp-audio", "pp:method": "POST", "pp:path": "/CampaignManagement/v13/SupportedClipchampAudio/Query", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
			}
			path := "/CampaignManagement/v13/SupportedClipchampAudio/Query"
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
				body = map[string]any{"AudioFilter": bodyMap}
				if cmd.Flags().Changed("audio-filter-audio-names") {
					parsedAudioFilterAudioNames, parseErr := cliutil.ParseStringList(bodyAudioFilterAudioNames)
					if parseErr != nil {
						return fmt.Errorf("parsing --audio-filter-audio-names list: %w", parseErr)
					}
					bodyMap["AudioNames"] = parsedAudioFilterAudioNames
				}
				if cmd.Flags().Changed("audio-filter-categories") {
					parsedAudioFilterCategories, parseErr := cliutil.ParseStringList(bodyAudioFilterCategories)
					if parseErr != nil {
						return fmt.Errorf("parsing --audio-filter-categories list: %w", parseErr)
					}
					bodyMap["Categories"] = parsedAudioFilterCategories
				}
			}
			data, statusCode, err := c.PostQueryWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if isDryRunResponse(c.IsDryRun(), data) {
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "dry-run"}, map[string]bool{"Data": true})
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
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), formatData, flags, map[string]any{"source": "live"}, map[string]bool{"Data": true})
		},
	}
	cmd.Flags().StringVar(&bodyAudioFilterAudioNames, "audio-filter-audio-names", "", "Audio names")
	cmd.Flags().StringVar(&bodyAudioFilterCategories, "audio-filter-categories", "", "Categories")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
