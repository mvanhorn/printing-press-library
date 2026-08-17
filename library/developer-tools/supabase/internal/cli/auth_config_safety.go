// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/client"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func rejectSensitiveAuthConfigFlags(cmd *cobra.Command) error {
	var blocked []string
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		wireName := strings.ReplaceAll(flag.Name, "-", "_")
		if client.IsSensitiveAuthConfigField(wireName) {
			blocked = append(blocked, "--"+flag.Name)
		}
	})
	if len(blocked) == 0 {
		return nil
	}
	sort.Strings(blocked)
	return fmt.Errorf("secret-valued auth-config flags are disabled (%s); pass the JSON request on stdin with --stdin", strings.Join(blocked, ", "))
}

func hideSensitiveAuthConfigFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		wireName := strings.ReplaceAll(flag.Name, "-", "_")
		if client.IsSensitiveAuthConfigField(wireName) {
			_ = cmd.Flags().MarkHidden(flag.Name)
		}
	})
}
