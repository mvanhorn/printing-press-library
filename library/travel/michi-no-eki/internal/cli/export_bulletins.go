// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/michi"
)

// noticesScanPages and noticesScanLimit are the notices command ceilings.
// Bulletin export reuses that domain path instead of treating /notices as JSON.
const (
	noticesScanPages = 5
	noticesScanLimit = 50
)

func writeParsedBulletinExport(parent context.Context, flags *rootFlags, args []string, format string, limit int, writer *bufio.Writer) (int, error) {
	src, err := michiSource(flags)
	if err != nil {
		return 0, err
	}
	return writeBulletinExportFromSource(parent, flags, src, args, format, limit, writer)
}

func writeBulletinExportFromSource(parent context.Context, flags *rootFlags, src *michi.Source, args []string, format string, limit int, writer *bufio.Writer) (int, error) {
	ctx, cancel := boundCtx(parent, flags)
	defer cancel()
	if len(args) > 1 {
		id := args[1]
		if err := michi.ValidateIDs(id); err != nil || strings.Contains(id, ",") {
			if err == nil {
				err = fmt.Errorf("one notice ID is required")
			}
			return 0, usageErr(err)
		}
		notice, err := src.Notice(ctx, id)
		if err != nil {
			return 0, classifyAPIErrorOnly(err)
		}
		if err := writeBulletinPayload(writer, format, []michi.Notice{notice}, true); err != nil {
			return 0, err
		}
		return 1, nil
	}
	want := limit
	if want <= 0 || want > noticesScanLimit {
		want = noticesScanLimit
	}
	page, err := src.Notices(ctx, noticesScanPages, want, "", "")
	if err != nil {
		return 0, classifyAPIErrorOnly(err)
	}
	rows := page.Notices
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
	}
	if err := writeBulletinPayload(writer, format, rows, false); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func writeBulletinPayload(w *bufio.Writer, format string, rows []michi.Notice, single bool) error {
	if format == "jsonl" {
		for _, row := range rows {
			raw, err := json.Marshal(row)
			if err != nil {
				return fmt.Errorf("encoding bulletin: %w", err)
			}
			if _, err := fmt.Fprintln(w, string(raw)); err != nil {
				return fmt.Errorf("writing export: %w", err)
			}
		}
		return nil
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	var output any
	if single {
		if len(rows) != 1 {
			return fmt.Errorf("encoding bulletin: expected one record")
		}
		output = rows[0]
	} else {
		if rows == nil {
			rows = []michi.Notice{}
		}
		output = rows
	}
	if err := enc.Encode(output); err != nil {
		return err
	}
	return nil
}
