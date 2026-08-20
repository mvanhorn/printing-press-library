// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"testing"
)

func TestChartQueryParamsPeriod1OverridesRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		dateRange string
		period1   int
		period2   int
		want      map[string]string
	}{
		{
			name:      "default range remains without absolute start",
			dateRange: "1mo",
			want: map[string]string{
				"events":   "div|split|earn",
				"interval": "1d",
				"range":    "1mo",
			},
		},
		{
			name:      "period1 omits default range",
			dateRange: "1mo",
			period1:   631152000,
			want: map[string]string{
				"events":   "div|split|earn",
				"interval": "1d",
				"period1":  "631152000",
			},
		},
		{
			name:      "absolute interval preserves both periods and omits range",
			dateRange: "1mo",
			period1:   631152000,
			period2:   1785369600,
			want: map[string]string{
				"events":   "div|split|earn",
				"interval": "1d",
				"period1":  "631152000",
				"period2":  "1785369600",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := chartQueryParams(
				"1d",
				tt.dateRange,
				tt.period1,
				tt.period2,
				"div|split|earn",
				false,
			)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("chartQueryParams() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
