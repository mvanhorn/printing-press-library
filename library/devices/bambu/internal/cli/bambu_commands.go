package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/bambu"
	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/config"
	"github.com/spf13/cobra"
)

type deviceContext struct {
	Host       string
	Serial     string
	AccessCode string
	Discovery  bambu.Discovery
}

func validateBambuDataSource(cmd *cobra.Command, flags *rootFlags) error {
	if flags == nil {
		return nil
	}
	path := strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()+" ")
	if flags.printerName != "" && (path == "search" || path == "sql") {
		return usageErr(fmt.Errorf("%s does not apply --printer automatically; query the persisted printer_key field explicitly", path))
	}
	if flags.dataSource == "auto" {
		return nil
	}
	localOnly := path == "observations" || path == "search" || path == "sql" || path == "maintenance" || strings.HasPrefix(path, "maintenance ") || strings.HasPrefix(path, "history ") || path == "history" || path == "job repeats" || path == "job timeline" || path == "printer field-diff"
	liveOnly := path == "discover" || path == "doctor" || path == "live" || path == "sync" || path == "tail" || strings.HasPrefix(path, "events ") || strings.HasPrefix(path, "files ") || strings.HasPrefix(path, "ams ") || (strings.HasPrefix(path, "printer ") && path != "printer field-diff") || (strings.HasPrefix(path, "job ") && path != "job repeats" && path != "job timeline")
	if localOnly && flags.dataSource == "live" {
		return usageErr(fmt.Errorf("%s uses persisted local history and does not support --data-source live", path))
	}
	if liveOnly && flags.dataSource == "local" {
		return usageErr(fmt.Errorf("%s requires the printer LAN and does not support --data-source local", path))
	}
	return nil
}

func selectedPrinterSerial(flags *rootFlags) (string, error) {
	if flags.printerName != "" {
		store, err := loadBambuPrinterProfiles()
		if err != nil {
			return "", configErr(err)
		}
		profile, ok := store.Profiles[flags.printerName]
		if !ok {
			return "", notFoundErr(fmt.Errorf("printer profile %q was not found", flags.printerName))
		}
		serial := strings.TrimSpace(os.Getenv(profile.SerialEnv))
		if serial == "" {
			return "", authErr(fmt.Errorf("printer profile %q requires environment variable %s", flags.printerName, profile.SerialEnv))
		}
		if err := bambu.ValidateSerial(serial); err != nil {
			return "", authErr(fmt.Errorf("printer profile %q has invalid serial in %s: %w", flags.printerName, profile.SerialEnv, err))
		}
		return serial, nil
	}
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return "", configErr(err)
	}
	serial := strings.TrimSpace(cfg.BambuSerial)
	if serial == "" {
		return "", authErr(fmt.Errorf("BAMBU_SERIAL is required for printer-scoped history; configure the default printer or pass --printer"))
	}
	if err := bambu.ValidateSerial(serial); err != nil {
		return "", authErr(fmt.Errorf("BAMBU_SERIAL: %w", err))
	}
	return serial, nil
}

func selectedPrinterScope(ctx context.Context, flags *rootFlags, history *bambu.History) (string, error) {
	serial, err := selectedPrinterSerial(flags)
	if err != nil {
		return "", err
	}
	canonical := bambu.PrinterKey(serial)
	if err := history.ClaimLegacyPrinterKeys(ctx, canonical, bambu.LegacyPrinterKeys(serial)...); err != nil {
		return "", configErr(err)
	}
	return canonical, nil
}

func resolveBambuDevice(ctx context.Context, flags *rootFlags, host string) (deviceContext, error) {
	if flags.printerName != "" {
		if host != "" {
			return deviceContext{}, usageErr(fmt.Errorf("--host and --printer cannot be used together"))
		}
		device, err := resolveNamedBambuPrinter(ctx, flags.printerName)
		if err != nil {
			return deviceContext{}, err
		}
		flags.outputSource = "live"
		return device, nil
	}
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return deviceContext{}, configErr(fmt.Errorf("load Bambu configuration: %w", err))
	}
	serial := strings.TrimSpace(cfg.BambuSerial)
	accessCode := strings.TrimSpace(cfg.BambuAccessCode)
	if serial == "" {
		return deviceContext{}, authErr(fmt.Errorf("BAMBU_SERIAL is required; set it from the printer's LAN settings"))
	}
	if accessCode == "" {
		return deviceContext{}, authErr(fmt.Errorf("BAMBU_ACCESS_CODE is required; set it from the printer's LAN settings"))
	}
	flags.outputSource = "live"
	if host == "" {
		host = strings.TrimSpace(os.Getenv("BAMBU_HOST"))
	}
	if host != "" {
		if err := bambu.ValidatePrivateIP(host); err != nil {
			return deviceContext{}, usageErr(err)
		}
	}
	discovered := bambu.Discovery{Host: host, Serial: serial}
	if host == "" {
		results, err := bambu.Discover(ctx, serial)
		if err != nil {
			return deviceContext{}, apiErr(err)
		}
		if len(results) == 0 {
			return deviceContext{}, notFoundErr(fmt.Errorf("printer %s was not found by SSDP; ensure it is online on this LAN or set BAMBU_HOST", redactSerial(serial)))
		}
		discovered = results[0]
		host = discovered.Host
	}
	return deviceContext{Host: host, Serial: serial, AccessCode: accessCode, Discovery: discovered}, nil
}

func redactSerial(serial string) string {
	if len(serial) <= 6 {
		return "configured serial"
	}
	return serial[:3] + "..." + serial[len(serial)-3:]
}

func fetchBambuStatus(ctx context.Context, flags *rootFlags, host string) (deviceContext, bambu.Snapshot, error) {
	device, err := resolveBambuDevice(ctx, flags, host)
	if err != nil {
		return deviceContext{}, bambu.Snapshot{}, err
	}
	client, err := bambu.Connect(ctx, device.Host, device.Serial, device.AccessCode)
	if err != nil {
		return deviceContext{}, bambu.Snapshot{}, apiErr(err)
	}
	defer client.Close()
	snapshot, err := client.Status(ctx)
	if err != nil {
		return deviceContext{}, bambu.Snapshot{}, apiErr(err)
	}
	return device, snapshot, nil
}

func addHostFlag(cmd *cobra.Command, host *string) {
	cmd.Flags().StringVar(host, "host", "", "Printer private-LAN host; defaults to BAMBU_HOST then SSDP discovery")
}

func newBambuDiscoverCmd(flags *rootFlags) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover Bambu printers on the private LAN",
		Example: strings.Trim(`
  bambu-pp-cli discover --agent
  bambu-pp-cli discover --all --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags.outputSource = "live"
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_probe": "SSDP", "service": bambu.SSDPService, "printer": flags.printerName}, flags)
			}
			if flags.printerName != "" && all {
				return usageErr(fmt.Errorf("--all and --printer cannot be used together"))
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			serial := cfg.BambuSerial
			if flags.printerName != "" {
				profiles, err := loadBambuPrinterProfiles()
				if err != nil {
					return configErr(err)
				}
				profile, ok := profiles.Profiles[flags.printerName]
				if !ok {
					return notFoundErr(fmt.Errorf("printer profile %q was not found", flags.printerName))
				}
				serial = strings.TrimSpace(os.Getenv(profile.SerialEnv))
				if serial == "" {
					return authErr(fmt.Errorf("printer profile %q requires environment variable %s", flags.printerName, profile.SerialEnv))
				}
			}
			if all {
				serial = ""
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			results, err := bambu.Discover(ctx, serial)
			if err != nil {
				return apiErr(err)
			}
			redacted := make([]map[string]any, 0, len(results))
			for _, result := range results {
				redacted = append(redacted, map[string]any{"host": result.Host, "serial": redactSerial(result.Serial), "model": result.Model, "name": result.Name})
			}
			return printJSONFiltered(cmd.OutOrStdout(), redacted, flags)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Return all Bambu printers instead of filtering to BAMBU_SERIAL")
	return cmd
}

func newBambuPrinterStatusCmd(flags *rootFlags) *cobra.Command {
	var host string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Fetch a fresh normalized Bambu MQTT status snapshot",
		Example: strings.Trim(`
  bambu-pp-cli printer status --agent
  bambu-pp-cli printer status --json --select state,job_name,percent,remaining_minutes`, "\n"),
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_request": "MQTT pushall"}, flags)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			device, snapshot, err := fetchBambuStatus(ctx, flags, host)
			if err != nil {
				return err
			}
			if err := persistSnapshot(ctx, snapshot); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: status was fetched but could not be persisted: %v\n", err)
			}
			view := statusView(device, snapshot)
			if flags.compact {
				view = compactStatusView(device, snapshot)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	addHostFlag(cmd, &host)
	return cmd
}

func compactStatusView(device deviceContext, snapshot bambu.Snapshot) map[string]any {
	finishAt := any(nil)
	if snapshot.RemainingMinutes != nil && *snapshot.RemainingMinutes > 0 {
		finishAt = snapshot.ObservedAt.Add(time.Duration(*snapshot.RemainingMinutes) * time.Minute).Format(time.RFC3339)
	}
	active := activeAMSView(snapshot.Raw)
	delete(active, "ams")
	delete(active, "virtual_slot")
	return map[string]any{
		"observed_at":  snapshot.ObservedAt,
		"printer":      map[string]any{"serial": redactSerial(device.Serial), "name": device.Discovery.Name, "model": device.Discovery.Model},
		"state":        snapshot.State,
		"job":          map[string]any{"name": snapshot.JobName, "percent": snapshot.Percent, "remaining_minutes": snapshot.RemainingMinutes, "estimated_finish_at": finishAt, "current_layer": snapshot.CurrentLayer, "total_layers": snapshot.TotalLayers},
		"temperatures": temperatureView(snapshot.Raw), "ams_active": active, "print_error": snapshot.PrintError,
	}
}

func statusView(device deviceContext, snapshot bambu.Snapshot) map[string]any {
	finishAt := any(nil)
	if snapshot.RemainingMinutes != nil && *snapshot.RemainingMinutes > 0 {
		finishAt = snapshot.ObservedAt.Add(time.Duration(*snapshot.RemainingMinutes) * time.Minute).Format(time.RFC3339)
	}
	return map[string]any{
		"observed_at": snapshot.ObservedAt,
		"printer":     map[string]any{"serial": redactSerial(device.Serial), "host": device.Host, "name": device.Discovery.Name, "model": device.Discovery.Model},
		"state":       snapshot.State,
		"job": map[string]any{
			"name": snapshot.JobName, "task_id": snapshot.TaskID, "subtask_id": snapshot.SubtaskID,
			"file": snapshot.GCodeFile, "plate": snapshot.PlateNumber, "percent": snapshot.Percent,
			"remaining_minutes": snapshot.RemainingMinutes, "estimated_finish_at": finishAt,
			"current_layer": snapshot.CurrentLayer, "total_layers": snapshot.TotalLayers,
		},
		"temperatures": temperatureView(snapshot.Raw),
		"ams":          amsView(snapshot.Raw),
		"print_error":  snapshot.PrintError,
	}
}

func newBambuPrinterRawCmd(flags *rootFlags) *cobra.Command {
	var host string
	cmd := &cobra.Command{
		Use:         "raw",
		Short:       "Fetch the complete redacted MQTT print report",
		Example:     "  bambu-pp-cli printer raw --agent",
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_request": "MQTT pushall", "redacted": true}, flags)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			_, snapshot, err := fetchBambuStatus(ctx, flags, host)
			if err != nil {
				return err
			}
			if err := persistSnapshot(ctx, snapshot); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: raw report was fetched but could not be persisted: %v\n", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), redactRaw(snapshot.Raw), flags)
		},
	}
	addHostFlag(cmd, &host)
	return cmd
}

func newBambuDerivedCmd(flags *rootFlags, family, use, short string, derive func(bambu.Snapshot) any) *cobra.Command {
	var host string
	cmd := &cobra.Command{
		Use: use, Short: short, Example: "  bambu-pp-cli " + family + " " + use + " --agent", Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_request": "MQTT pushall", "view": use}, flags)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			_, snapshot, err := fetchBambuStatus(ctx, flags, host)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), derive(snapshot), flags)
		},
	}
	addHostFlag(cmd, &host)
	return cmd
}

func temperatureView(raw map[string]any) map[string]any {
	result := fields(raw, "bed_temper", "bed_target_temper", "nozzle_temper", "nozzle_target_temper", "chamber_temper")
	if nozzles, activeID, ok := h2dNozzleTemperatures(raw); ok {
		result["nozzles"] = nozzles
		result["active_nozzle_id"] = activeID
		for _, nozzle := range nozzles {
			if nozzle["id"] == activeID {
				result["nozzle_temper"] = nozzle["current_temper"]
				result["nozzle_target_temper"] = nozzle["target_temper"]
				break
			}
		}
	}
	ams, _ := raw["ams"].(map[string]any)
	units, _ := ams["ams"].([]any)
	amsTemps := make([]map[string]any, 0, len(units))
	for _, value := range units {
		unit, _ := value.(map[string]any)
		amsTemps = append(amsTemps, map[string]any{"id": unit["id"], "temperature": unit["temp"], "humidity": unit["humidity"]})
	}
	if len(amsTemps) > 0 {
		result["ams"] = amsTemps
	}
	return result
}

func h2dNozzleTemperatures(raw map[string]any) ([]map[string]any, int, bool) {
	device, _ := raw["device"].(map[string]any)
	extruder, _ := device["extruder"].(map[string]any)
	stateValue, ok := numberValue(extruder["state"])
	if !ok {
		return nil, 0, false
	}
	state := uint64(stateValue)
	count := int(state & 0x0f)
	activeID := int((state >> 4) & 0x0f)
	info, _ := extruder["info"].([]any)
	if count < 2 || activeID >= count || len(info) < count {
		return nil, 0, false
	}
	nozzles := make([]map[string]any, 0, count)
	for _, value := range info[:count] {
		item, _ := value.(map[string]any)
		idValue, idOK := numberValue(item["id"])
		packedValue, packedOK := numberValue(item["temp"])
		if !idOK || !packedOK || packedValue < 0 {
			return nil, 0, false
		}
		packed := uint64(packedValue)
		nozzles = append(nozzles, map[string]any{
			"id":             int(idValue),
			"active":         int(idValue) == activeID,
			"current_temper": int(packed & 0xffff),
			"target_temper":  int((packed >> 16) & 0xffff),
		})
	}
	return nozzles, activeID, true
}

func fanView(raw map[string]any) map[string]any {
	return fields(raw, "cooling_fan_speed", "aux_part_fan", "big_fan1_speed", "big_fan2_speed", "heatbreak_fan_speed", "fan_gear")
}

func capabilityView(raw map[string]any) map[string]any {
	return fields(raw, "lights_report", "spd_lvl", "spd_mag", "wifi_signal", "sdcard", "ver", "nozzle_type", "nozzle_diameter", "device", "hw_switch_state")
}

func serviceView(raw map[string]any) map[string]any {
	return fields(raw, "queue", "queue_sts", "queue_number", "queue_total", "queue_est", "upload", "upgrade_state", "ipcam", "xcam", "xcam_status")
}

func normalizedHealth(snapshot bambu.Snapshot) map[string]any {
	messages := make([]map[string]any, 0)
	if rawMessages, ok := snapshot.Raw["hms"].([]any); ok {
		for _, value := range rawMessages {
			message, _ := value.(map[string]any)
			code := fmt.Sprint(message["code"])
			messages = append(messages, map[string]any{"code": code, "attr": message["attr"], "wiki_url": "https://wiki.bambulab.com/en/x1/troubleshooting/hmscode", "raw": message})
		}
	}
	return map[string]any{"observed_at": snapshot.ObservedAt, "state": snapshot.State, "print_error": snapshot.PrintError, "messages": messages, "mc_err": snapshot.Raw["mc_err"], "fail_reason": snapshot.Raw["fail_reason"]}
}

func newBambuPrinterHealthCmd(flags *rootFlags) *cobra.Command {
	var host string
	var historyLimit int
	cmd := &cobra.Command{Use: "health", Short: "Show normalized current HMS/error codes and persisted health history", Example: "  bambu-pp-cli printer health --agent", Annotations: map[string]string{"mcp:local-write": "true"}, RunE: func(cmd *cobra.Command, _ []string) error {
		if historyLimit < 1 || historyLimit > 1000 {
			return usageErr(fmt.Errorf("--history-limit must be between 1 and 1000"))
		}
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"current": map[string]any{}, "history": []any{}}, flags)
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		_, snapshot, err := fetchBambuStatus(ctx, flags, host)
		if err != nil {
			return err
		}
		_ = persistSnapshot(ctx, snapshot)
		history, err := openBambuHistory(ctx)
		if err != nil {
			return configErr(err)
		}
		defer history.Close()
		printerKey, err := selectedPrinterScope(ctx, flags, history)
		if err != nil {
			return err
		}
		snapshots, err := history.HealthSnapshots(ctx, printerKey, historyLimit)
		if err != nil {
			return configErr(err)
		}
		items := make([]map[string]any, 0, len(snapshots))
		for _, stored := range snapshots {
			items = append(items, normalizedHealth(stored))
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"current": normalizedHealth(snapshot), "history": items}, flags)
	}}
	addHostFlag(cmd, &host)
	cmd.Flags().IntVar(&historyLimit, "history-limit", 25, "Maximum persisted health observations to return")
	return cmd
}

func amsView(raw map[string]any) map[string]any {
	return fields(raw, "ams", "ams_status", "ams_rfid_status", "vir_slot", "tray_now", "tray_tar")
}

func activeAMSView(raw map[string]any) map[string]any {
	active := nestedInt(raw["ams"], "tray_now")
	if active == nil {
		active = intFromAny(raw["tray_now"])
	}
	source := "unknown"
	if active != nil {
		switch {
		case *active >= 0 && *active <= 15:
			source = "ams_tray"
		case *active == 254:
			source = "external_spool"
		case *active == 255:
			source = "unloaded"
		}
	}
	return map[string]any{"tray_now": active, "source": source, "ams": raw["ams"], "virtual_slot": raw["vir_slot"]}
}

func amsServiceView(raw map[string]any) map[string]any {
	return fields(raw, "ams", "ams_status", "ams_rfid_status", "mapping")
}

func currentJobView(snapshot bambu.Snapshot) any {
	return map[string]any{"observed_at": snapshot.ObservedAt, "state": snapshot.State, "name": snapshot.JobName, "task_id": snapshot.TaskID, "subtask_id": snapshot.SubtaskID, "file": snapshot.GCodeFile, "plate": snapshot.PlateNumber, "percent": snapshot.Percent, "remaining_minutes": snapshot.RemainingMinutes, "current_layer": snapshot.CurrentLayer, "total_layers": snapshot.TotalLayers, "stage": snapshot.Raw["mc_print_stage"], "sub_stage": snapshot.Raw["mc_print_sub_stage"], "speed_level": snapshot.Raw["spd_lvl"], "print_error": snapshot.PrintError}
}

func fields(raw map[string]any, names ...string) map[string]any {
	result := make(map[string]any, len(names))
	for _, name := range names {
		if value, ok := raw[name]; ok {
			result[name] = bambu.Redact(value)
		}
	}
	return result
}

func nestedInt(value any, key string) *int {
	object, _ := value.(map[string]any)
	return intFromAny(object[key])
}

func intFromAny(value any) *int {
	var n int
	switch typed := value.(type) {
	case float64:
		n = int(typed)
	case int:
		n = typed
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return nil
		}
		n = int(parsed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return nil
		}
		n = parsed
	default:
		return nil
	}
	return &n
}

func redactRaw(raw map[string]any) map[string]any {
	redacted, _ := bambu.Redact(raw).(map[string]any)
	return redacted
}

func newBambuPrinterWatchCmd(flags *rootFlags) *cobra.Command {
	var host string
	var maxEvents int
	cmd := &cobra.Command{
		Use:         "watch",
		Short:       "Stream normalized MQTT snapshots and print transitions as NDJSON",
		Example:     "  bambu-pp-cli printer watch --max-events 1 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if maxEvents < 0 {
				return usageErr(fmt.Errorf("--max-events must be zero or greater"))
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_stream": "MQTT reports"}, flags)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			device, err := resolveBambuDevice(ctx, flags, host)
			if err != nil {
				return err
			}
			client, err := bambu.Connect(ctx, device.Host, device.Serial, device.AccessCode)
			if err != nil {
				return apiErr(err)
			}
			defer client.Close()
			if err := client.RequestPushAll(ctx); err != nil {
				return apiErr(err)
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			count := 0
			for maxEvents <= 0 || count < maxEvents {
				snapshot, events, err := client.Next(ctx)
				if err != nil {
					return apiErr(err)
				}
				if snapshot.ObservedAt.IsZero() {
					continue
				}
				if err := encoder.Encode(map[string]any{"type": "printer.snapshot", "snapshot": statusView(device, snapshot), "transitions": events}); err != nil {
					return err
				}
				count++
			}
			return nil
		},
	}
	addHostFlag(cmd, &host)
	cmd.Flags().IntVar(&maxEvents, "max-events", 0, "Stop after this many MQTT reports; zero watches until timeout or cancellation")
	return cmd
}

func newTailCmd(flags *rootFlags) *cobra.Command {
	cmd := newBambuPrinterWatchCmd(flags)
	cmd.Use = "tail"
	cmd.Short = "Stream live Bambu MQTT snapshots and transitions as NDJSON"
	cmd.Example = "  bambu-pp-cli tail --max-events 1 --agent"
	return cmd
}

func newBambuMetadataCmd(flags *rootFlags, mode string) *cobra.Command {
	var host string
	var output string
	use, short := mode, "Read exact-current-job 3MF metadata over implicit FTPS"
	if mode == "thumbnail" {
		short = "Write the validated current plate thumbnail to an explicit path"
	}
	if mode == "objects" {
		short = "Show printable objects from the exact current 3MF"
	}
	example := "  bambu-pp-cli job " + use + " --agent"
	if mode == "thumbnail" {
		example = "  bambu-pp-cli job thumbnail --output ./bambu_plate.png --agent"
	}
	cmd := &cobra.Command{
		Use: use, Short: short, Example: example, Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_read": "exact current 3MF", "mode": mode}, flags)
			}
			if mode == "thumbnail" && output == "" {
				return usageErr(fmt.Errorf("--output is required for job thumbnail"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			device, snapshot, err := fetchBambuStatus(ctx, flags, host)
			if err != nil {
				return err
			}
			ftps, err := bambu.DialFTPS(ctx, device.Host, device.Serial, device.AccessCode)
			if err != nil {
				return apiErr(err)
			}
			defer ftps.Close()
			metadata, err := ftps.JobMetadata(snapshot)
			if err != nil {
				return apiErr(err)
			}
			switch mode {
			case "objects":
				return printJSONFiltered(cmd.OutOrStdout(), metadata.Objects, flags)
			case "thumbnail":
				if len(metadata.Thumbnail) == 0 {
					return notFoundErr(fmt.Errorf("current 3MF has no validated plate thumbnail"))
				}
				if err := writePrivateFile(output, metadata.Thumbnail); err != nil {
					return configErr(err)
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"path": filepath.Clean(output), "filename": metadata.ThumbnailName, "content_type": "image/png", "bytes": len(metadata.Thumbnail), "source_3mf": metadata.SourcePath}, flags)
			default:
				metadata.Thumbnail = nil
				return printJSONFiltered(cmd.OutOrStdout(), metadata, flags)
			}
		},
	}
	addHostFlag(cmd, &host)
	if mode == "thumbnail" {
		cmd.Annotations = nil
		cmd.Flags().StringVarP(&output, "output", "o", "", "Explicit output path for the current plate PNG")
	}
	return cmd
}

func writePrivateFile(output string, payload []byte) error {
	clean := filepath.Clean(output)
	if info, err := os.Lstat(clean); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to overwrite symlink %q", clean)
	}
	if err := os.MkdirAll(filepath.Dir(clean), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(clean), ".bambu-asset-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		return err
	}
	return os.Rename(name, clean)
}

func newBambuFilesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "files", Short: "Read printer files over certificate-verified implicit FTPS", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newBambuFilesListCmd(flags), newBambuFilesDownloadCmd(flags))
	return cmd
}

func newBambuFilesListCmd(flags *rootFlags) *cobra.Command {
	var host, remotePath string
	var limit int
	cmd := &cobra.Command{Use: "list", Short: "List bounded printer files over implicit FTPS", Example: "  bambu-pp-cli files list --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, _ []string) error {
		if limit < 1 || limit > 1000 {
			return usageErr(fmt.Errorf("--limit must be between 1 and 1000"))
		}
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_list": remotePath}, flags)
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		device, err := resolveBambuDevice(ctx, flags, host)
		if err != nil {
			return err
		}
		ftps, err := bambu.DialFTPS(ctx, device.Host, device.Serial, device.AccessCode)
		if err != nil {
			return apiErr(err)
		}
		defer ftps.Close()
		files, err := ftps.List(remotePath, limit)
		if err != nil {
			return apiErr(err)
		}
		return printJSONFiltered(cmd.OutOrStdout(), files, flags)
	}}
	addHostFlag(cmd, &host)
	cmd.Flags().StringVar(&remotePath, "path", "/", "Remote printer directory to list")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum directory entries to return")
	return cmd
}

func newBambuFilesDownloadCmd(flags *rootFlags) *cobra.Command {
	var host, remotePath, output string
	var maxBytes int64
	cmd := &cobra.Command{Use: "download", Short: "Download one bounded printer file to an explicit local path", Example: "  bambu-pp-cli files download --path /cache/current.gcode.3mf --output ./current.3mf --dry-run", RunE: func(cmd *cobra.Command, _ []string) error {
		if maxBytes < 1 || maxBytes > bambu.MaxArchiveBytes {
			return usageErr(fmt.Errorf("--max-bytes must be between 1 and %d", bambu.MaxArchiveBytes))
		}
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_download": remotePath, "output": output}, flags)
		}
		if remotePath == "" || output == "" {
			return usageErr(fmt.Errorf("--path and --output are required"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		device, err := resolveBambuDevice(ctx, flags, host)
		if err != nil {
			return err
		}
		ftps, err := bambu.DialFTPS(ctx, device.Host, device.Serial, device.AccessCode)
		if err != nil {
			return apiErr(err)
		}
		defer ftps.Close()
		written, err := ftps.Download(remotePath, output, maxBytes)
		if err != nil {
			return apiErr(err)
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"remote_path": remotePath, "output_path": filepath.Clean(output), "bytes": written}, flags)
	}}
	addHostFlag(cmd, &host)
	cmd.Flags().StringVar(&remotePath, "path", "", "Exact remote printer file path")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Explicit safe local output path")
	cmd.Flags().Int64Var(&maxBytes, "max-bytes", bambu.MaxArchiveBytes, "Maximum bytes accepted before aborting the download")
	return cmd
}

func newBambuTopLevelCommands(flags *rootFlags) []*cobra.Command {
	return []*cobra.Command{
		newBambuDiscoverCmd(flags), newBambuEventsCmd(flags), newBambuFilesCmd(flags),
		newBambuMaintenanceCmd(flags),
	}
}

func sortMapKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

var _ = cliutil.IsVerifyEnv
