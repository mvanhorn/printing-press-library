package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/mcp/cobratree"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func mktFlagFixture(t *testing.T) (int64, int64) {
	t.Helper()
	db := mktOpenFixtureStore(t)
	defer db.Close()
	var older, latest int64
	for i := 0; i < 2; i++ {
		latest = mktFxSnapshot(t, db.DB(), time.Date(2026, 11, 18+i, 0, 0, 0, 0, time.UTC), []string{"linen apron"}, 1, "complete", mktFxProg{})
		if i == 0 {
			older = latest
		}
		for j := 0; j < 12; j++ {
			mktFxResult(t, db.DB(), latest, mktFxListing{term: "linen apron", pos: j + 1, id: 1000000001 + int64(j), shop: 9000001, title: "linen apron", price: 2400, tags: []string{"linen apron"}})
		}
		mktFxShop(t, db.DB(), latest, 9000001, 10, 5, 1700000000)
	}
	return older, latest
}

func mktFlagRun(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	return out.String(), err
}

func mktFlagArgs(command string) []string {
	args := []string{"market", command}
	if command == "gaps" {
		args = append(args, "--rules-json", `{"targets":[{"name":"linen apron","include":["linen apron"]}]}`)
	}
	return args
}

func TestMarketPlainPopulated(t *testing.T) {
	mktTestEnv(t)
	mktFlagFixture(t)
	for _, command := range []string{"tags", "trend", "gaps", "shops", "price", "demand"} {
		t.Run(command, func(t *testing.T) {
			out, err := mktFlagRun(t, append(mktFlagArgs(command), "--plain")...)
			t.Logf("plain: err=%v stdout=%s", err, out)
			if command == "tags" || command == "trend" {
				if ExitCode(err) != 2 || !strings.Contains(fmt.Sprint(err), "nested; use --json") || strings.Contains(out, "(no rows)") {
					t.Fatalf("err=%v stdout=%s", err, out)
				}
			} else if err != nil || strings.Contains(out, "(no rows)") || strings.TrimSpace(out) == "" {
				t.Fatalf("err=%v stdout=%s", err, out)
			}
			if command != "tags" && command != "trend" {
				_, rows, ok := strings.Cut(out, "\n")
				want := "linen apron"
				if command == "shops" {
					want = "9000001"
				}
				if command == "price" {
					want = "\t24\t24\t24\t12\t"
				}
				if !ok || !strings.Contains(rows, want) {
					t.Fatalf("missing data after header: %s", out)
				}
			}
			out, err = mktFlagRun(t, append(mktFlagArgs(command), "--json")...)
			if err != nil {
				t.Fatal(err)
			}
			m := mktDecode(t, out)
			if m["results"] == nil || ((command == "tags" || command == "trend") && !strings.Contains(out, "linen apron")) {
				t.Fatalf("unpopulated JSON: %s", out)
			}
		})
	}
}

func TestMarketSnapshotFlag(t *testing.T) {
	for _, command := range []string{"demand", "tags", "shops", "price", "gaps"} {
		for _, state := range []string{"populated", "empty", "partial"} {
			t.Run(command+"_"+state, func(t *testing.T) {
				mktTestEnv(t)
				var older, latest int64
				if state == "populated" {
					older, latest = mktFlagFixture(t)
				}
				if state == "partial" {
					db := mktOpenFixtureStore(t)
					mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "partial", mktFxProg{})
					db.Close()
				}
				for _, value := range []string{"99", "0", "-5", "", "older", "dry_run_0"} {
					t.Run("value_"+strings.ReplaceAll(value, "-", "neg"), func(t *testing.T) {
						if value == "older" && state != "populated" {
							t.Skip("requires two complete snapshots")
						}
						args := mktFlagArgs(command)
						if value != "" {
							selected := value
							if value == "older" {
								selected = fmt.Sprint(older)
							}
							if value == "dry_run_0" {
								selected = "0"
								args = append(args, "--dry-run")
							}
							args = append(args, "--snapshot", selected)
						}
						out, err := mktFlagRun(t, append(args, "--json")...)
						wantCode, wantText := 0, ""
						switch value {
						case "99":
							wantCode, wantText = 3, "snapshot 99 not found"
						case "0", "-5", "dry_run_0":
							wantCode, wantText = 2, "--snapshot must be a positive snapshot ID (omit it for the latest complete snapshot)"
						}
						code := 0
						if err != nil {
							code = ExitCode(err)
						}
						if code != wantCode || (wantText != "" && !strings.Contains(fmt.Sprint(err), wantText)) {
							t.Fatalf("snapshot=%q exit=%d err=%v out=%s", value, code, err, out)
						}
						if value == "older" {
							if got := mktDecode(t, out)["snapshot_id"]; got != float64(older) || got == float64(latest) {
								t.Fatalf("explicit older snapshot not selected: %s", out)
							}
						}
						if value == "" {
							m := mktDecode(t, out)
							if state == "populated" && m["snapshot_id"] != float64(latest) {
								t.Fatalf("latest not selected: %s", out)
							}
							if state == "empty" && !strings.Contains(out, "no market snapshots yet") {
								t.Fatalf("missing empty note: %s", out)
							}
							if state == "partial" && !strings.Contains(out, "no complete market snapshot yet") {
								t.Fatalf("missing partial note: %s", out)
							}
						}
					})
				}
			})
		}
	}
}

// Exercise the same companion-CLI shell-out seam as the MCP mirror tests.
func TestMarketSnapshotMissingMCP(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "etsy-pp-cli")
	build := exec.Command("go", "build", "-o", bin, "./cmd/etsy-pp-cli")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	mktTestEnv(t)
	mktFlagFixture(t)
	s := server.NewMCPServer("etsy", "test")
	cobratree.RegisterAll(s, RootCmd(), func() (string, error) { return bin, nil })
	t.Run("trend_schema", func(t *testing.T) {
		tool := s.ListTools()["market_trend"]
		if tool == nil {
			t.Fatal("missing trend tool")
		}
		schema, err := json.Marshal(tool.Tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var decoded struct {
			Properties map[string]struct{ Description, Type string }
		}
		if err := json.Unmarshal(schema, &decoded); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"from", "to"} {
			property := decoded.Properties[name]
			if property.Type != "number" && property.Type != "integer" {
				t.Fatalf("%s must be numeric: %s", name, schema)
			}
			description := property.Description
			if strings.Contains(description, "default") || !strings.Contains(description, "omit both for the latest two") {
				t.Fatalf("%s schema: %s", name, schema)
			}
		}
	})
	for _, command := range []string{"demand", "tags", "shops", "price", "gaps"} {
		t.Run(command, func(t *testing.T) {
			tool := s.ListTools()["market_"+command]
			if tool == nil {
				t.Fatal("missing mirror tool")
			}
			schema, err := json.Marshal(tool.Tool.InputSchema)
			if err != nil {
				t.Fatal(err)
			}
			var decoded struct {
				Properties map[string]struct{ Description, Type string }
			}
			if err := json.Unmarshal(schema, &decoded); err != nil {
				t.Fatal(err)
			}
			property := decoded.Properties["snapshot"]
			if property.Type != "number" && property.Type != "integer" {
				t.Fatalf("snapshot must be numeric: %s", schema)
			}
			description := property.Description
			if strings.Contains(description, "default") || !strings.Contains(description, "omit for the latest") {
				t.Fatalf("snapshot schema: %s", schema)
			}
			for _, value := range []float64{99, 0} {
				t.Run(fmt.Sprintf("value_%.0f", value), func(t *testing.T) {
					args := map[string]any{"snapshot": value}
					if command == "gaps" {
						args["rules-json"] = `{"targets":[{"name":"linen apron","include":["linen apron"]}]}`
					}
					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer cancel()
					result, err := tool.Handler(ctx, mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: args}})
					if err != nil {
						t.Fatal(err)
					}
					if !result.IsError {
						t.Fatalf("missing snapshot did not reach MCP: %+v", result)
					}
					var text strings.Builder
					for _, content := range result.Content {
						if c, ok := content.(mcplib.TextContent); ok {
							text.WriteString(c.Text)
						}
					}
					want := "snapshot 99 not found"
					if value == 0 {
						want = "--snapshot must be a positive snapshot ID"
					}
					if !strings.Contains(text.String(), want) {
						t.Fatalf("wrong MCP error: %s", text.String())
					}
				})
			}
		})
	}
}

// Validation must precede dry-run and any store access.
func TestMarketTrendSnapshotFlags(t *testing.T) {
	for _, populated := range []bool{false, true} {
		t.Run(fmt.Sprintf("store_%t", populated), func(t *testing.T) {
			mktTestEnv(t)
			if populated {
				mktFlagFixture(t)
			}
			for _, tc := range []struct {
				name    string
				args    []string
				code    int
				message string
			}{
				{"missing", []string{"--from", "99", "--to", "98"}, 3, "snapshot 99 or 98 not found"},
				{"zero", []string{"--from", "0", "--to", "0"}, 2, "--from/--to must be positive snapshot IDs"},
				{"negative", []string{"--from", "-1", "--to", "3"}, 2, "--from/--to must be positive snapshot IDs"},
				{"from_only", []string{"--from", "5"}, 2, "--from and --to must be given together"},
				{"to_only", []string{"--to", "5"}, 2, "--from and --to must be given together"},
				{"dry_zero", []string{"--dry-run", "--from", "0", "--to", "0"}, 2, "--from/--to must be positive snapshot IDs"},
				{"omitted", nil, 0, ""},
			} {
				t.Run(tc.name, func(t *testing.T) {
					out, err := mktFlagRun(t, append([]string{"market", "trend", "--json"}, tc.args...)...)
					code := 0
					if err != nil {
						code = ExitCode(err)
					}
					if code != tc.code || !strings.Contains(fmt.Sprint(err), tc.message) {
						t.Fatalf("exit=%d err=%v out=%s", code, err, out)
					}
					if tc.name == "omitted" {
						if !populated && !strings.Contains(out, "no market snapshots yet") {
							t.Fatalf("missing note: %s", out)
						}
						if populated && mktDecode(t, out)["from_snapshot"] == nil {
							t.Fatalf("missing comparison: %s", out)
						}
					}
				})
			}
		})
	}
}

func TestMarketSnapshotFlagHelp(t *testing.T) {
	mktTestEnv(t)
	for _, name := range []string{"demand", "tags", "shops", "price", "gaps", "trend"} {
		t.Run(name, func(t *testing.T) {
			out, err := mktFlagRun(t, "market", name, "--help")
			if err != nil || !strings.Contains(out, "Positive snapshot ID;") {
				t.Fatalf("err=%v help=%s", err, out)
			}
			flags := []string{"snapshot"}
			if name == "trend" {
				flags = []string{"from", "to"}
			}
			for _, flag := range flags {
				found := false
				for _, line := range strings.Split(out, "\n") {
					if strings.HasPrefix(strings.TrimSpace(line), "--"+flag+" ") {
						found = true
						if strings.Contains(line, "(default") {
							t.Fatalf("unexpected default: %s", line)
						}
					}
				}
				if !found {
					t.Fatalf("missing --%s help line: %s", flag, out)
				}
			}
		})
	}
}
