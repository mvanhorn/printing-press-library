// Hand-authored provider commands. Preserved through Printing Press regeneration.
// pp:data-source auto
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/ecbo-cloak/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/ecbo-cloak/internal/ecbo"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		// Raw primitives use the same confirmed read-only offer fixture in live checks.
		for _, parent := range root.Commands() {
			if parent.Name() == "source" {
				for _, cmd := range parent.Commands() {
					if cmd.Name() == "price" || cmd.Name() == "validate" {
						cmd.Annotations["pp:happy-args"] = "--space-id=0c3fb1e9-ad5a-42de-bfda-3027ebe4921e;--from=2026-10-03 19:00;--to=2026-10-03 21:00;--reservation-items-small=1;--reservation-items-large=1"
					}
				}
			}
		}

		// Examples use known read-only inputs so every exposed primitive can be exercised live.
		for _, parent := range root.Commands() {
			if parent.Name() == "source" {
				for _, cmd := range parent.Commands() {
					switch cmd.Name() {
					case "detail":
						cmd.Example = "  ecbo-cloak-pp-cli source detail --id 0c3fb1e9-ad5a-42de-bfda-3027ebe4921e --agent"
					case "nearby":
						cmd.Example = "  ecbo-cloak-pp-cli source nearby --latitude 35.6812 --longitude 139.7671 --agent"
					case "price", "validate":
						cmd.Example = `  ecbo-cloak-pp-cli source ` + cmd.Name() + ` --space-id 0c3fb1e9-ad5a-42de-bfda-3027ebe4921e --from "2026-10-03 19:00" --to "2026-10-03 21:00" --reservation-items-small 1 --reservation-items-large 1 --agent`
					}
				}
			}
		}
		// The generated raw endpoint dry-run mixes JSON and text; keep request previews structured.
		for _, parent := range root.Commands() {
			if parent.Name() == "source" {
				for _, leaf := range parent.Commands() {
					original := leaf.RunE
					if original != nil {
						leaf.RunE = func(cmd *cobra.Command, args []string) error {
							if flags.dryRun {
								return ecboSourcePreview(cmd, flags)
							}
							return original(cmd, args)
						}
					}
				}
			}
		}
		// Keep a focused read-only product; generic arbitrary API and unrelated write helpers are excluded.
		for _, cmd := range root.Commands() {
			switch cmd.Name() {
			case "api", "import", "feedback", "profile", "which":
				root.RemoveCommand(cmd)
			}
		}
	})
}
func ecboAnnot(happy, source string) map[string]string {
	return map[string]string{"mcp:read-only": "true", "pp:data-source": source, "pp:happy-args": happy}
}
func ecboError(e error) error {
	var x *ecbo.Error
	if errors.As(e, &x) {
		switch x.Code {
		case 2:
			return usageErr(e)
		case 3:
			return notFoundErr(e)
		case 4:
			return authErr(e)
		case 10:
			return configErr(e)
		default:
			return apiErr(e)
		}
	}
	var r *cliutil.RateLimitError
	if errors.As(e, &r) {
		return rateLimitErr(e)
	}
	return apiErr(e)
}
func ecboCache(s string) string {
	if s != "" {
		return s
	}
	if s = os.Getenv("ECBO_CLOAK_CACHE_DIR"); s != "" {
		return s
	}
	base, e := os.UserCacheDir()
	if e != nil {
		return filepath.Join(os.TempDir(), "ecbo-cloak-cli")
	}
	return filepath.Join(base, "ecbo-cloak-cli")
}
func ecboSetup(cmd *cobra.Command, flags *rootFlags, cache, locale string, refresh bool) (*ecbo.Client, context.Context, context.CancelFunc, error) {
	if locale != "ja" && locale != "en" && locale != "zh-TW" && locale != "zh-CN" {
		return nil, nil, nil, usageErr(fmt.Errorf("--locale must be ja, en, zh-TW or zh-CN"))
	}
	timeout := 15 * time.Second
	if flags.timeoutExplicit {
		timeout = flags.timeout
	}
	if timeout < time.Second || timeout > 60*time.Second {
		return nil, nil, nil, usageErr(fmt.Errorf("--timeout must be 1s..60s"))
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	return ecbo.New(ecboCommandCache(cache, flags), locale, timeout, refresh, flags.noCache), ctx, cancel, nil
}
func ecboPrint(cmd *cobra.Command, flags *rootFlags, c *ecbo.Client, v map[string]any) error {
	if e := ecboOutputMode(cmd, flags); e != nil {
		return e
	}
	// Projection applies to each result for lists, and to the payload for detail/offer.
	if flags.selectFields != "" {
		if list, ok := v["results"].([]any); ok {
			if e := ecbo.ValidateListProjection(flags.selectFields); e != nil {
				return ecboError(e)
			}
			projected := []any{}
			for _, item := range list {
				m, e := ecbo.Project(item.(map[string]any), flags.selectFields)
				if e != nil {
					return ecboError(e)
				}
				projected = append(projected, m)
			}
			v["results"] = projected
		} else {
			p, e := ecbo.Project(v, flags.selectFields)
			if e != nil {
				return ecboError(e)
			}
			v = p
		}
	}
	if c != nil {
		v["meta"] = c.Finish()
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// Focused commands preserve their JSON envelope and source evidence. Projection
// is explicit via --select; framework-only output modes cannot silently replace it.
func ecboOutputMode(cmd *cobra.Command, flags *rootFlags) error {
	for _, mode := range []struct {
		name string
		set  bool
	}{{"csv", flags.csv}, {"plain", flags.plain}, {"quiet", flags.quiet}, {"compact", flags.compact && (!flags.agent || cmd.Flags().Changed("compact"))}} {
		if mode.set {
			return usageErr(fmt.Errorf("--%s is unsupported for focused ecbo commands; use JSON with --select", mode.name))
		}
	}
	return nil
}

func ecboCommon(cmd *cobra.Command, flags *rootFlags, cache, locale *string, refresh *bool) {
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error { return ecboOutputMode(cmd, flags) }
	cmd.Flags().StringVar(cache, "cache-dir", "", "Response cache directory (env ECBO_CLOAK_CACHE_DIR)")
	cmd.Flags().StringVar(locale, "locale", "en", "First-party language: ja, en, zh-TW, zh-CN")
	cmd.Flags().BoolVar(refresh, "refresh", false, "Fetch a new source observation instead of cached discovery/detail")
}
func ecboNear(cmd *cobra.Command, o *ecbo.NearOptions) {
	cmd.Flags().Float64Var(&o.Lat, "lat", 0, "Japan latitude in decimal degrees (required)")
	cmd.Flags().Float64Var(&o.Lon, "lon", 0, "Japan longitude in decimal degrees (required)")
	cmd.Flags().Float64Var(&o.Radius, "radius-km", 5, "Local radius within source nearest window (max 100 km)")
	cmd.Flags().StringVar(&o.Query, "query", "", "Name substring within nearest window, Japanese or English")
	cmd.Flags().StringVar(&o.From, "from", "", "Deposit YYYY-MM-DDTHH:MM, Asia/Tokyo")
	cmd.Flags().StringVar(&o.To, "to", "", "Pickup YYYY-MM-DDTHH:MM, Asia/Tokyo")
	cmd.Flags().IntVar(&o.Small, "small", 0, "Number of bag-size pieces; requires interval")
	cmd.Flags().IntVar(&o.Large, "large", 0, "Number of suitcase-size pieces; requires interval")
	cmd.Flags().IntVar(&o.Limit, "limit", 10, "Maximum results, 1..50")
	cmd.Flags().IntVar(&o.Offset, "offset", 0, "Local offset within source's 50-hit window")
}
func newNovelFacilitiesNearCmd(flags *rootFlags) *cobra.Command {
	const inventory = false
	var o ecbo.NearOptions
	var cache, locale string
	var refresh bool
	short := "Find Japan luggage storage in a bounded nearby source window"
	if inventory {
		short = "Explicitly refresh one bounded local facility inventory window"
	}
	cmd := &cobra.Command{Use: "near", Short: short, Annotations: ecboAnnot("--lat=35.6812;--lon=139.7671;--limit=3", "live"), Example: "  ecbo-cloak-pp-cli facilities near --lat 35.6812 --lon 139.7671 --limit 5 --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "bounded ecbo discovery")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("unexpected positional argument"))
		}
		if e := o.Validate(); e != nil {
			return ecboError(e)
		}
		if e := ecbo.ValidateListProjection(flags.selectFields); e != nil {
			return ecboError(e)
		}
		c, ctx, cancel, e := ecboSetup(cmd, flags, cache, locale, refresh || inventory)
		if e != nil {
			return e
		}
		defer cancel()
		c.Meta.Coverage = "At most 50 nearby provider hits; local pagination/filtering; listed facility differs from available capacity"
		fetch := o
		if inventory {
			fetch.Limit = 50
			fetch.Offset = 0
		}
		v, e := c.Near(ctx, fetch)
		if e != nil {
			return ecboError(e)
		}
		if inventory {
			if flags.noCache {
				return usageErr(fmt.Errorf("inventory refresh requires cache; omit --no-cache"))
			}
			if e = c.SaveInventory(v); e != nil {
				return ecboError(e)
			}
			v = ecbo.WindowPage(v, o.Limit, o.Offset)
			v["inventory_saved"] = true
		}
		return ecboPrint(cmd, flags, c, v)
	}}
	ecboCommon(cmd, flags, &cache, &locale, &refresh)
	ecboNear(cmd, &o)
	return cmd
}
func newNovelInventoryRefreshCmd(flags *rootFlags) *cobra.Command {
	const inventory = true
	var o ecbo.NearOptions
	var cache, locale string
	var refresh bool
	short := "Find Japan luggage storage in a bounded nearby source window"
	if inventory {
		short = "Explicitly refresh one bounded local facility inventory window"
	}
	cmd := &cobra.Command{Use: "refresh", Short: short, Annotations: ecboAnnot("--lat=35.6812;--lon=139.7671;--limit=3;--home="+ecboFixtureHome(), "live"), Example: "  ecbo-cloak-pp-cli inventory refresh --lat 35.6812 --lon 139.7671 --limit 5 --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "bounded ecbo discovery")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("unexpected positional argument"))
		}
		if flags.noCache {
			return usageErr(fmt.Errorf("inventory refresh requires cache; omit --no-cache"))
		}
		if e := o.Validate(); e != nil {
			return ecboError(e)
		}
		if e := ecbo.ValidateListProjection(flags.selectFields); e != nil {
			return ecboError(e)
		}
		c, ctx, cancel, e := ecboSetup(cmd, flags, cache, locale, refresh || inventory)
		if e != nil {
			return e
		}
		defer cancel()
		c.Meta.Coverage = "At most 50 nearby provider hits; local pagination/filtering; listed facility differs from available capacity"
		fetch := o
		if inventory {
			fetch.Limit = 50
			fetch.Offset = 0
		}
		v, e := c.Near(ctx, fetch)
		if e != nil {
			return ecboError(e)
		}
		if inventory {
			if flags.noCache {
				return usageErr(fmt.Errorf("inventory refresh requires cache; omit --no-cache"))
			}
			if e = c.SaveInventory(v); e != nil {
				return ecboError(e)
			}
			v = ecbo.WindowPage(v, o.Limit, o.Offset)
			v["inventory_saved"] = true
		}
		return ecboPrint(cmd, flags, c, v)
	}}
	cmd.Annotations["mcp:read-only"] = "false"
	cmd.Annotations["mcp:local-write"] = "true"
	cmd.Annotations["pp:destructive-auth"] = "false"
	ecboCommon(cmd, flags, &cache, &locale, &refresh)
	ecboNear(cmd, &o)
	return cmd
}
func newNovelFacilitiesGetCmd(flags *rootFlags) *cobra.Command {
	var id, cache, locale string
	var refresh bool
	cmd := &cobra.Command{Use: "get [id-or-url]", Short: "Fetch identity, hours, daily prices, overnight rules and restrictions lazily", Annotations: ecboAnnot("id=0c3fb1e9-ad5a-42de-bfda-3027ebe4921e", "live"), Example: "  ecbo-cloak-pp-cli facilities get GBy4uBrI --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "read ecbo facility")
		}
		if len(args) > 1 || len(args) == 1 && id != "" {
			return usageErr(fmt.Errorf("use one positional ID or --id"))
		}
		if len(args) == 1 {
			id = args[0]
		}
		if _, e := ecbo.ID(id); e != nil {
			return ecboError(e)
		}
		c, ctx, cancel, e := ecboSetup(cmd, flags, cache, locale, refresh)
		if e != nil {
			return e
		}
		defer cancel()
		c.Meta.Coverage = "Public facility listing; capacity ratios and maximum counts are not remaining inventory"
		v, e := c.Detail(ctx, id)
		if e != nil {
			return ecboError(e)
		}
		return ecboPrint(cmd, flags, c, v)
	}}
	cmd.Flags().StringVar(&id, "id", "", "Facility UUID, legacy source ID or first-party canonical URL")
	ecboCommon(cmd, flags, &cache, &locale, &refresh)
	return cmd
}
func newNovelOfferInspectCmd(flags *rootFlags) *cobra.Command {
	var id, from, to, cache, locale string
	var small, large int
	var refresh bool
	cmd := &cobra.Command{Use: "inspect [id-or-url]", Short: "Inspect a live date/time/bag-specific quote and source validation, without booking", Annotations: ecboAnnot("id=0c3fb1e9-ad5a-42de-bfda-3027ebe4921e;--from=2026-10-03T19:00;--to=2026-10-03T21:00;--small=1;--large=1", "live"), Example: "  ecbo-cloak-pp-cli offer inspect 0c3fb1e9-ad5a-42de-bfda-3027ebe4921e --from 2026-10-03T19:00 --to 2026-10-03T21:00 --small 1 --large 1 --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "read-only ecbo price and validation")
		}
		if len(args) > 1 || len(args) == 1 && id != "" {
			return usageErr(fmt.Errorf("use one positional ID or --id"))
		}
		if len(args) == 1 {
			id = args[0]
		}
		if _, e := ecbo.ID(id); e != nil {
			return ecboError(e)
		}
		if _, _, e := ecbo.ParseTimes(from, to); e != nil {
			return ecboError(e)
		}
		if e := ecbo.ValidateCounts(small, large); e != nil {
			return ecboError(e)
		}
		c, ctx, cancel, e := ecboSetup(cmd, flags, cache, locale, refresh)
		if e != nil {
			return e
		}
		defer cancel()
		c.Meta.Coverage = "Source quote and validation observed for exact interval and bag counts; remaining capacity count undisclosed; no reservation created"
		v, e := c.Offer(ctx, id, from, to, small, large)
		if e != nil {
			return ecboError(e)
		}
		return ecboPrint(cmd, flags, c, v)
	}}
	cmd.Flags().StringVar(&id, "id", "", "Facility UUID, legacy source ID or canonical URL")
	cmd.Flags().StringVar(&from, "from", "", "Deposit YYYY-MM-DDTHH:MM, Asia/Tokyo (required)")
	cmd.Flags().StringVar(&to, "to", "", "Pickup YYYY-MM-DDTHH:MM, Asia/Tokyo (required)")
	cmd.Flags().IntVar(&small, "small", 0, "Number of bag-size pieces, total 1..50")
	cmd.Flags().IntVar(&large, "large", 0, "Number of suitcase-size pieces, total 1..50")
	ecboCommon(cmd, flags, &cache, &locale, &refresh)
	return cmd
}
func newNovelInventoryListCmd(flags *rootFlags) *cobra.Command {
	var cache, query string
	var limit, offset int
	cmd := &cobra.Command{Use: "list", Short: "Search the last explicitly refreshed inventory window offline", Annotations: ecboAnnot("--limit=3", "local"), Example: "  ecbo-cloak-pp-cli inventory list --query 東京 --limit 5 --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "read local inventory")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("unexpected positional argument"))
		}
		c := ecbo.New(ecboCommandCache(cache, flags), "en", 15*time.Second, false, false)
		c.Meta.Coverage = "Last bounded inventory snapshot, explicit refresh only"
		v, e := c.Inventory(query, limit, offset)
		if e != nil {
			return ecboError(e)
		}
		return ecboPrint(cmd, flags, c, v)
	}}
	cmd.Flags().StringVar(&cache, "cache-dir", "", "Inventory/cache directory (env ECBO_CLOAK_CACHE_DIR)")
	cmd.Flags().StringVar(&query, "query", "", "Japanese/English name substring in saved window")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum results, 1..50")
	cmd.Flags().IntVar(&offset, "offset", 0, "Local offset, 0..50")
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error { return ecboOutputMode(cmd, flags) }
	return cmd
}

// EcboMCPRootCmd keeps CLI cache configuration under the server operator's control.
func EcboMCPRootCmd() *cobra.Command {
	root := RootCmd()
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if flag := cmd.Flags().Lookup("cache-dir"); flag != nil {
			flag.Hidden = true
		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
	return root
}

// Explicit CLI --home keeps focused cache writes inside the same selected task root.
func ecboCommandCache(cache string, flags *rootFlags) string {
	base := strings.TrimSpace(flags.homePath)
	if cache == "" && base != "" {
		if base == "~" || strings.HasPrefix(base, "~/") {
			if home, e := os.UserHomeDir(); e == nil {
				if base == "~" {
					base = home
				} else {
					base = filepath.Join(home, strings.TrimPrefix(base, "~/"))
				}
			}
		}
		return filepath.Join(filepath.Clean(base), "cache", "ecbo-cloak-cli")
	}
	return ecboCache(cache)
}

// Process-only fixture override supports older Press runners that leave relative homes unexpanded.
func ecboFixtureHome() string {
	if path := strings.TrimSpace(os.Getenv("ECBO_CLOAK_DOGFOOD_HOME")); filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return ".printing-press-fixtures/inventory"
}

// Raw endpoint previews describe requests, never synthetic provider results.
func ecboSourcePreview(cmd *cobra.Command, flags *rootFlags) error {
	method := cmd.Annotations["pp:method"]
	path := cmd.Annotations["pp:path"]
	base := "https://api.ecbo.io"
	var params any
	var body any
	switch cmd.Name() {
	case "detail":
		id, _ := cmd.Flags().GetString("id")
		path = strings.ReplaceAll(path, "{id}", id)
	case "nearby":
		base = "https://search.ecbo.io"
		lat, _ := cmd.Flags().GetFloat64("latitude")
		lon, _ := cmd.Flags().GetFloat64("longitude")
		params = map[string]float64{"latitude": lat, "longitude": lon}
	case "price", "validate":
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		id, _ := cmd.Flags().GetString("space-id")
		small, _ := cmd.Flags().GetInt("reservation-items-small")
		large, _ := cmd.Flags().GetInt("reservation-items-large")
		body = map[string]any{"space_id": id, "from": from, "to": to, "reservation_items": map[string]int{"small": small, "large": large}}
		stdin, _ := cmd.Flags().GetBool("stdin")
		if stdin {
			input, e := ecboReadSourceBody(cmd)
			if e != nil {
				return e
			}
			body = input
		}
	default:
		return usageErr(fmt.Errorf("unsupported source preview"))
	}
	if flags.quiet {
		return nil
	}
	target := path
	if !strings.HasPrefix(target, "https://") {
		target = base + path
	}
	request := map[string]any{"planned": true, "operation": cmd.Annotations["pp:endpoint"], "method": method, "url": target, "params": params, "body": body}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"dry_run": true, "action": cmd.CommandPath(), "kind": "request_preview", "items": []any{request}})
}

// Share the same bound for live raw POST requests and dry-run previews.
func ecboReadSourceBody(cmd *cobra.Command) (map[string]any, error) {
	b, e := io.ReadAll(io.LimitReader(cmd.InOrStdin(), (2<<20)+1))
	if e != nil {
		return nil, usageErr(fmt.Errorf("reading stdin: %w", e))
	}
	if len(b) > 2<<20 {
		return nil, usageErr(fmt.Errorf("stdin request body exceeds 2 MiB"))
	}
	var input map[string]any
	if json.Unmarshal(b, &input) != nil || input == nil {
		return nil, usageErr(fmt.Errorf("stdin must be a JSON object"))
	}
	return input, nil
}
