// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/michi"
)

func validateExportFormat(format string) error {
	switch format {
	case "json", "jsonl":
		return nil
	default:
		return usageErr(fmt.Errorf("--format must be json or jsonl"))
	}
}

func writeParsedBulletinExportFile(parent context.Context, flags *rootFlags, args []string, format string, limit int, outputFile string) (int, error) {
	src, err := michiSource(flags)
	if err != nil {
		return 0, err
	}
	return writeBulletinExportFileFromSource(parent, flags, src, args, format, limit, outputFile)
}

func writeBulletinExportFileFromSource(parent context.Context, flags *rootFlags, src *michi.Source, args []string, format string, limit int, outputFile string) (int, error) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	count, err := writeBulletinExportFromSource(parent, flags, src, args, format, limit, writer)
	if err != nil {
		return 0, err
	}
	if err := writer.Flush(); err != nil {
		return 0, fmt.Errorf("flushing export: %w", err)
	}
	if err := replacePrivateFile(outputFile, buf.Bytes()); err != nil {
		return 0, err
	}
	return count, nil
}

// replacePrivateFile writes data to a temporary file in the destination
// directory and renames it onto path only after the write succeeds.
func replacePrivateFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	if dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
	}
	f, err := os.CreateTemp(dir, ".michi-export-*")
	if err != nil {
		return fmt.Errorf("creating export file: %w", err)
	}
	tmp := f.Name()
	committed := false
	defer func() {
		if !committed {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()
	if err := f.Chmod(0o600); err != nil {
		return fmt.Errorf("setting output file permissions: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("writing export: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing export file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replacing export file: %w", err)
	}
	committed = true
	return nil
}

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
	if err := validateExportFormat(format); err != nil {
		return 0, err
	}
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
	if limit < 0 {
		return 0, usageErr(fmt.Errorf("--limit must be 0 (all available notices) or a positive maximum"))
	}
	rows, err := src.ExportNotices(ctx, limit)
	if err != nil {
		return 0, classifyAPIErrorOnly(err)
	}
	if err := writeBulletinPayload(writer, format, rows, false); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func writeBulletinPayload(w *bufio.Writer, format string, rows []michi.Notice, single bool) error {
	if err := validateExportFormat(format); err != nil {
		return err
	}
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
