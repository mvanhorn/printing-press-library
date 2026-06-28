package cli

import (
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/other/insurance-finder/internal/insurance"
	"github.com/spf13/cobra"
)

func newProvidersCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "List or inspect the editable provider registry",
		Long:  "providers prints the registry that the matcher draws from. Edit providers.json (or point --providers at your own copy) to change appetites, add carriers, or fix details.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listProviders(cmd, f)
		},
	}
	cmd.AddCommand(newProvidersListCmd(f))
	cmd.AddCommand(newProvidersShowCmd(f))
	return cmd
}

func newProvidersListCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all providers in the registry",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return listProviders(cmd, f) },
	}
}

func newProvidersShowCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show one provider's full registry entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := f.loadRegistry()
			if err != nil {
				return err
			}
			prov, ok := reg.Get(args[0])
			if !ok {
				return notFoundErr(fmt.Errorf("no provider with id %q\nhint: run 'insurance-finder providers list' to see ids", args[0]))
			}
			return f.emit(cmd, func(w io.Writer) { renderProviderDetail(w, prov) }, prov)
		},
	}
}

// providerRow is the compact list shape (also drives CSV/plain output).
type providerRow struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Channel    string `json:"quote_channel"`
	Instant    bool   `json:"instant_quote"`
	Unverified bool   `json:"unverified"`
	AMBest     string `json:"am_best"`
}

func listProviders(cmd *cobra.Command, f *rootFlags) error {
	reg, err := f.loadRegistry()
	if err != nil {
		return err
	}
	rows := make([]providerRow, 0, len(reg.Providers))
	for _, p := range reg.SortedByName() {
		rows = append(rows, providerRow{p.ID, p.Name, p.Type, p.QuoteChannel, p.InstantQuote, p.Unverified, p.AMBest})
	}
	return f.emit(cmd, func(w io.Writer) {
		fmt.Fprintf(w, "%s (%d, source: %s)\n\n", bold("Provider registry"), len(rows), reg.Source)
		for _, r := range rows {
			flag := ""
			if r.Unverified {
				flag = " " + yellow("[unverified]")
			}
			fmt.Fprintf(w, "  %-18s %s%s\n", r.ID, r.Name, flag)
			fmt.Fprintf(w, "    %s\n", dim(fmt.Sprintf("%s | %s | instant=%t | AM Best %s", r.Type, r.Channel, r.Instant, r.AMBest)))
		}
	}, rows)
}

func renderProviderDetail(w io.Writer, p insurance.Provider) {
	fmt.Fprintf(w, "%s  (%s)\n", bold(p.Name), p.ID)
	if p.Unverified {
		fmt.Fprintf(w, "%s\n", yellow("UNVERIFIED - confirm details before relying on this entry."))
	}
	fmt.Fprintf(w, "  Type:     %s\n", p.Type)
	fmt.Fprintf(w, "  Channel:  %s\n", p.QuoteChannel)
	fmt.Fprintf(w, "  AM Best:  %s\n", p.AMBest)
	fmt.Fprintf(w, "  URL:      %s\n", p.URL)
	if p.QuoteURL != "" {
		fmt.Fprintf(w, "  Quote:    %s\n", p.QuoteURL)
	}
	fmt.Fprintf(w, "  Appetite: importer=%s private-label=%s manufacturer=%s retail=%s service=%s\n",
		dash(p.Appetite.Importer), dash(p.Appetite.PrivateLabel), dash(p.Appetite.Manufacturer), dash(p.Appetite.Retail), dash(p.Appetite.Service))
	if p.Notes != "" {
		fmt.Fprintf(w, "  Notes:    %s\n", p.Notes)
	}
	if p.SubmitNote != "" {
		fmt.Fprintf(w, "  Submit:   %s\n", p.SubmitNote)
	}
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
