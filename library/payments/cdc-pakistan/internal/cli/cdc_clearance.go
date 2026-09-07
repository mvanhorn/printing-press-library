// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/cdcparse"
	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/client"
)

// Clearance is a Cloudflare clearance cookie together with the browser identity
// that minted it.
//
// THE THREE FIELDS ARE ONE UNIT AND MUST TRAVEL TOGETHER.
//   - cf_clearance is bound to the User-Agent that minted it. Replaying it under
//     a different UA fails, so storing the cookie without its UA is useless.
//   - MintedAt is required because the cookie LIES about its own lifetime. Its
//     `expires` attribute claims 365 days; the measured server-side lifetime is
//     a HARD ~30 minutes from mint, regardless of traffic. Measured twice: alive
//     at t+25m and dead at t+30m with traffic every <=5 min, and dead at t+31m
//     after 23 minutes of idle. So an idle timeout is ruled out and the cookie's
//     own expiry field must never be trusted.
type Clearance struct {
	Cookie    string    `json:"cookie"`
	UserAgent string    `json:"user_agent"`
	MintedAt  time.Time `json:"minted_at"`
}

// clearanceLifetime is the measured hard TTL. clearanceMargin is the refusal
// threshold: a command that cannot finish inside the remaining window refuses
// UP FRONT rather than dying mid-fan-out, which is the failure mode that made a
// MUFAP backfill silently store 33 of 131 dates.
const (
	clearanceLifetime = 30 * time.Minute
	clearanceMargin   = 3 * time.Minute
)

// ErrClearanceExpired and ErrClearanceMissing are distinct: one means re-mint,
// the other means set it up for the first time.
var (
	ErrClearanceExpired = errors.New("cf_clearance has expired (measured hard lifetime is ~30 minutes from mint)")
	ErrClearanceMissing = errors.New("no cf_clearance configured")
)

func clearancePath(flags *rootFlags) string {
	if flags != nil && flags.homePath != "" {
		return filepath.Join(flags.homePath, "cdc-clearance.json")
	}
	return filepath.Join(filepath.Dir(defaultDBPath("cdc-pakistan-pp-cli")), "cdc-clearance.json")
}

// LoadClearance resolves the clearance from the environment first, then the
// sidecar file. Env wins so CI and the live-dogfood harness can inject one
// without touching disk.
func LoadClearance(flags *rootFlags) (*Clearance, error) {
	if c := os.Getenv("CDC_PAKISTAN_CF_CLEARANCE"); c != "" {
		cl := &Clearance{
			Cookie:    c,
			UserAgent: os.Getenv("CDC_PAKISTAN_USER_AGENT"),
			MintedAt:  time.Now(),
		}
		if ts := os.Getenv("CDC_PAKISTAN_CLEARANCE_MINTED_AT"); ts != "" {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				cl.MintedAt = t
			}
		}
		if cl.UserAgent == "" {
			return nil, configErr(fmt.Errorf(
				"CDC_PAKISTAN_CF_CLEARANCE is set but CDC_PAKISTAN_USER_AGENT is not; " +
					"the clearance cookie is bound to the User-Agent that minted it and is useless without it"))
		}
		return cl, nil
	}

	b, err := os.ReadFile(clearancePath(flags))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, authErr(fmt.Errorf("%w: run `cdc-pakistan-pp-cli auth clearance set --help` to configure one", ErrClearanceMissing))
		}
		return nil, configErr(fmt.Errorf("reading clearance: %w", err))
	}
	var cl Clearance
	if err := json.Unmarshal(b, &cl); err != nil {
		return nil, configErr(fmt.Errorf("clearance file is not valid JSON: %w", err))
	}
	if cl.Cookie == "" || cl.UserAgent == "" {
		return nil, authErr(fmt.Errorf("%w: stored clearance is missing its cookie or user-agent", ErrClearanceMissing))
	}
	return &cl, nil
}

// Remaining reports how much of the measured window is left. It can be negative.
func (c *Clearance) Remaining() time.Duration {
	return clearanceLifetime - time.Since(c.MintedAt)
}

// CheckMargin refuses when there is not enough window left for the work.
// need is the caller's own estimate; pass 0 for a single request.
func (c *Clearance) CheckMargin(need time.Duration) error {
	rem := c.Remaining()
	if rem <= 0 {
		return authErr(fmt.Errorf("%w: minted %s ago; re-mint and retry",
			ErrClearanceExpired, time.Since(c.MintedAt).Round(time.Second)))
	}
	want := need + clearanceMargin
	if rem < want {
		return authErr(fmt.Errorf(
			"clearance has %s left but this run needs about %s: refusing to start work that cannot finish. "+
				"Re-mint the clearance, then re-run -- progress already stored is preserved and the run resumes",
			rem.Round(time.Second), want.Round(time.Second)))
	}
	return nil
}

// clearanceFetcher performs raw HTTP against CDC carrying the clearance.
//
// It exists rather than reusing the generated JSON client because CDC returns
// HTML fragments and PDF bytes, and because a Cloudflare challenge must be
// classified as a TRANSPORT ERROR here -- never handed onward as a body that a
// selector-based parser would read as "zero items".
type clearanceFetcher struct {
	cl      *Clearance
	base    string
	http    *http.Client
	reqs    int
	minGap  time.Duration
	lastReq time.Time
}

func newClearanceFetcher(cl *Clearance, timeout time.Duration) *clearanceFetcher {
	if timeout <= 0 || timeout > 2*time.Minute {
		// Per-request timeout is deliberately short and independent of the root
		// --timeout. A giant per-request timeout turns one dead TCP connection
		// into a multi-hour hang; that was observed on MUFAP with Send-Q stuck
		// and zero progress for 32 minutes.
		timeout = 45 * time.Second
	}
	return &clearanceFetcher{
		cl:     cl,
		base:   "https://www.cdcpakistan.com",
		http:   &http.Client{Timeout: timeout},
		minGap: time.Second,
	}
}

func (f *clearanceFetcher) pace() {
	if !f.lastReq.IsZero() {
		if gap := time.Since(f.lastReq); gap < f.minGap {
			time.Sleep(f.minGap - gap)
		}
	}
	f.lastReq = time.Now()
}

func (f *clearanceFetcher) do(ctx context.Context, req *http.Request) (int, []byte, error) {
	req.Header.Set("User-Agent", f.cl.UserAgent)
	req.Header.Set("Cookie", "cf_clearance="+f.cl.Cookie)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	f.pace()
	f.reqs++

	resp, err := f.http.Do(req)
	if err != nil {
		return 0, nil, apiErr(fmt.Errorf("requesting %s: %w", req.URL.Path, err))
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if readErr != nil {
		return resp.StatusCode, nil, apiErr(fmt.Errorf("reading %s: %w", req.URL.Path, readErr))
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return resp.StatusCode, body, rateLimitErr(fmt.Errorf("CDC returned 429 for %s", req.URL.Path))
	}
	// Challenge classification happens HERE, at the transport boundary, so no
	// caller can mistake an interstitial for an empty result.
	if resp.Header.Get("cf-mitigated") == "challenge" || cdcparse.IsChallenge(string(body)) {
		return resp.StatusCode, body, authErr(fmt.Errorf(
			"%w: CDC answered %s with a Cloudflare challenge for %s",
			ErrClearanceExpired, resp.Status, req.URL.Path))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, body, apiErr(fmt.Errorf("CDC returned %s for %s", resp.Status, req.URL.Path))
	}
	return resp.StatusCode, body, nil
}

// GetHTML fetches an HTML page.
func (f *clearanceFetcher) GetHTML(ctx context.Context, path string) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.base+path, nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	code, body, err := f.do(ctx, req)
	return code, string(body), err
}

// GetBinary fetches a PDF or other binary asset.
func (f *clearanceFetcher) GetBinary(ctx context.Context, rawURL string) (int, []byte, error) {
	if strings.HasPrefix(rawURL, "/") {
		rawURL = f.base + rawURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/pdf,*/*;q=0.8")
	return f.do(ctx, req)
}

// PostDownloads calls the ONLY valid downloads enumerator.
//
// The /downloads-category/<cat>/page/N/ URL path is NOT an alternative: it
// returns a byte-identical item block for every N (proven, sha256 of the item
// set is constant across pages 1-8), so walking it yields thousands of
// duplicates of the same ten items.
func (f *clearanceFetcher) PostDownloads(ctx context.Context, category string, year, paged int) (int, string, error) {
	form := url.Values{
		"action":   {"update_posts_by_year"},
		"year":     {fmt.Sprint(year)},
		"paged":    {fmt.Sprint(paged)},
		"cpt":      {"downloads"},
		"taxonomy": {"downloads_category"},
		"term":     {category},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		f.base+"/wp-admin/admin-ajax.php", strings.NewReader(form.Encode()))
	if err != nil {
		return 0, "", err
	}
	// Measured minimal header set: User-Agent + cf_clearance only. Unlike MUFAP,
	// X-Requested-With / Origin / Referer are NOT required.
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Accept", "*/*")
	code, body, err := f.do(ctx, req)
	return code, string(body), err
}

// --- Bridging the clearance store to the generated client ---------------
//
// THE DEFECT THIS CLOSES. The promoted `downloads`, `statistics` and `assets`
// commands are generated: they build the generated client, which reads its
// credentials from config/credentials.toml and the cookie jar. LoadClearance
// reads a different file (cdc-clearance.json) written by `auth clearance set`,
// and nothing joined the two. The consequence was not a warning but SILENCE:
// a user could store a clearance successfully and those three commands would
// still send NO Cookie header at all, get the Cloudflare interstitial, and
// report it as an ordinary failure. It is also visible in the publish-time
// live gate, where exactly those three commands are the ones recorded
// `unverified-needs-access`.
//
// WHY A CLIENT HOOK, AND WHY APPLY-ONLY. The promoted command files carry
// "DO NOT EDIT" and would lose any edit on regeneration, so the bridge lives
// here, in a preserved file, on the hook seam root.go already provides.
// The hook only ever ADDS the pair to a live client:
//
//   - It never persists. Writing the cookie through to credentials.toml or
//     cookies.json would be actively harmful: this file's own
//     `auth clearance set` declares `pp:happy-args` containing
//     `--cookie=example-clearance-value`, which the verify and shipcheck
//     harnesses EXECUTE. That placeholder is not the `<your-token>` shape the
//     client screens for, so a write-through would permanently install a junk
//     clearance, after which `doctor` would cheerfully report auth as
//     configured while every request shipped garbage.
//   - It never fails. Returning an error here would break `doctor`, `api`,
//     `import`, `--dry-run` and every MCP tool, all of which construct a
//     client whether or not a clearance exists. A missing or unreadable
//     clearance must leave those commands diagnosing the problem, not dying
//     before they start.
//   - It skips dry-run, so `--dry-run` never needs a credential to print a
//     request. root.go sets c.DryRun before it applies the hooks, so this is
//     reliable.
//
// The margin refusal deliberately stays where it already is, at the novel
// commands that do long fan-outs. A refusal inside client construction would
// turn "your clearance is old" into "client init error" in doctor's output.

// activeClearanceFlags is the live root flag set, captured at root-build time.
//
// The hook signature is func(*client.Client) error and carries no flags, but
// clearancePath needs them: with --home set it resolves to
// <home>/cdc-clearance.json, and with a nil receiver to the data directory
// instead. Using nil here would read a different file than the one
// `auth clearance set --home ...` just wrote.
var activeClearanceFlags *rootFlags

func init() {
	registerNovelCommand(func(_ *cobra.Command, flags *rootFlags) {
		activeClearanceFlags = flags
	})
	registerClientHook(applyClearanceToClient)
}

// applyClearanceToClient puts the stored cookie and its minting User-Agent on
// a newly constructed client, in memory only.
//
// The two travel together or not at all: cf_clearance is bound to the
// User-Agent that minted it, so sending the cookie under the generated
// client's default UA is as useless as sending no cookie, and would fail in a
// way that looks like a bad cookie rather than a mismatched one.
func applyClearanceToClient(c *client.Client) error {
	if c == nil || c.DryRun {
		return nil
	}
	cl, err := LoadClearance(activeClearanceFlags)
	if err != nil || cl == nil || cl.Cookie == "" || cl.UserAgent == "" {
		// Apply-only: no clearance configured is a normal state here.
		return nil
	}
	if c.Config != nil {
		if c.Config.Headers == nil {
			c.Config.Headers = map[string]string{}
		}
		// Config.Headers are applied ahead of the client's hardcoded UA
		// default, so this wins.
		c.Config.Headers["User-Agent"] = cl.UserAgent
	}
	if c.HTTPClient != nil {
		// Seeds the in-memory jar only and never writes cookies.json, so a
		// stale value cannot be left behind on disk for the next run.
		client.SeedCookieJarForDomain(c.HTTPClient.Jar,
			"https://www.cdcpakistan.com", "cf_clearance="+cl.Cookie, "www.cdcpakistan.com")
	}
	return nil
}
