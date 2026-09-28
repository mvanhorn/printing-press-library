package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/ikyu"
	"github.com/spf13/cobra"
	"strings"
	"testing"
	"time"
)

type mockStayReader struct {
	calls    int
	search   ikyu.SearchRequest
	rooms    ikyu.RoomsRequest
	offer    ikyu.OfferRequest
	deadline time.Time
}

func (m *mockStayReader) Destinations(ctx context.Context, q string, l, o int) (ikyu.DestinationsResult, error) {
	m.calls++
	return ikyu.DestinationsResult{Data: []ikyu.Destination{{ID: "140000", Name: "東京", Path: "/tokyo/140000/"}}}, nil
}
func (m *mockStayReader) Search(ctx context.Context, r ikyu.SearchRequest) (ikyu.SearchResult, error) {
	m.calls++
	m.search = r
	m.deadline, _ = ctx.Deadline()
	return ikyu.SearchResult{Data: []ikyu.Property{{ID: "00000946", Name: "ザ・プリンス パークタワー東京"}}, Stay: r.Stay, Pagination: ikyu.Pagination{Total: 587, Returned: 1, HasNext: true}}, nil
}
func (m *mockStayReader) Property(ctx context.Context, id string) (ikyu.PropertyResult, error) {
	m.calls++
	return ikyu.PropertyResult{Data: ikyu.Property{ID: id, Name: "旅館"}}, nil
}
func (m *mockStayReader) Rooms(ctx context.Context, r ikyu.RoomsRequest) (ikyu.RoomsResult, error) {
	m.calls++
	m.rooms = r
	return ikyu.RoomsResult{Data: []ikyu.Room{{ID: "10193741", Name: "客室"}}, Stay: r.Stay}, nil
}
func (m *mockStayReader) Offer(ctx context.Context, r ikyu.OfferRequest) (ikyu.OfferResult, error) {
	m.calls++
	m.offer = r
	return ikyu.OfferResult{Data: ikyu.OfferData{Property: ikyu.Property{ID: r.PropertyID, Name: "旅館"}, Room: ikyu.Room{ID: r.RoomID, Name: "客室"}, Plan: ikyu.Plan{ID: r.PlanID, Name: "素泊まり"}, Offer: ikyu.DatedOffer{Stay: r.Stay}}}, nil
}
func (m *mockStayReader) Stats() ikyu.Stats { return ikyu.Stats{Requests: m.calls} }
func runStay(t *testing.T, m ikyu.Reader, args ...string) (map[string]any, string, error) {
	t.Helper()
	testenv.Isolate(t)
	oldReader, oldNow := stayReaderFactory, stayNow
	t.Cleanup(func() { stayReaderFactory = oldReader; stayNow = oldNow })
	stayNow = func() time.Time { return time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC) }
	stayReaderFactory = func(o ikyu.Options) (ikyu.Reader, error) { return m, nil }
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, diagnostic bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&diagnostic)
	err := cmd.Execute()
	result := map[string]any{}
	if out.Len() > 0 && strings.HasPrefix(strings.TrimSpace(out.String()), "{") {
		if e := json.Unmarshal(out.Bytes(), &result); e != nil {
			t.Fatalf("invalid JSON %q: %v", out.String(), e)
		}
	}
	return result, out.String(), err
}
func TestStayCoreCommandsAndPerRoomParty(t *testing.T) {
	party := []string{"--check-in", "2026-10-18", "--check-out", "2026-10-19", "--adults", "2", "--rooms", "2", "--children-e", "1"}
	for _, args := range [][]string{{"stay", "destinations", "tokyo"}, {"stay", "property", "00002889"}, append([]string{"stay", "search", "--destination", "tokyo"}, party...), append([]string{"stay", "rooms", "00002889", "--plan-limit", "2", "--plan-offset", "3"}, party...), append([]string{"stay", "offer", "00002889", "10193741", "11055986"}, party...)} {
		t.Run(strings.Join(args[:2], " "), func(t *testing.T) {
			m := &mockStayReader{}
			result, text, e := runStay(t, m, args...)
			if e != nil || result["data"] == nil || m.calls != 1 {
				t.Fatalf("core command err=%v data=%v calls=%d", e, result, m.calls)
			}
			if strings.Count(text, "\n") != 1 {
				t.Fatalf("JSON is not compact: %q", text)
			}
			if args[1] == "search" {
				if m.search.Destination != "tokyo" || m.search.Stay.Rooms != 2 || m.search.Stay.Adults != 2 || m.search.Stay.Children[4] != 1 {
					t.Fatalf("party changed %+v", m.search)
				}
				if m.deadline.IsZero() || time.Until(m.deadline) > 120*time.Second {
					t.Fatal("missing command timeout")
				}
			}
			if args[1] == "rooms" && m.rooms.PlanOffset != 3 {
				t.Fatal("plan offset omitted")
			}
		})
	}
}
func TestStayDryRunHasNoReaderOrCacheWrites(t *testing.T) {
	for _, leaf := range []string{"destinations", "property", "search", "rooms", "offer"} {
		m := &mockStayReader{}
		result, _, e := runStay(t, m, "stay", leaf, "--dry-run")
		if e != nil || m.calls != 0 || result["network_requests"] != float64(0) || result["cache_writes"] != float64(0) {
			t.Fatalf("dry run %s err=%v calls=%d result=%v", leaf, e, m.calls, result)
		}
	}
}
func TestStayFieldsAliasAndAgentProjection(t *testing.T) {
	for _, flag := range []string{"--select", "--fields"} {
		m := &mockStayReader{}
		result, _, e := runStay(t, m, "stay", "property", "00002889", flag, "data.id")
		if e != nil || len(result) != 1 {
			t.Fatalf("projection %v %v", result, e)
		}
		data := result["data"].(map[string]any)
		if len(data) != 1 || data["id"] != "00002889" {
			t.Fatalf("wrong projection %v", data)
		}
	}
	m := &mockStayReader{}
	result, _, e := runStay(t, m, "stay", "property", "00002889", "--agent", "--select", "data.id")
	if e != nil {
		t.Fatal(e)
	}
	meta := result["meta"].(map[string]any)
	if meta["source"] != "live" {
		t.Fatalf("provenance %v", meta)
	}
	data := result["results"].(map[string]any)["data"].(map[string]any)
	if data["id"] != "00002889" {
		t.Fatalf("agent projection %v", result)
	}
}
func TestStayInvalidDateAdultsAndDestinationConflict(t *testing.T) {
	for _, args := range [][]string{{"stay", "search", "tokyo", "--destination", "tokyo", "--check-in", "2026-10-18", "--check-out", "2026-10-19"}, {"stay", "search", "--destination", "tokyo", "--check-in", "2026-13-40", "--check-out", "2026-10-19"}, {"stay", "rooms", "00002889", "--check-in", "2026-10-18", "--check-out", "2026-10-19", "--adults", "0"}, {"stay", "property", "00002889", "--data-source", "local"}} {
		m := &mockStayReader{}
		_, _, e := runStay(t, m, args...)
		if e == nil || m.calls != 0 {
			t.Fatalf("validation %v err=%v calls=%d", args, e, m.calls)
		}
	}
}
func TestStayCacheFlagsArePassed(t *testing.T) {
	testenv.Isolate(t)
	old := stayReaderFactory
	t.Cleanup(func() { stayReaderFactory = old })
	var options ikyu.Options
	stayReaderFactory = func(o ikyu.Options) (ikyu.Reader, error) { options = o; return &mockStayReader{}, nil }
	cmd := RootCmd()
	cmd.SetArgs([]string{"stay", "property", "00002889", "--refresh", "--allow-stale", "--cache-dir", t.TempDir()})
	cmd.SetOut(&bytes.Buffer{})
	if e := cmd.Execute(); e != nil {
		t.Fatal(e)
	}
	if !options.Refresh || !options.AllowStale || options.CacheDir == "" {
		t.Fatalf("cache policy ignored %+v", options)
	}
}

func TestStayEmitPreservesLargeIntegerYenAndSingleLine(t *testing.T) {
	const exact int64 = 9007199254740993
	for _, selectFields := range []string{"", "data.source_amount"} {
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		f := &rootFlags{compact: true, selectFields: selectFields}
		if e := stayEmit(cmd, f, map[string]any{"data": map[string]any{"source_amount": exact}}, nil); e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(out.String(), "9007199254740993") || strings.Count(out.String(), "\n") != 1 {
			t.Fatalf("precision/format changed %q", out.String())
		}
	}
}

func TestStayDryRunWithLiveProjection(t *testing.T) {
	for _, leaf := range []string{"destinations", "search", "property", "rooms", "offer", "compare", "dates"} {
		for _, flag := range []string{"--select", "--fields"} {
			reader := &mockStayReader{}
			plan, _, err := runStay(t, reader, "stay", leaf, "--dry-run", "--agent", flag, "data")
			if err != nil || reader.calls != 0 {
				t.Fatalf("dry-run %s %s err=%v calls=%d", leaf, flag, err, reader.calls)
			}
			if plan["dry_run"] != true || !strings.HasSuffix(plan["action"].(string), "stay "+leaf) || plan["planned"] == nil || plan["network_requests"] != float64(0) || plan["cache_writes"] != float64(0) || plan["data"] != nil || plan["results"] != nil {
				t.Fatalf("dry-run control envelope lost/fabricated %v", plan)
			}
		}
	}
}
