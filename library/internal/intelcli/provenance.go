package intelcli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type DataProvenance struct {
	SchemaVersion         string            `json:"schema_version,omitempty"`
	DateRange             DateRange         `json:"date_range"`
	SourceCommandVersions map[string]string `json:"source_command_versions,omitempty"`
	InputHashes           map[string]string `json:"input_hashes,omitempty"`
}

type DateRange struct {
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
}

func HashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("sha256:%x", sum)
}

func HashJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "sha256:unavailable"
	}
	return HashBytes(b)
}

func MergeStringMaps(base map[string]string, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range base {
		if strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}
	for k, v := range extra {
		if strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}
	return out
}

func DefaultDateRange(start, end string) (string, string) {
	if end == "" {
		end = time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	}
	if start == "" {
		t, err := time.Parse("2006-01-02", end)
		if err == nil {
			start = t.AddDate(0, 0, -7).Format("2006-01-02")
		} else {
			start = end
		}
	}
	return start, end
}

func ChildCLIVersion(binary string) string {
	for _, args := range [][]string{{"--version"}, {"version"}} {
		cmd := exec.Command(binary, args...)
		b, err := cmd.Output()
		if err == nil {
			if version := strings.TrimSpace(string(b)); version != "" {
				lines := strings.Split(version, "\n")
				return strings.TrimSpace(lines[0])
			}
		}
	}
	return "unknown"
}
