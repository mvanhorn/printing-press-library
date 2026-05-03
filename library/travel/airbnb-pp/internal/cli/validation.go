package cli

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// validatePropertyType returns nil for documented Airbnb property-type values
// and a usage error for anything else. The empty string is allowed (no filter).
//
// Pre-fix: passing --property-type bogus returned an empty result set with
// exit 0; passing --property-type entire_home returned an empty result set
// silently (the underlying URL param mapping is broken upstream).
func validatePropertyType(t string) error {
	if t == "" {
		return nil
	}
	allowed := []string{"entire_home", "private_room", "shared_room", "hotel_room"}
	for _, a := range allowed {
		if t == a {
			return nil
		}
	}
	sort.Strings(allowed)
	return fmt.Errorf("unknown --property-type %q; expected one of: %s", t, strings.Join(allowed, ", "))
}

// validateDates returns nil when both dates are empty (no filter), or when the
// dates form a valid forward-looking stay. The fixed reference today is
// dateNow(), which uses the local timezone.
//
// Pre-fix: --checkin 2025-01-01 --checkout 2025-01-04 (past) returned full
// results; --checkin 2026-05-09 --checkout 2026-05-06 (reversed) returned
// full results. Both shapes silently masked the user's mistake.
func validateDates(checkin, checkout string) error {
	if checkin == "" && checkout == "" {
		return nil
	}
	if checkin == "" || checkout == "" {
		return errors.New("--checkin and --checkout must both be set or both be empty")
	}
	in, err := time.Parse("2006-01-02", checkin)
	if err != nil {
		return fmt.Errorf("--checkin %q: expected YYYY-MM-DD", checkin)
	}
	out, err := time.Parse("2006-01-02", checkout)
	if err != nil {
		return fmt.Errorf("--checkout %q: expected YYYY-MM-DD", checkout)
	}
	if !out.After(in) {
		return fmt.Errorf("--checkout (%s) must be after --checkin (%s)", checkout, checkin)
	}
	today := dateNow()
	if in.Before(today) {
		return fmt.Errorf("--checkin %s is in the past (today is %s)", checkin, today.Format("2006-01-02"))
	}
	return nil
}

// dateNow returns midnight today in local timezone. Pulled out as a function
// so tests can swap in a fixed clock.
var dateNow = func() time.Time {
	now := time.Now().Local()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// ErrListingNotFound is returned by source layers when a listing ID resolves
// to no listing on the platform. CLI layer maps it to exit code 3.
var ErrListingNotFound = errors.New("listing not found")
