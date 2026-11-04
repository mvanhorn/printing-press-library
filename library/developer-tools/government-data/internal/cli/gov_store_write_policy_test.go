package cli

import (
	"github.com/spf13/cobra"
	"testing"
)

func TestGovCatalogStoreWritePolicy(t *testing.T) {
	for _, command := range []string{"find", "export"} {
		for _, tc := range []struct {
			name string
			args []string
			want bool
		}{
			{"omitted", nil, false}, {"explicit-empty", []string{"--save="}, false}, {"snapshot", []string{"--save=sample"}, true},
		} {
			t.Run(command+"/"+tc.name, func(t *testing.T) {
				root := &cobra.Command{Use: "gov"}
				root.AddCommand(newGovCatalog(&rootFlags{}))
				cmd, _, err := root.Find([]string{"catalog", command})
				if err != nil {
					t.Fatal(err)
				}
				if err := cmd.ParseFlags(tc.args); err != nil {
					t.Fatal(err)
				}
				if got := commandMayWriteStore(cmd); got != tc.want {
					t.Fatalf("may write=%v, want %v", got, tc.want)
				}
			})
		}
	}
	if !commandMayWriteStore(newGovSync(&rootFlags{})) {
		t.Fatal("sync must remain a writer")
	}
}

func TestConditionalStoreWriteBooleanValue(t *testing.T) {
	for _, value := range []string{"false", "true"} {
		cmd := &cobra.Command{Use: "conditional", Annotations: map[string]string{"mcp:read-only-unless-flags": "persist"}}
		cmd.Flags().Bool("persist", false, "")
		if err := cmd.ParseFlags([]string{"--persist=" + value}); err != nil {
			t.Fatal(err)
		}
		if got := commandMayWriteStore(cmd); got != (value == "true") {
			t.Fatalf("persist=%s: may write=%v", value, got)
		}
	}
}
