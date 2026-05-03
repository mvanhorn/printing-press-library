package cli

import (
	"context"
	"io"
	"math"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/fingerprint"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/source/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/source/vrbo"
	"github.com/spf13/cobra"
)

func newMatchCmd(flags *rootFlags) *cobra.Command {
	var platform string
	cmd := &cobra.Command{
		Use:         "match <listing-url>",
		Short:       "Find likely same-property listings on the other platform",
		Example:     "  airbnb-vrbo-pp-cli match https://www.airbnb.com/rooms/37124493 --platform both",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"matches": []any{}, "method": "dry_run"}, flags)
			}
			target := stripURLArg(args[0])
			ref, err := parseListingURL(target)
			if err != nil {
				return usageErr(err)
			}
			out := map[string]any{"source": ref, "matches": []any{}}
			switch ref.Platform {
			case "airbnb":
				ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
				l, err := airbnb.Get(ctx, ref.ID, airbnb.GetParams{})
				cancel()
				if err != nil {
					return classifyAPIError(err)
				}
				fp := fingerprint.FromAirbnb(l)
				out["fingerprint"] = fp
				out["matches"] = runMatchSearches(cmd.Context(), platform, ref.Platform, l.City, fp, cmd.ErrOrStderr())
			case "vrbo":
				ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
				l, err := vrbo.Get(ctx, ref.ID, vrbo.GetParams{})
				cancel()
				if err != nil {
					return classifyAPIError(err)
				}
				fp := fingerprint.FromVRBO(l)
				out["fingerprint"] = fp
				out["matches"] = runMatchSearches(cmd.Context(), platform, ref.Platform, l.City, fp, cmd.ErrOrStderr())
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "both", "Platform to search: airbnb, vrbo, both")
	return cmd
}

type matchSource struct {
	name string
}

func runMatchSearches(ctx context.Context, platform, sourcePlatform, city string, fp *fingerprint.Fingerprint, errOut io.Writer) []map[string]any {
	sources := matchSources(platform, sourcePlatform)
	results, errs := cliutil.FanoutRun(ctx, sources, func(s matchSource) string { return s.name }, func(ctx context.Context, s matchSource) ([]map[string]any, error) {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		switch s.name {
		case "airbnb":
			listings, _, err := airbnb.Search(ctx, airbnb.SearchParams{Location: city})
			if err != nil {
				return nil, err
			}
			return matchAirbnb(fp, listings), nil
		default:
			props, _, err := vrbo.Search(ctx, vrbo.SearchParams{Location: city, PageSize: 25})
			if err != nil {
				return nil, err
			}
			return matchVRBO(fp, props), nil
		}
	}, cliutil.WithConcurrency(2))
	cliutil.FanoutReportErrors(errOut, errs)
	out := []map[string]any{}
	for _, r := range results {
		out = append(out, r.Value...)
	}
	return out
}

func matchSources(platform, sourcePlatform string) []matchSource {
	switch platform {
	case "airbnb", "vrbo":
		if platform == sourcePlatform {
			return nil
		}
		return []matchSource{{name: platform}}
	default:
		if sourcePlatform == "airbnb" {
			return []matchSource{{name: "vrbo"}}
		}
		return []matchSource{{name: "airbnb"}}
	}
}

func matchVRBO(fp *fingerprint.Fingerprint, props []vrbo.Property) []map[string]any {
	var out []map[string]any
	for i := range props {
		pfp := fingerprint.FromVRBO(&props[i])
		if conf := fpConfidence(fp, pfp); conf > .65 {
			out = append(out, map[string]any{"listing": props[i], "fingerprint": pfp, "confidence": conf})
		}
	}
	return out
}

func matchAirbnb(fp *fingerprint.Fingerprint, listings []airbnb.Listing) []map[string]any {
	var out []map[string]any
	for i := range listings {
		lfp := fingerprint.FromAirbnb(&listings[i])
		if conf := fpConfidence(fp, lfp); conf > .65 {
			out = append(out, map[string]any{"listing": listings[i], "fingerprint": lfp, "confidence": conf})
		}
	}
	return out
}

func fpConfidence(a, b *fingerprint.Fingerprint) float64 {
	if a.Hash == b.Hash {
		return 1
	}
	score, total := 0.0, 0.0
	for _, k := range []string{"city", "lat", "lng", "beds", "baths", "sleeps_max"} {
		total++
		if a.Components[k] == b.Components[k] && a.Components[k] != "" && a.Components[k] != "0" {
			score++
			continue
		}
		if k == "lat" || k == "lng" {
			if math.Abs(valueString(a.Components[k])-valueString(b.Components[k])) <= .002 {
				score += .8
			}
		}
	}
	return score / total
}

func valueString(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
