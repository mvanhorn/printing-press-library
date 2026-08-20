package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

const sellerNoDSPPersona = "seller-no-dsp"

type personaView struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Groups      []personaGroup     `json:"groups"`
	FullSurface personaFullSurface `json:"full_surface"`
}

type personaGroup struct {
	Name     string   `json:"name"`
	Commands []string `json:"commands"`
}

type personaFullSurface struct {
	Collapsed bool     `json:"collapsed"`
	Groups    []string `json:"groups"`
}

func resolvePersona(flags *rootFlags, cmd *cobra.Command) string {
	if flags.persona == "" && os.Getenv("AMAZON_ADS_CLI_PERSONA") != "" && !cmd.Flags().Changed("persona") {
		flags.persona = os.Getenv("AMAZON_ADS_CLI_PERSONA")
	}
	if strings.TrimSpace(flags.persona) == "" {
		return "default"
	}
	return strings.TrimSpace(flags.persona)
}

func personaDefinition(name string) *personaView {
	if name != sellerNoDSPPersona {
		return nil
	}
	return &personaView{
		Name:        sellerNoDSPPersona,
		Description: "Profitability-focused seller view without DSP, audiences, demand platform, or attribution clutter.",
		Groups: []personaGroup{
			{Name: "Sponsored Products", Commands: []string{"sp", "sponsored-products-sp"}},
			{Name: "Sponsored Brands", Commands: []string{"sb", "sponsored-brands-sb"}},
			{Name: "Sponsored Display", Commands: []string{"sd", "sponsored-display-sd"}},
			{Name: "Profitability", Commands: []string{"break-even-acos", "true-profit", "acos-vs-tacos", "portfolio-dashboard", "product-ad-profitability", "campaign-comparison", "weekly-review"}},
			{Name: "Reports", Commands: []string{"reports", "normalize-report"}},
			{Name: "Support", Commands: []string{"auth", "auth-login", "profile", "doctor", "agent-context", "feedback", "which"}},
		},
		FullSurface: personaFullSurface{
			Collapsed: true,
			Groups:    []string{"amazon-ads-dsp-dsp", "dsp", "dsp-reports-dsp", "audiences", "dp", "attribution", "measurement-dsp", "assets", "billing", "manager-accounts", "stores"},
		},
	}
}

func newCommandGroupsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "command-groups",
		Short:       "Show curated command groups for the active persona",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			persona := resolvePersona(flags, cmd)
			view := personaDefinition(persona)
			if view == nil {
				view = &personaView{
					Name:        "default",
					Description: "Full Amazon Ads CLI surface.",
					FullSurface: personaFullSurface{
						Collapsed: false,
					},
				}
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderPersonaView(cmd, view)
		},
	}
	return cmd
}

func renderPersonaView(cmd *cobra.Command, view *personaView) error {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "persona:\t%s\n", view.Name)
	fmt.Fprintf(tw, "description:\t%s\n", view.Description)
	for _, group := range view.Groups {
		fmt.Fprintf(tw, "\n%s:\t%s\n", group.Name, strings.Join(group.Commands, ", "))
	}
	if len(view.FullSurface.Groups) > 0 {
		state := "visible"
		if view.FullSurface.Collapsed {
			state = "collapsed"
		}
		fmt.Fprintf(tw, "\nFull surface (%s):\t%s\n", state, strings.Join(view.FullSurface.Groups, ", "))
	}
	return tw.Flush()
}

func renderPersonaHelp(cmd *cobra.Command, flags *rootFlags) bool {
	view := personaDefinition(resolvePersona(flags, cmd))
	if view == nil {
		return false
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n", cmd.Root().Long)
	_ = renderPersonaView(cmd, view)
	fmt.Fprintf(cmd.OutOrStdout(), "\nRun '%s command-groups --json' for machine-readable groups or '%s --help' without --persona for the full command tree.\n", cmd.Root().Name(), cmd.Root().Name())
	return true
}
