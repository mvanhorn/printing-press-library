package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/pocket-concierge/internal/pocket"
	"github.com/spf13/cobra"
)

var version = "0.1.0"

type rootFlags struct {
	lang, cacheDir, selectFields                              string
	timeout                                                   time.Duration
	refresh, noCache, dryRun, agent, asJSON, compact, noInput bool
}

func RootCmd() *cobra.Command { return newRootCmd(&rootFlags{}) }
func Execute() error          { return RootCmd().Execute() }
func ExitCode(e error) int    { return pocket.ExitCode(e) }
func newRootCmd(f *rootFlags) *cobra.Command {
	root := &cobra.Command{Use: "pocket-concierge-pp-cli", Short: "Discover fine dining with sourced courses, session identity and safe booking handoff.", Long: "Public read-only Pocket Concierge discovery. Compact JSON is the default. No account, reservation or payment operations.\nUse filters for native source IDs, then restaurants search/get, courses list, availability dates/slots and booking handoff.", Version: version, SilenceUsage: true, SilenceErrors: true}
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		cacheRoot = os.TempDir()
	}
	root.PersistentFlags().StringVar(&f.lang, "lang", "en", "First-party content language: en or ja")
	root.PersistentFlags().StringVar(&f.cacheDir, "cache-dir", filepath.Join(cacheRoot, "pocket-concierge-pp-cli"), "Directory for bounded public response cache")
	root.PersistentFlags().DurationVar(&f.timeout, "timeout", 30*time.Second, "Whole-command deadline, from 1s to 120s")
	root.PersistentFlags().StringVar(&f.selectFields, "select", "", "Project dotted payload fields; freshness meta is retained")
	root.PersistentFlags().BoolVar(&f.refresh, "refresh", false, "Refresh queried inventory explicitly, replacing cached responses")
	root.PersistentFlags().BoolVar(&f.noCache, "no-cache", false, "Bypass all response cache reads and writes")
	root.PersistentFlags().BoolVar(&f.dryRun, "dry-run", false, "Describe fixed read requests without network or cache access")
	root.PersistentFlags().BoolVar(&f.agent, "agent", false, "Use compact JSON for non-interactive agents (default output)")
	root.PersistentFlags().BoolVar(&f.asJSON, "json", false, "Emit compact JSON (also the default)")
	root.PersistentFlags().BoolVar(&f.compact, "compact", false, "Use compact JSON (also the default; detail terms retained)")
	root.PersistentFlags().BoolVar(&f.noInput, "no-input", false, "Never request interactive input (all commands are non-interactive)")
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if f.lang != "en" && f.lang != "ja" {
			return pocket.Fail("usage", "--lang must be en or ja")
		}
		if f.timeout < time.Second || f.timeout > 120*time.Second {
			return pocket.Fail("usage", "--timeout must be between 1s and 120s")
		}
		return nil
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, e error) error { return pocket.Fail("usage", e.Error()) })
	root.AddCommand(newFiltersCmd(f), newRestaurantsCmd(f), newCoursesCmd(f), newAvailabilityCmd(f), newBookingCmd(f), newDoctorCmd(f), newContextCmd(f), newSchemaCmd(f), newVersionCmd(f))
	return root
}
func emit(cmd *cobra.Command, f *rootFlags, result any) error {
	v, e := project(result, f.selectFields)
	if e != nil {
		return e
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
func run(cmd *cobra.Command, f *rootFlags, args []string, request string, fn func(context.Context, *pocket.Client) (map[string]any, error)) error {
	if len(args) > 0 {
		return pocket.Fail("usage", "unexpected positional arguments; use named flags")
	}
	if f.dryRun {
		dryFlags := *f
		dryFlags.selectFields = ""
		return emit(cmd, &dryFlags, map[string]any{"action": cmd.CommandPath(), "would": request, "would_select": f.selectFields, "dry_run": true, "command": cmd.CommandPath(), "method": "POST", "url": pocket.Endpoint, "operation": request, "language": f.lang, "read_only": true, "max_attempts_per_request": 2, "command_timeout_seconds": f.timeout.Seconds()})
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), f.timeout)
	defer cancel()
	start := time.Now()
	c := pocket.New(f.cacheDir, f.refresh, f.noCache)
	result, err := fn(ctx, c)
	if err != nil {
		return err
	}
	meta := c.Meta(f.lang, time.Since(start))
	if m, ok := result["meta"].(map[string]any); ok {
		for k, v := range m {
			meta[k] = v
		}
	}
	result["meta"] = meta

	return emit(cmd, f, result)
}
func noArgs(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return pocket.Fail("usage", "unexpected positional arguments; use named flags")
	}
	return nil
}
func requireID(s string) error {
	if s == "" {
		return pocket.Fail("usage", "--id is required; find source IDs with restaurants search")
	}
	return pocket.ValidateID(s)
}
func parseIDs(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	out := []string{}
	seen := map[string]bool{}
	for _, x := range strings.Split(s, ",") {
		x = strings.TrimSpace(x)
		if err := pocket.ValidateID(x); err != nil {
			return nil, err
		}
		if !seen[x] {
			out = append(out, x)
			seen[x] = true
		}
	}
	if len(out) > 10 {
		return nil, pocket.Fail("usage", "at most 10 source IDs per filter")
	}
	return out, nil
}
func newVersionCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "version", Short: "Show CLI version and source provider", Example: "  pocket-concierge-pp-cli " + "version", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": ""}, Args: noArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return emit(cmd, f, map[string]any{"version": version, "provider": "Pocket Concierge"})
	}}
}
func newFiltersCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "filters", Short: "List first-party area and cuisine IDs with Japanese names", Example: "  pocket-concierge-pp-cli " + "filters --select areas.id,areas.name,areas.name_ja", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": ""}, Args: noArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd, f, args, "Filters + Japanese identity", func(ctx context.Context, c *pocket.Client) (map[string]any, error) { return c.Filters(ctx, f.lang) })
	}}
}
func newDoctorCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Check public read access without account credentials", Example: "  pocket-concierge-pp-cli " + "doctor", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": ""}, Args: noArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd, f, args, "Filters connectivity probe", func(ctx context.Context, c *pocket.Client) (map[string]any, error) {
			_, e := c.Filters(ctx, f.lang)
			if e != nil {
				return nil, e
			}
			return map[string]any{"status": "ok", "auth_required": false, "paid_key_required": false, "runtime": "Go HTTPS; no resident browser", "read_only": true}, nil
		})
	}}
}
func newContextCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "agent-context", Short: "Show task routing, safety, output and source semantics", Example: "  pocket-concierge-pp-cli " + "agent-context", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": ""}, Args: noArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return emit(cmd, f, map[string]any{"provider": "Pocket Concierge", "read_only": true, "schema_version": "4", "cli": map[string]any{"name": "pocket-concierge-pp-cli", "description": cmd.Root().Short, "version": version}, "auth": map[string]any{"mode": "none", "env_vars": []any{}}, "commands": collectAgentCommands(cmd.Root()), "workflow": "filters → restaurants search → restaurants get / courses list → availability dates / slots → booking handoff", "identity": "Join by restaurant/course/session source ID. name_ja is first-party Japanese content.", "availability": "Options are not confirmed reservations. Waitlists have no source session ID.", "prices": "JPY per guest plus optional fixed per group; all_in_total remains null. Review both course and restaurant fee statements.", "freshness": "Inspect meta.observations. Date/party search and sessions are live by default; --refresh explicitly refreshes detail/catalog cache.", "projection": "--select items.id,items.name,items.name_ja,items.url retains meta", "conditions": "Use only explicit reservation_terms, source_services and course text. Page language does not guarantee staff language."})
	}}
}
func newSchemaCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "schema", Short: "Describe compact output fields, missing values and exit codes", Example: "  pocket-concierge-pp-cli " + "schema", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": ""}, Args: noArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return emit(cmd, f, map[string]any{"schema_version": 1, "containers": []string{"items", "restaurant", "areas", "cuisines", "pagination", "meta"}, "identity": []string{"id", "restaurant_id", "course_id", "session_id", "name", "name_ja", "url"}, "price": map[string]any{"currency": "JPY", "per_guest": "integer or null", "fixed_per_group": "integer or null", "all_in_total": "null; never inferred"}, "session_statuses": []string{"instant_confirmation", "reservation_request", "waitlist", "null when unknown"}, "missing": "null means not explicitly known; empty arrays mean successful empty source collections", "exit_codes": map[string]int{"success": 0, "internal": 1, "usage": 2, "not_found": 3, "access_denied": 4, "rate_limited": 5, "network_timeout": 6, "source_contract": 7}, "source_ids": "Numeric strings, preserved without numeric conversion", "meta": "Always retained by field projection; observed timestamps, cache hits, request count, latency and explicit partial coverage"})
	}}
}
