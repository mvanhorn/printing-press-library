// Copyright 2026 Olivier and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: leave-by.
//
// Plans backwards from a fixed arrival deadline, applies live delay, and keeps
// a safety margin. The API exposes timesel=arrival but no tool frames it as an
// answer to "what is the last train I can take".

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type leaveByOption struct {
	DepartsAt       string `json:"departs_at"`
	ArrivesAt       string `json:"arrives_at"`
	ScheduledArrive string `json:"scheduled_arrival"`
	DurationSec     int    `json:"duration_seconds"`
	Transfers       int    `json:"transfers"`
	DepartDelaySec  int    `json:"departure_delay_seconds"`
	ArriveDelaySec  int    `json:"arrival_delay_seconds"`
	Platform        string `json:"platform,omitempty"`
	Vehicle         string `json:"vehicle,omitempty"`
	SlackSec        int    `json:"slack_seconds"`
	Canceled        bool   `json:"canceled"`
	Viable          bool   `json:"viable"`
	Reason          string `json:"reason,omitempty"`
}

type leaveByView struct {
	From        string          `json:"from"`
	To          string          `json:"to"`
	ArriveBy    string          `json:"arrive_by"`
	MarginSec   int             `json:"margin_seconds"`
	Recommended *leaveByOption  `json:"recommended"`
	Options     []leaveByOption `json:"options"`
	Note        string          `json:"note,omitempty"`
}

func newNovelLeaveByCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string
	var flagArriveBy string
	var flagDate string
	var flagMargin int
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "leave-by",
		Short: "Latest departure that still arrives before a deadline, allowing for delays",
		Long: "Plans backwards from a fixed arrival time, applies each option's live delay,\n" +
			"and keeps a safety margin, then names the latest departure that still works.\n\n" +
			"Use this command when the arrival deadline is fixed. Do NOT use it for\n" +
			"open-ended browsing; use 'route' for that.",
		Example: `  irail-pp-cli leave-by --from Leuven --to Brussels-Central --arrive-by 09:00
  irail-pp-cli leave-by --from Bruges --to Ghent-Sint-Pieters --arrive-by 17:30 --margin 300 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would find the latest departure meeting the arrival deadline")
				return nil
			}
			if flagFrom == "" || flagTo == "" || flagArriveBy == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from, --to and --arrive-by are all required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			now := nowInBelgium()
			from := resolveStationName(flagFrom)
			to := resolveStationName(flagTo)

			date, hhmm, rolled, err := resolveWhen(flagDate, flagArriveBy, now)
			if err != nil {
				return usageErr(err)
			}
			if hhmm == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--arrive-by must be a time such as 09:00"))
			}
			deadline, err := deadlineInstant(date, hhmm, now)
			if err != nil {
				return usageErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{
				"from":    from,
				"to":      to,
				"timesel": "arrival", // plan backwards from the arrival time
				"time":    hhmm,
				"lang":    "en",
			}
			if date != "" {
				params["date"] = date
			}
			env, err := irailFetch(ctx, c, "/v1/connections?format=json", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			view := leaveByView{
				From:      from,
				To:        to,
				ArriveBy:  deadline.Format(time.RFC3339),
				MarginSec: flagMargin,
				Options:   make([]leaveByOption, 0),
			}

			for _, raw := range sliceAt(env, "connection") {
				conn, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				opt := leaveByOptionFrom(conn, deadline, flagMargin)
				view.Options = append(view.Options, opt)
			}

			// Latest viable departure first: that is the answer to the question.
			sort.Slice(view.Options, func(i, j int) bool {
				return view.Options[i].DepartsAt > view.Options[j].DepartsAt
			})
			for i := range view.Options {
				if view.Options[i].Viable {
					opt := view.Options[i]
					view.Recommended = &opt
					break
				}
			}
			if flagLimit > 0 && len(view.Options) > flagLimit {
				view.Options = view.Options[:flagLimit]
			}

			switch {
			case len(view.Options) == 0:
				view.Note = "iRail returned no connections for this journey; check the station names or the date"
			case view.Recommended == nil:
				view.Note = fmt.Sprintf(
					"no option arrives by %s with a %s margin once current delays are applied; leave earlier or relax --margin",
					deadline.Format("15:04"), humanDuration(flagMargin))
			}
			if rolled {
				view.Note = joinNotes(view.Note,
					"that time had already passed today, so this is tomorrow's deadline")
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if view.Recommended != nil {
				r := view.Recommended
				fmt.Fprintf(cmd.OutOrStdout(),
					"Leave at %s (%s), arriving %s with %s to spare\n",
					clockOf(r.DepartsAt), r.Vehicle, clockOf(r.ArrivesAt), humanDuration(r.SlackSec))
			}
			for _, o := range view.Options {
				mark := "  "
				if !o.Viable {
					mark = "x "
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s -> %s  %-8s %d transfer(s)  %s\n",
					mark, clockOf(o.DepartsAt), clockOf(o.ArrivesAt),
					humanDuration(o.DurationSec), o.Transfers, o.Reason)
			}
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", view.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagFrom, "from", "", "Origin station (name, telegraph code or id)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Destination station (name, telegraph code or id)")
	cmd.Flags().StringVar(&flagArriveBy, "arrive-by", "", "Arrival deadline: 09:00, 0900 or +2h")
	cmd.Flags().StringVar(&flagDate, "date", "", "Date: tomorrow, monday, 2026-07-25, +2d or ddmmyy")
	cmd.Flags().IntVar(&flagMargin, "margin", 0, "Seconds of slack required before the deadline")
	cmd.Flags().IntVar(&flagLimit, "limit", 8, "Maximum options to list")
	return cmd
}

// deadlineInstant turns the resolved wire date/time into a concrete instant.
func deadlineInstant(date, hhmm string, now time.Time) (time.Time, error) {
	layout, value := "1504", hhmm
	if date != "" {
		layout, value = "0201061504", date+hhmm
	}
	t, err := time.ParseInLocation(layout, value, now.Location())
	if err != nil {
		return time.Time{}, fmt.Errorf("could not read the arrival deadline %q: %w", hhmm, err)
	}
	if date == "" {
		t = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	}
	return t, nil
}

// leaveByOptionFrom evaluates one itinerary against the deadline.
func leaveByOptionFrom(conn map[string]any, deadline time.Time, marginSec int) leaveByOption {
	dep := mapAt(conn, "departure")
	arr := mapAt(conn, "arrival")

	opt := leaveByOption{
		DurationSec: irailInt(conn["duration"]),
		Transfers:   len(sliceAt(conn, "vias", "via")),
	}
	if dep != nil {
		opt.DepartsAt = unixToLocal(dep["time"])
		opt.DepartDelaySec = irailInt(dep["delay"])
		opt.Platform = irailString(dep["platform"])
		opt.Vehicle = irailString(dep["vehicle"])
		if vi := mapAt(dep, "vehicleinfo"); vi != nil {
			if short := irailString(vi["shortname"]); short != "" {
				opt.Vehicle = short
			}
		}
		opt.Canceled = irailBool(dep["canceled"])
	}
	if arr == nil {
		opt.Reason = "iRail reported no arrival for this option"
		return opt
	}

	scheduled := irailInt64(arr["time"])
	opt.ArriveDelaySec = irailInt(arr["delay"])
	opt.ScheduledArrive = unixToLocal(arr["time"])
	if irailBool(arr["canceled"]) {
		opt.Canceled = true
	}

	// The real arrival is the scheduled arrival plus the live delay.
	actual := time.Unix(scheduled+int64(opt.ArriveDelaySec), 0).In(belgiumTZ())
	opt.ArrivesAt = actual.Format(time.RFC3339)
	opt.SlackSec = int(deadline.Sub(actual).Seconds())

	switch {
	case opt.Canceled:
		opt.Viable = false
		opt.Reason = "cancelled"
	case opt.SlackSec < marginSec:
		opt.Viable = false
		if opt.SlackSec < 0 {
			opt.Reason = fmt.Sprintf("arrives %s late", humanDuration(-opt.SlackSec))
		} else {
			opt.Reason = fmt.Sprintf("only %s spare, below the %s margin",
				humanDuration(opt.SlackSec), humanDuration(marginSec))
		}
	default:
		opt.Viable = true
		opt.Reason = fmt.Sprintf("%s to spare", humanDuration(opt.SlackSec))
	}
	return opt
}
