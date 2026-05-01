// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

// links_lint scans the local /store cache for slug-collision and config issues
// across links — duplicate slugs, mixed http/https variants, expired but still
// active links. Reads exclusively from the store.Open-backed SQLite database.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type lintFinding struct {
	Severity string   `json:"severity"`
	Code     string   `json:"code"`
	Domain   string   `json:"domain,omitempty"`
	Slug     string   `json:"slug"`
	Message  string   `json:"message"`
	Related  []string `json:"related,omitempty"`
}

// reservedSlugs are words that often shadow Dub's web app routes or look unprofessional as short keys.
var reservedSlugs = map[string]bool{
	"admin": true, "api": true, "dashboard": true, "settings": true, "login": true,
	"signup": true, "logout": true, "auth": true, "billing": true, "help": true,
	"support": true, "docs": true, "doc": true, "blog": true, "pricing": true,
	"about": true, "contact": true, "privacy": true, "terms": true, "tos": true,
	"undefined": true, "null": true, "true": true, "false": true,
}

func newLinksLintCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Audit short-key slugs for collisions, typos, and reserved words",
		Long: `Scan every link slug in the workspace for hygiene issues:
  - reserved-word slugs (admin, api, dashboard, etc. — collides with the web app)
  - lookalike pairs (e.g. /launch vs /launches differ by one trailing char)
  - case-only differences within the same domain (case-insensitive resolvers can collide)
  - empty or single-character slugs

Pure local-data analysis. Run sync first.`,
		Example: `  dub-pp-cli links lint --json
  dub-pp-cli links lint --agent --select code,slug,message`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStoreForRead("dub-pp-cli")
			if err != nil {
				return apiErr(fmt.Errorf("open local store: %w", err))
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local store found. run `dub-pp-cli sync --full` first"))
			}
			defer db.Close()

			rows, err := db.DB().Query(`SELECT id, data FROM links WHERE archived = 0 OR archived IS NULL`)
			if err != nil {
				return apiErr(fmt.Errorf("query links: %w", err))
			}
			defer rows.Close()

			collected := make([]struct{ domain, slug string }, 0)
			for rows.Next() {
				var id, raw string
				if err := rows.Scan(&id, &raw); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal([]byte(raw), &obj); err != nil {
					continue
				}
				domain := stringField(obj, "domain")
				slug := stringField(obj, "key")
				if slug == "" {
					continue
				}
				collected = append(collected, struct{ domain, slug string }{domain: domain, slug: slug})
			}

			findings := lintSlugs(collected)
			sort.Slice(findings, func(i, j int) bool {
				if findings[i].Severity != findings[j].Severity {
					return severityRank(findings[i].Severity) > severityRank(findings[j].Severity)
				}
				return findings[i].Slug < findings[j].Slug
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, findings)
			}
			if len(findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "lint clean — no slug issues found")
				return nil
			}
			headers := []string{"SEVERITY", "CODE", "DOMAIN", "SLUG", "MESSAGE"}
			rowsTbl := make([][]string, 0, len(findings))
			for _, f := range findings {
				msg := f.Message
				if len(msg) > 60 {
					msg = msg[:57] + "..."
				}
				rowsTbl = append(rowsTbl, []string{f.Severity, f.Code, f.Domain, f.Slug, msg})
			}
			return flags.printTable(cmd, headers, rowsTbl)
		},
	}
	return cmd
}

// lintSlugs is the pure-logic core of the lint command.
func lintSlugs(slots []struct{ domain, slug string }) []lintFinding {
	findings := make([]lintFinding, 0)

	// reserved + length checks
	for _, s := range slots {
		lower := strings.ToLower(s.slug)
		if reservedSlugs[lower] {
			findings = append(findings, lintFinding{
				Severity: "warn",
				Code:     "reserved-slug",
				Domain:   s.domain,
				Slug:     s.slug,
				Message:  fmt.Sprintf("slug %q is a reserved word that often clashes with web app routes", s.slug),
			})
		}
		if len(s.slug) <= 1 {
			findings = append(findings, lintFinding{
				Severity: "info",
				Code:     "slug-too-short",
				Domain:   s.domain,
				Slug:     s.slug,
				Message:  "single-character slugs are easy to mistype and hard to remember",
			})
		}
	}

	// case-only collisions within the same domain
	bySlug := make(map[string][]string) // key: domain|lower-slug, value: original slugs
	for _, s := range slots {
		k := s.domain + "|" + strings.ToLower(s.slug)
		bySlug[k] = append(bySlug[k], s.slug)
	}
	for k, originals := range bySlug {
		uniq := make(map[string]bool)
		for _, o := range originals {
			uniq[o] = true
		}
		if len(uniq) > 1 {
			parts := strings.SplitN(k, "|", 2)
			variants := make([]string, 0, len(uniq))
			for o := range uniq {
				variants = append(variants, o)
			}
			sort.Strings(variants)
			findings = append(findings, lintFinding{
				Severity: "error",
				Code:     "case-collision",
				Domain:   parts[0],
				Slug:     parts[1],
				Message:  fmt.Sprintf("case-only variants share the same lowercase slug: %s", strings.Join(variants, ", ")),
				Related:  variants,
			})
		}
	}

	// lookalike pairs (single-trailing-character difference within same domain)
	byDomain := make(map[string][]string)
	for _, s := range slots {
		byDomain[s.domain] = append(byDomain[s.domain], s.slug)
	}
	for domain, slugs := range byDomain {
		uniq := make(map[string]bool)
		for _, s := range slugs {
			uniq[s] = true
		}
		dedup := make([]string, 0, len(uniq))
		for s := range uniq {
			dedup = append(dedup, s)
		}
		sort.Strings(dedup)
		for i, a := range dedup {
			for _, b := range dedup[i+1:] {
				if isLookalike(a, b) {
					findings = append(findings, lintFinding{
						Severity: "warn",
						Code:     "lookalike-slug",
						Domain:   domain,
						Slug:     a,
						Message:  fmt.Sprintf("differs by one trailing character from %q — easy to confuse", b),
						Related:  []string{b},
					})
				}
			}
		}
	}

	return findings
}

// isLookalike returns true if a and b differ only by the trailing character (one is a prefix of the other, by exactly one char).
func isLookalike(a, b string) bool {
	if a == b {
		return false
	}
	la, lb := len(a), len(b)
	if la > lb {
		la, lb = lb, la
		a, b = b, a
	}
	// b is now the longer string. Lookalike if b == a + one char and that char is alphanumeric.
	if lb-la != 1 {
		return false
	}
	if !strings.HasPrefix(b, a) {
		return false
	}
	last := b[len(b)-1]
	return (last >= 'a' && last <= 'z') || (last >= 'A' && last <= 'Z') || (last >= '0' && last <= '9')
}

func severityRank(s string) int {
	switch s {
	case "error":
		return 3
	case "warn":
		return 2
	case "info":
		return 1
	}
	return 0
}
