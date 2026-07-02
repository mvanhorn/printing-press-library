// Copyright 2026 wandreis and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"

	"github.com/mvanhorn/printing-press-library/library/commerce/mercadolivre/internal/mlextract"
	"github.com/mvanhorn/printing-press-library/library/commerce/mercadolivre/internal/store"
)

// openLocalStore opens the default local SQLite store. Callers own Close.
func openLocalStore(ctx context.Context) (*store.Store, error) {
	return store.OpenWithContext(ctx, defaultDBPath("mercadolivre-pp-cli"))
}

// persistListings upserts parsed search listings into the local store and
// records one price snapshot per listing that carries a price. Best-effort:
// returns the first error but never blocks command output.
func persistListings(ctx context.Context, listings []mlextract.Listing) error {
	if len(listings) == 0 {
		return nil
	}
	s, err := openLocalStore(ctx)
	if err != nil {
		return err
	}
	defer s.Close()

	items := make([]json.RawMessage, 0, len(listings))
	for _, l := range listings {
		if l.CatalogID == "" {
			continue
		}
		items = append(items, l.StoreJSON())
	}
	if len(items) > 0 {
		if _, _, err := s.UpsertBatch("listings", items); err != nil {
			return err
		}
	}
	for _, l := range listings {
		if l.CatalogID == "" || l.Price <= 0 {
			continue
		}
		if err := s.InsertPriceSnapshot(l.CatalogID, l.Price, l.Currency); err != nil {
			return err
		}
	}
	return nil
}

// persistProduct upserts a parsed product, its spec attributes, and a price
// snapshot into the local store. Best-effort.
func persistProduct(ctx context.Context, p *mlextract.ProductDetail, attrs []mlextract.Attribute) error {
	if p == nil || p.CatalogID == "" {
		return nil
	}
	s, err := openLocalStore(ctx)
	if err != nil {
		return err
	}
	defer s.Close()

	if _, _, err := s.UpsertBatch("products", []json.RawMessage{p.StoreJSON()}); err != nil {
		return err
	}
	if len(attrs) > 0 {
		if err := s.UpsertAttributes(p.CatalogID, attrs); err != nil {
			return err
		}
	}
	if p.Price > 0 {
		if err := s.InsertPriceSnapshot(p.CatalogID, p.Price, p.Currency); err != nil {
			return err
		}
	}
	return nil
}
