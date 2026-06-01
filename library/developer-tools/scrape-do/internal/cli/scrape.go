// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored core `scrape` command — the full Scrape.do core-endpoint proxy
// surface (render, super proxy, geo, sessions, markdown, screenshots, browser
// interaction, wait controls, headers/cookies, retry/redirect controls), routed
// through the governor so every call respects the concurrency lease, debits the
// credit ledger, and honors the spend ceiling. Hand file (no generator header).

package cli

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newScrapeCmd(flags *rootFlags) *cobra.Command {
	var (
		render, super        bool
		geo, regionalGeo     string
		device               string
		markdown, returnJSON bool
		screenshot, fullShot bool
		session              int
		waitUntil, waitSel   string
		customWait           int
		noBlockResources     bool
		setCookies, play     string
		noRetry, noRedirect  bool
		transparent          bool
		retryTimeout         int
		targetTimeout        int
		outFile              string
		agentID              string
		maxCredits           int
	)

	cmd := &cobra.Command{
		Use:   "scrape <url>",
		Short: "Scrape any URL through Scrape.do (proxy, JS render, geo, markdown, screenshots) — governed",
		Long: `Scrape a URL through the Scrape.do proxy with full control over rendering,
proxy type, geo-targeting, output format, and browser behavior. Every call
acquires a shared concurrency lease (so parallel agents stay under the plan's
ConcurrentRequest cap) and debits the local credit ledger from the authoritative
per-call cost header.

By default the scraped content is written to stdout (pipe it to a file or a
parser); cost and remaining-credit info go to stderr. Use --json for a
structured envelope or --out to write the body to a file.`,
		Example: strings.Trim(`
  scrape-do-pp-cli scrape "https://example.com"
  scrape-do-pp-cli scrape "https://example.com" --render --markdown
  scrape-do-pp-cli scrape "https://example.com" --super --geo us
  scrape-do-pp-cli scrape "https://example.com" --json --select status,cost,remaining_credits
`, "\n"),
		Annotations: map[string]string{
			"pp:happy-args": "url=https://example.com",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			target := strings.TrimSpace(args[0])
			if target == "" {
				return usageErr(fmt.Errorf("scrape requires a target URL"))
			}
			if !strings.Contains(target, "://") {
				target = "https://" + target
			}
			// Validate the target locally so obviously-invalid input fails fast
			// (exit 2) without spending a credit on a guaranteed Scrape.do 400.
			if u, perr := url.Parse(target); perr != nil || u.Hostname() == "" ||
				(!strings.Contains(u.Hostname(), ".") && u.Hostname() != "localhost") {
				return usageErr(fmt.Errorf("invalid target URL %q: expected a fully-qualified URL like https://example.com", args[0]))
			}

			params := map[string]string{"url": target}
			if render || screenshot || fullShot || returnJSON || waitUntil != "" || waitSel != "" || customWait > 0 || play != "" {
				params["render"] = "true"
				render = true
			}
			if super {
				params["super"] = "true"
			}
			if geo != "" {
				params["geoCode"] = geo
			}
			if regionalGeo != "" {
				params["regionalGeoCode"] = regionalGeo
			}
			if device != "" {
				params["device"] = device
			}
			if markdown {
				params["output"] = "markdown"
			}
			if returnJSON {
				params["returnJSON"] = "true"
			}
			if screenshot {
				params["screenShot"] = "true"
			}
			if fullShot {
				params["fullScreenShot"] = "true"
			}
			if session > 0 {
				params["sessionId"] = strconv.Itoa(session)
			}
			if waitUntil != "" {
				params["waitUntil"] = waitUntil
			}
			if customWait > 0 {
				params["customWait"] = strconv.Itoa(customWait)
			}
			if waitSel != "" {
				params["waitSelector"] = waitSel
			}
			if noBlockResources {
				params["blockResources"] = "false"
			}
			if setCookies != "" {
				params["setCookies"] = setCookies
			}
			if play != "" {
				params["playWithBrowser"] = play
			}
			if noRetry {
				params["disableRetry"] = "true"
			}
			if retryTimeout > 0 {
				params["retryTimeout"] = strconv.Itoa(retryTimeout)
			}
			if noRedirect {
				params["disableRedirection"] = "true"
			}
			if transparent {
				params["transparentResponse"] = "true"
			}
			if targetTimeout > 0 {
				params["timeout"] = strconv.Itoa(targetTimeout)
			}

			est, mode := estimateScrapeCost(target, render, super)

			if dryRunOK(flags) {
				payload := map[string]any{
					"would_scrape": target, "mode": mode, "estimated_credits": est, "params": params,
				}
				return emitGov(cmd, flags, payload, fmt.Sprintf("would scrape %s [mode=%s, ~%d credits]", target, mode, est))
			}

			st, ext, err := openExtras(cmd.Context(), "")
			if err != nil {
				return err
			}
			defer st.Close()

			req := scrapeRequest{
				kind: "scrape", path: "/", params: params, target: target,
				family: hostOf(target), mode: mode, estCost: est,
				agent: resolveAgentID(agentID), maxCredits: maxCredits,
			}
			res, err := flags.runGoverned(cmd.Context(), ext, req)
			if err != nil {
				return err
			}

			if outFile != "" {
				if err := os.WriteFile(outFile, res.Body, 0o644); err != nil {
					return fmt.Errorf("writing --out file: %w", err)
				}
				payload := map[string]any{
					"target": target, "out": outFile, "bytes": len(res.Body),
					"cost": res.Cost, "cost_source": res.CostSource, "remaining_credits": res.RemainingCredits, "status": res.Status,
				}
				return emitGov(cmd, flags, payload, fmt.Sprintf("wrote %d bytes to %s  [cost=%d credits, remaining=%d]", len(res.Body), outFile, res.Cost, res.RemainingCredits))
			}

			if flags.asJSON {
				payload := map[string]any{
					"target": target, "status": res.Status, "mode": res.Mode,
					"cost": res.Cost, "cost_source": res.CostSource, "remaining_credits": res.RemainingCredits,
					"bytes": len(res.Body),
				}
				if !flags.compact {
					payload["body"] = string(res.Body)
				}
				return flags.printJSON(cmd, payload)
			}

			// Default: scraped content to stdout, cost to stderr.
			cmd.OutOrStdout().Write(res.Body)
			if !strings.HasSuffix(string(res.Body), "\n") {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			if !flags.quiet {
				fmt.Fprintf(os.Stderr, "[scrape] %s  status=%d  cost=%d credits  remaining=%d\n", target, res.Status, res.Cost, res.RemainingCredits)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&render, "render", false, "Enable headless-browser JS rendering (raises cost to 5/25)")
	cmd.Flags().BoolVar(&super, "super", false, "Use the residential/mobile (super) proxy pool (raises cost to 10/25)")
	cmd.Flags().StringVar(&geo, "geo", "", "Country geo-targeting (ISO alpha-2, e.g. us, gb, de)")
	cmd.Flags().StringVar(&regionalGeo, "regional-geo", "", "Region targeting: europe, asia, africa, oceania, northamerica, southamerica")
	cmd.Flags().StringVar(&device, "device", "", "Device emulation: desktop, mobile, or tablet")
	cmd.Flags().BoolVar(&markdown, "markdown", false, "Convert the scraped HTML to Markdown")
	cmd.Flags().BoolVar(&returnJSON, "return-json", false, "Return structured JSON (implies --render)")
	cmd.Flags().BoolVar(&screenshot, "screenshot", false, "Capture a viewport screenshot (implies --render)")
	cmd.Flags().BoolVar(&fullShot, "full-screenshot", false, "Capture a full-page screenshot (implies --render)")
	cmd.Flags().IntVar(&session, "session", 0, "Sticky proxy session id (1-1000000) to reuse the same IP")
	cmd.Flags().StringVar(&waitUntil, "wait-until", "", "Load condition: domcontentloaded, load, networkidle0, networkidle2 (implies --render)")
	cmd.Flags().IntVar(&customWait, "wait", 0, "Extra wait in ms after load for dynamic content (implies --render)")
	cmd.Flags().StringVar(&waitSel, "wait-selector", "", "Wait until this CSS selector appears (implies --render)")
	cmd.Flags().BoolVar(&noBlockResources, "no-block-resources", false, "Load CSS/images/fonts (default blocks them for speed)")
	cmd.Flags().StringVar(&setCookies, "set-cookies", "", "Cookie string to send with the request")
	cmd.Flags().StringVar(&play, "play", "", "playWithBrowser action script as a JSON array (implies --render)")
	cmd.Flags().BoolVar(&noRetry, "no-retry", false, "Disable Scrape.do's internal retries")
	cmd.Flags().IntVar(&retryTimeout, "retry-timeout", 0, "Max ms before cyclic retry of a failed request (5000-55000)")
	cmd.Flags().BoolVar(&noRedirect, "no-redirect", false, "Do not follow redirects")
	cmd.Flags().BoolVar(&transparent, "transparent", false, "Return the target site's actual status code")
	cmd.Flags().IntVar(&targetTimeout, "target-timeout", 0, "Per-request timeout to the target in ms (5000-120000)")
	cmd.Flags().StringVar(&outFile, "out", "", "Write the scraped body to this file instead of stdout")
	cmd.Flags().StringVar(&agentID, "agent-id", "", "Attribution id for the credit ledger (or set SCRAPEDO_AGENT_ID)")
	cmd.Flags().IntVar(&maxCredits, "max-credits", 0, "Refuse to dispatch if month-to-date spend + this call would exceed N credits")
	return cmd
}
