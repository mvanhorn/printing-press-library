package cli

import (
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/other/insurance-finder/internal/insurance"
	"github.com/spf13/cobra"
)

func newProfileCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Show, locate, or validate the saved applicant profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			return showProfile(cmd, f)
		},
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "show",
			Short: "Print the saved profile",
			RunE:  func(cmd *cobra.Command, args []string) error { return showProfile(cmd, f) },
		},
		&cobra.Command{
			Use:   "path",
			Short: "Print the resolved profile file path",
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Fprintln(cmd.OutOrStdout(), f.resolvedProfilePath())
				return nil
			},
		},
		&cobra.Command{
			Use:   "validate",
			Short: "Report any missing required fields",
			RunE: func(cmd *cobra.Command, args []string) error {
				p, err := f.loadProfile()
				if err != nil {
					return err
				}
				problems := p.Validate()
				res := struct {
					Path     string   `json:"path"`
					Complete bool     `json:"complete"`
					Problems []string `json:"problems"`
				}{f.resolvedProfilePath(), len(problems) == 0, problems}
				return f.emit(cmd, func(w io.Writer) {
					if res.Complete {
						fmt.Fprintf(w, "%s profile is complete\n", green("OK"))
						return
					}
					fmt.Fprintf(w, "%s %d field(s) missing:\n", yellow("Incomplete:"), len(problems))
					for _, p := range problems {
						fmt.Fprintf(w, "  - %s\n", p)
					}
				}, res)
			},
		},
	)
	return cmd
}

func showProfile(cmd *cobra.Command, f *rootFlags) error {
	p, err := f.loadProfile()
	if err != nil {
		return err
	}
	return f.emit(cmd, func(w io.Writer) { renderProfile(w, p) }, p)
}

func renderProfile(w io.Writer, p insurance.Profile) {
	row := func(k, v string) { fmt.Fprintf(w, "  %-26s %s\n", k+":", v) }
	fmt.Fprintln(w, bold("Applicant profile"))
	row("Legal entity", p.LegalName)
	if p.DBA != "" {
		row("DBA / brand", p.DBA)
	}
	row("Structure", p.EntityStructure+" ("+p.FormationState+")")
	row("Business address", p.BusinessAddress)
	row("Contact", fmt.Sprintf("%s <%s> %s", p.ContactName, p.ContactEmail, p.ContactPhone))
	row("Year started", fmt.Sprintf("%d", p.YearStarted))
	row("Revenue", p.RevenueBand)
	row("Staff", fmt.Sprintf("%d W-2 / %d 1099", p.EmployeesW2, p.Contractors1099))
	row("Class", p.IndustryClass)
	row("Importer class", importerClassLabel(p))
	if len(p.CountriesOfOrigin) > 0 {
		row("Country of origin", fmt.Sprintf("%v", p.CountriesOfOrigin))
	}
	row("GL limits", glLine(p))
	row("Effective date", p.EffectiveDate)
	if p.BudgetAnnualUSD > 0 {
		row("Budget", fmt.Sprintf("$%d/yr", p.BudgetAnnualUSD))
	}
}

func importerClassLabel(p insurance.Profile) string {
	if !p.IsImporterClass() {
		return "No (reseller / service)"
	}
	var parts []string
	if p.Importer {
		parts = append(parts, "importer")
	}
	if p.PrivateLabel {
		parts = append(parts, "private-label")
	}
	if p.Manufacturer {
		parts = append(parts, "manufacturer")
	}
	return yellow("Yes - deemed manufacturer") + fmt.Sprintf(" (%v)", parts)
}

func glLine(p insurance.Profile) string {
	if p.GLPerOccurrence == "" && p.GLAggregate == "" {
		return "(not set)"
	}
	return p.GLPerOccurrence + " / " + p.GLAggregate
}
