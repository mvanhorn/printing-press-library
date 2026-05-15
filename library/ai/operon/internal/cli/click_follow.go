// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — not generated.

package cli

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/operon/internal/cliutil"
)

var impressionIDPattern = regexp.MustCompile(`^imp_[a-f0-9]{16}$`)

// clickFollowResult is the structured output for `click follow`. Field names
// are snake_case for stable JSON consumption by agents/scripts; the human
// table renders the same data.
type clickFollowResult struct {
	ImpressionID  string `json:"impression_id"`
	RedirectURL   string `json:"redirect_url"`
	Scheme        string `json:"scheme"`
	SchemeOK      bool   `json:"scheme_ok"`
	Fallback      bool   `json:"fallback"`
	HeadAttempted bool   `json:"head_attempted"`
	HeadStatus    int    `json:"head_status,omitempty"`
	HeadOK        bool   `json:"head_ok"`
	HeadError     string `json:"head_error,omitempty"`
	Notes         string `json:"notes,omitempty"`
}

// newClickCmd is a top-level parent for click-tracking utilities. The
// existing `c` command is kept (it is the literal /c/{impressionId}
// endpoint shortcut) and `click follow` lives here so users can reach
// it under a discoverable verb namespace.
func newClickCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "click",
		Short: "Click-tracking utilities (redirect inspection, scheme validation).",
	}
	cmd.AddCommand(newClickFollowCmd(flags))
	return cmd
}

func newClickFollowCmd(flags *rootFlags) *cobra.Command {
	var skipHead bool

	cmd := &cobra.Command{
		Use:   "follow <impression-id>",
		Short: "Walk the /c/{impressionId} redirect and validate the landing URL.",
		Long: `Fetch /c/<impression-id>, inspect the 302 Location header, validate the URL
scheme is https or http (rejecting javascript:, data:, file:), detect the
operon.so fallback redirect (issued when an impression id is unknown), and
optionally HEAD the landing URL to confirm it responds.

The redirect itself is NOT followed by the underlying request — Location
header is read manually so the chain can be inspected and validated.`,
		Example: strings.Trim(`
  operon-pp-cli click follow imp_a1b2c3d4e5f60718
  operon-pp-cli click follow imp_a1b2c3d4e5f60718 --json
  operon-pp-cli click follow imp_a1b2c3d4e5f60718 --no-head
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			impID := strings.TrimSpace(args[0])
			if impID == "" {
				return cmd.Help()
			}
			if !impressionIDPattern.MatchString(impID) {
				if flags.asJSON {
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error":          "impression id must match ^imp_[a-f0-9]{16}$",
						"impression_id":  impID,
						"expected_shape": "imp_<16 lowercase hex chars>",
					}, flags)
				}
				return usageErr(fmt.Errorf(
					"impression id %q does not match ^imp_[a-f0-9]{16}$ (example: imp_a1b2c3d4e5f60718)",
					impID,
				))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			baseURL := c.BaseURL
			if baseURL == "" {
				baseURL = "https://api.operon.so"
			}
			target := baseURL + "/c/" + impID

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "GET %s\n", target)
				fmt.Fprintf(cmd.OutOrStdout(), "  (no follow — read Location header)\n")
				if !skipHead {
					fmt.Fprintf(cmd.OutOrStdout(), "  HEAD on resolved Location (10s timeout)\n")
				}
				return nil
			}

			// Manual redirect handling: stop the underlying client from
			// following so we can inspect Location.
			noFollow := &http.Client{
				Timeout: flags.timeout,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}

			req, err := http.NewRequest("GET", target, nil)
			if err != nil {
				return apiErr(fmt.Errorf("building request: %w", err))
			}
			req.Header.Set("User-Agent", "operon-pp-cli/click-follow")
			req.Header.Set("Accept", "*/*")

			resp, err := noFollow.Do(req)
			if err != nil {
				return apiErr(fmt.Errorf("GET %s: %w", target, err))
			}
			defer resp.Body.Close()

			result := clickFollowResult{ImpressionID: impID}

			if resp.StatusCode < 300 || resp.StatusCode >= 400 {
				result.Notes = fmt.Sprintf("expected 302, got HTTP %d", resp.StatusCode)
				return emitClickFollowAndExit(cmd, &result, flags, apiErr(fmt.Errorf(
					"GET %s returned HTTP %d (expected 302)",
					target, resp.StatusCode,
				)))
			}

			location := strings.TrimSpace(resp.Header.Get("Location"))
			if location == "" {
				result.Notes = "302 response carried no Location header"
				return emitClickFollowAndExit(cmd, &result, flags, apiErr(fmt.Errorf(
					"302 response had no Location header",
				)))
			}
			result.RedirectURL = location
			result.Scheme = schemeOf(location)
			result.SchemeOK = result.Scheme == "https" || result.Scheme == "http"

			if !result.SchemeOK {
				result.Notes = fmt.Sprintf("blocked scheme %q (only http/https are valid landing pages)", result.Scheme)
				return emitClickFollowAndExit(cmd, &result, flags, apiErr(fmt.Errorf(
					"redirect target uses unsafe scheme %q: %s",
					result.Scheme, location,
				)))
			}

			if location == "https://operon.so" || location == "https://operon.so/" {
				result.Fallback = true
				result.Notes = "fallback redirect (impression not found or invalid)"
			}

			if !skipHead {
				result.HeadAttempted = true
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				headReq, herr := http.NewRequestWithContext(ctx, "HEAD", location, nil)
				if herr != nil {
					result.HeadError = herr.Error()
				} else {
					headReq.Header.Set("User-Agent", "operon-pp-cli/click-follow")
					headClient := &http.Client{
						Timeout: 10 * time.Second,
						// One hop only — don't recurse beyond the
						// first redirect on the landing page.
						CheckRedirect: func(req *http.Request, via []*http.Request) error {
							if len(via) >= 1 {
								return http.ErrUseLastResponse
							}
							return nil
						},
					}
					hr, herr := headClient.Do(headReq)
					if herr != nil {
						result.HeadError = herr.Error()
					} else {
						result.HeadStatus = hr.StatusCode
						result.HeadOK = hr.StatusCode < 400
						hr.Body.Close()
					}
				}
			}

			return emitClickFollowAndExit(cmd, &result, flags, nil)
		},
	}

	cmd.Flags().BoolVar(&skipHead, "no-head", false, "Skip the HEAD probe on the redirect target")
	// Suppress an unused-import warning when cliutil drops out of the build
	// graph (it stays referenced via the verify-env guard pattern used by
	// peer commands).
	_ = cliutil.VerifyEnvVar
	return cmd
}

func schemeOf(rawURL string) string {
	idx := strings.Index(rawURL, ":")
	if idx <= 0 {
		return ""
	}
	return strings.ToLower(rawURL[:idx])
}

// emitClickFollowAndExit renders the result through the standard output
// pipeline and surfaces a typed error (if any) so the caller's exit code
// reflects the validation outcome.
func emitClickFollowAndExit(cmd *cobra.Command, r *clickFollowResult, flags *rootFlags, runErr error) error {
	if flags.asJSON || flags.compact || flags.csv || flags.selectFields != "" {
		if err := printJSONFiltered(cmd.OutOrStdout(), r, flags); err != nil {
			return err
		}
		return runErr
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "impression_id : %s\n", r.ImpressionID)
	fmt.Fprintf(w, "redirect_url  : %s\n", r.RedirectURL)
	fmt.Fprintf(w, "scheme        : %s (ok=%t)\n", r.Scheme, r.SchemeOK)
	fmt.Fprintf(w, "fallback      : %t\n", r.Fallback)
	if r.HeadAttempted {
		if r.HeadError != "" {
			fmt.Fprintf(w, "head          : error: %s\n", r.HeadError)
		} else {
			fmt.Fprintf(w, "head          : %d (ok=%t)\n", r.HeadStatus, r.HeadOK)
		}
	} else {
		fmt.Fprintf(w, "head          : skipped\n")
	}
	if r.Notes != "" {
		fmt.Fprintf(w, "notes         : %s\n", r.Notes)
	}
	return runErr
}
