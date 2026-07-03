package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/biz-insurance-finder/internal/insurance"
	"github.com/spf13/cobra"
)

// intakeResult is the structured output of the intake command.
type intakeResult struct {
	Path     string            `json:"path"`
	Saved    bool              `json:"saved"`
	Problems []string          `json:"problems,omitempty"`
	Profile  insurance.Profile `json:"profile"`
}

// prompter asks one question at a time. Prompts go to stderr so stdout stays
// clean for JSON. When interactive is false (--no-input), every answer is the
// default, so the command builds a profile without blocking.
type prompter struct {
	r           *bufio.Reader
	out         io.Writer
	interactive bool
}

func (p *prompter) ask(label, def string) string {
	if !p.interactive {
		return def
	}
	if def != "" {
		fmt.Fprintf(p.out, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(p.out, "%s: ", label)
	}
	line, err := p.r.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		if err != nil {
			p.interactive = false // EOF: stop prompting, take defaults for the rest
		}
		return def
	}
	return line
}

func (p *prompter) askBool(label string, def bool) bool {
	d := "y/N"
	if def {
		d = "Y/n"
	}
	ans := strings.ToLower(strings.TrimSpace(p.ask(label+" ("+d+")", "")))
	switch ans {
	case "":
		return def
	case "y", "yes", "true", "1":
		return true
	case "n", "no", "false", "0":
		return false
	default:
		return def
	}
}

func (p *prompter) askInt(label string, def int) int {
	s := p.ask(label, strconv.Itoa(def))
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}

func newIntakeCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "intake",
		Short: "Interview the business and save a reusable applicant profile (profile.json)",
		Long: `intake walks through the applicant questions one at a time with sensible
defaults and writes a reusable profile to profile.json. Re-running it edits the
existing profile (your prior answers become the defaults).

Prompts are written to stderr, so you can script it by piping answers on stdin
(blank line = accept the default). With --no-input every answer takes its
default - useful for agents that will edit the JSON directly afterward.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := f.resolvedProfilePath()

			// Start from an existing profile (edit mode) or the defaults. If a
			// profile file exists but cannot be read, stop instead of silently
			// starting from defaults - in --no-input mode we would otherwise
			// overwrite the user's saved data with a fully-defaulted profile.
			base := insurance.DefaultProfile(time.Now())
			if _, err := os.Stat(path); err == nil {
				existing, lerr := insurance.LoadProfile(path)
				if lerr != nil {
					return dataErr(fmt.Errorf("%w\nmove it aside or delete it to start fresh", lerr))
				}
				base = existing
			}

			pr := &prompter{
				r:           bufio.NewReader(cmd.InOrStdin()),
				out:         cmd.ErrOrStderr(),
				interactive: !f.noInput,
			}
			if pr.interactive {
				fmt.Fprintln(pr.out, bold("Applicant intake")+" - press Enter to accept the [default]. Ctrl-C to cancel.")
				fmt.Fprintln(pr.out)
			}

			p := runInterview(pr, base)

			// Effective date default: next business working day.
			if p.EffectiveDate == "" {
				p.EffectiveDate = insurance.NextBusinessDay(time.Now()).Format("2006-01-02")
			}

			problems := p.Validate()
			res := intakeResult{Path: path, Profile: p, Problems: problems}

			if f.dryRun {
				res.Saved = false
			} else {
				if err := insurance.SaveProfile(path, p); err != nil {
					return dataErr(err)
				}
				res.Saved = true
				res.Profile = p
			}

			return f.emit(cmd, func(w io.Writer) { renderIntake(w, res, f.dryRun) }, res)
		},
	}
}

// runInterview fills the profile one question at a time.
func runInterview(pr *prompter, p insurance.Profile) insurance.Profile {
	section := func(name string) {
		if pr.interactive {
			fmt.Fprintln(pr.out, dim("-- "+name+" --"))
		}
	}

	section("Business identity")
	p.LegalName = pr.ask("Legal entity name", p.LegalName)
	p.DBA = pr.ask("Brand / DBA (optional)", p.DBA)
	p.EntityStructure = pr.ask("Entity structure (LLC, S-Corp, Sole Proprietor...)", p.EntityStructure)
	p.FormationState = pr.ask("State of formation", p.FormationState)
	p.BusinessAddress = pr.ask("Business address", p.BusinessAddress)
	p.MailingAddress = pr.ask("Mailing address (blank = same as business)", p.MailingAddress)

	section("Contact / named insured")
	p.ContactName = pr.ask("Contact / named insured name", p.ContactName)
	p.ContactEmail = pr.ask("Contact email", p.ContactEmail)
	p.ContactPhone = pr.ask("Contact phone", p.ContactPhone)

	section("Operations")
	p.YearStarted = pr.askInt("Year business started (operating history, NOT a rebrand)", p.YearStarted)
	p.RevenueBand = pr.ask("Annual revenue band (e.g. $100k-$175k)", p.RevenueBand)
	p.EmployeesW2 = pr.askInt("Number of W-2 employees", p.EmployeesW2)
	p.Contractors1099 = pr.askInt("Number of 1099 contractors", p.Contractors1099)
	p.IndustryClass = pr.ask("Industry / class (e.g. retail store, e-commerce, contractor, consultant, importer)", p.IndustryClass)

	section("Importer / private-label / manufacturer status (drives routing)")
	p.Importer = pr.askBool("Do you IMPORT products?", p.Importer)
	p.PrivateLabel = pr.askBool("Do you sell your own PRIVATE-LABEL brand?", p.PrivateLabel)
	p.Manufacturer = pr.askBool("Do you MANUFACTURE (or have products made for you)?", p.Manufacturer)
	p.Products = pr.ask("Products (short description)", p.Products)
	if p.IsImporterClass() {
		coo := pr.ask("Country/countries of origin (comma-separated)", strings.Join(p.CountriesOfOrigin, ", "))
		p.CountriesOfOrigin = splitCSV(coo)
	}
	p.SellsOnAmazon = pr.askBool("Do you sell on Amazon?", p.SellsOnAmazon)
	p.RetailerLimitRequirement = pr.ask("Any retailer-mandated coverage limit? (blank = none)", p.RetailerLimitRequirement)

	section("Loss history / current coverage")
	p.PriorClaims5yr = pr.askBool("Any prior claims/losses in the last 5 years?", p.PriorClaims5yr)
	if p.PriorClaims5yr {
		p.PriorClaimsDetail = pr.ask("Briefly describe the prior claim(s)", p.PriorClaimsDetail)
	}
	p.CurrentCoverage = pr.ask("Current / prior coverage (blank = none / first-time buyer)", p.CurrentCoverage)

	section("Coverage requested")
	p.GLPerOccurrence = pr.ask("GL limit per occurrence", p.GLPerOccurrence)
	p.GLAggregate = pr.ask("GL aggregate limit", p.GLAggregate)
	p.ProductsCompletedOps = pr.askBool("Include products-completed operations?", p.ProductsCompletedOps)
	p.TradeShowAdditionalInsured = pr.askBool("Need trade-show additional-insured endorsement?", p.TradeShowAdditionalInsured)
	p.WantCyber = pr.askBool("Want a cyber / e-commerce liability quote too?", p.WantCyber)
	p.WantBOP = pr.askBool("Want a BOP (business owners policy) quote too?", p.WantBOP)
	p.EffectiveDate = pr.ask("Desired effective date (YYYY-MM-DD)", p.EffectiveDate)
	p.BudgetAnnualUSD = pr.askInt("Annual budget in USD (0 = market rate)", p.BudgetAnnualUSD)

	return p
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func renderIntake(w io.Writer, res intakeResult, dryRun bool) {
	if dryRun {
		fmt.Fprintf(w, "%s (no file written)\n", bold("Dry run"))
	} else {
		fmt.Fprintf(w, "%s profile saved to %s\n", green("OK"), bold(res.Path))
	}
	if len(res.Problems) > 0 {
		fmt.Fprintf(w, "\n%s the profile is missing %d field(s):\n", yellow("Note:"), len(res.Problems))
		for _, p := range res.Problems {
			fmt.Fprintf(w, "  - %s\n", p)
		}
		fmt.Fprintln(w, "  (Re-run 'biz-insurance-finder intake' to fill them in.)")
	}
	fmt.Fprintf(w, "\nNext: %s\n", bold("biz-insurance-finder match"))
}
