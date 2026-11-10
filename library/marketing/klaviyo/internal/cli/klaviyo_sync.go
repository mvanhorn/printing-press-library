// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
	"io"
	"os"
	"strings"
	"time"
)

func syncResource(ctx context.Context, c interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
	RateLimit() float64
}, db *store.Store, resource, sinceTS string, full bool, maxPages int, latestOnly, prune bool, userParams *syncUserParams, syncEvents io.Writer, warningWriters ...io.Writer) syncResult {
	if syncEvents == nil {
		syncEvents = os.Stderr
	}
	if resource == "campaigns" || resource == "campaign-messages" {
		messages, _ := ctx.Value(campaignSyncMessagesKey{}).(*campaignMessageReader)
		if messages == nil {
			messages = &campaignMessageReader{client: c}
		}
		enriched := &campaignSyncClient{reader: c, db: db, resource: resource, messages: messages}
		return syncResourceOnce(ctx, enriched, db, resource, sinceTS, full, maxPages, latestOnly, prune, userParams, syncEvents, warningWriters...)
	}
	filters := []string{}
	supplied := map[string]string{}
	userParams.applyTo(resource, supplied, false)
	if supplied["filter"] != "" {
		return syncResourceOnce(ctx, c, db, resource, sinceTS, full, maxPages, latestOnly, prune, userParams, syncEvents, warningWriters...)
	}
	switch resource {
	case "coupon-codes":
		ids, err := db.ListIDs("coupons")
		if err != nil {
			return syncResult{Resource: resource, Err: err}
		}
		if len(ids) == 0 {
			return syncResult{Resource: resource, Warn: fmt.Errorf("requires synced coupons rows before the coupon.id filter can be built")}
		}
		for _, id := range ids {
			filters = append(filters, fmt.Sprintf("equals(coupon.id,'%s')", strings.ReplaceAll(id, `'`, `\'`)))
		}
	default:
		return syncResourceOnce(ctx, c, db, resource, sinceTS, full, maxPages, latestOnly, prune, userParams, syncEvents, warningWriters...)
	}
	started := time.Now()
	result := syncResult{Resource: resource}
	for _, filter := range filters {
		params := &syncUserParams{perResource: map[string]map[string]string{resource: {"filter": filter}}}
		if userParams != nil {
			params.flatGlobal = userParams.flatGlobal
			params.trueGlobal = userParams.trueGlobal
			for name, values := range userParams.perResource {
				copied := map[string]string{}
				for k, v := range values {
					copied[k] = v
				}
				params.perResource[name] = copied
			}
			if params.perResource[resource] == nil {
				params.perResource[resource] = map[string]string{}
			}
			params.perResource[resource]["filter"] = filter
		}
		next := syncResourceOnce(ctx, c, db, resource, sinceTS, full, maxPages, latestOnly, prune, params, syncEvents, warningWriters...)
		result.Count += next.Count
		if next.Err != nil || next.Warn != nil {
			result.Err = next.Err
			result.Warn = next.Warn
			result.Duration = time.Since(started)
			return result
		}
	}
	if sinceTS == "" {
		result.Err = db.SaveSyncState(resource, "", result.Count)
	}
	result.Duration = time.Since(started)
	return result
}
