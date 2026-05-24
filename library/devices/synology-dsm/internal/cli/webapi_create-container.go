package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

func newWebapiCreateContainerCmd(flags *rootFlags) *cobra.Command {
	var bodyApi string
	var bodyMethod string
	var bodyVersion string
	var bodyImageName string
	var bodyName string
	var bodyCmd string
	var bodyHostname string
	var bodyDomainname string
	var bodyEntrypoint string
	var bodyNetworkMode string
	var bodyPortBindings string
	var bodyVolumeBindings string
	var bodyEnv string
	var bodyRestartPolicy string
	var bodyMemoryLimit string
	var bodyCpuShares int

	cmd := &cobra.Command{
		Use:         "create-container",
		Short:       "Create a new Docker container on the NAS",
		Long: `Create a new Docker container from an image. Pass container configuration
as flags. Port bindings, volume bindings, and env vars accept JSON arrays.

Example JSON formats:
  --port-bindings '[{"HostPort":"8080","HostIp":"","ContainerPort":80,"Protocol":"tcp"}]'
  --volume-bindings '["/volume1/docker/data:/data:rw"]'
  --env '["FOO=bar","BAZ=qux"]'`,
		Example: `  # Simple container
  synology-dsm-pp-cli webapi create-container --image-name hello-world --name test-hello

  # With port and volume mappings
  synology-dsm-pp-cli webapi create-container \
    --image-name nginx:latest \
    --name my-nginx \
    --port-bindings '[{"HostPort":"8080","HostIp":"","ContainerPort":80,"Protocol":"tcp"}]' \
    --volume-bindings '["/volume1/web:/usr/share/nginx/html:ro"]'`,
		Annotations: map[string]string{"pp:endpoint": "webapi.create-container", "pp:method": "POST", "pp:path": "/webapi/entry.cgi/docker/container/create"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("image-name") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "image-name")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/webapi/entry.cgi/docker/container/create"
			params := map[string]string{}
			fields := url.Values{}
			if bodyApi != "" {
				fields.Set("api", bodyApi)
			}
			if bodyMethod != "" {
				fields.Set("method", bodyMethod)
			}
			if bodyVersion != "" {
				fields.Set("version", bodyVersion)
			}
			if bodyImageName != "" {
				fields.Set("image_name", bodyImageName)
			}
			if bodyName != "" {
				fields.Set("name", bodyName)
			}
			if bodyCmd != "" {
				fields.Set("Cmd", bodyCmd)
			}
			if bodyHostname != "" {
				fields.Set("Hostname", bodyHostname)
			}
			if bodyDomainname != "" {
				fields.Set("Domainname", bodyDomainname)
			}
			if bodyEntrypoint != "" {
				fields.Set("Entrypoint", bodyEntrypoint)
			}
			if bodyNetworkMode != "" {
				fields.Set("NetworkMode", bodyNetworkMode)
			}
			if bodyPortBindings != "" {
				fields.Set("PortBindings", bodyPortBindings)
			}
			if bodyVolumeBindings != "" {
				fields.Set("Binds", bodyVolumeBindings)
			}
			if bodyEnv != "" {
				fields.Set("Env", bodyEnv)
			}
			if bodyRestartPolicy != "" {
				fields.Set("RestartPolicy", bodyRestartPolicy)
			}
			if bodyMemoryLimit != "" {
				fields.Set("Memory", bodyMemoryLimit)
			}
			if cmd.Flags().Changed("cpu-shares") {
				fields.Set("CpuShares", fmt.Sprintf("%d", bodyCpuShares))
			}

			data, statusCode, err := c.PostFormWithParams(path, params, fields)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
					} else {
						return nil
					}
				} else {
					var wrapped struct {
						Data []map[string]any `json:"data"`
					}
					if json.Unmarshal(data, &wrapped) == nil && len(wrapped.Data) > 0 {
						if err := printAutoTable(cmd.OutOrStdout(), wrapped.Data); err != nil {
							fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
						} else {
							return nil
						}
					}
				}
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				if flags.quiet {
					return nil
				}
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				envelope := map[string]any{
					"action":   "post",
					"resource": "webapi",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300,
				}
				if flags.dryRun {
					envelope["dry_run"] = true
					envelope["status"] = 0
					envelope["success"] = false
				}
				if len(filtered) > 0 {
					var parsed any
					if err := json.Unmarshal(filtered, &parsed); err == nil {
						envelope["data"] = parsed
					}
				}
				envelopeJSON, err := json.Marshal(envelope)
				if err != nil {
					return err
				}
				return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&bodyApi, "api", "SYNO.Docker.Container", "DSM API name")
	cmd.Flags().StringVar(&bodyMethod, "method", "create", "API method")
	cmd.Flags().StringVar(&bodyVersion, "version", "1", "API version")
	cmd.Flags().StringVar(&bodyImageName, "image-name", "", "Image name (e.g. nginx:latest) (required)")
	cmd.Flags().StringVar(&bodyName, "name", "", "Container name")
	cmd.Flags().StringVar(&bodyCmd, "cmd", "", "Command to run (JSON array)")
	cmd.Flags().StringVar(&bodyHostname, "hostname", "", "Container hostname")
	cmd.Flags().StringVar(&bodyDomainname, "domainname", "", "Container domain name")
	cmd.Flags().StringVar(&bodyEntrypoint, "entrypoint", "", "Entrypoint (JSON array)")
	cmd.Flags().StringVar(&bodyNetworkMode, "network-mode", "", "Network mode (e.g. bridge, host)")
	cmd.Flags().StringVar(&bodyPortBindings, "port-bindings", "", "Port bindings (JSON array)")
	cmd.Flags().StringVar(&bodyVolumeBindings, "volume-bindings", "", "Volume bindings (JSON array)")
	cmd.Flags().StringVar(&bodyEnv, "env", "", "Environment variables (JSON array)")
	cmd.Flags().StringVar(&bodyRestartPolicy, "restart-policy", "", "Restart policy (no, always, on-failure)")
	cmd.Flags().StringVar(&bodyMemoryLimit, "memory", "", "Memory limit in bytes")
	cmd.Flags().IntVar(&bodyCpuShares, "cpu-shares", 0, "CPU shares (relative weight)")

	return cmd
}
