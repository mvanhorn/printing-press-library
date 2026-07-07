package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOnePasswordFetchField(t *testing.T) {
	var gotArgs []string
	op := OnePassword{
		Runner: func(ctx context.Context, args ...string) ([]byte, error) {
			gotArgs = args
			// Simulate `op` returning the field value for the requested label.
			for i, a := range args {
				if a == "--fields" && i+1 < len(args) {
					switch args[i+1] {
					case "label=username":
						return []byte("alice@example.com\n"), nil
					case "label=password":
						return []byte("s3cr3t\n"), nil
					}
				}
			}
			return nil, fmt.Errorf("unknown field")
		},
	}

	u, err := op.FetchField(context.Background(), "Agent", "Masterparking", "username")
	if err != nil {
		t.Fatalf("FetchField username: %v", err)
	}
	if u != "alice@example.com" {
		t.Errorf("username = %q", u)
	}
	// Verify op invocation shape.
	want := []string{"item", "get", "Masterparking", "--vault", "Agent", "--fields", "label=username", "--reveal"}
	if len(gotArgs) != len(want) {
		t.Fatalf("args = %v", gotArgs)
	}
	for i := range want {
		if gotArgs[i] != want[i] {
			t.Errorf("arg[%d] = %q, want %q", i, gotArgs[i], want[i])
		}
	}

	p, err := op.FetchField(context.Background(), "Agent", "Masterparking", "password")
	if err != nil || p != "s3cr3t" {
		t.Fatalf("password = %q err=%v", p, err)
	}
}

func TestResolveUsernameOnePasswordErrorPropagates(t *testing.T) {
	os.Unsetenv(EnvUsername)
	os.Unsetenv(EnvPassword)

	f := &File{
		OnePassword: &OnePasswordRef{
			Vault: "Agent", Item: "Masterparking",
			UsernameField: "username", PasswordField: "password",
		},
	}
	op := OnePassword{
		Runner: func(ctx context.Context, args ...string) ([]byte, error) {
			for i, a := range args {
				if a == "--fields" && i+1 < len(args) && args[i+1] == "label=username" {
					return nil, fmt.Errorf("op item get failed")
				}
			}
			return []byte("op-password"), nil
		},
	}
	creds, err := Resolve(context.Background(), f, op, CredInput{})
	if err == nil {
		t.Fatalf("expected error from username 1Password lookup, got creds=%+v", creds)
	}
	if creds.Username != "" || creds.UsernameSource == SourceOnePassword {
		t.Errorf("username must not be silently set on lookup error: %q (%s)", creds.Username, creds.UsernameSource)
	}
}

func TestResolvePriorityAndSources(t *testing.T) {
	os.Setenv(EnvUsername, "envuser")
	defer os.Unsetenv(EnvUsername)
	os.Unsetenv(EnvPassword)

	f := &File{
		Username: "configuser",
		OnePassword: &OnePasswordRef{
			Vault: "Agent", Item: "Masterparking",
			UsernameField: "username", PasswordField: "password",
		},
	}
	op := OnePassword{
		Runner: func(ctx context.Context, args ...string) ([]byte, error) {
			return []byte("op-password"), nil
		},
	}
	creds, err := Resolve(context.Background(), f, op, CredInput{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if creds.Username != "envuser" || creds.UsernameSource != SourceEnv {
		t.Errorf("username = %q (%s), want envuser/env", creds.Username, creds.UsernameSource)
	}
	if creds.Password != "op-password" || creds.PasswordSource != SourceOnePassword {
		t.Errorf("password source = %s, want 1password", creds.PasswordSource)
	}
}

func TestResolveExplicitFlagsBeatEverything(t *testing.T) {
	os.Setenv(EnvUsername, "envuser")
	os.Setenv(EnvPassword, "envpass")
	defer os.Unsetenv(EnvUsername)
	defer os.Unsetenv(EnvPassword)

	f := &File{Username: "configuser"}
	creds, err := Resolve(context.Background(), f, OnePassword{}, CredInput{
		Username: "flaguser",
		Password: "flagpass",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if creds.Username != "flaguser" || creds.UsernameSource != SourceFlag {
		t.Errorf("username = %q (%s), want flaguser/flag", creds.Username, creds.UsernameSource)
	}
	if creds.Password != "flagpass" || creds.PasswordSource != SourceFlag {
		t.Errorf("password = %q (%s), want flagpass/flag", creds.Password, creds.PasswordSource)
	}
}

func TestResolveCredCommandsBeatEnv(t *testing.T) {
	os.Setenv(EnvUsername, "envuser")
	os.Setenv(EnvPassword, "envpass")
	defer os.Unsetenv(EnvUsername)
	defer os.Unsetenv(EnvPassword)

	var gotCmds [][]string
	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		gotCmds = append(gotCmds, append([]string{name}, args...))
		switch {
		case name == "op" && len(args) > 1 && args[1] == "op://Agent/Masterparking/username":
			return []byte("cmduser\n"), nil
		case name == "op" && len(args) > 1 && args[1] == "op://Agent/Masterparking/password":
			return []byte("cmdpass\n"), nil
		}
		return nil, fmt.Errorf("unexpected command: %v %v", name, args)
	}

	creds, err := Resolve(context.Background(), &File{}, OnePassword{}, CredInput{
		UsernameCommand: "op read op://Agent/Masterparking/username",
		PasswordCommand: "op read op://Agent/Masterparking/password",
		CmdRunner:       runner,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if creds.Username != "cmduser" || creds.UsernameSource != SourceCommand {
		t.Errorf("username = %q (%s), want cmduser/command", creds.Username, creds.UsernameSource)
	}
	if creds.Password != "cmdpass" || creds.PasswordSource != SourceCommand {
		t.Errorf("password = %q (%s), want cmdpass/command", creds.Password, creds.PasswordSource)
	}
	if len(gotCmds) != 2 {
		t.Fatalf("expected 2 command invocations, got %v", gotCmds)
	}
	want := []string{"op", "read", "op://Agent/Masterparking/username"}
	for i := range want {
		if gotCmds[0][i] != want[i] {
			t.Errorf("username command argv = %v, want %v", gotCmds[0], want)
		}
	}
}

func TestSplitCommandQuoting(t *testing.T) {
	argv, err := splitCommand(`op item get "Master Parking" --fields label=password`)
	if err != nil {
		t.Fatalf("splitCommand: %v", err)
	}
	want := []string{"op", "item", "get", "Master Parking", "--fields", "label=password"}
	if len(argv) != len(want) {
		t.Fatalf("argv = %v, want %v", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Errorf("argv[%d] = %q, want %q", i, argv[i], want[i])
		}
	}
}

func TestProfileRoundTripNoPassword(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	f := &File{
		Username: "alice",
		Profile: &Profile{
			FirstName:  "Alice",
			LastName:   "Smith",
			Email:      "alice@example.com",
			Phone:      "phone-test",
			CustomerID: "C123",
			Vehicles: []VehicleProfile{
				{Make: "Honda", Model: "Civic", Color: "Blue", License: "ABC123", State: "WA", Type: "standard"},
			},
		},
	}
	if err := Save(path, f); err != nil {
		t.Fatalf("Save: %v", err)
	}
	b, _ := os.ReadFile(path)
	if strings.Contains(strings.ToLower(string(b)), "password") {
		t.Errorf("config file must not contain a password field: %s", b)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Profile == nil || len(loaded.Profile.Vehicles) != 1 {
		t.Fatalf("profile round-trip lost data: %+v", loaded.Profile)
	}
	if loaded.Profile.Vehicles[0].Make != "Honda" || loaded.Profile.FirstName != "Alice" {
		t.Errorf("profile mismatch: %+v", loaded.Profile)
	}
}

func TestSaveNeverWritesPassword(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	f := &File{
		Username:    "alice",
		OnePassword: &OnePasswordRef{Vault: "Agent", Item: "Masterparking", PasswordField: "password"},
	}
	if err := Save(path, f); err != nil {
		t.Fatalf("Save: %v", err)
	}
	b, _ := os.ReadFile(path)
	if string(b) == "" {
		t.Fatal("empty config written")
	}
	// The File type has no password field, so a password can never be persisted.
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Username != "alice" || loaded.OnePassword == nil {
		t.Errorf("round-trip mismatch: %+v", loaded)
	}
}
