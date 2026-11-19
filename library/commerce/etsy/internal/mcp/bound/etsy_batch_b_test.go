package bound

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"runtime"
	"strings"
	"testing"
)

func TestBatchBMirrorReference(t *testing.T) {
	oldWriter := mirrorEventWriter
	mirrorEventWriter = io.Discard
	defer func() { mirrorEventWriter = oldWriter }()
	rng := rand.New(rand.NewSource(5418))
	for trial := 0; trial < 600; trial++ {
		n := rng.Intn(51)
		items := make([]any, n)
		for i := range items {
			size := rng.Intn(5001)
			if i == 0 {
				size = trial % 5001
			}
			items[i] = map[string]any{"listing_id": 1000000001 + i, "title": strings.Repeat("陶<&é", size/7), "nested": []any{nil, true, map[string]any{"v": size}}}
		}
		var value any = items
		switch trial % 3 {
		case 1:
			value = map[string]any{"items": items, "shop": "Juniper Ceramics", "count": n}
		case 2:
			value = map[string]any{"meta": map[string]any{"x": true}, "results": map[string]any{"items": items, "note": "linen apron"}}
		}
		data, _ := json.MarshalIndent(value, "", " ")
		for j := 0; j < 3; j++ {
			req := MirrorRequest{Tool: "test", ArgsHash: "test", SupportsCursor: trial%2 == 0, MaxBytes: 128 + rng.Intn(35000), Reserve: rng.Intn(100)}
			if j > 0 && n > 0 {
				req.Cursor = &MirrorCursor{O: rng.Intn(n), D: mirrorDigest(data)}
			}
			got, want := Mirror(data, req), oldMirror(data, req)
			if got != want {
				t.Fatalf("trial=%d offset=%v limit=%d\ngot %s\nwant %s", trial, req.Cursor, req.MaxBytes, got.Text, want.Text)
			}
		}
	}
}
func BenchmarkBatchBMirror(b *testing.B) {
	old := mirrorEventWriter
	mirrorEventWriter = io.Discard
	defer func() { mirrorEventWriter = old }()
	for _, n := range []int{1000, 10000} {
		items := make([]any, n)
		for i := range items {
			items[i] = map[string]any{"listing_id": 1000000001 + i, "title": strings.Repeat("ceramic mug ", 125)}
		}
		for _, wrapped := range []bool{false, true} {
			var value any = items
			if wrapped {
				value = map[string]any{"meta": map[string]any{}, "results": map[string]any{"items": items}}
			}
			data, _ := json.Marshal(value)
			req := MirrorRequest{Tool: "test", ArgsHash: "test", SupportsCursor: true, MaxBytes: 32000}
			for _, tc := range []struct {
				name string
				run  func([]byte, MirrorRequest) MirrorResult
			}{{"before", oldMirror}, {"after", Mirror}} {
				b.Run(fmt.Sprintf("n=%d/wrapper=%v/%s", n, wrapped, tc.name), func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						tc.run(data, req)
					}
				})
			}
		}
	}
}

func TestBatchBBoundarySweep(t *testing.T) {
	old := mirrorEventWriter
	mirrorEventWriter = io.Discard
	defer func() { mirrorEventWriter = old }()
	items := make([]any, 30)
	for i := range items {
		items[i] = map[string]any{"v": "<&é"}
	}
	for _, value := range []any{items, map[string]any{"items": items}, map[string]any{"meta": map[string]any{}, "results": map[string]any{"items": items}}} {
		data, _ := json.MarshalIndent(value, "", " ")
		for _, cursor := range []bool{false, true} {
			for limit := 128; limit <= len(data)+400; limit++ {
				req := MirrorRequest{Tool: "test", ArgsHash: "test", SupportsCursor: cursor, MaxBytes: limit}
				if got, want := Mirror(data, req), oldMirror(data, req); got != want {
					t.Fatalf("cursor=%v limit=%d got=%s want=%s", cursor, limit, got.Text, want.Text)
				}
			}
		}
	}
}
func TestBatchBItemCap(t *testing.T) {
	old := mirrorEventWriter
	mirrorEventWriter = io.Discard
	defer func() { mirrorEventWriter = old }()
	for _, n := range []int{100, 1000, 10001, 10005} {
		data := []byte("[" + strings.Repeat(" 0 ,", n-1) + " 0 ]")
		for _, budget := range []int{32000, 200000} {
			req := MirrorRequest{Tool: "test", ArgsHash: "test", MaxBytes: budget}
			// Force canonical paging even when the compact input fits.
			data = append(data, []byte(strings.Repeat(" ", budget))...)
			got, want := Mirror(data, req), oldMirror(data, req)
			if got != want {
				t.Fatalf("n=%d budget=%d got=%s want=%s", n, budget, got.Text, want.Text)
			}
			if n > 10000 && (!strings.Contains(got.Text, `"returned_count":10000`) || !strings.Contains(got.Text, `"truncated":true`)) {
				t.Fatalf("cap: %s", got.Text)
			}
		}
	}
}
func TestBatchBPagingWorkBound(t *testing.T) {
	// Measure only paging: parsing every input item necessarily scales with N.
	var allocated []uint64
	large := json.RawMessage(`{"v":"` + strings.Repeat("x", 32000) + `"}`)
	for _, n := range []int{1000, 20000} {
		items := make([]json.RawMessage, n)
		for i := range items {
			items[i] = json.RawMessage(`{"v":"tiny"}`)
			if i >= 1000 {
				items[i] = large
			}
		}
		mirrorPageSize(items, 100, 200, 1000)
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		for i := 0; i < 5; i++ {
			mirrorPageSize(items, 100, 200, 1000)
		}
		runtime.ReadMemStats(&after)
		allocated = append(allocated, after.TotalAlloc-before.TotalAlloc)
	}
	if allocated[1] > 2*allocated[0] {
		t.Fatalf("paging allocated bytes grow: %v", allocated)
	}
}
