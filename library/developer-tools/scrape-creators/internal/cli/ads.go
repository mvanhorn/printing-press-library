package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

func newAdsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ads",
		Short: "Cross-network ad library queries (Facebook + Google + LinkedIn)",
	}
	cmd.AddCommand(newAdsSearchCmd(flags))
	cmd.AddCommand(newAdsMonitorCmd(flags))
	return cmd
}

type adRow struct {
	Network string `json:"network"`
	AdID    string `json:"ad_id"`
	Brand   string `json:"brand"`
	Title   string `json:"title,omitempty"`
	Snippet string `json:"snippet,omitempty"`
	URL     string `json:"url,omitempty"`
}

func newAdsSearchCmd(flags *rootFlags) *cobra.Command {
	var network string
	var limit int
	cmd := &cobra.Command{
		Use:   "search <brand>",
		Short: "Search Facebook + Google + LinkedIn ad libraries in one query",
		Long: `Fan-out search across:
  - /v1/facebook/adLibrary/search/companies
  - /v1/google/adLibrary/advertisers/search
  - /v1/linkedin/ads/search

Results are returned per-network with the ad ID and a snippet. Pass --network
to restrict to one source.`,
		Example:     "  scrape-creators-pp-cli ads search \"Liquid Death\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			brand := args[0]
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type call struct {
				network string
				path    string
				params  map[string]string
			}
			calls := []call{
				{network: "facebook", path: "/v1/facebook/adLibrary/search/companies", params: map[string]string{"query": brand}},
				{network: "google", path: "/v1/google/adLibrary/advertisers/search", params: map[string]string{"query": brand}},
				{network: "linkedin", path: "/v1/linkedin/ads/search", params: map[string]string{"query": brand}},
			}
			if network != "" && network != "all" {
				kept := calls[:0]
				for _, k := range calls {
					if k.network == network {
						kept = append(kept, k)
					}
				}
				calls = kept
			}

			var (
				wg   sync.WaitGroup
				mu   sync.Mutex
				rows []adRow
			)
			for _, k := range calls {
				wg.Add(1)
				go func(k call) {
					defer wg.Done()
					raw, err := c.Get(k.path, k.params)
					if err != nil {
						return
					}
					rs := flattenAds(raw, k.network, brand, limit)
					mu.Lock()
					rows = append(rows, rs...)
					mu.Unlock()
				}(k)
			}
			wg.Wait()
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "NETWORK\tAD_ID\tTITLE\tURL")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Network, truncate(r.AdID, 16), truncate(r.Title, 50), truncate(r.URL, 60))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&network, "network", "all", "Restrict to one network (all | facebook | google | linkedin)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows per network")
	return cmd
}

func newAdsMonitorCmd(flags *rootFlags) *cobra.Command {
	var network string
	cmd := &cobra.Command{
		Use:   "monitor <brand>",
		Short: "Snapshot brand ads to SQLite; on rerun, diff new vs disappeared",
		Long: `Fetches the current ad set for <brand> across Facebook, Google, and LinkedIn,
fingerprints each ad, and writes the snapshot to ad_snapshots. On the next
run, diffs the current set against the previous one — new ads, gone ads,
unchanged.`,
		Example:     "  scrape-creators-pp-cli ads monitor \"Liquid Death\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			brand := args[0]
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := novelOpenStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			type call struct {
				network string
				path    string
				params  map[string]string
			}
			calls := []call{
				{network: "facebook", path: "/v1/facebook/adLibrary/search/companies", params: map[string]string{"query": brand}},
				{network: "google", path: "/v1/google/adLibrary/advertisers/search", params: map[string]string{"query": brand}},
				{network: "linkedin", path: "/v1/linkedin/ads/search", params: map[string]string{"query": brand}},
			}
			if network != "" && network != "all" {
				kept := calls[:0]
				for _, k := range calls {
					if k.network == network {
						kept = append(kept, k)
					}
				}
				calls = kept
			}

			// 1) Read previous snapshot fingerprints.
			previousFP := map[string]bool{}
			rows, err := s.DB().QueryContext(cmd.Context(),
				`SELECT fingerprint FROM ad_snapshots WHERE brand = ? AND snapshot_at = (
					SELECT MAX(snapshot_at) FROM ad_snapshots WHERE brand = ?
				)`, brand, brand)
			if err == nil {
				for rows.Next() {
					var fp string
					if err := rows.Scan(&fp); err == nil {
						previousFP[fp] = true
					}
				}
				rows.Close()
			}

			// 2) Fetch current ads.
			var current []adRow
			for _, k := range calls {
				raw, err := c.Get(k.path, k.params)
				if err != nil {
					continue
				}
				current = append(current, flattenAds(raw, k.network, brand, 100)...)
			}

			// 3) Fingerprint + insert.
			now := time.Now().UTC().Format(time.RFC3339)
			currentFP := map[string]bool{}
			for _, r := range current {
				blob, _ := json.Marshal(r)
				h := sha256.Sum256(blob)
				fp := hex.EncodeToString(h[:8])
				currentFP[fp] = true
				_, _ = s.DB().ExecContext(cmd.Context(),
					`INSERT INTO ad_snapshots (brand, platform, ad_id, fingerprint, data, snapshot_at) VALUES (?, ?, ?, ?, ?, ?)`,
					brand, r.Network, r.AdID, fp, string(blob), now)
			}

			// 4) Diff.
			var newAds, goneAds []string
			for fp := range currentFP {
				if !previousFP[fp] {
					newAds = append(newAds, fp)
				}
			}
			for fp := range previousFP {
				if !currentFP[fp] {
					goneAds = append(goneAds, fp)
				}
			}

			result := map[string]any{
				"brand":          brand,
				"snapshot_at":    now,
				"current_count":  len(currentFP),
				"previous_count": len(previousFP),
				"new_ads":        len(newAds),
				"gone_ads":       len(goneAds),
				"sample_current": current,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s — %d ads now (was %d). %d new, %d gone.\n",
				brand, len(currentFP), len(previousFP), len(newAds), len(goneAds))
			return nil
		},
	}
	cmd.Flags().StringVar(&network, "network", "all", "Restrict to one network (all | facebook | google | linkedin)")
	return cmd
}

// flattenAds normalizes the per-network response into a uniform slice.
// The schemas differ wildly; we walk for any objects with an `id`-like field
// and collect title/snippet/url heuristically.
func flattenAds(raw json.RawMessage, network, brand string, limit int) []adRow {
	var anyVal any
	if err := json.Unmarshal(raw, &anyVal); err != nil {
		return nil
	}
	out := []adRow{}
	walkAds(anyVal, network, brand, &out, limit)
	if len(out) > limit {
		out = out[:limit]
	}
	if out == nil {
		return []adRow{}
	}
	return out
}

func walkAds(v any, network, brand string, out *[]adRow, limit int) {
	if len(*out) >= limit {
		return
	}
	switch x := v.(type) {
	case map[string]any:
		// Treat any object with an "id"-like field as a candidate ad row.
		id := lookupString(x, []string{"id", "ad_id", "adId", "advertiserId", "company_id", "page_id", "page_alias"})
		if id != "" {
			row := adRow{Network: network, AdID: id, Brand: brand}
			row.Title = lookupString(x, []string{"name", "title", "company_name", "advertiserName", "page_name"})
			row.URL = lookupString(x, []string{"url", "link", "page_url", "snapshot_url"})
			row.Snippet = lookupString(x, []string{"description", "body", "ad_creative_body"})
			*out = append(*out, row)
		}
		for _, vv := range x {
			walkAds(vv, network, brand, out, limit)
		}
	case []any:
		for _, vv := range x {
			walkAds(vv, network, brand, out, limit)
		}
	}
}
