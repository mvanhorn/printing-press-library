package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/bambu"
	"github.com/spf13/cobra"
)

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	var host string
	var failOn string
	cmd := &cobra.Command{Use: "doctor", Short: "Verify Bambu LAN credentials discovery TLS MQTT and FTPS", Example: "  bambu-pp-cli doctor --json", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, _ []string) error {
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"status": "dry-run", "checks": []string{"credentials", "ssdp", "tls_identity", "mqtt_pushall", "implicit_ftps"}}, flags)
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		started := time.Now()
		device, err := resolveBambuDevice(ctx, flags, host)
		if err != nil {
			return err
		}
		checks := map[string]any{"credentials": "OK configured", "ssdp_or_host": "OK private LAN target resolved", "tls_identity": "pending", "mqtt_pushall": "pending", "implicit_ftps": "pending"}
		client, err := bambu.Connect(ctx, device.Host, device.Serial, device.AccessCode)
		if err != nil {
			return apiErr(err)
		}
		checks["tls_identity"] = "OK Bambu CA chain and serial CN verified"
		snapshot, err := client.Status(ctx)
		client.Close()
		if err != nil {
			return apiErr(err)
		}
		checks["mqtt_pushall"] = "OK fresh status received"
		ftps, ftpErr := bambu.DialFTPS(ctx, device.Host, device.Serial, device.AccessCode)
		if ftpErr == nil {
			_, ftpErr = ftps.List("/cache", 1)
			_ = ftps.Close()
		}
		if ftpErr != nil {
			checks["implicit_ftps"] = "WARN " + ftpErr.Error()
		} else {
			checks["implicit_ftps"] = "OK control and resumed data TLS sessions"
		}
		view := map[string]any{"status": "OK", "printer": map[string]any{"serial": redactSerial(device.Serial), "name": device.Discovery.Name, "model": device.Discovery.Model}, "state": snapshot.State, "latency_ms": time.Since(started).Milliseconds(), "checks": checks}
		if ftpErr != nil && failOn == "warn" {
			_ = printJSONFiltered(cmd.OutOrStdout(), view, flags)
			return apiErr(fmt.Errorf("FTPS diagnostic warning: %w", ftpErr))
		}
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}}
	addHostFlag(cmd, &host)
	cmd.Flags().StringVar(&failOn, "fail-on", "error", "Failure threshold: error or warn")
	return cmd
}
