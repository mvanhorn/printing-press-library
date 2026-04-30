package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// resolveCategoryID accepts either a numeric category id (e.g. "7") or a
// category slug (e.g. "payments", "developer-productivity") and returns the
// numeric id the proxy expects on browse / top / velocity / publishers
// commands. Returns 0 and a nil error when the input is empty so callers can
// treat that as "no filter".
//
// All-digit input short-circuits without a network call. For slugs we hit
// /v2/api/category once to build the lookup map, then match either against
// the URL slug returned by that endpoint or the entity-embedded slug stored
// on each category record.
func resolveCategoryID(c *categoryResolverClient, raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	if id, err := strconv.Atoi(raw); err == nil {
		return id, nil
	}
	categories, err := c.listCategories()
	if err != nil {
		return 0, fmt.Errorf("looking up category slug %q: %w (use --category 7 with a numeric id instead)", raw, err)
	}
	wanted := strings.ToLower(raw)
	wantedNoSuffix := strings.TrimSuffix(wanted, "-apis")
	for _, cat := range categories {
		slug := strings.ToLower(cat.Slug)
		if slug == wanted || strings.TrimSuffix(slug, "-apis") == wantedNoSuffix {
			return cat.ID, nil
		}
		// Some entity payloads embed the short slug ("payments") even though
		// the listing uses the long slug ("payments-apis"). Match the short
		// form too.
		if strings.ToLower(cat.Name) == wanted {
			return cat.ID, nil
		}
	}
	return 0, fmt.Errorf("no category matches %q — run 'category list-categories' to see valid slugs and IDs", raw)
}

// categoryResolverClient is the slice of *client.Client behavior the resolver
// needs. We keep it as a small interface so unit tests can substitute a fake
// listCategories without standing up a real HTTP client.
type categoryResolverClient struct {
	listCategories func() ([]categorySummary, error)
}

type categorySummary struct {
	ID   int    `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// newCategoryResolverFromFlags returns a resolver bound to the live API client
// behind the user's rootFlags. Used by the novel commands that take --category.
func newCategoryResolverFromFlags(flags *rootFlags) (*categoryResolverClient, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	return &categoryResolverClient{
		listCategories: func() ([]categorySummary, error) {
			data, err := c.Get("/v2/api/category", nil)
			if err != nil {
				return nil, err
			}
			var resp struct {
				Data []categorySummary `json:"data"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return nil, fmt.Errorf("decoding category list: %w", err)
			}
			return resp.Data, nil
		},
	}, nil
}
