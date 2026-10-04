package cli

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/cliutil"
)

func TestMichiReadOnlyCallsLeaveNoLearningState(t *testing.T) {
	t.Setenv("MICHI_NO_EKI_NO_LEARN", "")
	for _, argv := range [][]string{{}, {"--json"}, {"--home", "TEST_HOME"}, {"--json", "--home", "TEST_HOME"}, {"catalog", "--json"}, {"catalog", "--help"}, {"catalog", "--wrong-native-flag"}, {"guidance", "--json"}, {"stations", "get", "--id", "19187", "--dry-run", "--json"}, {"stations", "get", "--wrong-source-flag"}, {"stations", "get", "--help"}, {"bulletins", "list", "--dry-run", "--json"}, {"bulletins", "list", "--wrong-source-flag"}, {"bulletins", "list", "--help"}} {
		t.Run("root-or-"+strings.Join(argv, " "), func(t *testing.T) {
			home := t.TempDir()
			restore, err := cliutil.SetHomeOverride(home)
			if err != nil {
				t.Fatal(err)
			}
			defer restore()
			old := os.Args
			defer func() { os.Args = old }()
			invocation := append([]string(nil), argv...)
			for i, arg := range invocation {
				if arg == "TEST_HOME" {
					invocation[i] = home
				}
			}
			os.Args = append([]string{"michi-no-eki-pp-cli"}, invocation...)
			var flags rootFlags
			root := newRootCmd(&flags)
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			root.SetArgs(invocation)
			executed, err := root.ExecuteC()
			journalInvocation(&flags, root, executed, err, "", "")
			deriveFlagCorrections(&flags, root, executed, err)
			entries, readErr := os.ReadDir(home)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("read-only call mutated isolated home: %v, %v", entries, readErr)
			}
		})
	}
}
