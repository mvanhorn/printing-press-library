// PATCH: hand-authored novel feature `menu half-and-half` — composes two
// sandwiches as a share order. Jimmy John's doesn't natively offer half-and-half
// slicing the way pizza shops do, but two-person sharing is a real workflow:
// order both products, ask in-store for them to be halved at pickup or split
// in delivery. Emits the cart structure plus a human note so agents always
// surface the constraint to the user. See .printing-press-patches.json
// patch id "novel-half-and-half".

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type halfAndHalfSide struct {
	ProductID string `json:"product_id"`
	Label     string `json:"label,omitempty"`
}

type halfAndHalfPlan struct {
	Left   halfAndHalfSide `json:"left"`
	Right  halfAndHalfSide `json:"right"`
	Cart   []planCartLine  `json:"cart"`
	Notes  []string        `json:"notes_for_agent"`
	Native bool            `json:"native_half_and_half_supported"`
}

func newMenuHalfAndHalfCmd(flags *rootFlags) *cobra.Command {
	var leftID, rightID, leftLabel, rightLabel string
	cmd := &cobra.Command{
		Use:   "half-and-half",
		Short: "Compose two sandwiches as a shareable half-and-half order",
		Long: `Construct a 2-product share order. Jimmy John's does NOT natively support
half-and-half slicing the way pizza shops do — this command emits both
sandwiches as full products, with an agent-surfaced note that the user
should ask the store to halve and split at pickup or delivery.

Use this when an agent is asked to "split two sandwiches between two people"
and wants a structured cart with the constraint disclosed up front.`,
		Example: `  jimmy-johns-pp-cli menu half-and-half --left 33328641 --right 33328700 --json
  jimmy-johns-pp-cli menu half-and-half --left 33328641 --left-label "Vito" --right 33328700 --right-label "Pepe"`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if leftID == "" || rightID == "" {
				return cmd.Help()
			}
			if leftID == rightID {
				return fmt.Errorf("--left and --right cannot be the same product; use 'order add-items --quantity 2' instead")
			}
			plan := halfAndHalfPlan{
				Left:   halfAndHalfSide{ProductID: leftID, Label: leftLabel},
				Right:  halfAndHalfSide{ProductID: rightID, Label: rightLabel},
				Native: false,
				Cart: []planCartLine{
					{ProductID: leftID, Name: orFallback(leftLabel, leftID), Quantity: 1, Reason: "left half — full sandwich, ask store to halve"},
					{ProductID: rightID, Name: orFallback(rightLabel, rightID), Quantity: 1, Reason: "right half — full sandwich, ask store to halve"},
				},
				Notes: []string{
					"Jimmy John's API does not natively support half-and-half — this orders both as full sandwiches.",
					"At pickup: ask the cashier to halve and split each sandwich. Delivery: add a note to the delivery instructions.",
					"For two-person sharing without splitting, consider ordering one 8\" and one Slim instead.",
				},
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(plan)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Half-and-half order (Jimmy John's does NOT natively support — see notes below):\n")
			for _, l := range plan.Cart {
				fmt.Fprintf(w, "  %dx %s [%s]\n", l.Quantity, l.Name, l.Reason)
			}
			fmt.Fprintln(w)
			for _, n := range plan.Notes {
				fmt.Fprintf(w, "  • %s\n", n)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&leftID, "left", "", "Product ID for the left half (required at runtime)")
	cmd.Flags().StringVar(&rightID, "right", "", "Product ID for the right half (required at runtime)")
	cmd.Flags().StringVar(&leftLabel, "left-label", "", "Optional human-readable label for the left half (e.g. \"Vito\")")
	cmd.Flags().StringVar(&rightLabel, "right-label", "", "Optional human-readable label for the right half (e.g. \"Pepe\")")
	return cmd
}

func orFallback(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
