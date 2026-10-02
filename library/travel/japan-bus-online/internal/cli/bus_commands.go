// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-bus-online/internal/jbo"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

func busEmit(cmd *cobra.Command, flags *rootFlags, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), b, flags, map[string]any{"source": "live", "provider": "Japan Bus Online"})
}

type busOptions struct {
	route, language, date string
	direction             int
}

func busFlags(cmd *cobra.Command, o *busOptions, date bool) {
	cmd.Flags().StringVar(&o.route, "route", "", "Provider course ID from routes list")
	cmd.Flags().StringVar(&o.language, "language", "en", "Source language: en (verified public surface)")
	cmd.Flags().IntVar(&o.direction, "direction", 0, "Direction identity from bus route: 0 or 1")
	if date {
		cmd.Flags().StringVar(&o.date, "date", time.Now().In(jbo.JST).AddDate(0, 0, 7).Format("2006-01-02"), "JST service day in YYYY-MM-DD format")
	}
}
func busClient(cmd *cobra.Command, o busOptions, flags *rootFlags) (*jbo.Client, error) {
	if e := jbo.ValidID(o.route); e != nil {
		return nil, usageErr(e)
	}
	if o.direction < 0 || o.direction > 1 {
		return nil, usageErr(fmt.Errorf("direction must be 0 or 1"))
	}
	if o.date != "" {
		if _, e := jbo.ParseDate(o.date); e != nil {
			return nil, usageErr(e)
		}
	}
	c, err := jbo.New(o.language, flags.rateLimit)
	if err != nil {
		return nil, usageErr(err)
	}
	return c, nil
}
func busAnnotations(args string) map[string]string {
	return map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": strings.ReplaceAll(args, " ", ";")}
}
