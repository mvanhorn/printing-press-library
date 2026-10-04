// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package learn

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"modernc.org/sqlite"
)

var carstayReadFailureOnce sync.Once
var carstayReadFailureRegistration error
var carstayReadFailureMode atomic.Int32
var carstayReadCancellation atomic.Pointer[context.CancelFunc]

func carstayRegisterReadFailure(t *testing.T) {
	t.Helper()
	carstayReadFailureOnce.Do(func() {
		carstayReadFailureRegistration = sqlite.RegisterScalarFunction("carstay_failed_identity_payload", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if carstayReadFailureMode.Load() == 1 {
				return nil, fmt.Errorf("synthetic cached-identity read failure")
			}
			if cancel := carstayReadCancellation.Load(); cancel != nil {
				(*cancel)()
			}
			return args[0], nil
		})
	})
	if carstayReadFailureRegistration != nil {
		t.Fatal(carstayReadFailureRegistration)
	}
	t.Cleanup(func() { carstayReadFailureMode.Store(0); carstayReadCancellation.Store(nil) })
}

func TestCarstayRecallPropagatesActualCachedIdentityReadFailures(t *testing.T) {
	for _, pattern := range []bool{false, true} {
		for _, cancelRead := range []bool{false, true} {
			t.Run(fmt.Sprintf("pattern=%v/cancel=%v", pattern, cancelRead), func(t *testing.T) {
				carstayRegisterReadFailure(t)
				db, opts := carstayRankedFixture(t)
				carstayRankedResource(t, db, "widgets", carstayAlphaID, `{"name":"Alpha"}`)
				if pattern {
					carstayRankedPattern(t, db, "widget {entity}", "000000000000000000000{entity:target}", "widgets", "target", 2, false)
				} else {
					seedCanonicalLearning(t, db, "alpha widget today", `["Alpha"]`, carstayAlphaID, "widgets")
				}
				if _, err := db.Exec(`ALTER TABLE resources RENAME TO failed_identity_rows`); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`CREATE VIEW resources AS SELECT resource_type,id,carstay_failed_identity_payload(data) AS data FROM failed_identity_rows`); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if cancelRead {
					carstayReadFailureMode.Store(2)
					carstayReadCancellation.Store(&cancel)
				} else {
					carstayReadFailureMode.Store(1)
				}
				got, err := Recall(ctx, db, "Alpha widget today", opts)
				if err == nil || got.Found || len(got.Results) != 0 {
					t.Fatalf("failed cache read claimed success: err=%v result=%+v", err, got)
				}
				if cancelRead {
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("cancellation classification lost: %v", err)
					}
				} else if !strings.Contains(err.Error(), "synthetic cached-identity read failure") {
					t.Fatalf("read diagnostic lost: %v", err)
				}
			})
		}
	}
}

func TestCarstayRecallRetainsMissingAndUnknownIdentityFallback(t *testing.T) {
	for _, tc := range []struct {
		name             string
		pattern, missing bool
		wantMatch        string
	}{{"direct missing row", false, true, EntityMatchExact}, {"direct unknown identity", false, false, EntityMatchPartial}, {"pattern existing ID unknown identity", true, false, EntityMatchExact}} {
		t.Run(tc.name, func(t *testing.T) {
			db, opts := carstayRankedFixture(t)
			if !tc.missing {
				carstayRankedResource(t, db, "widgets", carstayAlphaID, `{}`)
			}
			if tc.pattern {
				carstayRankedPattern(t, db, "widget {entity}", "000000000000000000000{entity:target}", "widgets", "target", 2, false)
			} else {
				seedCanonicalLearning(t, db, "alpha widget today", `["Alpha"]`, carstayAlphaID, "widgets")
			}
			got := carstayRankedRecall(t, db, "Alpha widget today", opts)
			if !got.Found || len(got.Results) != 1 || got.Results[0].EntityMatch != tc.wantMatch {
				t.Fatalf("legitimate fallback lost: %+v", got)
			}
			warned := strings.Contains(strings.Join(got.Results[0].Warnings, " "), WarningResourceNotInStore)
			if warned != tc.missing {
				t.Fatalf("missing/unknown distinction lost: %+v", got)
			}
		})
	}
}

func TestCarstayRecallRejectsUnscannablePresentPayload(t *testing.T) {
	for _, pattern := range []bool{false, true} {
		t.Run(fmt.Sprintf("pattern=%v", pattern), func(t *testing.T) {
			db, opts := carstayRankedFixture(t)
			carstayRankedResource(t, db, "widgets", carstayAlphaID, `{"name":"Alpha"}`)
			if pattern {
				carstayRankedPattern(t, db, "widget {entity}", "000000000000000000000{entity:target}", "widgets", "target", 2, false)
			} else {
				seedCanonicalLearning(t, db, "alpha widget today", `["Alpha"]`, carstayAlphaID, "widgets")
			}
			if _, err := db.Exec(`ALTER TABLE resources RENAME TO unscannable_identity_rows`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`CREATE VIEW resources AS SELECT resource_type,id,NULL AS data FROM unscannable_identity_rows`); err != nil {
				t.Fatal(err)
			}
			var verifiedID string
			if err := db.QueryRow(`SELECT id FROM resources WHERE resource_type='widgets' AND id=?`, carstayAlphaID).Scan(&verifiedID); err != nil || verifiedID != carstayAlphaID {
				t.Fatalf("ID verification fixture failed: %q %v", verifiedID, err)
			}
			got, err := Recall(context.Background(), db, "Alpha widget today", opts)
			if err == nil || got.Found || len(got.Results) != 0 || !strings.Contains(err.Error(), "NULL") {
				t.Fatalf("present but unscannable payload became missing/exact: err=%v result=%+v", err, got)
			}
		})
	}
}
