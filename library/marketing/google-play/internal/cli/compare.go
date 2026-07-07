// Hand-authored transcendence command: multi-app side-by-side comparison.
package cli

import (
	"fmt"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/gplay"
)

type compareRow struct {
	AppID        string  `json:"appId"`
	Title        string  `json:"title"`
	Developer    string  `json:"developer"`
	Score        float64 `json:"score"`
	Ratings      int64   `json:"ratings"`
	Installs     string  `json:"installs"`
	RealInstalls int64   `json:"realInstalls"`
	Free         bool    `json:"free"`
	Price        float64 `json:"price"`
	OffersIAP    bool    `json:"offersIAP"`
	ContainsAds  bool    `json:"containsAds"`
	Updated      int64   `json:"updated"`
	Version      string  `json:"version"`
}

type fetchFailure struct {
	AppID string `json:"appId"`
	Error string `json:"error"`
}

type compareView struct {
	Items         []compareRow   `json:"items"`
	FetchFailures []fetchFailure `json:"fetch_failures"`
}

// pp:data-source live
func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <appId> <appId> [appId...]",
		Short: "Fetch details for several apps and lay their key fields side by side.",
		Long: "Fetch live details for multiple apps and transpose their key fields (installs, score, ratings, IAP, ads, version) " +
			"into one comparison table. Failed fetches are reported separately and excluded from the comparison rows.\n\n" +
			"Use this command to compare current details of multiple apps at once. For one app's full field set use 'app'; " +
			"for change-over-time on a single app use 'watch-listing'.",
		Example:     "  google-play-pp-cli compare com.dreamgames.royalkingdom com.dreamgames.royalmatch --agent --select items.appId,items.score,items.offersIAP",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare apps")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare needs at least two appIds"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newGplayClient(cmd, flags)

			type res struct {
				idx int
				app *gplay.App
				err error
			}
			results := make(chan res, len(args))
			sem := make(chan struct{}, 4)
			var wg sync.WaitGroup
			for i, id := range args {
				wg.Add(1)
				go func(i int, id string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					a, err := c.AppDetails(ctx, id)
					results <- res{idx: i, app: a, err: err}
				}(i, id)
			}
			go func() { wg.Wait(); close(results) }()

			rows := make([]*compareRow, len(args))
			errs := make([]string, len(args))
			var sampleErr error   // a representative failure for the all-failed path
			var rateLimited error // prefer surfacing a rate-limit over a generic error
			for r := range results {
				if r.err != nil {
					errs[r.idx] = r.err.Error()
					if sampleErr == nil {
						sampleErr = r.err
					}
					if _, ok := r.err.(*cliutil.RateLimitError); ok && rateLimited == nil {
						rateLimited = r.err
					}
					continue
				}
				a := r.app
				rows[r.idx] = &compareRow{
					AppID: a.AppID, Title: a.Title, Developer: a.Developer, Score: a.Score,
					Ratings: a.Ratings, Installs: a.Installs, RealInstalls: a.RealInstalls,
					Free: a.Free, Price: a.Price, OffersIAP: a.OffersIAP, ContainsAds: a.ContainsAds,
					Updated: a.Updated, Version: a.Version,
				}
			}
			view := compareView{Items: []compareRow{}, FetchFailures: []fetchFailure{}}
			for i := range args {
				if rows[i] != nil {
					view.Items = append(view.Items, *rows[i])
				} else {
					view.FetchFailures = append(view.FetchFailures, fetchFailure{AppID: args[i], Error: errs[i]})
				}
			}
			if len(view.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d app fetches failed; comparison covers the remaining %d\n",
					len(view.FetchFailures), len(args), len(view.Items))
			}
			// All failed: surface a rate-limit (retryable, actionable) if any
			// fetch hit one, otherwise the first error with its real type.
			if len(view.Items) == 0 {
				if rateLimited != nil {
					return classifyGplayErr(rateLimited)
				}
				if sampleErr != nil {
					return classifyGplayErr(sampleErr)
				}
				return apiErr(fmt.Errorf("all %d app fetches failed", len(args)))
			}
			return emit(cmd, flags, view)
		},
	}
	return cmd
}
