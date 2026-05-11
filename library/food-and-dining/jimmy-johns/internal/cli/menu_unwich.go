// PATCH: hand-authored novel feature `menu unwich-convert` — pure-local
// conversion of a sandwich's modifier set to an "Unwich" (lettuce-wrap)
// variant. Works against synced product modifiers or a JSON file piped via
// --stdin. No live API call required. See .printing-press-patches.json
// patch id "novel-unwich-convert".

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// unwichModifier is the shape we emit — a normalized modifier delta the user
// (or an agent composing a cart-add request) can splice into /api/order/batchItems.
type unwichModifier struct {
	ProductID    string   `json:"product_id"`
	BreadGroup   string   `json:"bread_modifier_group"`
	OriginalBread string  `json:"original_bread"`
	UnwichOption string   `json:"unwich_option"`
	Diff         []string `json:"diff"`
}

// modifierGroupShape is a permissive shape over the /api/products/{id}/modifiers
// response. We only need to find the bread/wrapper group and the unwich option.
type modifierGroupShape struct {
	GroupID   any              `json:"groupId"`
	Name      string           `json:"name"`
	Modifiers []map[string]any `json:"modifiers"`
}

func newMenuUnwichConvertCmd(flags *rootFlags) *cobra.Command {
	var fromFile, productID, currentBread string
	cmd := &cobra.Command{
		Use:   "unwich-convert",
		Short: "Convert a sandwich's bread choice to an Unwich (lettuce wrap)",
		Long: `Take the modifier set for a sandwich product and emit the modifier delta needed
to convert it to an Unwich (lettuce wrap). Pure-local computation — no API
call required if you pipe modifier JSON via --stdin or --from-file.

Pairs naturally with 'menu product-modifiers <productId>'.`,
		Example: `  jimmy-johns-pp-cli menu product-modifiers 33328641 --json > /tmp/mods.json
  jimmy-johns-pp-cli menu unwich-convert --from-file /tmp/mods.json --product-id 33328641 --current-bread "8-inch"

  # Or in one pipeline:
  jimmy-johns-pp-cli menu product-modifiers 33328641 --json | \
    jimmy-johns-pp-cli menu unwich-convert --stdin --product-id 33328641`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if productID == "" {
				return cmd.Help()
			}
			var raw []byte
			var err error
			switch {
			case fromFile != "":
				raw, err = os.ReadFile(fromFile)
			default:
				// PATCH: skip io.ReadAll when stdin is a TTY. Otherwise
				// `menu unwich-convert --product-id 33328641` (no pipe)
				// hangs forever waiting for Ctrl-D instead of falling
				// through to the empty-input error. Same fix as order_plan.go.
				if !readerIsTerminal(cmd.InOrStdin()) {
					raw, err = io.ReadAll(cmd.InOrStdin())
				}
			}
			if err != nil {
				return fmt.Errorf("reading modifier input: %w", err)
			}
			if len(raw) == 0 {
				return fmt.Errorf("no modifier data on stdin or --from-file; run 'menu product-modifiers <id> --json' first")
			}
			var groups []modifierGroupShape
			// Tolerate either a bare array or an object wrapping `results`/`modifiers`.
			if err := json.Unmarshal(raw, &groups); err != nil {
				var wrap struct {
					Results   []modifierGroupShape `json:"results"`
					Modifiers []modifierGroupShape `json:"modifiers"`
					Data      []modifierGroupShape `json:"data"`
				}
				if e2 := json.Unmarshal(raw, &wrap); e2 != nil {
					return fmt.Errorf("parsing modifier JSON: %w", err)
				}
				switch {
				case len(wrap.Results) > 0:
					groups = wrap.Results
				case len(wrap.Modifiers) > 0:
					groups = wrap.Modifiers
				case len(wrap.Data) > 0:
					groups = wrap.Data
				}
			}

			result := unwichModifier{
				ProductID:     productID,
				OriginalBread: currentBread,
				UnwichOption:  "Unwich (Lettuce Wrap)",
			}
			for _, g := range groups {
				name := strings.ToLower(g.Name)
				if !strings.Contains(name, "bread") && !strings.Contains(name, "wrap") {
					continue
				}
				result.BreadGroup = g.Name
				for _, m := range g.Modifiers {
					mn := strings.ToLower(fmt.Sprint(m["name"]))
					if strings.Contains(mn, "unwich") || strings.Contains(mn, "lettuce") {
						if id, ok := m["modifierId"]; ok {
							result.Diff = append(result.Diff, fmt.Sprintf("set %s -> modifierId=%v", g.Name, id))
						} else {
							result.Diff = append(result.Diff, fmt.Sprintf("set %s -> %v", g.Name, m["name"]))
						}
					}
				}
				break
			}

			if result.BreadGroup == "" {
				return fmt.Errorf("no bread/wrap modifier group found for product %s — is this a non-sandwich item?", productID)
			}
			if len(result.Diff) == 0 {
				result.Diff = []string{"no Unwich option found in this product's modifier set"}
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Product %s → Unwich (Lettuce Wrap)\n", productID)
			fmt.Fprintf(w, "Bread group: %s\n", result.BreadGroup)
			if currentBread != "" {
				fmt.Fprintf(w, "Was: %s\n", currentBread)
			}
			for _, d := range result.Diff {
				fmt.Fprintf(w, "  %s\n", d)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Path to JSON file containing modifier data (alternative to stdin)")
	cmd.Flags().StringVar(&productID, "product-id", "", "Numeric product ID being converted (required at runtime)")
	cmd.Flags().StringVar(&currentBread, "current-bread", "", "Optional: human-readable current bread choice shown in plain output")
	// --stdin flag is documented in --help text; we always fall through to stdin when --from-file isn't set.
	cmd.Flags().Bool("stdin", false, "Read modifier JSON from stdin (default if --from-file unset)")
	return cmd
}
