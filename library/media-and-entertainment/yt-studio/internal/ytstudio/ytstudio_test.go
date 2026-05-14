package ytstudio

import "testing"

func TestSessionCookieHeader_SortsKeys(t *testing.T) {
	t.Parallel()
	s := &SessionFile{
		Cookies: map[string]string{
			"SAPISID":           "value1",
			"LOGIN_INFO":        "value2",
			"__Secure-3PAPISID": "value3",
		},
	}
	got := s.CookieHeader()
	// Should be alphabetical
	want := "LOGIN_INFO=value2; SAPISID=value1; __Secure-3PAPISID=value3"
	if got != want {
		t.Errorf("CookieHeader = %q, want %q", got, want)
	}
}

func TestSAPISID_FallsBackToSecure3PAPISID(t *testing.T) {
	t.Parallel()
	s := &SessionFile{Cookies: map[string]string{"__Secure-3PAPISID": "fallback"}}
	if got := s.SAPISID(); got != "fallback" {
		t.Errorf("expected fallback, got %q", got)
	}
	s2 := &SessionFile{Cookies: map[string]string{"SAPISID": "primary", "__Secure-3PAPISID": "fallback"}}
	if got := s2.SAPISID(); got != "primary" {
		t.Errorf("expected primary, got %q", got)
	}
	s3 := &SessionFile{Cookies: map[string]string{"random": "value"}}
	if got := s3.SAPISID(); got != "" {
		t.Errorf("expected empty when neither present, got %q", got)
	}
}

func TestEffectiveClientName_DefaultsToStudio(t *testing.T) {
	t.Parallel()
	s := &SessionFile{}
	if got := s.EffectiveClientName(); got != "62" {
		t.Errorf("default should be Studio (62), got %q", got)
	}
	s.ClientName = "85"
	if got := s.EffectiveClientName(); got != "85" {
		t.Errorf("explicit override should win, got %q", got)
	}
}
