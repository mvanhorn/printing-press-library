// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/travelclick/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/travelclick/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/travelclick/internal/store"
	"github.com/spf13/cobra"
)

type CodeCheckResult struct {
	HotelID      string `json:"hotel_id"`
	Alias        string `json:"alias,omitempty"`
	Code         string `json:"code"`
	CodeType     string `json:"code_type"`
	Valid        bool   `json:"valid"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type CodeCheckOutput struct {
	Results       []CodeCheckResult `json:"results"`
	FetchFailures []map[string]any  `json:"fetch_failures,omitempty"`
}

func newNovelCodesCheckAllCmd(flags *rootFlags) *cobra.Command {
	var flagType string
	var flagHotels string

	cmd := &cobra.Command{
		Use:     "check-all <code>",
		Short:   "Test one corporate or group code against every saved hotel at once.",
		Example: "  travelclick-pp-cli codes check-all ACME2026 --type corporate --hotels 'made-nyc,102306'",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "code=ACME2026;--type=corporate;--hotels=102306",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "codes check-all")
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("code is required"))
			}
			code := args[0]

			if flagType != "corporate" && flagType != "group" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--type must be either 'corporate' or 'group'"))
			}
			if flagHotels == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--hotels is required"))
			}

			// Validate --data-source is live / auto
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			tokens := strings.Split(flagHotels, ",")
			var hotelTokens []string
			for _, t := range tokens {
				t = strings.TrimSpace(t)
				if t != "" {
					hotelTokens = append(hotelTokens, t)
				}
			}

			// Resolve every --hotels token against the local store BEFORE
			// fanning out -- see resolveHotelTokensSequential's doc comment.
			// Resolving concurrently from inside the fan-out closure crashed
			// this command with a SIGBUS fault.
			resolutions := resolveHotelTokensSequential(ctx, hotelTokens)

			results, errs := cliutil.FanoutRun(ctx, hotelTokens, func(token string) string {
				return token
			}, func(ctx context.Context, token string) (CodeCheckResult, error) {
				resolvedID, alias := resolutions[token].HotelID, resolutions[token].Alias

				var path string
				if flagType == "corporate" {
					path = "/ibe-codes/v1/hotel/{hotel_id}/specialcodes/corporate/{code}"
				} else {
					path = "/ibe-codes/v1/hotel/{hotel_id}/specialcodes/group/attendee/{code}"
				}
				path = replacePathParam(path, "hotel_id", resolvedID)
				path = replacePathParam(path, "code", code)

				_, err := c.GetWithHeaders(ctx, path, nil, nil)
				if err == nil {
					// HTTP 200 -> Valid! Persistence happens sequentially
					// after fan-out completes, not here -- see below.
					return CodeCheckResult{
						HotelID:  resolvedID,
						Alias:    alias,
						Code:     code,
						CodeType: flagType,
						Valid:    true,
					}, nil
				}

				var apiErr *client.APIError
				if errors.As(err, &apiErr) {
					if apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
						var errEnvelope struct {
							Errors []struct {
								ErrorCode    string `json:"errorCode"`
								ErrorMessage string `json:"errorMessage"`
							} `json:"errors"`
						}
						errorCode := "INVALID_CODE"
						errorMessage := "Invalid code"
						if json.Unmarshal([]byte(apiErr.Body), &errEnvelope) == nil && len(errEnvelope.Errors) > 0 {
							errorCode = errEnvelope.Errors[0].ErrorCode
							errorMessage = errEnvelope.Errors[0].ErrorMessage
						} else {
							errorMessage = fmt.Sprintf("HTTP %d: %s", apiErr.StatusCode, apiErr.Body)
						}
						return CodeCheckResult{
							HotelID:      resolvedID,
							Alias:        alias,
							Code:         code,
							CodeType:     flagType,
							Valid:        false,
							ErrorCode:    errorCode,
							ErrorMessage: errorMessage,
						}, nil
					}
				}

				return CodeCheckResult{}, err
			})

			var finalResults []CodeCheckResult
			for _, r := range results {
				finalResults = append(finalResults, r.Value)
			}

			// Persist every result sequentially, after fan-out has fully
			// completed. Writing from inside the parallel closure above
			// (one store open/close per goroutine) was the same SIGBUS
			// hazard as the alias resolution -- see
			// resolveHotelTokensSequential's doc comment.
			for _, ccr := range finalResults {
				_ = saveCodeCheck(ctx, ccr.HotelID, ccr.CodeType, ccr.Code, ccr.Valid, ccr.ErrorCode, ccr.ErrorMessage)
			}

			var fetchFailures []map[string]any
			for _, fe := range errs {
				fetchFailures = append(fetchFailures, map[string]any{
					"hotel": fe.Source,
					"error": fe.Err.Error(),
				})
			}

			if len(errs) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d hotel code check transport failures encountered\n", len(errs))
				cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)
			}

			output := CodeCheckOutput{
				Results:       finalResults,
				FetchFailures: fetchFailures,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), output, flags)
			}

			if len(finalResults) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No hotel check results available.")
				return nil
			}

			var rows [][]string
			for _, item := range finalResults {
				valStr := "INVALID"
				if item.Valid {
					valStr = "VALID"
				}
				rows = append(rows, []string{
					item.HotelID,
					item.Alias,
					valStr,
					item.ErrorCode,
					item.ErrorMessage,
				})
			}

			return flags.printTable(cmd, []string{"HOTEL_ID", "ALIAS", "STATUS", "ERROR_CODE", "ERROR_MESSAGE"}, rows)
		},
	}

	cmd.Flags().StringVar(&flagType, "type", "", "Type of code to check ('corporate' or 'group')")
	cmd.Flags().StringVar(&flagHotels, "hotels", "", "Comma-separated list of hotel IDs or aliases to test")

	return cmd
}

func saveCodeCheck(ctx context.Context, hotelID string, codeType string, code string, valid bool, errCode string, errMsg string) error {
	db, err := openStore(ctx)
	if err != nil || db == nil {
		return nil
	}
	defer db.Close()
	valInt := 0
	if valid {
		valInt = 1
	}
	cc := &store.CodeCheck{
		HotelID:      hotelID,
		CodeType:     codeType,
		Code:         code,
		Valid:        valInt,
		ErrorCode:    errCode,
		ErrorMessage: errMsg,
	}
	if err := db.InsertCodeCheck(ctx, cc); err != nil {
		fmt.Fprintf(os.Stderr, "warning: local code check persistence failed: %v\n", err)
	}
	return nil
}
