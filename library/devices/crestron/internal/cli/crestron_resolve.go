// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared model-resolution helpers for the hand-authored Crestron commands.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronparse"
	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronstore"
)

// errNeedsSync marks the soft case where the model may well exist but the local
// catalog has not been built, so the answer is "cannot determine yet" rather
// than "wrong input". Callers report it as an empty result with exit 0.
// A model that Crestron itself does not know is a plain error and exits non-zero.
var errNeedsSync = errors.New("needs sync")

// needsSync reports whether an error is the unsynced-catalog case.
func needsSync(err error) bool { return errors.Is(err, errNeedsSync) }

// resolvedProduct is a model located either in the local mirror or live.
type resolvedProduct struct {
	Model      string
	URL        string
	DocumentID string
	Source     string
}

// openMirror opens the local mirror if it exists. A missing mirror is not an
// error: the commands that use it degrade to live lookups.
func openMirror(ctx context.Context, dbPath string) (*crestronstore.Store, bool) {
	if dbPath == "" {
		dbPath = defaultDBPath("crestron-pp-cli")
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, false
	}
	st, err := crestronstore.Open(ctx, dbPath)
	if err != nil {
		return nil, false
	}
	return st, true
}

// resolveModel finds a model's product page.
//
// Crestron product URLs cannot be constructed from a model number — the path
// encodes the full category chain — so the model must be looked up. The local
// mirror answers instantly when synced; otherwise the model's spec sheet is
// found via resource search, which at least confirms the model exists and
// yields its document id where possible.
func resolveModel(ctx context.Context, c apiGetter, st *crestronstore.Store, model string) (resolvedProduct, error) {
	out := resolvedProduct{Model: model}
	if st != nil {
		if p, ok, err := st.FindProduct(ctx, model); err == nil && ok {
			out.Model = p.Model
			out.URL = strings.TrimPrefix(p.URL, "https://www.crestron.com")
			out.DocumentID = p.DocumentID
			out.Source = "local"
			if out.URL != "" || out.DocumentID != "" {
				return out, nil
			}
		}
	}
	// Live fallback: the catalog has no server-rendered product search, so use
	// the spec-sheet category to confirm the model exists.
	body, err := c.Get(ctx, "/Support/Search-Results", map[string]string{
		"q": model, "c": "5", "p": "1", "m": "10",
	})
	if err != nil {
		return out, fmt.Errorf("looking up %s: %w", model, err)
	}
	page, err := crestronparse.ParseSearchResults(body)
	if err != nil {
		return out, fmt.Errorf("parsing lookup for %s: %w", model, err)
	}
	if page.Count == 0 {
		return out, fmt.Errorf("model %q not found on crestron.com; run 'crestron-pp-cli sync' to build the local catalog, or check the model number", model)
	}
	// The model exists, but Crestron has no server-rendered product search, so
	// its catalog path is only available once the mirror has been synced.
	out.Source = "live"
	return out, fmt.Errorf("%w: %q exists but its catalog path is only available after 'crestron-pp-cli sync'", errNeedsSync, model)
}

// fetchProductPage retrieves and parses a product page by its catalog path.
func fetchProductPage(ctx context.Context, c apiGetter, path string) (crestronparse.Product, []crestronparse.SpecSection, error) {
	if path == "" {
		return crestronparse.Product{}, nil, fmt.Errorf("no product page path known; run 'crestron-pp-cli sync' first")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	body, err := c.Get(ctx, path, nil)
	if err != nil {
		return crestronparse.Product{}, nil, err
	}
	p, err := crestronparse.ParseProductPage(body, path)
	if err != nil {
		return crestronparse.Product{}, nil, err
	}
	specs, _ := crestronparse.ParseSpecTable(body)
	return p, specs, nil
}

// fetchAssets retrieves a product's documentation assets by document id.
func fetchAssets(ctx context.Context, c apiGetter, documentID string) ([]crestronparse.Asset, error) {
	if documentID == "" {
		return nil, fmt.Errorf("no document id supplied")
	}
	body, err := c.Get(ctx, "/Handlers/ResourceHandler.ashx", map[string]string{"dID": documentID})
	if err != nil {
		return nil, err
	}
	return crestronparse.ParseAssets(body)
}

// assetsForModel returns a model's documentation assets.
//
// The per-product handler is preferred because it returns the curated set the
// product page shows. When the document id is unknown — no local mirror, or the
// model lives in an unsynced category — it falls back to searching the resource
// library across all categories for the model, which finds the same public
// /getmedia/ assets by name.
func assetsForModel(ctx context.Context, c apiGetter, model, documentID string) ([]crestronparse.Asset, string, error) {
	if documentID != "" {
		assets, err := fetchAssets(ctx, c, documentID)
		if err == nil && len(assets) > 0 {
			return assets, "product-handler", nil
		}
	}
	body, err := c.Get(ctx, "/Support/Search-Results", map[string]string{
		"q": model, "c": "0", "p": "1", "m": "50",
	})
	if err != nil {
		return nil, "", fmt.Errorf("searching resources for %s: %w", model, err)
	}
	page, err := crestronparse.ParseSearchResults(body)
	if err != nil {
		return nil, "", fmt.Errorf("parsing resource search for %s: %w", model, err)
	}
	out := make([]crestronparse.Asset, 0, page.Count)
	seen := map[string]bool{}
	for _, r := range page.Results {
		// Only public /getmedia/ assets are downloadable; firmware detail pages
		// are auth-gated and are not submittal documents.
		if !strings.HasPrefix(r.URL, "/getmedia/") || seen[r.URL] {
			continue
		}
		seen[r.URL] = true
		out = append(out, crestronparse.Asset{
			Title: r.Title, URL: r.URL, Kind: crestronparse.ClassifyAssetTitle(r.Title),
		})
	}
	if len(out) == 0 {
		return out, "resource-search", fmt.Errorf("no public documentation assets found for %q", model)
	}
	return out, "resource-search", nil
}

var _ = json.Marshal
