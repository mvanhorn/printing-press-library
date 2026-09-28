// pp:data-source live
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/ikyu"
	"github.com/spf13/cobra"
)

var stayNow = time.Now
var stayReaderFactory = func(o ikyu.Options) (ikyu.Reader, error) { return ikyu.NewClient(o) }

type stayFlags struct {
	stay                           ikyu.Stay
	limit, offset, scanPages       int
	refresh, allowStale            bool
	cacheDir                       string
	minBudget, maxBudget           int64
	meals                          string
	outdoor, hotSpring, nonsmoking bool
	minSize                        float64
}

func bindStayFlags(cmd *cobra.Command, s *stayFlags, dates bool, filters bool, list bool) {
	if dates {
		cmd.Flags().StringVar(&s.stay.CheckIn, "check-in", "", "Check-in date in Japan, exactly YYYY-MM-DD")
		cmd.Flags().StringVar(&s.stay.CheckOut, "check-out", "", "Check-out date in Japan, after check-in")
		cmd.Flags().IntVar(&s.stay.Adults, "adults", 2, "Adults per room, identical in every room")
		cmd.Flags().IntVar(&s.stay.Rooms, "rooms", 1, "Identical rooms with the same per-room party (1–10)")
		labels := []string{"older primary-school children", "younger primary-school children", "infants with meals and bedding", "infants with meals, no bedding", "infants with bedding, no meals", "infants without meals or bedding"}
		for i, label := range labels {
			cmd.Flags().IntVar(&s.stay.Children[i], fmt.Sprintf("children-%c", 'a'+i), 0, "Per-room count of "+label+" (Ikyu source category)")
		}
	}
	if list {
		cmd.Flags().IntVar(&s.limit, "limit", 10, "Maximum returned records, between 1 and 50")
		cmd.Flags().IntVar(&s.offset, "offset", 0, "Source record offset; preserve pagination coverage")
	}
	if filters {
		cmd.Flags().Int64Var(&s.minBudget, "min-budget", 0, "Minimum source instant-points JPY booking-total quote")
		cmd.Flags().Int64Var(&s.maxBudget, "max-budget", 0, "Maximum source instant-points JPY booking-total quote")
		cmd.Flags().StringVar(&s.meals, "meals", "", "Comma-separated Ikyu meal codes 000–007")
		cmd.Flags().BoolVar(&s.outdoor, "outdoor-bath", false, "Require explicit room outdoor-bath evidence")
		cmd.Flags().BoolVar(&s.hotSpring, "hot-spring-bath", false, "Require explicit room hot-spring outdoor-bath evidence")
		cmd.Flags().BoolVar(&s.nonsmoking, "nonsmoking", false, "Require explicit room nonsmoking attribute")
		cmd.Flags().Float64Var(&s.minSize, "min-size", 0, "Minimum source room size in square metres")
	}
	cmd.Flags().BoolVar(&s.refresh, "refresh", false, "Bypass existing public cache and request current source data")
	cmd.Flags().BoolVar(&s.allowStale, "allow-stale", false, "Allow timestamped stale cache only after an upstream failure")
	cmd.Flags().StringVar(&s.cacheDir, "cache-dir", "", "Override bounded public read-through cache directory")
}
func (s *stayFlags) preferences(cmd *cobra.Command) ikyu.Preferences {
	p := ikyu.Preferences{OutdoorBath: s.outdoor, HotSpringBath: s.hotSpring, Nonsmoking: s.nonsmoking}
	if cmd.Flags().Changed("min-budget") {
		p.MinBudget = &s.minBudget
	}
	if cmd.Flags().Changed("max-budget") {
		p.MaxBudget = &s.maxBudget
	}
	if cmd.Flags().Changed("min-size") {
		p.MinSizeM2 = &s.minSize
	}
	if s.meals != "" {
		for _, m := range strings.Split(s.meals, ",") {
			p.Meals = append(p.Meals, strings.TrimSpace(m))
		}
	}
	return p
}
func addFieldsAlias(cmd *cobra.Command, f *rootFlags) {
	cmd.Flags().StringVar(&f.selectFields, "fields", "", "Project comma-separated JSON fields; alias of --select")
}
func stayClient(cmd *cobra.Command, f *rootFlags, s *stayFlags) (ikyu.Reader, error) {
	if f.dataSource != "" && f.dataSource != "auto" && f.dataSource != "live" {
		return nil, usageErr(fmt.Errorf("stay commands support --data-source auto|live; local-only cache reads are not supported"))
	}
	dir := s.cacheDir
	if dir == "" {
		dir = os.Getenv("IKYU_CACHE_DIR")
	}
	if dir == "" {
		base, e := os.UserCacheDir()
		if e != nil {
			return nil, configErr(e)
		}
		dir = filepath.Join(base, "ikyu-pp-cli", "public-v1")
	}
	if f.noCache {
		dir = ""
	}
	return stayReaderFactory(ikyu.Options{CacheDir: dir, Refresh: s.refresh || f.noCache || f.dataSource == "live", AllowStale: s.allowStale, Now: stayNow})
}
func stayContext(cmd *cobra.Command, f *rootFlags) (context.Context, context.CancelFunc) {
	ctx, cancel := boundCtx(cmd.Context(), f)
	bounded, stop := context.WithTimeout(ctx, 120*time.Second)
	return bounded, func() { stop(); cancel() }
}
func stayEmit(cmd *cobra.Command, f *rootFlags, v any, stats *ikyu.Stats) error {
	raw, e := json.Marshal(v)
	if e != nil {
		return e
	}
	var envelope map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if e = decoder.Decode(&envelope); e != nil {
		return e
	}
	if stats != nil {
		envelope["stats"] = stats
	}
	f.asJSON = true
	// The owned typed views are already bounded. Framework field compaction
	// decodes through float64, so keep exact source int64 yen in the raw path.
	originalCompact := f.compact
	f.compact = false
	defer func() { f.compact = originalCompact }()
	var rendered bytes.Buffer
	err := printJSONFiltered(&rendered, envelope, f)
	if f.csv || f.plain || f.quiet {
		_, writeErr := cmd.OutOrStdout().Write(rendered.Bytes())
		if writeErr != nil {
			return writeErr
		}
		return err
	}
	var compact bytes.Buffer
	if compactErr := json.Compact(&compact, rendered.Bytes()); compactErr != nil {
		return compactErr
	}
	compact.WriteByte('\n')
	_, writeErr := cmd.OutOrStdout().Write(compact.Bytes())
	if writeErr != nil {
		return writeErr
	}
	return err
}
func stayDryRun(cmd *cobra.Command, _ *rootFlags, request any) error {
	// Dry-run control metadata stays top-level, as in generated writeDryRun.
	// Agent wrapping and result projection describe observations, not a request plan.
	return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"dry_run": true, "action": cmd.CommandPath(), "read_only": true, "planned": request, "network_requests": 0, "cache_writes": 0})
}
func staySourceError(e error) error {
	var input *ikyu.InputError
	if errors.As(e, &input) {
		return usageErr(e)
	}
	var rate *cliutil.RateLimitError
	if errors.As(e, &rate) {
		return rateLimitErr(e)
	}
	if ikyu.IsNotFound(e) {
		return notFoundErr(e)
	}
	return apiErr(e)
}
func stayValidate(s ikyu.Stay) error {
	if e := ikyu.ValidateStay(s, stayNow()); e != nil {
		return usageErr(e)
	}
	return nil
}
func stayArg(args []string, want int, usage string) error {
	if len(args) != want {
		return usageErr(fmt.Errorf("%s", usage))
	}
	return nil
}
