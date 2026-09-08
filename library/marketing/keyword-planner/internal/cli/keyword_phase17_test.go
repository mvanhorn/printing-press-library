// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/csv"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadPlannerLinesRejectsOversizedInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seeds.txt")
	data := bytes.Repeat([]byte{'\n'}, maxPlannerFileBytes+1)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPlannerLines(nil, path, "seed", maxPlannerSeeds); err == nil || !strings.Contains(err.Error(), "exceeds the maximum size") {
		t.Fatalf("oversized seed file error = %v; want explicit size error", err)
	}
}

func TestPlannerRateLimitSettingsEnforcePlannerPolicy(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		wantRate float64
		wantWait time.Duration
		wantErr  bool
	}{
		{name: "auto", value: -1, wantRate: 1, wantWait: time.Second},
		{name: "one qps", value: 1, wantRate: 1, wantWait: time.Second},
		{name: "half qps", value: 0.5, wantRate: 0.5, wantWait: 2 * time.Second},
		{name: "disabled", value: 0, wantErr: true},
		{name: "above planner ceiling", value: 1.0001, wantErr: true},
		{name: "other negative", value: -2, wantErr: true},
		{name: "nan", value: math.NaN(), wantErr: true},
		{name: "positive infinity", value: math.Inf(1), wantErr: true},
		{name: "duration overflow", value: 1e-300, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rate, wait, err := plannerRateLimitSettings(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("plannerRateLimitSettings(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if rate != tt.wantRate || wait != tt.wantWait {
				t.Fatalf("plannerRateLimitSettings(%v) = rate %v/wait %s, want %v/%s", tt.value, rate, wait, tt.wantRate, tt.wantWait)
			}
		})
	}
}

func TestPlannerLiveConfigNormalizesEnvironmentIDsAndSharesRuntimeSettings(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), "ads.env")
	if err := os.WriteFile(envPath, []byte("GOOGLE_ADS_CUSTOMER_ID=123-456-7890\nGOOGLE_ADS_LOGIN_CUSTOMER_ID=098-765-4321\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Process bindings intentionally contain only fixtures. LoadConfig gives
	// process bindings precedence, so this also proves no ambient credential
	// value is consulted by the test.
	for _, key := range []string{
		"GOOGLE_ADS_CLIENT_ID", "GOOGLE_ADS_CLIENT_SECRET", "GOOGLE_ADS_REFRESH_TOKEN",
		"GOOGLE_ADS_DEVELOPER_TOKEN", "GOOGLE_ADS_CUSTOMER_ID", "GOOGLE_ADS_LOGIN_CUSTOMER_ID",
	} {
		t.Setenv(key, "fixture-value")
	}
	t.Setenv("GOOGLE_ADS_CUSTOMER_ID", "123-456-7890")
	t.Setenv("GOOGLE_ADS_LOGIN_CUSTOMER_ID", "098-765-4321")

	flags := &rootFlags{rateLimit: 0.5, timeout: 45 * time.Second}
	config, err := plannerLiveConfig(envPath, "", flags)
	if err != nil {
		t.Fatal(err)
	}
	if config.CustomerID != "1234567890" || config.LoginCustomerID != "0987654321" {
		t.Fatalf("normalized IDs = customer %q/login %q", config.CustomerID, config.LoginCustomerID)
	}
	if config.RatePerSecond != 0.5 || config.MinRequestInterval != 2*time.Second || config.Timeout != 45*time.Second {
		t.Fatalf("runtime settings = rate %v/interval %s/timeout %s", config.RatePerSecond, config.MinRequestInterval, config.Timeout)
	}
}

func TestPlannerDefaultPathsHonorExplicitEnvironmentAndHomePrecedence(t *testing.T) {
	home := t.TempDir()
	envDB := filepath.Join(t.TempDir(), "env.db")
	envFile := filepath.Join(t.TempDir(), "env-file")
	explicitDB := filepath.Join(t.TempDir(), "explicit.db")
	explicitEnv := filepath.Join(t.TempDir(), "explicit.env")
	t.Setenv(plannerDBEnv, envDB)
	t.Setenv(plannerEnvFileEnv, envFile)
	if got := plannerPortfolioDBPath(explicitDB, home); got != explicitDB {
		t.Fatalf("explicit DB path = %q, want %q", got, explicitDB)
	}
	if got := plannerPortfolioDBPath("", home); got != envDB {
		t.Fatalf("environment DB path = %q, want %q", got, envDB)
	}
	if got := plannerEnvFilePath(explicitEnv, home); got != explicitEnv {
		t.Fatalf("explicit env path = %q, want %q", got, explicitEnv)
	}
	if got := plannerEnvFilePath("", home); got != envFile {
		t.Fatalf("environment env path = %q, want %q", got, envFile)
	}

	t.Setenv(plannerDBEnv, "")
	t.Setenv(plannerEnvFileEnv, "")
	wantDB := filepath.Join(home, plannerDBDefaultDir, plannerDBFilename)
	if got := plannerPortfolioDBPath("", home); got != wantDB {
		t.Fatalf("--home DB path = %q, want %q", got, wantDB)
	}
	wantEnv := filepath.Join(home, ".env")
	if got := plannerEnvFilePath("", home); got != wantEnv {
		t.Fatalf("--home env path = %q, want %q", got, wantEnv)
	}
}

func TestPlannerDoctorDryRunUsesRootHomeForCuratedDefaults(t *testing.T) {
	t.Setenv(plannerDBEnv, "")
	t.Setenv(plannerEnvFileEnv, "")
	home := t.TempDir()
	root := RootCmd()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"doctor", "--dry-run", "--live", "--home", home, "--agent", "--no-learn"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), filepath.Join(home, plannerDBDefaultDir, plannerDBFilename)) || !strings.Contains(output.String(), filepath.Join(home, ".env")) {
		t.Fatalf("doctor dry-run paths = %s", output.String())
	}
}

func TestProjectPlannerPortfolioCSVPreservesSelectionNullsAndInt64Text(t *testing.T) {
	index := func(name string) int {
		for i, field := range plannerPortfolioCSVFields {
			if field == name {
				return i
			}
		}
		t.Fatalf("missing fixture field %q", name)
		return -1
	}
	row := make([]string, len(plannerPortfolioCSVFields))
	row[index("keyword")] = "precision keyword"
	row[index("monthly_searches")] = "9223372036854775807"
	row[index("low_bid_micros")] = ""
	var source bytes.Buffer
	writer := csv.NewWriter(&source)
	if err := writer.Write(plannerPortfolioCSVFields); err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(row); err != nil {
		t.Fatal(err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatal(err)
	}

	projected, err := projectPlannerPortfolioCSV(source.Bytes(), "monthly_searches,keyword,low_bid_micros")
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(bytes.NewReader(projected))
	header, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"monthly_searches", "keyword", "low_bid_micros"}; !reflect.DeepEqual(header, want) {
		t.Fatalf("projected header = %#v, want %#v", header, want)
	}
	values, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"9223372036854775807", "precision keyword", ""}; !reflect.DeepEqual(values, want) {
		t.Fatalf("projected values = %#v, want %#v", values, want)
	}
	if _, err := reader.Read(); err != io.EOF {
		t.Fatalf("projected trailing row error = %v, want EOF", err)
	}
}

func TestPlannerPortfolioExportCSVHonorsSelectForExistingAndMissingDB(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fixture := newPlannerViewFixture(t)

	run := func(t *testing.T, dbPath string) string {
		t.Helper()
		root := RootCmd()
		var output, errOutput bytes.Buffer
		root.SetOut(&output)
		root.SetErr(&errOutput)
		root.SetArgs([]string{"portfolio", "export", "--format", "csv", "--db", dbPath, "--snapshot", "latest", "--select", "monthly_searches,keyword", "--no-learn"})
		if err := root.Execute(); err != nil {
			t.Fatalf("portfolio export: %v; stderr=%s", err, errOutput.String())
		}
		return output.String()
	}

	existing := run(t, fixture.dbPath)
	reader := csv.NewReader(strings.NewReader(existing))
	header, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"monthly_searches", "keyword"}; !reflect.DeepEqual(header, want) {
		t.Fatalf("existing selected header = %#v, want %#v", header, want)
	}
	rows := make(map[string]string)
	for {
		row, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		if len(row) != 2 {
			t.Fatalf("existing selected row = %#v", row)
		}
		rows[row[1]] = row[0]
	}
	if rows["beef steak"] != "12" || rows["beef brisket"] != "20" {
		t.Fatalf("existing selected rows = %#v", rows)
	}

	missing := run(t, filepath.Join(t.TempDir(), "missing.db"))
	missingReader := csv.NewReader(strings.NewReader(missing))
	missingHeader, err := missingReader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"monthly_searches", "keyword"}; !reflect.DeepEqual(missingHeader, want) {
		t.Fatalf("missing selected header = %#v, want %#v", missingHeader, want)
	}
	if _, err := missingReader.Read(); err != io.EOF {
		t.Fatalf("missing selected rows error = %v, want EOF", err)
	}
}
