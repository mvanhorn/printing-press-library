// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Jane exposes each patient's appointments as an iCalendar feed
// (`/api/v2/appointments.ics`, Accept: text/calendar). This command imports the
// clinic session, fetches the feed, and either prints the subscribe URL/feed or
// writes the .ics to a file — a clean way to sync booked appointments into any
// calendar app.

package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Jane's patient calendar feed is a tokenized URL like
// https://<clinic>.janeapp.com/ical/<token> ("Subscribe To Your Calendar").
// It is session-independent (the token authenticates), so it works in any
// calendar app.
var reICSHref = regexp.MustCompile(`(?i)(https?://[^"'\s]+/ical/[A-Za-z0-9_-]+|webcal://[^"'\s]+|/ical/[A-Za-z0-9_-]+|https?://[^"'\s]+\.ics[^"'\s]*)`)

// authedGet performs a GET against the clinic with the stored session cookie.
func clinicAuthedClient(clinic *Clinic, timeout time.Duration) (*http.Client, string, error) {
	if strings.TrimSpace(clinic.Session) == "" {
		return nil, "", fmt.Errorf("not logged in to clinic %q; run 'janeapp-pp-cli auth login --clinic %s --chrome'", clinic.Name, clinic.Name)
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, "", err
	}
	base := strings.TrimRight(clinic.BaseURL, "/")
	u, err := url.Parse(base)
	if err != nil {
		return nil, "", err
	}
	name, value, _ := strings.Cut(clinic.Session, "=")
	jar.SetCookies(u, []*http.Cookie{{Name: strings.TrimSpace(name), Value: value, Path: "/"}})
	return &http.Client{Timeout: timeout, Jar: jar}, base, nil
}

func newCalendarCmd(flags *rootFlags) *cobra.Command {
	var outFile string
	var showURL bool
	var allClinics bool
	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Get your Jane appointments as an iCalendar (.ics) feed",
		Long: `Fetch your booked appointments as an iCalendar feed for the active
clinic, using your imported session. Print the .ics to stdout, save it with
--out <file>, or print the subscribe URL with --url to add to a calendar app.`,
		Example:     "  janeapp-pp-cli calendar --clinic leahkangas --out ~/Desktop/leah.ics\n  janeapp-pp-cli calendar --clinic leahkangas --url",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}

			if showURL {
				// The subscribe URL is per-clinic, so this path needs one active
				// clinic. (The ICS-export path below works across --all-clinics and
				// must not require a single active/valid clinic session.)
				clinic, err := requireActiveClinic(flags)
				if err != nil {
					return err
				}
				hc, base, err := clinicAuthedClient(clinic, flags.timeout)
				if err != nil {
					return usageErr(err)
				}
				// Jane's native tokenized feed URL — works as a live subscription
				// in a calendar app (which negotiates Jane's cookie gate).
				feedURL := discoverICSSubscribeURL(cmd.Context(), hc, base)
				if feedURL == "" {
					return fmt.Errorf("could not find your calendar feed URL on %s/account (session may be expired — re-run 'auth login --clinic %s --chrome')", base, clinic.Name)
				}
				webcal := strings.Replace(feedURL, "https://", "webcal://", 1)
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"clinic": clinic.Name, "ics_url": feedURL, "webcal_url": webcal}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Live subscribe URL (add in your calendar app):\n  %s\n", webcal)
				fmt.Fprintln(cmd.OutOrStdout(), "Apple Calendar: File → New Calendar Subscription → paste the URL.")
				return nil
			}

			// Build the .ics from your appointments (reliable path). --all-clinics
			// merges every logged-in clinic into one calendar file.
			clinics, err := clinicsForRead(flags, allClinics)
			if err != nil {
				return err
			}
			recs, err := gatherAppointments(cmd, flags, clinics)
			if err != nil {
				return err
			}
			ics := buildICS(recs)
			events := strings.Count(ics, "BEGIN:VEVENT")
			if outFile != "" {
				path := expandHome(outFile)
				if err := os.WriteFile(path, []byte(ics), 0o644); err != nil {
					return fmt.Errorf("writing %s: %w", path, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d appointment(s) to %s\n", events, path)
				fmt.Fprintln(cmd.ErrOrStderr(), "Tip: 'calendar --url' prints Jane's live subscribe URL for auto-syncing.")
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), ics)
			return nil
		},
	}
	cmd.Flags().StringVar(&outFile, "out", "", "Write the .ics to this file instead of stdout")
	cmd.Flags().BoolVar(&showURL, "url", false, "Print Jane's live subscribe URL (for a calendar app) instead of generating a file")
	cmd.Flags().BoolVar(&allClinics, "all-clinics", false, "Include appointments from every logged-in clinic")
	return cmd
}

// buildICS renders appointment records as a VCALENDAR. Times use the UTC form
// Jane returns (already absolute), so no timezone table is needed.
func buildICS(recs []apptRecord) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//janeapp-pp-cli//EN\r\nCALSCALE:GREGORIAN\r\nMETHOD:PUBLISH\r\n")
	for _, r := range recs {
		start, _ := extractTimeFromKeys(r.Raw, apptStartKeys)
		end, _ := extractTimeFromKeys(r.Raw, apptEndKeys)
		if start.IsZero() {
			continue
		}
		if end.IsZero() {
			end = start.Add(time.Hour)
		}
		v := r.view
		summary := str(v["treatment"])
		if p := str(v["practitioner"]); p != "" {
			summary = summary + " with " + p
		}
		if summary == "" {
			summary = "Jane appointment"
		}
		uid := fmt.Sprintf("%v-%s@janeapp-pp-cli", v["id"], r.Clinic)
		b.WriteString("BEGIN:VEVENT\r\n")
		fmt.Fprintf(&b, "UID:%s\r\n", uid)
		fmt.Fprintf(&b, "DTSTAMP:%s\r\n", start.UTC().Format("20060102T150405Z"))
		fmt.Fprintf(&b, "DTSTART:%s\r\n", start.UTC().Format("20060102T150405Z"))
		fmt.Fprintf(&b, "DTEND:%s\r\n", end.UTC().Format("20060102T150405Z"))
		fmt.Fprintf(&b, "SUMMARY:%s\r\n", icsEscape(summary))
		if loc := str(v["location"]); loc != "" {
			fmt.Fprintf(&b, "LOCATION:%s\r\n", icsEscape(loc+", "+r.Clinic))
		}
		if st := str(v["status"]); st == "cancelled" {
			b.WriteString("STATUS:CANCELLED\r\n")
		} else {
			b.WriteString("STATUS:CONFIRMED\r\n")
		}
		b.WriteString("END:VEVENT\r\n")
	}
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

func icsEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// discoverICSSubscribeURL scrapes the account page for a tokenized .ics/webcal
// URL that works without the session cookie. Returns "" if none is found.
func discoverICSSubscribeURL(ctx context.Context, hc *http.Client, base string) string {
	for _, p := range []string{"/account", "/", "/account/settings"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+p, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", janeUserAgent)
		resp, err := hc.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if os.Getenv("JANEAPP_DEBUG_KEYS") != "" {
			s := string(body)
			fmt.Fprintf(os.Stderr, "[cal-debug] %s status=%d len=%d\n", p, resp.StatusCode, len(s))
			for _, kw := range []string{"ics", "ical", "webcal", "calendar", "subscribe", "feed_token", "sync"} {
				if i := strings.Index(strings.ToLower(s), kw); i >= 0 {
					lo := i - 40
					if lo < 0 {
						lo = 0
					}
					hi := i + 60
					if hi > len(s) {
						hi = len(s)
					}
					fmt.Fprintf(os.Stderr, "  %s: ...%s...\n", kw, strings.ReplaceAll(s[lo:hi], "\n", " "))
				}
			}
		}
		for _, m := range reICSHref.FindAllString(string(body), -1) {
			if strings.Contains(m, "/ical/") || strings.Contains(m, "webcal") || strings.HasSuffix(m, ".ics") {
				m = strings.Replace(m, "webcal://", "https://", 1)
				if strings.HasPrefix(m, "/") {
					return base + m
				}
				return m
			}
		}
	}
	return ""
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return home + p[1:]
		}
	}
	return p
}
