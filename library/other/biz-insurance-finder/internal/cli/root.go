package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/mvanhorn/printing-press-library/library/other/biz-insurance-finder/internal/config"
	"github.com/mvanhorn/printing-press-library/library/other/biz-insurance-finder/internal/insurance"
	"github.com/spf13/cobra"
)

const version = "0.1.0"

// rootFlags holds the global, persistent flags shared by every command.
type rootFlags struct {
	asJSON       bool
	compact      bool
	csv          bool
	plain        bool
	quiet        bool
	noColor      bool
	human        bool
	agent        bool
	noInput      bool
	yes          bool
	dryRun       bool
	selectFields string
	profilePath  string
	providers    string
}

// RootCmd builds the command tree (used by Execute and by tests).
func RootCmd() *cobra.Command {
	var f rootFlags
	return newRootCmd(&f)
}

// Execute runs the CLI.
func Execute() error {
	return RootCmd().Execute()
}

func newRootCmd(f *rootFlags) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "biz-insurance-finder",
		Short: "Guided assistant to find and apply for US small-business commercial insurance (General Liability first).",
		Long: `biz-insurance-finder is a guided assistant that helps a US small business find and
apply for commercial insurance, starting with General Liability.

It interviews you once, saves a reusable applicant profile, then for each matched
provider gives you the quote-start URL, a paste-ready answer sheet, and a checklist
of the manual steps it will NOT do for you (CAPTCHAs, accounts, EIN/SSN, payment,
and the final submit). It guides you through your own browser - it never fills,
submits, or pays on your behalf.

Key routing rule: an importer / private-label / manufacturer ("deemed
manufacturer") is sent to specialty markets, not the mainstream instant-quote
carriers that decline that class.

Typical flow:
  biz-insurance-finder intake          # one-time interview -> profile.json
  biz-insurance-finder match           # ranked shortlist of providers
  biz-insurance-finder guide           # per-provider URL + answer sheet + checklist

Agent mode: add --agent to any command for JSON + non-interactive output.
Health check: biz-insurance-finder doctor`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	rootCmd.SetVersionTemplate("biz-insurance-finder {{ .Version }}\n")

	pf := rootCmd.PersistentFlags()
	pf.BoolVar(&f.asJSON, "json", false, "Output as JSON")
	pf.BoolVar(&f.compact, "compact", false, "Return only key fields for minimal token usage")
	pf.BoolVar(&f.csv, "csv", false, "Output as CSV (for array/table results)")
	pf.BoolVar(&f.plain, "plain", false, "Output as plain tab-separated text")
	pf.BoolVar(&f.quiet, "quiet", false, "Suppress output; exit code communicates result")
	pf.BoolVar(&f.noColor, "no-color", false, "Disable colored output")
	pf.BoolVar(&f.human, "human-friendly", false, "Enable colored, rich human formatting")
	pf.BoolVar(&f.agent, "agent", false, "Agent defaults (--json --compact --no-input --no-color --yes)")
	pf.BoolVar(&f.noInput, "no-input", false, "Never prompt interactively (for CI/agents)")
	pf.BoolVar(&f.yes, "yes", false, "Skip confirmation prompts")
	pf.BoolVar(&f.dryRun, "dry-run", false, "Show what would happen without writing files")
	pf.StringVar(&f.selectFields, "select", "", "Comma-separated fields to include in JSON output")
	pf.StringVar(&f.profilePath, "profile", "", "Path to the applicant profile JSON (default: ./profile.json or $BIZ_INSURANCE_FINDER_PROFILE)")
	pf.StringVar(&f.providers, "providers", "", "Path to a custom provider registry JSON (default: ./providers.json or embedded)")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if f.agent {
			f.asJSON, f.compact, f.noInput, f.yes, f.noColor = true, true, true, true, true
		}
		// Wire color state to the package-level switches used by helpers.go.
		noColor = f.noColor
		humanFriendly = f.human
		return nil
	}

	rootCmd.AddCommand(newIntakeCmd(f))
	rootCmd.AddCommand(newProfileCmd(f))
	rootCmd.AddCommand(newProvidersCmd(f))
	rootCmd.AddCommand(newMatchCmd(f))
	rootCmd.AddCommand(newAnswersheetCmd(f))
	rootCmd.AddCommand(newChecklistCmd(f))
	rootCmd.AddCommand(newWarningsCmd(f))
	rootCmd.AddCommand(newGuideCmd(f))
	rootCmd.AddCommand(newFillPlanCmd(f))
	rootCmd.AddCommand(newDoctorCmd(f))
	rootCmd.AddCommand(newVersionCmd())
	return rootCmd
}

// emit routes a value to the right output format. For the default (human)
// path it calls the provided renderer; every other path goes through JSON/CSV/TSV.
func (f *rootFlags) emit(cmd *cobra.Command, human func(io.Writer), v any) error {
	w := cmd.OutOrStdout()
	if f.quiet {
		return nil
	}
	asJSON := f.asJSON
	if !asJSON && !f.plain && !f.csv && f.selectFields == "" && !isTerminal(w) {
		asJSON = true // piped output defaults to JSON
	}
	switch {
	case f.csv:
		return emitCSV(w, v)
	case f.plain:
		return emitTSV(w, v)
	case f.selectFields != "":
		return emitJSON(w, v, f.selectFields, f.compact)
	case asJSON:
		return emitJSON(w, v, "", f.compact)
	default:
		human(w)
		return nil
	}
}

func (f *rootFlags) loadRegistry() (insurance.Registry, error) {
	reg, err := insurance.LoadRegistry(config.ProvidersPath(f.providers))
	if err != nil {
		return insurance.Registry{}, dataErr(fmt.Errorf("%w\nhint: check your providers.json, or omit --providers to use the built-in registry", err))
	}
	return reg, nil
}

func (f *rootFlags) resolvedProfilePath() string {
	return config.ProfilePath(f.profilePath)
}

func (f *rootFlags) loadProfile() (insurance.Profile, error) {
	path := f.resolvedProfilePath()
	if _, err := os.Stat(path); err != nil {
		return insurance.Profile{}, notFoundErr(fmt.Errorf("no profile at %q\nhint: run 'biz-insurance-finder intake' to create one", path))
	}
	p, err := insurance.LoadProfile(path)
	if err != nil {
		return insurance.Profile{}, dataErr(err)
	}
	return p, nil
}
