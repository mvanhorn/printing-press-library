// PATCH: novel `db lint` — run splinter-style checks locally against a migration BEFORE apply. Shifts the advisor left. Not in the Management API.
package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/dbchange"
)

// lintFinding mirrors the shape of a splinter advisor finding so local lint and
// the live `advisors` command read the same way.
type lintFinding struct {
	Rule     string `json:"rule"`
	Level    string `json:"level"` // ERROR | WARN | INFO
	Stmt     int    `json:"statement_index"`
	Object   string `json:"object,omitempty"`
	Message  string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

func newDBLintCmd(flags *rootFlags) *cobra.Command {
	var inlineSQL string

	cmd := &cobra.Command{
		Use:   "lint [file.sql]",
		Short: "Run splinter-style checks against a migration locally, before apply",
		Long: `Static security/performance checks against a migration file (or --sql), run
locally with no network. Shifts the Supabase advisor (splinter) left so the
008-class issues are caught at author time:

  - SECURITY DEFINER functions with PUBLIC/anon/authenticated EXECUTE not revoked
  - GRANT ... TO anon|authenticated on tables without RLS enabled in the same file
  - RLS policies using bare auth.uid() / auth.role() (init-plan re-eval; wrap in (select ...))
  - destructive statements (DROP/TRUNCATE/DELETE) without an obvious guard

Exit code is non-zero (5) if any ERROR-level finding is present, so it gates a
CI pipeline before 'db apply'.`,
		Example:     `  supabase-pp-cli db lint migrations/008_rls.sql --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			sql, src, err := loadSQL(args, inlineSQL)
			if err != nil {
				return err
			}
			m := dbchange.Parse(sql, src)
			findings := lintManifest(m, sql)

			out := cmd.OutOrStdout()
			errCount := 0
			for _, f := range findings {
				if f.Level == "ERROR" {
					errCount++
				}
			}

			if flags.asJSON || (!isTerminal(out) && !flags.csv && !flags.quiet && !flags.plain) {
				payload := map[string]any{
					"source":        src,
					"finding_count": len(findings),
					"error_count":   errCount,
					"findings":      findings,
				}
				if perr := printJSONFiltered(out, payload, flags); perr != nil {
					return perr
				}
			} else {
				if len(findings) == 0 {
					fmt.Fprintf(out, "No lint findings in %s.\n", src)
				} else {
					fmt.Fprintf(out, "%d finding(s) in %s:\n\n", len(findings), src)
					fmt.Fprintf(out, "%-6s %-30s %-4s %s\n", "LEVEL", "RULE", "STMT", "MESSAGE")
					fmt.Fprintf(out, "%-6s %-30s %-4s %s\n", "-----", "----", "----", "-------")
					for _, f := range findings {
						fmt.Fprintf(out, "%-6s %-30s %-4d %s\n", f.Level, truncate(f.Rule, 28), f.Stmt, f.Message)
					}
				}
			}
			if errCount > 0 {
				return apiErr(fmt.Errorf("%d error-level lint finding(s)", errCount))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&inlineSQL, "sql", "", "Inline SQL to lint (instead of a file argument)")
	return cmd
}

var (
	reAuthCall = regexp.MustCompile(`(?i)\bauth\.(uid|role|jwt)\s*\(\s*\)`)
	reSelectWrappedAuth = regexp.MustCompile(`(?i)\(\s*select\s+auth\.(uid|role|jwt)\s*\(\s*\)\s*\)`)
	reSecurityDefiner = regexp.MustCompile(`(?i)security\s+definer`)
	reGrantToPublic = regexp.MustCompile(`(?i)\bgrant\b.*\bto\b.*\b(public|anon|authenticated)\b`)
	reRevokeFromPublic = regexp.MustCompile(`(?i)\brevoke\b.*\bfrom\b.*\b(public|anon|authenticated)\b`)
)

// lintManifest runs the static ruleset over the parsed manifest + raw SQL.
func lintManifest(m *dbchange.Manifest, rawSQL string) []lintFinding {
	var out []lintFinding

	// Track whether the file enables RLS on a table that it also grants anon access to.
	rlsEnabledTables := map[string]bool{}
	grantedAnonTables := map[string]struct {
		stmt int
		obj  string
	}{}

	hasSecurityDefiner := false
	hasRevokeFromPublic := reRevokeFromPublic.MatchString(rawSQL)

	for _, s := range m.Statements {
		head := strings.ToUpper(s.SQL)
		obj := ""
		if len(s.Objects) > 0 {
			obj = s.Objects[0]
		}

		// Rule: RLS policy with bare auth.uid() (init-plan re-eval). Fires when
		// an auth.*() call appears that is NOT wrapped in (select ...).
		if s.OpClass == dbchange.OpRLS && strings.Contains(head, "POLICY") {
			if reAuthCall.MatchString(s.SQL) && !reSelectWrappedAuth.MatchString(s.SQL) {
				out = append(out, lintFinding{
					Rule:    "rls_init_plan",
					Level:   "WARN",
					Stmt:    s.Index,
					Object:  obj,
					Message: "policy uses bare auth.uid()/auth.role(); re-evaluated per row",
					Remediation: "wrap as (select auth.uid()) so the planner evaluates it once (initplan)",
				})
			}
		}

		// Track RLS enablement.
		if s.OpClass == dbchange.OpRLS && strings.Contains(head, "ENABLE ROW LEVEL SECURITY") {
			rlsEnabledTables[obj] = true
		}

		// SECURITY DEFINER function.
		if s.OpClass == dbchange.OpDDL && reSecurityDefiner.MatchString(s.SQL) {
			hasSecurityDefiner = true
			out = append(out, lintFinding{
				Rule:    "security_definer_function",
				Level:   "INFO",
				Stmt:    s.Index,
				Object:  obj,
				Message: "SECURITY DEFINER function defined; verify EXECUTE is revoked from public/anon",
				Remediation: "REVOKE EXECUTE ON FUNCTION " + obj + " FROM public, anon, authenticated; then GRANT only to intended roles",
			})
		}

		// GRANT to anon/authenticated. Split function grants from table grants:
		// RLS only applies to tables, so a `grant execute on function` must not
		// trip the anon-without-RLS rule. A function granted to anon/public is
		// instead the SECURITY DEFINER escalation surface, flagged below.
		if s.OpClass == dbchange.OpGrant && reGrantToPublic.MatchString(s.SQL) {
			if strings.Contains(head, "ON FUNCTION") || strings.Contains(head, "EXECUTE") {
				out = append(out, lintFinding{
					Rule:    "function_execute_to_anon",
					Level:   "WARN",
					Stmt:    s.Index,
					Object:  obj,
					Message: "EXECUTE on a function granted to public/anon/authenticated; if SECURITY DEFINER, this is a privilege-escalation surface",
					Remediation: "confirm the function is not SECURITY DEFINER, or REVOKE then GRANT to only intended roles",
				})
			} else {
				grantedAnonTables[obj] = struct {
					stmt int
					obj  string
				}{s.Index, obj}
			}
		}

		// Destructive without WHERE/guard.
		if s.Destructive {
			level := "WARN"
			msg := "destructive statement; ensure intended"
			if strings.HasPrefix(head, "DELETE") && !strings.Contains(head, " WHERE ") {
				level, msg = "ERROR", "DELETE without WHERE clause removes ALL rows"
			}
			if strings.HasPrefix(head, "TRUNCATE") {
				level, msg = "WARN", "TRUNCATE removes all rows and cannot be reverted from a receipt"
			}
			out = append(out, lintFinding{
				Rule:    "destructive_statement",
				Level:   level,
				Stmt:    s.Index,
				Object:  obj,
				Message: msg,
			})
		}
	}

	// Cross-statement: anon-granted table without RLS enabled in the same file.
	for tbl, g := range grantedAnonTables {
		if !rlsEnabledTables[tbl] {
			out = append(out, lintFinding{
				Rule:    "grant_anon_without_rls",
				Level:   "ERROR",
				Stmt:    g.stmt,
				Object:  tbl,
				Message: fmt.Sprintf("GRANT to anon/authenticated on %s but RLS is not enabled in this migration", tbl),
				Remediation: "ALTER TABLE " + tbl + " ENABLE ROW LEVEL SECURITY; and add policies, or move the grant",
			})
		}
	}

	// SECURITY DEFINER present but no REVOKE FROM public anywhere — the 008 gap.
	if hasSecurityDefiner && !hasRevokeFromPublic {
		out = append(out, lintFinding{
			Rule:    "security_definer_unrevoked",
			Level:   "ERROR",
			Stmt:    -1,
			Message: "migration defines SECURITY DEFINER function(s) but contains no REVOKE ... FROM public/anon — privilege-escalation risk",
			Remediation: "add REVOKE EXECUTE ... FROM public, anon, authenticated; before granting to intended roles",
		})
	}
	return out
}
