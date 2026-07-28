// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/zoomurl"
)

// newZoomNextCmd: T15. Resolves the soonest upcoming cloud meeting and turns it
// into a one-keystroke join. Printing the zoommtg:// URL is the default because
// launching the desktop app is a visible side effect an agent should not take
// without being asked; --launch opts in.
func newZoomNextCmd(flags *rootFlags) *cobra.Command {
	var (
		launch bool
		userID string
		uname  string
		within string
	)
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Resolve the soonest upcoming meeting and print its desktop join URL (--launch to open it)",
		Long: "GETs /users/{userId}/meetings?type=upcoming (userId defaults to 'me'), picks the meeting with the " +
			"earliest start_time still in the future, and builds the canonical zoommtg:// URL for it. Prints by " +
			"default; pass --launch to hand the URL to the platform opener. --within 4h narrows the window so an " +
			"agent can distinguish \"nothing imminent\" from \"nothing scheduled\".",
		Example: strings.Trim(`
  zoom-pp-cli next --json
  zoom-pp-cli next --within 2h --json
  zoom-pp-cli next --launch --name "Maya"
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if userID == "" {
				userID = "me"
			}
			var window time.Duration
			if within != "" {
				d, err := time.ParseDuration(within)
				if err != nil || d <= 0 {
					return usageErr(fmt.Errorf("next: --within must be a positive Go duration (e.g. 90m, 4h), got %q", within))
				}
				window = d
			}

			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{
					"would_read": "GET /users/" + userID + "/meetings?type=upcoming",
					"would_open": launch,
					"within":     within,
				})
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(ctx, replacePathParam("/users/{userId}/meetings", "userId", userID), map[string]string{
				"type":      "upcoming",
				"page_size": "300",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			meetings := extractNextMeetings(data)
			now := time.Now()
			var picked *upcomingMeeting
			sort.Slice(meetings, func(i, j int) bool { return meetings[i].Start.Before(meetings[j].Start) })
			for i := range meetings {
				m := meetings[i]
				if m.Start.IsZero() || m.Start.Before(now) {
					continue
				}
				if window > 0 && m.Start.After(now.Add(window)) {
					break
				}
				picked = &m
				break
			}
			if picked == nil {
				return flags.printJSON(cmd, map[string]any{
					"status":    "none",
					"within":    within,
					"candidate": nil,
					"hint":      "no upcoming meeting found in the window; widen --within or check `zoom-pp-cli today`",
				})
			}

			p := zoomurl.Params{Action: zoomurl.ActionJoin, ConfNo: picked.ID, Pwd: picked.Password}
			if uname != "" {
				p.Uname = uname
			}
			joinURL, err := zoomurl.Build(p)
			if err != nil {
				return err
			}

			payload := map[string]any{
				"url":          joinURL,
				"meeting_id":   picked.ID,
				"topic":        picked.Topic,
				"start_time":   picked.Start,
				"starts_in":    picked.Start.Sub(now).Round(time.Minute).String(),
				"web_url":      picked.JoinURL,
				"platform":     runtime.GOOS,
				"has_pwd":      picked.Password != "",
				"duration_min": picked.Duration,
			}

			// The verifier's mock subprocesses must never dial an OS handler.
			// This env check is the floor beneath the --launch opt-in.
			if !launch || cliutil.IsVerifyEnv() {
				payload["status"] = "would_launch"
				if !flags.asJSON {
					fmt.Fprintf(cmd.OutOrStdout(), "next: %s at %s (in %s)\nwould launch: %s\n",
						picked.Topic, picked.Start.Local().Format(time.RFC1123), payload["starts_in"], joinURL)
					return nil
				}
				return flags.printJSON(cmd, payload)
			}

			if err := openURL(ctx, joinURL); err != nil {
				return fmt.Errorf("launching Zoom: %w", err)
			}
			payload["status"] = "launched"
			return flags.printJSON(cmd, payload)
		},
	}
	cmd.Flags().BoolVar(&launch, "launch", false, "Actually hand the zoommtg:// URL to the desktop app (default: print only)")
	cmd.Flags().StringVar(&userID, "user", "", "Zoom user ID or email whose calendar to read (default: me)")
	cmd.Flags().StringVar(&uname, "name", "", "Display name to join with")
	cmd.Flags().StringVar(&within, "within", "", "Only consider meetings starting within this Go duration (e.g. 90m, 4h)")
	return cmd
}

type upcomingMeeting struct {
	ID       string
	Topic    string
	Password string
	JoinURL  string
	Duration int
	Start    time.Time
}

// extractNextMeetings tolerates both the bare array the paginated read helper
// unwraps to and the raw {"meetings":[...]} envelope Zoom returns, so `next`
// keeps working whichever shape reaches it.
func extractNextMeetings(data json.RawMessage) []upcomingMeeting {
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		var env struct {
			Meetings []map[string]any `json:"meetings"`
		}
		if json.Unmarshal(data, &env) != nil {
			return nil
		}
		items = env.Meetings
	}
	out := make([]upcomingMeeting, 0, len(items))
	for _, it := range items {
		m := upcomingMeeting{
			ID:       numericField(it, "id"),
			Topic:    stringField(it, "topic"),
			Password: stringField(it, "password"),
			JoinURL:  stringField(it, "join_url"),
		}
		if d, err := strconv.Atoi(strings.TrimSpace(numericField(it, "duration"))); err == nil {
			m.Duration = d
		}
		if s := stringField(it, "start_time"); s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				m.Start = t
			}
		}
		if m.ID == "" {
			continue
		}
		out = append(out, m)
	}
	return out
}
