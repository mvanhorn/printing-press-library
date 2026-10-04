// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/toretabi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/toretabi/internal/ticket"
	"github.com/spf13/cobra"
)

func ticketCachePath() (string, error) {
	dir, e := cliutil.CacheDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(dir, ticket.CacheFilename), nil
}
func ticketCommandError(cmd *cobra.Command, flags *rootFlags, e error) error {
	var typed *cliError
	if !errors.As(e, &typed) {
		var rate *cliutil.RateLimitError
		var h *ticket.HTTPError
		if errors.As(e, &rate) {
			e = rateLimitErr(e)
		} else if errors.As(e, &h) && h.Status == 404 {
			e = notFoundErr(e)
		} else {
			e = apiErr(e)
		}
	}
	writeAPIErrorEnvelope(cmd.OutOrStdout(), flags, e, ExitCode(e))
	return e
}
func validateTicketDate(name, s string) error {
	if s == "" {
		return nil
	}
	if _, e := time.Parse("2006-01-02", s); e != nil {
		return usageErr(fmt.Errorf("--%s must be an exact YYYY-MM-DD date", name))
	}
	return nil
}
func tokyoToday() string {
	return time.Now().In(time.FixedZone("Asia/Tokyo", 9*3600)).Format("2006-01-02")
}
func ticketGet(ctx context.Context, c *ticket.Client, id string, flags *rootFlags, asOf string) (ticket.Ticket, error) {
	if !ticket.ValidID(id) {
		return ticket.Ticket{}, usageErr(fmt.Errorf("ticket ID %q must look like tokai_043; URLs are not accepted", id))
	}
	path, e := ticketCachePath()
	if e != nil {
		return ticket.Ticket{}, e
	}
	readLocal := func() (ticket.Ticket, error) {
		xs, e := ticket.Cached(ctx, path, id, 50)
		if e != nil {
			return ticket.Ticket{}, fmt.Errorf("reading ticket cache: %w", e)
		}
		for _, t := range xs {
			if t.ID == id {
				markTicketFreshness(&t, flags.maxAge)
				return t, nil
			}
		}
		return ticket.Ticket{}, notFoundErr(fmt.Errorf("ticket %s is absent from local cache; run tickets get %s --data-source live first", id, id))
	}
	if flags.dataSource == "local" {
		if flags.noCache {
			return ticket.Ticket{}, usageErr(errors.New("--data-source local conflicts with --no-cache"))
		}
		return readLocal()
	}
	t, e := c.Get(ctx, id)
	if e != nil {
		var rate *cliutil.RateLimitError
		var h *ticket.HTTPError
		if flags.dataSource == "auto" && !flags.noCache && !errors.As(e, &rate) && !(errors.As(e, &h) && (h.Status == 404 || h.Status == 403)) {
			cached, cacheErr := readLocal()
			if cacheErr == nil {
				cached.Transport = "local_fallback"
				if cached.Operator != nil {
					cached.Operator.Transport = "local_fallback"
				}
				reason := e.Error()
				cached.SourceFailure = &reason
				return cached, nil
			}
			return ticket.Ticket{}, apiErr(fmt.Errorf("source read failed: %v; local fallback failed: %v", e, cacheErr))
		}
		return ticket.Ticket{}, e
	}
	op, opErr := c.ReadOperator(ctx, t, asOf)
	if opErr != nil {
		return ticket.Ticket{}, opErr
	}
	t.Operator = &op
	t.OperatorVerification = op.Status
	if !flags.noCache {
		if e = ticket.Save(ctx, path, t); e != nil {
			v := e.Error()
			t.CacheWarning = &v
		}
	}
	return t, nil
}
func ticketOutput(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFilteredKeep(cmd.OutOrStdout(), v, flags, "tickets", "ticket", "coverage", "comparisons", "fetch_failures", "observed_at", "source_boundary", "note", "areas", "types", "count", "capacity")
}
func warnTicket(cmd *cobra.Command, t ticket.Ticket) {
	if t.SourceFailure != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s source failed; explicit cached fallback from %s: %s\n", t.ID, t.ObservedAt, *t.SourceFailure)
	}
	if t.CacheWarning != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s fetched but cache save failed: %s\n", t.ID, *t.CacheWarning)
	}
}

func strOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Keep this source CLI's public operations strictly read-only.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, child := range root.Commands() {
			if child.Name() == "api" {
				root.RemoveCommand(child)
			}
		}
	})
}

// Both saved source clocks remain unchanged; freshness is a read-time view.
func markTicketFreshness(t *ticket.Ticket, maxAge time.Duration) {
	now := time.Now()
	t.Stale = false
	if at, e := time.Parse(time.RFC3339Nano, t.ObservedAt); e == nil {
		t.Stale = maxAge > 0 && now.Sub(at) > maxAge
	}
	if t.Operator == nil {
		return
	}
	o := t.Operator
	o.Stale = false
	o.Freshness = "unknown_clock"
	o.ObservationAgeSeconds = 0
	if o.ObservedAt == nil {
		return
	}
	at, e := time.Parse(time.RFC3339Nano, *o.ObservedAt)
	if e != nil {
		return
	}
	age := max(time.Duration(0), now.Sub(at))
	o.ObservationAgeSeconds = int64(age.Seconds())
	if maxAge <= 0 {
		o.Freshness = "disabled"
		return
	}
	o.Stale = age > maxAge
	o.Freshness = "fresh"
	if o.Stale {
		o.Freshness = "stale"
	}
}
