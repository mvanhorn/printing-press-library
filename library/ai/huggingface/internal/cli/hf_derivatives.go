package cli

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface/internal/hfx"
)

type derivativesResponse struct {
	hfx.Envelope
	Base    string             `json:"base"`
	Total   int                `json:"total"`
	Results []derivativeResult `json:"results"`
	Notes   string             `json:"notes,omitempty"`
	Explain string             `json:"explain,omitempty"`
}

type derivativeResult struct {
	ID            string   `json:"id"`
	Author        string   `json:"author"`
	Downloads     int      `json:"downloads"`
	Likes         int      `json:"likes"`
	LastModified  string   `json:"last_modified"`
	BaseModelTags []string `json:"base_model_tags,omitempty"`
	MatchSource   string   `json:"match_source"` // tag|cardData
}

func newHFDerivativesCmd(flags *rootFlags) *cobra.Command {
	var sinceFilter string
	cmd := &cobra.Command{
		Use:   "derivatives <base-id>",
		Short: "Find models fine-tuned from a given base model.",
		Long: `derivatives searches HF for models claiming the named base via cardData.base_model
or the tag 'base_model:<id>' / 'base_model:finetune:<id>' / 'base_model:quantized:<id>'.

HF has no first-class derivatives endpoint and no server-side base_model filter,
so this command does a search-then-client-filter pass. False-positive rate is
real because search-text matches more than just legitimate fine-tunes.`,
		Example: `  huggingface-pp-cli derivatives Qwen/Qwen2.5-7B
  huggingface-pp-cli derivatives meta-llama/Meta-Llama-3-8B --since 30d --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			base := args[0]
			ctx := cmd.Context()

			q := url.Values{}
			q.Set("search", base)
			pull := flags.limit * 4
			if pull < 100 {
				pull = 100
			}
			q.Set("limit", strconv.Itoa(pull))
			q.Set("sort", "downloads")
			q.Set("direction", "-1")
			q.Set("full", "true")

			ms, status, err := hfListModels(ctx, q, hfTokenForRequests())
			if err != nil {
				if status == 429 {
					return hfRateLimited("rate limited (HTTP 429)")
				}
				return err
			}

			var sinceCutoff time.Time
			sinceDays := 0
			if sinceFilter != "" {
				sinceDays = parseSinceDays(sinceFilter)
				if sinceDays > 0 {
					sinceCutoff = time.Now().UTC().Add(-time.Duration(sinceDays) * 24 * time.Hour)
				}
			}

			baseLower := strings.ToLower(base)
			results := []derivativeResult{}
			for _, m := range ms {
				// Skip the base itself
				if strings.EqualFold(m.ID, base) {
					continue
				}
				// Match: explicit base_model:* tag containing base, OR cardData.base_model contains base
				matchSource := ""
				baseTags := []string{}
				for _, t := range m.Tags {
					tl := strings.ToLower(t)
					if strings.HasPrefix(tl, "base_model:") && strings.Contains(tl, baseLower) {
						matchSource = "tag"
						baseTags = append(baseTags, t)
					}
				}
				if matchSource == "" {
					for _, bm := range hfBaseModelStrings(m.CardData.BaseModel) {
						if strings.EqualFold(bm, base) {
							matchSource = "cardData"
							baseTags = append(baseTags, "cardData:"+bm)
							break
						}
					}
				}
				if matchSource == "" {
					continue
				}
				if !sinceCutoff.IsZero() {
					if t, err := time.Parse(time.RFC3339Nano, m.LastModified); err == nil && t.Before(sinceCutoff) {
						continue
					}
				}
				results = append(results, derivativeResult{
					ID:            m.ID,
					Author:        m.Author,
					Downloads:     m.Downloads,
					Likes:         m.Likes,
					LastModified:  m.LastModified,
					BaseModelTags: baseTags,
					MatchSource:   matchSource,
				})
			}

			sort.SliceStable(results, func(i, j int) bool {
				return results[i].Downloads > results[j].Downloads
			})
			if flags.limit > 0 && len(results) > flags.limit {
				results = results[:flags.limit]
			}

			if len(results) == 0 {
				return hfNotFound("no derivatives matched base %q (HF search results may not surface the base; try a more specific search)", base)
			}

			resp := derivativesResponse{
				Envelope: hfx.NewEnvelope("derivatives"),
				Base:     base,
				Total:    len(results),
				Results:  results,
				Notes:    "HF has no server-side base_model filter; this is a search-then-client-filter pass. False positives possible.",
			}
			if flags.explain {
				resp.Explain = fmt.Sprintf("explain: pulled %d candidates via search=%s, kept %d that explicitly tag base_model:%s. Sort: downloads desc.",
					len(ms), base, len(results), base)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "derivatives of %s (%d results, sort=downloads)\n\n", base, len(results))
			fmt.Fprintf(cmd.OutOrStdout(), "  %-55s  %-12s  %-10s  %s\n", "ID", "AUTHOR", "DOWNLOADS", "MATCH")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-55s  %-12s  %-10d  %s\n", r.ID, r.Author, r.Downloads, r.MatchSource)
			}
			if resp.Explain != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", resp.Explain)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sinceFilter, "since", "", "Only include models modified within this window (e.g. 30d)")
	return cmd
}
