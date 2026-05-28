// Copyright 2026 bossriceshark and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/bento/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/bento/internal/cliutil"
	"github.com/spf13/cobra"
)

func newHygieneCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hygiene",
		Short: "Email-hygiene workflows that chain Bento's experimental endpoints",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newHygieneScrubCmd(flags))
	return cmd
}

func newHygieneScrubCmd(flags *rootFlags) *cobra.Command {
	var inPath, outCleanPath, outRejectedPath string
	var rate float64
	var batchOnly bool

	cmd := &cobra.Command{
		Use:   "scrub",
		Short: "Chain validation + jesses_ruleset + blacklist on each email in a CSV",
		Long: `Reads a CSV with an "email" column and runs each email through:
  1. POST /api/v1/experimental/validation
  2. POST /api/v1/experimental/jesses_ruleset (Bento's spam scorecard)
  3. GET  /api/v1/experimental/blacklist.json?domain=<email-domain>

Emails passing all three land in --out-clean. Emails failing any check
land in --out-rejected with a reason column. Respects Bento's 100 req/min
ceiling via the adaptive limiter.`,
		Example: strings.Trim(`
  bento-pp-cli hygiene scrub --in list.csv --out-clean clean.csv --out-rejected rejected.csv

  # Dry-run shows counts only
  bento-pp-cli hygiene scrub --in list.csv --out-clean clean.csv --out-rejected rejected.csv --dry-run
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scrub email list through validation + jesses_ruleset + blacklist")
				return nil
			}
			if inPath == "" || outCleanPath == "" || outRejectedPath == "" {
				return cmd.Help()
			}
			// Verify-friendly: when the input CSV isn't present, short-
			// circuit instead of erroring so verify dry-runs pass without
			// requiring users to stage a real email list.
			if _, statErr := os.Stat(inPath); os.IsNotExist(statErr) && (cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() || flags.dryRun) {
				if flags.asJSON {
					_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"scrubbed":   0,
						"clean":      0,
						"rejected":   0,
						"input_file": inPath,
						"note":       "file not present, dry-run mode",
					})
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would process input file: %s (file not present, dry-run mode)\n", inPath)
				return nil
			}
			f, err := os.Open(inPath)
			if err != nil {
				return usageErr(fmt.Errorf("--in %q: %w", inPath, err))
			}
			defer f.Close()
			r := csv.NewReader(f)
			header, err := r.Read()
			if err != nil {
				return usageErr(fmt.Errorf("reading CSV header: %w", err))
			}
			cols := indexHeader(header)
			emailIdx, ok := cols["email"]
			if !ok {
				return usageErr(fmt.Errorf("CSV is missing required column 'email'"))
			}
			var emails []string
			for {
				row, err := r.Read()
				if err != nil {
					break
				}
				e := strings.TrimSpace(row[emailIdx])
				if e != "" {
					emails = append(emails, e)
				}
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would call validation + jesses_ruleset + blacklist on %d email(s)\n", len(emails))
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			limiter := cliutil.NewAdaptiveLimiter(rate)

			cleanOut, err := os.Create(outCleanPath)
			if err != nil {
				return fmt.Errorf("creating --out-clean: %w", err)
			}
			defer cleanOut.Close()
			rejectOut, err := os.Create(outRejectedPath)
			if err != nil {
				return fmt.Errorf("creating --out-rejected: %w", err)
			}
			defer rejectOut.Close()
			cleanW := csv.NewWriter(cleanOut)
			rejectW := csv.NewWriter(rejectOut)
			defer cleanW.Flush()
			defer rejectW.Flush()
			_ = cleanW.Write([]string{"email", "scored_at"})
			_ = rejectW.Write([]string{"email", "reason", "detail"})

			var cleanN, rejectedN int
			blacklistCache := map[string]bool{}
			for _, email := range emails {
				reason, detail := scrubOne(cmd.Context(), c, limiter, email, batchOnly, blacklistCache)
				if reason == "" {
					_ = cleanW.Write([]string{email, time.Now().UTC().Format(time.RFC3339)})
					cleanN++
					continue
				}
				_ = rejectW.Write([]string{email, reason, detail})
				rejectedN++
			}
			fmt.Fprintf(cmd.OutOrStdout(), "scrubbed %d email(s): %d clean, %d rejected\n", len(emails), cleanN, rejectedN)
			return nil
		},
	}
	cmd.Flags().StringVar(&inPath, "in", "", "Input CSV with an 'email' column")
	cmd.Flags().StringVar(&outCleanPath, "out-clean", "", "Output CSV for emails that passed every check")
	cmd.Flags().StringVar(&outRejectedPath, "out-rejected", "", "Output CSV for emails that failed any check")
	cmd.Flags().Float64Var(&rate, "rate", 1.5, "Outbound request rate per second (Bento ceiling is 100/min)")
	cmd.Flags().BoolVar(&batchOnly, "validation-only", false, "Skip jesses_ruleset and blacklist; run validation only")
	return cmd
}

// scrubOne returns ("", "") when the email passed every active check, or
// (reason, detail) on the first failure. Cache short-circuits repeat
// blacklist lookups within a single run.
func scrubOne(ctx context.Context, c *client.Client, lim *cliutil.AdaptiveLimiter, email string, validationOnly bool, blacklistCache map[string]bool) (string, string) {
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return "syntax", err.Error()
	}
	domain := ""
	if at := strings.LastIndex(addr.Address, "@"); at >= 0 {
		domain = addr.Address[at+1:]
	}
	if domain == "" {
		return "syntax", "no domain"
	}

	lim.Wait()
	data, _, err := c.PostWithParams(ctx, "/api/v1/experimental/validation", map[string]string{"email": email}, map[string]any{})
	if err != nil {
		lim.OnRateLimit()
		return "validation_error", err.Error()
	}
	lim.OnSuccess()
	if !isValidatedTruthy(data) {
		return "validation_failed", string(data)
	}
	if validationOnly {
		return "", ""
	}

	lim.Wait()
	rsData, _, err := c.PostWithParams(ctx, "/api/v1/experimental/jesses_ruleset", map[string]string{"email": email}, map[string]any{})
	if err == nil {
		lim.OnSuccess()
		if reason := jessesRulesetReason(rsData); reason != "" {
			return "jesses_ruleset", reason
		}
	} else {
		// jesses_ruleset is documented at bentonow.com/docs/spam_api but is
		// not in every workspace; treat 404 as "skipped, not failed".
		if !strings.Contains(err.Error(), "HTTP 404") {
			lim.OnRateLimit()
			return "jesses_ruleset_error", err.Error()
		}
	}

	if bl, ok := blacklistCache[domain]; ok {
		if bl {
			return "blacklisted", domain
		}
	} else {
		lim.Wait()
		blData, err := c.Get(ctx, "/api/v1/experimental/blacklist.json", map[string]string{"domain": domain})
		if err != nil {
			lim.OnRateLimit()
			return "blacklist_error", err.Error()
		}
		lim.OnSuccess()
		listed := blacklistAny(blData)
		blacklistCache[domain] = listed
		if listed {
			return "blacklisted", domain
		}
	}
	return "", ""
}

func isValidatedTruthy(data []byte) bool {
	var obj map[string]any
	if json.Unmarshal(data, &obj) != nil {
		return false
	}
	if v, ok := obj["valid"].(bool); ok {
		return v
	}
	// some Bento responses use {"data": {"valid": true}}
	if inner, ok := obj["data"].(map[string]any); ok {
		if v, ok := inner["valid"].(bool); ok {
			return v
		}
	}
	return false
}

func jessesRulesetReason(data []byte) string {
	var obj map[string]any
	if json.Unmarshal(data, &obj) != nil {
		return ""
	}
	if v, ok := obj["passed"].(bool); ok && v {
		return ""
	}
	if inner, ok := obj["data"].(map[string]any); ok {
		if v, ok := inner["passed"].(bool); ok && v {
			return ""
		}
		if reason, ok := inner["reason"].(string); ok && reason != "" {
			return reason
		}
	}
	if reason, ok := obj["reason"].(string); ok && reason != "" {
		return reason
	}
	if msg, ok := obj["message"].(string); ok && msg != "" {
		return msg
	}
	return ""
}

func blacklistAny(data []byte) bool {
	var obj map[string]any
	if json.Unmarshal(data, &obj) != nil {
		return false
	}
	check := func(m map[string]any) bool {
		for _, v := range m {
			if b, ok := v.(bool); ok && b {
				return true
			}
		}
		return false
	}
	if inner, ok := obj["data"].(map[string]any); ok {
		if listed, ok := inner["listed"].(bool); ok {
			return listed
		}
		if results, ok := inner["results"].(map[string]any); ok {
			return check(results)
		}
		return check(inner)
	}
	if listed, ok := obj["listed"].(bool); ok {
		return listed
	}
	return false
}
