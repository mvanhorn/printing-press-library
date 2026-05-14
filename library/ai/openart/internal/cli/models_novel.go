package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/openart/internal/openartmodels"
)

func newModelsNovelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Browse the curated OpenArt model catalog",
		Long: `OpenArt does not expose a public model-listing API. This command surfaces
the curated catalog scraped from the OpenArt suite picker, with cost,
duration range, and per-model capabilities.`,
	}
	cmd.AddCommand(newModelsListCmd(flags))
	cmd.AddCommand(newModelsShowCmd(flags))
	cmd.AddCommand(newModelsCheapestCmd(flags))
	return cmd
}

func newModelsListCmd(flags *rootFlags) *cobra.Command {
	var familyFilter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List every OpenArt model the CLI knows about",
		Example: `  openart-pp-cli models list
  openart-pp-cli models list --family video --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			rows := []map[string]any{}
			for _, m := range openartmodels.Catalog {
				if familyFilter != "" && string(m.Family) != familyFilter {
					continue
				}
				rows = append(rows, modelRow(m))
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&familyFilter, "family", "", "Filter by family: video | image | audio")
	return cmd
}

func newModelsShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show <model>",
		Short:   "Show full details for one model (slug or shorthand)",
		Example: `  openart-pp-cli models show seedance2`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			m := openartmodels.Resolve(args[0])
			if m == nil {
				return fmt.Errorf("unknown model %q. Run 'openart-pp-cli models list' to see options.", args[0])
			}
			return printJSONFiltered(cmd.OutOrStdout(), modelRow(*m), flags)
		},
	}
	return cmd
}

func newModelsCheapestCmd(flags *rootFlags) *cobra.Command {
	var (
		family     string
		typeAlias  string
		duration   int
		resolution string
		needsAudio bool
		needsRef   bool
	)
	cmd := &cobra.Command{
		Use:   "cheapest",
		Short: "Find the cheapest model that satisfies a target shape",
		Long: `Filter the catalog by capability (family / resolution / duration / audio
/ reference-image support) and rank by credits per video at the requested
duration and resolution.

Use this to throw a non-precious prompt at the cheapest model that
covers your needs before committing credits to a Seedance/Veo run.`,
		Example: `  openart-pp-cli models cheapest --family video --duration 5 --resolution 720p`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if typeAlias != "" {
				family = typeAlias
			}
			if family == "" {
				family = "video"
			}
			cands := []openartmodels.Model{}
			for _, m := range openartmodels.Catalog {
				if string(m.Family) != family {
					continue
				}
				if duration > 0 && (duration < m.DurationMinSec || duration > m.DurationMaxSec) {
					continue
				}
				if resolution != "" && !modelSupports(m.Resolutions, resolution) {
					continue
				}
				if needsAudio && !m.HasAudio {
					continue
				}
				if needsRef && !m.SupportsReference {
					continue
				}
				cands = append(cands, m)
			}
			d := duration
			if d == 0 {
				d = 5
			}
			res := resolution
			if res == "" {
				res = "720p"
			}
			sort.SliceStable(cands, func(i, j int) bool {
				ci := cands[i].EstimateCredits(d, 1, res)
				cj := cands[j].EstimateCredits(d, 1, res)
				return ci < cj
			})
			rows := []map[string]any{}
			for _, m := range cands {
				row := modelRow(m)
				row["estimate_credits"] = m.EstimateCredits(d, 1, res)
				row["estimated_at_duration"] = d
				row["estimated_at_resolution"] = res
				rows = append(rows, row)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&family, "family", "video", "Family: video | image | audio")
	cmd.Flags().StringVar(&typeAlias, "type", "", "Alias for --family (video | image | audio)")
	cmd.Flags().IntVar(&duration, "duration", 0, "Required duration (s); filters out models that can't hit it")
	cmd.Flags().StringVar(&resolution, "resolution", "720p", "Required resolution")
	cmd.Flags().BoolVar(&needsAudio, "audio", false, "Require audio support")
	cmd.Flags().BoolVar(&needsRef, "reference", false, "Require visual reference support")
	return cmd
}

func modelRow(m openartmodels.Model) map[string]any {
	return map[string]any{
		"slug":                       m.Slug,
		"display_name":               m.DisplayName,
		"vendor":                     m.Vendor,
		"family":                     string(m.Family),
		"description":                m.Description,
		"supported_forms":            formStrings(m.SupportedForms),
		"resolutions":                m.Resolutions,
		"duration_min_sec":           m.DurationMinSec,
		"duration_max_sec":           m.DurationMaxSec,
		"has_audio":                  m.HasAudio,
		"supports_reference":         m.SupportsReference,
		"supports_start_end_frame":   m.SupportsStartEndFrame,
		"supports_multi_shots":       m.SupportsMultiShots,
		"credits_per_video_default":  m.CreditsPerVideoDefault,
		"tier":                       m.Tier,
		"recommended":                m.Recommended,
		// PATCH: surface Experimental in models list/show JSON so callers know
		// which models need --accept-experimental before submitting.
		"experimental":               m.Experimental,
	}
}

func formStrings(forms []openartmodels.FormType) []string {
	out := make([]string, len(forms))
	for i, f := range forms {
		out[i] = string(f)
	}
	return out
}

// modelHumanDescriptor returns a one-line summary used by stderr progress lines.
func modelHumanDescriptor(m openartmodels.Model) string {
	return fmt.Sprintf("%s (%s, %s, %d-%ds, %s)",
		m.DisplayName, m.Vendor, strings.Join(m.Resolutions, "/"), m.DurationMinSec, m.DurationMaxSec, m.Family)
}
