// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: the safe-mutation UX for event edits. Distinct from the
// generated `calendars events update` (a raw PUT mirror): this one reads the
// pre-image first, writes with an etag precondition, and returns the prior
// state plus a structured inverse operation so every change is reversible.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// undoOp is the structured inverse of the patch that was just applied:
// PATCHing body at path restores the changed fields to their prior values.
type undoOp struct {
	Op   string         `json:"op"`
	Path string         `json:"path"`
	Body map[string]any `json:"body"`
}

type eventsUpdateOutput struct {
	Result   json.RawMessage `json:"result"`
	Prior    json.RawMessage `json:"prior"`
	Undo     undoOp          `json:"undo"`
	EtagUsed string          `json:"etag_used"`
	Blind    bool            `json:"blind"`
}

// eventTimeBody renders a --start/--end flag value as the API's event-time
// object: YYYY-MM-DD becomes an all-day {"date": ...}; RFC3339 becomes
// {"dateTime": ...} with its offset preserved.
func eventTimeBody(flagName, value string) (map[string]any, error) {
	if _, err := time.Parse("2006-01-02", value); err == nil {
		return map[string]any{"date": value}, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return map[string]any{"dateTime": t.Format(time.RFC3339)}, nil
	}
	return nil, usageErr(fmt.Errorf("invalid %s value %q: use RFC3339 (e.g. 2026-08-19T15:00:00-06:00) or YYYY-MM-DD for all-day", flagName, value))
}

// pp:data-source live
func newNovelEventsUpdateCmd(flags *rootFlags) *cobra.Command {
	var flagStart string
	var flagEnd string
	var flagSummary string
	var flagDescription string
	var flagIfEtag string
	var flagForceBlind bool

	cmd := &cobra.Command{
		Use:   "update <calendarId> <eventId>",
		Short: "Etag-preconditioned event edit that returns the pre-image and a ready-to-run inverse patch",
		Long: `Reads the event first (the pre-image), then PATCHes only the fields you
passed, sending If-Match with the pre-image's etag (or --if-etag) so a
mid-flight human edit fails cleanly (HTTP 412) instead of being clobbered.
The response envelope carries:

  result     the event after the write
  prior      the full pre-image
  undo       {op, path, body} — PATCH body to restore the changed fields
  etag_used  the caller-side If-Match value ("" with --force-blind)
  blind      true when --force-blind skipped the caller-side precondition

--force-blind only skips the caller-side If-Match; the structural safety
barrier still applies its own freshest-etag concurrency check, forces
sendUpdates=none, and refuses attendee-bearing events outright.

The target account comes from --account, or is resolved from calendars.yaml
when exactly one account manifests the calendar. Calendars declared
role: read in the manifest are refused.`,
		Example: `  google-calendar-pp-cli events update derik@example.com evt123abc --start 2026-08-19T15:00:00-06:00 --end 2026-08-19T16:00:00-06:00
  google-calendar-pp-cli events update derik@example.com evt123abc --summary "Deep work (moved)" --account personal --json
  google-calendar-pp-cli events update derik@example.com evt123abc --description "agenda attached" --if-etag '"3390000000000000"'`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			calendarID, eventID := args[0], args[1]
			if calendarID == "" || eventID == "" {
				return usageErr(fmt.Errorf("calendarId and eventId are required\nUsage: %s <calendarId> <eventId>", cmd.CommandPath()))
			}
			if flagIfEtag != "" && flagForceBlind {
				return usageErr(fmt.Errorf("--if-etag and --force-blind contradict each other; pass one"))
			}

			body := map[string]any{}
			undoBody := map[string]any{}
			if cmd.Flags().Changed("start") {
				v, err := eventTimeBody("--start", flagStart)
				if err != nil {
					return err
				}
				body["start"] = v
			}
			if cmd.Flags().Changed("end") {
				v, err := eventTimeBody("--end", flagEnd)
				if err != nil {
					return err
				}
				body["end"] = v
			}
			if cmd.Flags().Changed("summary") {
				body["summary"] = flagSummary
			}
			if cmd.Flags().Changed("description") {
				body["description"] = flagDescription
			}
			if len(body) == 0 {
				return usageErr(fmt.Errorf("nothing to update: pass at least one of --start, --end, --summary, --description"))
			}

			account, err := resolveUpdateAccount(flags, calendarID)
			if err != nil {
				return err
			}
			c, err := flags.clientFor(account)
			if err != nil {
				return err
			}
			path := "/calendars/" + url.PathEscape(calendarID) + "/events/" + url.PathEscape(eventID)

			// Pre-image read: always uncached — a stale pre-image would poison
			// both the undo payload and the default etag precondition.
			prior, err := c.GetNoCache(cmd.Context(), path, nil)
			if err != nil {
				return classifyAPIError(fmt.Errorf("pre-image read failed (refusing to write blind): %w", err), flags)
			}
			var priorFields map[string]json.RawMessage
			if err := json.Unmarshal(prior, &priorFields); err != nil {
				return fmt.Errorf("unparseable pre-image for %s: %w", path, err)
			}
			var priorEtag string
			if raw, ok := priorFields["etag"]; ok {
				_ = json.Unmarshal(raw, &priorEtag)
			}
			for field := range body {
				if raw, ok := priorFields[field]; ok {
					var v any
					if json.Unmarshal(raw, &v) == nil {
						undoBody[field] = v
						continue
					}
				}
				// Field absent on the pre-image (e.g. adding a first
				// description): the inverse clears it.
				undoBody[field] = ""
			}

			etagUsed := ""
			headers := map[string]string{}
			switch {
			case flagForceBlind:
				// No caller-side If-Match. The client safety barrier will still
				// attach its own pre-check etag; "blind" here means the caller
				// waived their read-time precondition.
			case flagIfEtag != "":
				etagUsed = flagIfEtag
				headers["If-Match"] = flagIfEtag
			default:
				if priorEtag == "" {
					return fmt.Errorf("pre-image for %s carries no etag; pass --if-etag or --force-blind explicitly", path)
				}
				etagUsed = priorEtag
				headers["If-Match"] = priorEtag
			}

			result, _, err := c.PatchWithParamsAndHeaders(cmd.Context(), path, nil, body, headers)
			if err != nil {
				if errIsThirdPartyBarrier(err) {
					// The structural barrier's refusals are already actionable;
					// surface them verbatim.
					return err
				}
				if strings.Contains(err.Error(), "HTTP 412") {
					return apiErr(fmt.Errorf("%w\nhint: the event changed after your etag was read (precondition failed). Re-read the event and retry, or pass --force-blind to accept the barrier's freshest-etag check instead.", err))
				}
				return classifyAPIError(err, flags)
			}

			out := eventsUpdateOutput{
				Result:   result,
				Prior:    prior,
				Undo:     undoOp{Op: "patch", Path: path, Body: undoBody},
				EtagUsed: etagUsed,
				Blind:    flagForceBlind,
			}
			return emitVerdict(cmd, flags, out, func(w io.Writer) {
				fmt.Fprintf(w, "updated %s\n", path)
				fields := make([]string, 0, len(body))
				for f := range body {
					fields = append(fields, f)
				}
				fmt.Fprintf(w, "fields: %s\n", strings.Join(fields, ", "))
				if out.Blind {
					fmt.Fprintln(w, "precondition: caller-side If-Match WAIVED (--force-blind); safety barrier still applied its own")
				} else {
					fmt.Fprintf(w, "precondition: If-Match %s\n", out.EtagUsed)
				}
				undoJSON, _ := json.Marshal(out.Undo)
				fmt.Fprintf(w, "undo: %s\n", undoJSON)
			})
		},
	}
	cmd.Flags().StringVar(&flagStart, "start", "", "New start: RFC3339 (e.g. 2026-08-19T15:00:00-06:00) or YYYY-MM-DD for all-day")
	cmd.Flags().StringVar(&flagEnd, "end", "", "New end: RFC3339 or YYYY-MM-DD for all-day (Google's all-day end date is exclusive)")
	cmd.Flags().StringVar(&flagSummary, "summary", "", "New event title")
	cmd.Flags().StringVar(&flagDescription, "description", "", "New event description")
	cmd.Flags().StringVar(&flagIfEtag, "if-etag", "", "Explicit If-Match etag precondition (default: the pre-image's etag)")
	cmd.Flags().BoolVar(&flagForceBlind, "force-blind", false, "Skip the caller-side If-Match precondition (logged as blind:true; the safety barrier still enforces its own freshest-etag check)")
	return cmd
}
