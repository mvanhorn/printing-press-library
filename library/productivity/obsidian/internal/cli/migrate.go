package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/vault"
)

func newMigrateCmd(flags *rootFlags) *cobra.Command {
	var rule string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Auto-fix mechanical lint violations across the vault.",
		Long: "Apply the bounded set of mechanical fixes for the protocol:\n" +
			"  date-iso             Coerce non-ISO date strings to YYYY-MM-DD when unambiguous\n" +
			"  type-enum            Normalize common type aliases (Meeting->meeting, meeting-notes->meeting, ...)\n" +
			"  missing-description  Synthesize a description from the first body line (warn-tier only)\n" +
			"  missing-status       Default status to active\n" +
			"  all                  Apply every above rule (default)\n\n" +
			"Always run with --dry-run first to preview changes.",
		Example:     "  obsidian-pp-cli migrate --rule date-iso --dry-run\n  obsidian-pp-cli migrate --rule all",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			if rule == "" {
				rule = "all"
			}
			v, _, err := openVaultOnly()
			if err != nil {
				return err
			}
			type change struct {
				Path    string   `json:"path"`
				Applied []string `json:"applied"`
			}
			var changes []change
			err = v.Walk(func(n *vault.Note) error {
				if !n.HasFM {
					return nil
				}
				before, _ := n.Frontmatter.Encode()
				applied := []string{}
				if rule == "all" || rule == "date-iso" {
					if normalized, ok := normalizeDate(n.Frontmatter.Date); ok && normalized != n.Frontmatter.Date {
						n.Frontmatter.Date = normalized
						applied = append(applied, "date-iso")
					}
				}
				if rule == "all" || rule == "type-enum" {
					if normalized, ok := normalizeType(n.Frontmatter.Type); ok && normalized != n.Frontmatter.Type {
						n.Frontmatter.Type = normalized
						applied = append(applied, "type-enum")
					}
				}
				if rule == "all" || rule == "missing-description" {
					if n.Frontmatter.Description == "" {
						if d := synthesizeDescription(n.Body); d != "" {
							n.Frontmatter.Description = d
							applied = append(applied, "missing-description")
						}
					}
				}
				if rule == "all" || rule == "missing-status" {
					if n.Frontmatter.Status == "" {
						n.Frontmatter.Status = "active"
						applied = append(applied, "missing-status")
					}
				}
				if len(applied) == 0 {
					return nil
				}
				after, err := n.Frontmatter.Encode()
				if err != nil {
					return err
				}
				_ = before // could diff for verbose mode
				_ = after
				changes = append(changes, change{Path: n.Path, Applied: applied})
				if dryRun {
					return nil
				}
				data, err := vault.AssembleNote(n.Frontmatter, []byte(n.Body))
				if err != nil {
					return err
				}
				return vault.AtomicWrite(n.AbsPath, data, 0o644)
			})
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
					"dry_run": dryRun,
					"rule":    rule,
					"changes": changes,
					"count":   len(changes),
				})
			}
			if len(changes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no notes to migrate")
				return nil
			}
			label := "applied"
			if dryRun {
				label = "would apply"
			}
			for _, c := range changes {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %v: %s\n", label, c.Applied, c.Path)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d notes %s\n", len(changes), label)
			return nil
		},
	}
	cmd.Flags().StringVar(&rule, "rule", "", "Rule to apply (date-iso|type-enum|missing-description|missing-status|all)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing")
	return cmd
}

// normalizeDate tries to coerce common non-ISO formats into YYYY-MM-DD.
// Returns (normalized, true) when conversion succeeded.
func normalizeDate(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return s, false
	}
	if _, err := time.Parse("2006-01-02", s); err == nil {
		return s, false
	}
	candidates := []string{
		"2006-1-2", "2006-01-2", "2006-1-02",
		"1/2/2006", "01/02/2006", "1-2-2006", "01-02-2006",
		"2006/01/02", "2006/1/2",
		"January 2, 2006", "Jan 2, 2006",
	}
	for _, layout := range candidates {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02"), true
		}
	}
	return s, false
}

var typeAliases = map[string]string{
	"meeting-notes": "meeting",
	"meeting note":  "meeting",
	"meetings":      "meeting",
	"people":        "person",
	"persons":       "person",
	"companies":     "company",
	"projects":      "project",
	"journals":      "journal",
	"sessions":      "session",
	"decisions":     "decision",
	"ideas":         "idea",
	"frameworks":    "framework",
}

func normalizeType(s string) (string, bool) {
	if s == "" {
		return s, false
	}
	lower := strings.ToLower(strings.TrimSpace(s))
	if v, ok := typeAliases[lower]; ok {
		return v, true
	}
	// Casing fix (Meeting -> meeting)
	if lower != s {
		for _, t := range vault.AllowedTypes {
			if t == lower {
				return lower, true
			}
		}
	}
	return s, false
}

var headingRe = regexp.MustCompile(`^#+ +(.+)$`)
var sentenceRe = regexp.MustCompile(`[.!?]\s`)

// synthesizeDescription extracts a one-line description from the first
// non-empty, non-heading body line. Truncates to 150 chars.
func synthesizeDescription(body string) string {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		if headingRe.MatchString(l) {
			// Use heading text only if it's not a level-1 (title) heading.
			if strings.HasPrefix(l, "##") {
				match := headingRe.FindStringSubmatch(l)
				return truncateRunes(match[1], 150)
			}
			continue
		}
		// First sentence of the line.
		if loc := sentenceRe.FindStringIndex(l); loc != nil {
			return truncateRunes(l[:loc[0]+1], 150)
		}
		return truncateRunes(l, 150)
	}
	return ""
}

func truncateRunes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// Ensure os is referenced for future use.
var _ = os.Stat
