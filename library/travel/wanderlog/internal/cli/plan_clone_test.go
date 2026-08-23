// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func TestPlanCloneResolvesPlanKeys(t *testing.T) {
	tests := []struct {
		name      string
		sourceKey string
		sourceURL string
		want      string
		wantErr   bool
	}{
		{name: "source key", sourceKey: "naertjcoixqrgrfc", want: "naertjcoixqrgrfc"},
		{name: "plan url", sourceURL: "https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared", want: "naertjcoixqrgrfc"},
		{name: "view url", sourceURL: "https://wanderlog.com/view/uzyvvtuwtc/shared", want: "uzyvvtuwtc"},
		{name: "invalid source key", sourceKey: "bad key", wantErr: true},
		{name: "missing", wantErr: true},
		{name: "unrecognized url", sourceURL: "https://wanderlog.com/explore/okinawa", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolvePlanKey(tt.sourceKey, tt.sourceURL)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got key %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolvePlanKey returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolvePlanKey = %q, want %q", got, tt.want)
			}
		})
	}
}
