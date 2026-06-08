package storefront

import (
	"context"
	"encoding/json"
	"fmt"
)

type SearchParams struct {
	Query      string
	Start      int
	PageSize   int
	SortRule   string
	CategoryID string
	MinPrice   float64
	Prefn1     string
	Prefv1     string
	Prefn2     string
	Prefv2     string
}

type Getter interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

func FetchSuggestions(ctx context.Context, c Getter, query string) (json.RawMessage, error) {
	if c == nil {
		return nil, fmt.Errorf("storefront suggestions fetch: nil client")
	}
	return c.Get(ctx, "/on/demandware.store/Sites-continente-Site/default/SearchServices-GetSuggestions", map[string]string{
		"q": query,
	})
}

func FetchSearch(ctx context.Context, c Getter, params SearchParams) (json.RawMessage, error) {
	if c == nil {
		return nil, fmt.Errorf("storefront search fetch: nil client")
	}
	query := map[string]string{}
	if params.Query != "" {
		query["q"] = params.Query
	}
	if params.Start != 0 {
		query["start"] = fmt.Sprintf("%v", params.Start)
	}
	if params.SortRule != "" {
		query["srule"] = params.SortRule
	}
	if params.CategoryID != "" {
		query["cgid"] = params.CategoryID
	}
	return c.Get(ctx, "/pesquisa/", query)
}

func FetchSearchFragment(ctx context.Context, c Getter, params SearchParams) (json.RawMessage, error) {
	if c == nil {
		return nil, fmt.Errorf("storefront search fragment fetch: nil client")
	}
	query := map[string]string{}
	if params.Query != "" {
		query["q"] = params.Query
	}
	if params.CategoryID != "" {
		query["cgid"] = params.CategoryID
	}
	if params.Start != 0 {
		query["start"] = fmt.Sprintf("%v", params.Start)
	}
	if params.PageSize != 0 {
		query["sz"] = fmt.Sprintf("%v", params.PageSize)
	}
	if params.SortRule != "" {
		query["srule"] = params.SortRule
	}
	if params.MinPrice != 0 {
		query["pmin"] = fmt.Sprintf("%v", params.MinPrice)
	}
	if params.Prefn1 != "" {
		query["prefn1"] = params.Prefn1
	}
	if params.Prefv1 != "" {
		query["prefv1"] = params.Prefv1
	}
	if params.Prefn2 != "" {
		query["prefn2"] = params.Prefn2
	}
	if params.Prefv2 != "" {
		query["prefv2"] = params.Prefv2
	}
	return c.Get(ctx, "/on/demandware.store/Sites-continente-Site/default/Search-ShowAjax", query)
}
