// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/config"
)

type velocityReport struct {
	ThingID           string  `json:"thing_id"`
	Sub               string  `json:"subreddit"`
	Title             string  `json:"title"`
	CreatedUTC        float64 `json:"created_utc"`
	AgeMinutes        float64 `json:"age_minutes"`
	CommentsObserved  int     `json:"comments_observed"`
	CommentsPerMinute float64 `json:"comments_per_minute"`
	SubMedianCPM      float64 `json:"sub_median_cpm"`
	Percentile        float64 `json:"percentile_estimate"`
	Verdict           string  `json:"verdict"`
}

// newPostVelocityCmd estimates comments-per-minute for a submission, then
// compares it against the median CPM of recent hot posts in the same sub.
func newPostVelocityCmd(flags *rootFlags) *cobra.Command {
	var (
		baselineSample int
	)
	cmd := &cobra.Command{
		Use:   "velocity <submission-id>",
		Short: "Estimate comments/minute and compare to sub baseline (rising radar)",
		Long: `Compute a submission's comments-per-minute and compare to the median CPM of
the same sub's recent hot posts. Verdicts: hot (>= p90), rising (p50-p90),
average (p25-p50), slow (< p25).

The CLI calls /comments/<id> once for the target and a sample of recent hot
posts in the same sub for the baseline. The samples are small (default 50) so
this is a fast read.`,
		Example: `  reddit-pp-cli post velocity t3_abc123
  reddit-pp-cli post velocity abc123 --baseline-sample 30 --agent`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			id := strings.TrimSpace(args[0])
			id = strings.TrimPrefix(id, "t3_")
			if id == "" {
				return usageErr(fmt.Errorf("submission id required"))
			}
			if dryRunOK(flags) {
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := client.New(cfg, flags.timeout, flags.rateLimit)

			// Fetch target submission
			body, err := c.Get(cmd.Context(), "/comments/"+id, map[string]string{"limit": "1"})
			if err != nil {
				return apiErr(fmt.Errorf("fetching submission: %w", err))
			}
			meta := extractSubmissionMeta(body)
			if meta.ID == "" {
				return apiErr(fmt.Errorf("submission %s not found or has no data", id))
			}

			now := time.Now().Unix()
			ageMin := float64(now-int64(meta.CreatedUTC)) / 60.0
			if ageMin < 0.1 {
				ageMin = 0.1
			}
			targetCPM := float64(meta.NumComments) / ageMin

			// Baseline: sample from sub hot
			hotBody, err := c.Get(cmd.Context(), "/r/"+meta.Sub+"/hot", map[string]string{
				"limit": fmt.Sprintf("%d", baselineSample),
			})
			subMedianCPM, percentile := 0.0, 0.0
			if err == nil {
				cpms := extractSubCPMs(hotBody, now)
				if len(cpms) > 0 {
					sort.Float64s(cpms)
					subMedianCPM = cpms[len(cpms)/2]
					// Percentile of target vs sample
					rank := 0
					for _, v := range cpms {
						if targetCPM > v {
							rank++
						}
					}
					percentile = float64(rank) / float64(len(cpms)) * 100
				}
			}

			verdict := velocityVerdict(percentile)

			report := velocityReport{
				ThingID:           "t3_" + meta.ID,
				Sub:               meta.Sub,
				Title:             meta.Title,
				CreatedUTC:        meta.CreatedUTC,
				AgeMinutes:        ageMin,
				CommentsObserved:  meta.NumComments,
				CommentsPerMinute: targetCPM,
				SubMedianCPM:      subMedianCPM,
				Percentile:        percentile,
				Verdict:           verdict,
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			renderVelocity(cmd.OutOrStdout(), report)
			return nil
		},
	}
	cmd.Flags().IntVar(&baselineSample, "baseline-sample", 50, "How many hot posts to sample for sub baseline")
	return cmd
}

type subMeta struct {
	ID          string
	Sub         string
	Title       string
	CreatedUTC  float64
	NumComments int
}

func extractSubmissionMeta(body []byte) subMeta {
	// /comments/<id>.json returns an array: [submission_listing, comments_listing]
	var arr []struct {
		Data struct {
			Children []struct {
				Data struct {
					ID          string  `json:"id"`
					Subreddit   string  `json:"subreddit"`
					Title       string  `json:"title"`
					CreatedUTC  float64 `json:"created_utc"`
					NumComments int     `json:"num_comments"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &arr); err != nil {
		return subMeta{}
	}
	if len(arr) == 0 || len(arr[0].Data.Children) == 0 {
		return subMeta{}
	}
	c := arr[0].Data.Children[0].Data
	return subMeta{
		ID:          c.ID,
		Sub:         c.Subreddit,
		Title:       c.Title,
		CreatedUTC:  c.CreatedUTC,
		NumComments: c.NumComments,
	}
}

func extractSubCPMs(body []byte, now int64) []float64 {
	var env struct {
		Data struct {
			Children []struct {
				Data struct {
					CreatedUTC  float64 `json:"created_utc"`
					NumComments int     `json:"num_comments"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil
	}
	out := []float64{}
	for _, ch := range env.Data.Children {
		ageMin := float64(now-int64(ch.Data.CreatedUTC)) / 60.0
		if ageMin < 1 {
			continue
		}
		out = append(out, float64(ch.Data.NumComments)/ageMin)
	}
	return out
}

func velocityVerdict(p float64) string {
	switch {
	case p >= 90:
		return "hot"
	case p >= 50:
		return "rising"
	case p >= 25:
		return "average"
	default:
		return "slow"
	}
}

func renderVelocity(w io.Writer, r velocityReport) {
	fmt.Fprintf(w, "%s — r/%s — %s\n", r.ThingID, r.Sub, r.Title)
	fmt.Fprintf(w, "Age: %.1f min • Comments: %d • CPM: %.2f\n", r.AgeMinutes, r.CommentsObserved, r.CommentsPerMinute)
	fmt.Fprintf(w, "Sub median CPM: %.2f • Percentile: ~%.0f%% • Verdict: %s\n",
		r.SubMedianCPM, r.Percentile, r.Verdict)
}

var _ = context.Background
