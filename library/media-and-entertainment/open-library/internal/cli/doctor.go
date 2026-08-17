// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Report Open Library request posture and command readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newOpenLibraryClient(flags.timeout)
			result := DoctorResult{
				Source:       "open-library-pp-cli",
				Auth:         "none",
				BaseURL:      client.baseURL,
				UserAgentSet: strings.TrimSpace(env(userAgentEnv)) != "",
				ContactSet:   strings.TrimSpace(env(contactEmailEnv)) != "",
				Identified:   client.identified,
				GoVersion:    runtime.Version(),
				OSArch:       runtime.GOOS + "/" + runtime.GOARCH,
				ReadyCommands: []string{
					"book",
					"isbn",
					"author",
					"work",
					"editions",
					"subjects",
					"sources",
				},
				Caveats: sourceCaveats(client.identified),
			}
			return printResult(cmd, flags, result)
		},
	}
}
