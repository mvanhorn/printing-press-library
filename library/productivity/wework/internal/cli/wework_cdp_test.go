package cli

import "testing"

func TestPickWeworkTarget(t *testing.T) {
	targets := []cdpTarget{
		{Type: "page", URL: "https://www.google.com", WebSocketDebuggerURL: "ws://nope"},
		{Type: "service_worker", URL: "https://members.wework.com/sw", WebSocketDebuggerURL: "ws://sw"},
		{Type: "page", URL: "https://members.wework.com/workplaceone/content2/bookings/desks", WebSocketDebuggerURL: "ws://good"},
	}
	if got := pickWeworkTarget(targets); got != "ws://good" {
		t.Fatalf("got %q, want ws://good", got)
	}
	if got := pickWeworkTarget(nil); got != "" {
		t.Fatalf("expected empty for no targets, got %q", got)
	}
	// A page without a debugger URL is not selectable.
	if got := pickWeworkTarget([]cdpTarget{{Type: "page", URL: "https://members.wework.com/x"}}); got != "" {
		t.Fatalf("expected empty when no websocket URL, got %q", got)
	}
}

func TestParseWeworkSession(t *testing.T) {
	s, err := parseWeworkSession(`{"token":"tok","refresh":"ref","uuid":"u1","member":"1"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Token != "tok" || s.Refresh != "ref" || s.UUID != "u1" || s.Member != "1" {
		t.Fatalf("bad parse: %+v", s)
	}
	if e, err := parseWeworkSession(`{"error":"no WeWork session"}`); err != nil || e.Error == "" {
		t.Fatalf("expected error field parsed, got %+v err %v", e, err)
	}
	if _, err := parseWeworkSession("not json"); err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestParseCDPEvalMessage(t *testing.T) {
	v, done, err := parseCDPEvalMessage([]byte(`{"id":1,"result":{"result":{"type":"string","value":"HELLO"}}}`))
	if err != nil || !done || v != "HELLO" {
		t.Fatalf("got v=%q done=%v err=%v", v, done, err)
	}
	// Different id -> keep reading.
	if _, done, _ := parseCDPEvalMessage([]byte(`{"id":2,"method":"Runtime.consoleAPICalled"}`)); done {
		t.Fatal("id!=1 should not be done")
	}
	// Event frame (no id) -> keep reading.
	if _, done, _ := parseCDPEvalMessage([]byte(`{"method":"Network.requestWillBeSent"}`)); done {
		t.Fatal("event frame should not be done")
	}
	// Exception -> error, done.
	if _, done, err := parseCDPEvalMessage([]byte(`{"id":1,"result":{"exceptionDetails":{"text":"boom"}}}`)); err == nil || !done {
		t.Fatalf("expected exception error, got done=%v err=%v", done, err)
	}
}
