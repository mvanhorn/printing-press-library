// internal/cli/lookup.go
package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/catalog"
	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/domain"
	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/provider"
	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/render"
	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/resolve"
	"github.com/spf13/cobra"
)

var nameCols = []render.Column{
	{Header: "Runner", Value: func(r domain.Result) string { return r.Runner }},
	{Header: "Bib", Value: func(r domain.Result) string { return r.Bib }},
	{Header: "Net time", Value: func(r domain.Result) string { return r.NetTime }},
}

func newLookupCmd(reg *provider.Registry, entries []catalog.Entry) *cobra.Command {
	var year int
	var date string
	var asJSON bool
	var name string

	cmd := &cobra.Command{
		Use:   "lookup [race-name] [bib]",
		Short: "Resolve a race and return the result for a bib or name",
		Example: `  running-race-results-pp-cli lookup "nyc marathon" 12345 --year 2024
  running-race-results-pp-cli lookup "brooklyn half" --name "Sample Runner" --json`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			race := args[0]
			bib := ""
			if len(args) == 2 {
				bib = args[1]
			}
			if (bib == "") == (name == "") {
				return fmt.Errorf("provide exactly one of <bib> or --name")
			}

			if year == 0 && date != "" {
				t, perr := time.Parse("2006-01-02", date)
				if perr != nil {
					return fmt.Errorf("invalid --date %q (want YYYY-MM-DD): %w", date, perr)
				}
				year = t.Year()
			}

			cands, err := resolve.Resolve(entries, race, year)
			if err != nil {
				return fmt.Errorf("%w: %q", err, race)
			}
			// Ambiguity: two candidates within 0.05 of each other.
			if len(cands) > 1 && cands[0].Score-cands[1].Score < 0.05 {
				if asJSON {
					return fmt.Errorf("ambiguous race %q; candidates: %s, %s",
						race, cands[0].Event.Name, cands[1].Event.Name)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Multiple matches — refine with --year or a fuller name:")
				for _, c := range cands {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%d)\n", c.Event.Name, c.Event.Year)
				}
				return nil
			}

			ev := cands[0].Event
			p, ok := reg.Get(ev.Provider)
			if !ok {
				return fmt.Errorf("no adapter registered for provider %q", ev.Provider)
			}

			if name != "" {
				ns, ok := p.(provider.NameSearcher)
				if !ok {
					return fmt.Errorf("name search not supported for provider %q; use a bib", ev.Provider)
				}
				matches, err := ns.SearchByName(cmd.Context(), ev, name)
				if err != nil {
					return err
				}
				if asJSON {
					return render.JSONValue(cmd.OutOrStdout(), matches)
				}
				switch len(matches) {
				case 0:
					return fmt.Errorf("no runner matching %q in %s", name, ev.Name)
				case 1:
					return render.Table(cmd.OutOrStdout(), matches[0])
				default:
					fmt.Fprintln(cmd.OutOrStdout(), "Multiple matches — refine with the bib:")
					return render.List(cmd.OutOrStdout(), matches, nameCols)
				}
			}

			res, err := p.Lookup(cmd.Context(), ev, bib)
			if err != nil {
				return err
			}
			if asJSON {
				return render.JSON(cmd.OutOrStdout(), res)
			}
			return render.Table(cmd.OutOrStdout(), res)
		},
	}
	cmd.Flags().IntVar(&year, "year", 0, "race edition year")
	cmd.Flags().StringVar(&date, "date", "", "race date YYYY-MM-DD (year is derived if --year unset)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.Flags().StringVar(&name, "name", "", "look up by runner name instead of bib")
	return cmd
}
