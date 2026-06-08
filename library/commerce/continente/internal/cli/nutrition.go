package cli

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/continente/internal/client"
)

var (
	nutritionSectionRe    = regexp.MustCompile(`(?s)<section class="nutriInfo-group">(.*?)</section>`)
	nutritionServingRe    = regexp.MustCompile(`(?s)<div class="serving-size">\s*<p class="mb-0">.*?</p>\s*<p class="mb-20">(.*?)</p>`)
	nutritionServingUOMRe = regexp.MustCompile(`(?s)<div class="serving-size--uom">\s*<p class="mb-0">.*?</p>\s*<p class="mb-20">(.*?)</p>`)
	nutritionHouseholdRe  = regexp.MustCompile(`(?s)<div class="serving-size--household">\s*<p class="mb-0">.*?</p>\s*<p class="mb-20">(.*?)</p>`)
	nutrientRowRe         = regexp.MustCompile(`(?s)<div class="nutrients-row row">\s*<div class="nutriInfo-details col-4 col-sm nutrients-cell">(.*?)</div>\s*<div class="nutriInfo-details col-4 col-sm nutrients-cell">(.*?)</div>\s*<div class="nutriInfo-details col-4 col-sm nutrients-cell">(.*?)</div>\s*</div>`)
)

type nutritionProfile struct {
	Per100g    *nutritionFacts  `json:"per_100g,omitempty"`
	PerServing *nutritionFacts  `json:"per_serving,omitempty"`
	Servings   []nutritionFacts `json:"servings,omitempty"`
}

type nutritionFacts struct {
	ServingAmount float64 `json:"serving_amount,omitempty"`
	ServingUnit   string  `json:"serving_unit,omitempty"`
	HouseholdSize string  `json:"household_size,omitempty"`
	EnergyKJ      float64 `json:"energy_kj,omitempty"`
	EnergyKCal    float64 `json:"energy_kcal,omitempty"`
	FatG          float64 `json:"fat_g,omitempty"`
	SaturatesG    float64 `json:"saturates_g,omitempty"`
	CarbsG        float64 `json:"carbs_g,omitempty"`
	SugarsG       float64 `json:"sugars_g,omitempty"`
	FibreG        float64 `json:"fibre_g,omitempty"`
	ProteinG      float64 `json:"protein_g,omitempty"`
	SaltG         float64 `json:"salt_g,omitempty"`
}

func fetchNutritionProfile(ctx context.Context, c *client.Client, rawURL string) (*nutritionProfile, error) {
	if c == nil || strings.TrimSpace(rawURL) == "" {
		return nil, nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse nutrition url: %w", err)
	}
	params := map[string]string{}
	for key, values := range parsed.Query() {
		if len(values) == 0 {
			continue
		}
		params[key] = values[0]
	}
	data, err := c.Get(ctx, parsed.Path, params)
	if err != nil {
		return nil, err
	}
	profile, err := parseNutritionHTML([]byte(data))
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func parseNutritionHTML(body []byte) (*nutritionProfile, error) {
	matches := nutritionSectionRe.FindAllSubmatch(body, -1)
	if len(matches) == 0 {
		return nil, &extractionError{Operation: "nutrition", Reason: "no nutrition sections found"}
	}
	profile := &nutritionProfile{Servings: make([]nutritionFacts, 0, len(matches))}
	for _, match := range matches {
		facts, ok := parseNutritionSection(string(match[1]))
		if !ok {
			continue
		}
		profile.Servings = append(profile.Servings, facts)
		if isPer100gFacts(facts) && profile.Per100g == nil {
			copyFacts := facts
			profile.Per100g = &copyFacts
		} else if profile.PerServing == nil {
			copyFacts := facts
			profile.PerServing = &copyFacts
		}
	}
	if len(profile.Servings) == 0 {
		return nil, &extractionError{Operation: "nutrition", Reason: "no nutrition facts parsed"}
	}
	if profile.PerServing == nil && len(profile.Servings) > 0 {
		copyFacts := profile.Servings[0]
		profile.PerServing = &copyFacts
	}
	return profile, nil
}

func parseNutritionSection(section string) (nutritionFacts, bool) {
	facts := nutritionFacts{}
	if match := nutritionServingRe.FindStringSubmatch(section); len(match) == 2 {
		if amount, err := parseEuroNumber(match[1]); err == nil {
			facts.ServingAmount = amount
		}
	}
	if match := nutritionServingUOMRe.FindStringSubmatch(section); len(match) == 2 {
		facts.ServingUnit = normalizeNutritionUnit(match[1])
	}
	if match := nutritionHouseholdRe.FindStringSubmatch(section); len(match) == 2 {
		facts.HouseholdSize = normalizeLabel(match[1])
	}
	rows := nutrientRowRe.FindAllStringSubmatch(section, -1)
	if len(rows) == 0 {
		return nutritionFacts{}, false
	}
	for _, row := range rows {
		if len(row) != 4 {
			continue
		}
		name := normalizeNutritionKey(row[1])
		value, err := parseNutritionNumber(row[2])
		if err != nil {
			continue
		}
		unit := normalizeNutritionUnit(row[3])
		assignNutritionValue(&facts, name, value, unit)
	}
	return facts, true
}

func normalizeNutritionKey(raw string) string {
	raw = normalizeLabel(html.UnescapeString(raw))
	raw = strings.ToLower(raw)
	switch raw {
	case "energia":
		return "energy"
	case "lípidos", "lipidos":
		return "fat"
	case "lípidos > saturados", "lipidos > saturados":
		return "saturates"
	case "hidratos de carbono":
		return "carbs"
	case "hidratos de carbono > açúcares", "hidratos de carbono > acucares":
		return "sugars"
	case "fibra":
		return "fibre"
	case "proteínas", "proteinas":
		return "protein"
	case "sal":
		return "salt"
	default:
		return raw
	}
}

func normalizeNutritionUnit(raw string) string {
	raw = normalizeLabel(html.UnescapeString(raw))
	switch {
	case strings.Contains(raw, "Quilojoule"):
		return "kJ"
	case strings.Contains(raw, "Quilocaloria"):
		return "kcal"
	case strings.Contains(raw, "Grama"):
		return "g"
	default:
		return raw
	}
}

func parseNutritionNumber(raw string) (float64, error) {
	raw = normalizeLabel(html.UnescapeString(raw))
	raw = strings.ReplaceAll(raw, ".", "")
	raw = strings.ReplaceAll(raw, ",", ".")
	return strconv.ParseFloat(raw, 64)
}

func assignNutritionValue(facts *nutritionFacts, name string, value float64, unit string) {
	if facts == nil {
		return
	}
	switch name {
	case "energy":
		if unit == "kJ" {
			facts.EnergyKJ = value
		}
		if unit == "kcal" {
			facts.EnergyKCal = value
		}
	case "fat":
		facts.FatG = value
	case "saturates":
		facts.SaturatesG = value
	case "carbs":
		facts.CarbsG = value
	case "sugars":
		facts.SugarsG = value
	case "fibre":
		facts.FibreG = value
	case "protein":
		facts.ProteinG = value
	case "salt":
		facts.SaltG = value
	}
}

func isPer100gFacts(facts nutritionFacts) bool {
	return facts.ServingAmount == 100 && facts.ServingUnit == "g"
}

func nutritionSummary(left, right *nutritionFacts) []string {
	if left == nil || right == nil {
		return nil
	}
	var out []string
	out = appendNutritionDelta(out, "energy", "kcal/100 g", left.EnergyKCal, right.EnergyKCal, false)
	out = appendNutritionDelta(out, "sugars", "g/100 g", left.SugarsG, right.SugarsG, false)
	out = appendNutritionDelta(out, "fat", "g/100 g", left.FatG, right.FatG, false)
	out = appendNutritionDelta(out, "saturates", "g/100 g", left.SaturatesG, right.SaturatesG, false)
	out = appendNutritionDelta(out, "protein", "g/100 g", left.ProteinG, right.ProteinG, true)
	out = appendNutritionDelta(out, "salt", "g/100 g", left.SaltG, right.SaltG, false)
	return out
}

func appendNutritionDelta(out []string, label, unit string, left, right float64, higherIsBetter bool) []string {
	if left == 0 && right == 0 {
		return out
	}
	diff := roundMoney(right - left)
	if diff == 0 {
		return out
	}
	if higherIsBetter {
		if diff > 0 {
			return append(out, fmt.Sprintf("right has %.2f %s more %s", diff, unit, label))
		}
		return append(out, fmt.Sprintf("right has %.2f %s less %s", -diff, unit, label))
	}
	if diff > 0 {
		return append(out, fmt.Sprintf("right has %.2f %s more %s", diff, unit, label))
	}
	return append(out, fmt.Sprintf("right has %.2f %s less %s", -diff, unit, label))
}

func fetchAndAttachNutrition(ctx context.Context, c *client.Client, product *productResponse) error {
	if product == nil || product.NutritionalInfoURL == "" {
		if product != nil && product.NutritionalInfoURL == "" {
			product.NutritionStatus = "missing_url"
		}
		return nil
	}
	profile, err := fetchNutritionProfile(ctx, c, product.NutritionalInfoURL)
	if err != nil {
		var extraction *extractionError
		if As(err, &extraction) {
			product.NutritionStatus = "not_provided"
			return nil
		}
		return err
	}
	product.NutritionStatus = "available"
	product.Nutrition = profile
	return nil
}
