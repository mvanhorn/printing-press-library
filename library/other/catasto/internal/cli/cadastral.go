// Copyright 2026 roberto-bissanti. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/catasto/internal/comuni"
	"github.com/mvanhorn/printing-press-library/library/other/catasto/internal/ondata"

	"github.com/spf13/cobra"
)

func newCadastralCmd(flags *rootFlags) *cobra.Command {
	var comuneInput, provincia, cap, foglio, particella, sezione, cacheDir string

	cmd := &cobra.Command{
		Use:   "cadastral",
		Short: "Reverse lookup: cadastral reference → WGS84 lon/lat centroid via the ondata Parquet dataset.",
		Long: "Given a comune (by codice belfiore, by name + provincia, or by CAP) plus a foglio and particella, " +
			"returns the WGS84 centroid coordinates of that parcel.\n\n" +
			"Comune can be supplied three ways:\n" +
			"  1. --comune <belfiore>      e.g. --comune H501\n" +
			"  2. --comune <nome> [--provincia <sigla-or-name>]   e.g. --comune Roma --provincia RM\n" +
			"  3. --cap <5-digit CAP>      e.g. --cap 00184\n\n" +
			"For shared names (San Giorgio, Castro, etc.) the resolver returns ErrAmbiguous unless you " +
			"add --provincia. For shared CAPs (smaller comuni often share one) the resolver returns " +
			"ErrAmbiguous unless you also pass --comune or --provincia to narrow it down.\n\n" +
			"Data source: github.com/ondata/dati_catastali — per-region Parquet files precomputed from " +
			"the Agenzia delle Entrate WFS, downloaded and cached on first use.\n\n" +
			"Trentino-Alto-Adige (TN and BZ provinces) is NOT covered — TAA runs autonomous cadastral systems separate from AdE.",
		Example: "  # By codice belfiore (most precise)\n" +
			"  catasto-pp-cli cadastral --comune H501 --foglio 508 --particella B --json\n\n" +
			"  # By name + province\n" +
			"  catasto-pp-cli cadastral --comune Roma --provincia RM --foglio 508 --particella B --json\n\n" +
			"  # By CAP\n" +
			"  catasto-pp-cli cadastral --cap 00184 --foglio 508 --particella B --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if comuneInput == "" && cap == "" && foglio == "" && particella == "" {
				return cmd.Help()
			}
			if foglio == "" {
				return usageErr(fmt.Errorf("--foglio is required"))
			}
			if particella == "" {
				return usageErr(fmt.Errorf("--particella is required"))
			}
			if dryRunOK(flags) {
				return nil
			}

			// Resolve the comune from whichever input the user supplied.
			c, resolveSrc, err := resolveComune(comuneInput, provincia, cap)
			if err != nil {
				if errors.Is(err, comuni.ErrAmbiguous) {
					return usageErr(err)
				}
				return notFoundErr(err)
			}
			comuneCode := c.CodiceCatastale

			ondataCli := ondata.NewClient(cacheDir)
			res, err := ondataCli.LookupParcel(cmd.Context(), comuneCode, foglio, particella)
			if err != nil {
				if errors.Is(err, ondata.ErrNotFound) || errors.Is(err, ondata.ErrComuneNotIndexed) {
					return notFoundErr(err)
				}
				return apiErr(err)
			}

			out := map[string]any{
				"inspire_id":   res.InspireID,
				"comune":       res.Comune,
				"comune_name":  res.ComuneName,
				"codistat":     res.CODISTAT,
				"provincia":    c.Provincia.Nome,
				"sigla":        c.Sigla,
				"regione":      c.Regione.Nome,
				"sezione":      sezione,
				"foglio":       res.Foglio,
				"particella":   res.Particella,
				"lon":          res.Lon,
				"lat":          res.Lat,
				"resolved_via": resolveSrc,
				"source": map[string]any{
					"provider": "ondata/dati_catastali",
					"file":     res.RegionFile,
				},
			}
			data, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().StringVar(&comuneInput, "comune", "", "Codice belfiore (e.g. H501) OR human name (e.g. Roma). Auto-detected by shape.")
	cmd.Flags().StringVar(&provincia, "provincia", "", "Province sigla (RM) or name (Roma). Used to disambiguate when --comune is a name.")
	cmd.Flags().StringVar(&cap, "cap", "", "Italian CAP (5 digits, e.g. 00184). Alternative to --comune; combine with --provincia for shared CAPs.")
	cmd.Flags().StringVar(&foglio, "foglio", "", "Map sheet number (foglio).")
	cmd.Flags().StringVar(&particella, "particella", "", "Parcel number (particella). May be alphanumeric.")
	cmd.Flags().StringVar(&sezione, "sezione", "", "Cadastral section letter (optional; echoed in output but not used for matching).")
	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", "Directory for cached Parquet files (default: per-user OS cache dir).")
	return cmd
}

// resolveComune turns the user's chosen input form into a *comuni.Comune.
// Returns the comune and a string describing which path resolved it
// (useful for output provenance and debugging).
func resolveComune(comuneInput, provincia, cap string) (*comuni.Comune, string, error) {
	switch {
	case cap != "":
		hits, err := comuni.ResolveByCAP(cap)
		if err != nil {
			return nil, "", err
		}
		// Narrow with --comune (name) and/or --provincia when supplied.
		filtered := narrowComuneHits(hits, comuneInput, provincia)
		if len(filtered) == 1 {
			return filtered[0], fmt.Sprintf("cap=%s", cap), nil
		}
		if len(filtered) == 0 {
			return nil, "", fmt.Errorf("%w: cap=%s narrows to 0 candidates with provided filters", comuni.ErrNotFound, cap)
		}
		// Still ambiguous — surface candidates so the user can pick.
		return nil, "", ambiguousCAP(cap, filtered)

	case comuneInput != "":
		input := strings.TrimSpace(comuneInput)
		// Belfiore shape is restrictive: 4 chars, letter + 3 alphanumeric,
		// AND at least one digit (real belfiore codes have digits;
		// pure-letter 4-char strings like "ROMA" are comune names).
		// If shape matches AND the code exists in the dataset, prefer it.
		// Otherwise fall through to name resolution.
		if looksLikeBelfiore(input) {
			if c, err := comuni.ResolveByBelfiore(input); err == nil {
				return c, fmt.Sprintf("belfiore=%s", c.CodiceCatastale), nil
			}
			// Shape-match but no row: fall through to name resolution.
		}
		c, err := comuni.ResolveByName(input, provincia)
		if err != nil {
			return nil, "", err
		}
		if provincia != "" {
			return c, fmt.Sprintf("name=%q provincia=%q", input, provincia), nil
		}
		return c, fmt.Sprintf("name=%q", input), nil

	default:
		return nil, "", fmt.Errorf("specify one of: --comune <belfiore-or-name> or --cap <code>")
	}
}

// narrowComuneHits filters a multi-hit comune slice using optional
// name and province hints. Empty hints are no-ops.
func narrowComuneHits(hits []*comuni.Comune, nameHint, provHint string) []*comuni.Comune {
	if len(hits) == 0 {
		return hits
	}
	out := hits[:0:0]
	for _, c := range hits {
		if nameHint != "" && !strings.EqualFold(c.Nome, nameHint) {
			continue
		}
		if provHint != "" && !strings.EqualFold(c.Sigla, provHint) && !strings.EqualFold(c.Provincia.Nome, provHint) {
			continue
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return hits // no narrowing matched; return original so error can surface candidates
	}
	return out
}

func ambiguousCAP(cap string, hits []*comuni.Comune) error {
	names := make([]string, 0, len(hits))
	for _, c := range hits {
		names = append(names, fmt.Sprintf("%s (%s, %s)", c.Nome, c.Sigla, c.CodiceCatastale))
	}
	return fmt.Errorf("%w: cap=%s matches %d comuni: %s — add --comune <name> or --provincia <code> to disambiguate",
		comuni.ErrAmbiguous, cap, len(hits), strings.Join(names, "; "))
}

func isCodiceBelfiore(s string) bool {
	if len(s) != 4 {
		return false
	}
	if !(s[0] >= 'A' && s[0] <= 'Z') {
		return false
	}
	for i := 1; i < 4; i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

// looksLikeBelfiore is stricter than isCodiceBelfiore: it requires at
// least one digit in positions 1–3, which excludes pure-letter
// 4-character comune names (Roma, Pisa, Cuneo) that would otherwise
// match the shape. Used only for input heuristics; the dataset itself
// is authoritative via comuni.ResolveByBelfiore.
func looksLikeBelfiore(s string) bool {
	upper := strings.ToUpper(strings.TrimSpace(s))
	if !isCodiceBelfiore(upper) {
		return false
	}
	for i := 1; i < 4; i++ {
		if upper[i] >= '0' && upper[i] <= '9' {
			return true
		}
	}
	return false
}
