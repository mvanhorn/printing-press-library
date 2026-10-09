// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/internal/tickets"
)

// sourceCheck is one doctor result for a sight's official sources.
type sourceCheck struct {
	Sight  string `json:"sight"`
	Name   string `json:"name"`
	Status string `json:"status"` // ok, expected-boundary, error
	Detail string `json:"detail"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, c := range root.Commands() {
			if c.Name() == "doctor" {
				wrapDoctorWithSources(c, flags)
			}
		}
	})
}

// doctorBoundaries are the data the CLI never reads by design.
var doctorBoundaries = []sourceCheck{
	{Sight: "shibuya-sky", Name: "SHIBUYA SKY slot stock (Webket)", Status: "expected-boundary",
		Detail: "not probed: the store sits behind a virtual waiting room (Queue-it) that this CLI never joins; slot stock is reported as unknown"},
	{Sight: "ghibli-museum", Name: "Ghibli Museum per-date stock (Lawson Ticket)", Status: "expected-boundary",
		Detail: "not probed: needs a Lawson Ticket member login, which this CLI never uses; per-date stock is reported as unknown"},
}

// runDoctorSourceChecks reads every sight for today with the same readers and
// parsers the commands use, so doctor fails when a parser no longer finds its
// rule, calendar or bundle.
func runDoctorSourceChecks(cmd *cobra.Command, flags *rootFlags) []sourceCheck {
	ctx, cancelBound := boundCtx(cmd.Context(), flags)
	defer cancelBound()
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	sights := tickets.All()
	today := tickets.DateOf(clockNow())
	f := newTicketsFetcher(cmd.ErrOrStderr(), flags, tickets.RequestBudget(sights, 0))
	data, _ := tickets.Collect(ctx, f, sights, today, today, nil, clockNow)
	checks := make([]sourceCheck, 0, len(sights)+len(doctorBoundaries))
	for _, s := range sights {
		c := sourceCheck{Sight: s.ID, Name: s.NameEN, Status: "ok"}
		sd := data[s.ID]
		switch {
		case sd == nil:
			c.Status, c.Detail = "error", "not read"
		case !sd.RuleConfirmed || len(sd.Warnings) > 0:
			c.Status = "error"
			c.Detail = "rule or calendar not confirmed"
			if len(sd.Warnings) > 0 {
				c.Detail = strings.Join(sd.Warnings, "; ")
			}
		default:
			ok := 0
			for _, src := range sd.Sources {
				if src.OK {
					ok++
				}
			}
			noun := "sources"
			if ok == 1 {
				noun = "source"
			}
			c.Detail = fmt.Sprintf("%d %s read and parsed", ok, noun)
		}
		checks = append(checks, c)
	}
	return append(checks, doctorBoundaries...)
}

func wrapDoctorWithSources(doctor *cobra.Command, flags *rootFlags) {
	orig := doctor.RunE
	if orig == nil {
		return
	}
	doctor.Long = "Check CLI health and read each official source with the same parsers the commands use. The SHIBUYA SKY waiting room and the Lawson Ticket member login are reported as expected boundaries, not failures."
	doctor.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return orig(cmd, args)
		}
		failOn, _ := cmd.Flags().GetString("fail-on")
		out := cmd.OutOrStdout()
		if flags.asJSON {
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			origErr := orig(cmd, args)
			cmd.SetOut(out)
			report := map[string]any{}
			if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
				_, _ = out.Write(buf.Bytes())
				return origErr
			}
			checks := runDoctorSourceChecks(cmd, flags)
			report["sources"] = checks
			if err := printJSONFiltered(out, report, flags); err != nil {
				return err
			}
			if origErr != nil {
				return origErr
			}
			return sourcesFailOn(failOn, checks)
		}
		origErr := orig(cmd, args)
		checks := runDoctorSourceChecks(cmd, flags)
		fmt.Fprintln(out, "  Sources:")
		for _, c := range checks {
			ind := green("OK")
			switch c.Status {
			case "expected-boundary":
				ind = yellow("INFO")
			case "error":
				ind = red("FAIL")
			}
			fmt.Fprintf(out, "    %s %s: %s\n", ind, c.Name, oneLine(c.Detail))
		}
		if origErr != nil {
			return origErr
		}
		return sourcesFailOn(failOn, checks)
	}
}

func sourcesFailOn(failOn string, checks []sourceCheck) error {
	if strings.TrimSpace(failOn) == "" {
		return nil
	}
	bad := make([]string, 0)
	for _, c := range checks {
		if c.Status == "error" {
			bad = append(bad, c.Sight)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	return apiErr(fmt.Errorf("doctor: source checks failed: %s", strings.Join(bad, ", ")))
}
