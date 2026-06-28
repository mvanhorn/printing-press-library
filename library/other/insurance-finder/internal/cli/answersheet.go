package cli

import (
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/other/insurance-finder/internal/insurance"
	"github.com/spf13/cobra"
)

func newAnswersheetCmd(f *rootFlags) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "answersheet [provider-id]",
		Short: "Print paste-ready field values for a provider's quote form",
		Long:  "answersheet maps your saved profile to the exact values to paste into a provider's quote form. With no id it uses your top match; with --all it prints one sheet per shortlisted provider.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := f.loadProfile()
			if err != nil {
				return err
			}
			reg, err := f.loadRegistry()
			if err != nil {
				return err
			}

			if all {
				recs := insurance.Shortlist(insurance.Match(p, reg.Providers))
				sheets := make([]insurance.AnswerSheet, 0, len(recs))
				for _, r := range recs {
					sheets = append(sheets, insurance.GenerateAnswerSheet(p, r.Provider))
				}
				return f.emit(cmd, func(w io.Writer) {
					for i, s := range sheets {
						if i > 0 {
							fmt.Fprintln(w)
						}
						renderAnswerSheet(w, s)
					}
				}, sheets)
			}

			prov, err := resolveProvider(f, reg, p, args)
			if err != nil {
				return err
			}
			sheet := insurance.GenerateAnswerSheet(p, prov)
			return f.emit(cmd, func(w io.Writer) { renderAnswerSheet(w, sheet) }, sheet)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Generate an answer sheet for every shortlisted provider")
	return cmd
}

// resolveProvider returns the provider named by args[0], or the top-ranked
// match when no id is given.
func resolveProvider(f *rootFlags, reg insurance.Registry, p insurance.Profile, args []string) (insurance.Provider, error) {
	if len(args) == 1 {
		prov, ok := reg.Get(args[0])
		if !ok {
			return insurance.Provider{}, notFoundErr(fmt.Errorf("no provider with id %q\nhint: run 'insurance-finder providers list'", args[0]))
		}
		return prov, nil
	}
	recs := insurance.Match(p, reg.Providers)
	if len(recs) == 0 {
		return insurance.Provider{}, dataErr(fmt.Errorf("no providers matched"))
	}
	return recs[0].Provider, nil
}

func renderAnswerSheet(w io.Writer, s insurance.AnswerSheet) {
	fmt.Fprintf(w, "%s answer sheet\n", bold(s.ProviderName))
	fmt.Fprintf(w, "%s %s\n\n", dim("Start here:"), s.QuoteURL)
	for _, fld := range s.Fields {
		fmt.Fprintf(w, "  %-38s %s\n", fld.Field+":", fld.Value)
		if fld.Note != "" {
			fmt.Fprintf(w, "      %s %s\n", dim("->"), dim(fld.Note))
		}
	}
}
