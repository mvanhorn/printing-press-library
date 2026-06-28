package cli

import (
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/other/biz-insurance-finder/internal/insurance"
	"github.com/spf13/cobra"
)

func newChecklistCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checklist [provider-id]",
		Short: "Show the manual actions the tool will NOT do for you",
		Long: `checklist lists the human-only steps for a provider: CAPTCHAs, account &
password creation, EIN/SSN, payment, the explicit two-gate submit approval, and
declining optional marketing/SMS consents. The tool guides you through your own
browser - it never fills, submits, or pays for you. With no id it uses your top
match.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := f.loadProfile()
			if err != nil {
				return err
			}
			reg, err := f.loadRegistry()
			if err != nil {
				return err
			}
			prov, err := resolveProvider(f, reg, p, args)
			if err != nil {
				return err
			}
			cl := insurance.GenerateChecklist(prov)
			return f.emit(cmd, func(w io.Writer) { renderChecklist(w, cl) }, cl)
		},
	}
	return cmd
}

func renderChecklist(w io.Writer, cl insurance.ProviderChecklist) {
	fmt.Fprintf(w, "%s manual actions (you do these - the tool will not)\n", bold(cl.ProviderName))
	fmt.Fprintf(w, "%s %s\n\n", dim("Quote form:"), cl.QuoteURL)
	for _, it := range cl.Items {
		mark := "[ ]"
		if it.Required {
			mark = yellow("[!]")
		}
		fmt.Fprintf(w, "  %s %s\n", mark, bold(it.Action))
		fmt.Fprintf(w, "      %s\n", dim(it.Detail))
	}
}
