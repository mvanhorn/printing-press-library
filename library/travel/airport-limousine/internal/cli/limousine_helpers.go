// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/limousine"
)

func limousineAnnotations(happy string) map[string]string {
	return map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": happy}
}
func limousineError(err error) error {
	var rate *cliutil.RateLimitError
	if errors.As(err, &rate) {
		return rateLimitErr(err)
	}
	return apiErr(err)
}
func limousineLimit(n int) error {
	if n < 1 || n > 200 {
		return usageErr(fmt.Errorf("--limit must be 1–200"))
	}
	return nil
}
func limousineAirport(ap string) error {
	if ap != "" && ap != "haneda" && ap != "narita" {
		return usageErr(fmt.Errorf("--airport must be haneda or narita"))
	}
	return nil
}
func limousineDirection(d string, optional bool) error {
	if optional && d == "" {
		return nil
	}
	if d != "from-airport" && d != "to-airport" {
		return usageErr(fmt.Errorf("--direction must be from-airport or to-airport"))
	}
	return nil
}
func limousineDate(d string) (string, error) {
	v, e := limousine.ServiceDate(d, time.Now())
	if e != nil {
		return "", usageErr(e)
	}
	return v, nil
}
func limousineRoute(args []string, route string) (string, error) {
	if len(args) > 1 {
		return "", usageErr(fmt.Errorf("provide one route ID"))
	}
	if len(args) == 1 {
		if route != "Haneda-Narita" && route != args[0] {
			return "", usageErr(fmt.Errorf("route positional and --route disagree"))
		}
		route = args[0]
	}
	if e := limousine.ValidateID(route); e != nil {
		return "", usageErr(e)
	}
	return route, nil
}
func limousineNote(total, kept int, note string) string {
	if total == 0 {
		return "No matching source records; " + note
	}
	if total > kept {
		return note + " Output is capped by --limit."
	}
	return note
}
func limousineQuery(q string) string { return strings.TrimSpace(q) }

func limousineLive(flags *rootFlags) error {
	if flags.dataSource != "" && flags.dataSource != "auto" && flags.dataSource != "live" {
		return usageErr(fmt.Errorf("this command requires live Airport Limousine data; use --data-source live or auto"))
	}
	return nil
}
