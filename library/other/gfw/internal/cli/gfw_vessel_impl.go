// Hand-authored — NOT generated. The unified `vessel` command: friendly
// search/get/list over GFW vessel identity plus the due-diligence transcendence
// features dossier/risk/ports/gaps. Survives regen as a whole hand-authored
// unit; wired from root.go via newVesselParentCmd.
//
// pp:data-source auto
// search/get/dossier/risk/ports/gaps go live to the GFW API and cache identity
// in the local store; list is local-only (browses what's already been fetched).
package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/gfw/internal/store"

	"github.com/spf13/cobra"
)

// gfwVesselIDPattern matches GFW vessel ids — UUID4-shaped with a flexible
// first segment (the API returns both standard 8-hex and 9-hex first segments,
// e.g. 550e8400-... and 8c7304226-...). Case-insensitive. Used to reject
// obvious typos and runner sentinels (e.g. "__printing_press_invalid__")
// before round-tripping to the API.
var gfwVesselIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8,12}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// newVesselParentCmd reuses the generated novel parent (keeping its stub
// constructors referenced for the `unused` linter) then replaces the TODO stubs
// with the real implementations.
func newVesselParentCmd(flags *rootFlags) *cobra.Command {
	cmd := newNovelVesselCmd(flags)
	cmd.Short = "Vessel identity, behavior, and risk from Global Fishing Watch (with a local cache)."
	cmd.Long = "Resolve vessels (search/get), browse your cached set (list), and run due-diligence views: 'dossier' (identity + events + insights merged), 'risk', 'ports', and 'gaps' (dark-activity)."
	cmd.ResetCommands()
	cmd.AddCommand(newVesselSearchCmd(flags))
	cmd.AddCommand(newVesselGetCmd(flags))
	cmd.AddCommand(newVesselListCmd(flags))
	cmd.AddCommand(newVesselDossierCmd(flags))
	cmd.AddCommand(newVesselRiskCmd(flags))
	cmd.AddCommand(newVesselPortsCmd(flags))
	cmd.AddCommand(newVesselGapsCmd(flags))
	return cmd
}

// --- shared helpers ---

func eventType(raw json.RawMessage) string {
	var e struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(raw, &e)
	return e.Type
}

func countEventTypes(entries []json.RawMessage) map[string]int {
	counts := map[string]int{}
	for _, e := range entries {
		t := eventType(e)
		if t == "" {
			t = "unknown"
		}
		counts[t]++
	}
	return counts
}

// resolveVesselID returns the trimmed positional vessel id, or a usage error.
// Validates UUID-ish shape so user typos and runner sentinels fail-fast at
// exit code 2 rather than round-tripping to the API and returning empty data.
func resolveVesselID(args []string) (string, error) {
	if len(args) < 1 {
		return "", usageErr(fmt.Errorf("a GFW vessel id is required (resolve one with 'vessel search')"))
	}
	id := strings.TrimSpace(args[0])
	if id == "" {
		return "", usageErr(fmt.Errorf("a GFW vessel id is required"))
	}
	if !gfwVesselIDPattern.MatchString(id) {
		return "", usageErr(fmt.Errorf("not a GFW vessel id: %q (expected UUID-shaped id like 8c7304226-6c71-edbe-0b63-c246734b3c01; resolve one with 'vessel search')", id))
	}
	return id, nil
}

// --- search ---

func newVesselSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Search GFW vessels by name, MMSI, IMO, or callsign.",
		Long:        "Free-text search over GFW vessel identity (registry + AIS). Returns flattened identities (GFW id, name, flag, MMSI, IMO) and caches each so 'vessel list' can browse them offline. Use the GFW id with dossier/risk/events.",
		Example:     "  gfw-pp-cli vessel search \"PROGRESS 10\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return usageErr(fmt.Errorf("a search query is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := searchVessels(cmd.Context(), c, query, limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			ids := []vesselIdentity{}
			for _, e := range entriesOf(raw) {
				vi := extractVesselIdentity(mustEnvelope(e), "")
				if vi.ID == "" {
					continue
				}
				ids = append(ids, vi)
				if cerr := cacheVesselIdentity(cmd.Context(), flags, vi); cerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: cache %s: %v\n", vi.ID, cerr)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), ids, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum vessels to return")
	return cmd
}

// mustEnvelope wraps a bare vessel entry as {"entries":[entry]} so
// extractVesselIdentity (which expects a list-or-object) reads it uniformly.
func mustEnvelope(entry json.RawMessage) json.RawMessage {
	wrapped, err := json.Marshal(struct {
		Entries []json.RawMessage `json:"entries"`
	}{Entries: []json.RawMessage{entry}})
	if err != nil {
		return entry
	}
	return wrapped
}

// --- get ---

func newVesselGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get <vesselId>",
		Short:       "Get a vessel's GFW identity by id (and cache it).",
		Example:     "  gfw-pp-cli vessel get 8c7304226-6c71-edbe-0b63-c246734b3c01 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := resolveVesselID(args)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := fetchVesselByID(cmd.Context(), c, id)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			vi := extractVesselIdentity(raw, id)
			if cerr := cacheVesselIdentity(cmd.Context(), flags, vi); cerr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: cache %s: %v\n", id, cerr)
			}
			return printJSONFiltered(cmd.OutOrStdout(), vi, flags)
		},
	}
	return cmd
}

// --- list (offline cache) ---

func newVesselListCmd(flags *rootFlags) *cobra.Command {
	var flag, nameLike string
	var pinned bool
	var limit int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Browse vessels you've already fetched (offline cache).",
		Long:        "Lists vessel identities cached by 'vessel search'/'vessel get'/'vessel dossier', with optional flag/name filters and a pinned-only view.",
		Example:     "  gfw-pp-cli vessel list --flag COK --json\n  gfw-pp-cli vessel list --pinned",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStoreForRead(cmd.Context(), "gfw-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []store.VesselRow{}, flags)
			}
			defer db.Close()
			rows, err := db.ListVessels(store.ListVesselsOptions{Flag: flag, NameLike: nameLike, PinnedOnly: pinned, Limit: limit})
			if err != nil {
				return fmt.Errorf("querying local vessel cache: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&flag, "flag", "", "Filter by flag (exact, case-insensitive)")
	cmd.Flags().StringVar(&nameLike, "name-like", "", "Substring search on name/IMO/MMSI")
	cmd.Flags().BoolVar(&pinned, "pinned", false, "Only list watchlisted vessels")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum rows to return")
	return cmd
}

// --- dossier (identity + events + insights merged) ---

type vesselDossier struct {
	VesselID      string            `json:"vessel_id"`
	Identity      vesselIdentity    `json:"identity"`
	EventCounts   map[string]int    `json:"event_counts"`
	RecentEvents  []json.RawMessage `json:"recent_events,omitempty"`
	Insights      json.RawMessage   `json:"insights,omitempty"`
	InsightsError string            `json:"insights_error,omitempty"`
}

func newVesselDossierCmd(flags *rootFlags) *cobra.Command {
	var eventLimit, recent int
	cmd := &cobra.Command{
		Use:         "dossier <vesselId>",
		Short:       "One-shot DD snapshot: identity + recent events + risk insights merged.",
		Long:        "Fetches a vessel's GFW identity, its recent events across all behavioral datasets (encounters, port visits, loitering, gaps, fishing), and its risk insights, then merges them. For just identity use 'vessel get'; for raw events use 'events list'.",
		Example:     "  gfw-pp-cli vessel dossier 8c7304226-6c71-edbe-0b63-c246734b3c01 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := resolveVesselID(args)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vraw, err := fetchVesselByID(cmd.Context(), c, id)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			vi := extractVesselIdentity(vraw, id)
			_ = cacheVesselIdentity(cmd.Context(), flags, vi)

			d := vesselDossier{VesselID: id, Identity: vi}
			eraw, eerr := fetchEvents(cmd.Context(), c, id, allEventDatasets, eventLimit, "")
			if eerr == nil {
				entries := entriesOf(eraw)
				d.EventCounts = countEventTypes(entries)
				if recent > 0 && len(entries) > recent {
					d.RecentEvents = entries[:recent]
				} else {
					d.RecentEvents = entries
				}
			} else {
				d.EventCounts = map[string]int{}
			}
			start, end := defaultInsightDates()
			if iraw, ierr := fetchInsights(cmd.Context(), c, id, nil, start, end); ierr == nil {
				d.Insights = iraw
			} else {
				d.InsightsError = ierr.Error()
			}
			return printJSONFiltered(cmd.OutOrStdout(), d, flags)
		},
	}
	cmd.Flags().IntVar(&eventLimit, "event-limit", 50, "Max events to fetch across datasets")
	cmd.Flags().IntVar(&recent, "recent", 10, "Number of recent events to include in the dossier")
	return cmd
}

// --- risk ---

type vesselRiskView struct {
	VesselID      string          `json:"vessel_id"`
	Score         int             `json:"score"`
	Level         string          `json:"level"`
	Signals       []string        `json:"signals"`
	EventCounts   map[string]int  `json:"event_counts"`
	Insights      json.RawMessage `json:"insights,omitempty"`
	InsightsError string          `json:"insights_error,omitempty"`
}

// scoreRisk turns event counts into a 0-100 heuristic + human signals. Pure
// function (no IO) so it is unit-testable.
func scoreRisk(counts map[string]int) (int, string, []string) {
	weights := map[string]int{"GAP": 18, "gap": 18, "ENCOUNTER": 12, "encounter": 12, "LOITERING": 8, "loitering": 8, "PORT_VISIT": 1, "port_visit": 1, "FISHING": 1, "fishing": 1}
	score := 0
	var signals []string
	for t, n := range counts {
		w := weights[t]
		if w == 0 {
			w = 1
		}
		score += w * n
		if n > 0 && (strings.EqualFold(t, "gap") || strings.EqualFold(t, "encounter") || strings.EqualFold(t, "loitering")) {
			signals = append(signals, fmt.Sprintf("%d %s event(s)", n, strings.ToLower(t)))
		}
	}
	if score > 100 {
		score = 100
	}
	sort.Strings(signals)
	level := "low"
	switch {
	case score >= 60:
		level = "high"
	case score >= 30:
		level = "medium"
	}
	if len(signals) == 0 {
		signals = []string{"no high-risk behavioral events in window"}
	}
	return score, level, signals
}

func newVesselRiskCmd(flags *rootFlags) *cobra.Command {
	var eventLimit int
	cmd := &cobra.Command{
		Use:         "risk <vesselId>",
		Short:       "Composite risk rollup from insights + event patterns (encounters, gaps, loitering).",
		Long:        "Fetches risk insights and recent gap/encounter/loitering events and computes a 0-100 risk heuristic with human-readable signals. Not a substitute for the raw 'insights' indicators.",
		Example:     "  gfw-pp-cli vessel risk 8c7304226-6c71-edbe-0b63-c246734b3c01 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := resolveVesselID(args)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			counts := map[string]int{}
			if eraw, eerr := fetchEvents(cmd.Context(), c, id, []string{dsGaps, dsEncounters, dsLoitering}, eventLimit, ""); eerr == nil {
				counts = countEventTypes(entriesOf(eraw))
			} else {
				return classifyAPIError(eerr, flags)
			}
			score, level, signals := scoreRisk(counts)
			rv := vesselRiskView{VesselID: id, Score: score, Level: level, Signals: signals, EventCounts: counts}
			start, end := defaultInsightDates()
			if iraw, ierr := fetchInsights(cmd.Context(), c, id, nil, start, end); ierr == nil {
				rv.Insights = iraw
			} else {
				rv.InsightsError = ierr.Error()
			}
			return printJSONFiltered(cmd.OutOrStdout(), rv, flags)
		},
	}
	cmd.Flags().IntVar(&eventLimit, "event-limit", 100, "Max events to scan for the risk signal")
	return cmd
}

// --- ports ---

type portsView struct {
	VesselID    string            `json:"vessel_id"`
	TotalVisits int               `json:"total_visits"`
	Events      []json.RawMessage `json:"port_visits"`
}

func newVesselPortsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "ports <vesselId>",
		Short:       "A vessel's port-visit events (newest first).",
		Example:     "  gfw-pp-cli vessel ports 8c7304226-6c71-edbe-0b63-c246734b3c01 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := resolveVesselID(args)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := fetchEvents(cmd.Context(), c, id, []string{dsPortVisits}, limit, "")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			entries := entriesOf(raw)
			return printJSONFiltered(cmd.OutOrStdout(), portsView{VesselID: id, TotalVisits: len(entries), Events: entries}, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max port-visit events to return")
	return cmd
}

// --- gaps (dark activity) ---

type gapsView struct {
	VesselID string            `json:"vessel_id"`
	Count    int               `json:"count"`
	Events   []json.RawMessage `json:"events"`
}

func newVesselGapsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "gaps <vesselId>",
		Short:       "AIS-gap and loitering events — a dark-activity signal.",
		Long:        "Fetches a vessel's AIS-gap and loitering events, which can indicate AIS disabling (a sanctions-evasion signal). For raw events use 'events list --datasets-0 public-global-gaps-events:latest'.",
		Example:     "  gfw-pp-cli vessel gaps 8c7304226-6c71-edbe-0b63-c246734b3c01 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := resolveVesselID(args)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := fetchEvents(cmd.Context(), c, id, []string{dsGaps, dsLoitering}, limit, "")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			entries := entriesOf(raw)
			return printJSONFiltered(cmd.OutOrStdout(), gapsView{VesselID: id, Count: len(entries), Events: entries}, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max gap/loitering events to return")
	return cmd
}
