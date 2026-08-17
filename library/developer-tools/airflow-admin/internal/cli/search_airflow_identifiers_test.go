package cli

import (
	"encoding/json"
	"testing"
)

func TestSearchKeepsAirflowIdentifierRows(t *testing.T) {
	rows := []json.RawMessage{
		json.RawMessage(`{"dag_id":"daily_refresh","is_paused":false}`),
		json.RawMessage(`{"dag_run_id":"manual__2026-06-14","state":"failed"}`),
		json.RawMessage(`{"task_id":"load","state":"success"}`),
	}

	for _, row := range rows {
		if isNilOrEmpty(row) {
			t.Fatalf("Airflow row was filtered out: %s", string(row))
		}
	}
}
