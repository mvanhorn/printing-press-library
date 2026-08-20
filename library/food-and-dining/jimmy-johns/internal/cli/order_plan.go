// PATCH: hand-authored novel feature `order plan` — sized cart suggestion for
// group orders. Pure-local computation. Reads menu products from
// stdin/--from-file (the shape of `menu products --json`) or uses a sensible
// default catalog when no products are provided. See .printing-press-patches.json
// patch id "novel-order-plan".

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type planMenuItem struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Price    float64 `json:"price"`
	Calories int     `json:"calories"`
}

type planCartLine struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	Reason    string `json:"reason"`
}

type orderPlan struct {
	People        int            `json:"people"`
	Dietary       []string       `json:"dietary,omitempty"`
	Lines         []planCartLine `json:"lines"`
	EstSubtotal   float64        `json:"estimated_subtotal"`
	NotesForAgent []string       `json:"notes_for_agent,omitempty"`
}

// defaultCatalog is the fallback menu when no products are piped in — the
// canonical JJ subset most groups order from. Item IDs are placeholders; agents
// should substitute real IDs from `menu products --json` before submitting.
var defaultCatalog = []planMenuItem{
	{ID: "ph-pepe", Name: "#1 The Pepe", Category: "sandwich", Price: 7.49, Calories: 580},
	{ID: "ph-turkey-tom", Name: "#4 Turkey Tom", Category: "sandwich", Price: 7.49, Calories: 530},
	{ID: "ph-vito", Name: "#5 Vito", Category: "sandwich", Price: 7.99, Calories: 760},
	{ID: "ph-italian-night", Name: "#9 Italian Night Club", Category: "sandwich", Price: 9.49, Calories: 930},
	{ID: "ph-veggie", Name: "#6 Veggie", Category: "sandwich", Price: 7.49, Calories: 460},
	{ID: "ph-chips", Name: "Chips", Category: "side", Price: 1.99, Calories: 220},
	{ID: "ph-cookie", Name: "Cookie", Category: "dessert", Price: 1.99, Calories: 410},
	{ID: "ph-drink", Name: "Fountain Drink", Category: "drink", Price: 2.49, Calories: 0},
}

func newOrderPlanCmd(flags *rootFlags) *cobra.Command {
	var people int
	var dietary string
	var fromFile string
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Suggest a sized cart for a group order (subs + sides + drinks)",
		Long: `Compose a sized cart for N people. Pure-local computation: reads the menu
product catalog from stdin or --from-file (the output of 'menu products --json'),
filters by --dietary, and emits a quantity-tagged cart plan agents can convert
to /api/order/batchItems calls.

Sizing heuristic:
  - 1 sandwich per person (rounded up; ~1.2 sandwiches/person for buffer)
  - 1 side per 2 people
  - 1 cookie per 3 people
  - 1 drink per 2 people

If no menu data is piped in, a canonical JJ subset is used with placeholder
product IDs (agents should substitute real IDs before submitting).`,
		Example: `  jimmy-johns-pp-cli order plan --people 6 --json
  jimmy-johns-pp-cli menu products --json | jimmy-johns-pp-cli order plan --people 8 --dietary vegetarian --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if people <= 0 {
				return cmd.Help()
			}

			catalog, err := loadPlanCatalog(cmd.InOrStdin(), fromFile)
			if err != nil {
				return err
			}
			if len(catalog) == 0 {
				catalog = defaultCatalog
			}

			plan := composePlan(people, normalizeDietary(dietary), catalog)
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(plan)
			}
			renderPlanTable(cmd.OutOrStdout(), plan)
			return nil
		},
	}
	cmd.Flags().IntVar(&people, "people", 0, "Number of people the cart should feed (required at runtime)")
	cmd.Flags().StringVar(&dietary, "dietary", "", "Comma-separated dietary filters: vegetarian, no-meat, no-pork")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Path to a 'menu products --json' export (alternative to stdin)")
	return cmd
}

func normalizeDietary(in string) []string {
	if in == "" {
		return nil
	}
	parts := strings.Split(in, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func loadPlanCatalog(stdin io.Reader, fromFile string) ([]planMenuItem, error) {
	var raw []byte
	var err error
	if fromFile != "" {
		raw, err = os.ReadFile(fromFile)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", fromFile, err)
		}
	} else if stdin != nil && !readerIsTerminal(stdin) {
		// PATCH: only consume stdin when it's a pipe or file. Reading from a TTY
		// would block forever on os.Stdin (io.ReadAll waits for EOF, which never
		// arrives without Ctrl-D), making `order plan --people 6 --json` hang
		// silently when invoked interactively.
		raw, _ = io.ReadAll(stdin)
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var catalog []planMenuItem
	if err := json.Unmarshal(raw, &catalog); err != nil {
		// Tolerate wrapped shapes
		var wrap struct {
			Results []planMenuItem `json:"results"`
			Data    []planMenuItem `json:"data"`
		}
		if e2 := json.Unmarshal(raw, &wrap); e2 != nil {
			return nil, fmt.Errorf("parsing catalog JSON: %w", err)
		}
		if len(wrap.Results) > 0 {
			catalog = wrap.Results
		} else if len(wrap.Data) > 0 {
			catalog = wrap.Data
		}
	}
	return catalog, nil
}

// readerIsTerminal reports whether r is an *os.File backed by a character device
// (a TTY). Used to skip io.ReadAll on interactive stdin, which would otherwise
// block until Ctrl-D. Anything that isn't a real *os.File (bytes.Buffer in
// tests, a pipe wrapper, a network conn) is treated as non-terminal so the
// caller still drains it. Distinct from helpers.go's `isTerminal(io.Writer)`
// because cobra's InOrStdin/OutOrStdout split readers and writers.
func readerIsTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// composePlan builds a sized cart from the catalog. Pure function — easy to test.
func composePlan(people int, dietary []string, catalog []planMenuItem) orderPlan {
	plan := orderPlan{People: people, Dietary: dietary}

	sandwiches := filterCategory(catalog, "sandwich")
	sandwiches = applyDietary(sandwiches, dietary)
	sides := filterCategory(catalog, "side")
	drinks := filterCategory(catalog, "drink")
	desserts := filterCategory(catalog, "dessert")

	// Sandwich math: ceil(people * 1.2) for buffer.
	sandwichCount := int(math.Ceil(float64(people) * 1.2))
	if sandwichCount < 1 {
		sandwichCount = 1
	}
	// Distribute across top 3 sandwich options (or fewer if catalog is thin).
	maxVariety := 3
	if len(sandwiches) < maxVariety {
		maxVariety = len(sandwiches)
	}
	if maxVariety == 0 {
		plan.NotesForAgent = append(plan.NotesForAgent, "no matching sandwiches in catalog after dietary filter — supply --dietary 'vegetarian' or extend the catalog")
		return plan
	}
	per := sandwichCount / maxVariety
	rem := sandwichCount % maxVariety
	for i := 0; i < maxVariety; i++ {
		q := per
		if i < rem {
			q++
		}
		plan.Lines = append(plan.Lines, planCartLine{
			ProductID: sandwiches[i].ID,
			Name:      sandwiches[i].Name,
			Quantity:  q,
			Reason:    fmt.Sprintf("sandwich for %d guests, variety %d/%d", people, i+1, maxVariety),
		})
		plan.EstSubtotal += float64(q) * sandwiches[i].Price
	}

	// Sides: 1 per 2 people
	if sideCount := (people + 1) / 2; sideCount > 0 && len(sides) > 0 {
		plan.Lines = append(plan.Lines, planCartLine{
			ProductID: sides[0].ID, Name: sides[0].Name, Quantity: sideCount,
			Reason: fmt.Sprintf("1 side per 2 guests (%d total)", sideCount),
		})
		plan.EstSubtotal += float64(sideCount) * sides[0].Price
	}
	// Cookies: 1 per 3 people
	if cookieCount := (people + 2) / 3; cookieCount > 0 && len(desserts) > 0 {
		plan.Lines = append(plan.Lines, planCartLine{
			ProductID: desserts[0].ID, Name: desserts[0].Name, Quantity: cookieCount,
			Reason: fmt.Sprintf("1 cookie per 3 guests (%d total)", cookieCount),
		})
		plan.EstSubtotal += float64(cookieCount) * desserts[0].Price
	}
	// Drinks: 1 per 2 people
	if drinkCount := (people + 1) / 2; drinkCount > 0 && len(drinks) > 0 {
		plan.Lines = append(plan.Lines, planCartLine{
			ProductID: drinks[0].ID, Name: drinks[0].Name, Quantity: drinkCount,
			Reason: fmt.Sprintf("1 drink per 2 guests (%d total)", drinkCount),
		})
		plan.EstSubtotal += float64(drinkCount) * drinks[0].Price
	}

	// Round subtotal to two decimals.
	plan.EstSubtotal = math.Round(plan.EstSubtotal*100) / 100

	if len(plan.NotesForAgent) == 0 && hasPlaceholderIDs(plan.Lines) {
		plan.NotesForAgent = append(plan.NotesForAgent,
			"product IDs prefixed with 'ph-' are placeholders from the built-in fallback catalog — run 'menu products --json | order plan ...' for real IDs",
			"prices are placeholder estimates; revalidate via the cart endpoint before submitting")
	}
	return plan
}

func filterCategory(items []planMenuItem, cat string) []planMenuItem {
	out := make([]planMenuItem, 0, len(items))
	for _, it := range items {
		if strings.EqualFold(it.Category, cat) {
			out = append(out, it)
		}
	}
	// Sort by price ascending so cheapest options anchor the plan.
	sort.SliceStable(out, func(i, j int) bool { return out[i].Price < out[j].Price })
	return out
}

func applyDietary(items []planMenuItem, dietary []string) []planMenuItem {
	if len(dietary) == 0 {
		return items
	}
	// Apply name-based heuristics. Real JJ menu data may carry allergen/ingredient tags;
	// when present, use them instead — but the placeholder catalog only has names.
	noPork := contains(dietary, "no-pork") || contains(dietary, "halal") || contains(dietary, "kosher")
	noMeat := contains(dietary, "vegetarian") || contains(dietary, "no-meat") || contains(dietary, "vegan")
	out := items[:0]
	for _, it := range items {
		n := strings.ToLower(it.Name)
		if noMeat && !strings.Contains(n, "veggie") {
			continue
		}
		if noPork && (strings.Contains(n, "pepe") || strings.Contains(n, "italian") || strings.Contains(n, "vito") || strings.Contains(n, "pork")) {
			continue
		}
		out = append(out, it)
	}
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func hasPlaceholderIDs(lines []planCartLine) bool {
	for _, l := range lines {
		if strings.HasPrefix(l.ProductID, "ph-") {
			return true
		}
	}
	return false
}

func renderPlanTable(w io.Writer, plan orderPlan) {
	fmt.Fprintf(w, "Order plan for %d people", plan.People)
	if len(plan.Dietary) > 0 {
		fmt.Fprintf(w, " (dietary: %s)", strings.Join(plan.Dietary, ", "))
	}
	fmt.Fprintln(w)
	for _, l := range plan.Lines {
		fmt.Fprintf(w, "  %dx %-30s  [%s]\n", l.Quantity, l.Name, l.Reason)
	}
	fmt.Fprintf(w, "\nEstimated subtotal: $%.2f\n", plan.EstSubtotal)
	for _, n := range plan.NotesForAgent {
		fmt.Fprintf(w, "Note: %s\n", n)
	}
}
