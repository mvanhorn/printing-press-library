// Copyright 2026 startos00. Licensed under Apache-2.0. See LICENSE.

// Hand-built novel command: `instances` — a local registry of ePropertyPlus
// land-bank slugs, plus the base-URL resolution that lets every command target
// any instance via --instance / env / registry. Follows profile.go's JSON-file
// registry pattern and helpers.go's verify-friendly RunE conventions.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/epropertyplus/internal/landbank"

	"github.com/spf13/cobra"
)

// instanceEntry is one known land-bank instance.
type instanceEntry struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// instanceRegistry is the on-disk registry of known instances plus the
// currently-selected slug.
type instanceRegistry struct {
	Current   string                   `json:"current"`
	Instances map[string]instanceEntry `json:"instances"`
}

func instanceRegistryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	dir := filepath.Join(home, ".epropertyplus-pp-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating state dir: %w", err)
	}
	return filepath.Join(dir, "instances.json"), nil
}

// seededRegistry returns the registry seeded with the default kclb instance.
func seededRegistry() *instanceRegistry {
	return &instanceRegistry{
		Current: "",
		Instances: map[string]instanceEntry{
			landbank.DefaultSlug: {Slug: landbank.DefaultSlug, Name: "Kansas City Land Bank"},
		},
	}
}

func loadInstanceRegistry() (*instanceRegistry, error) {
	p, err := instanceRegistryPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return seededRegistry(), nil
		}
		return nil, fmt.Errorf("reading instances: %w", err)
	}
	var r instanceRegistry
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parsing instances: %w", err)
	}
	if r.Instances == nil {
		r.Instances = map[string]instanceEntry{}
	}
	// Always ensure the default seed is present so a hand-edited or partial
	// registry still resolves kclb.
	if _, ok := r.Instances[landbank.DefaultSlug]; !ok {
		r.Instances[landbank.DefaultSlug] = instanceEntry{Slug: landbank.DefaultSlug, Name: "Kansas City Land Bank"}
	}
	return &r, nil
}

func saveInstanceRegistry(r *instanceRegistry) error {
	p, err := instanceRegistryPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling instances: %w", err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("writing instances: %w", err)
	}
	return os.Rename(tmp, p)
}

// registryCurrentSlug returns the registry's current slug, or "" if the
// registry can't be loaded. Used by base-URL resolution; never errors out the
// command — a missing registry just means "fall through to default".
func registryCurrentSlug() string {
	r, err := loadInstanceRegistry()
	if err != nil {
		return ""
	}
	return r.Current
}

// resolveBaseURL applies the precedence:
//
//	--instance flag > EPROPERTYPLUS_BASE_URL > EPROPERTYPLUS_INSTANCE >
//	registry "current" > default kclb
//
// Wired into rootFlags.newClient via resolvedBaseURL().
func resolveBaseURL(flagInstance string) landbank.Resolution {
	return landbank.ResolveBaseURL(landbank.ResolveInput{
		FlagInstance:  flagInstance,
		EnvBaseURL:    os.Getenv("EPROPERTYPLUS_BASE_URL"),
		EnvInstance:   os.Getenv("EPROPERTYPLUS_INSTANCE"),
		RegistryCurr:  registryCurrentSlug(),
		DefaultSlugIn: landbank.DefaultSlug,
	})
}

func newInstancesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instances",
		Short: "Manage the local registry of known ePropertyPlus land-bank instances",
		Long: strings.Trim(`
Every ePropertyPlus land bank is reachable by slug at
https://public-<slug>.epropertyplus.com/landmgmtpub/remote/public.

This command manages a local registry of known slugs and which one is
current. Point any command at an instance for a single invocation with the
persistent --instance <slug> flag, or set a durable default with
'instances use <slug>'.

Resolution precedence (highest first):
  --instance flag > EPROPERTYPLUS_BASE_URL > EPROPERTYPLUS_INSTANCE >
  registry "current" > default kclb
`, "\n"),
	}
	cmd.AddCommand(newInstancesListCmd(flags))
	cmd.AddCommand(newInstancesAddCmd(flags))
	cmd.AddCommand(newInstancesUseCmd(flags))
	return cmd
}

func newInstancesListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List known land-bank instances and which one is current",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  epropertyplus-pp-cli instances list
  epropertyplus-pp-cli instances list --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := loadInstanceRegistry()
			if err != nil {
				return err
			}
			resolved := resolveBaseURL(flags.instance)

			slugs := make([]string, 0, len(r.Instances))
			for s := range r.Instances {
				slugs = append(slugs, s)
			}
			sort.Strings(slugs)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out := make([]map[string]any, 0, len(slugs))
				for _, s := range slugs {
					e := r.Instances[s]
					out = append(out, map[string]any{
						"slug":     e.Slug,
						"name":     e.Name,
						"base_url": landbank.BaseURLForSlug(e.Slug),
						"current":  s == r.Current,
					})
				}
				env := map[string]any{
					"instances":       out,
					"current":         r.Current,
					"active_base_url": resolved.BaseURL,
					"active_source":   resolved.Source,
				}
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}

			headers := []string{"CURRENT", "SLUG", "NAME", "BASE_URL"}
			rows := make([][]string, 0, len(slugs))
			for _, s := range slugs {
				e := r.Instances[s]
				marker := ""
				if s == r.Current {
					marker = "*"
				}
				rows = append(rows, []string{marker, e.Slug, e.Name, landbank.BaseURLForSlug(e.Slug)})
			}
			if err := flags.printTable(cmd, headers, rows); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\nactive: %s (source: %s)\n", resolved.BaseURL, resolved.Source)
			return nil
		},
	}
	return cmd
}

func newInstancesAddCmd(flags *rootFlags) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "add <slug> [--name <name>]",
		Short: "Add a land-bank instance to the registry by slug",
		Example: strings.Trim(`
  epropertyplus-pp-cli instances add detroit --name "Detroit Land Bank Authority"
  epropertyplus-pp-cli instances add stlouis
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := strings.TrimSpace(args[0])
			if slug == "" {
				return usageErr(fmt.Errorf("slug must not be empty"))
			}
			if strings.ContainsAny(slug, `/\: .`) {
				return usageErr(fmt.Errorf("slug %q contains reserved characters", slug))
			}
			r, err := loadInstanceRegistry()
			if err != nil {
				return err
			}
			if name == "" {
				name = slug
			}
			r.Instances[slug] = instanceEntry{Slug: slug, Name: name}
			if err := saveInstanceRegistry(r); err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"added":    slug,
					"name":     name,
					"base_url": landbank.BaseURLForSlug(slug),
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added instance %q (%s) → %s\n", slug, name, landbank.BaseURLForSlug(slug))
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Human-readable name for the instance")
	return cmd
}

func newInstancesUseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use <slug>",
		Short: "Set the current default instance in the registry",
		Example: strings.Trim(`
  epropertyplus-pp-cli instances use kclb
  epropertyplus-pp-cli instances use detroit
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := strings.TrimSpace(args[0])
			if slug == "" {
				return usageErr(fmt.Errorf("slug must not be empty"))
			}
			r, err := loadInstanceRegistry()
			if err != nil {
				return err
			}
			if _, ok := r.Instances[slug]; !ok {
				// Auto-register on use so the user isn't forced to add first;
				// the slug shape is fixed by the vendor convention anyway.
				r.Instances[slug] = instanceEntry{Slug: slug, Name: slug}
			}
			r.Current = slug
			if err := saveInstanceRegistry(r); err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"current":  slug,
					"base_url": landbank.BaseURLForSlug(slug),
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "now using instance %q → %s\n", slug, landbank.BaseURLForSlug(slug))
			return nil
		},
	}
	return cmd
}
