// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
//
// CONFIRMED 2026-08-17. Hotel search has a fundamentally different
// architecture than flights_search.go: the GraphQL mutation that creates a
// hotel shopping session (travel.hotel.startShoppingSession) is confirmed
// BLOCKED from scripted replay -- proven by replaying the exact byte-for-byte
// body of a request that had just succeeded natively in the browser, and
// still getting an "UNCLASSIFIED" server error. A fresh concur-correlationid
// header did not help either (rules out simple mutation-replay/idempotency
// dedup). Reads on an EXISTING session (shoppingSession(id:), properties(),
// trips.ruleClasses for policy lookup) are confirmed to replay perfectly
// fine via direct HTTP -- this is specifically a write-path restriction
// (most likely Akamai bot-mitigation gating mutations more strictly than
// queries; this tenant has ak_bmsc/_abck per the Aug 12 discovery report),
// not a broader API-replay block.
//
// So this command shells out to the already-installed `agent-browser` CLI
// for exactly one step -- driving the real Hotel Search form to completion
// so the mutation happens as a genuine browser-native request -- then
// switches to this CLI's normal direct-HTTP GraphQL client for everything
// else (results, output). User-approved before this was built, given the
// architecture implications: this command requires `agent-browser` installed
// and a live, already-logged-in Chrome session (same requirement as
// `auth login --chrome`), is markedly slower than flights search (real page
// navigation, not just HTTP calls), and is more fragile (depends on the
// Hotel Search form's structure/labels not changing).
//
// Extracting the resulting session id took three attempts to get right,
// in increasing order of cleverness, before landing on the simplest option:
//  1. Parse agent-browser's captured network response body for the
//     mutation. Rejected: the server confirmed a real 630-byte response
//     (content-length header) but agent-browser's own capture returned an
//     empty body for this specific request, reproducibly, even with HAR
//     recording active and with retries.
//  2. Inject a window.fetch override into the page before clicking Search,
//     to intercept the response in-page. Rejected: clicking "Search Hotels"
//     triggers a REAL navigation (confirmed via
//     performance.getEntriesByType('navigation')[0].type === 0, i.e.
//     TYPE_NAVIGATE, not an SPA soft-transition), which resets all page JS
//     state -- including the injected override -- before the async fetch
//     promise the override depends on can resolve.
//  3. That same real navigation lands on
//     https://us2.concursolutions.com/travel/hotel/shop/{sessionId} -- the
//     session id is sitting in the URL the entire time. Confirmed live:
//     reading the URL after the search and calling shoppingSession(id:)
//     with the extracted UUID returns real properties (verified against
//     actual hotel names/prices). This is what's implemented below.
//
// See .printing-press-patches/flights-hotels-search.json for the full
// investigation.
//
// UPDATED 2026-08-18. This command required its own fully independent login
// from `auth login --chrome`, because agent-browser drives a separate
// bundled "Chrome for Testing" binary in an isolated user-data-dir
// (confirmed via `ps aux` -- a different executable from
// /Applications/Google Chrome.app entirely). Two fixes were tried and
// disproven live before landing on the one that works:
//
//  1. Point agent-browser at the real "Default" Chrome profile
//     (--profile Default). Confirmed live that this silently falls back to
//     a fresh temp profile -- no error surfaced -- because the user's real
//     Chrome already holds that profile's lock.
//  2. Seed agent-browser's isolated session with the JWT/_csrf cookies
//     `auth login --chrome` already extracted. The cookies *set*
//     successfully, but Concur's server cleared the session on the next
//     navigation and redirected to sign-in. Re-tested copying every cookie
//     including the Akamai bot-sensor ones (_abck, ak_bmsc, bm_sv, bm_sz):
//     still rejected. This points to a device/TLS-fingerprint-level binding
//     that cookie values alone cannot cross -- consistent with this file's
//     already-documented finding that Concur's bot-mitigation gates
//     mutations more strictly than reads.
//
// What actually works: detectDedicatedConcurBrowser + the activeCDPPort
// plumbing below CDP-*attaches* to an already-running, already-logged-in
// Chrome instance instead of copying anything. Confirmed live: a Chrome
// instance launched with --remote-debugging-port, logged into Concur once,
// stays authenticated when agent-browser attaches via `--cdp <port>` --
// `get url` returned the real /home page and a snapshot showed zero
// "sign in" refs. Attaching is not copying: it's the same browser
// connection, so there is no separate fingerprint to mismatch. This is
// opt-in and falls back to the pre-existing isolated-launch behavior (its
// own separate login) when no dedicated debug-enabled Chrome is found, so
// nothing breaks for anyone who has not set one up. See
// .printing-press-patches/unify-browser-auth-sessions.json for the full
// investigation, including why a plain custom --user-data-dir profile is
// NOT recommended for the one-time setup (auth.go's Chrome-profile
// discovery only searches the standard OS profile locations, so a
// same-standard-location named profile, created via Chrome's own
// "Add Person" UI and launched with --profile-directory, is what lets
// `auth login --chrome --profile "<name>"` and this command's CDP-attach
// draw from the exact same session).
//
// This is a hand-authored file (no "Generated by ..." header) so
// `generate --force` preserves it.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/concur/internal/client"

	"github.com/spf13/cobra"
)

const hotelHomeURL = "https://us2.concursolutions.com/home"

// concurCDPPortCandidates lists the ports probed, in order, for an
// already-running Chrome instance with remote debugging enabled before
// falling back to agent-browser's isolated-profile launch (which requires
// its own separate login every time its session expires). 9222 is
// Chromium's conventional debug port; 9333 avoids collisions with other
// local tools that default to 9222. Override with CONCUR_CDP_PORT for a
// custom setup instead of probing this list.
var concurCDPPortCandidates = []string{"9222", "9333", "9229"}

// activeCDPPort, when non-empty, routes every agent-browser invocation in
// this file through that CDP debug port (an attach to an already-running,
// already-authenticated Chrome instance) instead of agent-browser's default
// isolated-profile launch. Package-level rather than threaded through every
// call site because this command is a single, non-concurrent execution path
// per process invocation -- see detectDedicatedConcurBrowser and this
// file's header comment for why attaching (not copying credentials) is
// what makes session reuse actually work here.
var activeCDPPort string

// detectDedicatedConcurBrowser looks for a Chrome instance already running
// with remote debugging enabled and an authenticated Concur session, so
// this command can attach to it instead of launching an isolated automation
// profile that needs its own login. Returns the port to use, or "" if none
// of the candidates has one. Failure to reach any given port is expected
// (most users have not set one up) and is not reported as an error --
// callers fall back to the pre-existing behavior silently.
func detectDedicatedConcurBrowser() string {
	candidates := concurCDPPortCandidates
	if p := strings.TrimSpace(os.Getenv("CONCUR_CDP_PORT")); p != "" {
		candidates = []string{p}
	}
	for _, port := range candidates {
		// #nosec G204 G702 -- exec.Command passes args as a literal argv
		// array with no shell involved, so shell-metacharacter injection is
		// not possible here. port comes from a hardcoded candidate list or
		// CONCUR_CDP_PORT (an env var the local operator controls on their
		// own machine); there is no remote/untrusted input and no privilege
		// boundary crossed by passing it to a locally-installed helper.
		out, err := exec.Command("agent-browser", "--cdp", port, "get", "url", "--json").Output()
		if err != nil {
			continue
		}
		var env struct {
			Success bool `json:"success"`
			Data    struct {
				URL string `json:"url"`
			} `json:"data"`
		}
		if err := json.Unmarshal(out, &env); err != nil || !env.Success {
			continue
		}
		if strings.Contains(env.Data.URL, "concursolutions.com") && !strings.Contains(env.Data.URL, "/nui/signin") {
			return port
		}
	}
	return ""
}

var hotelShopURLRE = regexp.MustCompile(`/travel/hotel/shop/([0-9a-fA-F-]{36})`)

type agentBrowserRefEntry struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type agentBrowserSnapshotEnvelope struct {
	Success bool `json:"success"`
	Data    struct {
		Origin string                          `json:"origin"`
		Refs   map[string]agentBrowserRefEntry `json:"refs"`
	} `json:"data"`
}

// runAgentBrowser shells out to the agent-browser CLI, which must already be
// installed and on PATH. The browser session it drives persists across
// calls via agent-browser's own daemon, matching how it was used
// interactively during discovery.
func runAgentBrowser(args ...string) ([]byte, error) {
	if _, err := exec.LookPath("agent-browser"); err != nil {
		return nil, fmt.Errorf("agent-browser not found on PATH -- required for hotel search (npm install -g agent-browser && agent-browser install)")
	}
	if activeCDPPort != "" {
		args = append([]string{"--cdp", activeCDPPort}, args...)
	}
	// #nosec G204 -- exec.Command passes args as a literal argv array with
	// no shell involved, so shell-metacharacter injection is not possible
	// here. Every caller-supplied element (URLs, cookie names/values,
	// browser refs, the user's --to/--check-in/--check-out flag values) is
	// local operator input to their own machine, not remote/untrusted data.
	cmd := exec.Command("agent-browser", args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("agent-browser %s failed: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out.Bytes(), nil
}

func agentBrowserSnapshotRefs() (map[string]agentBrowserRefEntry, error) {
	out, err := runAgentBrowser("snapshot", "-i", "--json")
	if err != nil {
		return nil, err
	}
	var env agentBrowserSnapshotEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		return nil, fmt.Errorf("parsing agent-browser snapshot: %w", err)
	}
	return env.Data.Refs, nil
}

func agentBrowserCurrentURL() (string, error) {
	out, err := runAgentBrowser("get", "url")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// findRef returns the first ref whose accessible name contains nameSubstr
// (case-insensitive) and, if role is non-empty, matches that role exactly.
func findRef(refs map[string]agentBrowserRefEntry, nameSubstr, role string) (string, bool) {
	nameSubstr = strings.ToLower(nameSubstr)
	for ref, entry := range refs {
		if role != "" && entry.Role != role {
			continue
		}
		if strings.Contains(strings.ToLower(entry.Name), nameSubstr) {
			return ref, true
		}
	}
	return "", false
}

var refOrderRE = regexp.MustCompile(`^e(\d+)$`)

// refOrder extracts the numeric suffix from an agent-browser ref like "e25"
// for sorting. Refs are assigned sequentially as elements appear in the
// snapshot, which -- confirmed empirically -- correlates with the
// suggestion dropdown's own relevance order (the UI lists its best match
// for a query first; "New York, NY, USA" appeared at a lower ref number
// than the less specific "New York, USA" in the same dropdown). Returns
// a large sentinel for anything that doesn't parse, so it sorts last.
func refOrder(ref string) int {
	m := refOrderRE.FindStringSubmatch(ref)
	if m == nil {
		return 1 << 30
	}
	n := 0
	for _, c := range m[1] {
		n = n*10 + int(c-'0')
	}
	return n
}

// findBestDestinationOption prefers a metro/city-level "Place" suggestion
// over Company/Hotel/Airport suggestions that might also match the query
// text, and among multiple Place matches (confirmed to happen: Concur's
// catalog has more than one "New York" Place entry, one scoped to NY state
// broadly and one to the specific NYC metro -- only the latter had any
// hotel inventory within a useful radius) prefers the one with the lowest
// ref number, i.e. the UI's own top-ranked suggestion. This replaced an
// earlier version that returned whichever Place match Go's non-deterministic
// map iteration happened to visit first -- confirmed as a real bug during
// testing: it selected a hotel-less "New York, USA" Place on one run and a
// specific named hotel (not a Place at all) on another.
func findBestDestinationOption(refs map[string]agentBrowserRefEntry, query string) (string, bool) {
	query = strings.ToLower(query)
	var placeCandidates, otherCandidates []string
	for ref, entry := range refs {
		if entry.Role != "option" {
			continue
		}
		name := strings.ToLower(entry.Name)
		if !strings.Contains(name, query) {
			continue
		}
		if strings.HasPrefix(name, "place") {
			placeCandidates = append(placeCandidates, ref)
		} else {
			otherCandidates = append(otherCandidates, ref)
		}
	}
	pick := func(candidates []string) (string, bool) {
		if len(candidates) == 0 {
			return "", false
		}
		best := candidates[0]
		for _, r := range candidates[1:] {
			if refOrder(r) < refOrder(best) {
				best = r
			}
		}
		return best, true
	}
	if ref, ok := pick(placeCandidates); ok {
		return ref, true
	}
	return pick(otherCandidates)
}

// waitForRef polls fresh snapshots until an element matching nameSubstr/role
// appears or timeout elapses. UI transitions (tab switches, dropdown
// population) render on variable delay -- a single fixed sleep proved too
// short during testing (a tab-switch snapshot raced the render and still
// showed the previous tab's fields), so this retries instead of guessing
// one sleep duration that covers every transition.
func waitForRef(nameSubstr, role string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		refs, err := agentBrowserSnapshotRefs()
		if err != nil {
			return "", err
		}
		if ref, ok := findRef(refs, nameSubstr, role); ok {
			return ref, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("no element matching %q (role %q) appeared within %s -- the page structure may have changed", nameSubstr, role, timeout)
		}
		time.Sleep(400 * time.Millisecond)
	}
}

// waitForHotelShopSessionID polls the browser's current URL until it
// navigates to /travel/hotel/shop/{id} (the real, confirmed navigation
// Concur's UI performs after a hotel search completes) and returns the
// extracted session id.
func waitForHotelShopSessionID(timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		url, err := agentBrowserCurrentURL()
		if err == nil {
			if m := hotelShopURLRE.FindStringSubmatch(url); m != nil {
				return m[1], nil
			}
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("browser did not navigate to a hotel shop page within %s (last url: %s) -- the search may not have completed", timeout, url)
		}
		time.Sleep(1 * time.Second)
	}
}

func newHotelsSearchCmd(flags *rootFlags) *cobra.Command {
	var toQuery, checkIn, checkOut string
	var limit int
	var navTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search real hotel availability and rates (drives a real browser search -- searches only, never books)",
		Long: `Search hotel availability by driving Concur's real Hotel Search form via
agent-browser, then reading results directly. Requires agent-browser
installed and a live, already-logged-in Chrome session -- if the browser
redirects to sign-in, log in manually in the opened window and re-run.

OPTIONAL, to avoid that separate login entirely: run a dedicated Chrome
profile with remote debugging enabled and log into Concur there once. Use a
real named profile (Chrome menu -> "Add Person", or chrome://settings ->
Add profile) rather than a throwaway --user-data-dir, so 'auth login
--chrome --profile "<name>"' can also read its cookies:

  "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
    --remote-debugging-port=9222 --profile-directory="<profile dir name>"

This command auto-detects that session (tries ports 9222, 9333, 9229, or
set CONCUR_CDP_PORT) and attaches to it before falling back to its own
isolated login. Attaching, not copying cookies, is what makes this work --
see this file's header comment for the two approaches that were tried and
failed.

This is markedly slower than 'flights search' (real page navigation, not
a single HTTP call) and more fragile (depends on the Hotel Search form's
current structure). See --help on the parent 'hotels' command or the
patch notes for why this command's architecture differs from every other
command in this CLI.`,
		Example:     "  concur-pp-cli hotels search --to \"New York\" --check-in 2026-10-12 --check-out 2026-10-18 --yes",
		Annotations: map[string]string{"pp:endpoint": "hotels.search", "pp:method": "POST", "pp:path": "https://www-us2.api.concursolutions.com/cds/graphql", "mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if toQuery == "" || checkIn == "" || checkOut == "" {
				return usageErr(fmt.Errorf("--to, --check-in, and --check-out are required"))
			}
			if !flags.yes && !flags.dryRun {
				fmt.Fprintf(cmd.ErrOrStderr(), "refusing to drive a real browser search for %q without --yes (this submits a real search on your Concur tenant, though it books nothing)\n", toQuery)
				return fmt.Errorf("confirmation required: pass --yes")
			}
			if flags.dryRun {
				// Match the real-output branch's format detection below so
				// --json/agent/piped callers get a parseable JSON preview
				// instead of a prose-only line that breaks JSON fidelity.
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					preview := map[string]any{
						"dry_run":   true,
						"to":        toQuery,
						"check_in":  checkIn,
						"check_out": checkOut,
					}
					b, _ := json.MarshalIndent(preview, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(b))
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "(dry run -- would drive agent-browser through Hotel Search for %q, %s to %s, then read results directly)\n", toQuery, checkIn, checkOut)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			const stepTimeout = 10 * time.Second

			// Step 0: attach to a dedicated, already-authenticated Chrome
			// instance if one is running, instead of defaulting straight to
			// agent-browser's isolated profile (which needs its own login).
			// See detectDedicatedConcurBrowser and this file's header
			// comment.
			if port := detectDedicatedConcurBrowser(); port != "" {
				activeCDPPort = port
				fmt.Fprintf(cmd.ErrOrStderr(), "Using dedicated Concur browser on CDP port %s (no separate login needed)\n", port)
			}

			// Step 1: open the home page and clear any blocking overlay
			// (a WalkMe onboarding popup was observed intercepting clicks
			// on a fresh page load).
			if _, err := runAgentBrowser("open", hotelHomeURL); err != nil {
				return err
			}
			time.Sleep(1500 * time.Millisecond)
			refs, err := agentBrowserSnapshotRefs()
			if err != nil {
				return err
			}
			if _, ok := findRef(refs, "sign in", ""); ok {
				if activeCDPPort != "" {
					return fmt.Errorf("the dedicated Concur browser on CDP port %s is no longer logged in -- log in to concursolutions.com in that window again, then re-run this command", activeCDPPort)
				}
				return fmt.Errorf("not logged in to Concur in the automated browser -- log in manually in the opened Chrome window, then re-run this command (or set up a dedicated debug-enabled Chrome profile to skip this every time; see --help)")
			}
			if ref, ok := findRef(refs, "close", "button"); ok {
				_, _ = runAgentBrowser("click", "@"+ref)
				time.Sleep(300 * time.Millisecond)
			}

			// Step 2: click the Hotel tab, fill destination, select the
			// best matching option, fill dates, submit search. Each
			// element lookup polls rather than assuming a fixed render
			// delay -- see waitForRef's comment for why a single sleep
			// proved unreliable during testing.
			hotelTab, err := waitForRef("hotel", "tab", stepTimeout)
			if err != nil {
				return fmt.Errorf("could not find the Hotel tab: %w", err)
			}
			if _, err := runAgentBrowser("click", "@"+hotelTab); err != nil {
				return err
			}

			destRef, err := waitForRef("destination", "combobox", stepTimeout)
			if err != nil {
				return fmt.Errorf("could not find the hotel Destination field: %w", err)
			}
			if _, err := runAgentBrowser("click", "@"+destRef); err != nil {
				return err
			}
			if _, err := runAgentBrowser("keyboard", "type", toQuery); err != nil {
				return err
			}

			var optRef string
			deadline := time.Now().Add(stepTimeout)
			for {
				refs, err := agentBrowserSnapshotRefs()
				if err != nil {
					return err
				}
				if ref, ok := findBestDestinationOption(refs, toQuery); ok {
					optRef = ref
					break
				}
				if time.Now().After(deadline) {
					return fmt.Errorf("no destination suggestion matched %q within %s", toQuery, stepTimeout)
				}
				time.Sleep(400 * time.Millisecond)
			}
			if _, err := runAgentBrowser("click", "@"+optRef); err != nil {
				return err
			}

			datesRef, err := waitForRef("dates", "textbox", stepTimeout)
			if err != nil {
				return fmt.Errorf("could not find the hotel Dates field: %w", err)
			}
			dateRange := formatMDY(checkIn) + " - " + formatMDY(checkOut)
			if _, err := runAgentBrowser("fill", "@"+datesRef, dateRange); err != nil {
				return err
			}

			searchRef, err := waitForRef("search hotels", "button", stepTimeout)
			if err != nil {
				return fmt.Errorf("could not find the Search Hotels button: %w", err)
			}
			if _, err := runAgentBrowser("click", "@"+searchRef); err != nil {
				return err
			}

			// Step 3: wait for the resulting navigation to
			// /travel/hotel/shop/{id} and extract the session id from the
			// URL -- see this file's header comment for why this is the
			// mechanism, after two more complex approaches were tried and
			// rejected.
			sessionID, err := waitForHotelShopSessionID(navTimeout)
			if err != nil {
				return err
			}

			// Step 4: switch to direct HTTP for the read side -- confirmed
			// working via scripted replay, unlike the mutation.
			content, err := fetchHotelProperties(cmd, c, flags, sessionID, limit)
			if err != nil {
				return err
			}

			outputData, err := json.Marshal(content)
			if err != nil {
				return fmt.Errorf("marshaling hotel results: %w", err)
			}
			prov := attachFreshness(DataProvenance{Source: "live"}, flags)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				printProvenance(cmd, len(content), prov)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := outputData
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				wrapped, wrapErr = wrapPlatformStructuredOutput(wrapped, flags, "results", true)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(outputData, &items) == nil && len(items) > 0 {
					return printAutoTable(cmd.OutOrStdout(), items)
				}
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), outputData, flags, map[string]any{"source": "live"})
		},
	}
	cmd.Flags().StringVar(&toQuery, "to", "", "Destination: city or hotel area name (required)")
	cmd.Flags().StringVar(&checkIn, "check-in", "", "Check-in date, YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&checkOut, "check-out", "", "Check-out date, YYYY-MM-DD (required)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum hotels to return")
	cmd.Flags().DurationVar(&navTimeout, "nav-timeout", 30*time.Second, "Maximum time to wait for the browser-driven search to complete")
	return cmd
}

// formatMDY converts YYYY-MM-DD to the M/D/YYYY-ish format the date field
// accepts (confirmed via direct fill during discovery -- the field echoed
// back exactly what was typed without needing calendar-widget interaction).
func formatMDY(iso string) string {
	parts := strings.Split(iso, "-")
	if len(parts) != 3 {
		return iso
	}
	return parts[1] + "/" + parts[2] + "/" + parts[0]
}

const hotelPropertiesQuery = `query hotelProperties($id: ID!) {
  travel {
    hotel {
      shoppingSession(id: $id) {
        properties(input: {}) {
          content {
            name
            starRating {
              value
            }
            totalPrice {
              price {
                amount
                currencyCode
              }
            }
            policyCompliance {
              violationLevel
            }
          }
        }
      }
    }
  }
}`

type hotelPropertiesGQLEnvelope struct {
	Data struct {
		Travel struct {
			Hotel struct {
				ShoppingSession struct {
					Properties struct {
						Content []json.RawMessage `json:"content"`
					} `json:"properties"`
				} `json:"shoppingSession"`
			} `json:"hotel"`
		} `json:"travel"`
	} `json:"data"`
	Errors []gqlErrorEntry `json:"errors"`
}

// fetchHotelProperties reads hotel results for an existing shopping session
// via direct HTTP -- confirmed working via scripted replay, unlike the
// mutation that created the session.
func fetchHotelProperties(cmd *cobra.Command, c *client.Client, flags *rootFlags, sessionID string, limit int) ([]json.RawMessage, error) {
	body := map[string]any{"query": hotelPropertiesQuery, "variables": map[string]any{"id": sessionID}}
	raw, status, err := c.PostQueryWithParams(cmd.Context(), concurGraphQLPath, map[string]string{}, body)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	if status == 401 || status == 403 {
		return nil, fmt.Errorf("not authenticated (HTTP %d): run 'concur-pp-cli auth login --chrome' after logging in to concursolutions.com in Chrome", status)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("concur GraphQL BFF returned HTTP %d", status)
	}
	var env hotelPropertiesGQLEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("parsing hotel properties response: %w", err)
	}
	if len(env.Errors) > 0 {
		return nil, fmt.Errorf("concur GraphQL error fetching hotel properties: %s", env.Errors[0].Detail())
	}
	content := env.Data.Travel.Hotel.ShoppingSession.Properties.Content
	if limit > 0 && len(content) > limit {
		content = content[:limit]
	}
	return content, nil
}
