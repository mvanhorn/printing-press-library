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
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/michi"
)

const bulletinExportIndex = `<main><div class="noticesList"><a href="/notices/views/20001"><time datetime="2026-9-30">2026年9月30日</time><span>長野県</span><p>試験駅のお知らせ</p></a><a href="/notices/views/20002"><time datetime="2026-10-1">2026年10月1日</time><span>長野県</span><p>別の駅のお知らせ</p></a></div><!-- NOT-A-RECORD --><a href="/notices?page=1">2</a></main>`

const bulletinExportDetail = `<article class="noticesView__content"><h3>試験日程のお知らせ</h3><a href="/stations/views/10001">試験駅</a><p>2026年10月6日は休業予定。掲載日は2026年9月30日。</p><p class="createdDate">2026年9月30日</p></article>`

func bulletinExportSource() *michi.Source {
	return &michi.Source{BaseURL: michi.Origin, Fetch: func(ctx context.Context, path string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch path {
		case "/notices":
			return []byte(bulletinExportIndex), nil
		case "/notices?page=1":
			return []byte(`<main><div class="noticesList"></div></main>`), nil
		case "/notices/views/20001":
			return []byte(bulletinExportDetail), nil
		default:
			return []byte(`<html><title>not a notice list</title></html>`), nil
		}
	}}
}

func exportBulletins(t *testing.T, src *michi.Source, args []string, format string, limit int) (int, string) {
	t.Helper()
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	n, err := writeBulletinExportFromSource(context.Background(), nil, src, args, format, limit, writer)
	if flushErr := writer.Flush(); flushErr != nil {
		t.Fatal(flushErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	return n, buf.String()
}

func TestBulletinExportWritesJSONLRecords(t *testing.T) {
	n, out := exportBulletins(t, bulletinExportSource(), []string{"bulletins"}, "jsonl", 0)
	if n != 2 {
		t.Fatalf("count = %d, want 2", n)
	}
	if strings.Contains(out, "NOT-A-RECORD") || strings.Contains(out, "noticesList") || strings.Contains(out, "<") {
		t.Fatalf("export wrote HTML:\n%s", out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2: %q", len(lines), out)
	}
	for i, wantID := range []string{"20001", "20002"} {
		var rec michi.Notice
		if err := json.Unmarshal([]byte(lines[i]), &rec); err != nil {
			t.Fatalf("line %d is not a JSON record: %v\n%s", i, err, lines[i])
		}
		if rec.ID != wantID || rec.Title == "" || rec.PublishedDate == "unknown" {
			t.Fatalf("record = %+v", rec)
		}
	}
}

func TestBulletinExportJSONArrayAndLimit(t *testing.T) {
	n, out := exportBulletins(t, bulletinExportSource(), []string{"bulletins"}, "json", 1)
	if n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
	var recs []michi.Notice
	if err := json.Unmarshal([]byte(out), &recs); err != nil {
		t.Fatalf("json export did not decode: %v\n%s", err, out)
	}
	if len(recs) != 1 || recs[0].ID != "20001" {
		t.Fatalf("records = %+v", recs)
	}
}

func TestBulletinExportSingleNoticeRecord(t *testing.T) {
	n, out := exportBulletins(t, bulletinExportSource(), []string{"bulletins", "20001"}, "json", 0)
	if n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
	var rec michi.Notice
	if err := json.Unmarshal([]byte(out), &rec); err != nil {
		t.Fatalf("single export did not decode: %v\n%s", err, out)
	}
	if rec.ID != "20001" || rec.PublishedDate != "2026-09-30" || len(rec.StationIDs) != 1 || rec.StationIDs[0] != "10001" {
		t.Fatalf("record = %+v", rec)
	}
	if strings.Contains(out, "<") {
		t.Fatalf("export wrote HTML:\n%s", out)
	}
}

func bulletinIndexPage(start, n, next int) string {
	var b strings.Builder
	b.WriteString(`<main><div class="noticesList">`)
	for i := 0; i < n; i++ {
		id := start + i
		fmt.Fprintf(&b, `<a href="/notices/views/%d"><time datetime="2026-9-30">2026年9月30日</time><span>長野県</span><p>お知らせ%d</p></a>`, id, id)
	}
	b.WriteString(`</div>`)
	if next > 0 {
		fmt.Fprintf(&b, `<a href="/notices?page=%d">next</a>`, next)
	}
	b.WriteString(`</main>`)
	return b.String()
}

func TestBulletinExportLimitZeroFollowsIndexPastNoticesCap(t *testing.T) {
	var fetched []string
	src := &michi.Source{BaseURL: michi.Origin, Fetch: func(ctx context.Context, path string) ([]byte, error) {
		fetched = append(fetched, path)
		switch path {
		case "/notices":
			return []byte(bulletinIndexPage(40000, 40, 1)), nil
		case "/notices?page=1":
			return []byte(bulletinIndexPage(40040, 11, 0)), nil
		default:
			t.Fatalf("unexpected path %s", path)
			return nil, fmt.Errorf("unexpected path %s", path)
		}
	}}
	n, out := exportBulletins(t, src, []string{"bulletins"}, "jsonl", 0)
	if n != 51 {
		t.Fatalf("count = %d, want 51 (limit 0 must not clamp to 50)", n)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 51 {
		t.Fatalf("lines = %d, want 51", len(lines))
	}
	if strings.Join(fetched, ",") != "/notices,/notices?page=1" {
		t.Fatalf("fetched %v", fetched)
	}
}

func TestBulletinExportPositiveLimitStopsWithoutScanningFurtherPages(t *testing.T) {
	var fetched []string
	src := &michi.Source{BaseURL: michi.Origin, Fetch: func(ctx context.Context, path string) ([]byte, error) {
		fetched = append(fetched, path)
		if path != "/notices" {
			t.Fatalf("limit 7 fetched %s", path)
		}
		return []byte(bulletinIndexPage(41000, 40, 1)), nil
	}}
	n, out := exportBulletins(t, src, []string{"bulletins"}, "json", 7)
	if n != 7 {
		t.Fatalf("count = %d, want 7", n)
	}
	var recs []michi.Notice
	if err := json.Unmarshal([]byte(out), &recs); err != nil {
		t.Fatal(err)
	}
	if len(recs) != 7 || recs[0].ID != "41000" || recs[6].ID != "41006" {
		t.Fatalf("records = %+v", recs)
	}
	if len(fetched) != 1 {
		t.Fatalf("fetched %v", fetched)
	}
}

func TestBulletinExportRejectsUnknownFormatAndNegativeLimit(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	_, err := writeBulletinExportFromSource(context.Background(), nil, bulletinExportSource(), []string{"bulletins"}, "yaml", 0, writer)
	if err == nil {
		t.Fatal("yaml format accepted")
	}
	writer.Flush()
	if buf.Len() != 0 {
		t.Fatalf("invalid format wrote %q", buf.String())
	}
	buf.Reset()
	writer = bufio.NewWriter(&buf)
	_, err = writeBulletinExportFromSource(context.Background(), nil, bulletinExportSource(), []string{"bulletins"}, "jsonl", -3, writer)
	if err == nil || !strings.Contains(err.Error(), "--limit") {
		t.Fatalf("negative limit error = %v", err)
	}
	writer.Flush()
	if buf.Len() != 0 {
		t.Fatalf("negative limit wrote %q", buf.String())
	}
}

func TestBulletinExportFailureLeavesExistingFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "bulletins.jsonl")
	original := []byte("previous backup\n")
	if err := os.WriteFile(dest, original, 0o644); err != nil {
		t.Fatal(err)
	}
	src := &michi.Source{BaseURL: michi.Origin, Fetch: func(context.Context, string) ([]byte, error) {
		return nil, fmt.Errorf("index page failed")
	}}
	_, err := writeBulletinExportFileFromSource(context.Background(), nil, src, []string{"bulletins"}, "jsonl", 0, dest)
	if err == nil {
		t.Fatal("failed scan replaced the backup")
	}
	got, readErr := os.ReadFile(dest)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(original) {
		t.Fatalf("backup = %q", got)
	}
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 1 || entries[0].Name() != "bulletins.jsonl" {
		t.Fatalf("directory after failure: %v", entries)
	}
}

func TestBulletinExportReplacesFileOnlyAfterSuccess(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "nested", "bulletins.jsonl")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("previous backup\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := writeBulletinExportFileFromSource(context.Background(), nil, bulletinExportSource(), []string{"bulletins"}, "jsonl", 0, dest)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("count = %d, want 2", n)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "previous backup") || strings.Contains(string(got), "<") || !strings.Contains(string(got), "20001") {
		t.Fatalf("replaced body = %q", got)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("file mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestExportRejectsFormatAndLimitBeforeCreatingOutput(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		{"bulletins", "--format", "yaml", "--output", filepath.Join(dir, "yaml.json")},
		{"bulletins", "--limit", "-1", "--output", filepath.Join(dir, "neg.json")},
	} {
		out := args[len(args)-1]
		cmd := newExportCmd(&rootFlags{})
		cmd.SetArgs(args)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted %v", args)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatalf("output created for %v: %v", args, err)
		}
	}
}

func TestBulletinExportWritesNothingWhenIndexContinuesAndFails(t *testing.T) {
	src := &michi.Source{BaseURL: michi.Origin, Fetch: func(ctx context.Context, path string) ([]byte, error) {
		if path == "/notices" {
			return []byte(bulletinIndexPage(42000, 2, 1)), nil
		}
		return nil, fmt.Errorf("index page failed")
	}}
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	_, err := writeBulletinExportFromSource(context.Background(), nil, src, []string{"bulletins"}, "jsonl", 0, writer)
	if err == nil {
		t.Fatal("partial index exported")
	}
	writer.Flush()
	if buf.Len() != 0 {
		t.Fatalf("failed export wrote %q", buf.String())
	}
}

func TestBulletinExportRejectsUnparsedHTML(t *testing.T) {
	src := &michi.Source{BaseURL: michi.Origin, Fetch: func(context.Context, string) ([]byte, error) {
		return []byte(`<html><title>not a notice list</title></html>`), nil
	}}
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	if _, err := writeBulletinExportFromSource(context.Background(), nil, src, []string{"bulletins"}, "jsonl", 0, writer); err == nil {
		t.Fatal("unrecognized HTML exported")
	}
	writer.Flush()
	if buf.Len() != 0 {
		t.Fatalf("failed export wrote %q", buf.String())
	}
}
