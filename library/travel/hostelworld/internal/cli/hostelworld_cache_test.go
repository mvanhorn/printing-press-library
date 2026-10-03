package cli

import (
	"bytes"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/cliutil"
	"strings"
	"testing"
)

func TestPlanningRejectsUnavailableDataSourcesBeforeIO(t *testing.T) {
	t.Setenv("HOSTELWORLD_NO_LEARN", "true")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"hostels", "inspect", "67481", "--data-source", "local"}, "no local data source"},
		{[]string{"hostels", "saved", "--data-source", "live"}, "no live equivalent"},
		{[]string{"search", "Nui", "--data-source", "live"}, "no live equivalent"},
		{[]string{"search", "Nui", "--limit", "-1"}, "--limit must be"},
		{[]string{"workflow", "archive"}, "provider archive is unsupported"},
	} {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			restore, err := cliutil.SetHomeOverride("")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(restore)
			cmd := RootCmd()
			cmd.SetArgs(append(tc.args, "--home", t.TempDir(), "--agent", "--no-learn"))
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			err = cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}
