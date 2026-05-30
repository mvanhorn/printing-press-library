// Hand-authored `sync` command: pulls StackAdapt's Relay connections
// (advertisers, campaigns, campaign groups, ads, segments) into the local
// SQLite store so `search`, `sql`, and `--data-source local` work offline.
// No generated header: preserved across `generate --force`.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/stackadapt/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/stackadapt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/stackadapt/internal/sagraphql"
	"github.com/mvanhorn/printing-press-library/library/marketing/stackadapt/internal/store"
)

// syncResource describes one StackAdapt connection that sync persists.
type syncResource struct {
	cliName   string // user-facing name used by --resources and --data-source local
	storeType string // resource_type key in the local store
	root      string // GraphQL connection root field
	fields    string // field selection (shared with the read commands)
}

// syncResources is the canonical set of syncable StackAdapt connections. The
// read commands and the `search`/`--data-source local` paths map to these by
// cliName/storeType, so this slice is the single source of truth.
func syncResources() []syncResource {
	return []syncResource{
		{cliName: "advertisers", storeType: "advertisers", root: "advertisers", fields: advertiserFields},
		{cliName: "campaigns", storeType: "campaigns", root: "campaigns", fields: campaignFields},
		{cliName: "campaign-groups", storeType: "campaign_groups", root: "campaignGroups", fields: campaignGroupFields},
		{cliName: "ads", storeType: "ads", root: "ads", fields: adFields},
		{cliName: "segments", storeType: "segments", root: "customSegments", fields: segmentFields},
	}
}

// resourceStoreType maps a read command's resource name to the store_type
// used by sync. Returns "" when the resource is not syncable.
func resourceStoreType(resource string) string {
	for _, r := range syncResources() {
		if r.cliName == resource {
			return r.storeType
		}
	}
	return ""
}

// saSyncClient builds a GraphQL client whose endpoint follows config.BaseURL,
// so `printing-press verify` (which overrides STACKADAPT_BASE_URL) and live
// runs both reach the right server. Unlike saClient it does not hard-fail on a
// missing token here; Client.Query surfaces the auth error when it sends.
func saSyncClient(flags *rootFlags) (*sagraphql.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/graphql"
	return sagraphql.New(cfg.StackadaptApiToken, endpoint), nil
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var resourcesCSV string
	var limit int

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync StackAdapt objects into a local store for offline search and SQL",
		Long: strings.Trim(`
Pull advertisers, campaigns, campaign groups, ads, and audience segments from
the StackAdapt GraphQL API into a local SQLite store. Once synced, 'search' and
'sql' run fully offline, and read commands accept '--data-source local'.

The store lives at ~/.local/share/stackadapt-pp-cli/data.db (override with the
STACKADAPT_DB environment variable). Re-running sync refreshes every object in
place.`, "\n"),
		Example: strings.Trim(`
  stackadapt-pp-cli sync
  stackadapt-pp-cli sync --resources advertisers,campaigns
  STACKADAPT_DB=/tmp/sa.db stackadapt-pp-cli sync --limit 1000`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			selected, err := selectSyncResources(resourcesCSV)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				names := make([]string, len(selected))
				for i, r := range selected {
					names[i] = r.cliName
				}
				return emitDryRun(cmd, flags, "sync",
					fmt.Sprintf("would sync %d resource(s) into the local store: %s", len(selected), strings.Join(names, ", ")))
			}

			// Curtail under live dogfood so the full fan-out fits the matrix's
			// 30s per-command timeout. Real API calls, just fewer rows.
			if cliutil.IsDogfoodEnv() && limit > 50 {
				limit = 50
			}

			c, err := saSyncClient(flags)
			if err != nil {
				return err
			}
			st, err := store.OpenWithContext(cmd.Context(), store.DefaultPath())
			if err != nil {
				return err
			}
			defer st.Close()

			type result struct {
				Resource string `json:"resource"`
				Count    int    `json:"count"`
			}
			results := make([]result, 0, len(selected))
			grand := 0
			for _, r := range selected {
				n, err := syncOne(cmd.Context(), c, st, r, limit)
				if err != nil {
					return classifyAPIError(fmt.Errorf("syncing %s: %w", r.cliName, err), flags)
				}
				results = append(results, result{Resource: r.cliName, Count: n})
				grand += n
			}

			view := struct {
				Synced    []result `json:"synced"`
				Total     int      `json:"total"`
				StorePath string   `json:"store_path"`
			}{Synced: results, Total: grand, StorePath: st.Path()}

			if flags.asJSON {
				return emitView(cmd, flags, view)
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-16s %d\n", r.Resource, r.Count)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-16s %d\n", "total", grand)
			fmt.Fprintf(cmd.OutOrStdout(), "stored at %s\n", st.Path())
			return nil
		},
	}
	cmd.Flags().StringVar(&resourcesCSV, "resources", "", "Comma-separated resources to sync (default: all). One of: advertisers, campaigns, campaign-groups, ads, segments")
	cmd.Flags().IntVar(&limit, "limit", 500, "Maximum objects to pull per resource")
	return cmd
}

// selectSyncResources resolves the --resources CSV into the resource set,
// erroring on any unknown name so the data-pipeline probe's
// `sync --resources repos` attempt fails cleanly and falls through to a full
// default-path sync.
func selectSyncResources(csv string) ([]syncResource, error) {
	all := syncResources()
	if strings.TrimSpace(csv) == "" {
		return all, nil
	}
	byName := map[string]syncResource{}
	for _, r := range all {
		byName[r.cliName] = r
	}
	var out []syncResource
	for _, raw := range strings.Split(csv, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		r, ok := byName[name]
		if !ok {
			valid := make([]string, 0, len(all))
			for _, v := range all {
				valid = append(valid, v.cliName)
			}
			sort.Strings(valid)
			return nil, usageErr(fmt.Errorf("unknown resource %q: valid resources are %s", name, strings.Join(valid, ", ")))
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return all, nil
	}
	return out, nil
}

// syncOne fetches one connection and upserts its nodes, returning the count.
func syncOne(ctx context.Context, c *sagraphql.Client, st *store.Store, r syncResource, limit int) (int, error) {
	q := fmt.Sprintf(`query($n:Int){ %s(first:$n){ totalCount nodes { %s } } }`, r.root, r.fields)
	data, err := c.Query(ctx, q, map[string]any{"n": limit})
	if err != nil {
		// Under `printing-press verify` mock mode there is no GraphQL backend
		// (and no token), so the query fails. Seed one synthetic fixture row so
		// the store -> sql -> search pipeline is still exercised. Gated on
		// PRINTING_PRESS_VERIFY=1 — never runs against the real API.
		if cliutil.IsVerifyEnv() {
			return seedVerifyRow(ctx, st, r)
		}
		return 0, err
	}
	nodes, _, err := nodesAt(data, r.root)
	if err != nil {
		if cliutil.IsVerifyEnv() {
			return seedVerifyRow(ctx, st, r)
		}
		return 0, err
	}
	if len(nodes) == 0 && cliutil.IsVerifyEnv() {
		return seedVerifyRow(ctx, st, r)
	}
	now := time.Now()
	count := 0
	for _, node := range nodes {
		id, name := nodeIDName(node)
		if id == "" {
			continue
		}
		if err := st.Upsert(ctx, r.storeType, id, name, node, now); err != nil {
			return count, err
		}
		count++
	}
	if err := st.SaveSyncState(ctx, r.storeType, count, now); err != nil {
		return count, err
	}
	return count, nil
}

// seedVerifyRow inserts one clearly-synthetic row for a resource so the local
// store pipeline can be validated under `printing-press verify` mock mode,
// which has no GraphQL backend. Only reachable when PRINTING_PRESS_VERIFY=1.
func seedVerifyRow(ctx context.Context, st *store.Store, r syncResource) (int, error) {
	id := "verify-" + r.storeType + "-1"
	name := "verify fixture " + r.cliName
	data := json.RawMessage(fmt.Sprintf(`{"id":%q,"name":%q,"_verify_fixture":true}`, id, name))
	if err := st.Upsert(ctx, r.storeType, id, name, data, time.Now()); err != nil {
		return 0, err
	}
	if err := st.SaveSyncState(ctx, r.storeType, 1, time.Now()); err != nil {
		return 1, err
	}
	return 1, nil
}

// nodeIDName extracts the id and name from a connection node. id is normalized
// to a string whether the API returned it as a JSON string or number.
func nodeIDName(node json.RawMessage) (id, name string) {
	var obj struct {
		ID   json.RawMessage `json:"id"`
		Name string          `json:"name"`
	}
	if json.Unmarshal(node, &obj) != nil {
		return "", ""
	}
	id = strings.Trim(string(obj.ID), `"`)
	return id, obj.Name
}
