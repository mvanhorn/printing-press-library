// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/travel/jr-east-status/internal/jreast"
	"strings"
	"time"
)

func newNovelCertificatesCmd(flags *rootFlags) *cobra.Command {
	var line, slot string
	var limit int
	cmd := &cobra.Command{Use: "certificates", Short: "List covered lines or currently published certificate handoffs", Long: "Currently published links only. Dates come from actual source links, including service-day rollover. The rounded route maximum does not describe your individual train. A dash/absent link remains unknown. Use the official website for prior dates.", Example: "  jr-east-status-pp-cli certificates --line yamanoteline --slot 02 --agent", Annotations: jrAnnotations("--line=yamanoteline;--agent=true", "live")}
	cmd.Flags().StringVar(&line, "line", "", "Native ID or exact Japanese certificate line name; omit for line discovery")
	cmd.Flags().StringVar(&slot, "slot", "", "Source time slot 01–05; omit for all five slots of the selected line")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum certificate lines returned, between 1 and 30")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "read published certificate handoffs")
		}
		if e := jrLiveMode(flags); e != nil {
			return e
		}
		if e := jrNoPositionals(args); e != nil {
			return e
		}
		if e := jrLimit(limit, 30); e != nil {
			return e
		}
		if slot != "" && (slot < "01" || slot > "05" || len(slot) != 2) {
			return usageErr(fmt.Errorf("--slot must be 01, 02, 03, 04 or 05"))
		}
		if slot != "" && line == "" {
			return usageErr(fmt.Errorf("--slot requires --line"))
		}
		if slot != "" && jreast.CertificateRoute(line) != nil {
			return usageErr(fmt.Errorf("--slot is unavailable for this route handoff; omit --slot or choose the actual travelled segment for a published slot link"))
		}
		ctx, cancel := jrContext(cmd, flags)
		defer cancel()
		now := time.Now()
		c := jreast.NewClient(1, flags.rateLimit)
		m := jrMeta(c, now)
		m.Note = "Local conventional trains only, approximately five-minute-plus delays. Publication follows confirmation near 07:00/10:00/16:00/21:00 and 02:00 following day JST; a missing link is not a zero-delay proof. Confirm travelled section at section_coverage_url."
		body, e := c.Get(ctx, jreast.CertificateURL)
		if e != nil {
			return jrSourceError(e)
		}
		certs, state, e := jreast.ParseCertificates(body, now)
		if e != nil {
			return apiErr(e)
		}
		m.Sources = append(m.Sources, state)
		m.RequestCount = c.RequestCount()
		out := make([]jreast.Certificate, 0)
		if route := jreast.CertificateRoute(line); route != nil {
			out = append(out, *route)
			return flags.printJSON(cmd, jreast.Envelope[jreast.Certificate]{Meta: m, Results: out})
		}
		q := jreast.Normalize(strings.TrimPrefix(line, "kanto:"))
		total := 0
		for _, cert := range certs {
			match := line == "" || jreast.Normalize(cert.NameJA) == q
			for _, id := range cert.LineIDs {
				if jreast.Normalize(id) == q {
					match = true
				}
			}
			if !match {
				continue
			}
			total++
			if len(out) >= limit {
				continue
			}
			if line == "" {
				cert.Slots = nil
			} else if slot != "" {
				for _, s := range cert.Slots {
					if s.ID == slot {
						cert.Slots = []jreast.CertificateSlot{s}
						break
					}
				}
			}
			out = append(out, cert)
		}
		m.Truncated = total > len(out)
		if len(out) == 0 {
			m.Note = "This current table does not contain the requested native line/name. Other coverage or prior dates require the official certificate/DOKOTORE handoff; no delay conclusion is established."
		}
		return flags.printJSON(cmd, jreast.Envelope[jreast.Certificate]{Meta: m, Results: out})
	}
	return cmd
}
