package cli

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// tmDeadline represents a computed trademark maintenance deadline.
type tmDeadline struct {
	Type       string `json:"type"`
	DueDate    string `json:"dueDate"`
	WindowOpen string `json:"windowOpen"`
	DaysAway   int    `json:"daysAway"`
	Status     string `json:"status"`   // "upcoming", "open", "overdue", "expired"
	Optional   bool   `json:"optional"` // true for filings that are available but not mandatory (e.g., Section 15)
}

func newTrademarkDeadlinesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deadlines <serialNumber>",
		Short: "Calculate Section 8 and Section 9 maintenance deadlines",
		Long: `Fetches the trademark registration date and computes all upcoming
maintenance deadlines:

  Section 8 (Declaration of Continued Use): Due between the 5th and 6th
  year after registration, then every 10 years.

  Section 9 (Renewal): Due every 10 years after registration.

  Section 15 (Incontestability): Optional, available after 5 years of
  continuous use post-registration.

Shows past deadlines as expired and highlights the next upcoming window.`,
		Example: strings.Trim(`
  uspto-tsdr-pp-cli trademark deadlines 97123456
  uspto-tsdr-pp-cli trademark deadlines 97123456 --json
  uspto-tsdr-pp-cli trademark deadlines 97123456 --json --select type,dueDate,daysAway`, "\n"),
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

			serial := args[0]
			caseID := normalizeCaseID(serial)

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// PATCH: use GetJSON (plain HTTP) — surf overrides Accept header.
			path := replacePathParam("/casestatus/{caseid}/info", "caseid", caseID)
			data, err := c.GetJSON(path, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			snap := parseTrademarkStatus(data, serial)

			if snap.RegistrationDt == "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "trademark %s is not registered (status: %s) — no maintenance deadlines apply\n", serial, snap.Status)
				return nil
			}

			regDate, err := time.Parse("2006-01-02", snap.RegistrationDt)
			if err != nil {
				return fmt.Errorf("cannot parse registration date %q: %w", snap.RegistrationDt, err)
			}

			now := time.Now()
			deadlines := computeDeadlines(regDate, now)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), deadlines, flags)
			}

			// Human-readable output
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Deadlines for %s", serial)
			if snap.MarkText != "" {
				fmt.Fprintf(w, " (%s)", snap.MarkText)
			}
			fmt.Fprintf(w, "\nRegistered: %s\n\n", snap.RegistrationDt)

			for _, dl := range deadlines {
				indicator := "  "
				switch dl.Status {
				case "open":
					indicator = green("▶ ")
				case "upcoming":
					indicator = yellow("◦ ")
				case "overdue":
					indicator = red("! ")
				case "expired":
					indicator = "  "
				}
				fmt.Fprintf(w, "%s%-30s  Due: %s  Window: %s  (%s)\n",
					indicator, dl.Type, dl.DueDate, dl.WindowOpen, formatDaysAway(dl.DaysAway))
			}
			return nil
		},
	}
	return cmd
}

func computeDeadlines(regDate, now time.Time) []tmDeadline {
	var deadlines []tmDeadline

	// Section 8: First at 5-6 years, then every 10 years
	// Section 9: Every 10 years
	// Section 15: Available after 5 years (optional, one-time)

	// Section 15 — Incontestability (optional, available after 5 years of
	// continuous use). This is NOT a mandatory filing — a trademark owner may
	// choose to file it but is never required to. Modeled as an availability
	// marker, not a mandatory deadline.
	sec15Due := regDate.AddDate(5, 0, 0)
	sec15Window := regDate.AddDate(5, 0, 0) // Can file anytime after 5 years
	deadlines = append(deadlines, tmDeadline{
		Type:       "Section 15 (Incontestability — optional)",
		DueDate:    sec15Due.Format("2006-01-02"),
		WindowOpen: sec15Window.Format("2006-01-02"),
		DaysAway:   daysUntil(now, sec15Due),
		Status:     deadlineStatus(now, sec15Window, sec15Due),
		Optional:   true,
	})

	// Section 8: First declaration due between 5th and 6th year
	sec8FirstWindow := regDate.AddDate(5, 0, 0)
	sec8FirstDue := regDate.AddDate(6, 0, 0)
	deadlines = append(deadlines, tmDeadline{
		Type:       "Section 8 (5-6 year)",
		DueDate:    sec8FirstDue.Format("2006-01-02"),
		WindowOpen: sec8FirstWindow.Format("2006-01-02"),
		DaysAway:   daysUntil(now, sec8FirstDue),
		Status:     deadlineStatus(now, sec8FirstWindow, sec8FirstDue),
	})

	// Section 8 + 9 combined: Every 10 years
	for i := 1; i <= 5; i++ {
		years := i * 10
		renewalDue := regDate.AddDate(years, 0, 0)
		renewalWindow := renewalDue.AddDate(-1, 0, 0) // Window opens 1 year before

		deadlines = append(deadlines, tmDeadline{
			Type:       fmt.Sprintf("Section 8 & 9 (%d year)", years),
			DueDate:    renewalDue.Format("2006-01-02"),
			WindowOpen: renewalWindow.Format("2006-01-02"),
			DaysAway:   daysUntil(now, renewalDue),
			Status:     deadlineStatus(now, renewalWindow, renewalDue),
		})

		// Stop generating deadlines that are more than 20 years away
		if daysUntil(now, renewalDue) > 365*20 {
			break
		}
	}

	return deadlines
}

func deadlineStatus(now, windowOpen, dueDate time.Time) string {
	if now.After(dueDate) {
		// Grace period: 6 months after due date
		grace := dueDate.AddDate(0, 6, 0)
		if now.After(grace) {
			return "expired"
		}
		return "overdue"
	}
	if now.After(windowOpen) || now.Equal(windowOpen) {
		return "open"
	}
	return "upcoming"
}

func daysUntil(from, to time.Time) int {
	return int(math.Round(to.Sub(from).Hours() / 24))
}

func formatDaysAway(days int) string {
	if days < 0 {
		absDays := -days
		if absDays > 365 {
			return fmt.Sprintf("%d years ago", absDays/365)
		}
		if absDays > 30 {
			return fmt.Sprintf("%d months ago", absDays/30)
		}
		return fmt.Sprintf("%d days ago", absDays)
	}
	if days > 365 {
		return fmt.Sprintf("in %d years", days/365)
	}
	if days > 30 {
		return fmt.Sprintf("in %d months", days/30)
	}
	return fmt.Sprintf("in %d days", days)
}
