package cli

import (
	"encoding/json"
	"testing"
)

func TestExtractPageItemsAirflowEnvelopeKeys(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			name: "dags envelope",
			body: `{"dags":[{"dag_id":"daily_refresh"}],"total_entries":1}`,
			want: 1,
		},
		{
			name: "empty dags envelope",
			body: `{"dags":[],"total_entries":0}`,
			want: 0,
		},
		{
			name: "pools envelope",
			body: `{"pools":[{"name":"default_pool","slots":128}],"total_entries":1}`,
			want: 1,
		},
		{
			name: "task instances envelope",
			body: `{"task_instances":[{"task_id":"load","state":"failed"}],"total_entries":1}`,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, _, _ := extractPageItems(json.RawMessage(tt.body), "offset")
			if len(items) != tt.want {
				t.Fatalf("len(items) = %d, want %d", len(items), tt.want)
			}
		})
	}
}
