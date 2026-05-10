package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

func newServicesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "services",
		Short: "Manage and call Home Assistant services",
	}
	cmd.AddCommand(newServicesCallCmd(flags))
	cmd.AddCommand(newServicesPayloadCmd(flags))
	return cmd
}

func newServicesCallCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "call <domain.service> [payload_json]",
		Short: "Call a service (e.g. light.turn_on '{\"entity_id\":\"light.kitchen\"}')",
		Example: `  # Turn on a light
  homeassistant-pp-cli services call light.turn_on '{"entity_id":"light.living_room"}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			parts := strings.Split(args[0], ".")
			if len(parts) != 2 {
				return fmt.Errorf("service must be in domain.service format (e.g., light.turn_on)")
			}
			domain, service := parts[0], parts[1]

			payload := "{}"
			if len(args) > 1 {
				payload = args[1]
				if !json.Valid([]byte(payload)) {
					return fmt.Errorf("invalid JSON payload: %s", payload)
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/services/%s/%s", url.PathEscape(domain), url.PathEscape(service))
			var body map[string]any
			if err := json.Unmarshal([]byte(payload), &body); err != nil {
				return fmt.Errorf("invalid JSON payload: %w", err)
			}

			data, _, err := c.Post(path, body)
			if err != nil {
				return err
			}

			var states []map[string]interface{}
			if err := json.Unmarshal(data, &states); err == nil && len(states) > 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), states, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Service %s.%s called successfully. Affected states:\n", domain, service)
				for _, st := range states {
					fmt.Fprintf(cmd.OutOrStdout(), " - %s is now %s\n", st["entity_id"], st["state"])
				}
			} else {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), string(data))
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Service %s.%s called successfully.\n", domain, service)
				}
			}
			return nil
		},
	}
	return cmd
}

func newServicesPayloadCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "payload <domain.service>",
		Short: "Print the exact JSON payload expected by any Home Assistant service",
		Example: `  # See the payload schema for light.turn_on
  homeassistant-pp-cli services payload light.turn_on`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			parts := strings.Split(args[0], ".")
			if len(parts) != 2 {
				return fmt.Errorf("service must be in domain.service format (e.g., light.turn_on)")
			}
			domain, service := parts[0], parts[1]

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Get("/services", nil)
			if err != nil {
				return err
			}

			var domains []struct {
				Domain   string `json:"domain"`
				Services map[string]struct {
					Name        string `json:"name"`
					Description string `json:"description"`
					Fields      map[string]struct {
						Required    bool        `json:"required"`
						Description string      `json:"description"`
						Example     interface{} `json:"example"`
					} `json:"fields"`
				} `json:"services"`
			}

			if err := json.Unmarshal(data, &domains); err != nil {
				return err
			}

			for _, d := range domains {
				if d.Domain == domain {
					if svc, ok := d.Services[service]; ok {
						if flags.asJSON {
							return printJSONFiltered(cmd.OutOrStdout(), svc, flags)
						}

						fmt.Fprintf(cmd.OutOrStdout(), "Service: %s.%s\n", domain, service)
						if svc.Name != "" {
							fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\n", svc.Name)
						}
						if svc.Description != "" {
							fmt.Fprintf(cmd.OutOrStdout(), "Description: %s\n", svc.Description)
						}
						fmt.Fprintln(cmd.OutOrStdout(), "\nExpected Payload Fields:")

						payload := make(map[string]interface{})
						for fieldName, field := range svc.Fields {
							reqStr := ""
							if field.Required {
								reqStr = " (REQUIRED)"
							}
							fmt.Fprintf(cmd.OutOrStdout(), "  - %s%s: %s\n", fieldName, reqStr, field.Description)
							if field.Example != nil {
								payload[fieldName] = field.Example
								fmt.Fprintf(cmd.OutOrStdout(), "      Example: %v\n", field.Example)
							} else {
								payload[fieldName] = "<value>"
							}
						}

						fmt.Fprintln(cmd.OutOrStdout(), "\nExample JSON Payload:")
						b, _ := json.MarshalIndent(payload, "", "  ")
						fmt.Fprintln(cmd.OutOrStdout(), string(b))
						return nil
					}
					return fmt.Errorf("service %s not found in domain %s", service, domain)
				}
			}

			return fmt.Errorf("domain %s not found", domain)
		},
	}
	return cmd
}
