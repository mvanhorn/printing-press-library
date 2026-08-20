// Per-site sensor capability commands. Not every Hayward OmniLogic install
// has pH, ORP, or salt probes; some pools have temperature sensors that only
// read while the pump is running. The capability table tells `chemistry get`,
// `telemetry get`, and `status` which sensors are real on each site so the
// verdict logic doesn't false-positive on null/-1 readings from absent
// equipment.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/devices/hayward-omnilogic/internal/omnilogic"
	"github.com/mvanhorn/printing-press-library/library/devices/hayward-omnilogic/internal/store"

	"github.com/spf13/cobra"
)

// PATCH (feat-per-site-sensor-capabilities): per-site sensor capability config (get/set/clear) backed by site_capabilities table (schema v2). Lets chemistry/status/telemetry/sweep distinguish 'sensor missing entirely' from 'sensor offline right now' so absent sensors do not false-positive verdicts.
func newCapabilitiesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capabilities",
		Short: "Per-site sensor capability configuration (pH / ORP / salt / temp-needs-flow).",
		Long: `Tells the CLI which sensors a given OmniLogic install actually has.
Hayward returns -1/null for absent sensors, and without this configuration the
CLI can't distinguish "sensor missing entirely" from "sensor offline right now".

After 'capabilities set':
  - 'status' excludes the missing sensors from its verdict
  - 'chemistry get' reports 'not_equipped' instead of 'unknown' for absent sensors
  - 'water_temp = -1' while the pump is idle is reported as 'n/a (pump off)'
    when temp_needs_flow is true

Stored per-MspSystemID in the local SQLite store. Falls back to "assume all
chemistry sensors equipped" when no row exists for a site.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newCapabilitiesGetCmd(flags))
	cmd.AddCommand(newCapabilitiesSetCmd(flags))
	cmd.AddCommand(newCapabilitiesClearCmd(flags))
	return cmd
}

func newCapabilitiesGetCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	cmd := &cobra.Command{
		Use:         "get",
		Short:       "Show the configured sensor capabilities for a site (or every site when --msp-system-id is omitted).",
		Example:     "  hayward-omnilogic-pp-cli capabilities get --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStore()
			if err != nil {
				return apiErr(err)
			}
			defer closeStore(s)
			if siteID != 0 {
				caps, err := s.GetSiteCapabilities(siteID)
				if err != nil {
					return apiErr(err)
				}
				if caps == nil {
					out := capabilitiesView{
						SiteMspSystemID: siteID,
						Configured:      false,
						Effective:       capabilitiesViewFromCore(store.AssumeAllEquipped(siteID)),
						Note:            "no capabilities row configured for this site; defaults assume all chemistry sensors are equipped",
					}
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				return printJSONFiltered(cmd.OutOrStdout(), buildCapabilitiesView(*caps), flags)
			}
			rows, err := s.ListSiteCapabilities()
			if err != nil {
				return apiErr(err)
			}
			out := make([]capabilitiesView, 0, len(rows))
			for _, r := range rows {
				out = append(out, buildCapabilitiesView(r))
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID (omit to list every configured site).")
	return cmd
}

func newCapabilitiesSetCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	var hasPH, hasORP, hasSalt, tempNeedsFlow string
	var notes string
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set per-site sensor capabilities. Each flag is independent — only flags you pass are updated.",
		Example: `  # Basic chlorine-tab pool with no chemistry probes and a flow-gated temp sensor
  hayward-omnilogic-pp-cli capabilities set \
    --has-ph false --has-orp false --has-salt false --temp-needs-flow true

  # Standard salt pool with full chemistry and continuous temp
  hayward-omnilogic-pp-cli capabilities set \
    --has-ph true --has-orp true --has-salt true --temp-needs-flow false`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := requireCredsUnlessDryRun(flags); err != nil {
				// We only need creds when siteID is 0 (need to resolve the
				// only-registered site). When the user supplies --msp-system-id
				// we can write the row offline.
				if siteID == 0 {
					return classifyOmnilogicError(err)
				}
			}
			s, err := openStore()
			if err != nil {
				return apiErr(err)
			}
			defer closeStore(s)
			// Resolve the site if the user didn't pass --msp-system-id.
			resolvedSiteID := siteID
			if resolvedSiteID == 0 {
				if err := requireCreds(); err != nil {
					return classifyOmnilogicError(err)
				}
				c := newOmnilogicClient(flags.timeout)
				site, rerr := resolveSite(c, s, 0)
				if rerr != nil {
					return classifyOmnilogicError(rerr)
				}
				resolvedSiteID = site.MspSystemID
			}
			// Load existing row (or default-equipped) so unspecified flags keep their current value.
			existing, _ := s.GetSiteCapabilities(resolvedSiteID)
			base := store.SiteCapabilities{}
			if existing != nil {
				base = *existing
				base.SiteMspSystemID = resolvedSiteID
			} else {
				base = store.AssumeAllEquipped(resolvedSiteID)
			}
			// Apply only the flags actually passed (empty string means unset).
			if err := applyBoolFlag(hasPH, &base.HasPHSensor, "has-ph"); err != nil {
				return usageErr(err)
			}
			if err := applyBoolFlag(hasORP, &base.HasORPSensor, "has-orp"); err != nil {
				return usageErr(err)
			}
			if err := applyBoolFlag(hasSalt, &base.HasSaltSensor, "has-salt"); err != nil {
				return usageErr(err)
			}
			if err := applyBoolFlag(tempNeedsFlow, &base.TempNeedsFlow, "temp-needs-flow"); err != nil {
				return usageErr(err)
			}
			if cmd.Flags().Changed("notes") {
				base.Notes = notes
			}
			if err := s.SetSiteCapabilities(base); err != nil {
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), buildCapabilitiesView(base), flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	cmd.Flags().StringVar(&hasPH, "has-ph", "", "Does this site have a pH probe? (true|false)")
	cmd.Flags().StringVar(&hasORP, "has-orp", "", "Does this site have an ORP probe? (true|false)")
	cmd.Flags().StringVar(&hasSalt, "has-salt", "", "Does this site have a salt cell? (true|false)")
	cmd.Flags().StringVar(&tempNeedsFlow, "temp-needs-flow", "", "Does the water-temp sensor only report while the pump is running? (true|false)")
	cmd.Flags().StringVar(&notes, "notes", "", "Free-form note attached to the capability row.")
	return cmd
}

func newCapabilitiesClearCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	cmd := &cobra.Command{
		Use:     "clear",
		Short:   "Remove the capability row for a site. Falls back to 'assume all sensors equipped' afterwards.",
		Example: "  hayward-omnilogic-pp-cli capabilities clear --msp-system-id 12345",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStore()
			if err != nil {
				return apiErr(err)
			}
			defer closeStore(s)
			resolvedSiteID := siteID
			if resolvedSiteID == 0 {
				if err := requireCreds(); err != nil {
					return classifyOmnilogicError(err)
				}
				c := newOmnilogicClient(flags.timeout)
				site, rerr := resolveSite(c, s, 0)
				if rerr != nil {
					return classifyOmnilogicError(rerr)
				}
				resolvedSiteID = site.MspSystemID
			}
			if err := s.ClearSiteCapabilities(resolvedSiteID); err != nil {
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"cleared":            true,
				"site_msp_system_id": resolvedSiteID,
			}, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	return cmd
}

// capabilitiesView is the JSON shape returned by `capabilities get/set`.
// Wraps the store row with explicit `configured` and `setup_hint` fields so
// agents reading the output know whether the row is operator-blessed or
// the default fallback.
type capabilitiesView struct {
	SiteMspSystemID int                        `json:"site_msp_system_id"`
	Configured      bool                       `json:"configured"`
	HasPHSensor     bool                       `json:"has_ph_sensor"`
	HasORPSensor    bool                       `json:"has_orp_sensor"`
	HasSaltSensor   bool                       `json:"has_salt_sensor"`
	TempNeedsFlow   bool                       `json:"temp_needs_flow"`
	ConfiguredAt    string                     `json:"configured_at,omitempty"`
	Notes           string                     `json:"notes,omitempty"`
	Effective       *capabilitiesViewEffective `json:"effective,omitempty"`
	Note            string                     `json:"note,omitempty"`
}

type capabilitiesViewEffective struct {
	HasPHSensor   bool `json:"has_ph_sensor"`
	HasORPSensor  bool `json:"has_orp_sensor"`
	HasSaltSensor bool `json:"has_salt_sensor"`
	TempNeedsFlow bool `json:"temp_needs_flow"`
}

func capabilitiesViewFromCore(c store.SiteCapabilities) *capabilitiesViewEffective {
	return &capabilitiesViewEffective{
		HasPHSensor:   c.HasPHSensor,
		HasORPSensor:  c.HasORPSensor,
		HasSaltSensor: c.HasSaltSensor,
		TempNeedsFlow: c.TempNeedsFlow,
	}
}

func buildCapabilitiesView(c store.SiteCapabilities) capabilitiesView {
	v := capabilitiesView{
		SiteMspSystemID: c.SiteMspSystemID,
		Configured:      true,
		HasPHSensor:     c.HasPHSensor,
		HasORPSensor:    c.HasORPSensor,
		HasSaltSensor:   c.HasSaltSensor,
		TempNeedsFlow:   c.TempNeedsFlow,
		Notes:           c.Notes,
	}
	if !c.ConfiguredAt.IsZero() {
		v.ConfiguredAt = c.ConfiguredAt.Format(jsonTimeFormat)
	}
	return v
}

const jsonTimeFormat = "2006-01-02T15:04:05Z07:00"

// applyBoolFlag parses an optional string-bool flag into target. Empty
// string is "flag not passed" and leaves target unchanged. Used by
// `capabilities set` so callers can update only one field at a time.
func applyBoolFlag(raw string, target *bool, name string) error {
	if raw == "" {
		return nil
	}
	switch raw {
	case "true", "True", "TRUE", "1", "yes", "y":
		*target = true
	case "false", "False", "FALSE", "0", "no", "n":
		*target = false
	default:
		return fmt.Errorf("invalid --%s value %q (use true|false)", name, raw)
	}
	return nil
}

// loadEffectiveCapabilities returns the capability row a consumer should
// honor for a site. When no row is configured it returns the AssumeAllEquipped
// default and configured=false so callers know to emit the setup hint.
func loadEffectiveCapabilities(s *store.Store, siteID int) (store.SiteCapabilities, bool) {
	if s == nil {
		return store.AssumeAllEquipped(siteID), false
	}
	caps, err := s.GetSiteCapabilities(siteID)
	if err != nil || caps == nil {
		return store.AssumeAllEquipped(siteID), false
	}
	return *caps, true
}

// chemistrySetupHint inspects a telemetry snapshot's chemistry fields and
// returns a setup-hint string when the readings look "absent" but no
// capability row is configured. Returns empty string when no hint is needed.
// Used by chemistry get / status / telemetry get to nudge first-run setup.
func chemistrySetupHint(t *omnilogic.Telemetry, configured bool) string {
	if configured || t == nil {
		return ""
	}
	suspicious := 0
	for _, bow := range t.BodiesOfWater {
		if bow.PH == nil && bow.ORP == nil && bow.SaltPPM == nil {
			suspicious++
		}
		if bow.WaterTemp != nil && *bow.WaterTemp == -1 {
			suspicious++
		}
	}
	if suspicious == 0 {
		return ""
	}
	return "chemistry/temp sensors returned null or -1 — if this site is not equipped with those sensors, run: hayward-omnilogic-pp-cli capabilities set --has-ph false --has-orp false --has-salt false --temp-needs-flow true (adjust each flag to match your install)"
}

var _ = errors.New
var _ = json.Marshal
