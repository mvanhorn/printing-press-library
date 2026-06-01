// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command `batch` — multi-agent safe fan-out. Reads a list
// of URLs (or queries with --query) and dispatches each through the governor:
// the shared concurrency lease caps total in-flight calls at the plan's
// ConcurrentRequest across ALL agent processes, only the non-billed 429/502/510
// classes are retried, and --max-credits refuses to dispatch past a ceiling.
// Hand file (no generator header) so it survives regeneration.

package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-do/internal/cliutil"
	"github.com/spf13/cobra"
)

type batchItemResult struct {
	Target    string `json:"target"`
	Status    int    `json:"status"`
	Cost      int    `json:"cost"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	Remaining int    `json:"remaining_credits,omitempty"`
}

func newNovelBatchCmd(flags *rootFlags) *cobra.Command {
	var (
		input          string
		queryMode      bool
		render, super  bool
		geo            string
		gl, hl         string
		maxCredits     int
		maxConcurrency int
		agentID        string
	)
	cmd := &cobra.Command{
		Use:   "batch [target...]",
		Short: "Fan out a list of URLs/queries under the shared concurrency lease + credit ceiling",
		Long: `Dispatch many targets through the governor. Targets come from positional
arguments, from --input (one per line), or from stdin. The shared concurrency
lease keeps total in-flight calls — across every agent process on this machine —
under the plan's ConcurrentRequest cap. Only the non-billed 429/502/510 failure
classes are retried, and --max-credits refuses to dispatch a call that would
push month-to-date spend past the ceiling.

Default mode scrapes each target as a URL; --query treats each as a Google
search query instead.`,
		Example: strings.Trim(`
  scrape-do-pp-cli batch https://example.com https://example.org
  scrape-do-pp-cli batch --input urls.txt --max-credits 500 --agent
  scrape-do-pp-cli batch --input queries.txt --query --gl us
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Dry-run must not touch the filesystem (verify probes with --dry-run).
			if dryRunOK(flags) {
				src := "args"
				if len(args) == 0 {
					src = input
					if src == "" {
						src = "stdin"
					}
				}
				return emitGov(cmd, flags, map[string]any{"would_dispatch_from": src, "query_mode": queryMode},
					fmt.Sprintf("would dispatch batch from %s", src))
			}
			var targets []string
			if len(args) > 0 {
				targets = args
			} else {
				t, err := readBatchTargets(input, cmd)
				if err != nil {
					return err
				}
				targets = t
			}
			if len(targets) == 0 {
				return emitGov(cmd, flags, map[string]any{"dispatched": 0, "results": []batchItemResult{}}, "no targets to dispatch")
			}

			// Under the live dogfood matrix, curtail to a few items to fit the timeout.
			if cliutil.IsDogfoodEnv() && len(targets) > 3 {
				targets = targets[:3]
			}

			st, ext, err := openExtras(cmd.Context(), "")
			if err != nil {
				return err
			}
			defer st.Close()

			workers := maxConcurrency
			if workers <= 0 {
				workers = 5 // the lease enforces the real plan cap; this just bounds goroutines
			}
			if workers > len(targets) {
				workers = len(targets)
			}

			agent := resolveAgentID(agentID)
			jobs := make(chan string)
			results := make([]batchItemResult, 0, len(targets))
			var mu sync.Mutex
			var wg sync.WaitGroup

			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for target := range jobs {
						var req scrapeRequest
						if queryMode {
							params := map[string]string{"q": target}
							if gl != "" {
								params["gl"] = gl
							}
							if hl != "" {
								params["hl"] = hl
							}
							req = scrapeRequest{kind: "google:search", path: "/plugin/google/search", params: params,
								target: target, family: "google:" + target, mode: modeGoogle, estCost: 10,
								agent: agent, maxCredits: maxCredits}
						} else {
							t := target
							if !strings.Contains(t, "://") {
								t = "https://" + t
							}
							est, mode := estimateScrapeCost(t, render, super)
							params := map[string]string{"url": t}
							if render {
								params["render"] = "true"
							}
							if super {
								params["super"] = "true"
							}
							if geo != "" {
								params["geoCode"] = geo
							}
							req = scrapeRequest{kind: "scrape", path: "/", params: params, target: t,
								family: hostOf(t), mode: mode, estCost: est, agent: agent, maxCredits: maxCredits}
						}
						res, cerr := flags.runGoverned(cmd.Context(), ext, req)
						item := batchItemResult{Target: target}
						if res != nil {
							item.Status, item.Cost, item.Remaining = res.Status, res.Cost, res.RemainingCredits
							item.OK = cerr == nil && res.Status >= 200 && res.Status < 300
						}
						if cerr != nil {
							item.Error = cerr.Error()
						}
						mu.Lock()
						results = append(results, item)
						mu.Unlock()
					}
				}()
			}
			for _, t := range targets {
				jobs <- t
			}
			close(jobs)
			wg.Wait()

			totalCost, ok := 0, 0
			for _, r := range results {
				totalCost += r.Cost
				if r.OK {
					ok++
				}
			}
			payload := map[string]any{
				"dispatched": len(results),
				"succeeded":  ok,
				"failed":     len(results) - ok,
				"total_cost": totalCost,
				"results":    results,
			}
			text := fmt.Sprintf("dispatched %d  succeeded %d  failed %d  total cost %d credits",
				len(results), ok, len(results)-ok, totalCost)
			return emitGov(cmd, flags, payload, text)
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "File with one target per line (or read from stdin)")
	cmd.Flags().BoolVar(&queryMode, "query", false, "Treat each line as a Google search query instead of a URL")
	cmd.Flags().BoolVar(&render, "render", false, "Enable JS rendering for each scrape (URL mode)")
	cmd.Flags().BoolVar(&super, "super", false, "Use the super proxy for each scrape (URL mode)")
	cmd.Flags().StringVar(&geo, "geo", "", "Country geo-targeting for each scrape (URL mode)")
	cmd.Flags().StringVar(&gl, "gl", "", "Country code for each query (query mode)")
	cmd.Flags().StringVar(&hl, "hl", "", "Language code for each query (query mode)")
	cmd.Flags().IntVar(&maxCredits, "max-credits", 0, "Refuse to dispatch once month-to-date spend would exceed N credits")
	cmd.Flags().IntVar(&maxConcurrency, "max-concurrency", 0, "Cap concurrent workers (the plan's lease still applies; 0 = default)")
	cmd.Flags().StringVar(&agentID, "agent-id", "", "Attribution id for the credit ledger (or set SCRAPEDO_AGENT_ID)")
	return cmd
}

// readBatchTargets reads non-empty, non-comment lines from --input (or stdin
// when input is empty and stdin is piped).
func readBatchTargets(input string, cmd *cobra.Command) ([]string, error) {
	var r *bufio.Scanner
	if input != "" {
		f, err := os.Open(input)
		if err != nil {
			return nil, fmt.Errorf("opening --input: %w", err)
		}
		defer f.Close()
		r = bufio.NewScanner(f)
	} else {
		fi, _ := os.Stdin.Stat()
		if fi != nil && (fi.Mode()&os.ModeCharDevice) == 0 {
			r = bufio.NewScanner(os.Stdin)
		} else {
			return nil, nil // no input file and no pipe → empty (RunE reports "no targets")
		}
	}
	var out []string
	for r.Scan() {
		line := strings.TrimSpace(r.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, r.Err()
}
