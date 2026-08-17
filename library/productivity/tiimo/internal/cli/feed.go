// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Reads the local mirror only. Run `tiimo-pp-cli sync` to refresh it.

package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

// feedResult is the machine-readable summary of a generated feed.
type feedResult struct {
	Path        string `json:"path,omitempty"`
	Format      string `json:"format"`
	Events      int    `json:"events"`
	From        string `json:"from"`
	To          string `json:"to"`
	Skipped     int    `json:"skipped_without_times"`
	WroteStdout bool   `json:"wrote_stdout"`
	// Document carries the generated calendar text when the caller asked for
	// stdout in a machine output mode, so the response stays valid JSON.
	Document string `json:"document,omitempty"`
}

func newNovelFeedCmd(flags *rootFlags) *cobra.Command {
	var flagOut, flagDays, flagDB, flagFormat string
	var flagIncludeExternal bool

	cmd := &cobra.Command{
		Use:   "feed",
		Short: "Generate a subscribable read-only iCalendar file of your Tiimo activities.",
		Long: `Write your Tiimo plan out as a calendar file.

Tiimo imports from your calendars but refuses to publish back to them. Users
asked for exactly this as a compromise -- a read-only calendar view of Tiimo
activities -- and were told it would not be built. This is that file.

Point --out at a location your calendar client can subscribe to, or pass "-"
to write to stdout and pipe it somewhere.

Imported external-calendar events are excluded by default: they already exist
in the calendar you would be subscribing from, and re-publishing them creates
duplicates.`,
		Example: "  tiimo-pp-cli feed --out ~/tiimo.ics --days 30",
		Annotations: map[string]string{
			"mcp:read-only":         "true",
			"mcp:write-positionals": "",
			"pp:happy-args":         "--out=-;--days=7",
			"pp:typed-exit-codes":   "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "feed")
			}

			format := strings.ToLower(strings.TrimSpace(flagFormat))
			if format == "" {
				format = "ics"
			}
			switch format {
			case "ics", "csv":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--format must be ics or csv"))
			}

			days := 30
			if strings.TrimSpace(flagDays) != "" {
				n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(flagDays), "d"))
				if err != nil || n <= 0 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --days %q: want a positive whole number of days", flagDays))
				}
				days = n
			}

			from, _, err := dateWindow("", "", "")
			if err != nil {
				return err
			}
			to := from.AddDate(0, 0, days).Add(-1)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			st, ok, err := openLocalMirror(ctx, cmd, flags, flagDB)
			if err != nil {
				return err
			}
			if !ok {
				return writeNoMirror(cmd, flags, flagDB, make([]feedResult, 0))
			}
			defer st.Close()

			acts, err := loadActivities(ctx, st.DB(), from, to)
			if err != nil {
				return err
			}

			// Filesystem work happens only after the dry-run short-circuit
			// above, so a --dry-run probe never touches disk.
			toStdout := flagOut == "" || flagOut == "-"
			// Writing the raw calendar document to stdout is right for a
			// human piping it to a file, but it is not JSON. When a machine
			// output mode is active, buffer the document and return it inside
			// the envelope instead so stdout stays parseable.
			machineMode := !wantsHumanTable(cmd.OutOrStdout(), flags)
			var docBuf bytes.Buffer
			var out io.Writer = cmd.OutOrStdout()
			if toStdout && machineMode {
				out = &docBuf
			}
			var closer func() error
			resolvedPath := ""
			if !toStdout {
				resolvedPath, err = expandHome(flagOut)
				if err != nil {
					return err
				}
				if dir := filepath.Dir(resolvedPath); dir != "" && dir != "." {
					// 0700: the calendar document below is written 0600 because
					// it is personal schedule data; a world-readable parent
					// directory would leak the filenames around it.
					if err := os.MkdirAll(dir, 0o700); err != nil {
						return fmt.Errorf("creating %s: %w", dir, err)
					}
				}
				f, err := os.OpenFile(resolvedPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
				if err != nil {
					return fmt.Errorf("writing %s: %w", resolvedPath, err)
				}
				out = f
				closer = f.Close
			}

			var events, skipped int
			switch format {
			case "ics":
				events, skipped, err = writeICS(out, acts, flagIncludeExternal)
			case "csv":
				events, skipped, err = writeFeedCSV(out, acts, flagIncludeExternal)
			}
			if closer != nil {
				if cerr := closer(); cerr != nil && err == nil {
					err = fmt.Errorf("closing %s: %w", resolvedPath, cerr)
				}
			}
			if err != nil {
				return err
			}

			res := feedResult{
				Path:        resolvedPath,
				Format:      format,
				Events:      events,
				From:        from.Format(tiimoDateLayout),
				To:          to.Format(tiimoDateLayout),
				Skipped:     skipped,
				WroteStdout: toStdout,
			}
			// An empty feed is a valid document and exits 0, which makes it
			// indistinguishable from a genuinely clear window. Say so on
			// stderr in every render mode rather than only in the human
			// summary, which --json callers never see.
			if events == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: no activities between %s and %s, so the feed has 0 events. Run 'tiimo-pp-cli sync' if the local mirror is stale.\n", res.From, res.To)
			}
			if toStdout {
				if machineMode {
					res.Document = docBuf.String()
					return printJSONFiltered(cmd.OutOrStdout(), []feedResult{res}, flags)
				}
				// The document itself already went to stdout; a summary there
				// too would corrupt it. Report on stderr instead.
				fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d event(s) to stdout (%s)\n", events, format)
				return nil
			}
			return writeTiimoResult(cmd, flags, []feedResult{res}, func(w io.Writer) {
				fmt.Fprintf(w, "Wrote %d event(s) to %s\n", events, resolvedPath)
				fmt.Fprintf(w, "Covering %s to %s\n", res.From, res.To)
				if skipped > 0 {
					fmt.Fprintf(w, "Skipped %d activity/activities with no usable start time.\n", skipped)
				}
				if format == "ics" {
					fmt.Fprintln(w, "\nSubscribe to this file from your calendar client to see your Tiimo plan alongside your other calendars.")
				}
			})
		},
	}

	cmd.Flags().StringVar(&flagOut, "out", "", `Destination file, or "-" for stdout`)
	cmd.Flags().StringVar(&flagDays, "days", "30", "How many days forward to include")
	cmd.Flags().StringVar(&flagFormat, "format", "ics", "Output format: ics or csv")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror (defaults to the standard cache location)")
	cmd.Flags().BoolVar(&flagIncludeExternal, "include-external", false, "Include events imported from linked external calendars")
	return cmd
}

// writeICS emits RFC 5545 VEVENTs. Times are written as floating local time
// (no Z suffix, no TZID) because that is exactly what Tiimo stores: a naive
// wall-clock time the user planned against. Converting to UTC would silently
// shift every event for anyone who later crosses a timezone.
func writeICS(w io.Writer, acts []activityRow, includeExternal bool) (int, int, error) {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//printing-press//tiimo-pp-cli//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")
	b.WriteString("X-WR-CALNAME:Tiimo\r\n")

	stamp := time.Now().UTC().Format("20060102T150405Z")
	events, skipped := 0, 0
	for _, a := range acts {
		if a.IsReadOnly && !includeExternal {
			continue
		}
		start, okS := a.Start()
		if !okS {
			skipped++
			continue
		}
		end, okE := a.End()
		if !okE {
			end = start.Add(30 * time.Minute)
		}

		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString("UID:" + icsUID(a) + "\r\n")
		b.WriteString("DTSTAMP:" + stamp + "\r\n")
		if a.IsAllDay {
			b.WriteString("DTSTART;VALUE=DATE:" + start.Format("20060102") + "\r\n")
			b.WriteString("DTEND;VALUE=DATE:" + start.AddDate(0, 0, 1).Format("20060102") + "\r\n")
		} else {
			b.WriteString("DTSTART:" + start.Format("20060102T150405") + "\r\n")
			b.WriteString("DTEND:" + end.Format("20060102T150405") + "\r\n")
		}
		summary := a.Title
		if a.IconID != "" {
			summary = a.IconID + " " + summary
		}
		b.WriteString(foldICSLine("SUMMARY:" + escapeICSText(summary)))
		if a.Completed() {
			b.WriteString("STATUS:CONFIRMED\r\n")
			b.WriteString(foldICSLine("DESCRIPTION:" + escapeICSText("Completed in Tiimo")))
		}
		if len(a.Checklist) > 0 {
			steps := make([]string, 0, len(a.Checklist))
			for _, s := range a.Checklist {
				mark := "[ ]"
				if s.IsChecked {
					mark = "[x]"
				}
				steps = append(steps, mark+" "+s.Title)
			}
			b.WriteString(foldICSLine("DESCRIPTION:" + escapeICSText(strings.Join(steps, "\n"))))
		}
		b.WriteString("TRANSP:OPAQUE\r\n")
		b.WriteString("END:VEVENT\r\n")
		events++
	}
	b.WriteString("END:VCALENDAR\r\n")

	if _, err := io.WriteString(w, b.String()); err != nil {
		return 0, 0, fmt.Errorf("writing calendar: %w", err)
	}
	return events, skipped, nil
}

// writeFeedCSV emits a flat spreadsheet-friendly view.
func writeFeedCSV(w io.Writer, acts []activityRow, includeExternal bool) (int, int, error) {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{
		"date", "start", "end", "duration_seconds", "title", "bucket",
		"completed", "all_day", "repeating", "recurrence", "external", "activity_id",
	}); err != nil {
		return 0, 0, fmt.Errorf("writing csv header: %w", err)
	}
	events, skipped := 0, 0
	for _, a := range acts {
		if a.IsReadOnly && !includeExternal {
			continue
		}
		start, okS := a.Start()
		if !okS {
			skipped++
			continue
		}
		endStr := ""
		if end, ok := a.End(); ok {
			endStr = end.Format("15:04")
		}
		if err := cw.Write([]string{
			a.Day(), start.Format("15:04"), endStr, strconv.Itoa(a.Duration),
			a.Title, a.Bucket(), strconv.FormatBool(a.Completed()),
			strconv.FormatBool(a.IsAllDay), strconv.FormatBool(a.IsRepeating),
			a.RecurrenceType, strconv.FormatBool(a.IsReadOnly), a.ActivityID,
		}); err != nil {
			return 0, 0, fmt.Errorf("writing csv row: %w", err)
		}
		events++
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return 0, 0, fmt.Errorf("flushing csv: %w", err)
	}
	return events, skipped, nil
}

// icsUID produces a stable per-event UID so re-generating the feed updates
// events in a subscribed client instead of duplicating them.
func icsUID(a activityRow) string {
	seed := a.ActivityID
	if seed == "" {
		seed = a.Title + "|" + a.StartTime
	}
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:16]) + "@tiimo-pp-cli"
}

// escapeICSText applies RFC 5545 text escaping.
func escapeICSText(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		";", `\;`,
		",", `\,`,
		"\r\n", `\n`,
		"\n", `\n`,
	)
	return r.Replace(s)
}

// foldICSLine wraps a content line at 75 octets per RFC 5545. Unfolded long
// SUMMARY lines are the most common reason a generated .ics is rejected.
func foldICSLine(line string) string {
	const limit = 73
	if len(line) <= limit {
		return line + "\r\n"
	}
	// The limit is counted in BYTES, but a split may only land on a rune
	// boundary: RFC 5545 folds an octet sequence, and cutting mid-rune yields
	// invalid UTF-8 that clients reject or render as mojibake. That is the
	// normal case here, not an exotic one -- Tiimo activities carry
	// `iconType: UnicodeEmoji`, so a 4-byte rune in a SUMMARY is routine.
	//
	// takeRunes fills up to max bytes without ever cutting a rune, and always
	// returns at least one rune so a single rune wider than the budget still
	// makes progress instead of looping forever.
	takeRunes := func(s string, max int) (head, tail string) {
		n := 0
		for i, r := range s {
			w := utf8.RuneLen(r)
			if n+w > max {
				if i == 0 {
					return s[:w], s[w:]
				}
				return s[:i], s[i:]
			}
			n += w
		}
		return s, ""
	}
	var b strings.Builder
	head, rest := takeRunes(line, limit)
	b.WriteString(head)
	b.WriteString("\r\n")
	for rest != "" {
		head, rest = takeRunes(rest, limit-1)
		b.WriteString(" ")
		b.WriteString(head)
		b.WriteString("\r\n")
	}
	return b.String()
}

// expandHome resolves a leading ~ so --out ~/tiimo.ics behaves the way the
// help text implies even when the shell did not expand it.
func expandHome(p string) (string, error) {
	p = strings.TrimSpace(p)
	if !strings.HasPrefix(p, "~") {
		return filepath.Clean(p), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory for %q: %w", p, err)
	}
	return filepath.Clean(filepath.Join(home, strings.TrimPrefix(p, "~"))), nil
}
