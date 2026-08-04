// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package granola

import (
	"errors"
	"fmt"
)

// PATCH(api-catalog-refresh): refresh recipes, panel templates and folders
// over the internal API so a degraded sync stops serving a frozen catalog.
//
// The transcript hydrate ahead of this one restored meeting content, but every
// catalog surface still came from the desktop cache alone. On a migrated
// install that cache can never be read again, so `recipes list`, `panel`,
// `folder list` and the memo runner keep answering with whatever was captured
// on the last successful decrypt -- and they answer confidently, with no sign
// the data stopped moving. A folder created since then simply does not exist
// as far as the CLI is concerned.
//
// Unlike transcripts and panels, each of these surfaces is one call for the
// whole set. There is no per-document fan-out, so there is deliberately no
// budget, no fetch-state table and no resume: a pass either refreshes a
// surface completely or leaves it as it was.

// CatalogFetcher is the slice of the internal API this refresh needs.
// Narrowing it to an interface keeps the derivation and merge logic testable
// without a live Granola account, exactly as TranscriptFetcher does.
// *InternalClient satisfies it.
type CatalogFetcher interface {
	GetRecipes() (RecipeCatalog, error)
	GetPanelTemplates() ([]PanelTemplate, error)
	GetDocumentLists() ([]DocumentListMetadata, error)
}

// CatalogHydrateResult reports what one pass put into the Cache. The counts
// are of records staged for the sync, not of rows written: HydrateCatalogFromAPI
// writes nothing to the domain tables itself.
type CatalogHydrateResult struct {
	// Recipes is the total across the public, user and shared buckets.
	Recipes int
	// RecipeUsages is the number of usage counters staged.
	RecipeUsages int
	// PanelTemplates is the number of templates staged.
	PanelTemplates int
	// Folders and Memberships come from the same single call: the API returns
	// each list's membership inline.
	Folders     int
	Memberships int

	// Per-surface failures. Each is independent: one surface failing must not
	// cost the caller the other two, because the three are unrelated
	// endpoints that happen to be refreshed together.
	RecipesErr        error
	PanelTemplatesErr error
	DocumentListsErr  error
}

// Err joins the per-surface failures into one value for callers that only
// need to know whether anything went wrong. Nil when every surface succeeded.
func (r CatalogHydrateResult) Err() error {
	return errors.Join(r.RecipesErr, r.PanelTemplatesErr, r.DocumentListsErr)
}

// HydrateCatalogFromAPI fills the catalog surfaces of cache -- recipes and
// their usage counters, panel templates, folder metadata and folder
// membership -- from the internal API, then lets SyncFromCache's existing
// loops persist them. It writes nothing to the domain tables itself, which
// keeps catalog writes on one code path regardless of whether the cache was
// readable.
//
// The returned error is the joined per-surface failure and is non-fatal by
// design: a 401 on one endpoint leaves the surfaces that did answer staged and
// the rest of the sync intact. Callers surface it as a warning.
func HydrateCatalogFromAPI(cache *Cache, client CatalogFetcher) (CatalogHydrateResult, error) {
	var res CatalogHydrateResult
	if cache == nil {
		return res, fmt.Errorf("nil cache")
	}
	if client == nil {
		ic, err := NewInternalClient()
		if err != nil {
			return res, fmt.Errorf("hydrate catalog: %w", err)
		}
		client = ic
	}
	res.RecipesErr = hydrateRecipes(cache, client, &res)
	res.PanelTemplatesErr = hydratePanelTemplates(cache, client, &res)
	res.DocumentListsErr = hydrateDocumentLists(cache, client, &res)
	return res, res.Err()
}

// hydrateRecipes stages the three recipe buckets the cache also carries, plus
// the usage counters.
func hydrateRecipes(cache *Cache, client CatalogFetcher, res *CatalogHydrateResult) error {
	cat, err := client.GetRecipes()
	if err != nil {
		return fmt.Errorf("refresh recipes: %w", err)
	}
	cache.PublicRecipes = stampRecipes(cat.PublicRecipes, "public")
	cache.UserRecipes = stampRecipes(cat.UserRecipes, "user")
	cache.SharedRecipes = stampRecipes(cat.SharedRecipes, "shared")
	res.Recipes = len(cache.PublicRecipes) + len(cache.UserRecipes) + len(cache.SharedRecipes)

	if cache.RecipesUsage == nil {
		cache.RecipesUsage = map[string]RecipeUsage{}
	}
	for id, u := range cat.Usage {
		cache.RecipesUsage[id] = u
		res.RecipeUsages++
	}
	return nil
}

// stampRecipes replays the two derivations LoadCache applies to every recipe
// bucket. The API payload carries neither: Source is the bucket the recipe
// came out of, which only the reader knows, and a recipe's display name lives
// in its slug when nothing else names it. Skipping either leaves `recipes
// list` unable to filter by source and showing blank names for most rows.
func stampRecipes(in []Recipe, source string) []Recipe {
	if len(in) == 0 {
		return nil
	}
	out := make([]Recipe, len(in))
	copy(out, in)
	for i := range out {
		out[i].Source = source
		if out[i].Name == "" {
			out[i].Name = out[i].Slug
		}
	}
	return out
}

// hydratePanelTemplates stages the panel templates, deriving the slug from the
// title when the payload has none -- the same fallback LoadCache applies,
// and the reason `panel --template <slug>` resolves at all.
func hydratePanelTemplates(cache *Cache, client CatalogFetcher, res *CatalogHydrateResult) error {
	templates, err := client.GetPanelTemplates()
	if err != nil {
		return fmt.Errorf("refresh panel templates: %w", err)
	}
	for i, t := range templates {
		if t.Slug == "" {
			templates[i].Slug = slugify(t.Title)
		}
	}
	cache.PanelTemplates = templates
	res.PanelTemplates = len(templates)
	return nil
}

// hydrateDocumentLists stages folder metadata and folder membership from the
// single /v2/get-document-lists call that carries both.
func hydrateDocumentLists(cache *Cache, client CatalogFetcher, res *CatalogHydrateResult) error {
	lists, err := client.GetDocumentLists()
	if err != nil {
		return fmt.Errorf("refresh folders: %w", err)
	}
	if cache.DocumentListsMetadata == nil {
		cache.DocumentListsMetadata = map[string]DocumentListMetadata{}
	}
	if cache.DocumentLists == nil {
		cache.DocumentLists = map[string][]string{}
	}
	for _, l := range lists {
		if l.ID == "" {
			continue
		}
		cache.DocumentListsMetadata[l.ID] = l
		res.Folders++

		seen := map[string]bool{}
		ids := make([]string, 0, len(l.Documents))
		for _, entry := range l.Documents {
			mid := entry.MeetingID()
			if mid == "" || seen[mid] {
				continue
			}
			seen[mid] = true
			ids = append(ids, mid)
		}
		if len(ids) == 0 {
			// Leave any membership already staged alone rather than replacing
			// it with an empty slice: an empty `documents` array is what a
			// genuinely empty folder and a folder whose membership this
			// payload omits both look like.
			continue
		}
		cache.DocumentLists[l.ID] = ids
		res.Memberships += len(ids)
	}
	return nil
}
