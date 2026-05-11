package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/unify/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/unify/internal/store"

	"github.com/spf13/cobra"
)

// newSyncCmd refreshes the local SQLite mirror: objects → attributes →
// attribute_options → records (from the watchlist). The Unify Data API has
// no LIST endpoint for records, so records are refreshed by calling
// find-unique for each watchlist entry. Sync is idempotent; rerunning is
// safe.
func newSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var concurrency int
	var skipRecords bool
	var skipAttrs bool
	var watchlistOnly bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Mirror Unify schema and watched records into the local SQLite store",
		Long: `Refreshes the local SQLite store from the Unify Data API:
  1. Lists every object (company, person, opportunity, salesforce_*, ...)
  2. For each object, lists attributes and attribute options
  3. For each watchlist entry, calls find-unique and stores the record

Records have no LIST endpoint in the Unify Data API, so sync uses the watchlist
as its cursor. Add entries with 'unify-pp-cli watch add <object> --match k=v'.`,
		Example: strings.Trim(`
  unify-pp-cli sync                       # full schema + records refresh
  unify-pp-cli sync --skip-records        # schema-only (faster, no API call per record)
  unify-pp-cli sync --skip-attrs          # objects table only
  unify-pp-cli sync --concurrency 8       # tune find-unique parallelism
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if watchlistOnly {
				skipAttrs = true
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := store.Open(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()

			report := map[string]any{}

			// Schema: objects.
			objCount, attrCount, optCount, err := syncSchema(ctx, c, s, skipAttrs)
			if err != nil {
				return apiErr(err)
			}
			report["objects"] = objCount
			report["attributes"] = attrCount
			report["attribute_options"] = optCount

			// Records via watchlist.
			recCount, recErrs := 0, 0
			if !skipRecords {
				wl, err := s.ListWatch(ctx, "")
				if err != nil {
					return apiErr(err)
				}
				recCount, recErrs = syncWatchlistRecords(ctx, c, s, wl, concurrency)
			}
			report["records_refreshed"] = recCount
			report["records_errors"] = recErrs
			report["watchlist_size"], _ = countWatch(ctx, s)
			report["db_path"] = s.Path

			blob, _ := json.MarshalIndent(report, "", "  ")
			return printOutputWithFlags(cmd.OutOrStdout(), blob, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store (default ~/.cache/unify-pp-cli/store.db)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "Parallel find-unique calls during record refresh")
	cmd.Flags().BoolVar(&skipRecords, "skip-records", false, "Skip record refresh (schema-only)")
	cmd.Flags().BoolVar(&skipAttrs, "skip-attrs", false, "Skip attribute/option refresh (objects-only)")
	cmd.Flags().BoolVar(&watchlistOnly, "watchlist", false, "Only refresh watchlist records (skip attribute refresh). Sync always consumes the watchlist when present; this flag just narrows the scope.")
	return cmd
}

// syncSchema lists objects, then for each object lists attributes (and
// per-select-attribute options). Returns counts.
func syncSchema(ctx context.Context, c *client.Client, s *store.Store, skipAttrs bool) (int, int, int, error) {
	raw, err := c.Get("/data/v1/objects", nil)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("list objects: %w", err)
	}
	var env struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return 0, 0, 0, fmt.Errorf("parse objects: %w", err)
	}
	objCount := 0
	attrCount := 0
	optCount := 0
	for _, o := range env.Data {
		obj := store.Object{
			APIName:     stringOf(o["api_name"]),
			Provider:    stringOf(o["provider"]),
			Category:    stringOf(o["category"]),
			DisplayName: stringOf(o["display_name"]),
			Description: stringOf(o["description"]),
			Raw:         o,
		}
		if obj.APIName == "" {
			continue
		}
		if err := s.UpsertObject(ctx, obj); err != nil {
			return objCount, attrCount, optCount, fmt.Errorf("store object %s: %w", obj.APIName, err)
		}
		// Pre-create the per-object record table so commands like `coverage`
		// and `audit-scores` find an empty table instead of erroring out on
		// objects that haven't had any records synced yet.
		_ = s.EnsureRecordTable(ctx, obj.APIName)
		objCount++
		if skipAttrs {
			continue
		}
		// Attributes.
		path := "/data/v1/objects/" + obj.APIName + "/attributes"
		attrRaw, err := c.Get(path, nil)
		if err != nil {
			// One object's attrs failing shouldn't abort all of sync.
			continue
		}
		var attrEnv struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.Unmarshal(attrRaw, &attrEnv); err != nil {
			continue
		}
		for _, a := range attrEnv.Data {
			at := store.Attribute{
				ObjectName:  obj.APIName,
				APIName:     stringOf(a["api_name"]),
				Type:        stringOf(a["type"]),
				DisplayName: stringOf(a["display_name"]),
				Description: stringOf(a["description"]),
				IsUnique:    boolOf(a["is_unique"]),
				Raw:         a,
			}
			if at.APIName == "" {
				continue
			}
			if err := s.UpsertAttribute(ctx, at); err != nil {
				continue
			}
			attrCount++
			// Options for select/multi-select.
			t := strings.ToUpper(at.Type)
			if t == "SELECT" || t == "MULTI_SELECT" {
				optPath := path + "/" + at.APIName + "/options"
				optRaw, err := c.Get(optPath, nil)
				if err != nil {
					continue
				}
				var optEnv struct {
					Data []map[string]any `json:"data"`
				}
				if err := json.Unmarshal(optRaw, &optEnv); err == nil {
					for _, op := range optEnv.Data {
						_ = s.UpsertAttributeOption(ctx, store.AttributeOption{
							ObjectName:    obj.APIName,
							AttributeName: at.APIName,
							APIName:       stringOf(op["api_name"]),
							DisplayName:   stringOf(op["display_name"]),
							Raw:           op,
						})
						optCount++
					}
				}
			}
		}
	}
	return objCount, attrCount, optCount, nil
}

// syncWatchlistRecords runs parallel find-unique for each watchlist entry.
// Returns (records-refreshed, error-count). Per-record errors are logged
// but do not abort the run; this is a refresh, not a strict sync.
func syncWatchlistRecords(ctx context.Context, c *client.Client, s *store.Store, wl []store.WatchEntry, concurrency int) (int, int) {
	if len(wl) == 0 {
		return 0, 0
	}
	if concurrency < 1 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	ok, fail := 0, 0
	var wg sync.WaitGroup
	for _, e := range wl {
		e := e
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			rec, err := findUnique(c, e.ObjectName, map[string]any{e.MatchKey: e.MatchValue})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fail++
				return
			}
			if rec == nil {
				return
			}
			id := stringOf(rec["id"])
			created := stringOf(rec["created_at"])
			updated := stringOf(rec["updated_at"])
			attrs, _ := rec["attributes"].(map[string]any)
			if err := s.UpsertRecord(ctx, e.ObjectName, id, created, updated, attrs); err != nil {
				fail++
				return
			}
			ok++
		}()
	}
	wg.Wait()
	return ok, fail
}

// findUnique calls POST .../records/find-unique and returns the record's
// data object (id + attributes + timestamps). Returns nil for a clean
// not-found.
func findUnique(c *client.Client, objectName string, match map[string]any) (map[string]any, error) {
	body := map[string]any{"match": match}
	path := "/data/v1/objects/" + objectName + "/records/find-unique"
	raw, status, err := c.Post(path, body)
	if status == 404 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	if jerr := json.Unmarshal(raw, &env); jerr != nil {
		return nil, jerr
	}
	return env.Data, nil
}

func countWatch(ctx context.Context, s *store.Store) (int, error) {
	wl, err := s.ListWatch(ctx, "")
	return len(wl), err
}

func stringOf(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%v", t)
	case bool:
		return fmt.Sprintf("%v", t)
	}
	return ""
}

func boolOf(v any) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// secondsAgo formats unix-epoch seconds as a humanish "Xm/Xh/Xd ago" string
// used by sync/schema/watch listings.
func secondsAgo(epoch int64) string {
	if epoch <= 0 {
		return ""
	}
	d := time.Since(time.Unix(epoch, 0))
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
