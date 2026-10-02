// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/cycling"
	"github.com/spf13/cobra"
)

type hcOptions struct {
	liveOnly bool
	offline  bool
	db       string
	age      time.Duration
	limit    int
	vehicle  string
}

// hcPrint satisfies the generated helpers' exact two-key envelope contract.
func hcPrint(c *cobra.Command, f *rootFlags, payload map[string]any) error {
	b, e := json.Marshal(payload["meta"])
	if e != nil {
		return e
	}
	meta := map[string]any{}
	if e = json.Unmarshal(b, &meta); e != nil {
		return e
	}
	jst := time.FixedZone("JST", 9*60*60)
	for _, key := range []string{"observed_at", "evaluated_at"} {
		if value, ok := meta[key].(string); ok {
			if ts, e := time.Parse(time.RFC3339Nano, value); e == nil {
				meta[key+"_jst"] = ts.In(jst).Format(time.RFC3339)
			}
		}
	}
	results, hasResults := payload["results"]
	if hasResults {
		for k, v := range payload {
			if k != "meta" && k != "results" {
				meta[k] = v
			}
		}
	} else {
		results = map[string]any{}
		for k, v := range payload {
			if k != "meta" {
				results.(map[string]any)[k] = v
			}
		}
	}
	out := map[string]any{"meta": meta, "results": results}
	copyFlags := *f
	if f.selectFields != "" {
		data, e := json.Marshal(out)
		if e != nil {
			return e
		}
		projected, e := filterFieldsChecked(data, f.selectFields)
		if e != nil {
			return usageErr(e)
		}
		var chosen map[string]any
		if e = json.Unmarshal(projected, &chosen); e != nil {
			return e
		}
		if chosenResults, ok := chosen["results"]; ok {
			out["results"] = chosenResults
		} else {
			out["results"] = chosen
		}
		// Preserve source context even when only row fields were requested.
		copyFlags.selectFields = ""
	}
	return copyFlags.printJSON(c, out)
}

func hcPath(o hcOptions, f *rootFlags) string {
	if o.db != "" {
		return o.db
	}
	return defaultDBPath("hello-cycling")
}

func hcLiveOnly(f *rootFlags) error {
	if f.dataSource == "local" {
		return usageErr(fmt.Errorf("this command requires live source data; use --data-source live or auto"))
	}
	return nil
}
func hcValidate(o hcOptions, max int) error {
	if e := cycling.ValidateLimit(o.limit, max); e != nil {
		return usageErr(e)
	}
	if o.age < time.Second || o.age > 24*time.Hour {
		return usageErr(fmt.Errorf("--status-max-age must be between 1s and 24h"))
	}
	if len(o.vehicle) > 128 {
		return usageErr(fmt.Errorf("--vehicle-type is too long; inspect vehicles rules"))
	}
	return nil
}
func hcContext(cmd *cobra.Command, f *rootFlags) (context.Context, context.CancelFunc) {
	timeout := f.timeout
	if timeout <= 0 || timeout > 60*time.Second {
		timeout = 60 * time.Second
	}
	bounded := *f
	bounded.timeout = timeout
	return boundCtx(cmd.Context(), &bounded)
}

var hcFetchSnapshot = func(ctx context.Context) (cycling.Snapshot, error) {
	return cycling.NewClient().Fetch(ctx)
}

func hcSnapshot(cmd *cobra.Command, f *rootFlags, o hcOptions) (cycling.Snapshot, string, error) {
	if o.offline && f.dataSource == "live" {
		return cycling.Snapshot{}, "local", usageErr(fmt.Errorf("--offline conflicts with --data-source live; use local or auto"))
	}
	if o.offline || f.dataSource == "local" {
		s, e := cycling.LoadSnapshot(hcPath(o, f))
		if e == nil && f.maxAge > 0 && time.Since(s.ObservedAt) > f.maxAge {
			s.Warnings = append(s.Warnings, "offline snapshot exceeds --max-age; run stations sync to refresh")
		}
		cmd.Annotations["pp:data-source"] = "local"
		return s, "local", e
	}
	ctx, cancel := hcContext(cmd, f)
	defer cancel()
	s, e := hcFetchSnapshot(ctx)
	var h *cycling.HTTPError
	protectedFailure := errors.As(e, &h) && (h.Status == 401 || h.Status == 403 || h.Status == 429)
	if e != nil && f.dataSource == "auto" && !f.noCache && !o.liveOnly && !protectedFailure {
		if cached, cacheErr := cycling.LoadSnapshot(hcPath(o, f)); cacheErr == nil && len(cached.Information) > 0 && len(cached.Statuses) > 0 {
			cached.Warnings = append(cached.Warnings, "live source failed; using explicit saved snapshot: "+e.Error())
			cmd.Annotations["pp:data-source"] = "local"
			return cached, "local", nil
		}
	}
	var statusErr *cycling.StatusFeedError
	if e != nil && !o.liveOnly && !protectedFailure && errors.As(e, &statusErr) && len(s.Information) > 0 {
		// No usable saved fallback was selected. Keep station discovery and
		// explicit source_missing states, but never save or compare this partial
		// observation as a successful new availability snapshot.
		return s, "live", nil
	}
	return s, "live", e
}
func hcErr(e error) error {
	var typed *cliError
	if errors.As(e, &typed) {
		return e
	}
	var h *cycling.HTTPError
	if errors.As(e, &h) {
		return classifyAPIErrorOnly(e)
	}
	return apiErr(e)
}
func hcKnownVehicle(s cycling.Snapshot, typ string) error {
	if typ == "" {
		return nil
	}
	if len(s.Vehicles) == 0 {
		return usageErr(fmt.Errorf("selected vehicle type cannot be verified because vehicle_types is unavailable; retry or inspect vehicles rules"))
	}
	for _, v := range s.Vehicles {
		if v.ID == typ {
			return nil
		}
	}
	return usageErr(fmt.Errorf("--vehicle-type %q is not published; use vehicles rules", typ))
}
func hcResult(s cycling.Snapshot, source string, rows any, total int) map[string]any {
	return map[string]any{"meta": s.Meta(source, time.Now().UTC()), "results": rows, "total_matches": total}
}

func newHCPriceAreas(f *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "areas", Short: "List price areas advertised by the official source.", Example: "  hello-cycling-pp-cli pricing areas --agent", Args: cobra.NoArgs, Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}}
	c.RunE = func(c *cobra.Command, args []string) error {
		if e := hcLiveOnly(f); e != nil {
			return e
		}
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "pricing areas")
		}
		ctx, cancel := hcContext(c, f)
		defer cancel()
		areas, n, e := cycling.NewClient().PriceAreas(ctx)
		if e != nil {
			return hcErr(e)
		}
		return hcPrint(c, f, map[string]any{"meta": map[string]any{"source": "live", "source_url": cycling.PriceURL, "observed_at": time.Now().UTC(), "request_count": 1, "response_bytes": n}, "results": areas})
	}
	return c
}
func newHCRules(f *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "rules", Short: "Inspect generic GBFS classes and source vehicle eligibility boundaries.", Example: "  hello-cycling-pp-cli vehicles rules --agent", Args: cobra.NoArgs, Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}}
	c.RunE = func(c *cobra.Command, args []string) error {
		if e := hcLiveOnly(f); e != nil {
			return e
		}
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "vehicles rules")
		}
		ctx, cancel := hcContext(c, f)
		defer cancel()
		out, e := cycling.NewClient().VehicleRules(ctx)
		if e != nil {
			return hcErr(e)
		}
		return hcPrint(c, f, out)
	}
	return c
}
func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		for _, parent := range root.Commands() {
			switch parent.Name() {
			case "pricing":
				addNovelCommandIfAbsent(parent, newHCPriceAreas(f))
			}
		}
		vehicles := &cobra.Command{Use: "vehicles", Short: "Source vehicle classes and model compatibility boundaries."}
		vehicles.AddCommand(newHCRules(f))
		addNovelCommandIfAbsent(root, vehicles)
	})
}
