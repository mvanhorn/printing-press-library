package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/config"
)

func newProfilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "profiles",
		Short:       "Manage Amazon account profiles",
		Long:        "Each profile has an isolated cookie store and SQLite history DB so personal and work accounts cannot cross-contaminate.",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newProfilesListCmd(), newProfilesAddCmd(), newProfilesUseCmd(), newProfilesPathsCmd())
	return cmd
}

func newProfilesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List configured profiles",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContextNoProfile(cmd)
			if err != nil {
				return err
			}
			defer closeIfStore(app)
			profiles := app.Cfg.ListProfiles()
			active := ""
			if app.Profile.Name != "" {
				active = app.Profile.Name
			} else {
				active = app.Cfg.File.ActiveProfile
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"active":   active,
					"profiles": profiles,
				})
			}
			if len(profiles) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no profiles configured. Run `amazon-pp-cli profiles add <name>`.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ACTIVE\tNAME\tLABEL\tMARKETPLACE")
			for _, p := range profiles {
				marker := " "
				if p.Name == active {
					marker = "*"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", marker, p.Name, p.Label, p.MarketplaceBaseURL)
			}
			return tw.Flush()
		},
	}
}

func newProfilesAddCmd() *cobra.Command {
	var label, marketplace string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Register a new profile (no auth performed yet)",
		Example: "  amazon-pp-cli profiles add personal --label 'Personal account'\n" +
			"  amazon-pp-cli profiles add work --label 'Work account' --marketplace https://www.amazon.com",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContextNoProfile(cmd)
			if err != nil {
				return err
			}
			defer closeIfStore(app)
			name := args[0]
			if app.DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would add profile %s (label=%q, marketplace=%s)\n", name, label, marketplace)
				return nil
			}
			if err := app.Cfg.AddProfile(name, label, marketplace); err != nil {
				return coded(ExitConflict, "%v", err)
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"added": name})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added profile %s\n", name)
			fmt.Fprintln(cmd.OutOrStdout(), "next: amazon-pp-cli --profile "+name+" auth login")
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "Human-readable label for this profile")
	cmd.Flags().StringVar(&marketplace, "marketplace", "https://www.amazon.com", "Marketplace base URL")
	return cmd
}

func newProfilesUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the default active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContextNoProfile(cmd)
			if err != nil {
				return err
			}
			defer closeIfStore(app)
			if err := app.Cfg.SetActive(args[0]); err != nil {
				return coded(ExitNotFound, "%v", err)
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"active": args[0]})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "active profile is now %s\n", args[0])
			return nil
		},
	}
}

func newProfilesPathsCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "paths",
		Short:       "Print on-disk paths for the active profile",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			info := map[string]string{
				"profile":      app.Profile.Name,
				"profile_dir":  app.Cfg.ProfileDir(app.Profile.Name),
				"cookies_path": app.Cfg.CookiesPath(app.Profile.Name),
				"db_path":      app.Cfg.DBPath(app.Profile.Name),
				"config_dir":   app.Cfg.BaseDir(),
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(info)
			}
			for _, k := range []string{"profile", "profile_dir", "cookies_path", "db_path", "config_dir"} {
				fmt.Fprintf(cmd.OutOrStdout(), "%-13s %s\n", k+":", info[k])
			}
			return nil
		},
	}
}

// newAppContextNoProfile is used by profile-management commands that should
// not require an existing profile to be selected (e.g., `profiles add` on a
// fresh install).
func newAppContextNoProfile(cmd *cobra.Command) (*AppContext, error) {
	app := &AppContext{Ctx: cmd.Context()}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	app.Cfg = cfg
	if profileName, _ := cmd.Flags().GetString("profile"); profileName != "" {
		// Lenient: a --profile pointed at a nonexistent profile in this command
		// is informational only.
		_ = app.Cfg.SetActiveOverride(profileName)
	}
	if p, err := app.Cfg.ActiveProfile(); err == nil {
		app.Profile = p
	}
	app.JSON, _ = cmd.Flags().GetBool("json")
	app.DryRun, _ = cmd.Flags().GetBool("dry-run")
	return app, nil
}

func closeIfStore(app *AppContext) {
	if app != nil && app.Store != nil {
		_ = app.Store.Close()
	}
}
