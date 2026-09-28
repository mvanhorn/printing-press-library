// Public Travel command infrastructure. Source-specific parsing belongs to internal/travel.
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/travel"
	"github.com/spf13/cobra"
)

// The factory injects a standard HTTP transport in deterministic CLI tests.
// Production always uses the credential-free, paced domain client.
var publicTravelClientFactory = func(cfg travel.Config) (travel.API, error) {
	return travel.NewClient(cfg)
}

type publicTravelOptions struct {
	refresh               bool
	inventoryCacheSeconds int
	maxRequests           int
	pretty                bool
}

func (o *publicTravelOptions) bind(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&o.refresh, "refresh", false, "Bypass cache reads and refresh metadata cache entries")
	cmd.Flags().IntVar(&o.inventoryCacheSeconds, "inventory-cache-seconds", 0, "Opt in to inventory caching for 0–60 seconds; 0 reads live inventory")
	cmd.Flags().IntVar(&o.maxRequests, "max-requests", 12, "Maximum outbound attempts, including retries, for this invocation (1–60)")
	cmd.Flags().BoolVar(&o.pretty, "pretty", false, "Indent JSON output for human reading")
}

func (o publicTravelOptions) validate(cmd *cobra.Command, flags *rootFlags, args []string) error {
	if len(args) != 0 {
		return usageErr(fmt.Errorf("%s accepts flags only; unexpected argument %q", cmd.CommandPath(), args[0]))
	}
	if flags.dataSource == "local" {
		return usageErr(fmt.Errorf("--data-source local is unsupported: public Travel has no offline inventory; use auto or live"))
	}
	if flags.timeout <= 0 {
		return usageErr(fmt.Errorf("--timeout must be positive"))
	}
	if o.inventoryCacheSeconds < 0 || o.inventoryCacheSeconds > 60 {
		return usageErr(fmt.Errorf("--inventory-cache-seconds must be between 0 and 60"))
	}
	if o.maxRequests < 1 || o.maxRequests > 60 {
		return usageErr(fmt.Errorf("--max-requests must be between 1 and 60"))
	}
	if flags.csv || flags.plain || flags.quiet {
		return usageErr(fmt.Errorf("public Travel commands emit a JSON envelope; --csv, --plain and --quiet are unsupported; use --select to narrow results"))
	}
	for _, name := range []string{"rate-limit", "max-age", "config", "client-profile"} {
		if cmd.Flags().Changed(name) {
			return usageErr(fmt.Errorf("--%s is unsupported for anonymous public Travel; request pacing and cache freshness are bounded by this backend", name))
		}
	}
	return nil
}

// The focused Travel leaves do not initialize a receipt writer. Reject these
// inherited flags before RunE, including --dry-run and profile-applied values,
// so a successful invocation never silently promises a missing receipt.
func rejectPublicTravelReceiptOptions(cmd *cobra.Command, flags *rootFlags) error {
	for _, option := range []struct {
		name   string
		active bool
	}{
		{"receipt", flags.receiptEnabled},
		{"receipt-file", flags.receiptFile != ""},
		{"audit-dir", flags.auditDir != ""},
	} {
		if cmd.Flags().Changed(option.name) || cmd.InheritedFlags().Changed(option.name) || option.active {
			return usageErr(fmt.Errorf("--%s is unsupported for focused Travel commands; omit this option", option.name))
		}
	}
	return nil
}

func (o publicTravelOptions) client(cmd *cobra.Command, flags *rootFlags) (travel.API, error) {
	cacheDir, err := cliutil.CacheDir()
	if err != nil {
		return nil, configErr(err)
	}
	api, err := publicTravelClientFactory(travel.Config{
		CacheDir: filepath.Join(cacheDir, "public-travel"),
		Refresh:  o.refresh, NoCache: flags.noCache,
		InventoryTTL: time.Duration(o.inventoryCacheSeconds) * time.Second,
		Timeout:      flags.timeout, MaxRequests: o.maxRequests,
		OnCacheWriteError: func(err error) {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: cache write failed: %v\n", err)
		},
	})
	if err != nil {
		return nil, usageErr(err)
	}
	return api, nil
}

func publicTravelDryRun(cmd *cobra.Command, flags *rootFlags) error {
	copyFlags := *flags
	copyFlags.asJSON = true
	return writeDryRun(cmd.OutOrStdout(), &copyFlags, strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()+" "))
}

func publicTravelGroup(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return usageErr(fmt.Errorf("unknown subcommand %q for %q; run '%s --help'", args[0], cmd.CommandPath(), cmd.CommandPath()))
	}
	return cmd.Help()
}

func requireTravelFlags(cmd *cobra.Command, names ...string) error {
	missing := make([]string, 0)
	for _, name := range names {
		if !cmd.Flags().Changed(name) {
			missing = append(missing, "--"+name)
		}
	}
	if len(missing) != 0 {
		return usageErr(fmt.Errorf("required flags: %s", strings.Join(missing, ", ")))
	}
	return nil
}

func validateTravelWindow(page, offset, limit int) error {
	if page < 1 || page > 100 || offset < 0 || offset > 10000 || limit < 1 || limit > 100 {
		return usageErr(fmt.Errorf("--page must be 1–100, --offset 0–10000 and --limit 1–100"))
	}
	return nil
}

func publicTravelError(err error) error {
	var sourceErr *travel.SourceError
	if errors.As(err, &sourceErr) {
		switch sourceErr.Kind {
		case "not_found":
			return notFoundErr(err)
		case "rate_limited", "throttled":
			return rateLimitErr(err)
		case "unsupported_query", "invalid_query":
			return usageErr(err)
		}
	}
	return apiErr(err)
}

func publicTravelErrorInfo(err error) map[string]string {
	kind := "fetch_error"
	var source *travel.SourceError
	if errors.As(err, &source) {
		kind = source.Kind
	}
	return map[string]string{"kind": kind, "message": err.Error()}
}

func publicTravelMeta(status string, source travel.SourceInfo, page *travel.PageInfo, query any, api travel.API) map[string]any {
	transport := "live"
	if strings.Contains(source.CacheState, "hit") {
		transport = "local"
	}
	meta := map[string]any{
		"source": transport, "data_origin": "live", "status": status, "source_info": source,
		"requests": api.Stats(),
	}
	if page != nil {
		meta["page"] = page
	}
	if query != nil {
		meta["query"] = query
	}
	return meta
}

// Travel has an explicit compact search projection. Bypass the generic field
// allowlist so price units, occupancy, continuation and nullable policy survive
// --agent/--compact. Full-envelope --select also supports dotted paths through
// results arrays and explicit metadata selection.
func (o publicTravelOptions) output(cmd *cobra.Command, flags *rootFlags, meta map[string]any, results any) error {
	raw, err := json.Marshal(map[string]any{"meta": meta, "results": results})
	if err != nil {
		return err
	}
	if flags.selectFields != "" {
		raw, err = filterFieldsChecked(raw, flags.selectFields)
		if err != nil {
			return usageErr(err)
		}
	}
	copyFlags := *flags
	copyFlags.asJSON, copyFlags.compact, copyFlags.selectFields, copyFlags.agent = true, false, "", false
	var buf bytes.Buffer
	if err := printJSONFiltered(&buf, json.RawMessage(raw), &copyFlags); err != nil {
		return err
	}
	if o.pretty {
		_, err = cmd.OutOrStdout().Write(buf.Bytes())
		return err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, buf.Bytes()); err != nil {
		return err
	}
	compact.WriteByte('\n')
	_, err = cmd.OutOrStdout().Write(compact.Bytes())
	return err
}

func init() {
	registerNovelCommand(func(rootCmd *cobra.Command, flags *rootFlags) {
		rootCmd.AddCommand(newPublicAreasCmd(flags))
		rootCmd.AddCommand(newPublicHotelsCmd(flags))
		rootCmd.Long = "Inspect real Japan room offers from anonymous public Rakuten Travel pages.\n\nBrowse areas or search hotels, then inspect offers for explicit Japan dates and\na uniform party in every room. Quotes retain their whole-stay per-room units.\nCompare at most nine explicit hotel/date cells; coverage stays bounded.\n\nOutput is compact JSON by default. Add --pretty for indentation, or --select\nresults.hotel_id,results.price to narrow results. --agent is supported."
		rootCmd.Example = "  rakuten-travel-pp-cli hotels search --query 品川 --limit 5\n  rakuten-travel-pp-cli offers search --hotel 51870 --checkin 2026-11-08 --checkout 2026-11-10 --rooms 1 --adults-per-room 2"
		rootCmd.PersistentFlags().Lookup("compact").Usage = "Compact output; Travel retains quote units, query and identity"
		rootCmd.PersistentFlags().Lookup("data-source").Usage = "Data source: auto or live for public Travel; local is unsupported by these commands"
		rootCmd.PersistentFlags().Lookup("receipt").Usage = "Write an atomic private run receipt (unsupported on focused Travel commands)"
		rootCmd.PersistentFlags().Lookup("receipt-file").Usage = "Override the run receipt destination (unsupported on focused Travel commands)"
		rootCmd.PersistentFlags().Lookup("audit-dir").Usage = "Aggregate the receipt and index under this audit directory (unsupported on focused Travel commands)"
		for _, path := range [][]string{
			{"areas", "list"}, {"hotels", "search"}, {"hotels", "show"},
			{"offers", "search"}, {"offers", "show"}, {"compare"},
		} {
			leaf, remaining, err := rootCmd.Find(path)
			if err != nil || len(remaining) != 0 {
				continue
			}
			previous := leaf.PreRunE
			leaf.PreRunE = func(cmd *cobra.Command, args []string) error {
				if err := rejectPublicTravelReceiptOptions(cmd, flags); err != nil {
					return err
				}
				if previous != nil {
					return previous(cmd, args)
				}
				return nil
			}
		}
		// Keep generated source routes reachable for discovery/verification while
		// normal help directs travelers to the structured command families.
		for _, cmd := range rootCmd.Commands() {
			switch cmd.Name() {
			case "hotel", "yado", "hotelinfo", "keyword":
				cmd.Hidden = true
			}
		}
	})
}
