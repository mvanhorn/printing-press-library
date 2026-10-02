package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestFerryCommandsEnforceDeclaredSourceBeforeIO(t *testing.T) {
	for _, tc := range []struct {
		name        string
		constructor func(*rootFlags) *cobra.Command
		source      string
		args        []string
	}{
		{"quote", newNovelQuoteCmd, "local", []string{"--date", "2026-10-15"}},
		{"sailings", newNovelSailingsCmd, "local", []string{"--date", "2026-10-15"}},
		{"calendar", newNovelCalendarCmd, "local", nil},
		{"cabins", newNovelCabinsCmd, "local", nil},
		{"ports", newNovelPortsCmd, "local", nil},
		{"conditions", newNovelConditionsCmd, "local", nil},
		{"routes list", ferryRoutesCmd, "live", []string{"list"}},
		{"handoff", ferryHandoffCmd, "live", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.constructor(&rootFlags{asJSON: true, dataSource: tc.source})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tc.args)
			e := cmd.Execute()
			if e == nil || (!strings.Contains(e.Error(), "no local data source") && !strings.Contains(e.Error(), "no live equivalent")) {
				t.Fatalf("source policy reached IO or returned wrong error: %v / %s", e, out.String())
			}
		})
	}
}
