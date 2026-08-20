// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: CookUnity server-driven-UI (SDUI) menu client. Preserved on regen.
//
// CookUnity has no public API. The subscription web app is an SDUI app whose
// weekly menu is fetched as deeply-nested UI-component JSON:
//
//	GET /sdui-service/web/view/menu/{date}/clustered-results
//	    -> menu chrome + FULL_MENU_LAZY_CLUSTER references (one per category/subcategory)
//	GET /sdui-service/web/view/menu/components/clustered-result?<cluster params>
//	    -> a cluster containing MEAL cards; each MEAL card's `properties`
//	       object is a complete meal record.
//
// This package fetches every cluster for a delivery date, extracts each
// MEAL.properties, flattens it to a clean types.Meal, and dedups by id.
// Auth is the raw Auth0 access token in the Authorization header (NO "Bearer "
// prefix) plus a constant `platform: web` header.
package cookunity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/types"
)

// Meal extends the generated types.Meal with a few fields the generated spec
// type does not carry (isFavorite), so the extra data rides in the stored JSON
// without editing generated code. Embedded fields marshal at the top level, so
// the typed `meals` columns still populate correctly.
type Meal struct {
	types.Meal
	IsFavorite bool `json:"isFavorite"`
}

// Client fetches the CookUnity weekly menu from the SDUI backend.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
	// limiter proactively paces the many per-sync cluster fetches so a burst
	// of requests doesn't trip CookUnity's throttle. Nil-safe: all methods
	// no-op on a nil receiver.
	limiter *cliutil.AdaptiveLimiter
}

// NewClient builds a menu client. baseURL is the app origin
// (https://subscription.cookunity.com), token is the raw Auth0 access token.
func NewClient(baseURL, token string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: timeout},
		// Conservative auto-ramping pace (2 req/s start), matching the
		// generated client's default; adapts down on any 429.
		limiter: cliutil.NewAdaptiveLimiterAuto(2.0),
	}
}

func (c *Client) get(ctx context.Context, path string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	// CookUnity auth: raw token in Authorization (no "Bearer " prefix) + platform header.
	if c.Token != "" {
		req.Header.Set("Authorization", c.Token)
	}
	req.Header.Set("platform", "web")
	req.Header.Set("accept", "application/json")
	// Proactively pace outbound requests so a burst of cluster fetches doesn't
	// trip the throttle in the first place.
	c.limiter.Wait()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	// Typed 429 handling at the transport boundary: surface throttling as an
	// error so callers abort with retry guidance instead of treating a
	// rate-limit response as a valid (empty) menu that would wipe the catalog.
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, resp.StatusCode, fmt.Errorf("rate limited (HTTP 429): CookUnity is throttling requests — wait a minute and re-run 'sync'")
	}
	c.limiter.OnSuccess()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// cluster holds the parameters needed to fetch one meal cluster.
type cluster struct {
	path   string
	params map[string]string
}

// FetchMenu fetches the full menu for a delivery date (YYYY-MM-DD) and returns
// the deduped list of meals. Each meal's DeliveryDate is set to date.
func (c *Client) FetchMenu(ctx context.Context, date string) ([]Meal, error) {
	chromePath := fmt.Sprintf("/sdui-service/web/view/menu/%s/clustered-results", url.PathEscape(date))
	body, status, err := c.get(ctx, chromePath)
	if err != nil {
		return nil, fmt.Errorf("fetching menu chrome for %s: %w", date, err)
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, fmt.Errorf("unauthorized (HTTP %d): the CookUnity token is missing or expired — re-copy it from a logged-in subscription.cookunity.com request into COOKUNITY_TOKEN", status)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("menu chrome for %s returned HTTP %d", date, status)
	}

	var chrome any
	if err := json.Unmarshal(body, &chrome); err != nil {
		return nil, fmt.Errorf("parsing menu chrome: %w", err)
	}
	clusters := collectClusters(chrome)
	if len(clusters) == 0 {
		return nil, fmt.Errorf("no meal clusters found in menu for %s (is %s a valid delivery date?)", date, date)
	}

	byID := map[int]Meal{}
	for _, cl := range clusters {
		q := url.Values{}
		for k, v := range cl.params {
			q.Set(k, v)
		}
		clPath := cl.path
		if !strings.HasPrefix(clPath, "/sdui-service") {
			clPath = "/sdui-service" + clPath
		}
		cbody, cstatus, cerr := c.get(ctx, clPath+"?"+q.Encode())
		if cerr != nil {
			return nil, fmt.Errorf("fetching cluster %s: %w", cl.params["filterBy"], cerr)
		}
		if cstatus == http.StatusUnauthorized || cstatus == http.StatusForbidden {
			// Auth failure on a cluster means the token expired mid-sync.
			return nil, fmt.Errorf("unauthorized (HTTP %d) fetching cluster %s: the CookUnity token expired mid-sync — re-copy it into COOKUNITY_TOKEN", cstatus, cl.params["filterBy"])
		}
		if cstatus != http.StatusOK {
			// Reject the whole fetch on ANY cluster failure. Silently skipping
			// a failed cluster would return a partial menu that the caller
			// treats as complete, corrupting both the current table and the
			// per-week snapshot (drift would then report the missing cluster's
			// meals as "removed"). A complete menu or an error, never a
			// partial success.
			return nil, fmt.Errorf("cluster %s returned HTTP %d; menu is incomplete — not syncing a partial menu, re-run 'sync'", cl.params["filterBy"], cstatus)
		}
		var cnode any
		if err := json.Unmarshal(cbody, &cnode); err != nil {
			return nil, fmt.Errorf("parsing cluster %s: %w (menu is incomplete — not syncing a partial menu)", cl.params["filterBy"], err)
		}
		for _, props := range collectMealProperties(cnode) {
			m := flattenMeal(props, date)
			if m.Id != 0 {
				byID[m.Id] = m
			}
		}
	}

	// Clusters were found but no meals extracted at all — treat as an error so
	// sync does not wipe the local catalog and write an empty snapshot week.
	if len(byID) == 0 {
		return nil, fmt.Errorf("menu for %s had %d clusters but yielded no meals (menu shape may have changed, or the token lacks access)", date, len(clusters))
	}

	meals := make([]Meal, 0, len(byID))
	for _, m := range byID {
		meals = append(meals, m)
	}
	sort.Slice(meals, func(i, j int) bool { return meals[i].Id < meals[j].Id })
	return meals, nil
}

// collectClusters walks the SDUI chrome tree collecting FULL_MENU_LAZY_CLUSTER
// lazy-loader parameters.
func collectClusters(node any) []cluster {
	var out []cluster
	var walk func(any)
	walk = func(n any) {
		switch v := n.(type) {
		case map[string]any:
			if v["type"] == "FULL_MENU_LAZY_CLUSTER" {
				if attrs, ok := v["attributes"].(map[string]any); ok {
					if lazy, ok := attrs["lazyCluster"].(map[string]any); ok {
						if la, ok := lazy["attributes"].(map[string]any); ok {
							path, _ := la["path"].(string)
							params := map[string]string{}
							if p, ok := la["params"].(map[string]any); ok {
								for k, val := range p {
									params[k] = toStr(val)
								}
							}
							if path != "" && len(params) > 0 {
								out = append(out, cluster{path: path, params: params})
							}
						}
					}
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(node)
	return out
}

// collectMealProperties walks a cluster response tree collecting the
// `properties` object of every MEAL-typed node.
func collectMealProperties(node any) []map[string]any {
	var out []map[string]any
	var walk func(any)
	walk = func(n any) {
		switch v := n.(type) {
		case map[string]any:
			if v["type"] == "MEAL" {
				if props, ok := v["properties"].(map[string]any); ok {
					out = append(out, props)
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(node)
	return out
}

// flattenMeal converts a MEAL.properties object into a clean Meal.
func flattenMeal(p map[string]any, date string) Meal {
	m := types.Meal{
		Id:                        toInt(p["id"]),
		InventoryId:               toStr(p["inventoryId"]),
		Sku:                       toStr(p["sku"]),
		Name:                      toStr(p["name"]),
		Description:               toStr(p["description"]),
		Price:                     toFloat(p["price"]),
		FinalPrice:                toFloat(p["finalPrice"]),
		Calories:                  toInt(p["calories"]),
		Stars:                     toFloat(p["stars"]),
		Reviews:                   toInt(p["reviews"]),
		Stock:                     toInt(p["stock"]),
		InStock:                   toBool(p["inStock"]),
		Category:                  toStr(p["category"]),
		IsPremiumMeal:             toBool(p["isPremiumMeal"]),
		PublicUrl:                 toStr(p["publicUrl"]),
		Image:                     toStr(p["image"]),
		MeatType:                  toStr(p["meatType"]),
		Weight:                    toFloat(p["weight"]),
		DeliveryDate:              date,
		HeatInstructionsMicrowave: toStr(p["heatInstructionsMicrowave"]),
		HeatInstructionsOven:      toStr(p["heatInstructionsOven"]),
	}
	// chef {id, firstname, lastname}
	if chef, ok := p["chef"].(map[string]any); ok {
		m.ChefId = toInt(chef["id"])
		fn := strings.TrimSpace(toStr(chef["firstname"]) + " " + toStr(chef["lastname"]))
		m.ChefName = fn
	}
	// nutritionalInfo (values may be JSON strings)
	if ni, ok := p["nutritionalInfo"].(map[string]any); ok {
		m.Protein = toFloat(ni["protein"])
		m.Carbs = toFloat(ni["carbs"])
		m.Fat = toFloat(ni["fat"])
		m.Fiber = toFloat(ni["fiber"])
		m.Sodium = toFloat(ni["sodium"])
		m.Sugar = toFloat(ni["sugar"])
		m.SaturatedFat = toFloat(ni["saturatedFat"])
		m.ServingSize = toStr(ni["servingSize"])
	} else {
		// fall back to top-level ratio fields
		m.Protein = toFloat(p["ratioProtein"])
		m.Carbs = toFloat(p["ratioCarb"])
		m.Fat = toFloat(p["ratioFat"])
	}
	m.Cuisines = joinStrings(p["cuisines"])
	m.DietTags = joinStrings(p["filterBy"])
	m.Allergens = joinNamed(p["allergens"], "name")
	m.Ingredients = joinNamed(p["ingredients"], "name")
	return Meal{Meal: m, IsFavorite: toBool(p["isFavorite"])}
}

// --- conversion helpers (tolerant of JSON strings vs numbers) ---

func toStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", x)
	}
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}

func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			f, ferr := strconv.ParseFloat(strings.TrimSpace(x), 64)
			if ferr != nil {
				return 0
			}
			return int(f)
		}
		return n
	default:
		return 0
	}
}

func toBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true" || x == "1"
	case float64:
		return x != 0
	default:
		return false
	}
}

// joinStrings joins a []string (or []any of strings) with ", ".
func joinStrings(v any) string {
	arr, ok := v.([]any)
	if !ok {
		if s, ok := v.(string); ok {
			return s
		}
		return ""
	}
	parts := make([]string, 0, len(arr))
	for _, e := range arr {
		if s := toStr(e); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

// joinNamed joins the `field` value of each object in a []any with ", ".
func joinNamed(v any, field string) string {
	arr, ok := v.([]any)
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(arr))
	for _, e := range arr {
		if obj, ok := e.(map[string]any); ok {
			if s := toStr(obj[field]); s != "" {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, ", ")
}
