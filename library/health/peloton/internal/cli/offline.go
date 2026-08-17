// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// This file is an intentional generated-tree extension. U3 retains a private,
// content-addressed provider-fact store; these commands expose factual offline
// inspection only. Keep predicates and output strictly non-prescriptive.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/store"
	"github.com/spf13/cobra"
)

type offlineClassFilters struct {
	instructor, category, classType, segmentRole, metric string
	duration, durationMin, durationMax, segmentCount     int
	targetMin, targetMax                                 float64
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.AddCommand(newOfflineCmd(flags))
	})
}

func newOfflineCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "offline", Short: "Inspect locally synced provider facts without network access.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newOfflineHistoryCmd(flags), newOfflineWorkoutCmd(flags), newOfflinePerformanceCmd(flags), newOfflineIntervalsCmd(flags), newOfflineClassesCmd(flags), newOfflineStrengthCmd(flags), newOfflineRepeatCmd(flags))
	return cmd
}

func newOfflineHistoryCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "history", Short: "List locally stored recorded workout facts.", RunE: func(cmd *cobra.Command, _ []string) error {
		facts, err := offlineFacts(cmd, "workouts", limit)
		if err != nil {
			return err
		}
		return printOffline(cmd, flags, map[string]any{"items": payloads(facts), "caveats": caveatIfEmpty(facts, "no recorded workout facts are stored")})
	}}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum facts to return; 0 returns all.")
	return cmd
}

func newOfflineWorkoutCmd(flags *rootFlags) *cobra.Command {
	return offlineIDCmd("workout <workout_id>", "Show a locally stored workout detail and its recorded history fact.", flags, func(cmd *cobra.Command, id string) (any, []string, error) {
		detail, err := offlineFact(cmd, "workout_details", id)
		if err != nil {
			return nil, nil, err
		}
		out := map[string]any{"detail": decodePayload(detail)}
		if history, e := offlineFact(cmd, "workouts", id); e == nil {
			out["history"] = decodePayload(history)
		} else {
			out["caveats"] = []string{"recorded history fact is unavailable"}
		}
		return out, nil, nil
	})
}

func newOfflinePerformanceCmd(flags *rootFlags) *cobra.Command {
	return offlineIDCmd("performance <workout_id>", "Show locally stored recorded performance samples and summary fields.", flags, func(cmd *cobra.Command, id string) (any, []string, error) {
		fact, err := offlineFact(cmd, "performance", id)
		if err != nil {
			if _, detailErr := offlineFact(cmd, "workout_details", id); detailErr == nil {
				return map[string]any{"workout_id": id, "samples": []any{}}, []string{"recorded performance graph is unavailable"}, nil
			}
			return nil, nil, err
		}
		return decodePayload(fact), nil, nil
	})
}

func newOfflineIntervalsCmd(flags *rootFlags) *cobra.Command {
	return offlineIDCmd("intervals <workout_id>", "Show the stored class segments associated with a recorded workout when available.", flags, func(cmd *cobra.Command, id string) (any, []string, error) {
		detail, err := offlineFact(cmd, "workout_details", id)
		if err != nil {
			return nil, nil, err
		}
		rideID := stringValue(decodePayload(detail), "ride_id", "rideId")
		if rideID == "" {
			return map[string]any{"workout_id": id, "segments": []any{}}, []string{"workout detail does not include a class identifier"}, nil
		}
		class, err := offlineFact(cmd, "classes", rideID)
		if err != nil {
			return map[string]any{"workout_id": id, "ride_id": rideID, "segments": []any{}}, []string{"stored class structure is unavailable"}, nil
		}
		obj := decodePayload(class)
		segments, ok := objectValue(obj, "segments", "intervals")
		if !ok {
			return map[string]any{"workout_id": id, "ride_id": rideID, "segments": []any{}}, []string{"stored class has no comparable segment list"}, nil
		}
		return map[string]any{"workout_id": id, "ride_id": rideID, "segments": segments}, nil, nil
	})
}

func newOfflineClassesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "classes", Short: "Search and inspect locally stored class facts.", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newOfflineClassSearchCmd(flags), offlineIDCmd("show <ride_id>", "Show one locally stored class fact.", flags, func(cmd *cobra.Command, id string) (any, []string, error) {
		f, e := offlineClass(cmd, id)
		if e != nil {
			return nil, nil, e
		}
		return decodePayload(f), nil, nil
	}), offlineIDCmd("structure <ride_id>", "Show ordered stored class segments and target fields.", flags, func(cmd *cobra.Command, id string) (any, []string, error) {
		f, e := offlineClass(cmd, id)
		if e != nil {
			return nil, nil, e
		}
		v := decodePayload(f)
		segments, ok := objectValue(v, "segments", "intervals")
		if !ok {
			return map[string]any{"ride_id": id, "segments": []any{}}, []string{"stored class has no comparable segment list"}, nil
		}
		return map[string]any{"ride_id": id, "segments": segments}, nil, nil
	}), newOfflineFiltersCmd(flags))
	return cmd
}

func newOfflineClassSearchCmd(flags *rootFlags) *cobra.Command {
	var f offlineClassFilters
	cmd := &cobra.Command{Use: "search", Short: "Search local class facts by factual stored fields and structural intersections.", RunE: func(cmd *cobra.Command, _ []string) error {
		facts, err := offlineClasses(cmd)
		if err != nil {
			return err
		}
		var matches []store.ProviderFact
		for _, fact := range facts {
			if classMatches(decodePayload(fact), f) {
				matches = append(matches, fact)
			}
		}
		caveats := []string{}
		if len(matches) == 0 {
			caveats = append(caveats, "no locally stored class facts match every requested predicate")
		}
		if f.segmentRole != "" || f.segmentCount != 0 || f.metric != "" || f.targetMin != 0 || f.targetMax != 0 {
			caveats = append(caveats, "structural predicates only compare fields retained in each stored class fact")
		}
		return printOffline(cmd, flags, map[string]any{"items": payloads(matches), "caveats": caveats})
	}}
	cmd.Flags().StringVar(&f.instructor, "instructor", "", "Stored instructor name or identifier.")
	cmd.Flags().IntVar(&f.duration, "duration", 0, "Exact stored duration in seconds.")
	cmd.Flags().IntVar(&f.durationMin, "duration-min", 0, "Minimum stored duration in seconds, inclusive.")
	cmd.Flags().IntVar(&f.durationMax, "duration-max", 0, "Maximum stored duration in seconds, inclusive.")
	cmd.Flags().StringVar(&f.category, "category", "", "Stored category or discipline.")
	cmd.Flags().StringVar(&f.classType, "type", "", "Stored class type.")
	cmd.Flags().StringVar(&f.segmentRole, "segment-role", "", "Stored segment role.")
	cmd.Flags().IntVar(&f.segmentCount, "segment-count", 0, "Exact stored segment count.")
	cmd.Flags().StringVar(&f.metric, "metric", "", "Stored metric or target metric.")
	cmd.Flags().Float64Var(&f.targetMin, "target-min", 0, "Inclusive minimum provider target value.")
	cmd.Flags().Float64Var(&f.targetMax, "target-max", 0, "Inclusive maximum provider target value.")
	return cmd
}

func newOfflineFiltersCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "filters", Short: "Show locally stored provider filter vocabulary.", RunE: func(cmd *cobra.Command, _ []string) error {
		fact, err := offlineFact(cmd, "filters", "v1")
		if err != nil {
			return err
		}
		return printOffline(cmd, flags, map[string]any{"filters": decodePayload(fact)})
	}}
}

func newOfflineStrengthCmd(flags *rootFlags) *cobra.Command {
	return offlineIDCmd("strength <workout_id>", "Show stored provider movement tracker fields without template fallback.", flags, func(cmd *cobra.Command, id string) (any, []string, error) {
		fact, err := offlineFact(cmd, "workout_details", id)
		if err != nil {
			return nil, nil, err
		}
		value := decodePayload(fact)
		movements, ok := objectValue(value, "movement_tracker_data", "movementTrackerData", "movements")
		if !ok {
			return map[string]any{"workout_id": id, "movements": []any{}}, []string{"stored workout detail has no movement tracker data"}, nil
		}
		return map[string]any{"workout_id": id, "movements": movements}, nil, nil
	})
}

func newOfflineRepeatCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "repeat <first_workout_id> <second_workout_id>", Short: "Compare two recorded workouts only when their stored class identifiers match.", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		first, err := offlineFact(cmd, "workout_details", args[0])
		if err != nil {
			return err
		}
		second, err := offlineFact(cmd, "workout_details", args[1])
		if err != nil {
			return err
		}
		firstRide, secondRide := stringValue(decodePayload(first), "ride_id", "rideId"), stringValue(decodePayload(second), "ride_id", "rideId")
		if firstRide == "" || secondRide == "" {
			return printOffline(cmd, flags, map[string]any{"same_class": false, "caveats": []string{"one or both workout details lack a comparable class identifier"}})
		}
		if firstRide != secondRide {
			return notFoundErr(fmt.Errorf("workouts %q and %q have different stored class identifiers", args[0], args[1]))
		}
		out := map[string]any{"same_class": true, "ride_id": firstRide, "workouts": []any{repeatFact(cmd, args[0]), repeatFact(cmd, args[1])}}
		return printOffline(cmd, flags, out)
	}}
}

func offlineIDCmd(use, short string, flags *rootFlags, run func(*cobra.Command, string) (any, []string, error)) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		value, caveats, err := run(cmd, args[0])
		if err != nil {
			return err
		}
		if len(caveats) > 0 {
			return printOffline(cmd, flags, map[string]any{"result": value, "caveats": caveats})
		}
		return printOffline(cmd, flags, value)
	}}
}

func offlineFacts(cmd *cobra.Command, family string, limit int) ([]store.ProviderFact, error) {
	db, err := openStoreForRead(cmd.Context(), "peloton-pp-cli")
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w", err)
	}
	if db == nil {
		return nil, fmt.Errorf("no local data. Run 'peloton-pp-cli sync' first")
	}
	defer db.Close()
	return db.ListProviderFacts(family, limit)
}
func offlineFact(cmd *cobra.Command, family, id string) (store.ProviderFact, error) {
	db, err := openStoreForRead(cmd.Context(), "peloton-pp-cli")
	if err != nil {
		return store.ProviderFact{}, fmt.Errorf("opening local database: %w", err)
	}
	if db == nil {
		return store.ProviderFact{}, fmt.Errorf("no local data. Run 'peloton-pp-cli sync' first")
	}
	defer db.Close()
	fact, err := db.GetProviderFact(family, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.ProviderFact{}, notFoundErr(fmt.Errorf("stored %s fact %q not found", family, id))
	}
	return fact, err
}
func offlineClass(cmd *cobra.Command, id string) (store.ProviderFact, error) {
	if f, e := offlineFact(cmd, "classes", id); e == nil {
		return f, nil
	}
	return offlineFact(cmd, "catalog_classes", id)
}
func offlineClasses(cmd *cobra.Command) ([]store.ProviderFact, error) {
	detailed, err := offlineFacts(cmd, "classes", 0)
	if err != nil {
		return nil, err
	}
	catalog, err := offlineFacts(cmd, "catalog_classes", 0)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	all := append([]store.ProviderFact{}, detailed...)
	for _, f := range detailed {
		seen[f.ProviderID] = true
	}
	for _, f := range catalog {
		if !seen[f.ProviderID] {
			all = append(all, f)
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].ProviderID < all[j].ProviderID })
	return all, nil
}
func printOffline(cmd *cobra.Command, flags *rootFlags, value any) error {
	out := map[string]any{"meta": map[string]any{"source": "local", "network": false}, "data": value}
	return printJSONFiltered(cmd.OutOrStdout(), out, flags)
}
func decodePayload(f store.ProviderFact) any {
	var value any
	dec := json.NewDecoder(strings.NewReader(string(f.Payload)))
	dec.UseNumber()
	if dec.Decode(&value) != nil {
		return map[string]any{"raw": string(f.Payload)}
	}
	return value
}
func payloads(facts []store.ProviderFact) []any {
	out := make([]any, 0, len(facts))
	for _, f := range facts {
		out = append(out, decodePayload(f))
	}
	return out
}
func caveatIfEmpty(facts []store.ProviderFact, caveat string) []string {
	if len(facts) == 0 {
		return []string{caveat}
	}
	return nil
}
func objectValue(value any, keys ...string) (any, bool) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	for _, key := range keys {
		if v, found := m[key]; found {
			return v, true
		}
	}
	return nil, false
}
func stringValue(value any, keys ...string) string {
	if v, ok := objectValue(value, keys...); ok {
		return fmt.Sprint(v)
	}
	return ""
}
func numberValue(value any, keys ...string) (float64, bool) {
	v, ok := objectValue(value, keys...)
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case json.Number:
		f, e := n.Float64()
		return f, e == nil
	case float64:
		return n, true
	case string:
		f, e := strconv.ParseFloat(n, 64)
		return f, e == nil
	default:
		return 0, false
	}
}
func containsValue(value any, needle string) bool {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return true
	}
	switch v := value.(type) {
	case map[string]any:
		for _, child := range v {
			if containsValue(child, needle) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if containsValue(child, needle) {
				return true
			}
		}
	case string:
		return strings.Contains(strings.ToLower(v), needle)
	}
	return false
}
func segments(value any) []any {
	if v, ok := objectValue(value, "segments", "intervals"); ok {
		if a, ok := v.([]any); ok {
			return a
		}
	}
	return nil
}
func classMatches(value any, f offlineClassFilters) bool {
	if f.instructor != "" && !containsNamedValue(value, []string{"instructor", "instructor_name", "instructor_id"}, f.instructor) {
		return false
	}
	if f.category != "" && !containsNamedValue(value, []string{"fitness_discipline", "category", "browse_category"}, f.category) {
		return false
	}
	if f.classType != "" && !containsNamedValue(value, []string{"class_type", "class_type_id", "type"}, f.classType) {
		return false
	}
	duration, hasDuration := numberValue(value, "duration", "duration_seconds", "length")
	if f.duration != 0 && (!hasDuration || duration != float64(f.duration)) {
		return false
	}
	if f.durationMin != 0 && (!hasDuration || duration < float64(f.durationMin)) {
		return false
	}
	if f.durationMax != 0 && (!hasDuration || duration > float64(f.durationMax)) {
		return false
	}
	ss := segments(value)
	if f.segmentCount != 0 && len(ss) != f.segmentCount {
		return false
	}
	if f.segmentRole != "" && !containsNamedValue(ss, []string{"role", "segment_role"}, f.segmentRole) {
		return false
	}
	if f.metric != "" && !containsNamedValue(value, []string{"metric", "metrics", "target_metric"}, f.metric) {
		return false
	}
	if f.targetMin != 0 || f.targetMax != 0 {
		found := false
		walkTargetNumbers(value, false, func(n float64) {
			if (f.targetMin == 0 || n >= f.targetMin) && (f.targetMax == 0 || n <= f.targetMax) {
				found = true
			}
		})
		if !found {
			return false
		}
	}
	return true
}
func walkNumbers(value any, visit func(float64)) {
	switch v := value.(type) {
	case map[string]any:
		for _, child := range v {
			walkNumbers(child, visit)
		}
	case []any:
		for _, child := range v {
			walkNumbers(child, visit)
		}
	case json.Number:
		if n, e := v.Float64(); e == nil {
			visit(n)
		}
	case float64:
		visit(v)
	}
}
func containsNamedValue(value any, names []string, needle string) bool {
	if needle == "" {
		return true
	}
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[normalKey(name)] = true
	}
	var walk func(any) bool
	walk = func(current any) bool {
		switch v := current.(type) {
		case map[string]any:
			for key, child := range v {
				if wanted[normalKey(key)] && containsValue(child, needle) {
					return true
				}
				if walk(child) {
					return true
				}
			}
		case []any:
			for _, child := range v {
				if walk(child) {
					return true
				}
			}
		}
		return false
	}
	return walk(value)
}
func walkTargetNumbers(value any, inTargetField bool, visit func(float64)) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			walkTargetNumbers(child, inTargetField || strings.Contains(normalKey(key), "target"), visit)
		}
	case []any:
		for _, child := range v {
			walkTargetNumbers(child, inTargetField, visit)
		}
	case json.Number:
		if inTargetField {
			if n, err := v.Float64(); err == nil {
				visit(n)
			}
		}
	case float64:
		if inTargetField {
			visit(v)
		}
	}
}
func normalKey(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(key), "_", ""), "-", "")
}
func repeatFact(cmd *cobra.Command, id string) any {
	out := map[string]any{"workout_id": id}
	if f, e := offlineFact(cmd, "workouts", id); e == nil {
		v := decodePayload(f)
		out["recorded_at"] = stringValue(v, "created_at", "start_time", "startTime", "date")
	}
	if out["recorded_at"] == "" {
		out["caveat"] = "recorded date is unavailable"
	}
	return out
}
