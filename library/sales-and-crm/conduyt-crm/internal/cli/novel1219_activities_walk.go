// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

// PATCH(novel1219-activities-list-page-number): GET /activities answers {"data":{"data":[...],"meta":{page, per_page,
// total, total_capped, next_cursor}}}. It takes a keyset `cursor` (the last shown row's createdAt and id) and a numeric
// `page`. `activities list --all` follows meta.next_cursor through `cursor` while one is returned, stops when it is
// null, and falls back to numeric pages only when a response carries no next_cursor key at all. A cursor is never sent
// as `page`: the route parses page as an integer and a keyset string there is a 500.

type activitiesGetter interface {
	GetWithHeaders(ctx context.Context, path string, params map[string]string, headers map[string]string) (json.RawMessage, error)
	IsDryRun() bool
}

func walkActivitiesAll(ctx context.Context, c activitiesGetter, path string, params map[string]string) (json.RawMessage, error) {
	q := map[string]string{}
	for k, v := range params {
		if v != "" {
			q[k] = v
		}
	}
	perPage := 50
	if n, err := strconv.Atoi(q["per_page"]); err == nil && n > 0 {
		perPage = n
	} else {
		q["per_page"] = strconv.Itoa(perPage)
	}
	if q["cursor"] != "" {
		delete(q, "page")
	} else if p := q["page"]; p != "" {
		if n, err := strconv.Atoi(p); err != nil || n < 1 {
			return nil, fmt.Errorf("--page %q is not a page number", p)
		}
	}
	rows := make([]json.RawMessage, 0)
	seen := map[string]bool{}
	for page := 1; ; page++ {
		data, err := c.GetWithHeaders(ctx, path, q, nil)
		if err != nil {
			return nil, err
		}
		if isDryRunResponse(c.IsDryRun(), data) {
			return data, nil
		}
		var env struct {
			Data *struct {
				Data []json.RawMessage          `json:"data"`
				Meta map[string]json.RawMessage `json:"meta"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &env); err != nil || env.Data == nil || env.Data.Data == nil {
			return nil, fmt.Errorf("GET %s: response is not the activities envelope {data:{data:[...],meta:{...}}}", path)
		}
		rows = append(rows, env.Data.Data...)
		raw, hasKey := env.Data.Meta["next_cursor"]
		next := ""
		if hasKey && string(raw) != "null" {
			if err := json.Unmarshal(raw, &next); err != nil {
				return nil, fmt.Errorf("GET %s: meta.next_cursor is not a string", path)
			}
		}
		switch {
		case q["cursor"] != "" && !hasKey:
			// A keyset page must say where the walk goes next; without a next_cursor key the walk cannot continue
			// (by page number or otherwise) and a short page here is not proof of the end.
			return nil, fmt.Errorf("GET %s: a cursor page returned no next_cursor; refusing to return a partial walk as complete", path)
		case next != "":
			if seen[next] {
				return nil, fmt.Errorf("GET %s: meta.next_cursor repeated; refusing to return a partial walk as complete", path)
			}
			seen[next] = true
			q["cursor"] = next
			delete(q, "page")
		case hasKey:
			return json.Marshal(rows)
		case len(env.Data.Data) >= perPage:
			current := 1
			if n, err := strconv.Atoi(q["page"]); err == nil && n > 0 {
				current = n
			}
			q["page"] = strconv.Itoa(current + 1)
		default:
			return json.Marshal(rows)
		}
		if page >= paginatedGetMaxPages*10 {
			return nil, fmt.Errorf("GET %s: more than %d pages; narrow the filters or raise --per-page", path, paginatedGetMaxPages*10)
		}
	}
}

// resolveActivitiesAll keeps the generated data-source contract for `activities list --all` around the live walker:
// --data-source live returns the walk or its error; auto (the default) writes the walked rows through to the local
// store and, when the API is unreachable, answers from the local store with its provenance, exactly as
// resolvePaginatedReadWithStrategy does for every other --all read. PATCH(novel1219-activities-list-page-number)
func resolveActivitiesAll(ctx context.Context, c activitiesGetter, flags *rootFlags, path string, params map[string]string, hintWriter io.Writer) (json.RawMessage, DataProvenance, error) {
	data, err := walkActivitiesAll(ctx, c, path, params)
	if flags.dataSource == "live" {
		if err != nil {
			return nil, DataProvenance{}, err
		}
		return data, attachFreshness(DataProvenance{Source: "live"}, flags), nil
	}
	if err == nil {
		writeThroughCache(ctx, "activities", data)
		return data, attachFreshness(DataProvenance{Source: "live"}, flags), nil
	}
	if !isNetworkError(err) {
		return nil, DataProvenance{}, err
	}
	fallbackData, fallbackProv, fallbackErr := resolveLocal(ctx, flags, hintWriter, "activities", true, path, params, networkFallbackReason)
	if fallbackErr != nil {
		return nil, DataProvenance{}, localFallbackError(err, fallbackErr)
	}
	return fallbackData, attachFreshness(fallbackProv, flags), nil
}
