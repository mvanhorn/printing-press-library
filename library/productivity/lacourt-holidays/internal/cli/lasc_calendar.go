// Copyright 2026 Bob Benjy and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared holiday loader for the hand-written court-day commands.

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	_ "time/tzdata" // embed tz database so America/Los_Angeles resolves everywhere

	"github.com/mvanhorn/printing-press-library/library/productivity/lacourt-holidays/internal/courtdays"
)

// newLASCCalendar returns a court-day calendar that fetches each year's
// holidays from the court on demand through the generated client (which
// carries the response cache, timeout, and verify-mode mock).
func newLASCCalendar(cmd *cobra.Command, flags *rootFlags) (*courtdays.Calendar, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	ctx := cmd.Context()
	return courtdays.NewCalendar(func(year int) ([]courtdays.Holiday, error) {
		data, err := c.Get(ctx, "/api/holidays/getholidays/"+strconv.Itoa(year), nil)
		if err != nil {
			return nil, apiErr(fmt.Errorf("fetching %d holidays from lacourt.ca.gov: %w", year, err))
		}
		hs, err := courtdays.ParseHolidays(data)
		if err != nil {
			return nil, apiErr(fmt.Errorf("unexpected holiday payload for %d (the court may have changed its format): %w", year, err))
		}
		return hs, nil
	}), nil
}

// laToday returns today's calendar date in Los Angeles, regardless of the
// machine's time zone, as UTC midnight.
func laToday() time.Time {
	now := time.Now()
	if loc, err := time.LoadLocation("America/Los_Angeles"); err == nil {
		now = now.In(loc)
	}
	return courtdays.Midnight(now)
}

// parseUserDate accepts YYYY-MM-DD, MM/DD/YYYY, "today", or "tomorrow"
// (today/tomorrow are Los Angeles dates).
func parseUserDate(s string) (time.Time, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "today":
		return laToday(), nil
	case "tomorrow":
		return laToday().AddDate(0, 0, 1), nil
	}
	t, err := courtdays.ParseDate(s)
	if err != nil {
		return t, usageErr(err)
	}
	return t, nil
}

// parseCount parses a day count positional; --back flips its sign.
func parseCount(s string, back bool) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, usageErr(fmt.Errorf("invalid day count %q: must be a whole number", s))
	}
	if back && n > 0 {
		n = -n
	}
	return n, nil
}

// calendarErr maps an unpublished-year answer to a not-found exit (3) with
// an actionable message; other errors pass through.
func calendarErr(err error) error {
	if courtdays.IsUnpublished(err) {
		return notFoundErr(fmt.Errorf("%v (the court publishes past years back to 2006 and about five years ahead; see 'lacourt-holidays-pp-cli holidays --year <year>')", err))
	}
	return err
}

// publishedYears probes the court for every year it has published, starting
// at the current year and walking forward and backward until an empty year.
func publishedYears(cal *courtdays.Calendar) ([]int, error) {
	now := laToday().Year()
	years := []int{}
	for y := now - 1; y >= now-40; y-- {
		if _, err := cal.Year(y); err != nil {
			if courtdays.IsUnpublished(err) {
				break
			}
			return nil, err
		}
		years = append([]int{y}, years...)
	}
	for y := now; y <= now+15; y++ {
		if _, err := cal.Year(y); err != nil {
			if courtdays.IsUnpublished(err) {
				break
			}
			return nil, err
		}
		years = append(years, y)
	}
	return years, nil
}
