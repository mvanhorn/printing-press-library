// pp:data-source live
package cli

import (
	"context"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/ikyu"
	"github.com/spf13/cobra"
	"regexp"
	"strings"
)

type offerSelection struct{ PropertyID, RoomID, PlanID string }

func (s offerSelection) String() string { return s.PropertyID + ":" + s.RoomID + ":" + s.PlanID }

var stayID = regexp.MustCompile(`^[0-9]{8}$`)

func parseSelection(s string) (offerSelection, error) {
	p := strings.Split(strings.TrimSpace(s), ":")
	if len(p) != 3 {
		return offerSelection{}, usageErr(fmt.Errorf("offer selector must be property:room:plan, with three eight-digit IDs"))
	}
	for _, id := range p {
		if !stayID.MatchString(id) {
			return offerSelection{}, usageErr(fmt.Errorf("offer selector %q must contain three eight-digit IDs", s))
		}
	}
	return offerSelection{p[0], p[1], p[2]}, nil
}

type offerRow struct {
	Selector  string          `json:"selector"`
	Stay      ikyu.Stay       `json:"stay"`
	Data      *ikyu.OfferData `json:"data"`
	Freshness *ikyu.Freshness `json:"freshness"`
	Error     *string         `json:"error"`
}
type offerFailure struct {
	Selector    string `json:"selector"`
	Error       string `json:"error"`
	sourceError error
}
type offerJob struct {
	Key       string
	Selection offerSelection
	Stay      ikyu.Stay
}

func fetchOfferRows(ctx context.Context, c ikyu.Reader, jobs []offerJob) ([]offerRow, []offerFailure, map[string]ikyu.OfferResult) {
	successes, failures := cliutil.FanoutRun(ctx, jobs, func(j offerJob) string { return j.Key }, func(ctx context.Context, j offerJob) (ikyu.OfferResult, error) {
		return c.Offer(ctx, ikyu.OfferRequest{PropertyID: j.Selection.PropertyID, RoomID: j.Selection.RoomID, PlanID: j.Selection.PlanID, Stay: j.Stay})
	}, cliutil.WithConcurrency(2))
	results := map[string]ikyu.OfferResult{}
	messages := map[string]string{}
	sourceErrors := map[string]error{}
	for _, s := range successes {
		results[s.Source] = s.Value
	}
	for _, e := range failures {
		messages[e.Source] = boundedOfferError(e.Err)
		sourceErrors[e.Source] = e.Err
	}
	rows := make([]offerRow, 0, len(jobs))
	errs := []offerFailure{}
	for _, j := range jobs {
		row := offerRow{Selector: j.Key, Stay: j.Stay}
		if result, ok := results[j.Key]; ok {
			data, fresh := result.Data, result.Freshness
			row.Data = &data
			row.Freshness = &fresh
		} else {
			message := messages[j.Key]
			if message == "" {
				message = "source result missing"
			}
			row.Error = &message
			errs = append(errs, offerFailure{Selector: j.Key, Error: message, sourceError: sourceErrors[j.Key]})
		}
		rows = append(rows, row)
	}
	return rows, errs, results
}
func reportOfferFailures(cmd *cobra.Command, failures []offerFailure, total int) {
	if len(failures) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d selected reads failed; %d successful observations retained\n", len(failures), total, total-len(failures))
	}
}

func boundedOfferError(err error) string {
	text := []rune(cliutil.ScrubTerminal(err.Error()))
	if len(text) > 512 {
		text = append(text[:512], '…')
	}
	return string(text)
}
func allOfferFailures(failures []offerFailure, total int) error {
	for _, failure := range failures {
		var rate *cliutil.RateLimitError
		if errors.As(failure.sourceError, &rate) {
			return rateLimitErr(failure.sourceError)
		}
	}
	return apiErr(fmt.Errorf("all %d selected source reads failed", total))
}
