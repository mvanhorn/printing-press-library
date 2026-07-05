// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/internal/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/internal/cliutil"
)

// classifyAirbnb maps an airbnb client error onto the CLI's typed exit codes so
// agents get a stable signal (2 usage, 4 auth, 5 API, 7 rate-limit).
func classifyAirbnb(err error, flags *rootFlags) error {
	if err == nil {
		return nil
	}
	var rlErr *cliutil.RateLimitError
	if errors.As(err, &rlErr) {
		return rateLimitErr(err)
	}
	var unknownOp *airbnb.ErrUnknownOperation
	if errors.As(err, &unknownOp) {
		return apiErr(err)
	}
	var apiE *airbnb.APIError
	if errors.As(err, &apiE) {
		switch {
		case apiE.Airlock:
			return authErr(fmt.Errorf("%w\nhint: re-verify your session in a browser, then run 'airbnb-outreach-pp-cli auth login --chrome'", err))
		case apiE.StatusCode == 401 || apiE.StatusCode == 403:
			return authErr(fmt.Errorf("%w\nhint: run 'airbnb-outreach-pp-cli auth login --chrome' (private data needs a logged-in session)", err))
		case apiE.StatusCode == 404:
			return notFoundErr(err)
		case apiE.StatusCode == 429:
			return rateLimitErr(err)
		default:
			return apiErr(err)
		}
	}
	return apiErr(err)
}

// newAirbnbClient builds the hand-written Airbnb client from root flags and
// environment. Locale/currency default to en/USD; set AIRBNB_LOCALE and
// AIRBNB_CURRENCY (e.g. de / EUR) to change them. The client carries the
// persisted login session and the operation-hash registry.
func newAirbnbClient(flags *rootFlags) *airbnb.Client {
	locale := envOr("AIRBNB_LOCALE", "en")
	currency := envOr("AIRBNB_CURRENCY", "USD")
	base := envOr("AIRBNB_BASE_URL", "https://www.airbnb.com")
	c := airbnb.NewClient(airbnb.Options{
		BaseURL:  base,
		Locale:   locale,
		Currency: currency,
		Timeout:  flags.timeout,
	})
	c.DryRun = flags.dryRun
	return c
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
