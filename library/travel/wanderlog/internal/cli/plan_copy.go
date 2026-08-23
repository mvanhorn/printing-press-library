// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live

import (
	"context"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlog/internal/client"
	"github.com/spf13/cobra"
)

type planCopyOptions struct {
	sourceURL           string
	sourceKey           string
	targetKey           string
	destination         string
	title               string
	startDate           string
	endDate             string
	privacy             string
	mode                string
	clientSchemaVersion int
	apply               bool
	force               bool
}

type planCopyReport struct {
	Command        string   `json:"command"`
	SourceKey      string   `json:"source_key"`
	SourceURL      string   `json:"source_url,omitempty"`
	SourceTitle    string   `json:"source_title,omitempty"`
	TargetKey      string   `json:"target_key,omitempty"`
	CreatedTarget  string   `json:"created_target_key,omitempty"`
	Applied        bool     `json:"applied"`
	ApplyRequested bool     `json:"apply_requested"`
	DryRun         bool     `json:"dry_run"`
	Mode           string   `json:"mode,omitempty"`
	Title          string   `json:"title,omitempty"`
	GeoID          int      `json:"geo_id,omitempty"`
	StartDate      string   `json:"start_date,omitempty"`
	EndDate        string   `json:"end_date,omitempty"`
	Days           int      `json:"days,omitempty"`
	Sections       int      `json:"sections"`
	DaySections    int      `json:"day_sections"`
	Blocks         int      `json:"blocks"`
	PlaceBlocks    int      `json:"place_blocks"`
	NoteBlocks     int      `json:"note_blocks"`
	Dates          []string `json:"dates,omitempty"`
	Resources      []string `json:"resources,omitempty"`
	Operations     []string `json:"operations,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

type planTripEnvelope struct {
	Success   bool                       `json:"success"`
	TripPlan  map[string]any             `json:"tripPlan"`
	Resources map[string]json.RawMessage `json:"resources"`
	Error     any                        `json:"error,omitempty"`
}

var planKeyRe = regexp.MustCompile(`^[A-Za-z0-9_-]{8,}$`)

// errSourcePlanKeyRequired is the clone/fill/preview missing-identifier
// message. resolveEditablePlanKey rewrites it for edit-target commands.
var errSourcePlanKeyRequired = errors.New("--source-url or --source-key is required")

func runPlanPreview(cmd *cobra.Command, flags *rootFlags, opts planCopyOptions) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := planLiveClient(flags)
	if err != nil {
		return err
	}
	source, sourceKey, err := fetchSourcePlan(ctx, c, opts)
	if err != nil {
		return err
	}
	report := buildPlanReport("plan preview", opts, source, sourceKey, nil)
	report.DryRun = true
	report.Operations = []string{"read source plan only"}
	return printPlanReport(cmd, flags, report)
}

func runPlanClone(cmd *cobra.Command, flags *rootFlags, opts planCopyOptions) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := planLiveClient(flags)
	if err != nil {
		return err
	}
	source, sourceKey, err := fetchSourcePlan(ctx, c, opts)
	if err != nil {
		return err
	}
	report := buildPlanReport("plan clone", opts, source, sourceKey, nil)
	report.ApplyRequested = opts.apply
	report.DryRun = !opts.apply || flags.dryRun
	report.Operations = []string{"fetch source plan", "create target trip", "replace target itinerary via ShareDB"}
	if !opts.apply || flags.dryRun {
		if flags.dryRun {
			report.Warnings = append(report.Warnings, "global --dry-run set: no trip will be created")
		}
		return printPlanReport(cmd, flags, report)
	}
	if err := requireCookie(c); err != nil {
		return authErr(err)
	}
	createdKey, createdTitle, err := createCloneTarget(ctx, c, source, opts)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	report.CreatedTarget = createdKey
	report.TargetKey = createdKey
	if createdTitle != "" {
		report.Title = createdTitle
	}
	if err := fillTargetViaShareDB(ctx, c, source, createdKey, opts, false); err != nil {
		return apiErr(err)
	}
	report.Applied = true
	return printPlanReport(cmd, flags, report)
}

func runPlanFill(cmd *cobra.Command, flags *rootFlags, opts planCopyOptions) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	if strings.TrimSpace(opts.targetKey) == "" {
		return usageErr(errors.New("--target-key is required"))
	}
	c, err := planLiveClient(flags)
	if err != nil {
		return err
	}
	source, sourceKey, err := fetchSourcePlan(ctx, c, opts)
	if err != nil {
		return err
	}
	target, err := fetchTargetPlan(ctx, c, opts.targetKey, opts.clientSchemaVersion)
	if err != nil && opts.apply && !flags.dryRun {
		return classifyAPIError(err, flags)
	}
	report := buildPlanReport("plan fill", opts, source, sourceKey, target)
	report.TargetKey = opts.targetKey
	report.ApplyRequested = opts.apply
	report.DryRun = !opts.apply || flags.dryRun
	report.Mode = opts.mode
	report.Operations = []string{"fetch source plan", "fetch target trip", "replace target itinerary via ShareDB"}
	if target != nil && countBlocks(target) > 0 && !opts.force {
		report.Warnings = append(report.Warnings, "target contains blocks; apply requires --force with replace-sections mode")
		if opts.apply && !flags.dryRun {
			return usageErr(errors.New("target trip contains blocks; rerun with --force after reviewing plan fill --dry-run"))
		}
	}
	if !opts.apply || flags.dryRun {
		if flags.dryRun {
			// Match `plan clone`: replace-sections is the more destructive of
			// the two, so the report must say plainly that nothing was written.
			report.Warnings = append(report.Warnings, "global --dry-run set: no sections will be replaced on the target trip")
		}
		return printPlanReport(cmd, flags, report)
	}
	if err := requireCookie(c); err != nil {
		return authErr(err)
	}
	if err := fillTargetViaShareDB(ctx, c, source, opts.targetKey, opts, opts.force); err != nil {
		return apiErr(err)
	}
	report.Applied = true
	return printPlanReport(cmd, flags, report)
}

func planLiveClient(flags *rootFlags) (*client.Client, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	// These commands use --dry-run as a preview mode. Reads must still execute;
	// mutations are gated separately by --apply.
	c.DryRun = false
	return c, nil
}

func fetchSourcePlan(ctx context.Context, c *client.Client, opts planCopyOptions) (map[string]any, string, error) {
	key, err := resolvePlanKey(opts.sourceKey, opts.sourceURL)
	if err != nil {
		return nil, "", usageErr(err)
	}
	return fetchPlan(ctx, c, key, opts.clientSchemaVersion)
}

func fetchTargetPlan(ctx context.Context, c *client.Client, key string, version int) (map[string]any, error) {
	trip, _, err := fetchPlan(ctx, c, key, version)
	return trip, err
}

func fetchPlan(ctx context.Context, c *client.Client, key string, version int) (map[string]any, string, error) {
	path := "/api/tripPlans/{key}"
	path = replacePathParam(path, "key", key)
	data, err := c.GetNoCache(ctx, path, map[string]string{"clientSchemaVersion": fmt.Sprintf("%d", version)})
	if err != nil {
		return nil, key, classifyAPIError(err, nil)
	}
	var env planTripEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, key, fmt.Errorf("parse trip plan response: %w", err)
	}
	if !env.Success || env.TripPlan == nil {
		return nil, key, fmt.Errorf("trip plan %s was not returned successfully", key)
	}
	if len(env.Resources) > 0 {
		env.TripPlan["_resources"] = env.Resources
	}
	return env.TripPlan, key, nil
}

func resolvePlanKey(sourceKey, sourceURL string) (string, error) {
	key := strings.TrimSpace(sourceKey)
	if key != "" {
		return validateResolvedPlanKey(key)
	}
	if strings.TrimSpace(sourceURL) == "" {
		return "", errSourcePlanKeyRequired
	}
	u, err := url.Parse(sourceURL)
	if err != nil {
		return "", fmt.Errorf("invalid --source-url: %w", err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, p := range parts {
		if (p == "plan" || p == "view") && i+1 < len(parts) {
			if got, err := validateResolvedPlanKey(parts[i+1]); err == nil {
				return got, nil
			} else if isAllDigits(parts[i+1]) {
				return "", err
			}
		}
	}
	return "", fmt.Errorf("could not find Wanderlog plan key in %q", sourceURL)
}

// PATCH(amend-2026-08-23: reject tripPlan.id as a plan key)
func validateResolvedPlanKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if isAllDigits(key) {
		return "", fmt.Errorf("%s is tripPlan.id, not a key. Use the 16-char key from trips home (field: key). Example: --target-key naertjcoixqrgrfc", key)
	}
	if !planKeyRe.MatchString(key) {
		return "", fmt.Errorf("invalid --source-key %q", key)
	}
	return key, nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func buildPlanReport(command string, opts planCopyOptions, source map[string]any, sourceKey string, target map[string]any) planCopyReport {
	dates := dayDates(source)
	start, end := planDateRange(source, dates)
	geoID := sourceGeoID(source)
	report := planCopyReport{
		Command:     command,
		SourceKey:   sourceKey,
		SourceURL:   opts.sourceURL,
		SourceTitle: stringField(source, "title"),
		Title:       firstNonEmpty(opts.title, cloneTitle(source)),
		GeoID:       geoID,
		StartDate:   firstNonEmpty(opts.startDate, start),
		EndDate:     firstNonEmpty(opts.endDate, end),
		Days:        len(dates),
		Sections:    len(sections(source)),
		DaySections: len(dates),
		Blocks:      countBlocks(source),
		PlaceBlocks: countBlocksByType(source, "place"),
		NoteBlocks:  countBlocksByType(source, "note"),
		Dates:       dates,
		Resources:   resourceKeys(source),
	}
	if geoID == 0 && opts.destination == "" {
		report.Warnings = append(report.Warnings, "source plan has no geo id; plan clone may need --destination")
	}
	if report.Blocks == 0 {
		report.Warnings = append(report.Warnings, "source plan has no place/note blocks; clone will copy the section/date template only")
	}
	if target != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("target currently has %d sections and %d blocks", len(sections(target)), countBlocks(target)))
	}
	return report
}

func printPlanReport(cmd *cobra.Command, flags *rootFlags, report planCopyReport) error {
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}

func createCloneTarget(ctx context.Context, c *client.Client, source map[string]any, opts planCopyOptions) (string, string, error) {
	geoID := sourceGeoID(source)
	if geoID == 0 {
		return "", "", errors.New("source plan has no geo id; --destination fallback is not implemented for apply mode yet")
	}
	dates := dayDates(source)
	start, end := planDateRange(source, dates)
	body := map[string]any{
		"geoIds":              []int{geoID},
		"initialMapsPlaceIds": []string{},
		"initialEmailId":      nil,
		"type":                "plan",
		"startDate":           firstNonEmpty(opts.startDate, start),
		"endDate":             firstNonEmpty(opts.endDate, end),
		"privacy":             firstNonEmpty(opts.privacy, "private"),
		"isMapEmbed":          false,
		"title":               firstNonEmpty(opts.title, cloneTitle(source)),
		"language":            "en",
	}
	data, _, err := c.Post(ctx, "/api/tripPlans", body)
	if err != nil {
		return "", "", err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", "", err
	}
	trip := mapField(obj, "tripPlan")
	if trip == nil {
		trip = mapField(obj, "data")
	}
	key := stringField(trip, "key")
	if key == "" {
		key = stringField(obj, "key")
	}
	if key == "" {
		return "", "", fmt.Errorf("create trip response did not include key: %s", string(data))
	}
	return key, firstNonEmpty(stringField(trip, "title"), stringField(obj, "title")), nil
}

func fillTargetViaShareDB(ctx context.Context, c *client.Client, source map[string]any, targetKey string, opts planCopyOptions, force bool) error {
	auth := c.Config.AuthHeader()
	if auth == "" {
		return errors.New("WANDERLOG_COOKIE is required for ShareDB fill")
	}
	wsURL, err := websocketURL(c.RequestBaseURL(), targetKey, opts.clientSchemaVersion)
	if err != nil {
		return err
	}
	header := http.Header{}
	header.Set("Cookie", auth)
	header.Set("Origin", c.RequestBaseURL())
	header.Set("User-Agent", "Mozilla/5.0 (compatible; wanderlog-pp-cli/0.1; +https://wanderlog.com)")
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return fmt.Errorf("connect ShareDB: %w", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	if err := conn.WriteJSON(map[string]any{"a": "hs", "id": nil, "protocol": 1, "protocolMinor": 2}); err != nil {
		return err
	}
	var sessionID string
	for {
		var frame map[string]any
		if err := conn.ReadJSON(&frame); err != nil {
			return fmt.Errorf("ShareDB handshake: %w", err)
		}
		if frame["a"] == "init" {
			sessionID = stringAny(frame["id"])
			continue
		}
		if frame["a"] == "hs" {
			break
		}
	}
	if err := conn.WriteJSON(map[string]any{"a": "s", "c": "TripPlans", "d": targetKey}); err != nil {
		return err
	}
	var version int
	var target map[string]any
	for {
		var frame map[string]any
		if err := conn.ReadJSON(&frame); err != nil {
			return fmt.Errorf("ShareDB subscribe: %w", err)
		}
		if frame["a"] != "s" {
			continue
		}
		data := mapField(frame, "data")
		version = intAny(data["v"])
		target = mapField(data, "data")
		break
	}
	if target == nil || version == 0 {
		return errors.New("ShareDB subscribe did not return target snapshot/version")
	}
	if countBlocks(target) > 0 && !force {
		return errors.New("target trip contains blocks; rerun with --force after preview")
	}
	op := buildFillOps(source, target)
	frame := map[string]any{"a": "op", "c": "TripPlans", "d": targetKey, "v": version, "seq": 1, "x": map[string]any{}, "op": op}
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	if err := conn.WriteJSON(frame); err != nil {
		return err
	}
	for {
		var ack map[string]any
		if err := conn.ReadJSON(&ack); err != nil {
			return fmt.Errorf("ShareDB op ack: %w", err)
		}
		if code := intAny(ack["code"]); code != 0 {
			return fmt.Errorf("ShareDB rejected op (%d): %s", code, stringAny(ack["message"]))
		}
		if ack["a"] == "op" {
			if intAny(ack["seq"]) == 1 || stringAny(ack["src"]) == sessionID {
				return nil
			}
		}
	}
}

func buildFillOps(source, target map[string]any) []map[string]any {
	sanitized := cloneJSONMap(mapField(source, "itinerary"))
	if sanitized == nil {
		sanitized = map[string]any{}
	}
	sanitizeItinerary(sanitized)
	ops := []map[string]any{{"p": []any{"itinerary"}, "od": mapField(target, "itinerary"), "oi": sanitized}}
	dates := dayDates(source)
	start, end := planDateRange(source, dates)
	if start != "" && stringField(target, "startDate") != start {
		ops = append(ops, map[string]any{"p": []any{"startDate"}, "od": target["startDate"], "oi": start})
	}
	if end != "" && stringField(target, "endDate") != end {
		ops = append(ops, map[string]any{"p": []any{"endDate"}, "od": target["endDate"], "oi": end})
	}
	if len(dates) > 0 && intAny(target["days"]) != len(dates) {
		ops = append(ops, map[string]any{"p": []any{"days"}, "od": target["days"], "oi": len(dates)})
	}
	return ops
}

func websocketURL(base string, key string, version int) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	}
	u.Path = "/api/tripPlans/wsOverall/" + url.PathEscape(key)
	u.RawQuery = fmt.Sprintf("clientSchemaVersion=%d", version)
	return u.String(), nil
}

func requireCookie(c *client.Client) error {
	if c == nil || c.Config == nil || strings.TrimSpace(c.Config.AuthHeader()) == "" {
		return errors.New("WANDERLOG_COOKIE is required for this command")
	}
	return nil
}

func sanitizeItinerary(it map[string]any) {
	secs, _ := it["sections"].([]any)
	for _, raw := range secs {
		sec, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		sec["id"] = randomWanderlogID()
		blocks, _ := sec["blocks"].([]any)
		for _, braw := range blocks {
			b, ok := braw.(map[string]any)
			if !ok {
				continue
			}
			b["id"] = randomWanderlogID()
			if addedBy, ok := b["addedBy"].(map[string]any); ok {
				addedBy["type"] = "user"
				delete(addedBy, "userId")
			}
		}
	}
}

// randomWanderlogID mints a 9-digit Wanderlog block/section id. These ids
// address rows inside a shared ShareDB document, so they must not collide
// across concurrent clients; a time-seeded math/rand generator produced
// identical ids for calls made inside the same nanosecond tick. Draw from
// crypto/rand instead and fall back to the process clock only if the
// system entropy source fails.
func randomWanderlogID() int {
	const span = 900000000
	n, err := crand.Int(crand.Reader, big.NewInt(span))
	if err != nil {
		return int(time.Now().UnixNano()%span) + 100000000
	}
	return int(n.Int64()) + 100000000
}

func cloneJSONMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	b, _ := json.Marshal(in)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}

func sections(trip map[string]any) []any {
	it := mapField(trip, "itinerary")
	secs, _ := it["sections"].([]any)
	return secs
}

func dayDates(trip map[string]any) []string {
	var dates []string
	for _, raw := range sections(trip) {
		sec, ok := raw.(map[string]any)
		if !ok || stringField(sec, "mode") != "dayPlan" {
			continue
		}
		if d := stringField(sec, "date"); d != "" {
			dates = append(dates, d)
		}
	}
	sort.Strings(dates)
	return dates
}

func planDateRange(trip map[string]any, dates []string) (string, string) {
	start := stringField(trip, "startDate")
	end := stringField(trip, "endDate")
	if start == "" && len(dates) > 0 {
		start = dates[0]
	}
	if end == "" && len(dates) > 0 {
		end = dates[len(dates)-1]
	}
	return start, end
}

func countBlocks(trip map[string]any) int {
	total := 0
	for _, raw := range sections(trip) {
		sec, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		blocks, _ := sec["blocks"].([]any)
		total += len(blocks)
	}
	return total
}

func countBlocksByType(trip map[string]any, typ string) int {
	total := 0
	for _, raw := range sections(trip) {
		sec, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		blocks, _ := sec["blocks"].([]any)
		for _, braw := range blocks {
			b, ok := braw.(map[string]any)
			if ok && stringField(b, "type") == typ {
				total++
			}
		}
	}
	return total
}

func sourceGeoID(trip map[string]any) int {
	resources, _ := trip["_resources"].(map[string]json.RawMessage)
	if raw := resources["geos"]; len(raw) > 0 {
		var geos []map[string]any
		if json.Unmarshal(raw, &geos) == nil && len(geos) > 0 {
			return intAny(geos[0]["id"])
		}
	}
	if raw := resources["geo"]; len(raw) > 0 {
		var geo map[string]any
		if json.Unmarshal(raw, &geo) == nil {
			return intAny(geo["id"])
		}
	}
	return 0
}

func resourceKeys(trip map[string]any) []string {
	resources, _ := trip["_resources"].(map[string]json.RawMessage)
	keys := make([]string, 0, len(resources))
	for k := range resources {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func cloneTitle(trip map[string]any) string {
	title := stringField(trip, "title")
	if title == "" {
		return "Cloned Wanderlog trip"
	}
	return title + " (copy)"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func mapField(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	v, _ := m[key].(map[string]any)
	return v
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	return stringAny(m[key])
}

func stringAny(v any) string {
	s, _ := v.(string)
	return s
}

func intAny(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}
