// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

// categoryAlias maps user-facing category inputs to the raw response key.
var categoryAlias = map[string]string{
	"discoverability":      "discoverability",
	"content":              "contentAccessibility",
	"contentaccessibility": "contentAccessibility",
	"bot":                  "botAccessControl",
	"botaccesscontrol":     "botAccessControl",
	"discovery":            "discovery",
	"api":                  "discovery",
	"auth":                 "discovery",
	"mcp":                  "discovery",
	"commerce":             "commerce",
}

func newReportCmd(flags *rootFlags) *cobra.Command {
	var category string
	var onlyFailing bool
	var checkID string
	var evidence bool
	var profile string

	cmd := &cobra.Command{
		Use:   "report <url>",
		Short: "Show the detailed per-check breakdown of a site's scan",
		Long: "Print the full per-check results for a site's most recent stored scan (run 'check'\n" +
			"first, or pass --data-source live to scan fresh). Filter with --category, --check,\n" +
			"--only-failing, or --profile, and add --evidence to include the raw probe requests\n" +
			"and responses the scanner ran.",
		Example: "  isitagentready-pp-cli report https://example.com --only-failing\n" +
			"  isitagentready-pp-cli report https://example.com --agent --select checks.discovery.mcpServerCard.status",
		// siteError for unreachable URLs is a valid 200 result, not an error;
		// skip the dogfood error-path probe (see check.go).
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would show the per-check report for a URL")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a URL argument is required, e.g. report https://example.com"))
			}
			url := args[0]

			// Resolve the category filter set (from --category and/or --profile).
			cats, err := resolveCategoryFilter(category, profile)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			raw, err := resolveReport(cmd, flags, url)
			if err != nil {
				return err
			}

			filtered, err := filterReportChecks(raw, cats, onlyFailing, checkID, evidence)
			if err != nil {
				return err
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printOutputWithFlags(cmd.OutOrStdout(), filtered, flags)
			}
			return renderReportTable(cmd, filtered)
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Show only one category: discoverability, content, bot, discovery, commerce")
	cmd.Flags().BoolVar(&onlyFailing, "only-failing", false, "Show only checks that failed")
	cmd.Flags().StringVar(&checkID, "check", "", "Show only a single check by id (e.g. mcpServerCard)")
	cmd.Flags().BoolVar(&evidence, "evidence", false, "Include the raw probe requests/responses for each check")
	cmd.Flags().StringVar(&profile, "profile", "", "Site-type view: all (default), content, apiApp")
	return cmd
}

// resolveCategoryFilter returns the set of raw category keys to keep, or nil to
// keep all. --category and --profile both narrow; when both are set the result
// is their intersection.
func resolveCategoryFilter(category, profile string) (map[string]bool, error) {
	var set map[string]bool
	if p := strings.ToLower(strings.TrimSpace(profile)); p != "" && p != "all" {
		cats, ok := profileCategories[p]
		if !ok {
			return nil, fmt.Errorf("unknown --profile %q (use all, content, or apiApp)", profile)
		}
		set = map[string]bool{}
		for _, c := range cats {
			set[c] = true
		}
	}
	if c := strings.ToLower(strings.TrimSpace(category)); c != "" {
		raw, ok := categoryAlias[c]
		if !ok {
			return nil, fmt.Errorf("unknown --category %q (use discoverability, content, bot, discovery, or commerce)", category)
		}
		if set == nil {
			set = map[string]bool{raw: true}
		} else if !set[raw] {
			// Intersection is empty: the requested category is not in the profile.
			return map[string]bool{}, nil
		} else {
			set = map[string]bool{raw: true}
		}
	}
	return set, nil
}

// filterReportChecks filters the "checks" object inside a raw report at the JSON
// level (preserving every check field except evidence when not requested) and
// returns the full report JSON with the filtered checks.
func filterReportChecks(raw json.RawMessage, cats map[string]bool, onlyFailing bool, checkID string, evidence bool) (json.RawMessage, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("parsing report: %w", err)
	}
	checksRaw, ok := top["checks"]
	if !ok {
		return raw, nil // siteError responses have no checks
	}
	var checks map[string]map[string]json.RawMessage
	if err := json.Unmarshal(checksRaw, &checks); err != nil {
		return raw, nil
	}
	filtered := map[string]map[string]json.RawMessage{}
	for cat, cks := range checks {
		if cats != nil && !cats[cat] {
			continue
		}
		for id, craw := range cks {
			if checkID != "" && id != checkID {
				continue
			}
			if onlyFailing && rawCheckStatus(craw) != "fail" {
				continue
			}
			if !evidence {
				craw = stripJSONKey(craw, "evidence")
			}
			if filtered[cat] == nil {
				filtered[cat] = map[string]json.RawMessage{}
			}
			filtered[cat][id] = craw
		}
	}
	b, err := json.Marshal(filtered)
	if err != nil {
		return nil, err
	}
	top["checks"] = b
	out, err := json.Marshal(top)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func rawCheckStatus(craw json.RawMessage) string {
	var c struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(craw, &c)
	return c.Status
}

func stripJSONKey(obj json.RawMessage, key string) json.RawMessage {
	var m map[string]json.RawMessage
	if json.Unmarshal(obj, &m) != nil {
		return obj
	}
	delete(m, key)
	b, err := json.Marshal(m)
	if err != nil {
		return obj
	}
	return b
}

// renderReportTable prints the filtered report as a human table of checks.
func renderReportTable(cmd *cobra.Command, raw json.RawMessage) error {
	rep, err := store.ParseReport(raw)
	if err != nil {
		return printOutput(cmd.OutOrStdout(), raw, false)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, bold(rep.URL))
	if rep.SiteError != nil {
		fmt.Fprintf(out, "  %s HTTP %d %s — target unreachable.\n", red("site error:"), rep.SiteError.HTTPStatus, rep.SiteError.StatusText)
		return nil
	}
	fmt.Fprintf(out, "  Level %d — %s\n", rep.Level, rep.LevelName)
	flat := rep.FlatChecks()
	if len(flat) == 0 {
		fmt.Fprintln(out, "  (no checks match the filter)")
		return nil
	}
	byCat := map[string][]store.CheckRef{}
	for _, c := range flat {
		byCat[c.Category] = append(byCat[c.Category], c)
	}
	for _, cat := range sortedCategories(rep) {
		checks := byCat[cat]
		if len(checks) == 0 {
			continue
		}
		sort.Slice(checks, func(i, j int) bool { return checks[i].ID < checks[j].ID })
		fmt.Fprintf(out, "\n  %s\n", bold(labelFor(cat)))
		tw := newTabWriter(out)
		for _, c := range checks {
			fmt.Fprintf(tw, "    %s\t%s\t%s\n", statusMark(c.Status), c.ID, truncate(c.Message, 70))
		}
		_ = tw.Flush()
	}
	return nil
}

func statusMark(status string) string {
	switch status {
	case "pass":
		return green("pass")
	case "fail":
		return red("fail")
	default:
		return yellow(status)
	}
}
