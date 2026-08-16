package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/wework/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/wework/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/wework/internal/config"
)

func authTestJWT(payload string) string {
	return "aaa." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".bbb"
}

func TestParseComposedAuthJSON(t *testing.T) {
	cases := []struct {
		name, in                   string
		wantT, wantR, wantU, wantM string
	}{
		{"snippet keys", `{"token":"t1","refreshToken":"r1","uuid":"u1","memberType":"m1"}`, "t1", "r1", "u1", "m1"},
		{"env keys", `{"WEWORK_TOKEN":"t2","WEWORK_REFRESH_TOKEN":"r2","WEWORK_UUID":"u2","WEWORK_MEMBER_TYPE":"m2"}`, "t2", "r2", "u2", "m2"},
		{"browser keys", `{"access_token":"t3","refresh_token":"r3","CurrentAccountUUID":"u3","WWMemberType":"m3"}`, "t3", "r3", "u3", "m3"},
		{"partial", `{"token":"t4"}`, "t4", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gt, gr, gu, gm, err := parseComposedAuthJSON([]byte(c.in))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gt != c.wantT || gr != c.wantR || gu != c.wantU || gm != c.wantM {
				t.Fatalf("got (%q,%q,%q,%q), want (%q,%q,%q,%q)", gt, gr, gu, gm, c.wantT, c.wantR, c.wantU, c.wantM)
			}
		})
	}
}

func TestParseComposedAuthJSONInvalid(t *testing.T) {
	if _, _, _, _, err := parseComposedAuthJSON([]byte("not json")); err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func isolateAuthCLI(t *testing.T) {
	t.Helper()
	testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, cliutil.StateDir)
	t.Setenv("WEWORK_TOKEN", "")
	t.Setenv("WEWORK_REFRESH_TOKEN", "")
	t.Setenv("WEWORK_UUID", "")
	t.Setenv("WEWORK_MEMBER_TYPE", "")
	t.Setenv(noLearnEnvVar, "true")
}

func executeAuthCLI(t *testing.T, stdin string, args ...string) (string, string, error) {
	t.Helper()
	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetIn(strings.NewReader(stdin))
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func decodeAuthJSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("decode auth JSON: %v (output=%q)", err, raw)
	}
	return out
}

func TestAuthImportAliasCreatesRenewableSession(t *testing.T) {
	isolateAuthCLI(t)
	jwt := authTestJWT(`{"iss":"https://idp.wework.com/","azp":"public-client","exp":4102444800}`)
	bundle, _ := json.Marshal(map[string]string{
		"token": jwt, "refreshToken": "refresh-1", "uuid": "account-1", "memberType": "3",
	})
	if stdout, stderr, err := executeAuthCLI(t, string(bundle), "auth", "import", "--stdin", "--json"); err != nil {
		t.Fatalf("auth import failed: %v (stdout=%q stderr=%q)", err, stdout, stderr)
	}

	stdout, stderr, err := executeAuthCLI(t, "", "auth", "whoami", "--json")
	if err != nil {
		t.Fatalf("auth whoami failed: %v (stderr=%q)", err, stderr)
	}
	status := decodeAuthJSON(t, stdout)
	if status["refresh_token"] != true || status["renewable"] != true || status["ready"] != true {
		t.Fatalf("renewable status missing after import: %#v", status)
	}
}

func TestAuthImportRejectsIncompleteBundleByDefault(t *testing.T) {
	isolateAuthCLI(t)
	jwt := authTestJWT(`{"iss":"https://idp.wework.com/","azp":"public-client","exp":4102444800}`)
	bundle, _ := json.Marshal(map[string]string{"token": jwt})
	stdout, stderr, err := executeAuthCLI(t, string(bundle), "auth", "session-import", "--stdin")
	if err == nil {
		t.Fatalf("auth import accepted an incomplete renewable bundle (stdout=%q stderr=%q)", stdout, stderr)
	}
	if !strings.Contains(err.Error(), "complete renewable session") {
		t.Fatalf("auth import returned an unactionable incomplete-bundle error: %v", err)
	}
}

func TestAuthImportAllowsExplicitPartialRepair(t *testing.T) {
	isolateAuthCLI(t)
	stdout, stderr, err := executeAuthCLI(t, `{"uuid":"account-1"}`, "auth", "session-import", "--stdin", "--allow-partial", "--json")
	if err != nil {
		t.Fatalf("explicit partial auth repair failed: %v (stdout=%q stderr=%q)", err, stdout, stderr)
	}
	status := decodeAuthJSON(t, stdout)
	if status["uuid"] != true || status["renewable"] != false {
		t.Fatalf("partial repair returned unexpected status: %#v", status)
	}
}

func TestAuthImportRejectsUntrustedRenewableToken(t *testing.T) {
	isolateAuthCLI(t)
	jwt := authTestJWT(`{"iss":"https://evil.example.com/","azp":"public-client","exp":4102444800}`)
	bundle, _ := json.Marshal(map[string]string{
		"token": jwt, "refreshToken": "refresh-1", "uuid": "account-1", "memberType": "3",
	})
	stdout, stderr, err := executeAuthCLI(t, string(bundle), "auth", "session-import", "--stdin")
	if err == nil {
		t.Fatalf("auth import accepted an untrusted refresh destination (stdout=%q stderr=%q)", stdout, stderr)
	}
	if !strings.Contains(err.Error(), "trusted WeWork issuer") {
		t.Fatalf("auth import returned an unactionable issuer error: %v", err)
	}
}

func TestAuthWhoamiReportsRenewableEnvironmentSession(t *testing.T) {
	isolateAuthCLI(t)
	t.Setenv("WEWORK_TOKEN", authTestJWT(`{"iss":"https://idp.wework.com/","azp":"public-client","exp":4102444800}`))
	t.Setenv("WEWORK_REFRESH_TOKEN", "refresh-from-env")
	t.Setenv("WEWORK_UUID", "account-env")
	t.Setenv("WEWORK_MEMBER_TYPE", "3")

	stdout, stderr, err := executeAuthCLI(t, "", "auth", "whoami", "--json")
	if err != nil {
		t.Fatalf("auth whoami failed: %v (stderr=%q)", err, stderr)
	}
	status := decodeAuthJSON(t, stdout)
	if status["refresh_token"] != true || status["renewable"] != true || status["ready"] != true {
		t.Fatalf("environment session should be renewable: %#v", status)
	}
}

func TestAuthWhoamiDistinguishesRefreshableFromRequestReady(t *testing.T) {
	isolateAuthCLI(t)
	t.Setenv("WEWORK_TOKEN", authTestJWT(`{"iss":"https://idp.wework.com/","azp":"public-client","exp":1}`))
	t.Setenv("WEWORK_REFRESH_TOKEN", "refresh-from-env")
	t.Setenv("WEWORK_UUID", "account-env")
	t.Setenv("WEWORK_MEMBER_TYPE", "3")

	stdout, stderr, err := executeAuthCLI(t, "", "auth", "whoami", "--json")
	if err != nil {
		t.Fatalf("auth whoami failed: %v (stderr=%q)", err, stderr)
	}
	status := decodeAuthJSON(t, stdout)
	if status["request_ready"] != false || status["headless_ready"] != true || status["refresh_required"] != true {
		t.Fatalf("expired renewable session status is ambiguous: %#v", status)
	}
}

func TestAuthRefreshIsDiscoverable(t *testing.T) {
	isolateAuthCLI(t)
	stdout, stderr, err := executeAuthCLI(t, "", "auth", "refresh", "--help")
	if err != nil {
		t.Fatalf("auth refresh --help failed: %v (stderr=%q)", err, stderr)
	}
	if !strings.Contains(stdout, "--force") {
		t.Fatalf("auth refresh help must describe --force: %q", stdout)
	}
}

func TestLiveClientConstructionSurfacesRequiredRefreshFailure(t *testing.T) {
	isolateAuthCLI(t)
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("load isolated config: %v", err)
	}
	expired := authTestJWT(`{"iss":"https://evil.example.com/","azp":"public-client","exp":1}`)
	if err := cfg.SaveWeworkAuth(expired, "refresh-1", "account-1", "3"); err != nil {
		t.Fatalf("seed isolated auth: %v", err)
	}
	flags := &rootFlags{}
	if _, err := flags.newClient(); err == nil {
		t.Fatal("live client construction swallowed a required refresh failure")
	} else if !strings.Contains(err.Error(), "refusing to refresh") {
		t.Fatalf("live client construction returned an unactionable refresh error: %v", err)
	}
}

func TestAuthLoginWithoutFlagsIsBrowserFree(t *testing.T) {
	isolateAuthCLI(t)
	help, helpErr, err := executeAuthCLI(t, "", "auth", "login", "--help")
	if err != nil {
		t.Fatalf("auth login --help failed: %v (stderr=%q)", err, helpErr)
	}
	if strings.Contains(help, "--chrome") && strings.Contains(help, "(default true)") {
		t.Fatalf("auth login must not default to browser access: %q", help)
	}
	stdout, stderr, err := executeAuthCLI(t, "", "auth", "login")
	if err != nil {
		t.Fatalf("auth login without flags failed: %v (stdout=%q stderr=%q)", err, stdout, stderr)
	}
	for _, want := range []string{"auth session-import --stdin", "auth handoff"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("auth login without flags must explain the headless bootstrap path; missing %q in %q", want, stdout)
		}
	}
}

func TestAuthHandoffDescribesRemoteWindowlessWorkflow(t *testing.T) {
	isolateAuthCLI(t)
	stdout, stderr, err := executeAuthCLI(t, "", "auth", "handoff", "--ssh-target", "agent@booking-host", "--json")
	if err != nil {
		t.Fatalf("auth handoff failed: %v (stderr=%q)", err, stderr)
	}
	handoff := decodeAuthJSON(t, stdout)
	if handoff["login_url"] != "https://members.wework.com" {
		t.Fatalf("handoff missing WeWork login URL: %#v", handoff)
	}
	if handoff["automatic_callback"] != false || handoff["device_flow_available"] != false {
		t.Fatalf("handoff must not claim unsupported device callback: %#v", handoff)
	}
	if script, _ := handoff["capture_script"].(string); !strings.Contains(script, "refreshToken") {
		t.Fatalf("capture script must include refresh token: %#v", handoff)
	}
	if command, _ := handoff["import_command"].(string); !strings.Contains(command, "ssh agent@booking-host") || !strings.Contains(command, "auth session-import --stdin") {
		t.Fatalf("handoff missing stdin-over-SSH import: %#v", handoff)
	}
}

func TestAuthHandoffRejectsUnsafeSSHTarget(t *testing.T) {
	isolateAuthCLI(t)
	_, _, err := executeAuthCLI(t, "", "auth", "handoff", "--ssh-target", "host;echo-pwned")
	if err == nil {
		t.Fatal("auth handoff accepted a shell-injectable SSH target")
	}
}

func installFakeSSH(t *testing.T, output string, exitCode int) (argsPath, stdinPath string) {
	t.Helper()
	dir := t.TempDir()
	argsPath = filepath.Join(dir, "args")
	stdinPath = filepath.Join(dir, "stdin")
	script := `#!/bin/sh
printf '%s\n' "$@" > "$FAKE_SSH_ARGS"
cat > "$FAKE_SSH_STDIN"
printf '%s\n' "$FAKE_SSH_OUTPUT"
exit "$FAKE_SSH_EXIT"
`
	sshPath := filepath.Join(dir, "ssh")
	if err := os.WriteFile(sshPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake ssh: %v", err)
	}
	t.Setenv("FAKE_SSH_ARGS", argsPath)
	t.Setenv("FAKE_SSH_STDIN", stdinPath)
	t.Setenv("FAKE_SSH_OUTPUT", output)
	t.Setenv("FAKE_SSH_EXIT", fmt.Sprintf("%d", exitCode))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsPath, stdinPath
}

func seedRenewableAuth(t *testing.T) (token, refresh string) {
	t.Helper()
	token = authTestJWT(`{"iss":"https://idp.wework.com/","azp":"public-client","exp":4102444800}`)
	refresh = "refresh-secret-1"
	bundle, err := json.Marshal(map[string]string{
		"token": token, "refreshToken": refresh, "uuid": "account-secret-1", "memberType": "3",
	})
	if err != nil {
		t.Fatalf("marshal auth fixture: %v", err)
	}
	if stdout, stderr, err := executeAuthCLI(t, string(bundle), "auth", "session-import", "--stdin", "--json"); err != nil {
		t.Fatalf("seed auth import failed: %v (stdout=%q stderr=%q)", err, stdout, stderr)
	}
	return token, refresh
}

func TestAuthPushTransfersCredentialsOnlyOverSSHStdinAndVerifiesRemote(t *testing.T) {
	isolateAuthCLI(t)
	token, refresh := seedRenewableAuth(t)
	remoteStatus := `{"token":true,"refresh_token":true,"renewable":true,"uuid":true,"member_type":true,"request_ready":true,"headless_ready":true,"unexpected":"refresh-secret-1"}`
	argsPath, stdinPath := installFakeSSH(t, remoteStatus, 0)

	stdout, stderr, err := executeAuthCLI(t, "", "auth", "push", "--ssh-target", "agent@booking-host", "--json")
	if err != nil {
		t.Fatalf("auth push failed: %v (stdout=%q stderr=%q)", err, stdout, stderr)
	}
	result := decodeAuthJSON(t, stdout)
	if result["pushed"] != true || result["remote_verified"] != true || result["ssh_target"] != "agent@booking-host" {
		t.Fatalf("auth push returned unexpected result: %#v", result)
	}

	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read fake ssh args: %v", err)
	}
	for _, secret := range []string{token, refresh, "account-secret-1"} {
		if strings.Contains(string(args), secret) || strings.Contains(stdout, secret) || strings.Contains(stderr, secret) {
			t.Fatalf("auth push exposed a credential outside SSH stdin")
		}
	}
	if !strings.Contains(string(args), "agent@booking-host") || !strings.Contains(string(args), "auth session-import --stdin") || !strings.Contains(string(args), "auth refresh --force") {
		t.Fatalf("auth push did not request remote import and refresh verification: %q", args)
	}

	stdin, err := os.ReadFile(stdinPath)
	if err != nil {
		t.Fatalf("read fake ssh stdin: %v", err)
	}
	gotToken, gotRefresh, gotUUID, gotMember, err := parseComposedAuthJSON(stdin)
	if err != nil {
		t.Fatalf("parse pushed credential bundle: %v", err)
	}
	if gotToken != token || gotRefresh != refresh || gotUUID != "account-secret-1" || gotMember != "3" {
		t.Fatalf("SSH stdin did not contain the complete renewable bundle")
	}
}

func TestAuthPushRejectsUnsafeSSHTargetBeforeExecutingSSH(t *testing.T) {
	isolateAuthCLI(t)
	seedRenewableAuth(t)
	argsPath, _ := installFakeSSH(t, `{}`, 0)
	_, _, err := executeAuthCLI(t, "", "auth", "push", "--ssh-target", "host;echo-pwned")
	if err == nil {
		t.Fatal("auth push accepted a shell-injectable SSH target")
	}
	if _, statErr := os.Stat(argsPath); !os.IsNotExist(statErr) {
		t.Fatalf("auth push executed SSH before rejecting target: %v", statErr)
	}
}

func TestAuthPushRequiresCompleteRenewableLocalSession(t *testing.T) {
	isolateAuthCLI(t)
	installFakeSSH(t, `{}`, 0)
	_, _, err := executeAuthCLI(t, "", "auth", "push", "--ssh-target", "booking-host")
	if err == nil || !strings.Contains(err.Error(), "complete renewable local session") {
		t.Fatalf("auth push should reject missing local credentials with an actionable error: %v", err)
	}
}

func TestAuthPushFailsWhenRemoteSessionIsNotHeadlessReady(t *testing.T) {
	isolateAuthCLI(t)
	seedRenewableAuth(t)
	remoteStatus := `{"token":true,"refresh_token":true,"renewable":false,"uuid":true,"member_type":true,"headless_ready":false}`
	installFakeSSH(t, remoteStatus, 0)
	_, _, err := executeAuthCLI(t, "", "auth", "push", "--ssh-target", "booking-host")
	if err == nil || !strings.Contains(err.Error(), "remote session did not verify") {
		t.Fatalf("auth push should fail closed on an unrenewable remote status: %v", err)
	}
}

func TestAuthPushSuppressesRemoteOutputOnSSHFailure(t *testing.T) {
	isolateAuthCLI(t)
	_, refresh := seedRenewableAuth(t)
	installFakeSSH(t, "remote-echo-"+refresh, 23)
	stdout, stderr, err := executeAuthCLI(t, "", "auth", "push", "--ssh-target", "booking-host")
	if err == nil || !strings.Contains(err.Error(), "remote auth push failed via SSH") {
		t.Fatalf("auth push should surface the SSH failure: %v", err)
	}
	if strings.Contains(stdout, refresh) || strings.Contains(stderr, refresh) || strings.Contains(err.Error(), refresh) {
		t.Fatal("auth push reflected untrusted remote output containing a credential")
	}
}
