// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.
// Hand-written transcendence command. Not generated.
//
// keywords volume — Google Ads search volume wrapper with auto-mode routing.
// --mode auto picks Live (sync, premium) for batches <= threshold, Standard
// (task-post + poll) for larger batches. Uses ResolveMode() from automode.go.

package cli

import (
	"bufio"
	"context"
	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/cliutil"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newKeywordsVolumeCmd(flags *rootFlags) *cobra.Command {
	var (
		kwFile       string
		stdinBody    bool
		locationCode int
		languageCode string
		mode         string
		threshold    int
		rateLimit    float64
		pollInterval time.Duration
	)

	cmd := &cobra.Command{
		Use:   "volume",
		Short: "Fetch Google Ads search volume with auto-mode routing (Live <-> Standard)",
		Long: `Fetches search volume from /v3/keywords_data/google_ads/search_volume.

--mode auto (default) picks Live (synchronous, premium) for batches at or
below --mode-threshold (default 5), and Standard (queued, ~3.3x cheaper) for
larger batches. --mode live and --mode standard force the route.

Input: either --keywords <file> (one keyword per line) or --stdin (a raw
JSON array of request items, same shape DataForSEO expects). Pick one.`,
		Example: strings.Trim(`
  dataforseo-pp-cli keywords volume --keywords /tmp/kw.txt --mode auto
  dataforseo-pp-cli keywords volume --keywords kw.txt --mode auto --json
  dataforseo-pp-cli keywords volume --keywords kw.txt --mode standard --location-code 2840
  cat batch.json | dataforseo-pp-cli keywords volume --stdin --mode live
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if kwFile == "" && !stdinBody {
				return usageErr(fmt.Errorf("either --keywords <file> or --stdin is required"))
			}
			if kwFile != "" && stdinBody {
				return usageErr(fmt.Errorf("--keywords and --stdin are mutually exclusive"))
			}

			// Build the POST body — a JSON array of request items.
			var body []any
			var batchSize int
			if stdinBody {
				stdinData, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				if err := json.Unmarshal(stdinData, &body); err != nil {
					return usageErr(fmt.Errorf("parsing stdin JSON array: %w", err))
				}
				// Count keywords across all items to drive auto-mode routing.
				for _, item := range body {
					if m, ok := item.(map[string]any); ok {
						if kws, ok := m["keywords"].([]any); ok {
							batchSize += len(kws)
						}
					}
				}
				if batchSize == 0 {
					batchSize = len(body)
				}
			} else {
				keywords, err := readKeywordsFile(kwFile)
				if err != nil {
					return err
				}
				if len(keywords) == 0 {
					return usageErr(fmt.Errorf("%s: no keywords found", kwFile))
				}
				batchSize = len(keywords)
				body = []any{map[string]any{
					"keywords":      keywords,
					"location_code": locationCode,
					"language_code": languageCode,
				}}
			}

			resolved := ResolveMode(batchSize, threshold, mode)

			if cliutil.IsVerifyEnv() {
				envelope := map[string]any{
					"action":      "volume",
					"resource":    "keywords-data",
					"mode":        resolved,
					"batch_size":  batchSize,
					"threshold":   threshold,
					"location":    locationCode,
					"language":    languageCode,
					"would_route": resolved,
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if rateLimit > 0 {
				// Honor --rate-limit on top of the client's existing limiter.
				// Best-effort: clients without SetRateLimit just ignore this.
				if setter, ok := any(c).(interface{ SetRateLimit(float64) }); ok {
					setter.SetRateLimit(rateLimit)
				}
			}

			switch resolved {
			case "live":
				path := "/v3/keywords_data/google_ads/search_volume/live"
				data, _, err := c.Post(path, body)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(data), flags)
			case "standard":
				ctx := context.Background()
				endpoint := "keywords_data/google_ads/search_volume/task_post"
				stderr := func(s string) { fmt.Fprint(cmd.ErrOrStderr(), s) }
				merged, err := runStandardTaskLifecycle(ctx, c, endpoint, body, pollInterval, stderr)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				return printJSONFiltered(cmd.OutOrStdout(), merged, flags)
			default:
				return usageErr(fmt.Errorf("unsupported mode %q (want auto|live|standard)", resolved))
			}
		},
	}

	cmd.Flags().StringVar(&kwFile, "keywords", "", "Path to keywords file (one per line)")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read raw JSON array of request items from stdin")
	cmd.Flags().IntVar(&locationCode, "location-code", 2840, "DataForSEO location_code (default 2840 = US)")
	cmd.Flags().StringVar(&languageCode, "language-code", "en", "DataForSEO language_code")
	cmd.Flags().StringVar(&mode, "mode", "auto", `Mode router: auto|live|standard. "auto" picks live when batch <= --mode-threshold.`)
	cmd.Flags().IntVar(&threshold, "mode-threshold", DefaultModeThreshold, "Batch size at or below which auto-mode picks Live")
	cmd.Flags().Float64Var(&rateLimit, "rate-limit", 0, "Extra rate limit (req/s); 0 disables")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 30*time.Second, "Wait between tasks_ready polls in standard mode")
	return cmd
}

// readKeywordsFile reads one-keyword-per-line input, skipping blanks.
func readKeywordsFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, usageErr(fmt.Errorf("open %s: %w", path, err))
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	out := make([]string, 0, 64)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return out, nil
}
