// Copyright 2026 wandreis and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cheapest listing meeting attribute floors, from the LOCAL store.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/mercadolivre/internal/mlextract"
	"github.com/spf13/cobra"
)

// specPred is one parsed --spec predicate.
type specPred struct {
	name string  // attribute name to match (case-insensitive substring)
	op   string  // "=", ">=", "<="
	valS string  // string operand (for "=")
	valN float64 // numeric operand (for ">=" / "<=")
}

func parseSpecPred(raw string) (specPred, error) {
	raw = strings.TrimSpace(raw)
	for _, op := range []string{">=", "<="} {
		if i := strings.Index(raw, op); i > 0 {
			name := strings.TrimSpace(raw[:i])
			rhs := strings.TrimSpace(raw[i+len(op):])
			n, ok := mlextract.ExtractLeadingNumber(rhs)
			if !ok {
				return specPred{}, fmt.Errorf("spec %q: %q is not numeric", raw, rhs)
			}
			return specPred{name: name, op: op, valN: n}, nil
		}
	}
	if i := strings.Index(raw, "="); i > 0 {
		return specPred{
			name: strings.TrimSpace(raw[:i]),
			op:   "=",
			valS: strings.TrimSpace(raw[i+1:]),
		}, nil
	}
	return specPred{}, fmt.Errorf("spec %q: expected name=value, name>=N, or name<=N", raw)
}

// satisfiedBy reports whether any attribute in attrs meets the predicate.
func (p specPred) satisfiedBy(attrs map[string]string) bool {
	wantName := strings.ToLower(p.name)
	for name, value := range attrs {
		if !strings.Contains(strings.ToLower(name), wantName) {
			continue
		}
		switch p.op {
		case "=":
			if strings.Contains(strings.ToLower(value), strings.ToLower(p.valS)) {
				return true
			}
		case ">=":
			if n, ok := mlextract.ExtractLeadingNumber(value); ok && n >= p.valN {
				return true
			}
		case "<=":
			if n, ok := mlextract.ExtractLeadingNumber(value); ok && n <= p.valN {
				return true
			}
		}
	}
	return false
}

func newNovelCheapestCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string
	var flagSpec []string

	cmd := &cobra.Command{
		Use:   "cheapest",
		Short: "Find the lowest-priced listing that satisfies attribute floors like voltage and power",
		Long: "Scan locally-stored listings (optionally scoped by --query against the listing name) " +
			"joined to their technical-spec attributes, keep only those meeting ALL --spec predicates, " +
			"and return them cheapest-first. Reads only the local store.",
		Example:     "  mercadolivre-pp-cli cheapest --query furadeira --spec voltagem=220V --spec potencia>=700W --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			preds := make([]specPred, 0, len(flagSpec))
			for _, raw := range flagSpec {
				p, err := parseSpecPred(raw)
				if err != nil {
					return usageErr(err)
				}
				preds = append(preds, p)
			}

			s, err := openLocalStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			listings, err := s.ListingsMatching(strings.TrimSpace(flagQuery))
			if err != nil {
				return err
			}

			// Keep listings meeting every predicate; ListingsMatching already
			// returns them price-ascending.
			type cheapestRow struct {
				CatalogID string  `json:"catalog_id"`
				Name      string  `json:"name"`
				Brand     string  `json:"brand"`
				Price     float64 `json:"price"`
				Currency  string  `json:"currency"`
				URL       string  `json:"url"`
			}
			var rows []cheapestRow
			for _, l := range listings {
				if len(preds) > 0 {
					attrs, err := s.ProductAttributes(l.CatalogID)
					if err != nil {
						return err
					}
					if !meetsAll(preds, attrs) {
						continue
					}
				}
				rows = append(rows, cheapestRow{
					CatalogID: l.CatalogID,
					Name:      l.Name,
					Brand:     l.Brand,
					Price:     l.Price,
					Currency:  l.Currency,
					URL:       l.URL,
				})
			}

			if len(rows) == 0 {
				// Honest empty result, never a fabricated match.
				note := "no listing meets the spec floor"
				if len(preds) == 0 {
					note = "no listings stored for this query"
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "%s (%s)\n", note, describeSpecs(flagQuery, flagSpec))
				return printJSONFiltered(cmd.OutOrStdout(), []cheapestRow{}, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&flagQuery, "query", "", "Scope to listings whose name matches this term (LIKE)")
	cmd.Flags().StringSliceVar(&flagSpec, "spec", nil, "Attribute predicate: name=value, name>=N, or name<=N (repeatable)")
	return cmd
}

func meetsAll(preds []specPred, attrs map[string]string) bool {
	for _, p := range preds {
		if !p.satisfiedBy(attrs) {
			return false
		}
	}
	return true
}

func describeSpecs(query string, specs []string) string {
	parts := []string{}
	if strings.TrimSpace(query) != "" {
		parts = append(parts, "query="+query)
	}
	parts = append(parts, specs...)
	if len(parts) == 0 {
		return "no filters"
	}
	return strings.Join(parts, ", ")
}
