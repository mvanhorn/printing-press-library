// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var comparableReports = map[string]string{"speed-to-lead": "/reports/speed-to-lead", "funnel": "/reports/funnel", "appointments": "/reports/appointments", "call-performance": "/reports/call-performance", "attribution": "/reports/attribution", "sms-delivery": "/reports/sms-delivery"}

type compareWindow struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// compareView names the report and both windows so the answer stands on its own (an agent reading it later knows which
// report and which weeks it compares), with the metric rows under rows.
type compareView struct {
	Report        string                 `json:"report"`
	Current       compareWindow          `json:"current"`
	Prior         compareWindow          `json:"prior"`
	Rows          []compareRow           `json:"rows"`
	NonComparable []compareNonComparable `json:"non_comparable"`
	Partial       bool                   `json:"partial"`
	Checked       int                    `json:"checked"`
	Total         int                    `json:"total"`
	Failures      []string               `json:"failures,omitempty"`
}

type compareNonComparable struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type compareRow struct {
	Metric   string   `json:"metric"`
	Prior    float64  `json:"prior"`
	Current  float64  `json:"current"`
	Delta    float64  `json:"delta"`
	DeltaPct *float64 `json:"delta_pct,omitempty"`
}

func newNovelReportsCompareCmd(flags *rootFlags) *cobra.Command {
	var flagRange, flagFrom, flagTo, flagPipeline, flagAssignedTo string
	cmd := &cobra.Command{
		Use: "compare <report>", Short: "Compare any core report with its equal prior window.",
		Long:        "Use this command for the change in a report between two equal periods. One-sided indexed dimension members are listed as non-comparable without making the comparison partial; one-sided fixed metrics still fail closed. Do NOT use it for target attainment; use 'reports scorecard' instead.",
		Example:     "  conduyt-crm-pp-cli reports compare speed-to-lead --range 7d --json\n  conduyt-crm-pp-cli reports compare speed-to-lead --range 7d --assigned-to <userId> --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "<report>=speed-to-lead;--range=7d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reports compare")
			}
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("report is required; valid reports: %s", validReportNames()))
			}
			path, ok := comparableReports[args[0]]
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unknown report %q; valid reports: %s", args[0], validReportNames()))
			}
			if args[0] == "funnel" && flagPipeline == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--pipeline is required for funnel"))
			}
			from, to, err := reportWindow(flagRange, flagFrom, flagTo, time.Now().UTC())
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			duration := to.Sub(from)
			priorFrom, priorTo := from.Add(-duration), from
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			base := map[string]string{}
			if flagPipeline != "" {
				base["pipelineId"] = flagPipeline
			}
			if flagAssignedTo != "" {
				base["assignedTo"] = flagAssignedTo
			}
			fetch := func(start, end time.Time) (any, error) {
				params := map[string]string{"from": start.Format(time.RFC3339), "to": end.Format(time.RFC3339)}
				for k, v := range base {
					params[k] = v
				}
				raw, e := c.Get(ctx, path, params)
				if e != nil {
					return nil, e
				}
				if e = rejectResponseErrorEnvelope(raw); e != nil {
					return nil, e
				}
				dec := json.NewDecoder(bytes.NewReader(raw))
				dec.UseNumber()
				var doc any
				if e = dec.Decode(&doc); e != nil {
					return nil, e
				}
				if top, yes := doc.(map[string]any); yes {
					if data, exists := top["data"]; exists {
						doc = data
					}
				}
				return doc, nil
			}
			view := compareView{Report: args[0], Current: compareWindow{From: from.Format(time.RFC3339), To: to.Format(time.RFC3339)}, Prior: compareWindow{From: priorFrom.Format(time.RFC3339), To: priorTo.Format(time.RFC3339)}, Rows: []compareRow{}, NonComparable: []compareNonComparable{}, Total: 2}
			current, err := fetch(from, to)
			if err != nil {
				return outputCompareFailure(cmd, flags, view, fmt.Errorf("fetching current %s report: %w", args[0], err))
			}
			view.Checked++
			prior, err := fetch(priorFrom, priorTo)
			if err != nil {
				return outputCompareFailure(cmd, flags, view, fmt.Errorf("fetching prior %s report: %w", args[0], err))
			}
			view.Checked++
			rows, nonComparable := compareDocumentsDetailed(prior, current)
			view.Rows, view.NonComparable = rows, nonComparable
			if comparableMetricCount(current) == 0 && !compatibleRootArrayPair(prior, current) {
				return outputCompareFailure(cmd, flags, view, fmt.Errorf("current %s report contains no comparable numeric metrics", args[0]))
			}
			if comparableMetricCount(prior) == 0 && !compatibleRootArrayPair(prior, current) {
				return outputCompareFailure(cmd, flags, view, fmt.Errorf("prior %s report contains no comparable numeric metrics", args[0]))
			}
			for _, item := range nonComparable {
				if strings.HasPrefix(item.Reason, "dimension member present in ") {
					continue
				}
				view.Partial = true
				view.Failures = append(view.Failures, item.Path+": "+item.Reason)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
				if view.Partial {
					return apiErr(fmt.Errorf("report comparison is incomplete: %s", strings.Join(view.Failures, "; ")))
				}
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s..%s vs %s..%s\n", view.Report, view.Prior.From, view.Prior.To, view.Current.From, view.Current.To)
			if err := printComparison(cmd, rows); err != nil {
				return err
			}
			printNonComparable(cmd.OutOrStdout(), view.NonComparable)
			if view.Partial {
				return apiErr(fmt.Errorf("report comparison is incomplete: %s", strings.Join(view.Failures, "; ")))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagRange, "range", "7d", "Window length as Nd")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Current window start (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Current window end (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagPipeline, "pipeline", "", "Pipeline ID (required for funnel)")
	cmd.Flags().StringVar(&flagAssignedTo, "assigned-to", "", "Filter by assigned user ID")
	return cmd
}

func comparableMetricCount(doc any) int {
	collected := collectComparableDocument(doc)
	return len(collected.metrics)
}

func compatibleRootArrayPair(prior, current any) bool {
	_, priorOK := prior.([]any)
	_, currentOK := current.([]any)
	return priorOK && currentOK
}

func outputCompareFailure(cmd *cobra.Command, flags *rootFlags, view compareView, cause error) error {
	view.Partial = true
	view.Failures = []string{cause.Error()}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "WARNING: report comparison is incomplete: %v\n", cause)
	}
	return classifyAPIErrorOnly(cause)
}

func validReportNames() string {
	names := make([]string, 0, len(comparableReports))
	for n := range comparableReports {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
func compareDocuments(prior, current any) []compareRow {
	rows, _ := compareDocumentsDetailed(prior, current)
	return rows
}

func compareDocumentsDetailed(prior, current any) ([]compareRow, []compareNonComparable) {
	p, c := collectComparableDocument(prior), collectComparableDocument(current)
	dimensions := dimensionCollections(p, c)
	mismatchedContainers := map[comparePathKey]bool{}
	for container := range dimensions {
		priorKind, currentKind := valueKind(p.kinds, container), valueKind(c.kinds, container)
		if len(pathFor(container, p, c)) == 0 && priorKind == currentKind {
			continue
		}
		if priorKind == currentKind && compatibleDimensionContainer(priorKind) {
			continue
		}
		mismatchedContainers[container] = true
	}
	for container := range mismatchedContainers {
		path := pathFor(container, p, c)
		if len(path) != 0 && mismatchedContainers[comparePathKey("")] {
			continue
		}
		if len(path) != 0 && hasNestedMismatch(path, container, mismatchedContainers, p, c) {
			continue
		}
		label := collectionLabel(path)
		p.nonComparable[container] = fmt.Sprintf("dimension collection %s is %s in the prior window and %s in the current window", label, valueKind(p.kinds, container), valueKind(c.kinds, container))
	}
	keys := make([]comparePathKey, 0)
	for k := range p.metrics {
		path := p.paths[k]
		if _, ok := c.metrics[k]; ok {
			keys = append(keys, k)
		} else if withinReportedMismatchedContainer(path, p.nonComparable, p, c) {
			continue
		} else {
			container, found := dimensionContainer(path)
			if found && p.kinds[container.key()] == c.kinds[container.key()] && compatibleDimensionContainer(p.kinds[container.key()]) {
				p.nonComparable[k] = "dimension member present in prior window only"
			} else {
				p.nonComparable[k] = "numeric metric is present in prior window only"
			}
		}
	}
	for k := range c.metrics {
		if _, ok := p.metrics[k]; !ok {
			path := c.paths[k]
			if withinReportedMismatchedContainer(path, p.nonComparable, p, c) {
				continue
			}
			container, found := dimensionContainer(path)
			if found && p.kinds[container.key()] == c.kinds[container.key()] && compatibleDimensionContainer(p.kinds[container.key()]) {
				c.nonComparable[k] = "dimension member present in current window only"
			} else {
				c.nonComparable[k] = "numeric metric is present in current window only"
			}
		}
	}
	rows := make([]compareRow, 0, len(keys))
	for _, k := range keys {
		delta := c.metrics[k] - p.metrics[k]
		var pct *float64
		if p.metrics[k] != 0 {
			v := delta / p.metrics[k] * 100
			pct = &v
		}
		rows = append(rows, compareRow{formatComparePath(p.paths[k]), p.metrics[k], c.metrics[k], delta, pct})
	}
	sort.Slice(rows, func(i, j int) bool {
		ai, aj := -1.0, -1.0
		if rows[i].DeltaPct != nil {
			ai = math.Abs(*rows[i].DeltaPct)
		}
		if rows[j].DeltaPct != nil {
			aj = math.Abs(*rows[j].DeltaPct)
		}
		if ai == aj {
			return rows[i].Metric < rows[j].Metric
		}
		return ai > aj
	})
	nonComparable := make([]compareNonComparable, 0, len(p.nonComparable)+len(c.nonComparable))
	for key, reason := range p.nonComparable {
		nonComparable = append(nonComparable, compareNonComparable{Path: collectionLabel(pathFor(key, p, c)), Reason: reason})
	}
	for key, reason := range c.nonComparable {
		if _, exists := p.nonComparable[key]; !exists {
			nonComparable = append(nonComparable, compareNonComparable{Path: collectionLabel(pathFor(key, p, c)), Reason: reason})
		}
	}
	sort.Slice(nonComparable, func(i, j int) bool { return nonComparable[i].Path < nonComparable[j].Path })
	return rows, nonComparable
}

type comparePathSegment struct {
	property      string
	selectorKey   string
	selectorValue string
}

type comparePath []comparePathSegment
type comparePathKey string

type comparableDocument struct {
	metrics       map[comparePathKey]float64
	nonComparable map[comparePathKey]string
	kinds         map[comparePathKey]string
	paths         map[comparePathKey]comparePath
}

func (path comparePath) key() comparePathKey {
	var b strings.Builder
	for _, segment := range path {
		if segment.selectorKey == "" {
			fmt.Fprintf(&b, "p%d:%s", len(segment.property), segment.property)
		} else {
			fmt.Fprintf(&b, "s%d:%s%d:%s", len(segment.selectorKey), segment.selectorKey, len(segment.selectorValue), segment.selectorValue)
		}
	}
	return comparePathKey(b.String())
}

func appendProperty(path comparePath, property string) comparePath {
	return append(append(comparePath(nil), path...), comparePathSegment{property: property})
}

func appendSelector(path comparePath, key, value string) comparePath {
	return append(append(comparePath(nil), path...), comparePathSegment{selectorKey: key, selectorValue: value})
}

func dimensionContainer(path comparePath) (comparePath, bool) {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i].selectorKey != "" {
			return path[:i], true
		}
	}
	return nil, false
}

func collectionLabel(path comparePath) string {
	if len(path) == 0 {
		return "$"
	}
	return formatComparePath(path)
}

func compatibleDimensionContainer(kind string) bool { return kind == "array" || kind == "object" }

func dimensionCollections(prior, current comparableDocument) map[comparePathKey]bool {
	out := map[comparePathKey]bool{"": true}
	for _, doc := range []comparableDocument{prior, current} {
		for key := range doc.metrics {
			if container, found := dimensionContainer(doc.paths[key]); found {
				out[container.key()] = true
			}
		}
		for key, kind := range doc.kinds {
			if kind == "array" || kind == "empty object" {
				out[key] = true
			}
		}
	}
	return out
}

func valueKind(kinds map[comparePathKey]string, path comparePathKey) string {
	kind := kinds[path]
	if kind == "" {
		return "absent"
	}
	if kind == "empty object" {
		return "object"
	}
	return kind
}

func hasNestedMismatch(container comparePath, containerKey comparePathKey, mismatched map[comparePathKey]bool, prior, current comparableDocument) bool {
	for candidate := range mismatched {
		if candidate != containerKey && pathDescendsFrom(pathFor(candidate, prior, current), container) {
			return true
		}
	}
	return false
}

func withinReportedMismatchedContainer(path comparePath, reported map[comparePathKey]string, prior, current comparableDocument) bool {
	for key, reason := range reported {
		if !strings.HasPrefix(reason, "dimension collection ") {
			continue
		}
		container := pathFor(key, prior, current)
		if len(container) == 0 || pathDescendsFrom(path, container) {
			return true
		}
	}
	return false
}

func pathDescendsFrom(path, ancestor comparePath) bool {
	if len(path) < len(ancestor) {
		return false
	}
	for i := range ancestor {
		if path[i] != ancestor[i] {
			return false
		}
	}
	return true
}

func pathFor(key comparePathKey, documents ...comparableDocument) comparePath {
	for _, document := range documents {
		if path, ok := document.paths[key]; ok {
			return path
		}
	}
	return nil
}

var reportIdentityKeys = []string{"id", "userId", "user_id", "key", "stage", "stageId", "status", "source", "date", "bucket", "name"}

func collectComparableDocument(value any) comparableDocument {
	document := comparableDocument{map[comparePathKey]float64{}, map[comparePathKey]string{}, map[comparePathKey]string{}, map[comparePathKey]comparePath{}}
	collectComparableNumbers(nil, value, &document)
	return document
}

func collectComparableNumbers(path comparePath, value any, document *comparableDocument) {
	pathKey := path.key()
	document.paths[pathKey] = append(comparePath(nil), path...)
	switch v := value.(type) {
	case map[string]any:
		document.kinds[pathKey] = "object"
		if len(v) == 0 {
			document.kinds[pathKey] = "empty object"
		}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if ignoredNumericLeaf(k) {
				continue
			}
			collectComparableNumbers(appendProperty(path, k), v[k], document)
		}
	case []any:
		document.kinds[pathKey] = "array"
		if len(v) == 0 {
			return
		}
		identityKeys, identityValues := make([]string, len(v)), make([]string, len(v))
		seen := map[comparePathKey]bool{}
		for i, item := range v {
			object, ok := item.(map[string]any)
			if !ok {
				document.nonComparable[pathKey] = "array values cannot be correlated by a stable identity key"
				return
			}
			key, value, ok := reportIdentity(object)
			if !ok {
				document.nonComparable[pathKey] = "array objects do not all expose a stable identity key"
				return
			}
			identity := appendSelector(nil, key, value).key()
			if seen[identity] {
				document.nonComparable[pathKey] = "array object identity keys are not unique"
				return
			}
			seen[identity] = true
			identityKeys[i], identityValues[i] = key, value
		}
		for i, item := range v {
			collectComparableNumbers(appendSelector(path, identityKeys[i], identityValues[i]), item, document)
		}
	case json.Number:
		document.kinds[pathKey] = "scalar"
		if n, e := v.Float64(); e == nil {
			document.metrics[pathKey] = n
		}
	case float64:
		document.kinds[pathKey] = "scalar"
		document.metrics[pathKey] = v
	case float32:
		document.kinds[pathKey] = "scalar"
		document.metrics[pathKey] = float64(v)
	case int:
		document.kinds[pathKey] = "scalar"
		document.metrics[pathKey] = float64(v)
	case int64:
		document.kinds[pathKey] = "scalar"
		document.metrics[pathKey] = float64(v)
	case nil:
		document.kinds[pathKey] = "null"
	default:
		document.kinds[pathKey] = "scalar"
	}
}

func formatComparePath(path comparePath) string {
	var b strings.Builder
	for i, segment := range path {
		if segment.selectorKey != "" {
			fmt.Fprintf(&b, "[%s=%s]", escapeSelectorPart(segment.selectorKey), escapeSelectorPart(segment.selectorValue))
			continue
		}
		if i > 0 {
			b.WriteByte('.')
		}
		if segment.property == "" {
			b.WriteString(`[""]`)
			continue
		}
		b.WriteString(escapePropertyName(segment.property))
	}
	return b.String()
}

func escapePropertyName(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `.`, `\.`, `[`, `\[`, `]`, `\]`)
	return replacer.Replace(value)
}

func escapeSelectorPart(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`, `=`, `\=`)
	return replacer.Replace(value)
}

func reportIdentity(object map[string]any) (string, string, bool) {
	for _, key := range reportIdentityKeys {
		value, exists := object[key]
		if !exists {
			continue
		}
		switch v := value.(type) {
		case string:
			return key, v, true
		case json.Number:
			return key, v.String(), true
		case float64:
			return key, strconv.FormatFloat(v, 'g', -1, 64), true
		case float32:
			return key, strconv.FormatFloat(float64(v), 'g', -1, 32), true
		case int:
			return key, strconv.Itoa(v), true
		case int64:
			return key, strconv.FormatInt(v, 10), true
		}
	}
	return "", "", false
}
func ignoredNumericLeaf(name string) bool {
	n := strings.ToLower(name)
	return n == "id" || strings.HasSuffix(n, "_id") || strings.HasSuffix(n, "id") || strings.Contains(n, "timestamp") || strings.HasSuffix(n, "_at")
}
func printComparison(cmd *cobra.Command, rows []compareRow) error {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "METRIC\tPRIOR\tCURRENT\tDELTA\tDELTA %")
	for _, r := range rows {
		pct := "—"
		if r.DeltaPct != nil {
			pct = strconv.FormatFloat(*r.DeltaPct, 'f', 1, 64) + "%"
		}
		fmt.Fprintf(tw, "%s\t%.2f\t%.2f\t%.2f\t%s\n", r.Metric, r.Prior, r.Current, r.Delta, pct)
	}
	return tw.Flush()
}

// maxNonComparableLines caps the human-readable NON-COMPARABLE listing; the
// full list is always available under --json as non_comparable.
const maxNonComparableLines = 10

func printNonComparable(w io.Writer, items []compareNonComparable) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "NON-COMPARABLE: %d value(s) present in only one window (full list under --json non_comparable)\n", len(items))
	for i, item := range items {
		if i == maxNonComparableLines {
			fmt.Fprintf(w, "  ... and %d more\n", len(items)-maxNonComparableLines)
			break
		}
		fmt.Fprintf(w, "  %s: %s\n", item.Path, item.Reason)
	}
}
