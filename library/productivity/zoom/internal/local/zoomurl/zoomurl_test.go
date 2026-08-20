package zoomurl

import (
	"strings"
	"testing"
)

func TestBuild(t *testing.T) {
	tests := []struct {
		name    string
		p       Params
		want    string
		wantErr bool
	}{
		{
			name: "join with id and pwd",
			p:    Params{Action: ActionJoin, ConfNo: "85123456789", Pwd: "123456"},
			want: "zoommtg://zoom.us/join?confno=85123456789&pwd=123456",
		},
		{
			name: "default action is join",
			p:    Params{ConfNo: "85123456789"},
			want: "zoommtg://zoom.us/join?confno=85123456789",
		},
		{
			name:    "missing confno is an error",
			p:       Params{Action: ActionJoin},
			wantErr: true,
		},
		{
			name:    "encrypted password is rejected",
			p:       Params{ConfNo: "85123456789", Pwd: "AbCdEf1234+/=", Encrypted: true},
			wantErr: true,
		},
		{
			name: "start with zak",
			p:    Params{Action: ActionStart, ConfNo: "85123456789", UID: "uid42", ZakToken: "ZAK"},
			want: "zoommtg://zoom.us/start?confno=85123456789&token=ZAK&uid=uid42",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Build(tt.p)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Build returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Build = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantID     string
		wantPwd    string
		wantEnc    bool
		wantErr    bool
		wantAction Action
	}{
		{
			name:       "https zoom URL with raw pwd",
			raw:        "https://us02web.zoom.us/j/85123456789?pwd=123456",
			wantID:     "85123456789",
			wantPwd:    "123456",
			wantAction: ActionJoin,
		},
		{
			name:       "https zoom URL with encrypted pwd",
			raw:        "https://zoom.us/j/85123456789?pwd=aBcDeFGhIjK12345+/==",
			wantID:     "85123456789",
			wantPwd:    "aBcDeFGhIjK12345+/==",
			wantEnc:    true,
			wantAction: ActionJoin,
		},
		{
			name:       "zoommtg scheme",
			raw:        "zoommtg://zoom.us/join?confno=85123456789&pwd=abc",
			wantID:     "85123456789",
			wantPwd:    "abc",
			wantAction: ActionJoin,
		},
		{
			name:       "zoommtg scheme start",
			raw:        "zoommtg://zoom.us/start?confno=85123456789&token=ZAK",
			wantID:     "85123456789",
			wantAction: ActionStart,
		},
		{
			name:       "bare numeric ID with dashes",
			raw:        "851-2345-6789",
			wantID:     "85123456789",
			wantAction: ActionJoin,
		},
		{
			name:    "garbage input",
			raw:     "not a zoom url",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}
			if got.ConfNo != tt.wantID {
				t.Errorf("ConfNo = %q, want %q", got.ConfNo, tt.wantID)
			}
			if got.Pwd != tt.wantPwd {
				t.Errorf("Pwd = %q, want %q", got.Pwd, tt.wantPwd)
			}
			if got.Encrypted != tt.wantEnc {
				t.Errorf("Encrypted = %v, want %v", got.Encrypted, tt.wantEnc)
			}
			if got.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", got.Action, tt.wantAction)
			}
		})
	}
}

func TestBuildIncludesAllFields(t *testing.T) {
	got, err := Build(Params{
		Action: ActionJoin, ConfNo: "85123456789", Pwd: "raw", Uname: "Maya", Stype: "101",
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, want := range []string{"confno=85123456789", "pwd=raw", "uname=Maya", "stype=101"} {
		if !strings.Contains(got, want) {
			t.Errorf("Build missing %q: %s", want, got)
		}
	}
}
