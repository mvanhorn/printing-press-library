package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeMonarch serves txn for the transaction read and records every operation it receives.
func fakeMonarch(t *testing.T, txn map[string]any) *[]string {
	t.Helper()
	var ops []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req gqlRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		ops = append(ops, req.OperationName)
		data := map[string]any{"getTransaction": txn}
		if req.OperationName == "Common_SplitTransactionMutation" {
			data = map[string]any{"updateTransactionSplit": map[string]any{"transaction": txn}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	t.Cleanup(srv.Close)
	old := graphqlURL
	graphqlURL = srv.URL
	t.Cleanup(func() { graphqlURL = old })
	t.Setenv("MONARCH_TOKEN", "test")
	return &ops
}

func TestTransactionSplitGuards(t *testing.T) {
	file := filepath.Join(t.TempDir(), "splits.json")
	lines := `[{"merchantName":"C","amount":-60,"categoryId":"a"},{"merchantName":"C","amount":-22.17,"categoryId":"b"}]`
	if err := os.WriteFile(file, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		txn     map[string]any
		args    []string
		wantErr string
		wantOps []string
	}{
		"dry-run sends nothing": {
			args: []string{"--file", file},
		},
		"amount changed": {
			txn:     map[string]any{"amount": -90.00},
			args:    []string{"--file", file, "--yes"},
			wantErr: "amount is -90.00",
			wantOps: []string{"GetTransactionDrawer"},
		},
		"split line": {
			txn:     map[string]any{"amount": -82.17, "isSplitTransaction": true},
			args:    []string{"--file", file, "--yes"},
			wantErr: "one line of a split",
			wantOps: []string{"GetTransactionDrawer"},
		},
		"already split": {
			txn:     map[string]any{"amount": -82.17, "hasSplitTransactions": true},
			args:    []string{"--file", file, "--yes"},
			wantErr: "already split",
			wantOps: []string{"GetTransactionDrawer"},
		},
		"clear unsplit": {
			txn:     map[string]any{"amount": -82.17},
			args:    []string{"--clear", "--yes"},
			wantErr: "nothing to undo",
			wantOps: []string{"GetTransactionDrawer"},
		},
		"replace existing split": {
			txn:     map[string]any{"amount": -82.17, "hasSplitTransactions": true},
			args:    []string{"--file", file, "--replace", "--yes"},
			wantOps: []string{"GetTransactionDrawer", "Common_SplitTransactionMutation"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ops := fakeMonarch(t, tc.txn)
			cmd := newTransactionSplitCmd()
			cmd.SetArgs(append([]string{"123", "--expect-total", "-82.17"}, tc.args...))
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			err := cmd.Execute()
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("want error %q, got %v", tc.wantErr, err)
			}
			if strings.Join(*ops, ",") != strings.Join(tc.wantOps, ",") {
				t.Fatalf("want requests %v, got %v", tc.wantOps, *ops)
			}
		})
	}
}
