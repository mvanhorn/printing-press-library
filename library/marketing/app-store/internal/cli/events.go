// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newEventsCmd(flags))
	})
}

// eventRow is one in-app event card. status is derived from the dates:
// upcoming, live, ended, or unknown when Apple sent no dates.
type eventRow struct {
	EventID    string `json:"eventId"`
	Title      string `json:"title"`
	Subtitle   string `json:"subtitle,omitempty"`
	Kind       string `json:"kind"`
	StartDate  string `json:"startDate"`
	EndDate    string `json:"endDate"`
	Badge      string `json:"badge"`
	Status     string `json:"status"`
	Detail     string `json:"detail"`
	Storefront string `json:"storefront"`
}

func newEventsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events <app>",
		Short: "Fetch an app's in-app events live: title, kind, start and end, live badge",
		Long: strings.Trim(`
List the in-app events on an app's product page: event id, title, kind
(CHALLENGE, MAJOR UPDATE, COMPETITION ...), start and end time (RFC3339 UTC),
Apple's badge (available, happening) and a status derived from the dates
(upcoming, live, ended). Events are recorded with first and last sighting so
changes can report new ones.

An app with no events returns [] and a note. Use 'app --full' for events
together with the rest of the listing. An unknown id exits 3.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli events 553834731 --json
  app-store-pp-cli events 850417475 --country gb`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
			"pp:happy-args":       "app=553834731",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "events")
			}
			ref, err := s2aOneAppArg(cmd, args)
			if err != nil {
				return err
			}
			cc, err := refCountry(cmd, flags, ref)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newAppStoreClient(flags)
			if err != nil {
				return err
			}
			st, err := openAppStoreStore(cmd, flags)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			attachCaptureLog(c, st)

			lk, err := lookupAppRef(ctx, c, ref, cc)
			if err != nil {
				return err
			}
			page, err := s2aProductPage(ctx, c, lk, cc, "")
			if err != nil {
				return err
			}
			s2aWarnStore(cmd, "the in-app events", s2aRecordProductPage(ctx, cmd, st, cc, page, lk))

			now := s2aNow()
			rows := make([]eventRow, 0, len(page.Events))
			for _, e := range page.Events {
				rows = append(rows, eventRow{
					EventID:    e.EventID,
					Title:      e.Title,
					Subtitle:   e.Subtitle,
					Kind:       e.Kind,
					StartDate:  e.StartDate,
					EndDate:    e.EndDate,
					Badge:      e.Badge,
					Status:     s2aEventStatus(e.StartDate, e.EndDate, now),
					Detail:     e.Detail,
					Storefront: cc,
				})
			}
			if len(rows) == 0 {
				s2aNote(cmd, flags, fmt.Sprintf("no in-app events are listed on the %s product page for %s", cc, lk.AppID()))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				return nil
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{r.Status, r.Kind, s2aOneLine(r.Title, 50), s2aDay(r.StartDate), s2aDay(r.EndDate), r.EventID})
			}
			return s2aTable(cmd.OutOrStdout(), []string{"status", "kind", "title", "start", "end", "event id"}, table)
		},
	}
	s2bDBFlag(cmd)
	return cmd
}
