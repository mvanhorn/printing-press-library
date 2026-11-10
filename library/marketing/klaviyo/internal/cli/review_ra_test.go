package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/spf13/cobra"
)

func raCLIEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		t.Setenv(name, t.TempDir())
	}
	for _, name := range []string{mcpBoundProfileEnv, "PRINTING_PRESS_CLIENT_PROFILE", "KLAVIYO_CONFIG", "KLAVIYO_API_KEY", "KLAVIYO_BASE_URL"} {
		t.Setenv(name, "")
	}
	registration, resolver := registeredPlatformSource, platformResolverFactory
	t.Cleanup(func() { registeredPlatformSource, platformResolverFactory = registration, resolver })
	registeredPlatformSource = platformCLITestRegistration(platformCLIAdapter{observed: platform.ObservedIdentity{AccountID: "acct-a"}})
	platformResolverFactory = func() platform.CredentialResolver { return platformCLIResolver{value: []byte("fixture")} }
	if err := platform.SaveProfile(&platform.Profile{SchemaVersion: platform.ProfileSchemaVersion, Name: "tenant-a", Sources: map[string]platform.SourceProfile{
		"test-source": {CredentialRef: "op://Test/Fixture/token", ExpectedAccountID: "acct-a"},
	}}); err != nil {
		t.Fatal(err)
	}
}

func raExecute(t *testing.T, args ...string) error {
	t.Helper()
	flags := &rootFlags{}
	root := newRootCmd(flags)
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		err = finalizeDelivery(flags)
	}
	finalizePlatformInvocation(flags, &err)
	return err
}

func TestRABoundMCPRejectsDestinationOverrides(t *testing.T) {
	raCLIEnvironment(t)
	t.Setenv(mcpBoundProfileEnv, "tenant-a")
	for _, tc := range []struct {
		flag, value, want string
	}{
		{"--receipt-file", filepath.Join(t.TempDir(), "receipt.json"), "MCP tools may not override --receipt-file"},
		{"--receipt-file", "", "MCP tools may not override --receipt-file"},
		{"--audit-dir", t.TempDir(), "MCP tools may not override --audit-dir"},
		{"--audit-dir", "", "MCP tools may not override --audit-dir"},
		{"--deliver", "file:" + filepath.Join(t.TempDir(), "output.json"), "MCP tools may not use --deliver file: or webhook:"},
		{"--deliver", "webhook:http://127.0.0.1:1", "MCP tools may not use --deliver file: or webhook:"},
	} {
		t.Run(tc.flag+tc.value, func(t *testing.T) {
			if err := raExecute(t, "whoami", tc.flag+"="+tc.value); err == nil || err.Error() != tc.want {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	if err := raExecute(t, "whoami", "--deliver=stdout"); err != nil {
		t.Fatal(err)
	}
	if err := raExecute(t, "whoami", "--receipt"); err != nil {
		t.Fatalf("default profile receipt was blocked: %v", err)
	}
}

func TestRAProtectedOutputDestinations(t *testing.T) {
	raCLIEnvironment(t)
	paths, err := platform.PathsFor("tenant-b", "klaviyo-pp-cli", "test-source")
	if err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(t.TempDir(), "legacy-credentials.toml")
	t.Setenv("KLAVIYO_CONFIG", custom)
	protected := []string{
		paths.ProfileFile, paths.ConfigFile, paths.DataFile, paths.DataFile + "-wal", custom,
		filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "klaviyo-pp-cli", "config.toml"),
	}
	for _, target := range protected {
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("preserve fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
		alias := filepath.Join(t.TempDir(), "alias")
		if err := os.Symlink(target, alias); err != nil {
			t.Fatal(err)
		}
		for _, destination := range []string{target, alias} {
			if err := deliverFile(destination, []byte("overwrite")); err == nil || err.Error() != "refusing output destination: configuration, credentials, and profile databases are protected" {
				t.Errorf("deliver %s: %v", destination, err)
			}
			flags := &rootFlags{receiptFile: destination, platformSession: platformCommandTestSession(t)}
			root := newRootCmd(flags)
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			// newRootCmd initializes flags, so set the requested destination afterwards.
			flags.receiptFile = destination
			err := initializePlatformReceipt(root, flags)
			if err == nil || err.Error() != "refusing output destination: configuration, credentials, and profile databases are protected" {
				t.Errorf("receipt %s: %v", destination, err)
			}
			finalizePlatformInvocation(flags, &err)
		}
		data, err := os.ReadFile(target)
		if err != nil || string(data) != "preserve fixture" {
			t.Errorf("protected file changed: %s: %q, %v", target, data, err)
		}
	}
	// Resolve existing symlink parents even when the output itself does not exist.
	alias := filepath.Join(t.TempDir(), "config-alias")
	if err := os.Symlink(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "printing-press"), alias); err != nil {
		t.Fatal(err)
	}
	if err := deliverFile(filepath.Join(alias, "new", "receipt.json"), nil); err == nil {
		t.Fatal("allowed a missing destination through a protected symlink parent")
	}
	caseAlias := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "PRINTING-PRESS")
	if _, err := os.Stat(caseAlias); err == nil {
		if err := deliverFile(filepath.Join(caseAlias, "new", "receipt.json"), nil); err == nil {
			t.Fatal("allowed a case alias of protected configuration")
		}
	}
	if _, err := os.Stat(strings.ToUpper(custom)); err == nil {
		if err := deliverFile(strings.ToUpper(custom), nil); err == nil {
			t.Fatal("allowed a case alias of a credential file")
		}
	}
	dangling := filepath.Join(t.TempDir(), "dangling")
	if err := os.Symlink(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "printing-press", "missing.json"), dangling); err != nil {
		t.Fatal(err)
	}
	if err := deliverFile(dangling, nil); err == nil {
		t.Fatal("allowed a dangling symlink into protected configuration")
	}
}

func TestRAUnboundDeliveryAndReceiptStillWork(t *testing.T) {
	raCLIEnvironment(t)
	output := filepath.Join(t.TempDir(), "out.json")
	if err := deliverFile(output, []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != `{"ok":true}` {
		t.Fatalf("delivery = %q, %v", data, err)
	}
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	if err := raExecute(t, "whoami", "--client-profile=tenant-a", "--receipt-file="+receipt); err != nil {
		t.Fatalf("unbound receipt: %v", err)
	}
	if data, err := os.ReadFile(receipt); err != nil || !bytes.Contains(data, []byte(`"status": "success"`)) {
		t.Fatalf("receipt was not written: %v", err)
	}
	// A bound server cannot inherit a receipt override through a saved run profile.
	t.Setenv(mcpBoundProfileEnv, "tenant-a")
	if err := saveProfileStore(&profileStore{Profiles: map[string]Profile{"unsafe": {Name: "unsafe", Values: map[string]string{"receipt-file": output}}}}); err != nil {
		t.Fatal(err)
	}
	if err := raExecute(t, "whoami", "--profile=unsafe"); err == nil || err.Error() != "MCP tools may not override --receipt-file" {
		t.Fatalf("saved profile override error = %v", err)
	}
}

func TestRAReceiptRechecksSymlinksAtFinalization(t *testing.T) {
	raCLIEnvironment(t)
	alias := filepath.Join(t.TempDir(), "output")
	if err := os.Symlink(t.TempDir(), alias); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{platformSession: platformCommandTestSession(t)}
	root := newRootCmd(flags)
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	flags.receiptFile = filepath.Join(alias, "receipt.json")
	if err := initializePlatformReceipt(root, flags); err != nil {
		t.Fatal(err)
	}
	// Simulate the output directory changing after command initialization.
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "printing-press")
	if err := os.Symlink(protected, alias); err != nil {
		t.Fatal(err)
	}
	var err error
	finalizePlatformInvocation(flags, &err)
	if err == nil || err.Error() != "refusing output destination: configuration, credentials, and profile databases are protected" {
		t.Fatalf("finalization error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(protected, "receipt.json")); !os.IsNotExist(err) {
		t.Fatalf("receipt reached protected configuration: %v", err)
	}
}

func TestRADeliveryDoesNotFollowPredictableTempSymlink(t *testing.T) {
	raCLIEnvironment(t)
	protected, err := platform.ProfilePath("tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(protected)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "out.json")
	if err := os.Symlink(protected, output+".tmp"); err != nil {
		t.Fatal(err)
	}
	if err := deliverFile(output, []byte("fixture")); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(protected)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("delivery followed the predictable temporary-file symlink")
	}
}

type raDeniedIdentityAdapter struct{}

func (raDeniedIdentityAdapter) EndpointClass() string { return "GET /identity" }
func (raDeniedIdentityAdapter) ProbeIdentity(context.Context, platform.ResolvedCredentials, platform.SourceProfile) (platform.ObservedIdentity, error) {
	return platform.ObservedIdentity{}, &platform.ProbeFailure{Outcome: platform.GateInsufficientScope, Err: errors.New("fixture probe denied")}
}

func TestRAReceiptResolutionErrorsStayInFinalization(t *testing.T) {
	for _, name := range []string{"success", "gate failure"} {
		t.Run(name, func(t *testing.T) {
			raCLIEnvironment(t)
			if name == "gate failure" {
				registeredPlatformSource.Adapter = raDeniedIdentityAdapter{}
			}
			blocker := filepath.Join(t.TempDir(), "not-a-directory")
			if err := os.WriteFile(blocker, []byte("fixture"), 0o600); err != nil {
				t.Fatal(err)
			}
			mkdirErr := os.MkdirAll(blocker, 0o700)
			if mkdirErr == nil {
				t.Fatal("receipt directory was not blocked")
			}
			stdout, err := os.CreateTemp(t.TempDir(), "stdout-*")
			if err != nil {
				t.Fatal(err)
			}
			stderr, err := os.CreateTemp(t.TempDir(), "stderr-*")
			if err != nil {
				stdout.Close()
				t.Fatal(err)
			}
			originalArgs, originalOut, originalErr := os.Args, os.Stdout, os.Stderr
			t.Cleanup(func() {
				os.Args, os.Stdout, os.Stderr = originalArgs, originalOut, originalErr
				stdout.Close()
				stderr.Close()
			})
			os.Args = []string{"klaviyo-pp-cli", "whoami", "--client-profile=tenant-a", "--json", "--receipt-file=" + filepath.Join(blocker, "receipt.json")}
			os.Stdout, os.Stderr = stdout, stderr
			commandErr := Execute()
			out, err := os.ReadFile(stdout.Name())
			if err != nil {
				t.Fatal(err)
			}
			diagnostic, err := os.ReadFile(stderr.Name())
			if err != nil {
				t.Fatal(err)
			}
			if name == "success" {
				var envelope struct {
					Data struct{ Gate platform.GateOutcome } `json:"data"`
				}
				if err := json.Unmarshal(out, &envelope); err != nil || envelope.Data.Gate != platform.GateVerified {
					t.Fatalf("command output=%q error=%v", out, err)
				}
				if commandErr == nil || commandErr.Error() != mkdirErr.Error() || string(diagnostic) != "Error: "+mkdirErr.Error()+"\n" {
					t.Fatalf("finalization error=%v diagnostic=%q, want original filesystem error once", commandErr, diagnostic)
				}
			} else {
				want := `client profile "tenant-a" source "test-source" tenant gate insufficient_scope: fixture probe denied`
				if commandErr == nil || commandErr.Error() != want || len(out) != 0 || strings.Count(string(diagnostic), "Error: ") != 1 || !bytes.Contains(diagnostic, []byte("Error: "+want+"\n")) || bytes.Contains(diagnostic, []byte(mkdirErr.Error())) {
					t.Fatalf("gate error=%v stdout=%q stderr=%q, want only the gate failure", commandErr, out, diagnostic)
				}
			}
		})
	}
}

func TestRADeliveryResolutionErrorsStayInFinalization(t *testing.T) {
	raCLIEnvironment(t)
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	mkdirErr := os.MkdirAll(blocker, 0o755)
	if mkdirErr == nil {
		t.Fatal("delivery directory was not blocked")
	}
	flags := &rootFlags{deliverSpec: "file:" + filepath.Join(blocker, "output.json")}
	root := &cobra.Command{Use: "fixture-pp-cli"}
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	if err := configureDelivery(root, flags); err != nil {
		t.Fatalf("delivery preflight failed early: %v", err)
	}
	payload := []byte(`{"ok":true}`)
	if _, err := root.OutOrStdout().Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := finalizeDelivery(flags); err == nil || err.Error() != "creating deliver dir: "+mkdirErr.Error() || !bytes.Equal(stdout.Bytes(), payload) {
		t.Fatalf("delivery finalization error=%v stdout=%q", err, stdout.Bytes())
	}
}
