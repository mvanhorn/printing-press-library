package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/myq/internal/client"
	"github.com/spf13/cobra"
)

var version = "1.0.0"

const pollInterval = 5 * time.Second

type rootFlags struct {
	username     string
	password     string
	debug        bool
	timeout      time.Duration
	waitFor      time.Duration
	clientSecret string
}

func Execute() error {
	return newRootCmd(&rootFlags{}).Execute()
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	return 1
}

func newRootCmd(flags *rootFlags) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "myq-pp-cli",
		Short:        "Control a myQ Smart Garage Opener from the terminal",
		Long:         "Control a myQ Smart Garage Opener from the terminal using the legacy myQ login flow and device APIs.",
		Version:      version,
		SilenceUsage: true,
	}
	rootCmd.SetVersionTemplate("myq-pp-cli {{ .Version }}\n")

	rootCmd.PersistentFlags().StringVarP(&flags.username, "username", "u", "", "myQ account email")
	rootCmd.PersistentFlags().StringVarP(&flags.password, "password", "p", "", "myQ account password")
	rootCmd.PersistentFlags().BoolVar(&flags.debug, "debug", false, "Print HTTP requests and responses to stderr")
	rootCmd.PersistentFlags().DurationVar(&flags.timeout, "timeout", 20*time.Second, "Per-request timeout")
	rootCmd.PersistentFlags().DurationVar(&flags.waitFor, "wait-timeout", 60*time.Second, "How long open/close waits for the door state to change")

	rootCmd.AddCommand(newDevicesCmd(flags))
	rootCmd.AddCommand(newStateCmd(flags))
	rootCmd.AddCommand(newOpenCmd(flags))
	rootCmd.AddCommand(newCloseCmd(flags))
	return rootCmd
}

func newDevicesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "devices",
		Short:       "List myQ devices",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(flags)
			if err != nil {
				return err
			}
			devices, err := c.Devices(cmd.Context())
			if err != nil {
				return err
			}
			if len(devices) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No devices found.")
				return nil
			}
			for _, d := range devices {
				fmt.Fprintf(cmd.OutOrStdout(), "Device %s\n", d.SerialNumber)
				fmt.Fprintf(cmd.OutOrStdout(), "  Account: %s\n", d.Account.Name)
				fmt.Fprintf(cmd.OutOrStdout(), "  Name: %s\n", d.Name)
				if d.Type != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  Type: %s\n", d.Type)
				}
				if d.DoorState != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  Door State: %s\n", d.DoorState)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
}

func newStateCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "state <serial-number>",
		Short:       "Fetch the current door state for a device",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(flags)
			if err != nil {
				return err
			}
			state, err := c.DeviceState(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if state == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Device %s has no door state\n", args[0])
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Device %s is %s\n", args[0], state)
			return nil
		},
	}
}

func newOpenCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "open <serial-number>",
		Short: "Open a garage door and wait for it to change state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return openOrClose(cmd, flags, args[0], client.ActionOpen)
		},
	}
}

func newCloseCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "close <serial-number>",
		Short: "Close a garage door and wait for it to change state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return openOrClose(cmd, flags, args[0], client.ActionClose)
		},
	}
}

func openOrClose(cmd *cobra.Command, flags *rootFlags, serialNumber string, action string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), flags.waitFor)
	defer cancel()

	c, err := newClient(flags)
	if err != nil {
		return err
	}
	if err := c.SetDoorState(ctx, serialNumber, action); err != nil {
		return err
	}

	desiredState := client.StateOpen
	if action == client.ActionClose {
		desiredState = client.StateClosed
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Waiting for door to be %s...\n", desiredState)

	lastState := ""
	for {
		state, err := c.DeviceState(ctx, serialNumber)
		if err != nil {
			return err
		}
		if state != lastState {
			if lastState != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Door state changed to %s\n", state)
			}
			lastState = state
		}
		if lastState == desiredState {
			fmt.Fprintf(cmd.OutOrStdout(), "Device %s is %s\n", serialNumber, desiredState)
			return nil
		}

		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("timed out waiting for door to be %s", desiredState)
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func newClient(flags *rootFlags) (*client.Client, error) {
	username := strings.TrimSpace(flags.username)
	password := strings.TrimSpace(flags.password)
	if username == "" {
		username = strings.TrimSpace(os.Getenv("MYQ_USERNAME"))
	}
	if password == "" {
		password = strings.TrimSpace(os.Getenv("MYQ_PASSWORD"))
	}
	clientSecret := strings.TrimSpace(flags.clientSecret)
	if clientSecret == "" {
		clientSecret = strings.TrimSpace(os.Getenv("MYQ_CLIENT_SECRET"))
	}
	if username == "" {
		return nil, errors.New("set -username or MYQ_USERNAME")
	}
	if password == "" {
		return nil, errors.New("set -password or MYQ_PASSWORD")
	}

	c := client.New(username, password, flags.debug, flags.timeout, clientSecret)
	return c, nil
}
