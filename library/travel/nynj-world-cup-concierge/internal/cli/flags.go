// Copyright 2026 USER and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

func bindCommonSourceFlags(cmd *cobra.Command, opts *sourceOptions) {
	cmd.Flags().StringVar(&opts.EventGUID, "event-guid", opts.EventGUID, "NYNJ Concierge event GUID")
	cmd.Flags().StringVar(&opts.APIBase, "api-base", opts.APIBase, "Neurun Concierge API base URL")
	cmd.Flags().StringVar(&opts.Lang, "lang", opts.Lang, "Prompt language")
	cmd.Flags().IntVar(&opts.TimeoutSeconds, "timeout", opts.TimeoutSeconds, "HTTP timeout in seconds")
	cmd.Flags().BoolVar(&opts.Pretty, "pretty", opts.Pretty, "Emit indented JSON")
	cmd.Flags().BoolVar(&opts.Agent, "agent", opts.Agent, "Emit compact JSON for agent ingestion")
	cmd.Flags().StringVar(&opts.EventJSON, "event-json", opts.EventJSON, "Read event JSON from a local fixture")
	cmd.Flags().StringVar(&opts.PromptsJSON, "prompts-json", opts.PromptsJSON, "Read prompts JSON from a local fixture")
	cmd.Flags().StringVar(&opts.DestinationHTML, "destination-html", opts.DestinationHTML, "Read destination HTML from a local fixture")
	cmd.Flags().StringVar(&opts.FanEventsHTML, "fan-events-html", opts.FanEventsHTML, "Read fan-events HTML from a local fixture")
}

func bindExtractFilterFlags(cmd *cobra.Command, opts *sourceOptions) {
	cmd.Flags().StringArrayVar(&opts.Categories, "category", opts.Categories, "Filter by category name. Repeatable.")
	cmd.Flags().StringVar(&opts.DateWindowStart, "date-window-start", opts.DateWindowStart, "Window start (YYYY-MM-DD): only include candidates whose parsed date range ends on or after this date")
	cmd.Flags().StringVar(&opts.DateWindowEnd, "date-window-end", opts.DateWindowEnd, "Window end (YYYY-MM-DD): only include candidates whose parsed date range starts on or before this date")
	cmd.Flags().BoolVar(&opts.ExcludeUndated, "exclude-undated", opts.ExcludeUndated, "Drop candidates with no parseable date_text when a date window is set")
}
