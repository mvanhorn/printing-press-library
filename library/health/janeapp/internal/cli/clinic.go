// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Jane is multi-tenant. Each clinic is its own subdomain with a
// separate patient account, so a single global base_url/session can't model it.
// This file adds a clinic profile store (name -> {base_url, username, session})
// on top of the generated single-tenant config. The active clinic overrides
// cfg.BaseURL and seeds its session cookie at client-build time (see root.go
// applyActiveClinic / newClientForClinic).

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/janeapp/internal/cliutil"
	"github.com/spf13/cobra"
)

// Clinic is one Jane tenant: a clinic's subdomain plus the patient session for
// it. Session is a cookie-jar string ("_front_desk_session=<value>") suitable
// for seeding the HTTP client's jar via SeedCookieJar.
type Clinic struct {
	Name     string `json:"name"`
	BaseURL  string `json:"base_url"`
	Username string `json:"username,omitempty"`
	Session  string `json:"session,omitempty"`
}

type clinicStore struct {
	Current string            `json:"current"`
	Clinics map[string]Clinic `json:"clinics"`
}

func clinicStorePath() (string, error) {
	dir, err := cliutil.ConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating clinic config dir: %w", err)
	}
	return filepath.Join(dir, "clinics.json"), nil
}

func loadClinicStore() (*clinicStore, error) {
	p, err := clinicStorePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &clinicStore{Clinics: map[string]Clinic{}}, nil
		}
		return nil, fmt.Errorf("reading clinics: %w", err)
	}
	var s clinicStore
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing clinics: %w", err)
	}
	if s.Clinics == nil {
		s.Clinics = map[string]Clinic{}
	}
	return &s, nil
}

func saveClinicStore(s *clinicStore) error {
	p, err := clinicStorePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling clinics: %w", err)
	}
	// 0600: the store holds session cookies (credential material).
	return cliutil.AtomicWritePrivateFile(p, data, 0o600, 0o700)
}

// normalizeClinicURL accepts "embophysio", "embophysio.janeapp.com", or a full
// URL and returns a clean scheme+host base URL (no trailing slash, no path).
func normalizeClinicURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("clinic URL is empty")
	}
	// Bare subdomain -> full janeapp.com host.
	if !strings.Contains(raw, ".") && !strings.Contains(raw, "/") {
		raw = raw + ".janeapp.com"
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid clinic URL %q: %w", raw, err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid clinic URL %q: no host", raw)
	}
	u.Scheme = "https"
	return "https://" + u.Host, nil
}

// resolveActiveClinic picks the clinic a command should target. Precedence:
// explicit --clinic flag, then the stored current clinic, then (if exactly one
// clinic is configured) that clinic. Returns (nil, nil) when no clinic is
// configured and none was requested — callers that require one use
// requireActiveClinic instead.
func resolveActiveClinic(f *rootFlags) (*Clinic, error) {
	s, err := loadClinicStore()
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(f.clinicName)
	if name == "" {
		name = s.Current
	}
	if name == "" {
		if len(s.Clinics) == 1 {
			for _, c := range s.Clinics {
				cc := c
				return &cc, nil
			}
		}
		return nil, nil
	}
	c, ok := s.Clinics[name]
	if !ok {
		return nil, usageErr(fmt.Errorf("clinic %q not found; add it with 'janeapp-pp-cli clinic add %s --url=https://<clinic>.janeapp.com'", name, name))
	}
	return &c, nil
}

// requireActiveClinic is resolveActiveClinic but errors when no clinic can be
// resolved, with guidance on how to add one.
func requireActiveClinic(f *rootFlags) (*Clinic, error) {
	c, err := resolveActiveClinic(f)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, usageErr(fmt.Errorf("no Jane clinic configured. Add one first:\n  janeapp-pp-cli clinic add embophysio --url=https://embophysio.janeapp.com\nThen select it with --clinic <name> or 'janeapp-pp-cli clinic use <name>'"))
	}
	return c, nil
}

// loggedInClinics returns every clinic that has a stored session, sorted by
// name. Used by --all-clinics fan-out commands (agenda, appointments).
func loggedInClinics() ([]Clinic, error) {
	s, err := loadClinicStore()
	if err != nil {
		return nil, err
	}
	out := make([]Clinic, 0, len(s.Clinics))
	for _, c := range s.Clinics {
		if strings.TrimSpace(c.Session) != "" {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// setClinicSession stores the captured session (and username) for a clinic.
func setClinicSession(name, username, session string) error {
	s, err := loadClinicStore()
	if err != nil {
		return err
	}
	c, ok := s.Clinics[name]
	if !ok {
		return fmt.Errorf("clinic %q not found", name)
	}
	if username != "" {
		c.Username = username
	}
	c.Session = session
	s.Clinics[name] = c
	if s.Current == "" {
		s.Current = name
	}
	return saveClinicStore(s)
}

func newClinicCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clinic",
		Short: "Manage your Jane clinics (each is a separate subdomain + login)",
		Long: `Jane is multi-tenant: every clinic is its own subdomain with its own
patient account. Register each clinic you use as a named profile, then log in
to each with 'janeapp-pp-cli auth login --clinic <name>'.

  clinic add <name> --url=https://<clinic>.janeapp.com   register a clinic
  clinic list                                            list registered clinics
  clinic use <name>                                      set the default clinic
  clinic remove <name>                                   forget a clinic

Select which clinic a command targets with --clinic <name>; read commands also
accept --all-clinics to fan out across every logged-in clinic.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newClinicAddCmd(flags))
	cmd.AddCommand(newClinicListCmd(flags))
	cmd.AddCommand(newClinicUseCmd(flags))
	cmd.AddCommand(newClinicRemoveCmd(flags))
	return cmd
}

func newClinicAddCmd(flags *rootFlags) *cobra.Command {
	var urlFlag string
	var usernameFlag string
	cmd := &cobra.Command{
		Use:     "add <name>",
		Short:   "Register a Jane clinic (use --url for the clinic subdomain)",
		Example: "  janeapp-pp-cli clinic add embophysio --url=https://embophysio.janeapp.com",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			name := strings.TrimSpace(args[0])
			if strings.ContainsAny(name, `/\: `) {
				return usageErr(fmt.Errorf("clinic name %q contains reserved characters", name))
			}
			if urlFlag == "" {
				// Allow `clinic add embophysio` with no --url by deriving the
				// subdomain from the name.
				urlFlag = name
			}
			base, err := normalizeClinicURL(urlFlag)
			if err != nil {
				return usageErr(err)
			}
			s, err := loadClinicStore()
			if err != nil {
				return err
			}
			existing := s.Clinics[name]
			existing.Name = name
			existing.BaseURL = base
			if usernameFlag != "" {
				existing.Username = usernameFlag
			}
			s.Clinics[name] = existing
			if s.Current == "" {
				s.Current = name
			}
			if err := saveClinicStore(s); err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), s.Clinics[name], flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added clinic %q (%s).\nLog in with:\n  janeapp-pp-cli auth login --clinic %s\n", name, base, name)
			return nil
		},
	}
	cmd.Flags().StringVar(&urlFlag, "url", "", "Clinic base URL or subdomain (e.g. https://embophysio.janeapp.com or embophysio)")
	cmd.Flags().StringVar(&usernameFlag, "username", "", "Optional: default username/email for this clinic")
	return cmd
}

func newClinicListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List registered Jane clinics",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  janeapp-pp-cli clinic list",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := loadClinicStore()
			if err != nil {
				return err
			}
			names := make([]string, 0, len(s.Clinics))
			for n := range s.Clinics {
				names = append(names, n)
			}
			sort.Strings(names)
			if flags.asJSON {
				out := make([]map[string]any, 0, len(names))
				for _, n := range names {
					c := s.Clinics[n]
					out = append(out, map[string]any{
						"name":       c.Name,
						"base_url":   c.BaseURL,
						"username":   c.Username,
						"logged_in":  strings.TrimSpace(c.Session) != "",
						"is_current": n == s.Current,
					})
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(names) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No clinics registered. Add one:\n  janeapp-pp-cli clinic add embophysio --url=https://embophysio.janeapp.com")
				return nil
			}
			headers := []string{"CURRENT", "NAME", "URL", "LOGGED IN"}
			rows := make([][]string, 0, len(names))
			for _, n := range names {
				c := s.Clinics[n]
				cur := ""
				if n == s.Current {
					cur = "*"
				}
				li := "no"
				if strings.TrimSpace(c.Session) != "" {
					li = "yes"
				}
				rows = append(rows, []string{cur, c.Name, c.BaseURL, li})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
}

func newClinicUseCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "use <name>",
		Short:   "Set the default clinic for commands",
		Example: "  janeapp-pp-cli clinic use embophysio",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			s, err := loadClinicStore()
			if err != nil {
				return err
			}
			if _, ok := s.Clinics[name]; !ok {
				return usageErr(fmt.Errorf("clinic %q not found", name))
			}
			s.Current = name
			if err := saveClinicStore(s); err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"current": name}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Default clinic set to %q.\n", name)
			return nil
		},
	}
}

func newClinicRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Short:   "Forget a registered clinic (and its stored session)",
		Example: "  janeapp-pp-cli clinic remove embophysio --yes",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			s, err := loadClinicStore()
			if err != nil {
				return err
			}
			if _, ok := s.Clinics[name]; !ok {
				return usageErr(fmt.Errorf("clinic %q not found", name))
			}
			if !flags.yes {
				return usageErr(fmt.Errorf("refusing to remove %q without --yes", name))
			}
			delete(s.Clinics, name)
			if s.Current == name {
				s.Current = ""
			}
			if err := saveClinicStore(s); err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"removed": name}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed clinic %q.\n", name)
			return nil
		},
	}
}
