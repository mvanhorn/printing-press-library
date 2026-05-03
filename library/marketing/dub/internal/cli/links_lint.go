// Hand-written novel feature: links lint.
// Audit short-key slugs for lookalike collisions, reserved-word violations,
// and brand-conflict hazards across domains.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// reservedWords are short-keys that conflict with Dub's own routing or
// common meta paths.
var reservedWords = map[string]bool{
	"admin": true, "api": true, "app": true, "auth": true, "dashboard": true,
	"docs": true, "help": true, "login": true, "logout": true, "register": true,
	"settings": true, "static": true, "support": true, "terms": true,
	"privacy": true, "robots.txt": true, "sitemap.xml": true, "favicon.ico": true,
	"qr": true, "track": true, "events": true,
}

type lintFinding struct {
	ID       string `json:"id"`
	Domain   string `json:"domain"`
	Key      string `json:"key"`
	Severity string `json:"severity"`
	Issue    string `json:"issue"`
	Detail   string `json:"detail,omitempty"`
}

func newLinksLintCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Audit slugs for lookalike collisions, reserved words, and brand-conflict hazards",
		Long: `Pure local-data audit:
  • lookalike pairs (` + "`/launch`" + ` vs ` + "`/launches`" + `, ` + "`/foo`" + ` vs ` + "`/Foo`" + `)
  • reserved-word violations (` + "`/admin`" + `, ` + "`/api`" + `, ` + "`/dashboard`" + `, etc.)
  • case-sensitivity hazards across the same domain
  • slugs containing whitespace or path separators

Run ` + "`dub-pp-cli sync`" + ` first.`,
		Example: strings.Trim(`
  dub-pp-cli links lint --json
  dub-pp-cli links lint --json --select severity,key,issue
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, COALESCE(domain,''), COALESCE("key",'')
				FROM links
			`)
			if err != nil {
				return fmt.Errorf("querying links: %w", err)
			}
			defer rows.Close()

			type slug struct{ id, domain, key string }
			var slugs []slug
			for rows.Next() {
				var id, domain, key string
				if err := rows.Scan(&id, &domain, &key); err != nil {
					return err
				}
				slugs = append(slugs, slug{id, domain, key})
			}
			if len(slugs) == 0 {
				return hintEmptyStore("links")
			}

			byDomain := map[string][]slug{}
			for _, s := range slugs {
				byDomain[s.domain] = append(byDomain[s.domain], s)
			}

			var findings []lintFinding
			for domain, ds := range byDomain {
				_ = domain
				lower := map[string]string{}
				for _, s := range ds {
					if reservedWords[strings.ToLower(s.key)] {
						findings = append(findings, lintFinding{
							ID: s.id, Domain: s.domain, Key: s.key,
							Severity: "error",
							Issue:    "reserved-word",
							Detail:   "slug collides with a reserved path",
						})
					}
					if strings.ContainsAny(s.key, " \t/?#") {
						findings = append(findings, lintFinding{
							ID: s.id, Domain: s.domain, Key: s.key,
							Severity: "error",
							Issue:    "invalid-character",
							Detail:   "slug contains whitespace or path separator",
						})
					}
					l := strings.ToLower(s.key)
					if existing, ok := lower[l]; ok && existing != s.key {
						findings = append(findings, lintFinding{
							ID: s.id, Domain: s.domain, Key: s.key,
							Severity: "warning",
							Issue:    "case-collision",
							Detail:   fmt.Sprintf("differs only in case from %q", existing),
						})
					} else {
						lower[l] = s.key
					}
				}
				// Lookalike pairs: pluralizations and trailing-character drift.
				keys := make([]string, 0, len(ds))
				for _, s := range ds {
					keys = append(keys, s.key)
				}
				for i := 0; i < len(keys); i++ {
					for j := i + 1; j < len(keys); j++ {
						a, b := keys[i], keys[j]
						if a == b {
							continue
						}
						if isLookalike(a, b) {
							findings = append(findings, lintFinding{
								Domain: domain, Key: a,
								Severity: "warning",
								Issue:    "lookalike",
								Detail:   fmt.Sprintf("similar to %q", b),
							})
						}
					}
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), findings, flags)
			}
			if len(findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No lint findings.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-30s %s\n", "SEV", "KEY", "ISSUE")
			for _, f := range findings {
				fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-30s %s\n",
					f.Severity, truncate(f.Domain+"/"+f.Key, 30), f.Issue)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// isLookalike returns true when two short-keys are confusingly similar:
// pluralization (foo → foos), trailing character (foo → foo1), or single
// character substitution.
func isLookalike(a, b string) bool {
	if a == b {
		return false
	}
	if a+"s" == b || b+"s" == a {
		return true
	}
	if a+"-" == b || b+"-" == a {
		return true
	}
	// Levenshtein distance == 1 for short keys.
	if abs(len(a)-len(b)) > 1 {
		return false
	}
	return editDistanceLE(a, b, 1)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// editDistanceLE returns true if Levenshtein distance between a and b is at
// most maxD. Cheap implementation for very short strings.
func editDistanceLE(a, b string, maxD int) bool {
	if a == b {
		return true
	}
	if abs(len(a)-len(b)) > maxD {
		return false
	}
	d := 0
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			i++
			j++
			continue
		}
		d++
		if d > maxD {
			return false
		}
		switch {
		case len(a) > len(b):
			i++
		case len(b) > len(a):
			j++
		default:
			i++
			j++
		}
	}
	d += (len(a) - i) + (len(b) - j)
	return d <= maxD
}

// _ = sql.NullString to ensure database/sql is used after refactors.
var _ sql.NullString
