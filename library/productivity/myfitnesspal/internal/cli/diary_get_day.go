// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// HAND-WRITTEN — overwrites the generator's emit. The diary endpoint returns
// HTML, not JSON, so the generator's resolveRead() flow doesn't apply. This
// file fetches the page directly and pipes it through internal/parser.

package cli

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/enetx/surf"
	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/myfitnesspal/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/myfitnesspal/internal/parser"
)

// PATCH(local): share the Surf-backed *http.Client across diary fetches via sync.Once.
// Previously fetchAuthenticatedHTML allocated a fresh Surf client (and its TLS
// connection pool) on every call, costing a full TLS handshake per date when
// reading multiple days. The shared client mirrors how internal/client/client.go's
// newHTTPClient builds its client: extract only Surf's Transport (which carries
// the Chrome TLS fingerprint) and wrap it in a fresh *http.Client without a Jar.
// A jar would accumulate Set-Cookie responses from MFP and re-send them alongside
// the manually-set Cookie: <session> header on the next request, producing
// duplicate/conflicting cookies for the rest of the process lifetime.
var (
	diaryHTTPClient     *http.Client
	diaryHTTPClientOnce sync.Once
)

func sharedDiaryHTTPClient() *http.Client {
	diaryHTTPClientOnce.Do(func() {
		surfClient := surf.NewClient().Builder().Impersonate().Chrome().Timeout(30 * time.Second).Build().Unwrap()
		surfStd := surfClient.Std()
		surfTransport := surfStd.Transport
		if surfTransport == nil {
			// PATCH(local): fail loud instead of silently degrading. Mirrors the
			// nil-transport guard in internal/client/client.go's newHTTPClient. A nil
			// transport here means Surf's API changed shape — falling back to
			// http.DefaultTransport would drop the Chrome TLS fingerprint and every
			// diary scrape would hit MFP's anti-bot wall with no visible cause.
			fmt.Fprintln(os.Stderr, "WARNING: Surf transport is nil — Chrome TLS fingerprint unavailable. "+
				"MFP diary requests will likely be rejected by anti-bot. Falling back to stdlib transport; "+
				"check the enetx/surf version.")
			surfTransport = http.DefaultTransport
		}
		diaryHTTPClient = &http.Client{
			Timeout:   30 * time.Second,
			Transport: surfTransport,
		}
	})
	return diaryHTTPClient
}

func newDiaryGetDayCmd(flags *rootFlags) *cobra.Command {
	var flagUsername string
	var flagDate string

	cmd := &cobra.Command{
		Use:   "get-day",
		Short: "Get one day's food diary parsed into structured JSON.",
		Long: `Fetches /food/diary on www.myfitnesspal.com using your imported browser
session, runs the HTML through a parser ported from python-myfitnesspal v2.0.4,
and emits a Day struct: meals (with named entries and per-nutrient values),
totals, goals, and completion state.`,
		Example: "  myfitnesspal-pp-cli diary get-day --date 2024-01-15 --json",
		Annotations: map[string]string{
			"pp:endpoint":   "diary.get_day",
			"pp:method":     "GET",
			"pp:path":       "/food/diary",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagDate == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"date\" not set")
			}
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would GET https://www.myfitnesspal.com/food/diary?date=%s\n", flagDate)
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}

			endpoint, err := buildDiaryURL(flagDate, flagUsername)
			if err != nil {
				return err
			}

			body, err := fetchAuthenticatedHTML(cfg, endpoint)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			day, err := parser.ParseDiary(strings.NewReader(body), flagDate, flagUsername)
			if err != nil {
				return fmt.Errorf("parsing diary: %w", err)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), day, flags)
			}
			return printDiaryHuman(cmd.OutOrStdout(), day)
		},
	}

	cmd.Flags().StringVar(&flagDate, "date", "", "Date to fetch (YYYY-MM-DD). Required.")
	cmd.Flags().StringVar(&flagUsername, "username", "", "Optional username override (defaults to the authenticated account).")
	return cmd
}

func buildDiaryURL(date, username string) (string, error) {
	u, err := url.Parse("https://www.myfitnesspal.com/food/diary")
	if err != nil {
		return "", err
	}
	if username != "" {
		u.Path = "/food/diary/" + username
	}
	q := u.Query()
	q.Set("date", date)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// PATCH(upstream cli-printing-press#787, fix #822): use Surf with Chrome impersonation
// instead of stdlib net/http for the diary scrape. MFP's anti-bot routes plain
// stdlib User-Agent strings to the login redirect; Surf's TLS fingerprint matches
// a real Chrome and clears the challenge. Cookie + Accept headers stay; Surf
// sets User-Agent itself via Impersonate().Chrome().

// fetchAuthenticatedHTML issues a GET with the user's session cookies attached
// via the Cookie header from config.AuthHeader(). Routes through Surf so MFP's
// browser-fingerprint check accepts the request.
func fetchAuthenticatedHTML(cfg *config.Config, target string) (string, error) {
	cookieHeader := cfg.AuthHeader()
	if cookieHeader == "" {
		return "", fmt.Errorf("no MFP session — run `myfitnesspal-pp-cli auth login --chrome`")
	}
	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	// User-Agent set by Surf's Impersonate().Chrome(); do not override here.

	resp, err := sharedDiaryHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", target, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("HTTP %d — session likely expired; run `myfitnesspal-pp-cli auth login --chrome`", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func printDiaryHuman(w io.Writer, d *parser.Diary) error {
	fmt.Fprintf(w, "Diary for %s", d.Date)
	if d.Username != "" {
		fmt.Fprintf(w, " (%s)", d.Username)
	}
	if d.Complete {
		fmt.Fprintln(w, " [complete]")
	} else {
		fmt.Fprintln(w, " [incomplete]")
	}

	for _, meal := range d.Meals {
		fmt.Fprintf(w, "\n## %s\n", meal.Name)
		for _, entry := range meal.Entries {
			cal := entry.Nutrients["calories"]
			fmt.Fprintf(w, "  - %s  (%.0f kcal)\n", entry.Name, cal)
		}
	}

	if len(d.Totals) > 0 {
		fmt.Fprintln(w, "\n## Totals")
		printNutrientLine(w, d.Totals)
	}
	if len(d.Goals) > 0 {
		fmt.Fprintln(w, "\n## Goals")
		printNutrientLine(w, d.Goals)
	}
	if len(d.RawErrors) > 0 {
		fmt.Fprintln(w, "\n## Warnings")
		for _, e := range d.RawErrors {
			fmt.Fprintf(w, "  - %s\n", e)
		}
	}
	return nil
}

func printNutrientLine(w io.Writer, m map[string]float64) {
	for _, k := range []string{"calories", "carbohydrates", "fat", "protein", "sodium", "sugar", "fiber"} {
		if v, ok := m[k]; ok {
			fmt.Fprintf(w, "  %-15s %g\n", k+":", v)
		}
	}
}

// (printJSONFiltered lives in helpers.go — it's the generator-emitted helper
// that routes through printOutputWithFlags so --select/--compact/--csv all work.)
