// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Hand-authored body.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

// pp:data-source live
//
// compare costs THREE requests and no more: the unified identity resolve, the
// iOS hub, and the Android batch lookup. The unified resolve is the only one of
// the three that needs a session cookie; it runs first so an unauthenticated
// caller fails fast with an auth error instead of burning the other two against
// a tight rate limit.
//
// The iOS hub ships install/revenue figures as pre-bucketed values with no
// display string, so they are humanized here into bucket labels — never printed
// as precise numbers.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

type compareSide struct {
	AppID            json.RawMessage `json:"app_id"`
	Name             string          `json:"name"`
	CurrentVersion   string          `json:"current_version"`
	Rating           *float64        `json:"rating"`
	RatingCount      *int64          `json:"rating_count"`
	CategoryRankings json.RawMessage `json:"category_rankings,omitempty"`
	Downloads        *string         `json:"downloads"`
}

type compareResult struct {
	UnifiedAppID string       `json:"unified_app_id,omitempty"`
	IOS          *compareSide `json:"ios"`
	Android      *compareSide `json:"android"`
	Note         string       `json:"note"`
}

const compareLadderNote = "iOS and Android chart ranks are separate ladders: a rank on one store is not comparable to a rank on the other, and the platforms' category taxonomies differ. Download figures are Sensor Tower one-significant-figure buckets, not precise counts."

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var flagCountry string

	cmd := &cobra.Command{
		Use:   "compare <ios-app-id> <android-package>",
		Short: "Compare one product's iOS and Android standing side by side after resolving cross-platform identity.",
		Long: "Put an app's iOS and Android records next to each other.\n\n" +
			"Resolves cross-platform identity through the unified endpoint (which requires a signed-in\n" +
			"session — run 'sensortower-pp-cli auth login --chrome' first), then fetches each platform's\n" +
			"record. Costs three API requests.\n\n" +
			"The two platforms' chart ranks are separate ladders and are reported side by side, never\n" +
			"differenced.",
		Example:     "  sensortower-pp-cli compare 460177396 tv.twitch.android.app --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			// Arg check precedes the dry-run short-circuit here: a --dry-run
			// probe of an under-specified invocation should still report the
			// usage problem it would hit.
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("both <ios-app-id> and <android-package> are required (e.g. 460177396 tv.twitch.android.app)"))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would resolve unified identity, then fetch the iOS and Android records (3 requests)")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			iosID, androidPkg := args[0], args[1]
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// 1/3: unified identity. Cookie-gated; fail fast before spending the
			// other two requests of the budget.
			unifiedRaw, err := c.Get(ctx, "/api/unified/apps", map[string]string{
				"app_id_type": "itunes",
				"app_ids":     iosID,
			})
			if err != nil {
				classified := classifyAPIError(err, flags)
				var typed *cliError
				if errors.As(classified, &typed) && typed.code == 4 {
					return authErr(fmt.Errorf("the unified cross-platform lookup requires a signed-in Sensor Tower session; "+
						"run 'sensortower-pp-cli auth login --chrome' and retry. Underlying error: %w", err))
				}
				return classified
			}
			var unified struct {
				Apps []struct {
					UnifiedAppID string `json:"unified_app_id"`
				} `json:"apps"`
			}
			if err := json.Unmarshal(unifiedRaw, &unified); err != nil {
				return apiErr(fmt.Errorf("decoding the unified apps response: %w", err))
			}
			result := compareResult{Note: compareLadderNote}
			if len(unified.Apps) > 0 {
				result.UnifiedAppID = unified.Apps[0].UnifiedAppID
			}

			// 2/3: the iOS hub.
			iosRaw, err := c.Get(ctx, replacePathParam("/api/ios/apps/{app_id}", "app_id", iosID), map[string]string{"country": flagCountry})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var iosHub struct {
				AppID            json.RawMessage `json:"app_id"`
				Name             string          `json:"name"`
				CurrentVersion   string          `json:"current_version"`
				Rating           *float64        `json:"rating"`
				RatingCount      *int64          `json:"rating_count"`
				CategoryRankings json.RawMessage `json:"category_rankings"`
				Downloads        *bucketedValue  `json:"worldwide_last_month_downloads"`
			}
			if err := json.Unmarshal(iosRaw, &iosHub); err != nil {
				return apiErr(fmt.Errorf("decoding the iOS app response: %w", err))
			}
			result.IOS = &compareSide{
				AppID:            iosHub.AppID,
				Name:             iosHub.Name,
				CurrentVersion:   iosHub.CurrentVersion,
				Rating:           iosHub.Rating,
				RatingCount:      iosHub.RatingCount,
				CategoryRankings: iosHub.CategoryRankings,
				// The hub has no humanized string of its own, so bucket-label it
				// rather than exposing the raw 1-sig-fig integer as a count.
				Downloads: humanizeBucket(iosHub.Downloads),
			}

			// 3/3: the Android batch endpoint (Android has no singular route).
			androidRaw, err := c.Get(ctx, "/api/android/apps", map[string]string{"app_ids": androidPkg})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var androidResp struct {
				Apps []struct {
					AppID       json.RawMessage     `json:"app_id"`
					Name        string              `json:"name"`
					Version     string              `json:"version"`
					Rating      *float64            `json:"rating"`
					RatingCount *int64              `json:"rating_count"`
					Downloads   *humanizedDownloads `json:"humanized_worldwide_last_month_downloads"`
				} `json:"apps"`
			}
			if err := json.Unmarshal(androidRaw, &androidResp); err != nil {
				return apiErr(fmt.Errorf("decoding the Android apps response: %w", err))
			}
			if len(androidResp.Apps) == 0 {
				// Report the gap; do not pass off a half-comparison as whole.
				result.Note = "no Android record came back for " + androidPkg + " — verify the package name. " + compareLadderNote
			} else {
				a := androidResp.Apps[0]
				result.Android = &compareSide{
					AppID:          a.AppID,
					Name:           a.Name,
					CurrentVersion: a.Version,
					Rating:         a.Rating,
					RatingCount:    a.RatingCount,
					Downloads:      downloadsBucket(a.Downloads),
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagCountry, "country", "US", "Two-letter country code for the iOS rank context (e.g. US, GB, JP)")
	return cmd
}
