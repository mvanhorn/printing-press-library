package intelcli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SnapshotFile struct {
	Path       string
	Name       string
	CapturedAt time.Time
}

func UniqueSnapshotPath(dir, date string) string {
	base := filepath.Join(dir, date+".json")
	if _, err := os.Stat(base); errors.Is(err, os.ErrNotExist) {
		return base
	}
	for i := 2; ; i++ {
		path := filepath.Join(dir, fmt.Sprintf("%s-%d.json", date, i))
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return path
		}
	}
}

func CompactSnapshotFiles(files []SnapshotFile, now time.Time) error {
	if len(files) <= 1 {
		return nil
	}
	sort.Slice(files, func(i, j int) bool { return files[i].CapturedAt.After(files[j].CapturedAt) })
	keep := map[string]bool{}
	daily := map[string]bool{}
	weekly := map[string]bool{}
	today := now.UTC().Format("2006-01-02")
	for _, ref := range files {
		day := ref.CapturedAt.UTC().Format("2006-01-02")
		age := now.UTC().Sub(ref.CapturedAt.UTC())
		switch {
		case day == today:
			keep[ref.Path] = true
		case age <= 30*24*time.Hour:
			if !daily[day] {
				keep[ref.Path] = true
				daily[day] = true
			}
		default:
			year, week := ref.CapturedAt.UTC().ISOWeek()
			key := fmt.Sprintf("%04d-W%02d", year, week)
			if !weekly[key] {
				keep[ref.Path] = true
				weekly[key] = true
			}
		}
	}
	for _, ref := range files {
		if !keep[ref.Path] {
			if err := os.Remove(ref.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

func SafeName(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return "default"
	}
	return strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-").Replace(v)
}

func CloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
