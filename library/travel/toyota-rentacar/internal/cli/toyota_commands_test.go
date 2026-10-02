package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/internal/toyota"
	"github.com/spf13/cobra"
)

func TestToyotaQuietPrintsDomainIdentities(t *testing.T) {
	price := 10010
	for _, tc := range []struct {
		name  string
		value any
		want  string
	}{
		{"shop", toyotaShopResult{Shop: toyota.Shop{ID: "63601:01V"}, Note: "Hours do not establish inventory."}, "63601:01V\n"},
		{"shops", toyota.ShopsResult{Shops: []toyota.Shop{{ID: "63601:01V"}, {ID: "63601:095"}}, Note: "Source note"}, "63601:01V\n63601:095\n"},
		{"classes", toyota.QuoteResult{Offers: []toyota.ClassOffer{{Class: "C1", Availability: "selectable", SourceEstimateJPY: &price}, {Class: "C0", Availability: "fully_booked"}}}, "C1\nC0\n"},
		{"options", toyota.OptionsResult{Options: []toyota.OptionFee{{Code: "etc", FeeJPY: 550, Basis: "per_rental"}}}, "etc\n"},
		{"eligibility", toyota.EligibilityResult{Paths: []toyota.EligibilityPathInfo{{License: "Japanese driver's license"}}, IndividualEligibility: "not_assessed"}, "Japanese driver's license\n"},
		{"route", toyota.OneWayResult{PickupShop: toyota.Shop{ID: "63601:01V"}, DropoffShop: toyota.Shop{ID: "63601:095"}, Family: "standard", Status: "calculated"}, "63601:01V->63601:095:standard\n"},
		{"handoff", toyota.Handoff{BookingURL: "https://rent.toyota.co.jp/eng/reservation/index01.aspx"}, "https://rent.toyota.co.jp/eng/reservation/index01.aspx\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			if err := toyotaPrint(cmd, &rootFlags{quiet: true}, tc.value); err != nil || out.String() != tc.want {
				t.Fatalf("quiet=%q, want=%q, err=%v", out.String(), tc.want, err)
			}
		})
	}
}

func TestToyotaHandoffPreservesPremiumClassCodes(t *testing.T) {
	pick := time.Now().In(time.FixedZone("JST", 9*3600)).AddDate(0, 0, 7)
	pick = time.Date(pick.Year(), pick.Month(), pick.Day(), 9, 0, 0, 0, pick.Location())
	for _, class := range []string{"LXC", "LXP"} {
		data, err := runToyotaCLI(t, []string{"booking", "handoff", "--pickup-shop", "63601:01V", "--pickup", pick.Format("2006-01-02T15:04"), "--dropoff", pick.AddDate(0, 0, 1).Format("2006-01-02T15:04"), "--class", class, "--json"})
		if err != nil {
			t.Fatal(err)
		}
		var out toyota.Handoff
		if err := json.Unmarshal(data, &out); err != nil || out.Class != class || out.InventoryChecked || out.ConfirmedFullTotalJPY != nil {
			t.Fatalf("premium handoff=%s err=%v", data, err)
		}
	}
}

func TestToyotaAgentQuietHandoffPreservesStructuredPlanning(t *testing.T) {
	pick := time.Now().In(time.FixedZone("JST", 9*3600)).AddDate(0, 0, 7)
	pick = time.Date(pick.Year(), pick.Month(), pick.Day(), 9, 0, 0, 0, pick.Location())
	data, err := runToyotaCLI(t, []string{"booking", "handoff", "--pickup-shop", "63601:01V", "--pickup", pick.Format("2006-01-02T15:04"), "--dropoff", pick.AddDate(0, 0, 1).Format("2006-01-02T15:04"), "--class", "C1", "--agent", "--quiet"})
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Meta struct {
			Source string `json:"source"`
		} `json:"meta"`
		Results toyota.Handoff `json:"results"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("agent quiet output must remain JSON: %s (%v)", data, err)
	}
	if out.Meta.Source != "computed" || out.Results.Class != "C1" || len(out.Results.Checklist) == 0 || len(out.Results.RequiresReentry) == 0 || out.Results.InventoryChecked || out.Results.ConfirmedFullTotalJPY != nil {
		t.Fatalf("agent quiet lost planning context: %s", data)
	}
}

func runToyotaCLI(t *testing.T, args []string) ([]byte, error) {
	t.Helper()
	testenv.Isolate(t)
	root := RootCmd()
	root.SetArgs(args)
	var out, stderr bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&stderr)
	err := root.Execute()
	return out.Bytes(), err
}

func TestToyotaDryRunAllCommandsHasStructuredOutputWithoutInputs(t *testing.T) {
	for _, path := range [][]string{{"shops", "search"}, {"shops", "get"}, {"cars", "quote"}, {"oneway", "quote"}, {"rental", "options"}, {"rental", "eligibility"}, {"booking", "handoff"}} {
		t.Run(fmt.Sprint(path), func(t *testing.T) {
			data, err := runToyotaCLI(t, append(path, "--dry-run", "--json"))
			if err != nil {
				t.Fatal(err)
			}
			var out map[string]any
			if err := json.Unmarshal(data, &out); err != nil {
				t.Fatalf("not JSON: %s %v", data, err)
			}
			if out["dry_run"] != true {
				t.Fatalf("dry-run missing: %s", data)
			}
		})
	}
}

func TestToyotaInputFailuresAreTypedAndPrecedeNetwork(t *testing.T) {
	for _, args := range [][]string{
		{"shops", "search", "--json"}, {"shops", "get", "--id", "invalid", "--json"},
		{"cars", "quote", "--pickup-shop", "63601:01V", "--json"},
		{"oneway", "quote", "--pickup-shop", "63601:01V", "--json"},
		{"rental", "options", "unexpected", "--json"}, {"rental", "eligibility", "--data-source", "local", "--json"},
	} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			_, err := runToyotaCLI(t, args)
			var ce *cliError
			if !errors.As(err, &ce) || ce.code != 2 {
				t.Fatalf("want exit2,got %v", err)
			}
		})
	}
}

func TestToyotaHandoffFieldSelectionAndAgentProvenance(t *testing.T) {
	pick := time.Now().In(time.FixedZone("JST", 9*3600)).AddDate(0, 0, 7)
	pick = time.Date(pick.Year(), pick.Month(), pick.Day(), 9, 0, 0, 0, pick.Location())
	base := []string{"booking", "handoff", "--pickup-shop", "63601:01V", "--dropoff-shop", "63601:095", "--pickup", pick.Format("2006-01-02T15:04"), "--dropoff", pick.AddDate(0, 0, 1).Format("2006-01-02T15:04"), "--class", "C1"}
	for _, tc := range []struct {
		name  string
		extra []string
		agent bool
	}{
		{"plain select", []string{"--json", "--select", "pickup_shop_id,confirmed_full_total_jpy"}, false},
		{"agent select", []string{"--agent", "--select", "pickup_shop_id,confirmed_full_total_jpy"}, true},
		{"agent quiet select", []string{"--agent", "--quiet", "--select", "pickup_shop_id,confirmed_full_total_jpy"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := runToyotaCLI(t, append(base, tc.extra...))
			if err != nil {
				t.Fatal(err)
			}
			var out map[string]any
			if err := json.Unmarshal(data, &out); err != nil {
				t.Fatal(err)
			}
			if tc.agent {
				meta := out["meta"].(map[string]any)
				if meta["source"] != "computed" {
					t.Fatalf("meta=%v", meta)
				}
				out = out["results"].(map[string]any)
			}
			if len(out) != 2 || out["pickup_shop_id"] != "63601:01V" || out["confirmed_full_total_jpy"] != nil {
				t.Fatalf("selection=%s", data)
			}
		})
	}
}
