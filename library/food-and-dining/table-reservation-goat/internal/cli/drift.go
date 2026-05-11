package cli

// PATCH: novel-commands — see .printing-press-patches.json for the change-set rationale.

// pp:client-call — `drift` calls the OpenTable and Tock clients via
// `internal/source/opentable` and `internal/source/tock` to capture each
// snapshot, then diffs against a local on-disk snapshot file. Dogfood's
// reimplementation_check sibling-import regex doesn't match multi-segment
// `internal/source/...` paths. Documented carve-out per AGENTS.md.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/opentable"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/resy"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/tock"
)

type driftSnapshot struct {
	CapturedAt time.Time      `json:"captured_at"`
	Venue      string         `json:"venue"`
	Network    string         `json:"network"`
	Hash       string         `json:"hash"`
	Fields     map[string]any `json:"fields"`
}

type driftChange struct {
	Field string `json:"field"`
	Old   any    `json:"old,omitempty"`
	New   any    `json:"new,omitempty"`
	Kind  string `json:"kind"` // added, removed, changed
}

type driftResponse struct {
	Venue       string        `json:"venue"`
	Network     string        `json:"network"`
	HasPrior    bool          `json:"has_prior"`
	PriorAt     *time.Time    `json:"prior_at,omitempty"`
	CurrentAt   time.Time     `json:"current_at"`
	Changed     bool          `json:"changed"`
	Changes     []driftChange `json:"changes"`
	SnapshotKey string        `json:"snapshot_key"`
}

// newDriftCmd surfaces what changed at a single venue between snapshots.
// First invocation per venue stores a baseline snapshot in
// `~/.cache/table-reservation-goat-pp-cli/drift/<network>/<slug>.json`;
// subsequent invocations diff and rewrite the snapshot. This is a pure
// local-store comparison — no API call cleverness, just rigorous diffing.
func newDriftCmd(flags *rootFlags) *cobra.Command {
	var since string
	_ = since
	cmd := &cobra.Command{
		Use:   "drift <restaurant>",
		Short: "Show what changed at a venue since the last drift snapshot",
		Long: "Captures a snapshot of the current restaurant detail (and Tock " +
			"experience offerings, if applicable) and diffs against the previous " +
			"local snapshot. First invocation captures the baseline; subsequent " +
			"invocations report the diff and overwrite the snapshot.",
		Example: "  table-reservation-goat-pp-cli drift alinea --since 7d --agent",
		// No mcp:read-only — every invocation writes a snapshot file to
		// ~/.cache/table-reservation-goat-pp-cli/drift/<network>/<slug>.json
		// via writeSnapshot(). The first call also creates the directory tree.
		// MCP hosts that honor read-only would otherwise skip the call in
		// side-effect-prohibited contexts and the snapshot baseline would
		// silently never be captured.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			input := args[0]
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), driftResponse{
					Venue: input, Network: "opentable", HasPrior: false,
					CurrentAt: time.Now().UTC(), SnapshotKey: "(dry-run)",
				}, flags)
			}
			session, err := auth.Load()
			if err != nil {
				return fmt.Errorf("loading session: %w", err)
			}
			network, slug := parseNetworkSlug(input)
			ctx := cmd.Context()
			fields, resolvedNetwork, err := captureDriftFields(ctx, session, network, slug)
			if err != nil {
				return fmt.Errorf("capturing snapshot: %w", err)
			}
			snap := driftSnapshot{
				CapturedAt: time.Now().UTC(),
				Venue:      slug,
				Network:    resolvedNetwork,
				Hash:       hashFields(fields),
				Fields:     fields,
			}
			// Read prior + write current
			path, err := driftSnapshotPath(resolvedNetwork, slug)
			if err != nil {
				return err
			}
			prior, hadPrior := readPriorSnapshot(path)
			if err := writeSnapshot(path, snap); err != nil {
				return fmt.Errorf("writing snapshot: %w", err)
			}
			out := driftResponse{
				Venue:       slug,
				Network:     resolvedNetwork,
				HasPrior:    hadPrior,
				CurrentAt:   snap.CapturedAt,
				SnapshotKey: snap.Hash[:16],
			}
			if hadPrior {
				out.PriorAt = &prior.CapturedAt
				out.Changes = diffFields(prior.Fields, snap.Fields)
				out.Changed = len(out.Changes) > 0
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Inform what time horizon mattered to the user (e.g., '7d', '2026-04-01'); informational — drift always compares against the last snapshot")
	return cmd
}

func captureDriftFields(ctx context.Context, s *auth.Session, network, slug string) (map[string]any, string, error) {
	tryOT := network == "" || network == "opentable"
	tryTock := network == "" || network == "tock"
	tryResy := network == "resy" // explicit prefix only; Resy doesn't share slug-space with OT/Tock

	if tryResy {
		return captureDriftFieldsResy(ctx, s, slug)
	}
	if tryOT {
		c, err := opentable.New(s)
		if err == nil {
			r, err := c.RestaurantBySlug(ctx, slug)
			if err == nil && r != nil {
				return pickRestaurantFields(r), "opentable", nil
			}
		}
	}
	if tryTock {
		c, err := tock.New(s)
		if err == nil {
			detail, err := c.VenueDetail(ctx, slug)
			if err == nil {
				out := map[string]any{}
				if biz, ok := detail["business"].(map[string]any); ok {
					out["business"] = pickBusinessFields(biz)
				}
				if cal, ok := detail["calendar"].(map[string]any); ok {
					if offerings, ok := cal["offerings"].(map[string]any); ok {
						if exp, ok := offerings["experience"].([]any); ok {
							out["experience_count"] = len(exp)
							names := []string{}
							for _, e := range exp {
								if em, ok := e.(map[string]any); ok {
									if n, ok := em["name"].(string); ok {
										names = append(names, n)
									}
								}
							}
							sort.Strings(names)
							out["experience_names"] = names
						}
					}
				}
				return out, "tock", nil
			}
		}
	}
	return nil, "unknown", fmt.Errorf("could not resolve %s on either network", slug)
}

// captureDriftFieldsResy snapshots the Resy-side fields that change
// meaningfully between polls. Resy doesn't expose a public restaurant-detail
// endpoint analogous to OT's BySlug or Tock's VenueDetail SSR — the
// reservation surface is the most stable signal of "what changed at this
// venue", so we capture the next 7 days of slot count by seating-area type
// as the drift baseline.
func captureDriftFieldsResy(ctx context.Context, s *auth.Session, venueID string) (map[string]any, string, error) {
	if s == nil || s.Resy == nil || s.Resy.AuthToken == "" {
		return nil, "resy", fmt.Errorf("resy: not authenticated; run `auth login --resy --email <you@example.com>` first")
	}
	if _, err := strconv.Atoi(venueID); err != nil {
		return nil, "resy", fmt.Errorf("resy: %q is not a numeric venue id", venueID)
	}
	client := resy.New(resy.Credentials{
		APIKey:    s.Resy.APIKey,
		AuthToken: s.Resy.AuthToken,
		Email:     s.Resy.Email,
	})
	today := time.Now()
	out := map[string]any{
		"venue_id": venueID,
	}
	totalSlots := 0
	bySeatingArea := map[string]int{}
	earliestSlot := ""
	successfulDays := 0
	var lastErr error
	for d := 0; d < 7; d++ {
		day := today.AddDate(0, 0, d).Format("2006-01-02")
		slots, err := client.Availability(ctx, resy.AvailabilityParams{
			VenueID:   venueID,
			Date:      day,
			PartySize: 2,
		})
		if err != nil {
			// Don't fail on a single bad day — Resy 5xxs are transient,
			// and a partial 7-day snapshot is still useful drift data.
			// But track per-day errors so we can refuse to write a
			// baseline derived from ZERO successful calls.
			lastErr = err
			continue
		}
		successfulDays++
		for _, sl := range slots {
			totalSlots++
			label := sl.Type
			if label == "" {
				label = "Standard"
			}
			bySeatingArea[label]++
			ts := day + "T" + sl.Time
			if earliestSlot == "" || ts < earliestSlot {
				earliestSlot = ts
			}
		}
	}
	// If every per-day call failed, the snapshot would record
	// `total_slots_7d=0` as if the venue had zero availability for the
	// whole week — and the next time drift runs after auth/network
	// recovery, the legitimate slot count would diff against that fake
	// zero baseline and trigger a spurious "slot count exploded from 0
	// to N" alert. Refuse to baseline from an all-error scan; the
	// caller propagates the error so the snapshot file is not written.
	if successfulDays == 0 {
		if lastErr != nil {
			return nil, "resy", fmt.Errorf("resy venue=%s: every per-day availability call failed; refusing to baseline a zero-slot snapshot. Last error: %w", venueID, lastErr)
		}
		return nil, "resy", fmt.Errorf("resy venue=%s: no per-day calls succeeded; refusing to baseline a zero-slot snapshot", venueID)
	}
	out["total_slots_7d"] = totalSlots
	out["earliest_slot_7d"] = earliestSlot
	out["successful_days_7d"] = successfulDays
	keys := make([]string, 0, len(bySeatingArea))
	for k := range bySeatingArea {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	areaCounts := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		areaCounts = append(areaCounts, map[string]any{"type": k, "count": bySeatingArea[k]})
	}
	out["seating_area_counts_7d"] = areaCounts
	return out, "resy", nil
}

var driftRestaurantFields = []string{
	"name", "cuisine", "priceBand", "diningStyle", "dressCode", "phone",
	"website", "maxAdvanceDays", "address", "hoursOfOperation",
}

var driftBusinessFields = []string{
	"name", "cuisine", "city", "state", "country", "phone", "webUrl", "address",
}

func pickRestaurantFields(in map[string]any) map[string]any {
	return pickFields(in, driftRestaurantFields)
}

func pickBusinessFields(in map[string]any) map[string]any {
	return pickFields(in, driftBusinessFields)
}

func pickFields(in map[string]any, keys []string) map[string]any {
	out := map[string]any{}
	for _, k := range keys {
		if v, ok := in[k]; ok && v != nil {
			out[k] = v
		}
	}
	return out
}

func hashFields(fields map[string]any) string {
	js, _ := json.Marshal(fields)
	sum := sha256.Sum256(js)
	return hex.EncodeToString(sum[:])
}

func driftSnapshotPath(network, slug string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".cache", "table-reservation-goat-pp-cli", "drift", network)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	clean := strings.ReplaceAll(slug, "/", "_")
	return filepath.Join(dir, clean+".json"), nil
}

func readPriorSnapshot(path string) (driftSnapshot, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return driftSnapshot{}, false
	}
	var s driftSnapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return driftSnapshot{}, false
	}
	return s, true
}

func writeSnapshot(path string, s driftSnapshot) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func diffFields(prior, current map[string]any) []driftChange {
	changes := []driftChange{}
	keys := map[string]struct{}{}
	for k := range prior {
		keys[k] = struct{}{}
	}
	for k := range current {
		keys[k] = struct{}{}
	}
	sortedKeys := make([]string, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)
	for _, k := range sortedKeys {
		oldV, oldOK := prior[k]
		newV, newOK := current[k]
		switch {
		case !oldOK && newOK:
			changes = append(changes, driftChange{Field: k, New: newV, Kind: "added"})
		case oldOK && !newOK:
			changes = append(changes, driftChange{Field: k, Old: oldV, Kind: "removed"})
		case !deepEqual(oldV, newV):
			changes = append(changes, driftChange{Field: k, Old: oldV, New: newV, Kind: "changed"})
		}
	}
	return changes
}

func deepEqual(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}
