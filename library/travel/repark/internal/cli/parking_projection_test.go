package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/repark/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/repark/internal/repark"
	"github.com/spf13/cobra"
)

func TestParkingProjectionPreservesChosenTariffArrays(t *testing.T) {
	for _, tc := range []struct {
		selector string
		keys     int
	}{{"id,name,rates,maximum_charges", 4}, {"id", 1}, {"rates.day_type", 1}} {
		t.Run(tc.selector, func(t *testing.T) {
			cmd := &cobra.Command{}
			var out bytes.Buffer
			cmd.SetOut(&out)
			flags := &rootFlags{asJSON: true, selectFields: tc.selector, compact: true}
			amount := 300
			data := repark.Lot{ID: "REP0022209", Name: "上汐４丁目第３", Address: "大阪府", Rates: []repark.Rate{{DayType: "全日", AmountJPY: &amount}}, Maximums: []repark.Maximum{{Kind: "time_window", Application: "repeating", AmountJPY: &amount}}}
			if err := parkingPrint(cmd, flags, repark.Meta{Source: "live"}, data); err != nil {
				t.Fatal(err)
			}
			var v struct {
				Results map[string]json.RawMessage `json:"results"`
			}
			if err := json.Unmarshal(out.Bytes(), &v); err != nil {
				t.Fatal(err)
			}
			if len(v.Results) != tc.keys {
				t.Fatalf("projection kept unrelated fields: %s", out.String())
			}
			if tc.selector == "id,name,rates,maximum_charges" {
				var rates []repark.Rate
				json.Unmarshal(v.Results["rates"], &rates)
				if len(rates) != 1 || rates[0].AmountJPY == nil || *rates[0].AmountJPY != 300 {
					t.Fatalf("explicit rates lost fields: %s", out.String())
				}
			}
			if flags.selectFields != tc.selector || !flags.compact {
				t.Fatal("output projection changed shared root flags")
			}
		})
	}
}

func TestParkingRefusesLocalSourceBeforeFetch(t *testing.T) {
	for _, args := range [][]string{{"detail", "REP0022209"}, {"nearby", "--park", "REP0022209"}, {"search", "東京駅"}, {"compare", "REP0022209", "REP0029431"}, {"quote", "REP0022209", "--bay", "1", "--start", "2026-10-03T08:00", "--end", "2026-10-03T12:00"}} {
		t.Run(args[0], func(t *testing.T) {
			testenv.Isolate(t)
			cmd := RootCmd()
			cmd.SetArgs(append(append([]string{"parking"}, args...), "--data-source", "local", "--json"))
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			err := cmd.Execute()
			var ce *cliError
			if !errors.As(err, &ce) || ce.code != 2 || !strings.Contains(err.Error(), "data-source") {
				t.Fatalf("expected source-strategy refusal, got %v; %s", err, out.String())
			}
			if strings.Contains(out.String(), `"requests": 1`) || strings.Contains(out.String(), "Repark request failed") {
				t.Fatalf("local refusal reached source: %s", out.String())
			}
		})
	}
}
