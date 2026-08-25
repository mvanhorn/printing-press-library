// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

const productFields = "code,product_name,brands,quantity,serving_size,categories_tags,labels_tags,countries_tags,nutriscore_grade,nova_group,ecoscore_grade,ingredients_text,ingredients_tags,ingredients_analysis_tags,allergens_tags,traces_tags,additives_tags,nutriments,data_quality_tags,image_url,last_modified_t"

func newProductCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "product <barcode>",
		Short: "Fetch one Open Food Facts product by barcode",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			barcode := strings.TrimSpace(args[0])
			off := newClient(flags.timeout)
			response, err := off.product(cmd.Context(), barcode)
			if err != nil {
				return err
			}
			out := map[string]any{
				"source":     "Open Food Facts Product API v3",
				"barcode":    barcode,
				"product":    summarizeProduct(response.Product, off.baseURL),
				"freshness":  freshnessFacts(),
				"caveats":    dataCaveats(),
				"request":    map[string]any{"endpoint": "/api/v3/product/<barcode>.json"},
				"configured": configuredEnv(),
			}
			return writeJSON(cmd, flags, out)
		},
	}
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var category, brand, country, label, nutritionGrade string
	var page, pageSize int
	cmd := &cobra.Command{
		Use:   "search [terms]",
		Short: "Run bounded Open Food Facts structured product search",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			page = clampInt(page, 1, 100)
			pageSize = clampInt(pageSize, 1, 25)
			query := url.Values{}
			query.Set("fields", productFields)
			query.Set("page", fmt.Sprintf("%d", page))
			query.Set("page_size", fmt.Sprintf("%d", pageSize))
			if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
				query.Set("search_terms", strings.TrimSpace(args[0]))
			}
			setSearchFilter(query, "categories_tags_en", category)
			setSearchFilter(query, "brands_tags", brand)
			setSearchFilter(query, "countries_tags_en", country)
			setSearchFilter(query, "labels_tags_en", label)
			setSearchFilter(query, "nutrition_grades_tags", nutritionGrade)
			if len(query) <= 3 {
				return fmt.Errorf("provide at least one search term or structured filter")
			}

			off := newClient(flags.timeout)
			response, err := off.search(cmd.Context(), query)
			if err != nil {
				return err
			}
			results := make([]productSummary, 0, len(response.Products))
			for _, product := range response.Products {
				results = append(results, summarizeProduct(product, off.baseURL))
			}
			out := map[string]any{
				"source":     "Open Food Facts Search API v2",
				"query":      searchQuerySummary(args, category, brand, country, label, nutritionGrade),
				"request":    map[string]any{"endpoint": "/api/v2/search", "page": page, "page_size": pageSize},
				"count":      response.Count,
				"page":       response.Page,
				"page_size":  response.PageSize,
				"results":    results,
				"caveats":    searchCaveats(),
				"configured": configuredEnv(),
			}
			return writeJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Category filter, for example 'breakfast cereals'")
	cmd.Flags().StringVar(&brand, "brand", "", "Brand filter")
	cmd.Flags().StringVar(&country, "country", "", "Country filter, for example 'united-states'")
	cmd.Flags().StringVar(&label, "label", "", "Label filter")
	cmd.Flags().StringVar(&nutritionGrade, "nutrition-grade", "", "Nutrition grade filter, for example a or b")
	cmd.Flags().IntVar(&page, "page", 1, "Search result page")
	cmd.Flags().IntVar(&pageSize, "page-size", 10, "Search result page size, capped at 25")
	return cmd
}

func newNutritionCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "nutrition <barcode>",
		Short: "Summarize nutrition facts for one product",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			off := newClient(flags.timeout)
			response, err := off.product(cmd.Context(), strings.TrimSpace(args[0]))
			if err != nil {
				return err
			}
			product := summarizeProduct(response.Product, off.baseURL)
			out := map[string]any{
				"source":    "Open Food Facts Product API v3",
				"barcode":   product.Barcode,
				"nutrition": nutritionSummary(product),
				"freshness": freshnessFacts(),
				"caveats":   dataCaveats(),
			}
			return writeJSON(cmd, flags, out)
		},
	}
}

func newAllergensCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "allergens <barcode>",
		Short: "Summarize ingredients, allergens, traces, and additives for one product",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			off := newClient(flags.timeout)
			response, err := off.product(cmd.Context(), strings.TrimSpace(args[0]))
			if err != nil {
				return err
			}
			product := summarizeProduct(response.Product, off.baseURL)
			out := map[string]any{
				"source":       "Open Food Facts Product API v3",
				"barcode":      product.Barcode,
				"source_url":   product.SourceURL,
				"ingredients":  product.IngredientsText,
				"analysis":     product.IngredientsAnalysis,
				"allergens":    product.Allergens,
				"traces":       product.Traces,
				"additives":    product.Additives,
				"data_quality": product.DataQuality,
				"caveats":      dataCaveats(),
			}
			return writeJSON(cmd, flags, out)
		},
	}
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "compare <barcode> <barcode> [barcode...]",
		Short: "Compare a small set of Open Food Facts products",
		Args:  cobra.RangeArgs(2, 5),
		RunE: func(cmd *cobra.Command, args []string) error {
			off := newClient(flags.timeout)
			products := make([]productSummary, 0, len(args))
			errors := make([]map[string]string, 0)
			for _, raw := range args {
				barcode := strings.TrimSpace(raw)
				response, err := off.product(cmd.Context(), barcode)
				if err != nil {
					errors = append(errors, map[string]string{"barcode": barcode, "error": err.Error()})
					continue
				}
				products = append(products, summarizeProduct(response.Product, off.baseURL))
			}
			out := map[string]any{
				"source":   "Open Food Facts Product API v3",
				"products": products,
				"errors":   errors,
				"comparison": map[string]any{
					"fields": []string{"nutriscore_grade", "nova_group", "ecoscore_grade", "nutriments", "allergens", "additives", "data_quality"},
				},
				"caveats": dataCaveats(),
			}
			return writeJSON(cmd, flags, out)
		},
	}
}

func newCategoryCmd(flags *rootFlags) *cobra.Command {
	pageSize := 5
	cmd := &cobra.Command{
		Use:   "category <category>",
		Short: "Fetch a small product sample for a category",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageSize = clampInt(pageSize, 1, 25)
			query := url.Values{}
			query.Set("fields", productFields)
			query.Set("page", "1")
			query.Set("page_size", fmt.Sprintf("%d", pageSize))
			setSearchFilter(query, "categories_tags_en", args[0])
			off := newClient(flags.timeout)
			response, err := off.search(cmd.Context(), query)
			if err != nil {
				return err
			}
			results := make([]productSummary, 0, len(response.Products))
			for _, product := range response.Products {
				results = append(results, summarizeProduct(product, off.baseURL))
			}
			return writeJSON(cmd, flags, map[string]any{
				"source":     "Open Food Facts Search API v2",
				"category":   args[0],
				"request":    map[string]any{"endpoint": "/api/v2/search", "page": 1, "page_size": pageSize},
				"results":    results,
				"count":      response.Count,
				"caveats":    searchCaveats(),
				"freshness":  freshnessFacts(),
				"configured": configuredEnv(),
			})
		},
	}
	cmd.Flags().IntVar(&pageSize, "page-size", 5, "Category sample size, capped at 25")
	return cmd
}

func newSourcesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "sources",
		Short: "Describe Open Food Facts API coverage, rate limits, and caveats",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := map[string]any{
				"source": "Open Food Facts",
				"api_docs": []string{
					"https://openfoodfacts.github.io/openfoodfacts-server/api/",
					"https://openfoodfacts.github.io/openfoodfacts-server/api/ref-cheatsheet/",
				},
				"base_url":    currentConfig().BaseURL,
				"auth":        "none for read operations",
				"read_only":   true,
				"coverage":    []string{"product lookup", "structured search", "nutrition", "ingredients/allergens", "small product comparison", "category samples"},
				"rate_limits": rateLimitFacts(),
				"freshness":   freshnessFacts(),
				"caveats":     append(dataCaveats(), "Use CSV/JSONL exports instead of live API calls for bulk product datasets."),
				"non_goals":   nonGoals(),
				"configured":  configuredEnv(),
			}
			return writeJSON(cmd, flags, out)
		},
	}
}

func summarizeProduct(product productRecord, baseURL string) productSummary {
	barcode := product.Code
	return productSummary{
		Barcode:             barcode,
		Name:                product.ProductName,
		Brands:              product.Brands,
		Quantity:            product.Quantity,
		ServingSize:         product.ServingSize,
		Categories:          trimTags(product.CategoriesTags),
		Labels:              trimTags(product.LabelsTags),
		Countries:           trimTags(product.CountriesTags),
		NutriScoreGrade:     product.NutriScoreGrade,
		NovaGroup:           product.NovaGroup,
		EcoScoreGrade:       product.EcoScoreGrade,
		IngredientsText:     product.IngredientsText,
		Ingredients:         trimTags(product.IngredientsTags),
		IngredientsAnalysis: trimTags(product.IngredientsAnalysis),
		Allergens:           trimTags(product.AllergensTags),
		Traces:              trimTags(product.TracesTags),
		Additives:           trimTags(product.AdditivesTags),
		Nutriments:          selectedNutriments(product.Nutriments),
		DataQuality:         trimTags(product.DataQualityTags),
		ImageURL:            product.ImageURL,
		SourceURL:           strings.TrimRight(baseURL, "/") + "/product/" + barcode,
	}
}

func nutritionSummary(product productSummary) map[string]any {
	return map[string]any{
		"barcode":          product.Barcode,
		"name":             product.Name,
		"nutriscore_grade": product.NutriScoreGrade,
		"nova_group":       product.NovaGroup,
		"ecoscore_grade":   product.EcoScoreGrade,
		"serving_size":     product.ServingSize,
		"nutriments":       product.Nutriments,
		"data_quality":     product.DataQuality,
		"source_url":       product.SourceURL,
	}
}

func selectedNutriments(nutriments map[string]any) map[string]any {
	if len(nutriments) == 0 {
		return nil
	}
	keys := []string{"energy-kcal_100g", "energy_100g", "fat_100g", "saturated-fat_100g", "carbohydrates_100g", "sugars_100g", "fiber_100g", "proteins_100g", "salt_100g", "sodium_100g"}
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := nutriments[key]; ok {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nutriments
	}
	return out
}

func setSearchFilter(query url.Values, key, value string) {
	if strings.TrimSpace(value) != "" {
		query.Set(key, strings.TrimSpace(value))
	}
}

func searchQuerySummary(args []string, category, brand, country, label, nutritionGrade string) map[string]string {
	summary := map[string]string{}
	if len(args) == 1 {
		summary["terms"] = strings.TrimSpace(args[0])
	}
	for key, value := range map[string]string{
		"category":        category,
		"brand":           brand,
		"country":         country,
		"label":           label,
		"nutrition_grade": nutritionGrade,
	} {
		if strings.TrimSpace(value) != "" {
			summary[key] = strings.TrimSpace(value)
		}
	}
	return summary
}

func trimTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = stripLocalePrefix(strings.TrimSpace(tag))
		tag = strings.ReplaceAll(tag, "-", " ")
		if tag != "" {
			out = append(out, tag)
		}
	}
	return out
}

func stripLocalePrefix(tag string) string {
	if len(tag) >= 3 && tag[2] == ':' && isASCIIAlpha(tag[0]) && isASCIIAlpha(tag[1]) {
		return tag[3:]
	}
	return tag
}

func isASCIIAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func rateLimitFacts() []string {
	return []string{
		"Product read queries are documented at 15 requests per minute per IP address.",
		"Search queries are documented at 10 requests per minute per IP address.",
		"Open Food Facts asks clients to use exports instead of live API calls for more than a few hundred products.",
	}
}

func freshnessFacts() []string {
	return []string{
		"Product records are community-contributed and may change whenever volunteers update data.",
		"The API does not guarantee every product has complete nutrition, ingredient, allergen, or image fields.",
	}
}

func dataCaveats() []string {
	return []string{
		"Open Food Facts data is voluntarily contributed and is not guaranteed accurate, complete, or reliable.",
		"This CLI reports source fields and data-quality tags; it does not provide medical or dietary advice.",
	}
}

func searchCaveats() []string {
	return append(dataCaveats(),
		"Structured search uses the v2 search endpoint because v3 search is not implemented.",
		"Do not use search-as-you-type loops; search has a documented 10 requests per minute per IP limit.",
	)
}

func nonGoals() []string {
	return []string{
		"product edits",
		"image uploads",
		"login or session workflows",
		"bulk harvesting",
		"medical advice",
		"raw endpoint mirroring",
	}
}
