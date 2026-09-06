// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
	"github.com/spf13/cobra"
)

func novelLocalMeta(db *store.Store) map[string]any {
	meta := map[string]any{"source": "local", "synced": false, "last_synced_at": nil}
	if db == nil {
		return meta
	}
	state, err := readSyncHintState(db, "availability")
	if err == nil && state.hasState {
		meta["synced"] = true
		meta["last_synced_at"] = state.lastSynced.UTC().Format(time.RFC3339)
	}
	return meta
}

func printNovelJSON(w io.Writer, value any, flags *rootFlags, db *store.Store) error {
	if flags != nil && (flags.csv || flags.plain) {
		v := reflect.ValueOf(value)
		if v.IsValid() && v.Kind() == reflect.Slice && v.Len() == 0 {
			elem := v.Type().Elem()
			for elem.Kind() == reflect.Pointer {
				elem = elem.Elem()
			}
			if elem.Kind() == reflect.Struct {
				keys := make([]string, 0, elem.NumField())
				for i := 0; i < elem.NumField(); i++ {
					name := strings.Split(elem.Field(i).Tag.Get("json"), ",")[0]
					if name != "" && name != "-" {
						keys = append(keys, name)
					}
				}
				sort.Strings(keys)
				sep := ","
				if flags.plain {
					sep = "\t"
				}
				_, err := io.WriteString(w, strings.Join(keys, sep)+"\n")
				return err
			}
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(w, raw, flags, novelLocalMeta(db))
}

func novelUsageError(cmd *cobra.Command, flags *rootFlags, err error) error {
	if flags != nil && flags.asJSON {
		raw, marshalErr := json.Marshal(map[string]any{
			"error": err.Error(),
			"usage": cmd.CommandPath() + " --help",
		})
		if marshalErr != nil {
			return marshalErr
		}
		if printErr := printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, novelLocalMeta(nil)); printErr != nil {
			return printErr
		}
	}
	return usageErr(err)
}
