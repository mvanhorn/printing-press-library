package cli

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const browserQuoteSchemaVersion = 1

//go:embed testdata/ifood_browser_quote.json
var browserExampleQuoteJSON []byte

var defaultBrowserItems = []string{
	"alcool em gel",
	"guardanapo",
	"papel toalha",
	"garrafa grande para beber agua",
	"esponja",
	"extensao eletrica ou filtro de linha",
}

type browserQuoteObservation struct {
	SchemaVersion  int                        `json:"schema_version"`
	CapturedAt     string                     `json:"captured_at,omitempty"`
	RequestedItems []browserRequestedItem     `json:"requested_items"`
	Markets        []browserMarketObservation `json:"markets"`
}

type browserRequestedItem struct {
	Term     string `json:"term"`
	Quantity int    `json:"quantity"`
}

type browserMarketObservation struct {
	ID             string                   `json:"id,omitempty"`
	Name           string                   `json:"name"`
	Rating         float64                  `json:"rating"`
	DeliveryFeeBRL *float64                 `json:"delivery_fee_brl,omitempty"`
	Items          []browserItemObservation `json:"items"`
}

type browserItemObservation struct {
	Term        string  `json:"term"`
	ProductID   string  `json:"product_id,omitempty"`
	ProductName string  `json:"product_name"`
	UnitPrice   float64 `json:"unit_price_brl"`
	Quantity    int     `json:"quantity,omitempty"`
	Available   bool    `json:"available"`
}

type browserValidatedMarket struct {
	ID                string                   `json:"id,omitempty"`
	Name              string                   `json:"name"`
	Rating            float64                  `json:"rating"`
	Eligible          bool                     `json:"eligible"`
	Complete          bool                     `json:"complete"`
	Items             []browserItemObservation `json:"items"`
	MissingTerms      []string                 `json:"missing_terms"`
	InvalidTerms      []string                 `json:"invalid_terms"`
	ItemsSubtotalBRL  float64                  `json:"items_subtotal_brl"`
	DeliveryFeeBRL    *float64                 `json:"delivery_fee_brl,omitempty"`
	EstimatedTotalBRL *float64                 `json:"estimated_total_brl,omitempty"`
}

type browserQuoteValidation struct {
	SchemaVersion       int                      `json:"schema_version"`
	Complete            bool                     `json:"complete"`
	RequiredMarketCount int                      `json:"required_market_count"`
	MinimumRating       float64                  `json:"minimum_rating"`
	RequestedItems      []browserRequestedItem   `json:"requested_items"`
	EligibleMarketCount int                      `json:"eligible_market_count"`
	CompleteMarketCount int                      `json:"complete_market_count"`
	Markets             []browserValidatedMarket `json:"markets"`
	SelectedMarketID    string                   `json:"selected_market_id,omitempty"`
	SelectedMarketName  string                   `json:"selected_market_name,omitempty"`
	SelectionBasis      string                   `json:"selection_basis,omitempty"`
	Warnings            []string                 `json:"warnings"`
}

// pp:data-source computed
func newBrowserCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "browser",
		Short: "Plan and validate iFood workflows executed through a signed-in browser",
		Long:  "Emit deterministic plans and validate observations for an AI agent using a signed-in browser. This command never exports browser credentials or performs remote writes.",
	}
	cmd.AddCommand(newBrowserSchemaCmd(flags), newBrowserPlanCmd(flags), newBrowserValidateQuoteCmd(flags), newBrowserCartPlanCmd(flags))
	return cmd
}

// pp:data-source computed
func newBrowserSchemaCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "schema",
		Short:       "Print the Browser quote-observation contract",
		Example:     "  ifood-pp-cli browser schema --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			fee := 0.0
			example := browserQuoteObservation{
				SchemaVersion: browserQuoteSchemaVersion,
				CapturedAt:    "2026-01-01T12:00:00Z",
				RequestedItems: []browserRequestedItem{
					{Term: "papel toalha", Quantity: 1},
				},
				Markets: []browserMarketObservation{{
					ID:             "merchant-id",
					Name:           "Mercado Exemplo",
					Rating:         4.8,
					DeliveryFeeBRL: &fee,
					Items: []browserItemObservation{{
						Term: "papel toalha", ProductID: "optional-product-id", ProductName: "Papel Toalha Exemplo", UnitPrice: 9.99, Quantity: 1, Available: true,
					}},
				}},
			}
			return printNovelJSON(cmd, flags, map[string]any{
				"schema_version": browserQuoteSchemaVersion,
				"input":          example,
				"rules": []string{
					"Record one market object per quoted store.",
					"Use the requested term verbatim in each matching item.term.",
					"Set available=false when the visible product cannot be added.",
					"product_id is optional because the browser UI may expose only a product name.",
					"Never include cookies, authorization headers, addresses, or payment data.",
				},
			})
		},
	}
}

// pp:data-source computed
func newBrowserPlanCmd(flags *rootFlags) *cobra.Command {
	var items []string
	var minimumRating float64
	var marketCount int
	command := &cobra.Command{
		Use:         "plan",
		Short:       "Build a safe Browser workflow plan",
		Example:     "  ifood-pp-cli browser plan --item 'papel toalha' --item esponja --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if len(items) == 0 {
				items = append([]string(nil), defaultBrowserItems...)
			}
			requirements, err := parseBrowserRequirements(items)
			if err != nil {
				return usageErr(err)
			}
			if err := validateBrowserThresholds(minimumRating, marketCount); err != nil {
				return usageErr(err)
			}
			return printNovelJSON(cmd, flags, map[string]any{
				"schema_version":         browserQuoteSchemaVersion,
				"transport":              "browser",
				"authentication":         "existing_browser_session",
				"credentials_exported":   false,
				"remote_write_performed": false,
				"minimum_rating":         minimumRating,
				"required_market_count":  marketCount,
				"requested_items":        requirements,
				"steps": []map[string]any{
					{"id": "session", "mode": "read", "instruction": "Confirm the iFood page shows the signed-in profile and selected delivery address."},
					{"id": "markets", "mode": "read", "instruction": "Collect at least the required number of markets whose visible rating meets the threshold."},
					{"id": "products", "mode": "read", "instruction": "Search every requested term inside every candidate market and record one addable product and visible price."},
					{"id": "validate", "mode": "local", "instruction": "Write the observation JSON without credentials and run browser validate-quote."},
					{"id": "cart_plan", "mode": "local", "instruction": "Run browser cart-plan and verify complete=true before any cart interaction."},
					{"id": "cart_write", "mode": "write", "instruction": "Obtain explicit user authorization for the exact selected market and products, then add items through the browser and verify the cart."},
				},
				"safety": map[string]any{
					"confirmation_required_before": "adding_the_first_cart_item",
					"unsupported_actions":          []string{"checkout", "payment", "order_submission", "address_change", "account_change"},
					"captcha":                      "pause_and_ask_the_user",
				},
			})
		},
	}
	command.Flags().StringSliceVar(&items, "item", nil, "Requested term with optional quantity, TERM[:QTY] (repeatable)")
	command.Flags().Float64Var(&minimumRating, "min-rating", 4.5, "Minimum visible market rating")
	command.Flags().IntVar(&marketCount, "markets", 3, "Minimum number of complete market quotes")
	return command
}

// pp:data-source computed
func newBrowserValidateQuoteCmd(flags *rootFlags) *cobra.Command {
	var inputPath string
	var minimumRating float64
	var marketCount int
	var useExample bool
	command := &cobra.Command{
		Use:         "validate-quote",
		Short:       "Validate Browser-collected market and product observations",
		Example:     "  ifood-pp-cli browser validate-quote --example --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateBrowserThresholds(minimumRating, marketCount); err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				return printBrowserLocalDryRun(cmd, flags, "validate_quote", inputPath, minimumRating, marketCount, "")
			}
			var observation browserQuoteObservation
			var err error
			if useExample {
				observation, err = loadBrowserExampleQuote()
			} else {
				observation, err = readBrowserQuoteObservation(cmd, inputPath)
			}
			if err != nil {
				return usageErr(err)
			}
			validation, err := validateBrowserQuote(observation, minimumRating, marketCount)
			if err != nil {
				return usageErr(err)
			}
			return printNovelJSON(cmd, flags, validation)
		},
	}
	command.Flags().StringVar(&inputPath, "input", "-", "Observation JSON file, or - for stdin")
	command.Flags().BoolVar(&useExample, "example", false, "Validate the embedded credential-free three-market example")
	command.Flags().Float64Var(&minimumRating, "min-rating", 4.5, "Minimum visible market rating")
	command.Flags().IntVar(&marketCount, "markets", 3, "Minimum number of complete market quotes")
	return command
}

// pp:data-source computed
func newBrowserCartPlanCmd(flags *rootFlags) *cobra.Command {
	var inputPath string
	var merchantID string
	var minimumRating float64
	var marketCount int
	var useExample bool
	command := &cobra.Command{
		Use:         "cart-plan",
		Short:       "Choose a validated market and emit exact Browser cart lines",
		Example:     "  ifood-pp-cli browser cart-plan --example --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateBrowserThresholds(minimumRating, marketCount); err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				return printBrowserLocalDryRun(cmd, flags, "cart_plan", inputPath, minimumRating, marketCount, merchantID)
			}
			var observation browserQuoteObservation
			var err error
			if useExample {
				observation, err = loadBrowserExampleQuote()
			} else {
				observation, err = readBrowserQuoteObservation(cmd, inputPath)
			}
			if err != nil {
				return usageErr(err)
			}
			validation, err := validateBrowserQuote(observation, minimumRating, marketCount)
			if err != nil {
				return usageErr(err)
			}
			return printNovelJSON(cmd, flags, buildBrowserCartPlan(validation, merchantID))
		},
	}
	command.Flags().StringVar(&inputPath, "input", "-", "Observation JSON file, or - for stdin")
	command.Flags().BoolVar(&useExample, "example", false, "Plan from the embedded credential-free three-market example")
	command.Flags().StringVar(&merchantID, "merchant", "", "Select a specific validated merchant ID; defaults to the lowest complete estimate")
	command.Flags().Float64Var(&minimumRating, "min-rating", 4.5, "Minimum visible market rating")
	command.Flags().IntVar(&marketCount, "markets", 3, "Minimum number of complete market quotes")
	return command
}

func loadBrowserExampleQuote() (browserQuoteObservation, error) {
	var observation browserQuoteObservation
	if err := json.Unmarshal(browserExampleQuoteJSON, &observation); err != nil {
		return browserQuoteObservation{}, fmt.Errorf("decode embedded browser example: %w", err)
	}
	return observation, nil
}

func printBrowserLocalDryRun(cmd *cobra.Command, flags *rootFlags, operation, inputPath string, minimumRating float64, marketCount int, merchantID string) error {
	result := map[string]any{
		"schema_version":         browserQuoteSchemaVersion,
		"dry_run":                true,
		"operation":              operation,
		"transport":              "local",
		"input":                  inputPath,
		"minimum_rating":         minimumRating,
		"required_market_count":  marketCount,
		"remote_write_performed": false,
	}
	if merchantID != "" {
		result["merchant_id"] = merchantID
	}
	return printNovelJSON(cmd, flags, result)
}

func parseBrowserRequirements(specs []string) ([]browserRequestedItem, error) {
	terms, err := parseQuantityTerms(specs)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	result := make([]browserRequestedItem, 0, len(terms))
	for _, term := range terms {
		key := normalizeBrowserTerm(term.Term)
		if seen[key] {
			return nil, fmt.Errorf("duplicate requested term %q", term.Term)
		}
		seen[key] = true
		result = append(result, browserRequestedItem{Term: strings.TrimSpace(term.Term), Quantity: term.Quantity})
	}
	return result, nil
}

func validateBrowserThresholds(minimumRating float64, marketCount int) error {
	if minimumRating < 0 || minimumRating > 5 {
		return errors.New("--min-rating must be between 0 and 5")
	}
	if marketCount < 1 {
		return errors.New("--markets must be at least 1")
	}
	return nil
}

func readBrowserQuoteObservation(cmd *cobra.Command, path string) (browserQuoteObservation, error) {
	var data []byte
	var err error
	if strings.TrimSpace(path) == "" || path == "-" {
		data, err = io.ReadAll(cmd.InOrStdin())
	} else {
		// #nosec G304 -- --input explicitly selects a local credential-free observation file.
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return browserQuoteObservation{}, fmt.Errorf("read quote observation: %w", err)
	}
	var observation browserQuoteObservation
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&observation); err != nil {
		return browserQuoteObservation{}, fmt.Errorf("decode quote observation: %w", err)
	}
	if observation.SchemaVersion != browserQuoteSchemaVersion {
		return browserQuoteObservation{}, fmt.Errorf("unsupported schema_version %d; expected %d", observation.SchemaVersion, browserQuoteSchemaVersion)
	}
	if len(observation.RequestedItems) == 0 {
		return browserQuoteObservation{}, errors.New("requested_items must not be empty")
	}
	if len(observation.Markets) == 0 {
		return browserQuoteObservation{}, errors.New("markets must not be empty")
	}
	return observation, nil
}

func validateBrowserQuote(observation browserQuoteObservation, minimumRating float64, marketCount int) (browserQuoteValidation, error) {
	requirements := make([]browserRequestedItem, 0, len(observation.RequestedItems))
	seen := map[string]bool{}
	for _, item := range observation.RequestedItems {
		item.Term = strings.TrimSpace(item.Term)
		if item.Term == "" || item.Quantity < 1 {
			return browserQuoteValidation{}, errors.New("every requested item needs a non-empty term and quantity >= 1")
		}
		key := normalizeBrowserTerm(item.Term)
		if seen[key] {
			return browserQuoteValidation{}, fmt.Errorf("duplicate requested term %q", item.Term)
		}
		seen[key] = true
		requirements = append(requirements, item)
	}

	validation := browserQuoteValidation{
		SchemaVersion:       browserQuoteSchemaVersion,
		RequiredMarketCount: marketCount,
		MinimumRating:       minimumRating,
		RequestedItems:      requirements,
		Warnings:            []string{},
	}
	for _, market := range observation.Markets {
		validated := validateBrowserMarket(market, requirements, minimumRating)
		if validated.Eligible {
			validation.EligibleMarketCount++
		}
		if validated.Complete {
			validation.CompleteMarketCount++
		}
		validation.Markets = append(validation.Markets, validated)
	}
	sort.SliceStable(validation.Markets, func(i, j int) bool {
		left, right := validation.Markets[i], validation.Markets[j]
		if left.Complete != right.Complete {
			return left.Complete
		}
		leftTotal, rightTotal := browserMarketSortTotal(left), browserMarketSortTotal(right)
		if leftTotal != rightTotal {
			return leftTotal < rightTotal
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	})
	validation.Complete = validation.CompleteMarketCount >= marketCount
	for _, market := range validation.Markets {
		if market.Complete {
			validation.SelectedMarketID = market.ID
			validation.SelectedMarketName = market.Name
			if market.EstimatedTotalBRL != nil {
				validation.SelectionBasis = "items_plus_known_delivery_fee"
			} else {
				validation.SelectionBasis = "items_subtotal_delivery_fee_unknown"
			}
			break
		}
	}
	if validation.EligibleMarketCount < marketCount {
		validation.Warnings = append(validation.Warnings, fmt.Sprintf("only %d markets meet the minimum rating; %d required", validation.EligibleMarketCount, marketCount))
	}
	if validation.CompleteMarketCount < marketCount {
		validation.Warnings = append(validation.Warnings, fmt.Sprintf("only %d complete market quotes; %d required", validation.CompleteMarketCount, marketCount))
	}
	return validation, nil
}

func validateBrowserMarket(market browserMarketObservation, requirements []browserRequestedItem, minimumRating float64) browserValidatedMarket {
	result := browserValidatedMarket{
		ID:             strings.TrimSpace(market.ID),
		Name:           strings.TrimSpace(market.Name),
		Rating:         market.Rating,
		Eligible:       market.Rating >= minimumRating && market.Rating <= 5 && strings.TrimSpace(market.Name) != "",
		MissingTerms:   []string{},
		InvalidTerms:   []string{},
		DeliveryFeeBRL: market.DeliveryFeeBRL,
	}
	itemsByTerm := map[string]browserItemObservation{}
	for _, item := range market.Items {
		key := normalizeBrowserTerm(item.Term)
		if key != "" {
			itemsByTerm[key] = item
		}
	}
	for _, requirement := range requirements {
		item, ok := itemsByTerm[normalizeBrowserTerm(requirement.Term)]
		if !ok {
			result.MissingTerms = append(result.MissingTerms, requirement.Term)
			continue
		}
		item.Term = requirement.Term
		item.Quantity = requirement.Quantity
		if !item.Available || strings.TrimSpace(item.ProductName) == "" || item.UnitPrice <= 0 || math.IsNaN(item.UnitPrice) || math.IsInf(item.UnitPrice, 0) {
			result.InvalidTerms = append(result.InvalidTerms, requirement.Term)
			continue
		}
		item.ProductName = strings.TrimSpace(item.ProductName)
		item.ProductID = strings.TrimSpace(item.ProductID)
		result.Items = append(result.Items, item)
		result.ItemsSubtotalBRL += item.UnitPrice * float64(requirement.Quantity)
	}
	result.ItemsSubtotalBRL = roundBRL(result.ItemsSubtotalBRL)
	if market.DeliveryFeeBRL != nil && *market.DeliveryFeeBRL >= 0 && !math.IsNaN(*market.DeliveryFeeBRL) && !math.IsInf(*market.DeliveryFeeBRL, 0) {
		total := roundBRL(result.ItemsSubtotalBRL + *market.DeliveryFeeBRL)
		result.EstimatedTotalBRL = &total
	}
	result.Complete = result.Eligible && len(result.MissingTerms) == 0 && len(result.InvalidTerms) == 0 && len(result.Items) == len(requirements)
	return result
}

func buildBrowserCartPlan(validation browserQuoteValidation, requestedMerchantID string) map[string]any {
	requestedMerchantID = strings.TrimSpace(requestedMerchantID)
	var selected *browserValidatedMarket
	for i := range validation.Markets {
		market := &validation.Markets[i]
		if requestedMerchantID != "" && market.ID == requestedMerchantID {
			selected = market
			break
		}
		if requestedMerchantID == "" && selected == nil && market.Complete {
			selected = market
		}
	}
	blocked := []string{}
	if !validation.Complete {
		blocked = append(blocked, fmt.Sprintf("quote requires %d complete eligible markets; found %d", validation.RequiredMarketCount, validation.CompleteMarketCount))
	}
	if selected == nil {
		if requestedMerchantID == "" {
			blocked = append(blocked, "no complete market is available")
		} else {
			blocked = append(blocked, fmt.Sprintf("merchant %q is not present in the quote", requestedMerchantID))
		}
	} else if !selected.Complete {
		blocked = append(blocked, fmt.Sprintf("merchant %q does not have a complete eligible quote", selected.Name))
	}
	ready := len(blocked) == 0
	result := map[string]any{
		"schema_version":         browserQuoteSchemaVersion,
		"transport":              "browser",
		"ready":                  ready,
		"remote_write_performed": false,
		"requires_confirmation":  true,
		"confirmation_boundary":  "before_adding_the_first_cart_item",
		"blocked_reasons":        blocked,
		"verification": []string{
			"The selected market name and visible rating match this plan.",
			"Every planned product name and quantity appears in the cart.",
			"The visible cart subtotal is consistent with current prices; stop on substitutions or material price changes.",
			"Do not proceed to checkout, payment, or order submission.",
		},
	}
	if selected != nil {
		result["merchant"] = map[string]any{"id": selected.ID, "name": selected.Name, "rating": selected.Rating}
		result["items"] = selected.Items
		result["items_subtotal_brl"] = selected.ItemsSubtotalBRL
		result["delivery_fee_brl"] = selected.DeliveryFeeBRL
		result["estimated_total_brl"] = selected.EstimatedTotalBRL
	}
	return result
}

func browserMarketSortTotal(market browserValidatedMarket) float64 {
	if market.EstimatedTotalBRL != nil {
		return *market.EstimatedTotalBRL
	}
	return market.ItemsSubtotalBRL
}

func normalizeBrowserTerm(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func roundBRL(value float64) float64 {
	return math.Round(value*100) / 100
}
