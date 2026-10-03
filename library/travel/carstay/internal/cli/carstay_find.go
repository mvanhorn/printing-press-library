// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import "github.com/spf13/cobra"

func newCarstayFindCmd(flags *rootFlags) *cobra.Command {
	o := defaultCarstayOptions()
	cmd := &cobra.Command{Use: "find [query]"}
	configureCarstayCommand(cmd, flags, "find", &o)
	cmd.Flags().StringVar(&o.language, "lang", "ja", "Published discovery subset or handoff language: ja (fuller) or en (English-approved only)")
	cmd.Flags().IntVar(&o.limit, "limit", 10, "Maximum result or coverage-gap rows, from 1 to 50")
	cmd.Flags().IntVar(&o.maxScan, "max-scan-records", 500, "Maximum directory records to examine independently of returned rows")
	cmd.Flags().StringVar(&o.prefecture, "prefecture", "", "Japanese prefecture or English name, such as Yamanashi")
	cmd.Flags().StringVar(&o.query, "query", "", "Text terms to match in original or source English station fields")
	cmd.Flags().StringVar(&o.checkIn, "check-in", "", "Japan-local check-in date in YYYY-MM-DD; provide check-out too")
	cmd.Flags().StringVar(&o.checkOut, "check-out", "", "Japan-local check-out date later than check-in in YYYY-MM-DD")
	cmd.Flags().IntVar(&o.maxPages, "max-scan-pages", 2, "Maximum 1-based provider date-search pages to examine, from 1 to 10")
	return cmd
}
