package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/store"

	"github.com/spf13/cobra"
)

// Hostfully's REST leads endpoint is only reliable when filtered by
// propertyUid, so every per-unit command needs a map from a short unit number
// to the agency's property UID. The map is resolved once per process from, in
// order: HOSTFULLY_UNITS, a units.json file, properties already synced into
// the local store, and finally a live /properties read for the agency.

const unitsEnvVar = "HOSTFULLY_UNITS"
const unitsFileEnvVar = "HOSTFULLY_UNITS_FILE"

type unitRegistry struct {
	timeZone   string
	byUnit     map[int]string
	byProperty map[string]int
	names      map[int]string
	source     string
}

type unitEntry struct {
	Unit        int    `json:"unit"`
	PropertyUID string `json:"propertyUid"`
	Name        string `json:"name,omitempty"`
}

var unitState struct {
	mu    sync.Mutex
	flags *rootFlags
	ready bool
	reg   unitRegistry
	err   error
}

var mockModeEnabled bool

func mockMode() bool { return mockModeEnabled }

func bindUnitFlags(flags *rootFlags) {
	unitState.mu.Lock()
	unitState.flags = flags
	unitState.mu.Unlock()
}

// setUnitRegistry replaces the resolved registry. Tests use it to pin units.
func setUnitRegistry(byUnit map[int]string, source string) {
	unitState.mu.Lock()
	defer unitState.mu.Unlock()
	unitState.reg = newUnitRegistry(byUnit, nil, source)
	unitState.err = nil
	unitState.ready = true
}

func resetUnitRegistry() {
	unitState.mu.Lock()
	defer unitState.mu.Unlock()
	unitState.ready = false
	unitState.reg = unitRegistry{}
	unitState.err = nil
}

func newUnitRegistry(byUnit map[int]string, names map[int]string, source string) unitRegistry {
	reg := unitRegistry{
		byUnit:     make(map[int]string, len(byUnit)),
		byProperty: make(map[string]int, len(byUnit)),
		names:      make(map[int]string, len(byUnit)),
		source:     source,
	}
	for unit, uid := range byUnit {
		reg.byUnit[unit] = uid
		reg.byProperty[uid] = unit
		if names != nil {
			reg.names[unit] = names[unit]
		}
	}
	return reg
}

func currentUnits() (unitRegistry, error) {
	unitState.mu.Lock()
	defer unitState.mu.Unlock()
	if !unitState.ready {
		unitState.reg, unitState.err = loadUnitRegistry(unitState.flags)
		unitState.ready = true
	}
	return unitState.reg, unitState.err
}

// requireUnits resolves the unit map, returning an actionable error when no
// source produced one.
func requireUnits() (unitRegistry, error) {
	reg, err := currentUnits()
	if err != nil {
		return reg, err
	}
	if len(reg.byUnit) == 0 {
		return reg, fmt.Errorf("no Hostfully properties found: set %s (e.g. \"1=<property-uid>,2=<property-uid>\"), add a units.json, or check HOSTFULLY_AGENCY_ID", unitsEnvVar)
	}
	return reg, nil
}

func unitUID(unit int) (string, bool) {
	reg, _ := currentUnits()
	uid, ok := reg.byUnit[unit]
	return uid, ok
}

func unitForProperty(propertyUID string) int {
	reg, _ := currentUnits()
	return reg.byProperty[propertyUID]
}

func unitNumbers() []int {
	reg, _ := currentUnits()
	units := make([]int, 0, len(reg.byUnit))
	for unit := range reg.byUnit {
		units = append(units, unit)
	}
	sort.Ints(units)
	return units
}

func unitUIDMap() map[int]string {
	reg, _ := currentUnits()
	out := make(map[int]string, len(reg.byUnit))
	for k, v := range reg.byUnit {
		out[k] = v
	}
	return out
}

func loadUnitRegistry(flags *rootFlags) (unitRegistry, error) {
	if mockMode() {
		return newUnitRegistry(map[int]string{
			1: "mock-hostfully-property-unit-1",
			2: "mock-hostfully-property-unit-2",
			3: "mock-hostfully-property-unit-3",
			4: "mock-hostfully-property-unit-4",
		}, nil, "mock"), nil
	}
	if raw := strings.TrimSpace(os.Getenv(unitsEnvVar)); raw != "" {
		byUnit, err := parseUnitsSpec(raw)
		if err != nil {
			return unitRegistry{}, fmt.Errorf("%s: %w", unitsEnvVar, err)
		}
		return newUnitRegistry(byUnit, nil, "env:"+unitsEnvVar), nil
	}
	if path := unitsFilePath(); path != "" {
		if data, err := os.ReadFile(path); err == nil { // #nosec G304 -- operator-configured units registry path (config dir or HOSTFULLY_UNITS_FILE)
			var entries []unitEntry
			if err := json.Unmarshal(data, &entries); err != nil {
				return unitRegistry{}, fmt.Errorf("parsing %s: %w", path, err)
			}
			byUnit := make(map[int]string, len(entries))
			names := make(map[int]string, len(entries))
			for _, e := range entries {
				if e.Unit <= 0 || e.PropertyUID == "" {
					return unitRegistry{}, fmt.Errorf("parsing %s: every entry needs a positive unit and a propertyUid", path)
				}
				byUnit[e.Unit] = e.PropertyUID
				names[e.Unit] = e.Name
			}
			return newUnitRegistry(byUnit, names, "file:"+path), nil
		}
	}
	if props := storedProperties(); len(props) > 0 {
		byUnit, names := numberProperties(props)
		reg := newUnitRegistry(byUnit, names, "local-store")
		reg.timeZone = firstTimeZone(props)
		return reg, nil
	}
	props, err := liveProperties(flags)
	if err != nil {
		return unitRegistry{}, err
	}
	byUnit, names := numberProperties(props)
	reg := newUnitRegistry(byUnit, names, "hostfully_live")
	reg.timeZone = firstTimeZone(props)
	return reg, nil
}

func unitsFilePath() string {
	if p := strings.TrimSpace(os.Getenv(unitsFileEnvVar)); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "hostfully-pp-cli", "units.json")
}

func parseUnitsSpec(raw string) (map[int]string, error) {
	out := make(map[int]string)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("entry %q must be <unit>=<property-uid>", part)
		}
		unit, err := strconv.Atoi(strings.TrimSpace(k))
		if err != nil || unit <= 0 {
			return nil, fmt.Errorf("entry %q: unit must be a positive integer", part)
		}
		uid := strings.TrimSpace(v)
		if uid == "" {
			return nil, fmt.Errorf("entry %q: property uid is empty", part)
		}
		out[unit] = uid
	}
	return out, nil
}

type propertySummary struct {
	TimeZone string `json:"timeZone"`
	UID      string `json:"uid"`
	Name     string `json:"name"`
	IsActive *bool  `json:"isActive"`
}

var unitNumberInName = regexp.MustCompile(`(\d+)\s*$`)

// numberProperties assigns stable unit numbers. When every active property's
// name ends in a distinct number ("Cottage #3", "Suite 12") that number is
// the unit; otherwise units are 1..N in name order.
func numberProperties(props []propertySummary) (map[int]string, map[int]string) {
	active := make([]propertySummary, 0, len(props))
	for _, p := range props {
		if p.UID == "" || (p.IsActive != nil && !*p.IsActive) {
			continue
		}
		active = append(active, p)
	}
	sort.SliceStable(active, func(i, j int) bool {
		if active[i].Name == active[j].Name {
			return active[i].UID < active[j].UID
		}
		return active[i].Name < active[j].Name
	})
	byUnit := make(map[int]string, len(active))
	names := make(map[int]string, len(active))
	fromNames := true
	for _, p := range active {
		m := unitNumberInName.FindStringSubmatch(p.Name)
		if m == nil {
			fromNames = false
			break
		}
		n, err := strconv.Atoi(m[1])
		if err != nil || n <= 0 {
			fromNames = false
			break
		}
		if _, dup := byUnit[n]; dup {
			fromNames = false
			break
		}
		byUnit[n] = p.UID
		names[n] = p.Name
	}
	if fromNames {
		return byUnit, names
	}
	byUnit = make(map[int]string, len(active))
	names = make(map[int]string, len(active))
	for i, p := range active {
		byUnit[i+1] = p.UID
		names[i+1] = p.Name
	}
	return byUnit, names
}

func storedProperties() []propertySummary {
	path := defaultDBPath("hostfully-pp-cli")
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	db, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		return nil
	}
	defer db.Close()
	rows, err := db.DB().Query(`SELECT data FROM resources WHERE resource_type = 'properties'`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var props []propertySummary
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var p propertySummary
		if json.Unmarshal(data, &p) == nil {
			props = append(props, p)
		}
	}
	return props
}

func liveProperties(flags *rootFlags) ([]propertySummary, error) {
	configPath := ""
	if flags != nil {
		configPath = flags.configPath
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	agencyUID := cfg.AgencyUID()
	if agencyUID == "" {
		return nil, fmt.Errorf("HOSTFULLY_AGENCY_ID environment variable is required to discover properties (or set %s)", unitsEnvVar)
	}
	timeout := 30 * time.Second
	rateLimit := 0.0
	if flags != nil {
		timeout = flags.timeout
		rateLimit = flags.rateLimit
	}
	c := client.New(cfg, timeout, rateLimit)
	c.NoCache = true
	ctx, cancel := boundCtx(context.Background(), flags)
	defer cancel()

	var props []propertySummary
	cursor := ""
	for page := 0; page < 50; page++ {
		params := map[string]string{"agencyUid": agencyUID, "_limit": "200"}
		if cursor != "" {
			params["_cursor"] = cursor
		}
		raw, err := c.Get(ctx, "/properties", params)
		if err != nil {
			return nil, fmt.Errorf("discovering Hostfully properties: %w", err)
		}
		var body struct {
			Properties []propertySummary `json:"properties"`
			Paging     struct {
				NextCursor string `json:"_nextCursor"`
			} `json:"_paging"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, fmt.Errorf("decoding Hostfully properties: %w", err)
		}
		props = append(props, body.Properties...)
		if body.Paging.NextCursor == "" || body.Paging.NextCursor == cursor {
			break
		}
		cursor, _ = url.QueryUnescape(body.Paging.NextCursor)
	}
	return props, nil
}

func newUnitsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "units",
		Short: "Show how unit numbers map to your Hostfully properties",
		Long: `Every per-unit command (today, availability, whois, quote, vacancy, ...) addresses
properties by a short unit number. The mapping comes from, in order:
HOSTFULLY_UNITS ("1=<property-uid>,2=<property-uid>"), units.json in the config
directory (or HOSTFULLY_UNITS_FILE), properties synced into the local store,
or a live read of the agency's properties. Live discovery uses the trailing
number in each property name when those are distinct, otherwise 1..N by name.`,
		Example:     "  hostfully-pp-cli units\n  hostfully-pp-cli units --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "units")
			}
			reg, err := requireUnits()
			if err != nil {
				return err
			}
			entries := make([]unitEntry, 0, len(reg.byUnit))
			for _, unit := range unitNumbers() {
				entries = append(entries, unitEntry{Unit: unit, PropertyUID: reg.byUnit[unit], Name: reg.names[unit]})
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"source": reg.source, "units": entries}, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Source: %s\n", reg.source)
			for _, e := range entries {
				fmt.Fprintf(w, "%3d  %s  %s\n", e.Unit, e.PropertyUID, e.Name)
			}
			return nil
		},
	}
	return cmd
}

func mustUnitUID(unit int) string {
	uid, _ := unitUID(unit)
	return uid
}

// boundLeadClient adapts the context-aware generated client to the
// context-free hostfullyLeadAPI used by the per-property fetch helpers.
type boundLeadClient struct {
	ctx context.Context
	c   *client.Client
}

func bindLeadAPI(ctx context.Context, c *client.Client) hostfullyLeadAPI {
	return boundLeadClient{ctx: ctx, c: c}
}

func (b boundLeadClient) GetNoRetry(path string, params map[string]string) (json.RawMessage, error) {
	return b.c.GetNoRetry(b.ctx, path, params)
}

// Post is only used for read-only GraphQL queries, so it rides the client's
// read-intent path rather than the mutating-verb path.
func (b boundLeadClient) Post(path string, body any) (json.RawMessage, int, error) {
	return b.c.PostQueryWithParams(b.ctx, path, nil, body)
}

const timeZoneEnvVar = "HOSTFULLY_TIMEZONE"

func firstTimeZone(props []propertySummary) string {
	for _, p := range props {
		if p.TimeZone != "" && (p.IsActive == nil || *p.IsActive) {
			return p.TimeZone
		}
	}
	return ""
}

// propertyLocation is the time zone "today" is evaluated in: HOSTFULLY_TIMEZONE,
// else the agency properties' timeZone, else the machine's local zone.
func propertyLocation() (*time.Location, error) {
	if name := strings.TrimSpace(os.Getenv(timeZoneEnvVar)); name != "" {
		loc, err := time.LoadLocation(name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", timeZoneEnvVar, err)
		}
		return loc, nil
	}
	if !mockMode() {
		if reg, err := currentUnits(); err == nil && reg.timeZone != "" {
			if loc, err := time.LoadLocation(reg.timeZone); err == nil {
				return loc, nil
			}
		}
	}
	return time.Local, nil
}

// liveFanoutRateLimit keeps an explicit --rate-limit, but turns the default
// auto pacer off for the bounded-concurrency per-property fan-outs. The auto
// pacer starts at 2 req/s, which made a caller-ID lookup across a handful of
// properties several times slower than the API allows; 429s are still retried
// by the client.
func liveFanoutRateLimit(flags *rootFlags) float64 {
	if flags == nil || flags.rateLimit == client.RateLimitAuto {
		return 0
	}
	return flags.rateLimit
}
