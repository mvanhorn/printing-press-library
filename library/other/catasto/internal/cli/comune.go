// Copyright 2026 roberto-bissanti. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/catasto/internal/comuni"

	"github.com/spf13/cobra"
)

func newComuneCmd(flags *rootFlags) *cobra.Command {
	var byBelfiore, byName, byProvincia, byCAP string

	cmd := &cobra.Command{
		Use:   "comune",
		Short: "Resolve an Italian comune by codice belfiore, name+provincia, or CAP. Embedded dataset, no network.",
		Long: "Looks up a comune in the embedded ISTAT+ANCI dataset (matteocontrini/comuni-json snapshot) " +
			"and prints its metadata: codice belfiore, ISTAT code, provincia, regione, CAPs, population.\n\n" +
			"Three input modes (use one):\n" +
			"  --belfiore <code>           e.g. --belfiore H501  → unique by definition\n" +
			"  --name <nome> [--provincia] e.g. --name Roma     → may need --provincia for shared names\n" +
			"  --cap <5-digit>             e.g. --cap 00184      → may match multiple comuni; returns all\n\n" +
			"Useful as a pre-flight resolver before `catasto-pp-cli cadastral` and as a free " +
			"name↔codice belfiore↔CAP lookup with zero network calls.",
		Example: "  catasto-pp-cli comune --belfiore H501 --json\n" +
			"  catasto-pp-cli comune --name Roma --provincia RM --json\n" +
			"  catasto-pp-cli comune --cap 00184 --json\n" +
			"  catasto-pp-cli comune --name Forlì --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			modes := 0
			if byBelfiore != "" {
				modes++
			}
			if byName != "" {
				modes++
			}
			if byCAP != "" {
				modes++
			}
			if modes == 0 {
				return cmd.Help()
			}
			if modes > 1 {
				return usageErr(fmt.Errorf("specify only one of --belfiore / --name / --cap (got %d)", modes))
			}
			if dryRunOK(flags) {
				return nil
			}
			switch {
			case byBelfiore != "":
				c, err := comuni.ResolveByBelfiore(byBelfiore)
				if err != nil {
					return notFoundErr(err)
				}
				return emitComune(cmd, flags, c)
			case byName != "":
				c, err := comuni.ResolveByName(byName, byProvincia)
				if err != nil {
					if errors.Is(err, comuni.ErrAmbiguous) {
						return usageErr(err)
					}
					return notFoundErr(err)
				}
				return emitComune(cmd, flags, c)
			case byCAP != "":
				hits, err := comuni.ResolveByCAP(byCAP)
				if err != nil {
					return notFoundErr(err)
				}
				return emitComuneSlice(cmd, flags, hits)
			}
			return cmd.Help()
		},
	}

	cmd.Flags().StringVar(&byBelfiore, "belfiore", "", "Codice catastale / belfiore (4 chars).")
	cmd.Flags().StringVar(&byName, "name", "", "Comune name (accent-insensitive). Use --provincia for shared names.")
	cmd.Flags().StringVar(&byProvincia, "provincia", "", "Province sigla (RM) or full name (Roma). Disambiguates --name.")
	cmd.Flags().StringVar(&byCAP, "cap", "", "Italian CAP (5 digits). May return multiple comuni.")
	return cmd
}

func emitComune(cmd *cobra.Command, flags *rootFlags, c *comuni.Comune) error {
	data, _ := json.Marshal(comuneToOutput(c))
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}

func emitComuneSlice(cmd *cobra.Command, flags *rootFlags, cs []*comuni.Comune) error {
	out := make([]map[string]any, 0, len(cs))
	for _, c := range cs {
		out = append(out, comuneToOutput(c))
	}
	data, _ := json.Marshal(out)
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}

func comuneToOutput(c *comuni.Comune) map[string]any {
	return map[string]any{
		"nome":             c.Nome,
		"codice_belfiore":  c.CodiceCatastale,
		"codice_istat":     c.Codice,
		"provincia":        c.Provincia.Nome,
		"provincia_sigla":  c.Sigla,
		"provincia_codice": c.Provincia.Codice,
		"regione":          c.Regione.Nome,
		"cap":              c.CAP,
		"popolazione":      c.Popolazione,
	}
}
