// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/japan47go/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/japan47go/internal/service"
	"github.com/spf13/cobra"
	"path/filepath"
	"strings"
	"time"
)

func serviceCachePath() (string, error) {
	dir, e := cliutil.CacheDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(dir, service.CacheFilename), nil
}
func serviceError(cmd *cobra.Command, f *rootFlags, e error) error {
	var typed *cliError
	if !errors.As(e, &typed) {
		var rate *cliutil.RateLimitError
		var h *service.HTTPError
		if errors.As(e, &rate) {
			e = rateLimitErr(e)
		} else if errors.As(e, &h) && h.Status == 404 {
			e = notFoundErr(e)
		} else {
			e = apiErr(e)
		}
	}
	writeAPIErrorEnvelope(cmd.OutOrStdout(), f, e, ExitCode(e))
	return e
}
func serviceOutput(cmd *cobra.Command, f *rootFlags, v any) error {
	keep := []string{"service", "candidates", "comparisons", "coverage", "query", "constraints", "fetch_failures", "source_boundary", "note", "capacity", "scanned_observations", "returned"}
	if f.selectFields == "" {
		return printJSONFilteredKeep(cmd.OutOrStdout(), v, f, keep...)
	}
	raw, e := json.Marshal(v)
	if e != nil {
		return e
	}
	var envelope map[string]json.RawMessage
	if e = json.Unmarshal(raw, &envelope); e != nil {
		return e
	}
	project := func(row json.RawMessage, key string) (json.RawMessage, error) {
		fields := []string{}
		for _, part := range strings.Split(f.selectFields, ",") {
			part = strings.TrimSpace(part)
			part = strings.TrimPrefix(part, key+".")
			part = strings.TrimPrefix(part, "service.")
			fields = append(fields, part)
		}
		selection := strings.Join(fields, ",")
		if key == "comparisons" {
			var comparison map[string]json.RawMessage
			if e := json.Unmarshal(row, &comparison); e != nil {
				return nil, e
			}
			value, e := filterFieldsChecked(comparison["service"], selection)
			if e != nil {
				return nil, e
			}
			comparison["service"] = value
			return json.Marshal(comparison)
		}
		return filterFieldsChecked(row, selection)
	}
	for _, key := range []string{"service", "candidates", "comparisons"} {
		value, ok := envelope[key]
		if !ok {
			continue
		}
		var rows []json.RawMessage
		if len(value) > 0 && value[0] == '[' {
			if e = json.Unmarshal(value, &rows); e != nil {
				return e
			}
			for i, row := range rows {
				rows[i], e = project(row, key)
				if e != nil {
					return serviceError(cmd, f, e)
				}
			}
			envelope[key], e = json.Marshal(rows)
		} else {
			envelope[key], e = project(value, key)
		}
		if e != nil {
			return serviceError(cmd, f, e)
		}
	}
	copyFlags := *f
	copyFlags.selectFields = ""
	copyFlags.compact = false
	return printJSONFilteredKeep(cmd.OutOrStdout(), envelope, &copyFlags, keep...)
}
func serviceGet(ctx context.Context, c *service.Client, id string, f *rootFlags, refresh bool) (service.Service, error) {
	path, e := serviceCachePath()
	if e != nil {
		return service.Service{}, e
	}
	local := func() (service.Service, error) {
		xs, _, e := service.Cached(ctx, path, id, 50)
		if e != nil {
			return service.Service{}, e
		}
		for _, s := range xs {
			if s.ID == id {
				s.Stale = f.maxAge > 0 && time.Duration(s.CacheAgeSeconds)*time.Second > f.maxAge
				return s, nil
			}
		}
		return service.Service{}, notFoundErr(fmt.Errorf("service %s is absent from local observations; run services inspect %s --data-source live", id, id))
	}
	if f.dataSource == "local" {
		if f.noCache || refresh {
			return service.Service{}, usageErr(errors.New("--data-source local conflicts with --no-cache or --refresh"))
		}
		return local()
	}
	s, e := c.Get(ctx, id)
	if e != nil {
		if f.dataSource == "auto" && !f.noCache && !refresh && isNetworkError(e) {
			v, cacheErr := local()
			if cacheErr == nil {
				v.Transport = "local_fallback"
				v.SourceFailure = strPointer(e.Error())
				return v, nil
			}
			return service.Service{}, fmt.Errorf("source failed: %v; local fallback failed: %v", e, cacheErr)
		}
		return service.Service{}, e
	}
	if !f.noCache {
		if e = service.Save(ctx, path, s); e != nil {
			s.CacheWarning = strPointer(e.Error())
		}
	}
	return s, nil
}
func strPointer(s string) *string { return &s }
func serviceWarn(cmd *cobra.Command, s service.Service) {
	if s.SourceFailure != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s uses saved observation from %s after source failure: %s\n", s.ID, s.ObservedAt, *s.SourceFailure)
	}
	if s.CacheWarning != nil {
		if s.Transport == "live" {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s source read succeeded but local save failed: %s\n", s.ID, *s.CacheWarning)
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s saved observation: %s\n", s.ID, *s.CacheWarning)
		}
	}
}
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, c := range root.Commands() {
			if c.Name() == "api" {
				root.RemoveCommand(c)
			}
		}
	})
}
