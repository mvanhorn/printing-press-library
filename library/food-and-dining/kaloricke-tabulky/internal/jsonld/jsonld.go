// Package jsonld extracts schema.org JSON-LD blocks from kaloricketabulky.cz
// detail pages (/potraviny/<slug>, /recepty/<slug>, /aktivita/<slug>) and
// projects the Czech-language nutrition keywords into a typed struct.
//
// Each detail page embeds at least one <script type="application/ld+json">
// block. The interesting payload sits in the "keywords" array of a Dataset
// (foodstuff) or in the "recipeIngredient" / "nutrition" properties of a
// Recipe. The format is Czech: "Energetická hodnota : 62,7 kJ" etc.
package jsonld

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/kaloricke-tabulky/internal/cliutil"
)

// limiter caps detail-page fetches when many run in a row (e.g. food
// substitutes fetches one JSON-LD per candidate). 4 req/s is the floor;
// the AdaptiveLimiter ramps it down on 429s.
var (
	limiterOnce sync.Once
	limiter     *cliutil.AdaptiveLimiter
)

func getLimiter() *cliutil.AdaptiveLimiter {
	limiterOnce.Do(func() {
		limiter = cliutil.NewAdaptiveLimiter(4)
	})
	return limiter
}

// Nutrition is the typed projection of a foodstuff's nutrition keywords.
// Values are per-100 g (per-100 ml for liquids) and units are normalized to
// kJ / g / mg.
type Nutrition struct {
	EnergyKJ            float64  `json:"energy_kj,omitempty"`
	EnergyKcal          float64  `json:"energy_kcal,omitempty"`
	ProteinG            float64  `json:"protein_g,omitempty"`
	FatG                float64  `json:"fat_g,omitempty"`
	CarbG               float64  `json:"carb_g,omitempty"`
	FiberG              float64  `json:"fiber_g,omitempty"`
	SugarsG             float64  `json:"sugars_g,omitempty"`
	SaturatedFatG       float64  `json:"saturated_fat_g,omitempty"`
	MonoUnsaturatedFatG float64  `json:"monounsaturated_fat_g,omitempty"`
	PolyUnsaturatedFatG float64  `json:"polyunsaturated_fat_g,omitempty"`
	CalciumMg           float64  `json:"calcium_mg,omitempty"`
	CholesterolMg       float64  `json:"cholesterol_mg,omitempty"`
	SaltG               float64  `json:"salt_g,omitempty"`
	SodiumMg            float64  `json:"sodium_mg,omitempty"`
	WaterG              float64  `json:"water_g,omitempty"`
	GIIndex             float64  `json:"glycemic_index,omitempty"`
	PHL                 float64  `json:"ph,omitempty"`
	Raw                 []string `json:"raw_keywords,omitempty"`
}

// FoodstuffDetail is the typed projection of a /potraviny/<slug> page.
type FoodstuffDetail struct {
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Description string    `json:"description,omitempty"`
	Type        string    `json:"type"`
	Nutrition   Nutrition `json:"nutrition"`
}

// jsonldEnvelope models the @type Dataset shape the site uses for
// /potraviny/<slug>. Recipe and activity pages use slightly different
// shapes; the extraction logic is robust to either.
type jsonldEnvelope struct {
	Context     interface{} `json:"@context"`
	Type        interface{} `json:"@type"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	URL         string      `json:"url"`
	Keywords    interface{} `json:"keywords"`
}

var scriptRe = regexp.MustCompile(`(?is)<script[^>]*type=["']application/ld\+json["'][^>]*>(.+?)</script>`)

// ExtractFromHTML reads all application/ld+json blocks from the given HTML
// and returns the first Dataset-shaped block as a FoodstuffDetail. Returns
// the parsed envelopes alongside so callers can pick a different type if the
// page mixes shapes (e.g., Recipe + BreadcrumbList).
func ExtractFromHTML(html []byte) (*FoodstuffDetail, []json.RawMessage, error) {
	matches := scriptRe.FindAllSubmatch(html, -1)
	if len(matches) == 0 {
		return nil, nil, fmt.Errorf("no application/ld+json script blocks found")
	}
	var envelopes []json.RawMessage
	var firstDataset *jsonldEnvelope
	for _, m := range matches {
		raw := strings.TrimSpace(string(m[1]))
		envelopes = append(envelopes, json.RawMessage(raw))
		var env jsonldEnvelope
		if err := json.Unmarshal([]byte(raw), &env); err != nil {
			continue
		}
		typeStr := stringifyType(env.Type)
		if firstDataset == nil && typeStr == "Dataset" {
			firstDataset = &env
		}
	}
	if firstDataset == nil {
		return nil, envelopes, fmt.Errorf("no Dataset-typed JSON-LD block (page returned %d blocks of other types)", len(envelopes))
	}
	keywords := stringifyKeywords(firstDataset.Keywords)
	d := &FoodstuffDetail{
		Title:       firstDataset.Name,
		URL:         firstDataset.URL,
		Description: firstDataset.Description,
		Type:        stringifyType(firstDataset.Type),
		Nutrition:   parseCzechNutritionKeywords(keywords),
	}
	return d, envelopes, nil
}

// FetchDetail fetches an HTML detail page and extracts the nutrition data.
// The base URL plus the slug path (e.g. "/potraviny/jablko") is the input.
func FetchDetail(client *http.Client, fullURL string, cookieHeader string) (*FoodstuffDetail, error) {
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "kaloricke-tabulky-pp-cli/1.0")
	if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if l := getLimiter(); l != nil {
		l.Wait()
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		if l := getLimiter(); l != nil {
			l.OnRateLimit()
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, &cliutil.RateLimitError{URL: fullURL, Body: string(body)}
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, fullURL)
	}
	if l := getLimiter(); l != nil {
		l.OnSuccess()
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, err
	}
	detail, _, err := ExtractFromHTML(body)
	if err != nil {
		return nil, fmt.Errorf("parse JSON-LD from %s: %w", fullURL, err)
	}
	return detail, nil
}

func stringifyType(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []interface{}:
		if len(t) > 0 {
			if s, ok := t[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

func stringifyKeywords(v interface{}) []string {
	switch t := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		return strings.Split(t, ",")
	}
	return nil
}

// parseCzechNutritionKeywords parses lines like "Energetická hodnota : 62,7 kJ"
// or "Bílkoviny : 0,37 g" into the typed struct. Czech decimal separator is
// comma; multipliers are kJ, kcal, g, mg.
func parseCzechNutritionKeywords(kws []string) Nutrition {
	n := Nutrition{Raw: kws}
	for _, kw := range kws {
		label, value, unit, ok := splitNutritionKeyword(kw)
		if !ok {
			continue
		}
		// Match Czech labels (case-insensitive, accent-insensitive prefix)
		switch {
		case hasPrefixFold(label, "Energetická hodnota") && unit == "kJ":
			n.EnergyKJ = value
		case hasPrefixFold(label, "Energetická hodnota") && unit == "kcal":
			n.EnergyKcal = value
		case hasPrefixFold(label, "Bílkoviny") || hasPrefixFold(label, "Protein"):
			n.ProteinG = value
		case hasPrefixFold(label, "Sacharidy") || hasPrefixFold(label, "Carb"):
			n.CarbG = value
		case hasPrefixFold(label, "Vláknina") || hasPrefixFold(label, "Fiber"):
			n.FiberG = value
		case hasPrefixFold(label, "Cukry") || hasPrefixFold(label, "Sugar"):
			n.SugarsG = value
		case hasPrefixFold(label, "Nasycené mastné kyseliny") || hasPrefixFold(label, "Saturated"):
			n.SaturatedFatG = value
		case hasPrefixFold(label, "Mononenasycené"):
			n.MonoUnsaturatedFatG = value
		case hasPrefixFold(label, "Polynenasycené"):
			n.PolyUnsaturatedFatG = value
		case hasPrefixFold(label, "Tuky") || hasPrefixFold(label, "Fat"):
			// Generic "Tuky" comes last so it doesn't catch the specific fat lines above
			n.FatG = value
		case hasPrefixFold(label, "Vápník") || hasPrefixFold(label, "Calcium"):
			n.CalciumMg = value
		case hasPrefixFold(label, "Cholesterol"):
			n.CholesterolMg = value
		case hasPrefixFold(label, "Sůl") || hasPrefixFold(label, "Salt"):
			n.SaltG = value
		case hasPrefixFold(label, "Sodík") || hasPrefixFold(label, "Sodium"):
			n.SodiumMg = value
		case hasPrefixFold(label, "Voda") || hasPrefixFold(label, "Water"):
			n.WaterG = value
		case hasPrefixFold(label, "GI"):
			n.GIIndex = value
		case hasPrefixFold(label, "pH"):
			n.PHL = value
		}
	}
	// Default unit: if both kJ and kcal are missing, leave both zero. Don't auto-derive.
	return n
}

func splitNutritionKeyword(in string) (label string, value float64, unit string, ok bool) {
	parts := strings.SplitN(in, ":", 2)
	if len(parts) != 2 {
		return "", 0, "", false
	}
	label = strings.TrimSpace(parts[0])
	rest := strings.TrimSpace(parts[1])
	tokens := strings.Fields(rest)
	if len(tokens) == 0 {
		return label, 0, "", false
	}
	// Czech decimal uses comma. Normalize.
	num := strings.Replace(tokens[0], ",", ".", 1)
	// Strip non-breaking spaces inside numbers (e.g. "62,7" arrives clean here)
	num = strings.ReplaceAll(num, " ", "")
	var v float64
	if _, err := fmt.Sscanf(num, "%f", &v); err != nil {
		return label, 0, "", false
	}
	unitTok := ""
	if len(tokens) > 1 {
		unitTok = tokens[1]
	}
	return label, v, unitTok, true
}

func hasPrefixFold(s, pfx string) bool {
	return strings.HasPrefix(strings.ToLower(s), strings.ToLower(pfx))
}

// ExtractAllergens scans the raw keywords for Czech allergen tokens.
// Returns the canonical English token set found.
func ExtractAllergens(n Nutrition) []string {
	found := map[string]bool{}
	for _, kw := range n.Raw {
		low := strings.ToLower(kw)
		if strings.Contains(low, "lepek") || strings.Contains(low, "gluten") {
			found["gluten"] = true
		}
		if strings.Contains(low, "laktóz") || strings.Contains(low, "laktoz") || strings.Contains(low, "lactose") {
			found["lactose"] = true
		}
		if strings.Contains(low, "vejce") || strings.Contains(low, "egg") {
			found["egg"] = true
		}
		if strings.Contains(low, "ořech") || strings.Contains(low, "orech") || strings.Contains(low, "nut") {
			found["nuts"] = true
		}
		if strings.Contains(low, "sój") || strings.Contains(low, "soj") || strings.Contains(low, "soy") {
			found["soy"] = true
		}
		if strings.Contains(low, "ryb") || strings.Contains(low, "fish") {
			found["fish"] = true
		}
		if strings.Contains(low, "med") {
			found["honey"] = true
		}
		if strings.Contains(low, "celer") || strings.Contains(low, "celery") {
			found["celery"] = true
		}
		if strings.Contains(low, "hořčic") || strings.Contains(low, "horcic") || strings.Contains(low, "mustard") {
			found["mustard"] = true
		}
		if strings.Contains(low, "sezam") || strings.Contains(low, "sesame") {
			found["sesame"] = true
		}
		if strings.Contains(low, "korýš") || strings.Contains(low, "korys") || strings.Contains(low, "crustacean") {
			found["crustaceans"] = true
		}
		if strings.Contains(low, "měkkýš") || strings.Contains(low, "mekkys") || strings.Contains(low, "mollusc") {
			found["molluscs"] = true
		}
	}
	out := make([]string, 0, len(found))
	for k := range found {
		out = append(out, k)
	}
	return out
}
