package craigslist

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Category is one entry from reference.craigslist.org/Categories. 178 entries as of 2026.
// Type values: H=housing, J=jobs, S=for sale, B=services/biz, C=community, G=gigs,
// E=events, R=resumes, L=calendar.
type Category struct {
	Abbreviation string `json:"abbreviation"`
	CategoryID   int    `json:"categoryId"`
	Description  string `json:"description"`
	Type         string `json:"type"`
}

// rawCategory matches the on-the-wire (PascalCase) shape Craigslist returns.
type rawCategory struct {
	Abbreviation string `json:"Abbreviation"`
	CategoryID   int    `json:"CategoryID"`
	Description  string `json:"Description"`
	Type         string `json:"Type"`
}

// GetCategories returns all 178 category entries from reference.craigslist.org/Categories.
// Cacheable for 30 days per Craigslist's own Cache-Control header.
func (c *Client) GetCategories(ctx context.Context) ([]Category, error) {
	body, err := c.RawGet(ctx, HostReference, "/Categories", nil)
	if err != nil {
		return nil, err
	}
	var raw []rawCategory
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode Categories: %w", err)
	}
	out := make([]Category, 0, len(raw))
	for _, r := range raw {
		out = append(out, Category{
			Abbreviation: r.Abbreviation,
			CategoryID:   r.CategoryID,
			Description:  r.Description,
			Type:         r.Type,
		})
	}
	return out, nil
}

func (c *Client) cachedCategoryAbbrs(ctx context.Context) (map[int]string, error) {
	c.cacheMu.Lock()
	if c.categoriesLoaded {
		abbrs := c.categoryAbbrs
		c.cacheMu.Unlock()
		return abbrs, nil
	}
	cats, err := c.GetCategories(ctx)
	if err != nil {
		c.cacheMu.Unlock()
		return nil, err
	}
	abbrs := make(map[int]string, len(cats))
	for _, cat := range cats {
		abbrs[cat.CategoryID] = cat.Abbreviation
	}

	c.categoryAbbrs = abbrs
	c.categoriesLoaded = true
	c.cacheMu.Unlock()
	return abbrs, nil
}

// Area is one entry from reference.craigslist.org/Areas. 707 entries as of 2026.
type Area struct {
	AreaID           int       `json:"areaId"`
	Abbreviation     string    `json:"abbreviation"`
	Hostname         string    `json:"hostname"`
	Description      string    `json:"description"`
	ShortDescription string    `json:"shortDescription"`
	Country          string    `json:"country"`
	Region           string    `json:"region"`
	Latitude         float64   `json:"lat"`
	Longitude        float64   `json:"lng"`
	Timezone         string    `json:"timezone"`
	SubAreas         []SubArea `json:"subAreas,omitempty"`
}

// SubArea is one of the subareas in an Area's SubAreas list.
type SubArea struct {
	SubAreaID        int    `json:"subAreaId"`
	Abbreviation     string `json:"abbreviation"`
	Description      string `json:"description"`
	ShortDescription string `json:"shortDescription"`
}

type rawArea2 struct {
	Abbreviation     string        `json:"Abbreviation"`
	AreaID           int           `json:"AreaID"`
	Country          string        `json:"Country"`
	Description      string        `json:"Description"`
	Hostname         string        `json:"Hostname"`
	Latitude         float64       `json:"Latitude"`
	Longitude        float64       `json:"Longitude"`
	Region           string        `json:"Region"`
	ShortDescription string        `json:"ShortDescription"`
	SubAreas         []rawSubArea2 `json:"SubAreas"`
	Timezone         string        `json:"Timezone"`
}

type rawSubArea2 struct {
	Abbreviation     string `json:"Abbreviation"`
	Description      string `json:"Description"`
	ShortDescription string `json:"ShortDescription"`
	SubAreaID        int    `json:"SubAreaID"`
}

// GetAreas returns all 707 area entries from reference.craigslist.org/Areas.
// The response is gzip-encoded; net/http handles transparent decoding.
func (c *Client) GetAreas(ctx context.Context) ([]Area, error) {
	body, err := c.RawGet(ctx, HostReference, "/Areas", nil)
	if err != nil {
		return nil, err
	}
	var raw []rawArea2
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode Areas: %w", err)
	}
	out := make([]Area, 0, len(raw))
	for _, r := range raw {
		a := Area{
			AreaID:           r.AreaID,
			Abbreviation:     r.Abbreviation,
			Hostname:         r.Hostname,
			Description:      r.Description,
			ShortDescription: r.ShortDescription,
			Country:          r.Country,
			Region:           r.Region,
			Latitude:         r.Latitude,
			Longitude:        r.Longitude,
			Timezone:         r.Timezone,
		}
		for _, s := range r.SubAreas {
			a.SubAreas = append(a.SubAreas, SubArea{
				SubAreaID:        s.SubAreaID,
				Abbreviation:     s.Abbreviation,
				Description:      s.Description,
				ShortDescription: s.ShortDescription,
			})
		}
		out = append(out, a)
	}
	return out, nil
}

func (c *Client) cachedAreaIndex(ctx context.Context) (map[string]Area, error) {
	c.cacheMu.Lock()
	if c.areasLoaded {
		areaByKey := c.areaByKey
		c.cacheMu.Unlock()
		return areaByKey, nil
	}
	areas, err := c.GetAreas(ctx)
	if err != nil {
		c.cacheMu.Unlock()
		return nil, err
	}
	areaByKey := make(map[string]Area, len(areas)*2)
	for _, area := range areas {
		areaByKey[normalizeSiteKey(area.Hostname)] = area
		areaByKey[strings.ToLower(area.Abbreviation)] = area
	}

	c.areaByKey = areaByKey
	c.areasLoaded = true
	c.cacheMu.Unlock()
	return areaByKey, nil
}

// ResolveArea finds a Craigslist area by hostname or abbreviation. Common user
// inputs include hostnames ("sfbay", "portland") and abbreviations ("nyc").
func (c *Client) ResolveArea(ctx context.Context, site string) (Area, bool, error) {
	key := normalizeSiteKey(site)
	if key == "" {
		return Area{}, false, nil
	}
	areaByKey, err := c.cachedAreaIndex(ctx)
	if err != nil {
		return Area{}, false, err
	}
	area, ok := areaByKey[key]
	if !ok {
		return Area{}, false, nil
	}
	return area, true, nil
}

func normalizeSiteKey(site string) string {
	key := strings.ToLower(strings.TrimSpace(site))
	key = strings.TrimPrefix(key, "https://")
	key = strings.TrimPrefix(key, "http://")
	key = strings.TrimSuffix(key, "/")
	key = strings.TrimSuffix(key, ".craigslist.org")
	return key
}
