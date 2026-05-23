package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/vault"
)

func newLintCmd(flags *rootFlags) *cobra.Command {
	var severity, ruleFilter, folder, typeFilter, exitNonzeroOn string
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Walk the vault and report frontmatter violations against the three-layer protocol.",
		Long: "Apply the UCE three-layer-memory protocol rules (the same set encoded in\n" +
			"the Python `frontmatter_parser.py`) to every note. Findings are grouped\n" +
			"by severity (error|warn|info) and include the rule ID so they can be\n" +
			"filtered or piped into other tools. Pass --exit-nonzero-on=error for\n" +
			"a pre-commit / CI gate.",
		Example: "  obsidian-pp-cli lint --severity error --json\n  obsidian-pp-cli lint --rule bad-date-format --exit-nonzero-on=error",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			minSev := vault.Severity(severity)
			if severity == "" {
				minSev = vault.SeverityInfo
			}
			v, _, err := openVaultOnly()
			if err != nil {
				return err
			}
			var findings []vault.Finding
			err = v.Walk(func(n *vault.Note) error {
				if folder != "" && !pathHasPrefix(n.Path, folder) {
					return nil
				}
				if typeFilter != "" && n.Frontmatter.Type != typeFilter {
					return nil
				}
				fnd := vault.Validate(n.Path, n.Frontmatter, n.HasFM)
				for _, f := range vault.FilterBySeverity(fnd, minSev) {
					if ruleFilter != "" && f.Rule != ruleFilter {
						continue
					}
					findings = append(findings, f)
				}
				return nil
			})
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(findings); err != nil {
					return apiErr(err)
				}
			} else {
				if len(findings) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "OK — no findings")
				}
				for _, f := range findings {
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s: %s — %s\n", f.Severity, f.Rule, f.Path, f.Message)
				}
				printLintSummary(cmd, findings)
			}
			if exitNonzeroOn != "" {
				gate := vault.Severity(exitNonzeroOn)
				for _, f := range findings {
					if severityRank(f.Severity) <= severityRank(gate) {
						return &cliError{code: 2, err: fmt.Errorf("%d findings at or above %s severity", len(findings), exitNonzeroOn)}
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&severity, "severity", "", "Minimum severity to report (error|warn|info; default: info)")
	cmd.Flags().StringVar(&ruleFilter, "rule", "", "Restrict findings to one rule ID")
	cmd.Flags().StringVar(&folder, "folder", "", "Restrict the walk to this vault folder")
	cmd.Flags().StringVar(&typeFilter, "type", "", "Restrict the walk to notes of this type")
	cmd.Flags().StringVar(&exitNonzeroOn, "exit-nonzero-on", "", "Exit code 2 when any finding at this severity is present (error|warn|info)")
	return cmd
}

// severityRank exposes the validator's severity ordering for callers in this package.
func severityRank(s vault.Severity) int {
	switch s {
	case vault.SeverityError:
		return 0
	case vault.SeverityWarn:
		return 1
	case vault.SeverityInfo:
		return 2
	}
	return 99
}

func newReadinessCmd(flags *rootFlags) *cobra.Command {
	var sinceFlag, sourceTag, exitNonzeroOn string
	cmd := &cobra.Command{
		Use:   "readiness",
		Short: "Audit the vault for cm extraction-blocking violations (subset of lint).",
		Long: "cm's ObsidianImport extractor depends on a specific subset of the\n" +
			"protocol's required fields (type, date, description, status, date format).\n" +
			"This command runs the same engine as lint but reports only those rules,\n" +
			"so you can gate a Tuck sync on a clean readiness pass.",
		Example: "  obsidian-pp-cli readiness --since 2026-05-01 --exit-nonzero-on error",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			_ = sinceFlag
			_ = sourceTag // Reserved for future filtering; currently audits whole vault.
			v, _, err := openVaultOnly()
			if err != nil {
				return err
			}
			var findings []vault.Finding
			err = v.Walk(func(n *vault.Note) error {
				fnd := vault.Validate(n.Path, n.Frontmatter, n.HasFM)
				for _, f := range fnd {
					if vault.IsCMBlocking(f.Rule) || f.Rule == "no-frontmatter" {
						findings = append(findings, f)
					}
				}
				return nil
			})
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(findings); err != nil {
					return apiErr(err)
				}
			} else if len(findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "OK — vault is ready for cm extraction")
			} else {
				for _, f := range findings {
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s: %s — %s\n", f.Severity, f.Rule, f.Path, f.Message)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d cm-blocking findings\n", len(findings))
			}
			if exitNonzeroOn != "" {
				gate := vault.Severity(exitNonzeroOn)
				for _, f := range findings {
					if severityRank(f.Severity) <= severityRank(gate) {
						return &cliError{code: 2, err: fmt.Errorf("%d findings at or above %s severity block cm extraction", len(findings), exitNonzeroOn)}
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sinceFlag, "since", "", "Only audit notes modified since this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&sourceTag, "source-tag", "", "Only audit notes with this tag")
	cmd.Flags().StringVar(&exitNonzeroOn, "exit-nonzero-on", "", "Exit code 2 when any finding at this severity blocks (error|warn|info)")
	return cmd
}

func printLintSummary(cmd *cobra.Command, findings []vault.Finding) {
	errs := countSev(findings, vault.SeverityError)
	warns := countSev(findings, vault.SeverityWarn)
	infos := countSev(findings, vault.SeverityInfo)
	if errs+warns+infos == 0 {
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n%d error · %d warn · %d info\n", errs, warns, infos)
}

func countSev(findings []vault.Finding, sev vault.Severity) int {
	n := 0
	for _, f := range findings {
		if f.Severity == sev {
			n++
		}
	}
	return n
}

func pathHasPrefix(p, prefix string) bool {
	if prefix == "" {
		return true
	}
	if len(p) < len(prefix) {
		return false
	}
	return p[:len(prefix)] == prefix
}
