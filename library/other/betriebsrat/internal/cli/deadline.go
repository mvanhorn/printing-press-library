package cli

import (
	"github.com/googlarz/betriebsrat/internal/betrvg"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type deadlineResult struct {
	Situation   string `json:"situation"`
	Paragraph   int    `json:"paragraph"`
	Days        int    `json:"days"`
	Description string `json:"description"`
	Note        string `json:"note"`
	FromDate    string `json:"from_date,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
}

func newDeadlineCmd(flags *rootFlags) *cobra.Command {
	var fromDate string
	var ical bool

	cmd := &cobra.Command{
		Use:   "deadline [situation]",
		Short: "Calculate BetrVG response deadline for a situation",
		Long: `Looks up the legal deadline (Frist) for the Betriebsrat to respond
in a given situation and optionally calculates the exact due date.

Key deadlines (BetrVG):
  § 102  Kündigung (ordentlich): 1 Woche
  § 102  Kündigung (außerordentlich): 3 Tage
  § 99   Einstellung / Versetzung: 1 Woche
  § 17 KSchG  Massenentlassung: 30 Tage`,
		Example: strings.Trim(`
  betriebsrat deadline "ordentliche Kündigung"
  betriebsrat deadline "außerordentliche Kündigung" --from 2026-05-09
  betriebsrat deadline "Einstellung neuer Mitarbeiter" --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			situation := strings.Join(args, " ")
			words := tokenize(situation)

			// Sort rules longest-situation-first so specific rules (e.g. "außerordentliche
			// kündigung") beat shorter overlapping ones (e.g. "ordentliche kündigung").
			rules := sortedBySpecificity(betrvg.Deadlines())

			var matched *betrvg.DeadlineRule
			// Phrase pass: situation contains the rule phrase (longest match wins).
			for _, rule := range rules {
				if betrvg.ContainsFold(situation, rule.Situation) {
					r := rule
					matched = &r
					break
				}
			}
			// Word-level fallback for single-word queries like "Kündigung".
			// Use original rule order (ordentliche first = safer default).
			if matched == nil {
				for _, rule := range betrvg.Deadlines() {
					for _, w := range words {
						if betrvg.ContainsFold(rule.Situation, w) || betrvg.ContainsFold(w, rule.Situation) {
							r := rule
							matched = &r
							break
						}
					}
					if matched != nil {
						break
					}
				}
			}

			result := deadlineResult{Situation: situation}

			if matched == nil {
				result.Note = "Keine spezifische BetrVG-Frist für diese Situation gefunden. Für allgemeine Stellungnahmen gilt in der Regel eine angemessene Frist. Konsultieren Sie einen Fachanwalt."
			} else {
				result.Paragraph = matched.Paragraph
				result.Days = matched.Days
				result.Description = matched.Description
				result.Note = matched.Note

				if fromDate != "" {
					t, err := time.Parse("2006-01-02", fromDate)
					if err == nil && matched.Days > 0 {
						due := t.AddDate(0, 0, matched.Days)
						// § 193 BGB: Samstag und Sonntag gelten wie Feiertage — Frist läuft bis zum nächsten Werktag.
						for due.Weekday() == time.Saturday || due.Weekday() == time.Sunday {
							due = due.AddDate(0, 0, 1)
						}
						result.FromDate = t.Format("02.01.2006")
						result.DueDate = due.Format("02.01.2006")
						result.Note += " Gesetzliche Feiertage können das Datum weiter verschieben (§ 193 BGB) — bitte landesabhängige Feiertage prüfen."

						if ical {
							return printDeadlineIcal(cmd, result, situation, t, due)
						}
					}
				}
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Situation: %s\n\n", result.Situation)
			if matched == nil {
				fmt.Fprintln(cmd.OutOrStdout(), result.Note)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Paragraf: § %d BetrVG\n", result.Paragraph)
			fmt.Fprintf(cmd.OutOrStdout(), "Frist: %d Tage\n", result.Days)
			fmt.Fprintf(cmd.OutOrStdout(), "Beschreibung: %s\n\n", result.Description)
			fmt.Fprintf(cmd.OutOrStdout(), "Hinweis: %s\n", result.Note)
			if result.DueDate != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nAnhörung ab: %s\nAntwort bis: %s\n", result.FromDate, result.DueDate)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&fromDate, "from", "", "Start date for deadline calculation (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&ical, "ical", false, "Output as iCalendar (.ics) format for import into calendar apps (requires --from)")
	return cmd
}

func printDeadlineIcal(cmd *cobra.Command, result deadlineResult, situation string, from, due time.Time) error {
	uid := fmt.Sprintf("betriebsrat-%d-%s@betriebsrat", result.Paragraph, from.Format("20060102"))
	nowStamp := time.Now().UTC().Format("20060102T150405Z")
	dueStamp := due.Format("20060102") // all-day event

	summary := fmt.Sprintf("BR-Frist: %s (§ %d BetrVG)", situation, result.Paragraph)
	description := strings.ReplaceAll(result.Note, "\n", "\\n")
	description = strings.ReplaceAll(description, ",", "\\,")

	ics := fmt.Sprintf(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//betriebsrat//DE
CALSCALE:GREGORIAN
METHOD:PUBLISH
BEGIN:VEVENT
UID:%s
DTSTAMP:%s
DTSTART;VALUE=DATE:%s
DTEND;VALUE=DATE:%s
SUMMARY:%s
DESCRIPTION:%s
STATUS:CONFIRMED
TRANSP:OPAQUE
BEGIN:VALARM
TRIGGER:-P1D
ACTION:DISPLAY
DESCRIPTION:Morgen läuft die BR-Frist ab!
END:VALARM
END:VEVENT
END:VCALENDAR
`, uid, nowStamp, dueStamp, due.AddDate(0, 0, 1).Format("20060102"), summary, description)

	fmt.Fprint(cmd.OutOrStdout(), ics)
	return nil
}

func sortedBySpecificity(rules []betrvg.DeadlineRule) []betrvg.DeadlineRule {
	out := make([]betrvg.DeadlineRule, len(rules))
	copy(out, rules)
	sort.SliceStable(out, func(i, j int) bool {
		return len(out[i].Situation) > len(out[j].Situation)
	})
	return out
}
