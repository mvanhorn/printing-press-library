// OmniLogic-aware command implementations. These replace the generator-emitted
// stubs whose handlers tried to JSON-encode requests against the legacy .ashx
// endpoint. Each command uses the hand-built omnilogic.Client (two-stage auth,
// XML envelopes) and writes side-effects to the local store's command_log.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/hayward-omnilogic/internal/omnilogic"
	"github.com/mvanhorn/printing-press-library/library/devices/hayward-omnilogic/internal/store"

	"github.com/spf13/cobra"
)

// ---------- sites ----------

func newOmniSitesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sites",
		Short:       "List every site (backyard) registered to your Hayward account.",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newOmniSitesListCmd(flags))
	return cmd
}

func newOmniSitesListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List every site under your Hayward account.",
		Example:     "  hayward-omnilogic-pp-cli sites list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {

				return nil

			}
			if err := requireCredsUnlessDryRun(flags); err != nil {

				return classifyOmnilogicError(err)

			}
			c := newOmnilogicClient(flags.timeout)
			sites, err := c.GetSiteList()
			if err != nil {
				return classifyOmnilogicError(err)
			}
			if s, err := openStore(); err == nil {
				_ = s.UpsertSites(sites)
				_ = s.Close()
			}
			return emitJSONOrTable(cmd, flags, sites, []string{"msp_system_id", "backyard_name"}, func(v omnilogic.Site) []string {
				return []string{strconv.Itoa(v.MspSystemID), v.BackyardName}
			})
		},
	}
	return cmd
}

// ---------- config (MSP config) ----------

func newOmniConfigCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "config",
		Short:       "Fetch the equipment inventory tree (pumps, heaters, lights, valves) for one site.",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newOmniConfigGetCmd(flags))
	return cmd
}

func newOmniConfigGetCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	cmd := &cobra.Command{
		Use:         "get",
		Short:       "Fetch the equipment inventory for one site.",
		Example:     "  hayward-omnilogic-pp-cli config get --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {

				return nil

			}
			if err := requireCredsUnlessDryRun(flags); err != nil {

				return classifyOmnilogicError(err)

			}
			c := newOmnilogicClient(flags.timeout)
			s, _ := openStore()
			defer closeStore(s)
			site, err := resolveSite(c, s, siteID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			cfg, err := c.GetMspConfig(site.MspSystemID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			cfg.BackyardName = site.BackyardName
			if s != nil {
				_ = s.UpsertMspConfig(cfg)
			}
			return printJSONFiltered(cmd.OutOrStdout(), cfg, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	return cmd
}

// ---------- alarms ----------

func newOmniAlarmsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "alarms",
		Short:       "List active alarms.",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newOmniAlarmsListCmd(flags))
	return cmd
}

func newOmniAlarmsListCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	var allSites bool
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List active alarms for a site (or every site with --all).",
		Example:     "  hayward-omnilogic-pp-cli alarms list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {

				return nil

			}
			if err := requireCredsUnlessDryRun(flags); err != nil {

				return classifyOmnilogicError(err)

			}
			c := newOmnilogicClient(flags.timeout)
			s, _ := openStore()
			defer closeStore(s)
			var targets []omnilogic.Site
			if allSites {
				sites, err := c.GetSiteList()
				if err != nil {
					return classifyOmnilogicError(err)
				}
				if s != nil {
					_ = s.UpsertSites(sites)
				}
				targets = sites
			} else {
				site, err := resolveSite(c, s, siteID)
				if err != nil {
					return classifyOmnilogicError(err)
				}
				targets = []omnilogic.Site{site}
			}
			var out []omnilogic.SiteAlarms
			for _, site := range targets {
				alarms, err := c.GetAlarmList(site.MspSystemID)
				if err != nil {
					return classifyOmnilogicError(err)
				}
				if s != nil {
					_ = s.UpsertAlarms(site.MspSystemID, alarms)
				}
				out = append(out, omnilogic.SiteAlarms{
					MspSystemID:  site.MspSystemID,
					BackyardName: site.BackyardName,
					Alarms:       alarms,
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	cmd.Flags().BoolVar(&allSites, "all", false, "Sweep every site registered to the account.")
	return cmd
}

// ---------- telemetry ----------

func newOmniTelemetryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "telemetry",
		Short:       "Snapshot live state for one site (chemistry, temps, pump/heater/light state).",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newOmniTelemetryGetCmd(flags))
	return cmd
}

func newOmniTelemetryGetCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	cmd := &cobra.Command{
		Use:         "get",
		Short:       "Live state snapshot: pH, ORP, salt, temps, pump/heater/light state, alarm flags.",
		Example:     "  hayward-omnilogic-pp-cli telemetry get --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {

				return nil

			}
			if err := requireCredsUnlessDryRun(flags); err != nil {

				return classifyOmnilogicError(err)

			}
			c := newOmnilogicClient(flags.timeout)
			s, _ := openStore()
			defer closeStore(s)
			site, err := resolveSite(c, s, siteID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			t, err := c.GetTelemetry(site.MspSystemID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			t.BackyardName = site.BackyardName
			if s != nil {
				_, _ = s.AppendTelemetry(t)
			}
			return printJSONFiltered(cmd.OutOrStdout(), t, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	return cmd
}

// ---------- chemistry (live read; chemistry log/drift are transcendence) ----------

func newOmniChemistryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "chemistry",
		Short:       "Pool chemistry: current snapshot, historical log, drift detection.",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newOmniChemistryGetCmd(flags))
	cmd.AddCommand(newOmniChemistryLogCmd(flags))
	cmd.AddCommand(newOmniChemistryDriftCmd(flags))
	return cmd
}

func newOmniChemistryGetCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	cmd := &cobra.Command{
		Use:         "get",
		Short:       "Current chemistry snapshot per BoW: pH, ORP, salt, water temp, traffic-light verdict.",
		Example:     "  hayward-omnilogic-pp-cli chemistry get --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {

				return nil

			}
			if err := requireCredsUnlessDryRun(flags); err != nil {

				return classifyOmnilogicError(err)

			}
			c := newOmnilogicClient(flags.timeout)
			s, _ := openStore()
			defer closeStore(s)
			site, err := resolveSite(c, s, siteID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			t, err := c.GetTelemetry(site.MspSystemID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			if s != nil {
				_, _ = s.AppendTelemetry(t)
			}
			caps, configured := loadEffectiveCapabilities(s, site.MspSystemID)
			out := buildChemistryReports(site.MspSystemID, t, caps)
			// Emit setup hint to stderr when telemetry is "suspicious" and
			// no capability row is configured. Stderr keeps the JSON output
			// clean for downstream pipes while still surfacing the setup
			// guidance to humans and to agents that read stderr.
			if hint := chemistrySetupHint(t, configured); hint != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "setup_hint: %s\n", hint)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	return cmd
}

// buildChemistryReports projects a telemetry snapshot into capability-aware
// Chemistry records. Sensors marked unequipped in caps are omitted from
// the per-BoW row (and excluded from the verdict computation). The temp
// state captures the pump-flow-gating quirk for installs that have it.
func buildChemistryReports(siteID int, t *omnilogic.Telemetry, caps store.SiteCapabilities) []omnilogic.Chemistry {
	out := make([]omnilogic.Chemistry, 0, len(t.BodiesOfWater))
	for _, bow := range t.BodiesOfWater {
		ch := omnilogic.Chemistry{
			MspSystemID: siteID,
			BowName:     bow.Name,
			AirTemp:     t.AirTemp,
			SampledAt:   t.SampledAt,
		}
		// pH
		if caps.HasPHSensor {
			ch.PH = bow.PH
		} else {
			ch.NotEquipped = append(ch.NotEquipped, "ph")
		}
		// ORP
		if caps.HasORPSensor {
			ch.ORP = bow.ORP
		} else {
			ch.NotEquipped = append(ch.NotEquipped, "orp")
		}
		// Salt
		if caps.HasSaltSensor {
			ch.SaltPPM = bow.SaltPPM
		} else {
			ch.NotEquipped = append(ch.NotEquipped, "salt")
		}
		// Water temperature: respect the pump-flow-gating quirk.
		ch.WaterTemp, ch.TempState = projectWaterTemp(bow, caps)
		// Verdict ignores omitted sensors.
		ch.Verdict, ch.Reasons = omnilogic.ChemistryVerdict(ch.PH, ch.ORP, ch.SaltPPM)
		// When every chemistry sensor is unequipped, downgrade "unknown"
		// (which means "no data") to "not_equipped" (which means "this site
		// doesn't measure chemistry; verdict cannot speak to it").
		if !caps.HasPHSensor && !caps.HasORPSensor && !caps.HasSaltSensor {
			ch.Verdict = "not_equipped"
		}
		out = append(out, ch)
	}
	return out
}

// projectWaterTemp returns the water-temp reading and a state label that
// captures whether a -1 reading is the expected silence-while-pump-idle
// state or a real sensor offline event.
func projectWaterTemp(bow omnilogic.TelemetryBOW, caps store.SiteCapabilities) (*int, string) {
	if bow.WaterTemp == nil {
		return nil, ""
	}
	val := *bow.WaterTemp
	if val != -1 {
		return bow.WaterTemp, "ok"
	}
	// Sensor reported -1. If the install reports temp only with flow, check pumps.
	if caps.TempNeedsFlow {
		pumpsRunning := false
		for _, p := range bow.Pumps {
			if p.Speed != nil && *p.Speed > 0 {
				pumpsRunning = true
				break
			}
			if p.IsOn != nil && *p.IsOn {
				pumpsRunning = true
				break
			}
		}
		if !pumpsRunning {
			// Expected silence — strip the -1 so it doesn't read as a real reading.
			return nil, "n/a-pump-off"
		}
		// Pump is running but temp is still -1: real offline.
		return nil, "offline"
	}
	// No flow-gating configured; pass the -1 through with no state hint
	// (the consumer can decide whether to treat -1 as offline).
	return bow.WaterTemp, "offline"
}

// ---------- heater ----------

func newOmniHeaterCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "heater",
		Short: "Heater control: enable / disable / set-temp.",
	}
	cmd.AddCommand(newOmniHeaterEnableCmd(flags))
	cmd.AddCommand(newOmniHeaterDisableCmd(flags))
	cmd.AddCommand(newOmniHeaterSetTempCmd(flags))
	return cmd
}

func newOmniHeaterEnableCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	var bow string
	cmd := &cobra.Command{
		Use:     "enable [heater-name]",
		Short:   "Enable a heater. Heater stays on until you disable it (or it reaches its setpoint).",
		Example: "  hayward-omnilogic-pp-cli heater enable Gas --bow Pool",
		RunE:    runHeaterEnableDisable(flags, &siteID, &bow, true),
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	cmd.Flags().StringVar(&bow, "bow", "", "Constrain to a body-of-water by name (e.g. 'Pool' or 'Spa'). Useful when shared heaters appear under multiple BoWs.")
	return cmd
}

func newOmniHeaterDisableCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	var bow string
	cmd := &cobra.Command{
		Use:     "disable [heater-name]",
		Short:   "Disable a heater by name; if only one heater is on the BoW, omit the name.",
		Example: "  hayward-omnilogic-pp-cli heater disable Gas --bow Pool",
		RunE:    runHeaterEnableDisable(flags, &siteID, &bow, false),
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	cmd.Flags().StringVar(&bow, "bow", "", "Constrain to a body-of-water by name (e.g. 'Pool' or 'Spa').")
	return cmd
}

func runHeaterEnableDisable(flags *rootFlags, siteID *int, bowFilter *string, enable bool) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		if dryRunOK(flags) {
			return nil
		}
		if err := requireCredsUnlessDryRun(flags); err != nil {
			return classifyOmnilogicError(err)
		}
		c := newOmnilogicClient(flags.timeout)
		s, _ := openStore()
		defer closeStore(s)
		site, err := resolveSite(c, s, *siteID)
		if err != nil {
			return classifyOmnilogicError(err)
		}
		cfg, err := resolveMspConfig(c, s, site.MspSystemID)
		if err != nil {
			return classifyOmnilogicError(err)
		}
		bf := ""
		if bowFilter != nil {
			bf = *bowFilter
		}
		poolID, heaterID, h, err := omnilogic.ResolveHeaterInBoW(cfg, name, bf)
		if err != nil {
			return usageErr(err)
		}
		op := "SetHeaterEnable"
		target := fmt.Sprintf("%s (heater %d)", h.Name, heaterID)
		if flags.dryRun {
			logDryRun(s, site.MspSystemID, op, target, map[string]any{"enable": enable, "pool_id": poolID, "heater_id": heaterID})
			return printCommandPreview(cmd, op, target, map[string]any{"enable": enable})
		}
		result, err := c.SetHeaterEnable(site.MspSystemID, poolID, heaterID, enable)
		if err != nil {
			return classifyOmnilogicError(err)
		}
		logResult(s, site.MspSystemID, op, target, map[string]any{"enable": enable, "pool_id": poolID, "heater_id": heaterID}, result)
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
}

func newOmniHeaterSetTempCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	var bow string
	cmd := &cobra.Command{
		Use:     "set-temp [heater-name]",
		Short:   "Set a heater's target setpoint in °F (must fall within heater Min-Settable / Max-Settable from MSP config).",
		Example: "  hayward-omnilogic-pp-cli heater set-temp Gas --bow Pool --temp 84",
		RunE: func(cmd *cobra.Command, args []string) error {
			temp, _ := cmd.Flags().GetInt("temp")
			if temp == 0 {
				return cmd.Help()
			}
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			if dryRunOK(flags) {
				return nil
			}
			if err := requireCredsUnlessDryRun(flags); err != nil {
				return classifyOmnilogicError(err)
			}
			c := newOmnilogicClient(flags.timeout)
			s, _ := openStore()
			defer closeStore(s)
			site, err := resolveSite(c, s, siteID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			cfg, err := resolveMspConfig(c, s, site.MspSystemID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			poolID, heaterID, h, err := omnilogic.ResolveHeaterInBoW(cfg, name, bow)
			if err != nil {
				return usageErr(err)
			}
			// Range check
			if minS, _ := strconv.Atoi(h.MinSettableWaterTemp); minS > 0 && temp < minS {
				return usageErr(fmt.Errorf("temp %d is below heater min-settable (%d°F)", temp, minS))
			}
			if maxS, _ := strconv.Atoi(h.MaxSettableWaterTemp); maxS > 0 && temp > maxS {
				return usageErr(fmt.Errorf("temp %d is above heater max-settable (%d°F)", temp, maxS))
			}
			op := "SetUIHeaterCmd"
			target := fmt.Sprintf("%s (heater %d)", h.Name, heaterID)
			params := map[string]any{"temp": temp, "pool_id": poolID, "heater_id": heaterID}
			if flags.dryRun {
				logDryRun(s, site.MspSystemID, op, target, params)
				return printCommandPreview(cmd, op, target, map[string]any{"temp": temp})
			}
			result, err := c.SetHeaterTemp(site.MspSystemID, poolID, heaterID, temp)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			logResult(s, site.MspSystemID, op, target, params, result)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	cmd.Flags().StringVar(&bow, "bow", "", "Constrain to a body-of-water by name (e.g. 'Pool' or 'Spa').")
	cmd.Flags().Int("temp", 0, "Target temperature in °F.")
	return cmd
}

// ---------- pump ----------

func newOmniPumpCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pump",
		Short: "Variable-speed pump control.",
	}
	cmd.AddCommand(newOmniPumpSetSpeedCmd(flags))
	return cmd
}

func newOmniPumpSetSpeedCmd(flags *rootFlags) *cobra.Command {
	var siteID, speed int
	var bow string
	cmd := &cobra.Command{
		Use:     "set-speed [pump-name]",
		Short:   "Set a pump's running speed (range comes from MSP config). Sending 0 stops the pump.",
		Example: "  hayward-omnilogic-pp-cli pump set-speed 'Filter Pump' --bow Pool --speed 50",
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			if dryRunOK(flags) {
				return nil
			}
			if err := requireCredsUnlessDryRun(flags); err != nil {
				return classifyOmnilogicError(err)
			}
			c := newOmnilogicClient(flags.timeout)
			s, _ := openStore()
			defer closeStore(s)
			site, err := resolveSite(c, s, siteID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			cfg, err := resolveMspConfig(c, s, site.MspSystemID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			poolID, eqID, _, displayName, err := omnilogic.ResolveEquipmentInBoW(cfg, name, "pump", bow)
			if err != nil {
				return usageErr(err)
			}
			op := "SetUIEquipmentCmd"
			target := fmt.Sprintf("%s (pump %d)", displayName, eqID)
			params := map[string]any{"speed": speed, "pool_id": poolID, "pump_id": eqID}
			if flags.dryRun {
				logDryRun(s, site.MspSystemID, op, target, params)
				return printCommandPreview(cmd, op, target, map[string]any{"speed": speed})
			}
			result, err := c.SetPumpSpeed(site.MspSystemID, poolID, eqID, speed)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			logResult(s, site.MspSystemID, op, target, params, result)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	cmd.Flags().IntVar(&speed, "speed", 0, "Speed value (RPM or percent depending on pump). 0 stops the pump.")
	cmd.Flags().StringVar(&bow, "bow", "", "Constrain to a body-of-water by name (e.g. 'Pool' or 'Spa'). Useful when pumps in different BoWs share names.")
	return cmd
}

// ---------- equipment ----------

func newOmniEquipmentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "equipment",
		Short: "Generic on/off + timed-run for valves, relays, lights, accessory pumps.",
	}
	cmd.AddCommand(newOmniEquipmentOnCmd(flags))
	cmd.AddCommand(newOmniEquipmentOffCmd(flags))
	return cmd
}

func newOmniEquipmentOnCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	var forDur, bow string
	cmd := &cobra.Command{
		Use:     "on [equipment-name]",
		Short:   "Turn an equipment item on, optionally for a bounded duration.",
		Example: "  hayward-omnilogic-pp-cli equipment on 'Filter Pump' --bow Pool --for 1h",
		RunE:    runEquipmentOnOff(flags, &siteID, &forDur, &bow, true),
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	cmd.Flags().StringVar(&forDur, "for", "", "Run for a bounded duration (e.g. 30m, 1h, 2h30m). Omit to run indefinitely.")
	cmd.Flags().StringVar(&bow, "bow", "", "Constrain to a body-of-water by name (e.g. 'Pool' or 'Spa'). Useful when equipment in different BoWs share names.")
	return cmd
}

func newOmniEquipmentOffCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	var bow string
	cmd := &cobra.Command{
		Use:     "off [equipment-name]",
		Short:   "Turn an equipment item off.",
		Example: "  hayward-omnilogic-pp-cli equipment off 'Filter Pump' --bow Pool",
		RunE:    runEquipmentOnOff(flags, &siteID, nil, &bow, false),
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	cmd.Flags().StringVar(&bow, "bow", "", "Constrain to a body-of-water by name (e.g. 'Pool' or 'Spa').")
	return cmd
}

func runEquipmentOnOff(flags *rootFlags, siteID *int, forDur *string, bowFilter *string, on bool) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		if name == "" {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return nil
		}
		if err := requireCredsUnlessDryRun(flags); err != nil {
			return classifyOmnilogicError(err)
		}
		dur := 0
		if forDur != nil && *forDur != "" {
			var err error
			dur, err = omnilogic.ParseDuration(*forDur)
			if err != nil {
				return usageErr(err)
			}
		}
		c := newOmnilogicClient(flags.timeout)
		s, _ := openStore()
		defer closeStore(s)
		site, err := resolveSite(c, s, *siteID)
		if err != nil {
			return classifyOmnilogicError(err)
		}
		cfg, err := resolveMspConfig(c, s, site.MspSystemID)
		if err != nil {
			return classifyOmnilogicError(err)
		}
		bf := ""
		if bowFilter != nil {
			bf = *bowFilter
		}
		poolID, eqID, kind, display, err := omnilogic.ResolveEquipmentInBoW(cfg, name, "", bf)
		if err != nil {
			return usageErr(err)
		}
		// Hayward overloads SetUIEquipmentCmd's IsOn parameter: int 0-100
		// for VSP pumps, bool True/False for everything else. Sending a
		// bool against a VSP returns "Input string was not in a correct
		// format". Detect the VSP case and route through SetPumpSpeed,
		// using DefaultVSPOnSpeed for "on" (max). Power users wanting a
		// specific RPM/% should call `pump set-speed` directly.
		op := "SetUIEquipmentCmd"
		target := fmt.Sprintf("%s (%s %d)", display, kind, eqID)
		isVSP := omnilogic.IsVSPPump(cfg, eqID)
		params := map[string]any{
			"on":           on,
			"duration_min": dur,
			"pool_id":      poolID,
			"equipment_id": eqID,
			"bow":          bf,
			"is_vsp":       isVSP,
		}
		if flags.dryRun {
			logDryRun(s, site.MspSystemID, op, target, params)
			return printCommandPreview(cmd, op, target, params)
		}
		var result *omnilogic.CommandResult
		var callErr error
		if isVSP {
			speed := 0
			if on {
				speed = omnilogic.DefaultVSPOnSpeed
			}
			result, callErr = c.SetPumpSpeed(site.MspSystemID, poolID, eqID, speed)
		} else {
			result, callErr = c.SetEquipment(site.MspSystemID, poolID, eqID, on, dur)
		}
		if callErr != nil {
			return classifyOmnilogicError(callErr)
		}
		logResult(s, site.MspSystemID, op, target, params, result)
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
}

// ---------- spillover ----------

func newOmniSpilloverCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spillover",
		Short: "Pool-to-spa spillover control.",
	}
	cmd.AddCommand(newOmniSpilloverSetCmd(flags))
	return cmd
}

func newOmniSpilloverSetCmd(flags *rootFlags) *cobra.Command {
	var siteID, speed int
	var forDur string
	cmd := &cobra.Command{
		Use:     "set",
		Short:   "Set spillover speed and optional run duration.",
		Example: "  hayward-omnilogic-pp-cli spillover set --speed 75 --for 1h",
		RunE: func(cmd *cobra.Command, args []string) error {
			if speed == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if err := requireCredsUnlessDryRun(flags); err != nil {
				return classifyOmnilogicError(err)
			}
			dur, err := omnilogic.ParseDuration(forDur)
			if err != nil {
				return usageErr(err)
			}
			c := newOmnilogicClient(flags.timeout)
			s, _ := openStore()
			defer closeStore(s)
			site, err := resolveSite(c, s, siteID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			// Spillover is per-BoW; with shared pool+spa this is the spa BoW.
			cfg, err := resolveMspConfig(c, s, site.MspSystemID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			var poolID int
			for _, bow := range cfg.BodiesOfWater {
				if strings.EqualFold(bow.SupportsSpillover, "true") || strings.EqualFold(bow.SupportsSpillover, "yes") {
					poolID = atoiSafe(bow.SystemID)
					break
				}
			}
			if poolID == 0 && len(cfg.BodiesOfWater) > 0 {
				poolID = atoiSafe(cfg.BodiesOfWater[0].SystemID)
			}
			op := "SetUISpilloverCmd"
			target := fmt.Sprintf("pool %d", poolID)
			params := map[string]any{"speed": speed, "duration_min": dur, "pool_id": poolID}
			if flags.dryRun {
				logDryRun(s, site.MspSystemID, op, target, params)
				return printCommandPreview(cmd, op, target, map[string]any{"speed": speed, "duration_min": dur})
			}
			result, err := c.SetSpillover(site.MspSystemID, poolID, speed, dur)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			logResult(s, site.MspSystemID, op, target, params, result)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	cmd.Flags().IntVar(&speed, "speed", 0, "Spillover speed percent.")
	cmd.Flags().StringVar(&forDur, "for", "", "Duration (e.g. 1h). Omit to run indefinitely.")
	return cmd
}

// ---------- superchlor ----------

func newOmniSuperchlorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "superchlor",
		Short: "One-shot superchlorination on the salt chlorinator.",
	}
	cmd.AddCommand(newOmniSuperchlorToggleCmd(flags, true))
	cmd.AddCommand(newOmniSuperchlorToggleCmd(flags, false))
	return cmd
}

func newOmniSuperchlorToggleCmd(flags *rootFlags, on bool) *cobra.Command {
	var siteID int
	var bowName string
	verb := "off"
	example := "  hayward-omnilogic-pp-cli superchlor off"
	short := "Stop a superchlorination cycle early on the salt chlorinator; chlorination resumes its normal setpoint."
	if on {
		verb = "on"
		example = "  hayward-omnilogic-pp-cli superchlor on"
		short = "Start a one-shot superchlorination cycle on the salt chlorinator (24-hour boost) for shock-treating after heavy use or storms."
	}
	cmd := &cobra.Command{
		Use:     verb,
		Short:   short,
		Example: example,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := requireCredsUnlessDryRun(flags); err != nil {
				return classifyOmnilogicError(err)
			}
			c := newOmnilogicClient(flags.timeout)
			s, _ := openStore()
			defer closeStore(s)
			site, err := resolveSite(c, s, siteID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			cfg, err := resolveMspConfig(c, s, site.MspSystemID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			poolID, chlorID, err := omnilogic.ResolveChlor(cfg, bowName)
			if err != nil {
				return usageErr(err)
			}
			op := "SetUISuperCHLORCmd"
			target := fmt.Sprintf("chlor %d", chlorID)
			params := map[string]any{"on": on, "pool_id": poolID, "chlor_id": chlorID}
			if flags.dryRun {
				logDryRun(s, site.MspSystemID, op, target, params)
				return printCommandPreview(cmd, op, target, map[string]any{"on": on})
			}
			result, err := c.SetSuperchlor(site.MspSystemID, poolID, chlorID, on)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			logResult(s, site.MspSystemID, op, target, params, result)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	cmd.Flags().StringVar(&bowName, "bow", "", "Body-of-water name (when multiple chlorinators exist).")
	return cmd
}

// ---------- light ----------

func newOmniLightCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "light",
		Short: "ColorLogic light shows.",
	}
	cmd.AddCommand(newOmniLightListShowsCmd(flags))
	cmd.AddCommand(newOmniLightShowCmd(flags))
	return cmd
}

func newOmniLightListShowsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list-shows",
		Short:       "List every ColorLogic show with its ID and name (V2-only flag tells you which need a V2 light).",
		Example:     "  hayward-omnilogic-pp-cli light list-shows",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			shows := make([]omnilogic.LightShow, len(omnilogic.LightShows))
			copy(shows, omnilogic.LightShows)
			return emitJSONOrTable(cmd, flags, shows, []string{"id", "name", "v2_only"}, func(s omnilogic.LightShow) []string {
				return []string{strconv.Itoa(s.ID), s.Name, strconv.FormatBool(s.V2Only)}
			})
		},
	}
	return cmd
}

func newOmniLightShowCmd(flags *rootFlags) *cobra.Command {
	var siteID, speed, brightness int
	var lightName string
	cmd := &cobra.Command{
		Use:     "show <show-name-or-id> [--light-name X] [--speed N] [--brightness N]",
		Short:   "Activate a ColorLogic show. V2 lights also accept --speed and --brightness.",
		Example: "  hayward-omnilogic-pp-cli light show 'Tranquility' --light-name 'Pool Light'",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			show, ok := omnilogic.ResolveShow(args[0])
			if !ok {
				return usageErr(fmt.Errorf("unknown show %q — run 'light list-shows' to enumerate", args[0]))
			}
			if dryRunOK(flags) {
				return nil
			}
			if err := requireCredsUnlessDryRun(flags); err != nil {
				return classifyOmnilogicError(err)
			}
			c := newOmnilogicClient(flags.timeout)
			s, _ := openStore()
			defer closeStore(s)
			site, err := resolveSite(c, s, siteID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			cfg, err := resolveMspConfig(c, s, site.MspSystemID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			poolID, lightID, _, _, err := omnilogic.ResolveEquipment(cfg, lightName, "light")
			if err != nil {
				return usageErr(err)
			}
			isV2 := false
			for _, bow := range cfg.BodiesOfWater {
				for _, l := range bow.Lights {
					if atoiSafe(l.SystemID) == lightID && (l.V2Active == "yes" || l.V2Active == "true") {
						isV2 = true
					}
				}
			}
			op := "SetStandAloneLightShow"
			if isV2 {
				op = "SetStandAloneLightShowV2"
			}
			target := fmt.Sprintf("light %d", lightID)
			params := map[string]any{"show": show.ID, "show_name": show.Name, "speed": speed, "brightness": brightness, "v2": isV2}
			if flags.dryRun {
				logDryRun(s, site.MspSystemID, op, target, params)
				return printCommandPreview(cmd, op, target, params)
			}
			var result *omnilogic.CommandResult
			if isV2 {
				if speed == 0 {
					speed = 4
				}
				if brightness == 0 {
					brightness = 100
				}
				result, err = c.SetLightShowV2(site.MspSystemID, poolID, lightID, show.ID, speed, brightness)
			} else {
				result, err = c.SetLightShow(site.MspSystemID, poolID, lightID, show.ID)
			}
			if err != nil {
				return classifyOmnilogicError(err)
			}
			logResult(s, site.MspSystemID, op, target, params, result)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	cmd.Flags().StringVar(&lightName, "light-name", "", "Light name. Omit to apply to the first ColorLogic light found.")
	cmd.Flags().IntVar(&speed, "speed", 0, "(V2 only) Show speed 0-8.")
	cmd.Flags().IntVar(&brightness, "brightness", 0, "(V2 only) Brightness 0-100.")
	return cmd
}

// ---------- chlorinator ----------

func newOmniChlorinatorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chlorinator",
		Short: "Salt chlorinator configuration.",
	}
	cmd.AddCommand(newOmniChlorinatorSetParamsCmd(flags))
	return cmd
}

func newOmniChlorinatorSetParamsCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	var bowName, opMode, cellType string
	var timedPct, scTimeout, orpTimeout int
	cmd := &cobra.Command{
		Use:     "set-params",
		Short:   "Set chlorinator parameters. Defaults to current MSP values for any flag you don't pass.",
		Example: "  hayward-omnilogic-pp-cli chlorinator set-params --op-mode timed --timed-pct 50",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := requireCredsUnlessDryRun(flags); err != nil {
				return classifyOmnilogicError(err)
			}
			c := newOmnilogicClient(flags.timeout)
			s, _ := openStore()
			defer closeStore(s)
			site, err := resolveSite(c, s, siteID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			cfg, err := resolveMspConfig(c, s, site.MspSystemID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			poolID, chlorID, err := omnilogic.ResolveChlor(cfg, bowName)
			if err != nil {
				return usageErr(err)
			}
			cp := omnilogic.ChlorParams{MspSystemID: site.MspSystemID, PoolID: poolID, ChlorID: chlorID}
			if cmd.Flags().Changed("op-mode") {
				mode := opModeFor(opMode)
				if mode < 0 {
					return usageErr(fmt.Errorf("op-mode must be one of: timed, orp-autosense, disabled"))
				}
				cp.OpMode = &mode
			}
			if cmd.Flags().Changed("timed-pct") {
				if timedPct < 0 || timedPct > 100 {
					return usageErr(fmt.Errorf("timed-pct must be 0-100"))
				}
				cp.TimedPercent = &timedPct
			}
			if cmd.Flags().Changed("cell-type") {
				ct := cellTypeFor(cellType)
				if ct < 0 {
					return usageErr(fmt.Errorf("cell-type must be one of: T-3, T-5, T-9, T-15"))
				}
				cp.CellType = &ct
			}
			if cmd.Flags().Changed("sc-timeout") {
				cp.SCTimeout = &scTimeout
			}
			if cmd.Flags().Changed("orp-timeout") {
				cp.ORPTimeout = &orpTimeout
			}
			op := "SetCHLORParams"
			target := fmt.Sprintf("chlor %d", chlorID)
			params := map[string]any{
				"op_mode": opMode, "timed_pct": timedPct, "cell_type": cellType,
				"sc_timeout": scTimeout, "orp_timeout": orpTimeout,
				"pool_id": poolID, "chlor_id": chlorID,
			}
			if flags.dryRun {
				logDryRun(s, site.MspSystemID, op, target, params)
				return printCommandPreview(cmd, op, target, params)
			}
			result, err := c.SetChlorParams(cp)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			logResult(s, site.MspSystemID, op, target, params, result)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	cmd.Flags().StringVar(&bowName, "bow", "", "Body-of-water name (when multiple chlorinators exist).")
	cmd.Flags().StringVar(&opMode, "op-mode", "", "Operating mode: timed | orp-autosense | disabled.")
	cmd.Flags().IntVar(&timedPct, "timed-pct", 0, "Timed-mode chlorination percent (0-100).")
	cmd.Flags().StringVar(&cellType, "cell-type", "", "Cell type: T-3 | T-5 | T-9 | T-15.")
	cmd.Flags().IntVar(&scTimeout, "sc-timeout", 0, "Superchlorinate timeout in hours (1-96).")
	cmd.Flags().IntVar(&orpTimeout, "orp-timeout", 0, "ORP timeout in hours (1-96).")
	return cmd
}

func opModeFor(s string) int {
	switch strings.ToLower(s) {
	case "disabled", "off":
		return 0
	case "timed":
		return 1
	case "orp-autosense", "orp", "autosense":
		return 2
	}
	return -1
}

func cellTypeFor(s string) int {
	switch strings.ToUpper(s) {
	case "T-3", "T3":
		return 1
	case "T-5", "T5":
		return 2
	case "T-9", "T9":
		return 3
	case "T-15", "T15":
		return 4
	}
	return -1
}

// ---------- shared helpers ----------

func closeStore(s *store.Store) {
	if s != nil {
		_ = s.Close()
	}
}

func logDryRun(s *store.Store, siteMspSystemID int, op, target string, params map[string]any) {
	if s == nil {
		return
	}
	_, _ = s.LogCommand(store.CommandLogEntry{
		Op: op, Target: target, Params: withSiteParam(params, siteMspSystemID), Status: "dry-run", DryRun: true,
	})
}

func logResult(s *store.Store, siteMspSystemID int, op, target string, params map[string]any, r *omnilogic.CommandResult) {
	if s == nil || r == nil {
		return
	}
	_, _ = s.LogCommand(store.CommandLogEntry{
		Op: op, Target: target, Params: withSiteParam(params, siteMspSystemID), Status: r.Status, Detail: r.Detail,
	})
}

// withSiteParam injects msp_system_id into a command's params_json so the
// command-log replay dispatcher can re-resolve the site without needing to
// know which one was active when the original command ran. Returns a fresh
// map when params is nil and copies-then-augments when non-nil so callers
// don't see mutation of their map.
func withSiteParam(params map[string]any, siteMspSystemID int) map[string]any {
	out := make(map[string]any, len(params)+1)
	for k, v := range params {
		out[k] = v
	}
	out["msp_system_id"] = siteMspSystemID
	return out
}

func printCommandPreview(cmd *cobra.Command, op, target string, params map[string]any) error {
	preview := map[string]any{
		"dry_run":   true,
		"operation": op,
		"target":    target,
		"params":    params,
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(preview)
}

func emitJSONOrTable[T any](cmd *cobra.Command, flags *rootFlags, items []T, headers []string, row func(T) []string) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), items, flags)
	}
	rows := make([][]string, 0, len(items))
	for _, it := range items {
		rows = append(rows, row(it))
	}
	return flags.printTable(cmd, headers, rows)
}

// atoiSafe is a forgiving int parser; matches the omnilogic package's helper.
func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

// sortSitesByID is a stable helper used by sweep to surface results in a
// deterministic order across runs.
func sortSitesByID(sites []omnilogic.Site) {
	sort.Slice(sites, func(i, j int) bool { return sites[i].MspSystemID < sites[j].MspSystemID })
}
