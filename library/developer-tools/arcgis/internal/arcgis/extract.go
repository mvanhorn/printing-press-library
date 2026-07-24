// Copyright 2026 togorashi45 and contributors. Licensed under Apache-2.0. See LICENSE.

package arcgis

import (
	"context"
	"fmt"
)

// PagerMode selects the extraction strategy.
type PagerMode string

const (
	PagerAuto   PagerMode = "auto"   // offset paging if supported, else OID chunking
	PagerOffset PagerMode = "offset" // force resultOffset paging
	PagerOID    PagerMode = "oid"    // force OBJECTID chunking
	PagerTile   PagerMode = "tile"   // recursive envelope subdivision
)

// ExtractOptions controls a full-layer extraction.
type ExtractOptions struct {
	Query    QueryOptions
	Mode     PagerMode
	PageSize int // 0 => use layer maxRecordCount
	Limit    int // 0 => no cap; stop after this many features
	MaxTiles int // safety cap for tile mode; 0 => 4096
}

// Emit is called for each feature during extraction; return an error to stop.
type Emit func(Feature) error

// ExtractAll pulls every matching feature from a layer, choosing the safest
// pagination strategy for the server's capabilities, and calls emit per feature.
// It returns the number of features emitted.
func (c *Client) ExtractAll(ctx context.Context, layerURL string, meta *LayerInfo, o ExtractOptions, emit Emit) (int, error) {
	layerURL = NormalizeLayerURL(layerURL)
	if meta == nil {
		m, err := c.LayerMeta(ctx, layerURL)
		if err != nil {
			return 0, err
		}
		meta = m
	}
	page := o.PageSize
	if page <= 0 {
		page = meta.MaxRecordCount
	}
	if page <= 0 {
		page = 1000
	}
	oidField := meta.ObjectIDField
	if oidField == "" {
		oidField = "OBJECTID"
	}

	mode := o.Mode
	if mode == "" || mode == PagerAuto {
		if meta.Advanced.SupportsPagination {
			mode = PagerOffset
		} else {
			mode = PagerOID
		}
	}

	switch mode {
	case PagerOffset:
		return c.extractOffset(ctx, layerURL, oidField, page, o, emit)
	case PagerOID:
		return c.extractOID(ctx, layerURL, oidField, page, o, emit)
	case PagerTile:
		return c.extractTiled(ctx, layerURL, meta, page, o, emit)
	default:
		return 0, fmt.Errorf("unknown pager mode %q", mode)
	}
}

func (c *Client) extractOffset(ctx context.Context, layerURL, oidField string, page int, o ExtractOptions, emit Emit) (int, error) {
	q := o.Query
	if q.OrderBy == "" {
		q.OrderBy = oidField // deterministic ordering is required for stable paging
	}
	q.ResultCount = page
	count := 0
	offset := 0
	for {
		q.ResultOffset = offset
		res, err := c.QueryPage(ctx, layerURL, q)
		if err != nil {
			return count, err
		}
		for _, f := range res.Features {
			if err := emit(f); err != nil {
				return count, err
			}
			count++
			if o.Limit > 0 && count >= o.Limit {
				return count, nil
			}
		}
		// Rely on exceededTransferLimit, not len(features), to detect the end.
		if !res.ExceededLimit {
			return count, nil
		}
		if len(res.Features) == 0 {
			// Guard against servers that report exceededTransferLimit with an
			// empty page indefinitely.
			offset += page
			if offset > 5_000_000 {
				return count, fmt.Errorf("offset paging exceeded 5M rows without terminating; try --pager oid")
			}
			continue
		}
		offset += len(res.Features)
	}
}

func (c *Client) extractOID(ctx context.Context, layerURL, oidField string, page int, o ExtractOptions, emit Emit) (int, error) {
	ids, field, err := c.IDs(ctx, layerURL, o.Query.Where)
	if err != nil {
		return 0, err
	}
	if field != "" {
		oidField = field
	}
	count := 0
	for start := 0; start < len(ids); start += page {
		end := start + page
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		q := o.Query
		q.Where = fmt.Sprintf("%s>=%d AND %s<=%d", oidField, chunk[0], oidField, chunk[len(chunk)-1])
		q.ResultOffset = 0
		q.ResultCount = 0
		res, err := c.QueryPage(ctx, layerURL, q)
		if err != nil {
			return count, err
		}
		for _, f := range res.Features {
			if err := emit(f); err != nil {
				return count, err
			}
			count++
			if o.Limit > 0 && count >= o.Limit {
				return count, nil
			}
		}
	}
	return count, nil
}

// extractTiled recursively subdivides the layer extent into quadrants until each
// tile's count is under the transfer cap, then pulls each tile. For servers that
// exceed the transfer limit and do not support resultOffset paging.
func (c *Client) extractTiled(ctx context.Context, layerURL string, meta *LayerInfo, page int, o ExtractOptions, emit Emit) (int, error) {
	if meta.Extent == nil {
		return 0, fmt.Errorf("tile mode needs a layer extent; the layer metadata has none")
	}
	maxTiles := o.MaxTiles
	if maxTiles <= 0 {
		maxTiles = 4096
	}
	capN := page
	if capN <= 0 {
		capN = 1000
	}
	count := 0
	tiles := 0
	seen := map[int64]bool{} // dedupe across tile boundaries by OID
	oidField := meta.ObjectIDField
	if oidField == "" {
		oidField = "OBJECTID"
	}

	type box struct{ xmin, ymin, xmax, ymax float64 }
	var pull func(b box) error
	pull = func(b box) error {
		tiles++
		if tiles > maxTiles {
			return fmt.Errorf("tile count exceeded %d; layer too dense, narrow --where or raise --max-tiles", maxTiles)
		}
		env := fmt.Sprintf("%g,%g,%g,%g", b.xmin, b.ymin, b.xmax, b.ymax)
		q := o.Query
		q.Where = andEnvelopeWhere(q.Where)
		q.Geometry = env
		q.GeometryType = "esriGeometryEnvelope"
		q.SpatialRel = "esriSpatialRelIntersects"
		// Probe the tile with a full page: if the server still reports
		// exceededTransferLimit, the tile is too dense and we subdivide.
		q.ResultCount = capN
		res, err := c.QueryPage(ctx, layerURL, q)
		if err != nil {
			return err
		}
		if res.ExceededLimit {
			// subdivide into 4 quadrants
			mx := (b.xmin + b.xmax) / 2
			my := (b.ymin + b.ymax) / 2
			for _, q4 := range []box{
				{b.xmin, b.ymin, mx, my},
				{mx, b.ymin, b.xmax, my},
				{b.xmin, my, mx, b.ymax},
				{mx, my, b.xmax, b.ymax},
			} {
				if err := pull(q4); err != nil {
					return err
				}
			}
			return nil
		}
		for _, f := range res.Features {
			if id, ok := f.OID(oidField); ok {
				if seen[id] {
					continue
				}
				seen[id] = true
			}
			if err := emit(f); err != nil {
				return err
			}
			count++
			if o.Limit > 0 && count >= o.Limit {
				return errStop
			}
		}
		return nil
	}
	e := meta.Extent
	err := pull(box{e.XMin, e.YMin, e.XMax, e.YMax})
	if err == errStop {
		return count, nil
	}
	return count, err
}

var errStop = fmt.Errorf("stop")

func andEnvelopeWhere(where string) string {
	if where == "" {
		return "1=1"
	}
	return where
}
