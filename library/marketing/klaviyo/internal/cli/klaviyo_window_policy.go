// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var klaviyoWindowNow = time.Now

func metricNow() time.Time {
	return klaviyoWindowNow().In(loadNovelLocation(defaultMetricTimezone()))
}

func platformCommandUsesMetricAggregates(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "subject-line-analysis":
		flag := cmd.Flags().Lookup("source")
		return flag == nil || !strings.EqualFold(flag.Value.String(), "campaign")
	case "revenue-per-email":
		flag := cmd.Flags().Lookup("by")
		return flag == nil || !strings.EqualFold(flag.Value.String(), "campaign")
	case "report", "revenue", "open-rates", "unsubscribes", "spam-complaints", "list-growth", "deliverability", "domain-reputation", "products",
		"optimal-send-time", "flow-path-analysis", "list-quality-score":
		return true
	default:
		return false
	}
}

func platformCalculationWindow(flags *rootFlags, since, until time.Time) (time.Time, time.Time) {
	if flags != nil && flags.platformSession != nil {
		if window := flags.platformSession.OutputMetadata().ResolvedWindow; window != nil {
			return window.Start, window.End
		}
	}
	return since, until
}
