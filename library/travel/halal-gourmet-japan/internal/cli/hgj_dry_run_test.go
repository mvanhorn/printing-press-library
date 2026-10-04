// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestHGJDryRunValidatesInputsBeforeReceiptWithoutIO(t *testing.T) {
	constructors := map[string]func(*rootFlags) *cobra.Command{
		"restaurants get": newRestaurantsGetCmd, "prayer get": newPrayerGetCmd,
		"restaurants search": newRestaurantsSearchCmd, "prayer search": newPrayerSearchCmd,
		"plan compare": newNovelPlanCompareCmd, "plan match": newNovelPlanMatchCmd,
		"plan gaps": newNovelPlanGapsCmd, "plan pair": newNovelPlanPairCmd,
		"plan changes": newNovelPlanChangesCmd,
	}
	for _, tc := range []struct {
		name, leaf, source string
		args               []string
		flags              map[string]string
		valid              bool
	}{
		{name: "missing restaurant ID", leaf: "restaurants get"},
		{name: "invalid prayer ID", leaf: "prayer get", args: []string{"not-numeric"}},
		{name: "extra get ID", leaf: "restaurants get", args: []string{"300739", "949742"}},
		{name: "unscoped search", leaf: "restaurants search"},
		{name: "search positional", leaf: "prayer search", args: []string{"Tokyo"}},
		{name: "search bad limit", leaf: "restaurants search", flags: map[string]string{"prefecture": "Tokyo", "limit": "0"}},
		{name: "search bad facility", leaf: "prayer search", flags: map[string]string{"prefecture": "Tokyo", "prayer-feature": "certified"}},
		{name: "local search", leaf: "restaurants search", source: "local", flags: map[string]string{"prefecture": "Tokyo"}},
		{name: "compare missing selections", leaf: "plan compare"},
		{name: "gaps missing selections", leaf: "plan gaps"},
		{name: "changes missing selections", leaf: "plan changes"},
		{name: "plan invalid ID", leaf: "plan compare", flags: map[string]string{"restaurants": "wrong"}},
		{name: "plan invalid limit", leaf: "plan compare", flags: map[string]string{"restaurants": "300739", "limit": "0"}},
		{name: "plan live source", leaf: "plan compare", source: "live", flags: map[string]string{"restaurants": "300739"}},
		{name: "match missing requirement", leaf: "plan match", flags: map[string]string{"restaurants": "300739"}},
		{name: "match invalid requirement", leaf: "plan match", flags: map[string]string{"restaurants": "300739", "require": "friendly"}},
		{name: "pair one family", leaf: "plan pair", flags: map[string]string{"restaurants": "300739"}},
		{name: "pair invalid radius", leaf: "plan pair", flags: map[string]string{"restaurants": "300739", "prayer": "838884", "max-km": "101"}},
		{name: "valid restaurant", leaf: "restaurants get", args: []string{"300739"}, valid: true},
		{name: "valid prayer", leaf: "prayer get", args: []string{"838884"}, valid: true},
		{name: "valid food search", leaf: "restaurants search", flags: map[string]string{"prefecture": "Tokyo"}, valid: true},
		{name: "valid prayer search", leaf: "prayer search", flags: map[string]string{"prefecture": "Tokyo"}, valid: true},
		{name: "valid comparison", leaf: "plan compare", flags: map[string]string{"restaurants": "300739"}, valid: true},
		{name: "valid gaps", leaf: "plan gaps", flags: map[string]string{"prayer": "838884"}, valid: true},
		{name: "valid changes", leaf: "plan changes", flags: map[string]string{"restaurants": "300739"}, valid: true},
		{name: "valid match", leaf: "plan match", flags: map[string]string{"restaurants": "300739", "require": "certified,prayer"}, valid: true},
		{name: "valid pair", leaf: "plan pair", flags: map[string]string{"restaurants": "300739", "prayer": "838884"}, valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Config does not exist: a correct dry-run must never resolve it,
			// create a database, or contact the provider after input validation.
			f := &rootFlags{dryRun: true, asJSON: true, noLearn: true, dataSource: tc.source, configPath: filepath.Join(t.TempDir(), "missing-config.json")}
			cmd := constructors[tc.leaf](f)
			cmd.SetContext(context.Background())
			for name, value := range tc.flags {
				if err := cmd.Flags().Set(name, value); err != nil {
					t.Fatal(err)
				}
			}
			var out bytes.Buffer
			cmd.SetOut(&out)
			err := cmd.RunE(cmd, tc.args)
			if !tc.valid {
				if err == nil || out.Len() != 0 {
					t.Fatalf("invalid input returned receipt: err=%v out=%s", err, out.String())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var receipt struct {
				DryRun bool `json:"dry_run"`
			}
			if json.Unmarshal(out.Bytes(), &receipt) != nil || !receipt.DryRun {
				t.Fatalf("missing dry-run receipt: %s", out.String())
			}
		})
	}
}
