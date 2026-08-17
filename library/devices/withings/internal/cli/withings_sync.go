// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Withings-specific `sync` implementation (hand-authored; no "DO NOT EDIT"
// header). The generated sync.go issues GET requests through a generic
// cursor/offset pagination loop, neither of which fits the Withings API: every
// call is a form-encoded POST selected by an `action` field, and the row list
// lives under a per-endpoint body key (measuregrps / activities / series /
// devices) inside a {status, body} envelope that WithingsForm already unwraps.
//
// This command replaces newSyncCmd in the root tree (see root.go). It reuses
// the store's typed UpsertBatch (which writes both the generic resources table
// and the typed projection) and emits the same NDJSON sync_* event shape the
// generated sync uses, so downstream consumers are unaffected.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/withings/internal/store"
	"github.com/spf13/cobra"
)

// withingsSyncForm is the slice of *client.Client the sync loop needs. Narrowed
// to one method so the extraction/sync logic can be unit-tested with a fake.
type withingsSyncForm interface {
	WithingsForm(ctx context.Context, path string, form map[string]any) (json.RawMessage, error)
}

// withingsSyncSpec describes how to fetch and unpack one resource.
type withingsSyncSpec struct {
	path     string // endpoint path, e.g. "/measure"
	action   string // action selector, e.g. "getmeas"
	listKey  string // body key holding the row array, e.g. "measuregrps"
	idField  string // JSON field used as the stable store id (after lookup)
	dateKind string // "ymd" => startdateymd/enddateymd window; "epoch" => startdate + lastupdate
}

// withingsSyncSpecs maps each syncable resource to its endpoint contract. These
// are the resources `sync` knows how to mirror.
var withingsSyncSpecs = map[string]withingsSyncSpec{
	"measure":  {path: "/measure", action: "getmeas", listKey: "measuregrps", idField: "grpid", dateKind: "epoch"},
	"activity": {path: "/v2/measure", action: "getactivity", listKey: "activities", idField: "date", dateKind: "ymd"},
	"workouts": {path: "/v2/measure", action: "getworkouts", listKey: "series", idField: "id", dateKind: "ymd"},
	"sleep":    {path: "/v2/sleep", action: "getsummary", listKey: "series", idField: "date", dateKind: "ymd"},
	"heart":    {path: "/v2/heart", action: "list", listKey: "series", idField: "timestamp", dateKind: "epoch"},
	"devices":  {path: "/v2/user", action: "getdevice", listKey: "devices", idField: "deviceid", dateKind: ""},
}

// withingsSyncResourceOrder is the deterministic default sync order.
var withingsSyncResourceOrder = []string{"measure", "activity", "sleep", "workouts", "heart", "devices"}

// withingsDefaultSyncResources returns the resources sync mirrors when the user
// passes no --resources flag.
func withingsDefaultSyncResources() []string {
	out := make([]string, len(withingsSyncResourceOrder))
	copy(out, withingsSyncResourceOrder)
	return out
}

func newWithingsSyncCmd(flags *rootFlags) *cobra.Command {
	var resources []string
	var since string
	var dbPath string
	var full bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Withings data to local SQLite for offline analysis",
		Long: `Mirror Withings data into a local SQLite database so the analytics commands
(recomp, recovery, bp-report, sleep debt, digest, correlate) can run fully
offline.

Each resource is fetched with a form-encoded POST to its Withings endpoint and
its rows are upserted with a stable id (grpid for measure, date for activity /
sleep, id for workouts, timestamp for heart, deviceid for devices).

Examples:
  withings-pp-cli sync
  withings-pp-cli sync --resources measure,activity,sleep,workouts,heart,devices
  withings-pp-cli sync --resources measure --since 30d`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			if dbPath == "" {
				dbPath = defaultDBPath("withings-pp-cli")
			}

			if len(resources) == 0 {
				resources = withingsDefaultSyncResources()
			}
			// Validate resource names up front so a typo fails fast rather than
			// silently mirroring nothing.
			unknown := make([]string, 0)
			for _, r := range resources {
				if _, ok := withingsSyncSpecs[r]; !ok {
					unknown = append(unknown, r)
				}
			}
			if len(unknown) > 0 {
				sort.Strings(unknown)
				known := make([]string, 0, len(withingsSyncSpecs))
				for k := range withingsSyncSpecs {
					known = append(known, k)
				}
				sort.Strings(known)
				return usageErr(fmt.Errorf("unknown sync resource(s): %v (known: %v)", unknown, known))
			}

			// Resolve --since into a lookback window. Defaults to 365 days so a
			// first sync backfills a useful history.
			window := 365 * 24 * time.Hour
			if since != "" {
				d, derr := parseSinceFlag(since, window)
				if derr != nil {
					return usageErr(fmt.Errorf("invalid --since value %q: %w", since, derr))
				}
				window = d
			}
			cutoff := time.Now().Add(-window)

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			events := cmd.OutOrStdout()
			started := time.Now()
			var totalSynced, successCount, errCount int
			var firstErr error

			// Deterministic order: follow the canonical order, filtered to the
			// requested set.
			ordered := orderWithingsResources(resources)
			for _, resource := range ordered {
				if !humanFriendly {
					fmt.Fprintf(events, `{"event":"sync_start","resource":"%s"}`+"\n", resource)
				}
				count, dryRun, serr := withingsSyncResource(cmd.Context(), c, db, resource, cutoff, full)
				if serr != nil {
					errCount++
					if firstErr == nil {
						firstErr = serr
					}
					if humanFriendly {
						fmt.Fprintf(os.Stderr, "  %s: error: %v\n", resource, serr)
					} else {
						fmt.Fprintln(events, syncErrorJSON(resource, "", serr))
					}
					continue
				}
				if dryRun {
					if !humanFriendly {
						fmt.Fprintf(events, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
					}
					successCount++
					continue
				}
				totalSynced += count
				successCount++
				if humanFriendly {
					fmt.Fprintf(os.Stderr, "  %s: %d synced (done)\n", resource, count)
				} else {
					fmt.Fprintf(events, `{"event":"sync_complete","resource":"%s","total":%d}`+"\n", resource, count)
				}
			}

			elapsed := time.Since(started)
			if humanFriendly {
				fmt.Fprintf(os.Stderr, "Sync complete: %d records across %d resources (%.1fs)\n",
					totalSynced, successCount+errCount, elapsed.Seconds())
			} else {
				fmt.Fprintf(events, `{"event":"sync_summary","total_records":%d,"resources":%d,"success":%d,"errored":%d,"duration_ms":%d}`+"\n",
					totalSynced, successCount+errCount, successCount, errCount, elapsed.Milliseconds())
			}

			// Exit non-zero only when every requested resource failed; a partial
			// failure still leaves a usable mirror.
			if successCount == 0 && errCount > 0 {
				return classifyAPIError(firstErr, flags)
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&resources, "resources", nil, "Comma-separated resources to sync (measure,activity,sleep,workouts,heart,devices). Default: all.")
	cmd.Flags().StringVar(&since, "since", "", "Lookback window for the sync (e.g. 30d, 12w, 365d). Default: 365d.")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard data dir)")
	cmd.Flags().BoolVar(&full, "full", false, "Full resync window (alias for the default 365d backfill; kept for compatibility)")
	return cmd
}

// orderWithingsResources returns the requested resources in the canonical sync
// order, preserving any unknowns at the end (validation happens earlier).
func orderWithingsResources(requested []string) []string {
	want := map[string]bool{}
	for _, r := range requested {
		want[r] = true
	}
	out := make([]string, 0, len(requested))
	for _, r := range withingsSyncResourceOrder {
		if want[r] {
			out = append(out, r)
			delete(want, r)
		}
	}
	// Append any remaining (shouldn't happen after validation) deterministically.
	rest := make([]string, 0, len(want))
	for r := range want {
		rest = append(rest, r)
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// withingsSyncResource fetches one resource via WithingsForm, extracts its rows,
// and upserts them. Returns the stored count, whether the call was a dry-run
// sentinel, and any error.
func withingsSyncResource(ctx context.Context, c withingsSyncForm, db *store.Store, resource string, cutoff time.Time, full bool) (count int, dryRun bool, err error) {
	spec, ok := withingsSyncSpecs[resource]
	if !ok {
		return 0, false, fmt.Errorf("unknown sync resource %q", resource)
	}

	form := buildWithingsSyncForm(spec, cutoff)
	body, err := c.WithingsForm(ctx, spec.path, form)
	if err != nil {
		return 0, false, err
	}
	if isDryRunResponse(body) {
		return 0, true, nil
	}

	items, err := withingsExtractItems(body, spec.listKey)
	if err != nil {
		return 0, false, err
	}
	if len(items) == 0 {
		return 0, false, nil
	}

	// Assign each item a stable id under spec.idField so UpsertBatch keys the
	// generic + typed rows deterministically. Withings list items don't all
	// carry an "id" field, so we synthesize one when needed.
	prepared := make([]json.RawMessage, 0, len(items))
	for _, it := range items {
		withID, perr := withingsEnsureID(it, spec.idField)
		if perr != nil {
			continue
		}
		prepared = append(prepared, withID)
	}

	stored, _, err := db.UpsertBatch(resource, prepared)
	if err != nil {
		return stored, false, err
	}
	return stored, false, nil
}

// buildWithingsSyncForm assembles the request form for a resource sync,
// applying the appropriate date window for the endpoint's date convention.
func buildWithingsSyncForm(spec withingsSyncSpec, cutoff time.Time) map[string]any {
	form := map[string]any{"action": spec.action}
	switch spec.dateKind {
	case "ymd":
		form["startdateymd"] = cutoff.UTC().Format("2006-01-02")
		form["enddateymd"] = time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	case "epoch":
		form["startdate"] = int(cutoff.Unix())
		form["enddate"] = int(time.Now().Unix())
	}
	return form
}

// withingsExtractItems pulls the row array out of a Withings response body.
// It first looks for the spec's declared list key (measuregrps / activities /
// series / devices); when the body is itself already an array (verify/mock
// servers sometimes return the bare list) it returns that. A body with no
// recognizable list yields an empty slice, not an error.
func withingsExtractItems(body json.RawMessage, listKey string) ([]json.RawMessage, error) {
	// Bare array.
	var arr []json.RawMessage
	if json.Unmarshal(body, &arr) == nil {
		return arr, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, fmt.Errorf("parsing response body: %w", err)
	}

	// Preferred: the declared list key.
	if listKey != "" {
		if raw, ok := obj[listKey]; ok {
			var items []json.RawMessage
			if json.Unmarshal(raw, &items) == nil {
				return items, nil
			}
		}
	}

	// Fallback: the first array-valued field (handles series vs activities vs
	// devices without a per-call key when the declared one is absent).
	for _, key := range []string{"measuregrps", "activities", "series", "devices"} {
		if raw, ok := obj[key]; ok {
			var items []json.RawMessage
			if json.Unmarshal(raw, &items) == nil {
				return items, nil
			}
		}
	}
	return []json.RawMessage{}, nil
}

// withingsEnsureID returns item JSON guaranteed to carry a non-empty value at
// idField, synthesizing one from the field's existing value (re-keying "id" to
// the value of idField) so heterogeneous Withings shapes upsert deterministically.
// Returns an error only when no usable id can be derived.
func withingsEnsureID(item json.RawMessage, idField string) (json.RawMessage, error) {
	obj, err := store.DecodeJSONObject(item)
	if err != nil {
		return nil, err
	}
	id := withingsItemID(obj, idField)
	if id == "" {
		return nil, fmt.Errorf("no id for item (field %q)", idField)
	}
	// Ensure a generic "id" key is present (UpsertBatch's ExtractResourceID and
	// the typed extractObjectID both look at id/Id/ID first). Setting it makes
	// the store id stable and equal to the chosen field value.
	if _, ok := obj["id"]; !ok {
		obj["id"] = id
		raw, merr := json.Marshal(obj)
		if merr != nil {
			return nil, merr
		}
		return raw, nil
	}
	return item, nil
}

// withingsItemID resolves the stable id string for a Withings list item from
// the spec's idField, falling back to a couple of common alternatives. Numbers
// are rendered without scientific notation via store.ResourceIDString.
func withingsItemID(obj map[string]any, idField string) string {
	if v, ok := obj[idField]; ok {
		if s := store.ResourceIDString(v); s != "" && s != "<nil>" {
			return s
		}
	}
	for _, k := range []string{"id", "grpid", "date", "timestamp", "deviceid", "startdate"} {
		if v, ok := obj[k]; ok {
			if s := store.ResourceIDString(v); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}
