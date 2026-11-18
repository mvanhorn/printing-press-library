package rewe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Recipe is the part of the schema.org Recipe the CLI uses.
type Recipe struct {
	Slug         string          `json:"slug"`
	Name         string          `json:"name"`
	Yield        string          `json:"yield,omitempty"`
	TotalTime    string          `json:"total_time,omitempty"`
	Ingredients  []string        `json:"ingredients"`
	Instructions []string        `json:"instructions,omitempty"`
	URL          string          `json:"url"`
	SchemaJSON   json.RawMessage `json:"-"`
}

var recipeSchemaRe = regexp.MustCompile(`(?s)<script[^>]*id="recipe-schema"[^>]*>(.*?)</script>`)

// RecipeSlug accepts a slug or a recipe URL.
func RecipeSlug(s string) string {
	s = strings.TrimSpace(s)
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		s = u.Path
	}
	s = strings.Trim(s, "/")
	s = strings.TrimPrefix(s, "rezepte/")
	return strings.Trim(s, "/")
}

// ParseRecipePage extracts the recipe schema from a recipe page.
func ParseRecipePage(page string) (*Recipe, error) {
	m := recipeSchemaRe.FindStringSubmatch(page)
	if m == nil {
		return nil, errors.New("recipe page has no script#recipe-schema (layout changed or not a recipe)")
	}
	raw := strings.TrimSpace(m[1])
	var node map[string]any
	if err := json.Unmarshal([]byte(raw), &node); err != nil {
		return nil, fmt.Errorf("recipe schema is not JSON: %w", err)
	}
	rec := findRecipeNode(node)
	if rec == nil {
		return nil, errors.New("recipe schema has no Recipe node")
	}
	// The ingredient mapper expects the Recipe node itself with list-valued
	// image, the way the recipe page's own script sends it.
	if img, ok := rec["image"].(string); ok {
		rec["image"] = []string{img}
	}
	nodeJSON, err := json.Marshal(rec)
	if err != nil {
		return nil, err
	}
	r := &Recipe{SchemaJSON: nodeJSON}
	r.Name, _ = rec["name"].(string)
	r.TotalTime, _ = rec["totalTime"].(string)
	switch y := rec["recipeYield"].(type) {
	case string:
		r.Yield = y
	case []any:
		if len(y) > 0 {
			r.Yield = fmt.Sprint(y[0])
		}
	case float64:
		r.Yield = fmt.Sprint(y)
	}
	if ings, ok := rec["recipeIngredient"].([]any); ok {
		for _, i := range ings {
			r.Ingredients = append(r.Ingredients, strings.TrimSpace(fmt.Sprint(i)))
		}
	}
	if steps, ok := rec["recipeInstructions"].([]any); ok {
		for _, s := range steps {
			switch v := s.(type) {
			case string:
				r.Instructions = append(r.Instructions, v)
			case map[string]any:
				if t, ok := v["text"].(string); ok {
					r.Instructions = append(r.Instructions, t)
				}
			}
		}
	}
	return r, nil
}

func findRecipeNode(n map[string]any) map[string]any {
	if t, ok := n["@type"]; ok && fmt.Sprint(t) == "Recipe" {
		return n
	}
	if me, ok := n["mainEntity"].(map[string]any); ok {
		if r := findRecipeNode(me); r != nil {
			return r
		}
	}
	if g, ok := n["@graph"].([]any); ok {
		for _, x := range g {
			if m, ok := x.(map[string]any); ok {
				if r := findRecipeNode(m); r != nil {
					return r
				}
			}
		}
	}
	return nil
}

// Recipe loads a recipe page by slug.
func (sh *Shop) Recipe(ctx context.Context, slugOrURL string) (*Recipe, error) {
	slug := RecipeSlug(slugOrURL)
	if slug == "" {
		return nil, errors.New("recipe slug is empty")
	}
	page, final, err := sh.HTML(ctx, "/rezepte/"+slug+"/")
	if err != nil {
		return nil, err
	}
	r, err := ParseRecipePage(page)
	if err != nil {
		return nil, err
	}
	r.Slug = slug
	r.URL = final
	return r, nil
}

// Mapping is REWE's ingredient-to-product mapping.
type Mapping struct {
	Processed   bool   `json:"processed"`
	MappingID   string `json:"mappingId"`
	Ingredients []struct {
		NormalizedSearchTerm   string `json:"normalizedSearchTerm"`
		Basic                  bool   `json:"basic"`
		DisplayName            string `json:"displayName"`
		ProductRecommendations []struct {
			ProductID string `json:"productId"`
			ListingID string `json:"listingId"`
			Title     string `json:"title"`
			Pricing   struct {
				CurrentRetailPrice int    `json:"currentRetailPrice"`
				Grammage           string `json:"grammage"`
			} `json:"pricing"`
			OrderLimit int `json:"orderLimit"`
		} `json:"productRecommendations"`
	} `json:"ingredients"`
}

var mappingIDRe = regexp.MustCompile(`/mappings/([0-9a-f-]{36})`)

// MapRecipe asks REWE to map a recipe's ingredients to products for N servings
// and polls until the mapping is processed.
func (sh *Shop) MapRecipe(ctx context.Context, r *Recipe, servings int) (*Mapping, error) {
	form := url.Values{"data": {string(r.SchemaJSON)}}
	resp, _, err := sh.Form(ctx, fmt.Sprintf("/api/v2/recipes/mapping?servings=%d", servings), form)
	if err != nil {
		return nil, fmt.Errorf("starting ingredient mapping: %w", err)
	}
	m := mappingIDRe.FindStringSubmatch(resp.Request.URL.String())
	if m == nil {
		return nil, fmt.Errorf("ingredient mapping did not redirect to a mapping page (got %s)", resp.Request.URL.Path)
	}
	var out Mapping
	for i := 0; i < 15; i++ {
		if err := sh.JSON(ctx, http.MethodGet, "/shop/api/v2/recipes/mapping/"+m[1], nil, &out, nil); err != nil {
			return nil, err
		}
		if out.Processed {
			return &out, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return &out, fmt.Errorf("ingredient mapping %s not processed after 15 s", m[1])
}
