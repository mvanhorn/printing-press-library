package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPCategoryTopCmd(flags *rootFlags) *cobra.Command {
	var limit, minReviews int
	cmd := &cobra.Command{
		Use:   "category-top <slug>",
		Short: "Rank Trustpilot category page entries by TrustScore",
		Example: `  trustpilot-pp-cli category-top books --limit 25 --min-reviews 100 --json
  trustpilot-pp-cli category-top electronics_technology`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := args[0]
			ctx := cmd.Context()
			db, err := openTPStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			sess, _, err := loadOrHarvestSession(ctx, db, true)
			if err != nil {
				return err
			}
			body, err := fetchCategoryHTML(ctx, sess, slug)
			if err != nil {
				return err
			}
			_, props, err := tpkg.ParseNextDataHTML(body)
			if err != nil {
				return err
			}
			companies, err := parseCategoryCompanies(props)
			if err != nil {
				return err
			}
			filtered := make([]map[string]any, 0, len(companies))
			for _, c := range companies {
				if c.NumberOfReviews >= minReviews {
					filtered = append(filtered, map[string]any{
						"domain":          c.IdentifyingName,
						"displayName":     c.DisplayName,
						"trustScore":      c.TrustScore,
						"stars":           c.Stars,
						"numberOfReviews": c.NumberOfReviews,
					})
				}
			}
			sort.Slice(filtered, func(i, j int) bool {
				return filtered[i]["trustScore"].(float64) > filtered[j]["trustScore"].(float64)
			})
			if limit > 0 && len(filtered) > limit {
				filtered = filtered[:limit]
			}
			payload := map[string]any{"slug": slug, "companies": filtered}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, payload)
			}
			for _, c := range filtered {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\tTrustScore %.1f\t%d reviews\n",
					c["domain"], c["displayName"], c["trustScore"], c["numberOfReviews"])
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum companies to return")
	cmd.Flags().IntVar(&minReviews, "min-reviews", 100, "Minimum review count")
	return cmd
}

func fetchCategoryHTML(ctx context.Context, sess tpkg.Session, slug string) ([]byte, error) {
	if sess.UserAgent == "" {
		sess.UserAgent = tpkg.DefaultUserAgent
	}
	u := "https://www.trustpilot.com/categories/" + url.PathEscape(slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", sess.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if sess.CookieJar != "" {
		req.Header.Set("Cookie", sess.CookieJar)
	} else if sess.AWSWAFToken != "" {
		req.Header.Set("Cookie", "aws-waf-token="+sess.AWSWAFToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("trustpilot category returned HTTP %d: %s", resp.StatusCode, truncateText(string(body), 200))
	}
	return body, nil
}

func parseCategoryCompanies(props json.RawMessage) ([]tpkg.BusinessUnit, error) {
	var raw struct {
		BusinessUnits []struct {
			IdentifyingName string  `json:"identifyingName"`
			DisplayName     string  `json:"displayName"`
			TrustScore      float64 `json:"trustScore"`
			Stars           float64 `json:"stars"`
			NumberOfReviews int     `json:"numberOfReviews"`
		} `json:"businessUnits"`
	}
	if err := json.Unmarshal(props, &raw); err != nil {
		return nil, err
	}
	out := make([]tpkg.BusinessUnit, 0, len(raw.BusinessUnits))
	for _, b := range raw.BusinessUnits {
		out = append(out, tpkg.BusinessUnit{
			IdentifyingName: b.IdentifyingName,
			DisplayName:     b.DisplayName,
			TrustScore:      b.TrustScore,
			Stars:           b.Stars,
			NumberOfReviews: b.NumberOfReviews,
		})
	}
	return out, nil
}
