// Package config resolves MasterPark credentials and CLI settings from
// environment variables, a config file, generic command injection, and
// 1Password (via the `op` CLI). Passwords are never written to the config file
// or printed.
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Env var names for credentials.
const (
	EnvUsername = "MASTERPARK_USERNAME"
	EnvPassword = "MASTERPARK_PASSWORD"
)

// CredSource identifies where a credential value came from.
type CredSource string

const (
	SourceNone        CredSource = "none"
	SourceFlag        CredSource = "flag"
	SourceCommand     CredSource = "command"
	SourceEnv         CredSource = "env"
	SourceConfig      CredSource = "config"
	SourceOnePassword CredSource = "1password"
)

// File is the on-disk config. It intentionally has no password field; only
// non-secret metadata is persisted.
type File struct {
	BaseURL  string `json:"base_url,omitempty"`
	Username string `json:"username,omitempty"`
	// OnePassword holds non-secret coordinates used to fetch credentials on
	// demand from the `op` CLI.
	OnePassword *OnePasswordRef `json:"onepassword,omitempty"`
	// UsernameCommand/PasswordCommand optionally hold command strings that print
	// credentials to stdout. This enables generic secret injection such as
	// `op read ...` without the agent seeing the secret value. Do not put literal
	// secrets in these command strings.
	UsernameCommand string `json:"username_command,omitempty"`
	PasswordCommand string `json:"password_command,omitempty"`
	// Profile is non-secret customer/vehicle data returned by verifyLogin and used
	// as defaults for reservation submission.
	Profile *Profile `json:"profile,omitempty"`
}

// OnePasswordRef are non-secret coordinates for an `op` lookup.
type OnePasswordRef struct {
	Vault         string `json:"vault,omitempty"`
	Item          string `json:"item,omitempty"`
	UsernameField string `json:"username_field,omitempty"`
	PasswordField string `json:"password_field,omitempty"`
}

// Profile contains non-secret reservation defaults learned from MasterPark.
type Profile struct {
	FirstName  string           `json:"first_name,omitempty"`
	LastName   string           `json:"last_name,omitempty"`
	Email      string           `json:"email,omitempty"`
	Phone      string           `json:"phone,omitempty"`
	CustomerID string           `json:"customer_id,omitempty"`
	Vehicles   []VehicleProfile `json:"vehicles,omitempty"`
}

// VehicleProfile contains non-secret saved vehicle details from MasterPark.
type VehicleProfile struct {
	License string `json:"license,omitempty"`
	State   string `json:"state,omitempty"`
	Color   string `json:"color,omitempty"`
	Make    string `json:"make,omitempty"`
	Model   string `json:"model,omitempty"`
	Type    string `json:"type,omitempty"`
}

// Vehicle is kept as an alias for older code/tests.
type Vehicle = VehicleProfile

// Credentials holds resolved credentials and where each came from.
type Credentials struct {
	Username       string
	Password       string
	UsernameSource CredSource
	PasswordSource CredSource
}

// CmdRunner runs a generic credential-producing command without a shell.
type CmdRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// CredInput are explicit credential inputs from CLI flags.
type CredInput struct {
	Username        string
	Password        string
	UsernameCommand string
	PasswordCommand string
	CmdRunner       CmdRunner
}

// DefaultCmdRunner runs command directly without shell interpolation. Command
// stdout may be a secret and must not be logged by callers.
func DefaultCmdRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

// DefaultPath returns the default config file location.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(home, ".config", "masterpark-pp-cli", "config.json")
}

// Load reads the config file at path. A missing file yields an empty File.
func Load(path string) (*File, error) {
	if path == "" {
		path = DefaultPath()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{}, nil
		}
		return nil, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &f, nil
}

// Save writes non-secret config to path, creating parent dirs. File deliberately
// has no password field.
func Save(path string, f *File) error {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// OpRunner runs the `op` CLI; overridable in tests.
type OpRunner func(ctx context.Context, args ...string) ([]byte, error)

// DefaultOpRunner shells out to the real `op` binary.
func DefaultOpRunner(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "op", args...)
	return cmd.Output()
}

// OnePassword wraps credential retrieval from 1Password.
type OnePassword struct {
	Runner OpRunner
}

// FetchField reads a single field of an item from a vault. The returned value is
// treated as secret by callers and must not be logged.
func (o OnePassword) FetchField(ctx context.Context, vault, item, field string) (string, error) {
	run := o.Runner
	if run == nil {
		run = DefaultOpRunner
	}
	args := []string{"item", "get", item, "--vault", vault, "--fields", "label=" + field, "--reveal"}
	out, err := run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("op item get %q field %q: %w", item, field, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Resolve merges credentials in priority order:
// explicit flags/commands > env > config command refs > config username >
// 1Password ref. Passwords are only held in memory.
func Resolve(ctx context.Context, f *File, op OnePassword, inputs ...CredInput) (Credentials, error) {
	input := CredInput{}
	if len(inputs) > 0 {
		input = inputs[0]
	}
	var c Credentials

	if input.Username != "" {
		c.Username, c.UsernameSource = input.Username, SourceFlag
	} else if input.UsernameCommand != "" {
		u, err := runCredentialCommand(ctx, input, input.UsernameCommand)
		if err != nil {
			return c, fmt.Errorf("username command: %w", err)
		}
		c.Username, c.UsernameSource = u, SourceCommand
	} else if v := os.Getenv(EnvUsername); v != "" {
		c.Username, c.UsernameSource = v, SourceEnv
	} else if f != nil && f.UsernameCommand != "" {
		u, err := runCredentialCommand(ctx, input, f.UsernameCommand)
		if err != nil {
			return c, fmt.Errorf("configured username command: %w", err)
		}
		c.Username, c.UsernameSource = u, SourceCommand
	} else if f != nil && f.Username != "" {
		c.Username, c.UsernameSource = f.Username, SourceConfig
	}

	if input.Password != "" {
		c.Password, c.PasswordSource = input.Password, SourceFlag
	} else if input.PasswordCommand != "" {
		p, err := runCredentialCommand(ctx, input, input.PasswordCommand)
		if err != nil {
			return c, fmt.Errorf("password command: %w", err)
		}
		c.Password, c.PasswordSource = p, SourceCommand
	} else if v := os.Getenv(EnvPassword); v != "" {
		c.Password, c.PasswordSource = v, SourceEnv
	} else if f != nil && f.PasswordCommand != "" {
		p, err := runCredentialCommand(ctx, input, f.PasswordCommand)
		if err != nil {
			return c, fmt.Errorf("configured password command: %w", err)
		}
		c.Password, c.PasswordSource = p, SourceCommand
	}

	if f != nil && f.OnePassword != nil {
		ref := f.OnePassword
		if c.Username == "" && ref.UsernameField != "" {
			u, err := op.FetchField(ctx, ref.Vault, ref.Item, ref.UsernameField)
			if err != nil {
				return c, err
			}
			if u != "" {
				c.Username, c.UsernameSource = u, SourceOnePassword
			}
		}
		if c.Password == "" && ref.PasswordField != "" {
			p, err := op.FetchField(ctx, ref.Vault, ref.Item, ref.PasswordField)
			if err != nil {
				return c, err
			}
			if p != "" {
				c.Password, c.PasswordSource = p, SourceOnePassword
			}
		}
	}

	if c.UsernameSource == "" {
		c.UsernameSource = SourceNone
	}
	if c.PasswordSource == "" {
		c.PasswordSource = SourceNone
	}
	return c, nil
}

func runCredentialCommand(ctx context.Context, input CredInput, command string) (string, error) {
	argv, err := splitCommand(command)
	if err != nil {
		return "", err
	}
	runner := input.CmdRunner
	if runner == nil {
		runner = DefaultCmdRunner
	}
	out, err := runner(ctx, argv[0], argv[1:]...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// splitCommand splits a simple shell-like command string into argv without
// invoking a shell. It supports single/double quotes and backslash escaping.
func splitCommand(command string) ([]string, error) {
	var args []string
	var b strings.Builder
	var quote rune
	escaped := false
	for _, r := range command {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case ' ', '\t', '\n', '\r':
			if b.Len() > 0 {
				args = append(args, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if escaped {
		return nil, fmt.Errorf("command ends with dangling escape")
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in command")
	}
	if b.Len() > 0 {
		args = append(args, b.String())
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	return args, nil
}

// ProfileFromLoginData extracts a non-secret customer profile + vehicles from a
// verifyLogin response payload. It is tolerant of nested (`customer`) or flat
// shapes and common key aliases. It never extracts a password.
func ProfileFromLoginData(data json.RawMessage) *Profile {
	if len(data) == 0 {
		return nil
	}
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	cust := root
	if c, ok := root["customer"].(map[string]interface{}); ok {
		cust = c
	}
	p := &Profile{
		FirstName:  profileString(cust, "first_name", "firstName", "fname", "first"),
		LastName:   profileString(cust, "last_name", "lastName", "lname", "last"),
		Email:      profileString(cust, "email", "email_address"),
		Phone:      profileString(cust, "cell_phone", "phone", "phone_number", "mobile"),
		CustomerID: profileString(cust, "id", "customer", "customer_id", "customerId", "customerID"),
	}
	var rawVehicles []interface{}
	if vs, ok := root["vehicles"].([]interface{}); ok {
		rawVehicles = vs
	} else if vs, ok := cust["vehicles"].([]interface{}); ok {
		rawVehicles = vs
	}
	for _, rv := range rawVehicles {
		vm, ok := rv.(map[string]interface{})
		if !ok {
			continue
		}
		p.Vehicles = append(p.Vehicles, VehicleProfile{
			Make:    profileString(vm, "make", "vehicle_make"),
			Model:   profileString(vm, "model", "vehicle_model"),
			Color:   profileString(vm, "color", "vehicle_color"),
			License: profileString(vm, "license", "plate", "licensePlate", "license_plate", "vehicle_license"),
			State:   profileString(vm, "state", "license_state", "plate_state"),
			Type:    profileString(vm, "type", "vehicle_type"),
		})
	}
	if p.FirstName == "" && p.LastName == "" && p.Email == "" && p.Phone == "" && len(p.Vehicles) == 0 {
		return nil
	}
	return p
}

func profileString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch s := v.(type) {
			case string:
				if s != "" {
					return s
				}
			case float64:
				return strconv.FormatFloat(s, 'f', 0, 64)
			case json.Number:
				return s.String()
			}
		}
	}
	return ""
}
