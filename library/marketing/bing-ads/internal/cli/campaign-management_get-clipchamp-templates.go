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

func newCampaignManagementGetClipchampTemplatesCmd(flags *rootFlags) *cobra.Command {
	var bodyLocale string
	var bodyMaxAdsCount int
	var bodyMock bool
	var bodyTemplateFilterAspectRatios string
	var bodyTemplateFilterDurations string
	var bodyTemplateFilterMaxMediaAssetCount int
	var bodyTemplateFilterMaxTextAssetCount int
	var bodyTemplateFilterTemplateIds string
	var bodyTemplateFilterThemes string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "get-clipchamp-templates",
		Short:       "get_clipchamp_templates",
		Example:     "  bing-ads-pp-cli campaign-management get-clipchamp-templates",
		Annotations: map[string]string{"pp:endpoint": "campaign-management.get-clipchamp-templates", "pp:method": "POST", "pp:path": "/CampaignManagement/v13/ClipchampTemplates/Query", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
			}
			path := "/CampaignManagement/v13/ClipchampTemplates/Query"
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
				if cmd.Flags().Changed("locale") || bodyLocale != "" {
					bodyMap["Locale"] = bodyLocale
				}
				if cmd.Flags().Changed("max-ads-count") || bodyMaxAdsCount != 0 {
					bodyMap["MaxAdsCount"] = bodyMaxAdsCount
				}
				if cmd.Flags().Changed("mock") {
					bodyMap["Mock"] = bodyMock
				}
				{
					nestedTemplateFilter := map[string]any{}
					if cmd.Flags().Changed("template-filter-aspect-ratios") {
						parsedTemplateFilterAspectRatios, parseErr := cliutil.ParseStringList(bodyTemplateFilterAspectRatios)
						if parseErr != nil {
							return fmt.Errorf("parsing --template-filter-aspect-ratios list: %w", parseErr)
						}
						nestedTemplateFilter["AspectRatios"] = parsedTemplateFilterAspectRatios
					}
					if cmd.Flags().Changed("template-filter-durations") || bodyTemplateFilterDurations != "" {
						var parsedTemplateFilterDurations any
						if err := json.Unmarshal([]byte(bodyTemplateFilterDurations), &parsedTemplateFilterDurations); err != nil {
							return fmt.Errorf("parsing --template-filter-durations JSON: %w", err)
						}
						asArray, ok := parsedTemplateFilterDurations.([]any)
						if !ok {
							return fmt.Errorf("--template-filter-durations must be a JSON array, got JSON %T", parsedTemplateFilterDurations)
						}
						nestedTemplateFilter["Durations"] = asArray
					}
					if cmd.Flags().Changed("template-filter-max-media-asset-count") || bodyTemplateFilterMaxMediaAssetCount != 0 {
						nestedTemplateFilter["MaxMediaAssetCount"] = bodyTemplateFilterMaxMediaAssetCount
					}
					if cmd.Flags().Changed("template-filter-max-text-asset-count") || bodyTemplateFilterMaxTextAssetCount != 0 {
						nestedTemplateFilter["MaxTextAssetCount"] = bodyTemplateFilterMaxTextAssetCount
					}
					if cmd.Flags().Changed("template-filter-template-ids") {
						parsedTemplateFilterTemplateIds, parseErr := cliutil.ParseStringList(bodyTemplateFilterTemplateIds)
						if parseErr != nil {
							return fmt.Errorf("parsing --template-filter-template-ids list: %w", parseErr)
						}
						nestedTemplateFilter["TemplateIds"] = parsedTemplateFilterTemplateIds
					}
					if cmd.Flags().Changed("template-filter-themes") {
						parsedTemplateFilterThemes, parseErr := cliutil.ParseStringList(bodyTemplateFilterThemes)
						if parseErr != nil {
							return fmt.Errorf("parsing --template-filter-themes list: %w", parseErr)
						}
						nestedTemplateFilter["Themes"] = parsedTemplateFilterThemes
					}
					if len(nestedTemplateFilter) > 0 {
						bodyMap["TemplateFilter"] = nestedTemplateFilter
					}
				}
			}
			data, statusCode, err := c.PostQueryWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if isDryRunResponse(c.IsDryRun(), data) {
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "dry-run"}, map[string]bool{"AspectRatio": true, "Duration": true, "NumberOfImages": true, "NumberOfLogos": true, "NumberOfText": true, "TemplateDescription": true, "TemplateId": true, "TemplateName": true, "TemplatePreviewUrl": true, "TemplateThumbnailUrl": true, "Themes": true})
				}
				return nil
			}
			_ = statusCode
			if !flags.dryRun {
				data = applyResponsePath(data, "Templates")
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
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), formatData, flags, map[string]any{"source": "live"}, map[string]bool{"AspectRatio": true, "Duration": true, "NumberOfImages": true, "NumberOfLogos": true, "NumberOfText": true, "TemplateDescription": true, "TemplateId": true, "TemplateName": true, "TemplatePreviewUrl": true, "TemplateThumbnailUrl": true, "Themes": true})
		},
	}
	cmd.Flags().StringVar(&bodyLocale, "locale", "", "Locale")
	cmd.Flags().IntVar(&bodyMaxAdsCount, "max-ads-count", 0, "Max ads count")
	cmd.Flags().BoolVar(&bodyMock, "mock", false, "Mock")
	cmd.Flags().StringVar(&bodyTemplateFilterAspectRatios, "template-filter-aspect-ratios", "", "Aspect ratios")
	cmd.Flags().StringVar(&bodyTemplateFilterDurations, "template-filter-durations", "", "Durations")
	cmd.Flags().IntVar(&bodyTemplateFilterMaxMediaAssetCount, "template-filter-max-media-asset-count", 0, "Max media asset count")
	cmd.Flags().IntVar(&bodyTemplateFilterMaxTextAssetCount, "template-filter-max-text-asset-count", 0, "Max text asset count")
	cmd.Flags().StringVar(&bodyTemplateFilterTemplateIds, "template-filter-template-ids", "", "Template ids")
	cmd.Flags().StringVar(&bodyTemplateFilterThemes, "template-filter-themes", "", "Themes")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
