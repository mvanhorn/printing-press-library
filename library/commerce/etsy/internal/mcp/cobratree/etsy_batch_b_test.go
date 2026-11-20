package cobratree

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

func TestBatchBAdmission(t *testing.T) {
	l := newMirrorLimiter()
	oldSlots, oldTimeout, oldLocal := l.slots, l.timeout, l.local
	l.slots = make(chan struct{}, 2)
	l.timeout = 50 * time.Millisecond
	l.local = &mirrorLimiter{slots: make(chan struct{}, 2), timeout: l.timeout, localRead: true}
	defer func() { l.slots = oldSlots; l.timeout = oldTimeout; l.local = oldLocal }()
	helper := filepath.Join(t.TempDir(), "child")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nif [ \"$1\" = fail ]; then exit 1; fi\nprintf '{}'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	root := &cobra.Command{Use: "etsy"}
	for name, ann := range map[string]map[string]string{
		"slow": {}, "fail": {}, "local": {"mcp:read-only": "true", "pp:data-source": "local"},
		"live":        {"mcp:read-only": "true", "pp:data-source": "live"},
		"conditional": {"pp:data-source": "local", "mcp:read-only-unless-flags": "refresh"},
	} {
		c := &cobra.Command{Use: name, Annotations: ann, Run: func(*cobra.Command, []string) {}}
		c.Flags().String("query", "", "synthetic argument")
		root.AddCommand(c)
	}
	srv := server.NewMCPServer("test", "1")
	RegisterAll(srv, root, func() (string, error) { return helper, nil })
	tools := srv.ListTools()
	req := mcplib.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "must not echo"}
	l.slots <- struct{}{}
	l.slots <- struct{}{}
	for _, name := range []string{"slow", "live", "conditional"} {
		r, e := tools[name].Handler(context.Background(), req)
		if e != nil || !r.IsError || len(r.Content) != 1 || mirrorTestText(t, r) != "busy: 2 mirror calls in flight (limit PP_MCP_MAX_CONCURRENCY); retry" {
			t.Fatalf("%s busy: %v %v", name, r, e)
		}
	}
	start := time.Now()
	r, e := tools["local"].Handler(context.Background(), req)
	if e != nil || r.IsError || time.Since(start) > time.Second {
		t.Fatalf("local starved: %v %v", r, e)
	}
	l.local.slots <- struct{}{}
	l.local.slots <- struct{}{}
	r, e = tools["local"].Handler(context.Background(), req)
	if e != nil || !r.IsError || len(r.Content) != 1 || mirrorTestText(t, r) != "busy: 2 local read calls in flight; retry" {
		t.Fatalf("local busy: %v %v", r, e)
	}
	l.timeout = 2 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	timer := time.AfterFunc(5*time.Millisecond, cancel)
	defer timer.Stop()
	start = time.Now()
	_, err := l.acquire(ctx)
	if err != context.Canceled || time.Since(start) > time.Second {
		t.Fatalf("cancel: %v", err)
	}
	<-l.slots
	<-l.slots
	r, e = tools["fail"].Handler(context.Background(), req)
	if e != nil || !r.IsError || len(l.slots) != 0 {
		t.Fatalf("failure slot leak: %v %v", r, e)
	}
}
func TestBatchBEnvWiring(t *testing.T) {
	if os.Getenv("BATCH_B_REEXEC") == "1" {
		l := newMirrorLimiter()
		if cap(l.slots) != 2 || cap(l.local.slots) != 8 || l.timeout != 50*time.Millisecond || l.local.timeout != l.timeout {
			t.Fatalf("running config: %+v", l)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestBatchBEnvWiring$")
	cmd.Env = append(os.Environ(), "BATCH_B_REEXEC=1", "PP_MCP_MAX_CONCURRENCY=2", "PP_MCP_QUEUE_TIMEOUT=50ms")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}
func TestBatchBLimiterMaximum(t *testing.T) {
	for _, n := range []int{1, 2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			l := &mirrorLimiter{slots: make(chan struct{}, n), timeout: time.Second}
			var mu sync.Mutex
			active, peak := 0, 0
			var wg sync.WaitGroup
			barrier := make(chan struct{})
			for i := 0; i < n+1; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					release, e := l.acquire(context.Background())
					if e != nil {
						t.Error(e)
						return
					}
					defer release()
					mu.Lock()
					active++
					if active > peak {
						peak = active
					}
					mu.Unlock()
					<-barrier
					mu.Lock()
					active--
					mu.Unlock()
				}()
			}
			deadline := time.Now().Add(time.Second)
			for {
				mu.Lock()
				ready := active == n
				mu.Unlock()
				if ready {
					break
				}
				if time.Now().After(deadline) {
					close(barrier)
					wg.Wait()
					t.Fatal("admission barrier timed out")
				}
				time.Sleep(time.Millisecond)
			}
			close(barrier)
			wg.Wait()
			if peak != n {
				t.Fatalf("peak=%d want=%d", peak, n)
			}
		})
	}
}

// raw=false stands in for the old string-copy path.
func BenchmarkBatchBCopies(b *testing.B) {
	for _, size := range []int{2000, 20000} {
		data := []byte(`{"title":"` + strings.Repeat("ceramic mug ", size) + `"}`)
		for _, raw := range []bool{false, true} {
			b.Run(fmt.Sprintf("bytes=%d/raw=%v", len(data), raw), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					var result CLICommandResult
					if raw {
						result.rawStdout = data
					} else {
						result.Stdout = string(data)
					}
					ToolResultFromCLICommand(result)
				}
			})
		}
	}
}

func TestBatchBConfiguration(t *testing.T) {
	for _, tc := range []struct {
		n, wait  string
		wantN    int
		wantWait time.Duration
		warn     bool
	}{
		{"", "", 4, 20 * time.Second, false}, {"1", "1ms", 1, time.Millisecond, false}, {"32", "5m", 32, 5 * time.Minute, false},
		{"0", "0s", 4, 20 * time.Second, true}, {"33", "5m1s", 4, 20 * time.Second, true}, {"-1", "-1s", 4, 20 * time.Second, true},
		{"2", "bad", 2, 20 * time.Second, true}, {"bad", "50ms", 4, 50 * time.Millisecond, true},
		{"bad", "bad", 4, 20 * time.Second, true}, {"999999999999999999999999", "1ns", 4, 20 * time.Second, true},
	} {
		t.Run(tc.n+"/"+tc.wait, func(t *testing.T) {
			t.Setenv("PP_MCP_MAX_CONCURRENCY", tc.n)
			t.Setenv("PP_MCP_QUEUE_TIMEOUT", tc.wait)
			original := os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stderr = w
			l := configuredMirrorLimiter()
			os.Stderr = original
			w.Close()
			warning, _ := io.ReadAll(r)
			r.Close()
			if cap(l.slots) != tc.wantN || l.timeout != tc.wantWait {
				t.Fatalf("config: %d %s", cap(l.slots), l.timeout)
			}
			if tc.warn {
				if (tc.wantN == 4 && tc.n != "4" && !strings.Contains(string(warning), "PP_MCP_MAX_CONCURRENCY invalid; using default 4")) || (tc.wantWait == 20*time.Second && !strings.Contains(string(warning), "PP_MCP_QUEUE_TIMEOUT invalid; using default 20s")) {
					t.Fatalf("warnings: %s", warning)
				}
			} else if len(warning) > 0 {
				t.Fatalf("unexpected warning: %s", warning)
			}
		})
	}
}
func TestBatchBRealChildrenMaximum(t *testing.T) {
	l := newMirrorLimiter()
	oldSlots, oldTimeout := l.slots, l.timeout
	defer func() { l.slots = oldSlots; l.timeout = oldTimeout }()
	dir := t.TempDir()
	helper := filepath.Join(dir, "child")
	events := filepath.Join(dir, "events")
	barrier := filepath.Join(dir, "release")
	// A single printf writes each event with O_APPEND; count overlapping children.
	script := fmt.Sprintf("#!/bin/sh\nprintf 'start\\n' >> '%s'\nwhile [ ! -f '%s' ]; do /bin/sleep 0.005; done\nprintf 'end\\n' >> '%s'\nprintf '{}'\n", events, barrier, events)
	if err := os.WriteFile(helper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	handler := shellOutToCLI(func() (string, error) { return helper, nil }, nil, nil, map[string]bool{}, nil, false, nil)
	for _, n := range []int{1, 2, 4} {
		l.slots = make(chan struct{}, n)
		l.timeout = 2 * time.Second
		if err := os.WriteFile(events, nil, 0600); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for i := 0; i < n+1; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r, e := handler(context.Background(), mcplib.CallToolRequest{})
				if e != nil || r.IsError {
					t.Errorf("child: %v %v", r, e)
				}
			}()
		}
		deadline := time.Now().Add(time.Second)
		for {
			data, _ := os.ReadFile(events)
			if strings.Count(string(data), "start") >= n {
				break
			}
			if time.Now().After(deadline) {
				os.WriteFile(barrier, nil, 0600)
				wg.Wait()
				t.Fatal("children barrier timed out")
			}
			time.Sleep(time.Millisecond)
		}
		if err := os.WriteFile(barrier, nil, 0600); err != nil {
			t.Fatal(err)
		}
		wg.Wait()
		if err := os.Remove(barrier); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(events)
		if err != nil {
			t.Fatal(err)
		}
		active, peak, starts := 0, 0, 0
		for _, event := range strings.Fields(string(data)) {
			if event == "start" {
				active++
				starts++
				if active > peak {
					peak = active
				}
			} else if event == "end" {
				active--
			} else {
				t.Fatalf("bad event %q", event)
			}
		}
		if starts != n+1 || peak != n || active != 0 {
			t.Fatalf("N=%d starts=%d peak=%d active=%d", n, starts, peak, active)
		}
	}
}
