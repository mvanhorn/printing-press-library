// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package learn

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestResourceValidationDistinguishesMissingFromReadFailure(t *testing.T) {
	for _, mode := range []string{"missing", "unknown", "closed", "cancelled", "schema"} {
		t.Run(mode, func(t *testing.T) {
			db := openRecallTestDB(t)
			ctx := context.Background()
			switch mode {
			case "unknown":
				seedRecallResource(t, db, "source", "park-ALPHA", `{}`)
			case "closed":
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "schema":
				if _, err := db.Exec(`DROP TABLE resources`); err != nil {
					t.Fatal(err)
				}
			}
			hit := Hit{ResourceType: "source", ResourceID: "park-ALPHA", Confidence: 2}
			present, err := validateResource(ctx, db, nil, &hit, []string{"Alpha"}, []string{"Alpha"}, nil)
			if mode == "missing" {
				if err != nil || present || hit.EntityMatch != EntityMatchExact || !slices.Contains(hit.Warnings, WarningResourceNotInStore) {
					t.Fatalf("missing fallback changed: %#v %v %v", hit, present, err)
				}
			} else if mode == "unknown" {
				if err != nil || !present || hit.EntityMatch != EntityMatchPartial {
					t.Fatalf("unknown identity changed: %#v %v %v", hit, present, err)
				}
			} else {
				if err == nil || present || hit.EntityMatch == EntityMatchExact || slices.Contains(hit.Warnings, WarningResourceNotInStore) {
					t.Fatalf("read failure became missing/exact: %#v %v %v", hit, present, err)
				}
				if mode == "cancelled" && !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation lost: %v", err)
				}
			}
		})
	}
}

func TestRecallRejectsPayloadReadFailureForDirectAndPattern(t *testing.T) {
	for _, mode := range []string{"direct", "pattern"} {
		t.Run(mode, func(t *testing.T) {
			db := openRecallTestDB(t)
			// Identifier verification succeeds, but a NULL payload produces a
			// genuine Scan error. This is not the missing-row compatibility case.
			if _, err := db.Exec(`DROP TABLE resources`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`CREATE VIEW resources AS SELECT 'source' resource_type,'park-ALPHA' id,NULL data`); err != nil {
				t.Fatal(err)
			}
			if mode == "direct" {
				if _, err := db.Exec(`INSERT INTO search_learnings(query_pattern,query_entities,resource_type,resource_id,action,source,confidence) VALUES('park','["Alpha"]','source','park-ALPHA','boost','taught',2)`); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := db.Exec(`INSERT INTO search_patterns(query_template,resource_template,resource_type,strategy,entity_kind,confidence,source) VALUES('park {entity}','park-{entity:uppercase}','source','substitute','uppercase',2,'taught')`); err != nil {
					t.Fatal(err)
				}
			}
			result, err := Recall(context.Background(), db, "Alpha park", Opts{})
			if err == nil || !strings.Contains(err.Error(), "recall resource validation source/park-ALPHA") || result.Found || len(result.Results) > 0 {
				t.Fatalf("payload read failure returned factual success: %#v %v", result, err)
			}
		})
	}
}
