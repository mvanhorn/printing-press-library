package machines

import (
	"context"
	"testing"
	"time"
)

type snapSeq struct {
	frames [][]byte
	errs   []error
	idx    int
}

func (s *snapSeq) Snapshot(ctx context.Context) ([]byte, error) {
	if s.idx >= len(s.frames) {
		return nil, nil
	}
	f := s.frames[s.idx]
	e := error(nil)
	if s.idx < len(s.errs) {
		e = s.errs[s.idx]
	}
	s.idx++
	if e != nil {
		return nil, e
	}
	return f, nil
}

type ocrSeq struct {
	snapSeq
	texts []string
}

func (o *ocrSeq) OCR(ctx context.Context, b []byte) (string, error) {
	// text aligned to snapshot index-1
	i := o.idx - 1
	if i < 0 || i >= len(o.texts) {
		return "", nil
	}
	return o.texts[i], nil
}

func TestVerifyFrameChange_SleepsAndAttempts(t *testing.T) {
	seq := &snapSeq{frames: [][]byte{[]byte("a"), []byte("a"), []byte("b")}}
	var sleeps []time.Duration
	sleep := func(ctx context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil }
	ok, err := VerifyFrameChange(context.Background(), seq, []byte("a"), 5, time.Second, sleep)
	if err != nil || !ok || len(sleeps) != 2 {
		t.Fatalf("ok=%v err=%v sleeps=%v", ok, err, sleeps)
	}
}

func TestVerifyFrameChange_BoundedAttempts(t *testing.T) {
	seq := &snapSeq{frames: [][]byte{[]byte("a"), []byte("a"), []byte("a")}}
	ok, _ := VerifyFrameChange(context.Background(), seq, []byte("a"), 2, 0, nil)
	if ok {
		t.Fatal("should not detect change")
	}
}

func TestVerifyOCRIdentity_MatchesSubstring(t *testing.T) {
	inv := DefaultInventory()
	tgt, _ := inv.Resolve("pve1")
	seq := &ocrSeq{snapSeq: snapSeq{frames: [][]byte{{1}, {2}}}, texts: []string{"hello", "pve1 login:"}}
	ok, txt, err := VerifyOCRIdentity(context.Background(), seq, tgt, 5, 0, nil)
	if err != nil || !ok || txt != "pve1 login:" {
		t.Fatalf("ok=%v txt=%q err=%v", ok, txt, err)
	}
}

func TestVerifyPromptPattern_MatchesRegex(t *testing.T) {
	inv := DefaultInventory()
	tgt, _ := inv.Resolve("kodi-build")
	seq := &ocrSeq{snapSeq: snapSeq{frames: [][]byte{{1}}}, texts: []string{"Keyboard Setup Assistant"}}
	ok, _, err := VerifyPromptPattern(context.Background(), seq, tgt, 1, 0, nil)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestRunVerifyPolicy_NoneAndFrameChange(t *testing.T) {
	inv := DefaultInventory()
	tgt, _ := inv.Resolve("pve3")
	seq := &snapSeq{frames: [][]byte{[]byte("x"), []byte("y")}}
	ok, detail, _ := RunVerifyPolicy(context.Background(), VerifyNone, seq, tgt, nil, 1, 0, nil)
	if ok || detail == "" {
		t.Fatalf("none %v %q", ok, detail)
	}
	ok, _, err := RunVerifyPolicy(context.Background(), VerifyFrameChangePolicy, seq, tgt, nil, 1, 0, nil)
	if err == nil {
		t.Fatal("expected baseline error")
	}
	ok, detail, _ = RunVerifyPolicy(context.Background(), VerifyFrameChangePolicy, seq, tgt, []byte("x"), 2, 0, nil)
	if !ok || detail != "screen changed" {
		t.Fatalf("frame %v %q", ok, detail)
	}
}

func TestVerifyCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	seq := &snapSeq{frames: [][]byte{[]byte("a")}}
	if _, err := VerifyFrameChange(ctx, seq, []byte("a"), 5, time.Second, nil); err == nil {
		t.Fatal("expected canceled")
	}
}
