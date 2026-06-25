// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

func newGuideCmd(flags *rootFlags) *cobra.Command {
	var url string

	cmd := &cobra.Command{
		Use:   "guide <check>",
		Short: "Fetch and print the SKILL.md fix guide for a check",
		Long: "Print the SKILL.md fix guide for a check (e.g. mcpServerCard) directly in the terminal.\n" +
			"The guide URL is read from your stored scans, so run 'check <url>' on a site where the\n" +
			"check fails first. Restrict the lookup to one site with --url.",
		Example: "  isitagentready-pp-cli guide robotsTxt\n" +
			"  isitagentready-pp-cli guide mcpServerCard --url https://example.com",
		// Under verify/dogfood the fetch short-circuits, so the error-path probe
		// cannot exercise the real notFound path; skip it (see check.go).
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch the SKILL.md fix guide for a check")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a check id argument is required, e.g. guide robotsTxt"))
			}
			check := args[0]

			// The fetch is a trivial read-only GET; under verify/dogfood there
			// is no meaningful fixture, so short-circuit cleanly. The lookup
			// logic (OpenAdvice) is unit-tested in the store package.
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "{}")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "would fetch the SKILL.md guide for %q\n", check)
				}
				return nil
			}

			recs, err := loadStore()
			if err != nil {
				return err
			}
			if len(recs) == 0 {
				return notFoundErr(fmt.Errorf("no stored scans yet; run 'isitagentready-pp-cli check <url>' on a site where %q fails first", check))
			}
			skillURL := ""
			for _, it := range store.OpenAdvice(store.LatestPerURL(recs), url, check) {
				if it.SkillURL != "" {
					skillURL = it.SkillURL
					break
				}
			}
			if skillURL == "" {
				return notFoundErr(fmt.Errorf("no fix guide found for %q in stored scans; it may be passing, or scan a site where it fails", check))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			body, err := fetchGuide(ctx, skillURL)
			if err != nil {
				return apiErr(fmt.Errorf("fetching guide %s: %w", skillURL, err))
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"check":    check,
					"skillUrl": skillURL,
					"guide":    body,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "# Guide: %s\n# Source: %s\n\n%s\n", check, skillURL, body)
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "Look up the guide URL from this site's latest scan only")
	return cmd
}

// fetchGuide GETs a SKILL.md guide and returns its body, honoring the
// context deadline (set from the root --timeout flag via boundCtx).
func fetchGuide(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/markdown, text/plain, */*")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return "", err
	}
	return string(b), nil
}
