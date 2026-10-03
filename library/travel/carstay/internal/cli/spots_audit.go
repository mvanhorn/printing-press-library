// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
// pp:client-call shared RunE uses internal/carstay public Directory/Detail/DateCandidates; see carstay_commands.go.
package cli

import "github.com/spf13/cobra"

func newNovelSpotsAuditCmd(flags *rootFlags) *cobra.Command {
	o := defaultCarstayOptions()
	cmd := &cobra.Command{Use: "audit <id> [second] [third] [fourth] [fifth]"}
	configureCarstayCommand(cmd, flags, "audit", &o)
	cmd.Flags().StringVar(&o.checkIn, "check-in", "", "Japan-local check-in date in YYYY-MM-DD; provide check-out too")
	cmd.Flags().StringVar(&o.checkOut, "check-out", "", "Japan-local check-out date later than check-in in YYYY-MM-DD")
	cmd.Flags().IntVar(&o.maxPages, "max-scan-pages", 2, "Maximum 1-based provider date-search pages to examine, from 1 to 10")
	cmd.Flags().IntVar(&o.textLimit, "text-limit", 2000, "Maximum Unicode characters per source text field; larger rules stay on the canonical page")
	return cmd
}
