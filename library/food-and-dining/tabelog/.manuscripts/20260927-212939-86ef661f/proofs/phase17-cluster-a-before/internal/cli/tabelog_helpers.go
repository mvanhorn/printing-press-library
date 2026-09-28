package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"tabelog-pp-cli/internal/cliutil"
	"tabelog-pp-cli/internal/domain"
	"tabelog-pp-cli/internal/notebook"
	"tabelog-pp-cli/internal/source"
	"tabelog-pp-cli/internal/store"
)

func tripOpen(ctx context.Context, flags *rootFlags) (*store.Store, *notebook.Store, error) {
	db, e := store.OpenWithContext(ctx, defaultDBPath("tabelog-pp-cli"))
	if e != nil {
		return nil, nil, fmt.Errorf("open local trip store: %w", e)
	}
	nb := notebook.New(db.DB())
	if e = nb.Init(ctx); e != nil {
		db.Close()
		return nil, nil, e
	}
	return db, nb, nil
}
func tripSnapshot(ctx context.Context, nb *notebook.Store, id string) (domain.Restaurant, error) {
	raw, e := nb.Snapshot(ctx, id)
	if e != nil {
		return domain.Restaurant{}, e
	}
	var r domain.Restaurant
	if e = json.Unmarshal(raw, &r); e != nil {
		return domain.Restaurant{}, fmt.Errorf("read saved restaurant snapshot: %w", e)
	}
	if r.ID != id || r.Name == "" || r.URL == "" {
		return domain.Restaurant{}, fmt.Errorf("saved snapshot identity is invalid")
	}
	return r, nil
}
func tripReplace(ctx context.Context, nb *notebook.Store, r domain.Restaurant) error {
	raw, e := json.Marshal(r)
	if e != nil {
		return e
	}
	return nb.ReplaceSnapshot(ctx, r.ID, raw)
}
func tripClient(flags *rootFlags) (*source.Client, error) {
	dir, e := cliutil.CacheDir()
	if e != nil {
		return nil, e
	}
	mode := flags.dataSource
	if flags.noCache {
		if mode == "local" {
			return nil, usageErr(fmt.Errorf("--no-cache conflicts with offline --data-source local"))
		}
		mode = "live"
	}
	return source.NewClient(filepath.Join(dir, "source"), mode)
}
func tripDetail(ctx context.Context, flags *rootFlags, raw string) (domain.Restaurant, error) {
	c, e := tripClient(flags)
	if e != nil {
		return domain.Restaurant{}, e
	}
	return c.FetchDetail(ctx, raw)
}

func tripError(err error) error {
	var re *cliutil.RateLimitError
	if errors.As(err, &re) {
		return rateLimitErr(err)
	}
	var se *source.Error
	if errors.As(err, &se) {
		switch se.Kind {
		case "usage":
			return usageErr(err)
		case "not_found", "cache_miss":
			return notFoundErr(err)
		case "ambiguous":
			return usageErr(err)
		default:
			return apiErr(err)
		}
	}
	return apiErr(err)
}

// Summary shaping saves tokens without changing the source facts. Projection
// always uses the full typed record, and always retains the response metadata.
func tripSummary(v any, meta domain.Meta) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = tripSummary(item, meta)
		}
		return out
	case map[string]any:
		if _, ok := x["source_surface"]; ok && x["id"] != nil && x["name"] != nil && x["url"] != nil {
			keep := []string{"id", "name", "url", "rating", "review_count", "categories", "area", "nearest_station", "nearest_station_distance_m", "lunch_budget", "dinner_budget", "review_lunch_budget", "review_dinner_budget", "fetched_at", "source_surface", "status", "source_warnings", "hours", "payment", "reservation", "address", "transportation", "service_charge", "closures", "awards"}
			out := map[string]any{}
			for _, k := range keep {
				if a, ok := x[k]; ok {
					out[k] = a
				}
			}
			if x["source_url"] != meta.SourceURL && x["source_url"] != nil {
				out["source_url"] = x["source_url"]
			}
			if x["fetched_at"] == meta.FetchedAt.UTC().Format(time.RFC3339Nano) {
				delete(out, "fetched_at")
			}
			if x["source_surface"] == meta.SourceSurface {
				delete(out, "source_surface")
			}
			if meta.BudgetSource != "" {
				for _, key := range []string{"lunch_budget", "dinner_budget"} {
					if budget, ok := out[key].(map[string]any); ok && budget["source"] == meta.BudgetSource {
						delete(budget, "source")
					}
				}
			}
			return out
		}
		for k, a := range x {
			x[k] = tripSummary(a, meta)
		}
		return x
	}
	return v
}

func tripPrint(cmd *cobra.Command, flags *rootFlags, items any, meta domain.Meta) error {
	full, e := json.Marshal(domain.Envelope{Items: items, Meta: meta})
	if e != nil {
		return e
	}
	var obj map[string]json.RawMessage
	if e = json.Unmarshal(full, &obj); e != nil {
		return e
	}
	var selectErr error
	if flags.selectFields != "" && !flags.dryRun {
		var projected json.RawMessage
		projected, selectErr = filterFieldsChecked(full, flags.selectFields)
		var p map[string]json.RawMessage
		if json.Unmarshal(projected, &p) == nil {
			p["meta"] = obj["meta"]
			obj = p
		}
	}
	if flags.selectFields == "" && !strings.HasPrefix(cmd.CommandPath(), cmd.Root().Name()+" lists") {
		var value any
		if json.Unmarshal(obj["items"], &value) == nil {
			obj["items"], e = json.Marshal(tripSummary(value, meta))
			if e != nil {
				return e
			}
		}
	}
	if flags.csv || flags.plain {
		var rows []map[string]any
		if e = json.Unmarshal(obj["items"], &rows); e != nil {
			return usageErr(fmt.Errorf("CSV/plain output requires a collection of objects"))
		}
		keys := map[string]bool{}
		for _, r := range rows {
			for k := range r {
				keys[k] = true
			}
		}
		cols := make([]string, 0, len(keys))
		for k := range keys {
			cols = append(cols, k)
		}
		sort.Strings(cols)
		w := csv.NewWriter(cmd.OutOrStdout())
		if flags.plain {
			w.Comma = '\t'
		}
		if len(cols) > 0 {
			_ = w.Write(cols)
		}
		for _, r := range rows {
			vals := make([]string, len(cols))
			for i, k := range cols {
				if s, ok := r[k].(string); ok {
					vals[i] = s
				} else if v := r[k]; v != nil {
					b, _ := json.Marshal(v)
					vals[i] = string(b)
				}
			}
			if e = w.Write(vals); e != nil {
				return e
			}
		}
		w.Flush()
		if e = w.Error(); e != nil {
			return e
		}
		return selectErr
	}
	if flags.quiet {
		var rows []map[string]any
		if json.Unmarshal(obj["items"], &rows) != nil {
			return usageErr(fmt.Errorf("--quiet requires a collection"))
		}
		for _, r := range rows {
			if id, ok := r["id"]; ok {
				fmt.Fprintln(cmd.OutOrStdout(), id)
			} else if selector, ok := r["selector"]; ok {
				fmt.Fprintln(cmd.OutOrStdout(), selector)
			}
		}
		return selectErr
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	if !flags.agent && !flags.compact {
		enc.SetIndent("", "  ")
	}
	if e = enc.Encode(obj); e != nil {
		return e
	}
	return selectErr
}

func tripReadAnnotations(fixture string) map[string]string {
	return map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": fixture}
}
func tripPlan(cmd *cobra.Command, flags *rootFlags, action string, criteria map[string]any) error {
	if flags.selectFields != "" {
		criteria["select"] = flags.selectFields
	}
	return tripPrint(cmd, flags, []map[string]any{{"dry_run": true, "action": action, "criteria": criteria, "planned_lookups": true}}, domain.Meta{Source: "dry-run", Coverage: "planned", Note: "No source request, cache write or local database access."})
}
func tripSourceFailure(cmd *cobra.Command, flags *rootFlags, err error) error {
	var se *source.Error
	if errors.As(err, &se) && len(se.Choices) > 0 {
		meta := se.Meta
		meta.Returned = len(se.Choices)
		meta.Scanned = len(se.Choices)
		meta.Coverage = "source_choices"
		meta.Note = se.Message
		if e := tripPrint(cmd, flags, se.Choices, meta); e != nil {
			return e
		}
	}
	return tripError(err)
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.CompletionOptions.HiddenDefaultCmd = true
		// Native source operations have a bounded whole-command deadline.
		flags.timeout = 20 * time.Second
		if f := root.PersistentFlags().Lookup("timeout"); f != nil {
			f.DefValue = "20s"
		}
		if f := root.PersistentFlags().Lookup("data-source"); f != nil {
			f.Usage = "auto uses fresh cache or fetches; live bypasses cache; local makes no network requests"
		}
		if f := root.PersistentFlags().Lookup("compact"); f != nil {
			f.Usage = "Compact output; source commands preserve restaurant decision facts"
		}
		if f := root.PersistentFlags().Lookup("max-age"); f != nil {
			f.Usage = "Age threshold for saved-evidence hints; 0 disables age checks"
		}
		visible := map[string]bool{"find": true, "show": true, "areas": true, "cuisines": true, "lists": true, "doctor": true, "version": true, "help": true}
		for _, cmd := range root.Commands() {
			if !visible[cmd.Name()] {
				cmd.Hidden = true
				if cmd.Annotations == nil {
					cmd.Annotations = map[string]string{}
				}
				cmd.Annotations["mcp:hidden"] = "true"
			}
		}
		for _, name := range []string{"receipt", "receipt-file", "audit-dir", "no-input", "yes", "no-color", "human-friendly", "profile", "client-profile", "deliver", "rate-limit", "config"} {
			_ = root.PersistentFlags().MarkHidden(name)
		}
		root.Short = "Find Japan restaurants and bars, then keep a factual trip shortlist."
		root.Long = strings.TrimSpace(root.Short + "\n\nResolve a location, find a few candidates, inspect selected details, and save trip lists. Source ratings, meal budgets, unknown facts and retrieval times remain explicit. Run a command with --help for its criteria and examples.")
	})
}
