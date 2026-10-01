package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/asoview/internal/asoview"
	"github.com/mvanhorn/printing-press-library/library/travel/asoview/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		root.AddCommand(newProductCmd(f), newHandoffCmd(f))
		root.PersistentFlags().BoolVar(&asoviewRefresh, "refresh", false, "Refresh only responses needed for this command")
	})
	registerClientHook(func(c *client.Client) error {
		if c.BaseURL != asoview.Origin {
			return usageErr(fmt.Errorf("Asoview source origin is fixed to %s", asoview.Origin))
		}
		base := c.HTTPClient.Transport
		if base == nil {
			base = http.DefaultTransport
		}
		c.HTTPClient.Transport = asoview.ReadOnlyTransport{Base: base}
		c.HTTPClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("redirect limit exceeded")
			}
			if req.URL.Scheme != "https" || req.URL.Host != "www.asoview.com" {
				return fmt.Errorf("redirect outside Asoview source")
			}
			return nil
		}
		return nil
	})
}

var asoviewRefresh bool

func sourceAnnotations(happy string) map[string]string {
	return map[string]string{"mcp:read-only": "true", "mcp:idempotent": "true", "pp:data-source": "auto", "pp:happy-args": strings.TrimSuffix(happy, ";--dry-run")}
}
func sourceClient(cmd *cobra.Command, f *rootFlags) (*asoview.Client, context.Context, context.CancelFunc, error) {
	if f.timeout <= 0 || f.timeout > 60*time.Second {
		return nil, nil, nil, usageErr(fmt.Errorf("--timeout must be greater than 0 and at most 60s"))
	}
	dir := os.Getenv("ASOVIEW_CACHE_DIR")
	if dir == "" {
		if f.homePath != "" {
			dir = filepath.Join(f.homePath, "cache", "asoview-public-v1")
		} else {
			base, err := os.UserCacheDir()
			if err != nil {
				return nil, nil, nil, configErr(err)
			}
			dir = filepath.Join(base, "asoview-pp-cli", "public-v1")
		}
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), f.timeout)
	return asoview.NewClient(dir, f.timeout, f.noCache || f.dataSource == "live", asoviewRefresh, f.dataSource == "local"), ctx, cancel, nil
}
func sourceFailure(err error) error {
	var e *asoview.Error
	if errors.As(err, &e) {
		return &cliError{code: e.Code, err: err}
	}
	return apiErr(err)
}
func sourceOutput(cmd *cobra.Command, f *rootFlags, c *asoview.Client, result any) error {
	payload := asoview.Object{"meta": asoview.Object{"schema_version": 1, "provider": "asoview", "language": "ja", "timezone": "Asia/Tokyo", "source": "live", "sources": c.Sources, "metrics": c.Metrics(), "missing_values": "null means not supplied or not verified", "purchase_or_booking_performed": false}, "results": result}
	if o, ok := result.(map[string]any); ok {
		if rows, exists := o["results"]; exists {
			payload["results"] = rows
			for k, v := range o {
				if k != "results" {
					payload[k] = v
				}
			}
		}
	}
	if len(c.Sources) == 0 {
		payload["meta"].(asoview.Object)["source"] = "bundled_inventory"
	} else {
		allCached := true
		for _, s := range c.Sources {
			if !s.Cached {
				allCached = false
				break
			}
		}
		if allCached {
			payload["meta"].(asoview.Object)["source"] = "cache"
		}
	}
	if o, ok := result.(map[string]any); ok {
		if inv, ok := o["inventory"].(map[string]any); ok {
			switch inv["basis"] {
			case "bundled_first_party_inventory":
				payload["meta"].(asoview.Object)["source"] = "bundled_inventory"
			case "locally_refreshed_first_party_inventory":
				payload["meta"].(asoview.Object)["source"] = "cache"
			}
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	// Domain views are already compact. Preserve price/validity semantics under --agent.
	if f.selectFields != "" {
		raw, err = filterFieldsChecked(raw, f.selectFields)
		if err != nil {
			return usageErr(err)
		}
	}
	if f.csv || f.plain || f.quiet {
		copyFlags := *f
		copyFlags.compact = false
		copyFlags.agent = false
		copyFlags.selectFields = ""
		return printOutputWithFlags(cmd.OutOrStdout(), raw, &copyFlags)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(raw))
	return err
}
func oneSourceID(args []string) (string, error) {
	if len(args) != 1 {
		return "", usageErr(fmt.Errorf("one product source ID is required; example ticket0000049223"))
	}
	if _, _, err := asoview.ParseID(args[0]); err != nil {
		return "", sourceFailure(err)
	}
	return args[0], nil
}
func newProductCmd(f *rootFlags) *cobra.Command {
	var full bool
	cmd := &cobra.Command{Use: "product <id>", Short: "Read ticket terms, validity, ages and advertised bands", Example: "  asoview-pp-cli product ticket0000049223 --agent\n  asoview-pp-cli product pln3000044589 --full --agent", Annotations: sourceAnnotations("id=ticket0000049223;--dry-run"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "product")
		}
		id, err := oneSourceID(args)
		if err != nil {
			return err
		}
		c, ctx, cancel, err := sourceClient(cmd, f)
		if err != nil {
			return err
		}
		defer cancel()
		p, err := c.Product(ctx, id, full)
		if err != nil {
			return sourceFailure(err)
		}
		return sourceOutput(cmd, f, c, p)
	}}
	cmd.Flags().BoolVar(&full, "full", false, "Include descriptions, user manual and extra fee prose")
	return cmd
}
func newHandoffCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "handoff <id>", Short: "Return the canonical product URL for human booking", Example: "  asoview-pp-cli handoff ticket0000049223 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:happy-args": "id=ticket0000049223"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "handoff")
		}
		id, err := oneSourceID(args)
		if err != nil {
			return err
		}
		kind, code, _ := asoview.ParseID(id)
		// URL derivation is explicit; this command does not claim current product existence.
		raw, err := json.Marshal(asoview.Object{"id": code, "kind": kind, "booking_url": asoview.BookingURL(code), "existence_verified": false, "booking_performed": false})
		if err != nil {
			return err
		}
		copyFlags := *f
		copyFlags.compact = false
		copyFlags.asJSON = true
		return printOutputWithFlags(cmd.OutOrStdout(), raw, &copyFlags)
	}}
	return cmd
}
func joinedIDs(args []string) ([]string, error) {
	if len(args) < 2 || len(args) > 5 {
		return nil, usageErr(fmt.Errorf("compare requires 2..5 product IDs"))
	}
	seen := map[string]bool{}
	for _, id := range args {
		_, code, err := asoview.ParseID(id)
		if err != nil {
			return nil, sourceFailure(err)
		}
		if seen[code] {
			return nil, usageErr(fmt.Errorf("duplicate comparison ID %s", code))
		}
		seen[code] = true
	}
	return args, nil
}
func validateDateMode(date, month string) error {
	if date != "" {
		if _, err := asoview.ParseDate(date); err != nil {
			return sourceFailure(err)
		}
	}
	if month != "" {
		if _, err := asoview.ParseMonth(month); err != nil {
			return sourceFailure(err)
		}
	}
	if strings.TrimSpace(date) == "" && strings.TrimSpace(month) == "" {
		return usageErr(fmt.Errorf("--date YYYY-MM-DD or --month YYYY-MM is required"))
	}
	return nil
}
