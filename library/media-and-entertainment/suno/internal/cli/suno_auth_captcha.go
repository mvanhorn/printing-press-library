// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// `auth captcha login|status|stop` — manage the dedicated piloted-Chrome solver
// profiles used to clear Suno's hCaptcha gate.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/captcha"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/config"
	"github.com/spf13/cobra"
)

func newAuthCaptchaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "captcha",
		Short: "Manage piloted-Chrome solver profiles for the hCaptcha gate",
	}
	cmd.AddCommand(newAuthCaptchaLoginCmd(flags))
	cmd.AddCommand(newAuthCaptchaStatusCmd(flags))
	cmd.AddCommand(newAuthCaptchaStopCmd(flags))
	return cmd
}

func newAuthCaptchaLoginCmd(flags *rootFlags) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Open a visible Chrome to sign a profile into Suno (then persists)",
		RunE: func(cmd *cobra.Command, args []string) error {
			captchaProfileFlag = profile
			opts, err := resolveCaptchaOptions(flags.configPath, true)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Opening Chrome for profile %q — sign into Suno, then close the window.\n", opts.Profile)
			return captcha.Login(cmd.Context(), opts)
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile name (default: configured default)")
	return cmd
}

func newAuthCaptchaStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show which managed Chrome solver profiles are running",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			if cfg.Captcha == nil || len(cfg.Captcha.Profiles) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no solver profiles configured")
				return nil
			}
			for name, p := range cfg.Captcha.Profiles {
				st := captcha.StatusFor(name, p.CDPPort)
				state := "stopped"
				if st.Running {
					state = "running"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s  port=%d  %s\n", name, p.CDPPort, state)
			}
			return nil
		},
	}
	return cmd
}

func newAuthCaptchaStopCmd(flags *rootFlags) *cobra.Command {
	var profile string
	var all bool
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Tear down a managed Chrome solver profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			if cfg.Captcha == nil {
				return nil
			}
			for name, p := range cfg.Captcha.Profiles {
				if all || name == cfg.ResolveCaptchaProfile(profile) {
					if serr := captcha.Stop(cmd.Context(), p.CDPPort); serr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: stop %s: %v\n", name, serr)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile to stop (default: configured default)")
	cmd.Flags().BoolVar(&all, "all", false, "Stop all profiles")
	return cmd
}
