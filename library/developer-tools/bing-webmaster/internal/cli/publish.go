// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.
//
// publish: quota-aware bulk URL submission / publish pipeline. Gathers URLs
// from a file or sitemap (following one level of sitemap-index), dedupes,
// paces against the live submission quota, chunks to the 500-per-request cap,
// and submits. Print-only unless --confirm. Hand-authored transcendence command.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/bing-webmaster/internal/cliutil"
)

const submitBatchCap = 500

type bPublishPlan struct {
	Site         string   `json:"site"`
	TotalURLs    int      `json:"total_urls"`
	DailyQuota   int      `json:"daily_quota"`
	WouldSubmit  int      `json:"would_submit"`
	SkippedQuota int      `json:"skipped_over_quota"`
	Chunks       int      `json:"chunks"`
	Submitted    int      `json:"submitted"`
	Confirmed    bool     `json:"confirmed"`
	SampleURLs   []string `json:"sample_urls"`
}

func newPublishCmd(flags *rootFlags) *cobra.Command {
	var site, fromSitemap, fromFile string
	var confirm bool
	cmd := &cobra.Command{
		Use:     "publish",
		Short:   "Quota-paced bulk URL submission from a sitemap or file (print-only unless --confirm)",
		Long:    "Gather URLs from --from-sitemap (following one level of sitemap-index) or --file (one URL per line), dedupe them, cap the batch to your live remaining daily quota, and chunk to Bing's 500-per-request limit. Prints the plan by default; pass --confirm to actually submit via SubmitUrlBatch.",
		Example: "  bing-webmaster-pp-cli publish --site https://example.com --from-sitemap https://example.com/sitemap.xml --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Verify/CI probe: never touch the network or require inputs.
			if cliutil.IsVerifyEnv() {
				cmd.Println("publish: would gather URLs, pace against quota, and submit in 500-URL chunks (verify mode: no network, no submission)")
				return nil
			}
			if site == "" {
				if dryRunOK(flags) {
					cmd.Println("publish: provide --site and --from-sitemap or --file to preview the submission plan")
					return nil
				}
				return errRequiredFlag("site")
			}
			if fromSitemap == "" && fromFile == "" {
				if dryRunOK(flags) {
					cmd.Println("publish: provide --from-sitemap or --file to preview the submission plan")
					return nil
				}
				return fmt.Errorf("provide --from-sitemap or --file with URLs to submit")
			}

			var urls []string
			var err error
			if fromFile != "" {
				urls, err = readURLFile(fromFile)
				if err != nil {
					return err
				}
			} else {
				urls, err = gatherSitemapURLs(fromSitemap)
				if err != nil {
					return err
				}
			}
			urls = bDedupeURLs(urls)
			if len(urls) == 0 {
				return fmt.Errorf("no URLs found in the provided sitemap/file")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Live quota check (best-effort; 0 means "unknown, submit all").
			daily := 0
			if qdata, qerr := c.Get(cmd.Context(), "/json/GetUrlSubmissionQuota", map[string]string{"siteUrl": site}); qerr == nil {
				if v, ok := bNum(bCIMap(qdata), "DailyQuota"); ok {
					daily = int(v)
				}
			}

			toSubmit := urls
			skipped := 0
			if daily > 0 && len(urls) > daily {
				toSubmit = urls[:daily]
				skipped = len(urls) - daily
			}
			chunks := bChunk(toSubmit, submitBatchCap)

			plan := bPublishPlan{
				Site:         site,
				TotalURLs:    len(urls),
				DailyQuota:   daily,
				WouldSubmit:  len(toSubmit),
				SkippedQuota: skipped,
				Chunks:       len(chunks),
				Confirmed:    confirm,
			}
			for i, u := range toSubmit {
				if i >= 5 {
					break
				}
				plan.SampleURLs = append(plan.SampleURLs, u)
			}

			// Print-only unless explicitly confirmed (never submits under --dry-run).
			if !confirm || flags.dryRun {
				return emitIntel(cmd, flags, plan, func() { printPublishPlan(cmd, plan, false) })
			}

			// Real submission.
			for _, chunk := range chunks {
				body := map[string]any{"siteUrl": site, "urlList": chunk}
				if _, _, serr := c.Post(cmd.Context(), "/json/SubmitUrlBatch", body); serr != nil {
					return fmt.Errorf("submitting batch (submitted %d so far): %w", plan.Submitted, classifyAPIError(serr, flags))
				}
				plan.Submitted += len(chunk)
			}
			return emitIntel(cmd, flags, plan, func() { printPublishPlan(cmd, plan, true) })
		},
	}
	cmd.Flags().StringVar(&site, "site", "", "Verified site URL (required)")
	cmd.Flags().StringVar(&fromSitemap, "from-sitemap", "", "Sitemap URL to gather URLs from (follows one level of sitemap-index)")
	cmd.Flags().StringVar(&fromFile, "file", "", "File with one URL per line")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually submit (default: print the plan only)")
	return cmd
}

func printPublishPlan(cmd *cobra.Command, p bPublishPlan, submitted bool) {
	fmt.Fprintf(cmd.OutOrStdout(), "Publish plan for %s\n", p.Site)
	fmt.Fprintf(cmd.OutOrStdout(), "  URLs found: %d   daily quota: %d   would submit: %d   over-quota skipped: %d   chunks: %d\n",
		p.TotalURLs, p.DailyQuota, p.WouldSubmit, p.SkippedQuota, p.Chunks)
	for _, u := range p.SampleURLs {
		fmt.Fprintf(cmd.OutOrStdout(), "    - %s\n", u)
	}
	if submitted {
		fmt.Fprintf(cmd.OutOrStdout(), "  SUBMITTED %d URLs.\n", p.Submitted)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "  (not submitted — re-run with --confirm to send)")
	}
}

func readURLFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening URL file %q: %w", path, err)
	}
	defer f.Close()
	var urls []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			urls = append(urls, line)
		}
	}
	return urls, sc.Err()
}

// gatherSitemapURLs fetches a sitemap and returns its page URLs, following one
// level of sitemap-index nesting.
func gatherSitemapURLs(sitemapURL string) ([]string, error) {
	body, err := httpGetBody(sitemapURL)
	if err != nil {
		return nil, err
	}
	locs := bExtractLocs(body)
	if !bIsSitemapIndex(body) {
		return locs, nil
	}
	// Sitemap index: locs point to child sitemaps. Fetch each one level deep.
	var urls []string
	for _, child := range locs {
		childBody, cerr := httpGetBody(child)
		if cerr != nil {
			continue // skip unreachable child sitemaps rather than fail the whole run
		}
		urls = append(urls, bExtractLocs(childBody)...)
	}
	return urls, nil
}

func httpGetBody(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "bing-webmaster-pp-cli/1.0.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %q: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fetching %q: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64*1024*1024))
}
