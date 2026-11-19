package cli

import (
	"bytes"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"strings"
	"testing"
)

func TestE3AuthPositionalSecret(t *testing.T) {
	testenv.Isolate(t)
	const secret = "synthetic_secret_do_not_echo"
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"auth", "set-token", secret})
	err := cmd.Execute()
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "pass the token on stdin") {
		t.Fatalf("err=%v out=%s", err, &out)
	}
	if strings.Contains(err.Error()+out.String(), secret) {
		t.Fatal("positional secret was echoed")
	}
}
