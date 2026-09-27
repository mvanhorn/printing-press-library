package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/planner"
	"github.com/spf13/cobra"
)

func availabilityOutputFixture() planner.Result {
	return planner.Result{
		"venues": []map[string]any{{"id": "venue-a-id", "slug": "venue-a", "name_ja": "会場A"}, {"id": nil, "slug": "venue-b", "name_ja": nil}},
		"checks": []map[string]any{
			{"venue_id": "venue-a-id", "slug": "venue-a", "date": "2026-09-30", "party": 2, "status": "available", "scope": "venue", "available_times": []string{"17:30"}, "error": nil},
			{"venue_id": "venue-a-id", "slug": "venue-a", "date": "2026-10-01", "party": 2, "status": "unknown", "scope": "venue", "available_times": []string{}, "error": "date missing from returned calendar"},
			{"venue_id": nil, "slug": "venue-b", "date": "2026-09-30", "party": 2, "status": "failed", "scope": "venue", "available_times": []string{}, "error": "upstream HTTP 503"},
			{"venue_id": nil, "slug": "venue-b", "date": "2026-10-01", "party": 2, "status": "failed", "scope": "venue", "available_times": []string{}, "error": "upstream HTTP 503"},
		},
		"fetch_failures": []map[string]any{{"slug": "venue-b", "error": "upstream HTTP 503"}},
		"window":         map[string]any{"from": "2026-09-30", "to": "2026-10-01", "days": 2, "venues": 2},
		"meta":           map[string]any{"source": "live", "requests": 3, "transport": "http", "optional": nil, "empty": map[string]any{}, "labels": []string{"cold", "a,b", "quoted\"name"}, "freshness": map[string]any{"observed_at": "2026-09-27T14:00:00Z", "served_at": "2026-09-27T14:00:01Z"}},
	}
}

func renderAvailabilityOutput(t *testing.T, flags rootFlags) (string, error) {
	t.Helper()
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := planningPrint(cmd, &flags, availabilityOutputFixture())
	return out.String(), err
}

func TestPlanningAvailabilityNativeFormatsRenderEveryCheckRow(t *testing.T) {
	for _, tc := range []struct {
		name      string
		flags     rootFlags
		separator rune
	}{
		{"csv", rootFlags{csv: true}, ','},
		{"plain", rootFlags{plain: true}, '\t'},
		{"selected csv", rootFlags{csv: true, selectFields: "checks.slug,checks.date,checks.status"}, ','},
		{"selected plain", rootFlags{plain: true, selectFields: "checks.slug,checks.date,checks.status"}, '\t'},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := renderAvailabilityOutput(t, tc.flags)
			if err != nil {
				t.Fatal(err)
			}
			reader := csv.NewReader(strings.NewReader(out))
			reader.Comma = tc.separator
			reader.LazyQuotes = tc.separator == '\t'
			rows, err := reader.ReadAll()
			if err != nil || len(rows) != 5 {
				t.Fatalf("expected header plus four check rows, including failures: rows=%v err=%v output=%q", rows, err, out)
			}
			columns := map[string]int{}
			for i, name := range rows[0] {
				columns[name] = i
			}
			for _, name := range []string{"slug", "date", "status"} {
				if _, ok := columns[name]; !ok {
					t.Fatalf("missing check column %s: %v", name, rows[0])
				}
			}
			for _, name := range []string{"venues", "checks", "fetch_failures", "meta", "window"} {
				if _, ok := columns[name]; ok {
					t.Fatalf("outer envelope rendered as a row: %v", rows[0])
				}
			}
			if tc.flags.selectFields != "" && len(rows[0]) != 3 {
				t.Fatalf("selection included extra columns: %v", rows[0])
			}
			want := [][]string{{"venue-a", "2026-09-30", "available"}, {"venue-a", "2026-10-01", "unknown"}, {"venue-b", "2026-09-30", "failed"}, {"venue-b", "2026-10-01", "failed"}}
			for i, row := range rows[1:] {
				got := []string{row[columns["slug"]], row[columns["date"]], row[columns["status"]]}
				if !reflect.DeepEqual(got, want[i]) {
					t.Fatalf("check row lost or changed: got=%v want=%v", got, want[i])
				}
			}
		})
	}
}

func TestPlanningAvailabilityQuietReturnsSlugPerCheckIncludingFailures(t *testing.T) {
	for _, selection := range []string{"", "checks.slug", "checks.slug,checks.date,checks.status", "checks", "checks.*", "CHECKS.SLUG"} {
		t.Run(selection, func(t *testing.T) {
			out, err := renderAvailabilityOutput(t, rootFlags{quiet: true, selectFields: selection})
			if err != nil {
				t.Fatal(err)
			}
			if want := "venue-a\nvenue-a\nvenue-b\nvenue-b\n"; out != want {
				t.Fatalf("quiet scan must retain every date/failure identity: got=%q want=%q", out, want)
			}
		})
	}
}

func TestPlanningAvailabilityQuietRejectsNonIdentitySelectionBeforeOutput(t *testing.T) {
	for _, selection := range []string{"checks.date", "checks.date,checks.status", "checks.venue_id"} {
		out, err := renderAvailabilityOutput(t, rootFlags{quiet: true, selectFields: selection})
		if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "checks.slug") || out != "" {
			t.Fatalf("nonidentity quiet selection must fail deterministically before output: selection=%q err=%v output=%q", selection, err, out)
		}
	}
}

func TestPlanningAvailabilityDefaultAndAgentJSONRetainCompleteScanEnvelope(t *testing.T) {
	for _, agent := range []bool{false, true} {
		out, err := renderAvailabilityOutput(t, rootFlags{agent: agent, asJSON: agent, compact: agent})
		if err != nil {
			t.Fatal(err)
		}
		var got, want map[string]any
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(availabilityOutputFixture())
		_ = json.Unmarshal(encoded, &want)
		if !reflect.DeepEqual(got, want) || strings.Count(strings.TrimSpace(out), "\n") != 0 {
			t.Fatalf("native projection leaked into complete JSON schema/freshness: %s", out)
		}
	}
}

func TestPlanningAvailabilityNativeWriterFailuresStillPropagate(t *testing.T) {
	for _, flags := range []rootFlags{{csv: true}, {plain: true}, {quiet: true}} {
		failure := errors.New("availability output writer failed")
		cmd := &cobra.Command{}
		cmd.SetOut(publicationFailingWriter{failure})
		if err := planningPrint(cmd, &flags, availabilityOutputFixture()); !errors.Is(err, failure) {
			t.Fatalf("native check rendering swallowed writer error: %v", err)
		}
	}
}

func TestPlanningAvailabilityNativeSelectionRetainsEnvelopeContext(t *testing.T) {
	for _, mode := range []string{"csv", "plain"} {
		for _, tc := range []struct {
			name, selection string
			rows            int
			columns         []string
		}{
			{"mixed count", "checks.slug,meta.requests", 4, []string{"slug", "meta.requests"}},
			{"mixed date", "checks.date,window.from", 4, []string{"date", "window.from"}},
			{"metadata only", "meta.requests", 1, []string{"meta.requests"}},
			{"context values", "checks.slug,meta.labels,meta.optional,meta.empty,fetch_failures", 4, []string{"slug", "meta.labels", "meta.optional", "meta.empty", "fetch_failures"}},
			{"context values only", "meta.labels,meta.optional,meta.empty,fetch_failures", 1, []string{"meta.labels", "meta.optional", "meta.empty", "fetch_failures"}},
		} {
			t.Run(mode+" "+tc.name, func(t *testing.T) {
				flags := rootFlags{csv: mode == "csv", plain: mode == "plain", selectFields: tc.selection}
				out, err := renderAvailabilityOutput(t, flags)
				if err != nil {
					t.Fatal(err)
				}
				reader := csv.NewReader(strings.NewReader(out))
				if mode == "plain" {
					reader.Comma = '\t'
					reader.LazyQuotes = true
				}
				rows, err := reader.ReadAll()
				if err != nil || len(rows) != tc.rows+1 {
					t.Fatalf("selected context row count wrong: rows=%v err=%v output=%q", rows, err, out)
				}
				columns := map[string]int{}
				for i, name := range rows[0] {
					columns[name] = i
				}
				if len(columns) != len(tc.columns) {
					t.Fatalf("selected columns dropped or invented: got=%v want=%v", rows[0], tc.columns)
				}
				for _, name := range tc.columns {
					if _, ok := columns[name]; !ok {
						t.Fatalf("selected context column %s missing: %v", name, rows[0])
					}
				}
				for i, row := range rows[1:] {
					if col, ok := columns["meta.requests"]; ok && row[col] != "3" {
						t.Errorf("count context lost on row%d: %v", i, row)
					}
					if col, ok := columns["window.from"]; ok && row[col] != "2026-09-30" {
						t.Errorf("window context lost on row%d: %v", i, row)
					}
					if col, ok := columns["slug"]; ok {
						want := []string{"venue-a", "venue-a", "venue-b", "venue-b"}
						if row[col] != want[i] {
							t.Errorf("partial check identity changed: %v", row)
						}
					}
					if col, ok := columns["date"]; ok {
						want := []string{"2026-09-30", "2026-10-01", "2026-09-30", "2026-10-01"}
						if row[col] != want[i] {
							t.Errorf("check date changed: %v", row)
						}
					}
					for _, name := range []string{"meta.labels", "fetch_failures"} {
						if col, ok := columns[name]; ok {
							var value any
							if err := json.Unmarshal([]byte(row[col]), &value); err != nil {
								t.Errorf("context array not retained as JSON cell: %s=%q err=%v", name, row[col], err)
							}
							source := availabilityOutputFixture()
							var expected any
							if name == "meta.labels" {
								expected = source["meta"].(map[string]any)["labels"]
							} else {
								expected = source[name]
							}
							raw, _ := json.Marshal(expected)
							var want any
							_ = json.Unmarshal(raw, &want)
							if !reflect.DeepEqual(value, want) {
								t.Errorf("context array lost selected values: %s=%v want%v", name, value, want)
							}
						}
					}
					if col, ok := columns["meta.optional"]; ok && row[col] != "null" {
						t.Errorf("explicit null lost: %v", row)
					}
					if col, ok := columns["meta.empty"]; ok && row[col] != "{}" {
						t.Errorf("empty object lost: %v", row)
					}
				}
			})
		}
	}
}

func TestPlanningAvailabilitySelectedContextProtectsCheckColumnCollisions(t *testing.T) {
	for _, mode := range []string{"csv", "plain"} {
		result := availabilityOutputFixture()
		checks := result["checks"].([]map[string]any)
		checks[0]["meta.requests"] = "row-local"
		cmd := &cobra.Command{}
		var out bytes.Buffer
		cmd.SetOut(&out)
		flags := rootFlags{csv: mode == "csv", plain: mode == "plain", selectFields: "checks.*,meta.requests"}
		if err := planningPrint(cmd, &flags, result); err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(strings.NewReader(out.String()))
		if mode == "plain" {
			reader.Comma = '\t'
			reader.LazyQuotes = true
		}
		rows, err := reader.ReadAll()
		if err != nil || len(rows) != 5 {
			t.Fatalf("collision output invalid: %v %v", rows, err)
		}
		columns := map[string]int{}
		for i, name := range rows[0] {
			columns[name] = i
		}
		rowColumn, rowOK := columns["meta.requests"]
		contextColumn, contextOK := columns["envelope.meta.requests"]
		if !rowOK || !contextOK || rows[1][rowColumn] != "row-local" {
			t.Fatalf("context overwrote check value: %v", rows)
		}
		for _, row := range rows[1:] {
			if row[contextColumn] != "3" {
				t.Fatalf("context collision dropped selected value: %v", row)
			}
		}
	}
}

func TestPlanningAvailabilityUnknownNativeSelectionStillErrorsBeforeOutput(t *testing.T) {
	for _, flags := range []rootFlags{{csv: true, selectFields: "meta.no_such_field"}, {plain: true, selectFields: "window.no_such_field"}} {
		out, err := renderAvailabilityOutput(t, flags)
		if err == nil || ExitCode(err) != 2 || out != "" {
			t.Fatalf("unknown native selection silently succeeded: err=%v output=%q", err, out)
		}
	}
}

func planningDetailOutputFixture(primary string) planner.Result {
	result := planner.Result{"meta": map[string]any{"source": "live", "requests": 0, "transport": "local-cache"}}
	switch primary {
	case "items":
		result["items"] = []map[string]any{{"id": "item-a", "price": "19800.0"}, {"id": "item-b", "price": nil}}
		result["venue"] = map[string]any{"id": "context-venue"}
	case "course":
		result["course"] = map[string]any{"id": "course-id", "name": "Example course", "price": "19800.0"}
		result["venue"] = map[string]any{"id": "venue-id", "slug": "venue-a"}
	case "venue":
		result["venue"] = map[string]any{"id": "venue-id", "slug": "venue-a", "name": "Example venue"}
	}
	return result
}

func TestPlanningPrimarySelectionsRetainContextAcrossListsAndDetails(t *testing.T) {
	for _, mode := range []string{"csv", "plain"} {
		for _, primary := range []string{"items", "course", "venue"} {
			for _, contextOnly := range []bool{false, true} {
				selection := primary + ".id,meta.requests"
				if contextOnly {
					selection = "meta.requests"
				}
				cmd := &cobra.Command{}
				var out bytes.Buffer
				cmd.SetOut(&out)
				flags := rootFlags{csv: mode == "csv", plain: mode == "plain", selectFields: selection}
				if err := planningPrint(cmd, &flags, planningDetailOutputFixture(primary)); err != nil {
					t.Fatal(err)
				}
				reader := csv.NewReader(strings.NewReader(out.String()))
				if mode == "plain" {
					reader.Comma = '\t'
					reader.LazyQuotes = true
				}
				rows, err := reader.ReadAll()
				wantRows := 1
				if primary == "items" && !contextOnly {
					wantRows = 2
				}
				if err != nil || len(rows) != wantRows+1 {
					t.Fatalf("%s %s summary/list selection wrong: rows%v err%v", mode, selection, rows, err)
				}
				columns := map[string]int{}
				for i, key := range rows[0] {
					columns[key] = i
				}
				countColumn, ok := columns["meta.requests"]
				if !ok {
					t.Fatalf("%s %s lost selected metadata: %v", mode, selection, rows)
				}
				for _, row := range rows[1:] {
					if row[countColumn] != "0" {
						t.Fatalf("cached request count lost: %v", rows)
					}
				}
				if contextOnly {
					if len(columns) != 1 {
						t.Fatalf("metadata-only selection invented primary fields: %v", rows)
					}
				} else {
					idColumn, ok := columns["id"]
					if !ok || len(columns) != 2 {
						t.Fatalf("primary identity selection changed: %v", rows)
					}
					wantID := primary + "-id"
					if primary == "items" {
						if rows[1][idColumn] != "item-a" || rows[2][idColumn] != "item-b" {
							t.Fatalf("list rows changed: %v", rows)
						}
					} else if rows[1][idColumn] != wantID {
						t.Fatalf("primary priority chose wrong identity: %v", rows)
					}
				}
			}
		}
	}
}

func TestPlanningDetailQuietReturnsSelectedPrimaryID(t *testing.T) {
	for _, primary := range []string{"course", "venue"} {
		for _, selection := range []string{"", primary + ".id", primary, primary + ".*", primary + ".id,meta.requests"} {
			cmd := &cobra.Command{}
			var out bytes.Buffer
			cmd.SetOut(&out)
			flags := rootFlags{quiet: true, selectFields: selection}
			if err := planningPrint(cmd, &flags, planningDetailOutputFixture(primary)); err != nil {
				t.Fatal(err)
			}
			if out.String() != primary+"-id\n" {
				t.Fatalf("%s quiet selected wrong identity or envelope: %q", primary, out.String())
			}
		}
		for _, selection := range []string{"meta.requests", primary + ".name", primary + ".name,meta.requests"} {
			cmd := &cobra.Command{}
			var out bytes.Buffer
			cmd.SetOut(&out)
			flags := rootFlags{quiet: true, selectFields: selection}
			err := planningPrint(cmd, &flags, planningDetailOutputFixture(primary))
			if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), primary+".id") || out.Len() != 0 {
				t.Fatalf("detail quiet nonidentity selection not rejected: selection%q err%v out%q", selection, err, out.String())
			}
		}
	}
}

func TestPlanningSelectedNativeContextWriterErrorsStillPropagate(t *testing.T) {
	for _, flags := range []rootFlags{{csv: true, selectFields: "checks.slug,meta.requests"}, {plain: true, selectFields: "meta.requests"}} {
		failure := errors.New("selected native writer failed")
		cmd := &cobra.Command{}
		cmd.SetOut(publicationFailingWriter{failure})
		if err := planningPrint(cmd, &flags, availabilityOutputFixture()); !errors.Is(err, failure) {
			t.Fatalf("selected table swallowed writer error: %v", err)
		}
	}
}

func TestPlanningQuietPrecedesCombinedNativeFormatFlags(t *testing.T) {
	for _, format := range []rootFlags{{csv: true}, {plain: true}, {csv: true, plain: true}} {
		format.quiet = true
		format.selectFields = "checks.slug,meta.requests"
		out, err := renderAvailabilityOutput(t, format)
		if err != nil || out != "venue-a\nvenue-a\nvenue-b\nvenue-b\n" {
			t.Fatalf("combined flags bypassed quiet precedence: output%q err%v", out, err)
		}
		format.selectFields = "meta.requests"
		out, err = renderAvailabilityOutput(t, format)
		if err == nil || ExitCode(err) != 2 || out != "" {
			t.Fatalf("combined flags bypassed quiet identity validation: output%q err%v", out, err)
		}
		for _, primary := range []string{"course", "venue"} {
			cmd := &cobra.Command{}
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			err = planningPrint(cmd, &format, planningDetailOutputFixture(primary))
			if err == nil || ExitCode(err) != 2 || buf.Len() != 0 {
				t.Fatalf("combined flags bypassed detail identity rule: primary%s output%q err%v", primary, buf.String(), err)
			}
		}
	}
}

func TestPlanningSelectedEmptyPrimaryArrayRetainsExplicitCell(t *testing.T) {
	for _, mode := range []string{"csv", "plain"} {
		for _, primary := range []string{"checks", "items"} {
			result := planner.Result{primary: []map[string]any{}, "meta": map[string]any{"requests": 0}}
			cmd := &cobra.Command{}
			var out bytes.Buffer
			cmd.SetOut(&out)
			flags := rootFlags{csv: mode == "csv", plain: mode == "plain", selectFields: primary + ",meta.requests"}
			if err := planningPrint(cmd, &flags, result); err != nil {
				t.Fatal(err)
			}
			reader := csv.NewReader(strings.NewReader(out.String()))
			if mode == "plain" {
				reader.Comma = '\t'
				reader.LazyQuotes = true
			}
			rows, err := reader.ReadAll()
			if err != nil || len(rows) != 2 || len(rows[0]) != 2 {
				t.Fatalf("empty primary selection lost summary: mode%s primary%s rows%v err%v", mode, primary, rows, err)
			}
			columns := map[string]int{}
			for i, name := range rows[0] {
				columns[name] = i
			}
			emptyColumn, hasEmpty := columns[primary]
			countColumn, hasCount := columns["meta.requests"]
			if !hasEmpty || !hasCount || rows[1][emptyColumn] != "[]" || rows[1][countColumn] != "0" {
				t.Fatalf("explicit empty array/context cell lost: %v", rows)
			}
		}
	}
}

func TestPlanningDetailQuietIgnoresNestedRuleIdentities(t *testing.T) {
	for _, primary := range []string{"venue", "course"} {
		for _, flags := range []rootFlags{{quiet: true}, {quiet: true, csv: true}, {quiet: true, plain: true}, {quiet: true, csv: true, plain: true}} {
			result := planningDetailOutputFixture(primary)
			detail := result[primary].(map[string]any)
			detail["rules"] = []map[string]any{{"id": "nested-rule-id", "description": "Synthetic source condition"}}
			cmd := &cobra.Command{}
			var out bytes.Buffer
			cmd.SetOut(&out)
			if err := planningPrint(cmd, &flags, result); err != nil {
				t.Fatal(err)
			}
			if want := primary + "-id\n"; out.String() != want {
				t.Fatalf("quiet emitted nested rule identity instead of primary detail: primary%s flags%+v got%q want%q", primary, flags, out.String(), want)
			}
		}
	}
}

func TestPlanningTableCellsPreserveDefaultAndSelectedStructuredValues(t *testing.T) {
	for _, mode := range []string{"csv", "plain"} {
		for _, primary := range []string{"course", "venue", "items", "checks"} {
			for _, selected := range []bool{false, true} {
				result := planningDetailOutputFixture(primary)
				if primary == "checks" {
					result = availabilityOutputFixture()
				}
				var record map[string]any
				if primary == "checks" || primary == "items" {
					record = result[primary].([]map[string]any)[0]
				} else {
					record = result[primary].(map[string]any)
				}
				arrayKey := map[string]string{"course": "valid_date_ranges", "venue": "service_categories", "items": "nested_data", "checks": "slots"}[primary]
				record[arrayKey] = []map[string]any{{"id": "nested-id", "label": "value,quoted\"", "amount": json.Number("9007199254740993")}}
				record["conditions"] = map[string]any{"tax": "included", "optional": nil}
				record["empty_object"] = map[string]any{}
				record["optional"] = nil
				flags := rootFlags{csv: mode == "csv", plain: mode == "plain"}
				if selected {
					flags.selectFields = strings.Join([]string{primary + "." + arrayKey, primary + ".conditions", primary + ".empty_object", primary + ".optional"}, ",")
				}
				cmd := &cobra.Command{}
				var out bytes.Buffer
				cmd.SetOut(&out)
				if err := planningPrint(cmd, &flags, result); err != nil {
					t.Fatal(err)
				}
				reader := csv.NewReader(strings.NewReader(out.String()))
				if mode == "plain" {
					reader.Comma = '\t'
					reader.LazyQuotes = true
				}
				rows, err := reader.ReadAll()
				if err != nil || len(rows) < 2 {
					t.Fatalf("structured table invalid: mode%s primary%s selected%v rows%v err%v", mode, primary, selected, rows, err)
				}
				columns := map[string]int{}
				for i, key := range rows[0] {
					columns[key] = i
				}
				for _, key := range []string{arrayKey, "conditions", "empty_object", "optional"} {
					col, ok := columns[key]
					if !ok {
						t.Fatalf("structured column lost: %s headers%v", key, rows[0])
					}
					var got, want any
					decoder := json.NewDecoder(strings.NewReader(rows[1][col]))
					decoder.UseNumber()
					if err := decoder.Decode(&got); err != nil {
						t.Fatalf("cell is Go notation instead of JSON: mode%s primary%s %s=%q err%v", mode, primary, key, rows[1][col], err)
					}
					raw, _ := json.Marshal(record[key])
					decoder = json.NewDecoder(bytes.NewReader(raw))
					decoder.UseNumber()
					_ = decoder.Decode(&want)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("structured cell changed values: mode%s primary%s key%s got%v want%v", mode, primary, key, got, want)
					}
				}
				if rows[1][columns["optional"]] != "null" || rows[1][columns["empty_object"]] != "{}" {
					t.Fatalf("null/empty object cells ambiguous: %v", rows[1])
				}
				if primary == "items" || primary == "checks" {
					records := result[primary].([]map[string]any)
					if len(rows) != len(records)+1 {
						t.Fatalf("heterogeneous rows lost: mode%s primary%s selected%v got%d want%d", mode, primary, selected, len(rows)-1, len(records))
					}
					for i, source := range records[1:] {
						for _, key := range []string{arrayKey, "conditions", "empty_object", "optional"} {
							if _, present := source[key]; present {
								continue
							}
							cell := rows[i+2][columns[key]]
							var value any
							if err := json.Unmarshal([]byte(cell), &value); err != nil || value != nil || cell != "null" {
								t.Fatalf("missing heterogeneous cell must be JSON null: mode%s primary%s selected%v row%d key%s cell%q value%v err%v", mode, primary, selected, i+1, key, cell, value, err)
							}
						}
					}
				}
			}
		}
	}
}

func TestPlanningNoPrimarySummaryCellsAndEmptyDefaults(t *testing.T) {
	for _, mode := range []string{"csv", "plain"} {
		result := planner.Result{"dry_run": true, "action": "availability check", "request_plan": map[string]any{"venues": []string{"venue-a", "venue-b"}, "optional": nil}, "meta": map[string]any{"requests": 0}}
		for _, selection := range []string{"", "request_plan.venues,request_plan.optional,meta.requests"} {
			cmd := &cobra.Command{}
			var out bytes.Buffer
			cmd.SetOut(&out)
			flags := rootFlags{csv: mode == "csv", plain: mode == "plain", selectFields: selection}
			if err := planningPrint(cmd, &flags, result); err != nil {
				t.Fatal(err)
			}
			reader := csv.NewReader(strings.NewReader(out.String()))
			if mode == "plain" {
				reader.Comma = '\t'
				reader.LazyQuotes = true
			}
			rows, err := reader.ReadAll()
			if err != nil || len(rows) != 2 {
				t.Fatalf("summary must be one row: %v %v", rows, err)
			}
			columns := map[string]int{}
			for i, key := range rows[0] {
				columns[key] = i
			}
			for _, key := range []string{"request_plan.venues", "request_plan.optional", "meta.requests"} {
				if _, ok := columns[key]; !ok {
					t.Fatalf("summary selected value missing: %s %v", key, rows[0])
				}
			}
			if rows[1][columns["request_plan.venues"]] != `["venue-a","venue-b"]` || rows[1][columns["request_plan.optional"]] != "null" || rows[1][columns["meta.requests"]] != "0" {
				t.Fatalf("summary cells changed: %v", rows)
			}
		}
		for _, primary := range []string{"items", "checks"} {
			cmd := &cobra.Command{}
			var out bytes.Buffer
			cmd.SetOut(&out)
			flags := rootFlags{csv: mode == "csv", plain: mode == "plain"}
			if err := planningPrint(cmd, &flags, planner.Result{primary: []map[string]any{}, "meta": map[string]any{"requests": 0}}); err != nil {
				t.Fatal(err)
			}
			if out.String() != emptyTabularResultMarker+"\n" {
				t.Fatalf("default empty table changed into a summary: mode%s primary%s out%q", mode, primary, out.String())
			}
		}
	}
}
