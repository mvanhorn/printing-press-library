// Copyright 2026 Ricardo Batista and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(bing-endpoint-is-hand-wired-until-the-spec-lists-it): hand-written
// from the google command. See
// .printing-press-patches/bing-endpoint-is-hand-wired-until-the-spec-lists-it.json.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newBingPromotedCmd(flags *rootFlags) *cobra.Command {
	var bodyCountry string
	var bodyIncludeAioverview bool
	var bodyIncludeHtml bool
	var bodyQuery string

	cmd := &cobra.Command{
		Use:         "bing",
		Short:       "Extract structured data from Bing Search: organic results and optional AI Overview",
		Long:        "Extract structured data from Bing Search: organic results and optional AI Overview.\n\nThe query can have a maximum of 3,500 characters after URL encoding, so fewer characters for non-Latin scripts. The country picks Bing's market. Costs 3 credits, plus 2 for a sync request and 2 for the AI Overview.",
		Example:     "  cloro-pp-cli bing --query \"best running shoes for flat feet\" --country US",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"pp:endpoint": "bing.bing", "pp:method": "POST", "pp:path": "/v1/monitor/bing", "mcp:read-only": "true", "pp:requires-input": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Bare invocation of a command with a required flag/body prints help
			// instead of pflag's terse "required flag not set" error. Optional-
			// only reads fall through so a bare call still executes; positional
			// commands keep their existing usageErr (exit 2 + JSON envelope).
			// Machine callers (--json/--agent, which sets asJSON) get a usage
			// error + exit 2 instead of silent exit-0 help.
			if !hasChangedLocalFlags(cmd) && len(args) == 0 && !flags.dryRun {
				if flags.asJSON {
					if printErr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "requires input",
						"usage": cmd.CommandPath() + " --help",
					}, flags); printErr != nil {
						return printErr
					}
					return usageErr(fmt.Errorf("%q requires input; run %q for usage", cmd.CommandPath(), cmd.CommandPath()+" --help"))
				}
				return cmd.Help()
			}
			if !cmd.Flags().Changed("country") && bodyCountry == "" && !requiredInputWaived() {
				return dryRunValidationErr(cmd, flags, fmt.Errorf("required flag \"%s\" not set", "country"))
			}
			if !cmd.Flags().Changed("query") && bodyQuery == "" && !requiredInputWaived() {
				return dryRunValidationErr(cmd, flags, fmt.Errorf("required flag \"%s\" not set", "query"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/v1/monitor/bing"
			headerOverrides := map[string]string{
				"X-Printing-Press-Replay-Safe": "false",
			}

			params := map[string]string{}
			// HasStore + non-GET falls through to a live API call here
			// rather than through resolveRead (GET-only internally); a
			// body-aware cached read helper is filed as #425 for when a
			// second store-backed POST-search consumer ships.
			bodyMap := map[string]any{}
			var body any = bodyMap
			if cmd.Flags().Changed("country") || bodyCountry != "" {
				bodyMap["country"] = bodyCountry
			}
			{
				nestedInclude := map[string]any{}
				if cmd.Flags().Changed("include-aioverview") {
					nestedInclude["aioverview"] = bodyIncludeAioverview
				}
				if cmd.Flags().Changed("include-html") {
					nestedInclude["html"] = bodyIncludeHtml
				}
				if len(nestedInclude) > 0 {
					bodyMap["include"] = nestedInclude
				}
			}
			if cmd.Flags().Changed("query") || bodyQuery != "" {
				bodyMap["query"] = bodyQuery
			}
			data, _, err := c.PostQueryWithParamsAndHeaders(cmd.Context(), path, params, body, headerOverrides)

			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			prov := attachFreshness(DataProvenance{Source: "live"}, flags)
			outputData := data
			// Print provenance to stderr for human-facing output only.
			// Machine-format flags (--json, --csv, --compact, --quiet, --plain,
			// --select) and piped stdout suppress this line; the JSON envelope
			// already carries meta.source for those consumers.
			// SYNC: keep this gate aligned with command_endpoint.go.tmpl.
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var countItems []json.RawMessage
				if json.Unmarshal(outputData, &countItems) != nil {
					// Single object, not an array
					countItems = []json.RawMessage{outputData}
				}
				printProvenance(cmd, len(countItems), prov)
			}
			// For JSON output, wrap with provenance envelope. --select wins over
			// --compact when both are set; --compact only runs when no explicit
			// fields were requested. Explicit format flags (--csv, --quiet, --plain)
			// opt out of the auto-JSON path so piped consumers that asked for a
			// non-JSON format reach the standard pipeline below.
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				var selectErr error
				filtered := data
				if flags.selectFields != "" {
					filtered, selectErr = filterFieldsChecked(filtered, flags.selectFields)
					selectErr = selectErrorForDryRun(selectErr, flags, data)
				} else if flags.compact {
					filtered = compactFields(filtered, nil)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				wrapped, wrapErr = wrapPlatformStructuredOutput(wrapped, flags, "results", true)
				if wrapErr != nil {
					return wrapErr
				}
				if err := printOutput(cmd.OutOrStdout(), wrapped, true); err != nil {
					return err
				}
				return selectErr
			}
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
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), formatData, flags, map[string]any{"source": "live"}, nil)
		},
	}
	cmd.Flags().StringVar(&bodyCountry, "country", "", "ISO 3166-1 alpha-2 country code. It picks Bing's market. Run 'countries --model bing' for the list.")
	cmd.Flags().BoolVar(&bodyIncludeAioverview, "include-aioverview", false, "Include the Bing AI Overview in result.aioverview. Paid add-on: +2 credits.")
	cmd.Flags().BoolVar(&bodyIncludeHtml, "include-html", false, "Include a URL to the full HTML of the results page (default: false)")
	cmd.Flags().StringVar(&bodyQuery, "query", "", "The search query to execute on Bing. Maximum 3,500 characters after URL encoding.")

	// Wire sibling endpoints and sub-resources as subcommands

	return cmd
}
