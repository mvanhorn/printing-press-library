// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for retraction.go. Not generated — safe to edit.

package cli

import (
	"context"
	"encoding/json"
	"testing"
)

// stubCrossref returns a canned Crossref envelope, so checkDOI can be
// exercised against exact JSON payloads without a network call.
type stubCrossref struct {
	payload string
}

func (s stubCrossref) Get(_ context.Context, _ string, _ map[string]string) (json.RawMessage, error) {
	return json.RawMessage(s.payload), nil
}

// TestCheckDOICrossrefFieldNames pins the Crossref field names that checkDOI
// unmarshals. encoding/json fails silently on a struct-tag mismatch: the slice
// is left empty, no error is raised, and the retraction goes undetected. These
// cases fail loudly if the tags drift.
func TestCheckDOICrossrefFieldNames(t *testing.T) {
	cases := []struct {
		name                 string
		payload              string
		wantRetracted        bool
		wantConcern          bool
		wantType             string
		wantDate             string
		wantNoticeDOI        string
		wantConcernNoticeDOI string
		wantSignals          []string
	}{
		{
			// The regression this test exists for. The field is "updated-by",
			// not "update-by", and the title carries no RETRACTED prefix, so
			// the tag is the only thing that can surface the retraction.
			name: "updated-by retraction without title prefix",
			payload: `{"message":{
				"DOI":"10.1538/expanim.54.1",
				"title":["Effect of Treadmill Exercise on Bone Mass in Female Rats"],
				"issued":{"date-parts":[[2005,1,1]]},
				"updated-by":[{
					"DOI":"10.1538/expanim.54.1.r1",
					"type":"retraction",
					"label":"Retraction",
					"source":"retraction-watch",
					"updated":{"date-parts":[[2022,8,1]]}
				}]
			}}`,
			wantRetracted: true,
			wantType:      "retraction",
			wantDate:      "2022-08-01",
			wantNoticeDOI: "10.1538/expanim.54.1.r1",
			wantSignals:   []string{"crossref-update:retraction"},
		},
		{
			// A correction precedes the retraction in the array. Selecting the
			// first entry would report the wrong type and the wrong date.
			name: "correction before retraction is skipped",
			payload: `{"message":{
				"DOI":"10.1016/s0140-6736(97)11096-0",
				"title":["RETRACTED: Ileal-lymphoid-nodular hyperplasia"],
				"published":{"date-parts":[[1998,2]]},
				"updated-by":[
					{"DOI":"10.1016/s0140-6736(04)15715-2","type":"correction","label":"Correction","source":"retraction-watch","updated":{"date-parts":[[2004,3,6]]}},
					{"DOI":"10.1016/s0140-6736(10)60175-4","type":"retraction","label":"Retraction","source":"retraction-watch","updated":{"date-parts":[[2010,2,6]]}}
				]
			}}`,
			wantRetracted: true,
			wantType:      "retraction",
			wantDate:      "2010-02-06",
			wantNoticeDOI: "10.1016/s0140-6736(10)60175-4",
			wantSignals:   []string{"crossref-update:retraction", "title-prefix"},
		},
		{
			// update-to means this checked record updates another work. It
			// does not prove the checked record itself was retracted.
			name: "update-to retraction points away from checked record",
			payload: `{"message":{
				"DOI":"10.1007/s11277-021-09072-0",
				"title":["Deep Reinforcement Learning-Based Smart Manufacturing Plants"],
				"issued":{"date-parts":[[2021]]},
				"update-to":[{
					"DOI":"10.1007/s11277-021-09072-0",
					"type":"retraction",
					"label":"Retraction",
					"source":"publisher",
					"updated":{"date-parts":[[2022,12,6]]}
				}]
			}}`,
			wantRetracted: false,
			wantType:      "",
			wantDate:      "",
			wantNoticeDOI: "",
			wantSignals:   nil,
		},
		{
			name: "expression of concern is not a retraction",
			payload: `{"message":{
				"DOI":"10.1000/concerned",
				"title":["Paper under review"],
				"updated-by":[{
					"DOI":"10.5555/notice","type":"expression_of_concern",
					"source":"publisher","updated":{"date-parts":[[2026,9,1]]}
				}]
			}}`,
			wantRetracted:        false,
			wantConcern:          true,
			wantType:             "expression_of_concern",
			wantDate:             "2026-09-01",
			wantNoticeDOI:        "10.5555/notice",
			wantConcernNoticeDOI: "10.5555/notice",
			wantSignals:          []string{"crossref-update:expression_of_concern"},
		},
		{
			name: "title retraction and concern keep distinct notices",
			payload: `{"message":{
				"DOI":"10.1000/mixed-title",
				"title":["RETRACTED: Paper under review"],
				"updated-by":[{
					"DOI":"10.5555/concern","type":"expression_of_concern",
					"source":"publisher","updated":{"date-parts":[[2026,9,1]]}
				}]
			}}`,
			wantRetracted:        true,
			wantConcern:          true,
			wantType:             "retraction",
			wantDate:             "",
			wantNoticeDOI:        "",
			wantConcernNoticeDOI: "10.5555/concern",
			wantSignals:          []string{"crossref-update:expression_of_concern", "title-prefix"},
		},
		{
			name: "retraction followed by concern retains both signals",
			payload: `{"message":{
				"DOI":"10.1000/mixed-updates",
				"title":["Paper under review"],
				"updated-by":[
					{"DOI":"10.5555/retraction","type":"retraction","updated":{"date-parts":[[2026,8,1]]}},
					{"DOI":"10.5555/concern","type":"expression_of_concern","updated":{"date-parts":[[2026,9,1]]}}
				]
			}}`,
			wantRetracted:        true,
			wantConcern:          true,
			wantType:             "retraction",
			wantDate:             "2026-08-01",
			wantNoticeDOI:        "10.5555/retraction",
			wantConcernNoticeDOI: "10.5555/concern",
			wantSignals:          []string{"crossref-update:retraction", "crossref-update:expression_of_concern"},
		},
		{
			// No update record at all: the title prefix still flags it, but
			// there is no date to report and none must be invented.
			name: "title prefix alone yields no date",
			payload: `{"message":{
				"DOI":"10.1000/example",
				"title":["RETRACTED ARTICLE: Something"],
				"issued":{"date-parts":[[2019,5,4]]}
			}}`,
			wantRetracted: true,
			wantType:      "retraction",
			wantDate:      "",
			wantNoticeDOI: "",
			wantSignals:   []string{"title-prefix"},
		},
		{
			name: "clean record stays clean",
			payload: `{"message":{
				"DOI":"10.1016/s1557-0843(07)80009-1",
				"title":["Insulin delivery devices"],
				"issued":{"date-parts":[[2007]]}
			}}`,
			wantRetracted: false,
			wantType:      "",
			wantDate:      "",
			wantNoticeDOI: "",
			wantSignals:   nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v, err := checkDOI(context.Background(), stubCrossref{payload: tc.payload}, "", "unused")
			if err != nil {
				t.Fatalf("checkDOI returned error: %v", err)
			}
			if v.Retracted != tc.wantRetracted {
				t.Errorf("Retracted = %v, want %v", v.Retracted, tc.wantRetracted)
			}
			if v.ExpressionOfConcern != tc.wantConcern {
				t.Errorf("ExpressionOfConcern = %v, want %v", v.ExpressionOfConcern, tc.wantConcern)
			}
			if v.UpdateType != tc.wantType {
				t.Errorf("UpdateType = %q, want %q", v.UpdateType, tc.wantType)
			}
			if v.Date != tc.wantDate {
				t.Errorf("Date = %q, want %q", v.Date, tc.wantDate)
			}
			if v.NoticeDOI != tc.wantNoticeDOI {
				t.Errorf("NoticeDOI = %q, want %q", v.NoticeDOI, tc.wantNoticeDOI)
			}
			if v.ConcernNoticeDOI != tc.wantConcernNoticeDOI {
				t.Errorf("ConcernNoticeDOI = %q, want %q", v.ConcernNoticeDOI, tc.wantConcernNoticeDOI)
			}
			if len(v.Signals) != len(tc.wantSignals) {
				t.Fatalf("Signals = %v, want %v", v.Signals, tc.wantSignals)
			}
			for i, want := range tc.wantSignals {
				if v.Signals[i] != want {
					t.Errorf("Signals[%d] = %q, want %q", i, v.Signals[i], want)
				}
			}
		})
	}
}

// TestCheckDOIPublishedIsNotRetractionDate guards the distinction the retraction
// date exists to make: published is when the article appeared, date is when it
// was retracted. Collapsing the two would misreport a 1998 paper as retracted
// in 1998.
func TestCheckDOIPublishedIsNotRetractionDate(t *testing.T) {
	payload := `{"message":{
		"DOI":"10.1016/s0140-6736(97)11096-0",
		"title":["RETRACTED: Ileal-lymphoid-nodular hyperplasia"],
		"published":{"date-parts":[[1998,2]]},
		"updated-by":[{
			"DOI":"10.1016/s0140-6736(10)60175-4",
			"type":"retraction",
			"label":"Retraction",
			"source":"retraction-watch",
			"updated":{"date-parts":[[2010,2,6]]}
		}]
	}}`
	v, err := checkDOI(context.Background(), stubCrossref{payload: payload}, "", "unused")
	if err != nil {
		t.Fatalf("checkDOI returned error: %v", err)
	}
	if v.Published != "1998-02" {
		t.Errorf("Published = %q, want %q", v.Published, "1998-02")
	}
	if v.Date != "2010-02-06" {
		t.Errorf("Date = %q, want %q", v.Date, "2010-02-06")
	}
	if v.Published == v.Date {
		t.Error("Published and Date must not collapse into the same value")
	}
}
