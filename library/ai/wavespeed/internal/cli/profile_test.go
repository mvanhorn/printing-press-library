package cli

import (
	"reflect"
	"testing"
)

func TestProfileArrayValuesParsesPflagStringArrayOutput(t *testing.T) {
	got := profileArrayValues(`["@style1.png","@style2.png","https://example.com/input.png"]`)
	want := []string{"@style1.png", "@style2.png", "https://example.com/input.png"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("profileArrayValues() = %#v, want %#v", got, want)
	}
}

func TestProfileArrayValuesEmptyArray(t *testing.T) {
	if got := profileArrayValues(`[]`); got != nil {
		t.Fatalf("profileArrayValues(empty array) = %#v, want nil", got)
	}
}

func TestOpenCommandUsesPlatformViewer(t *testing.T) {
	cases := []struct {
		goos     string
		wantName string
		wantArgs []string
	}{
		{goos: "darwin", wantName: "open", wantArgs: []string{"/tmp/out.png"}},
		{goos: "linux", wantName: "xdg-open", wantArgs: []string{"/tmp/out.png"}},
		{goos: "windows", wantName: "rundll32", wantArgs: []string{"url.dll,FileProtocolHandler", `/tmp/out.png`}},
	}
	for _, tc := range cases {
		t.Run(tc.goos, func(t *testing.T) {
			name, args, err := openCommand("/tmp/out.png", tc.goos)
			if err != nil {
				t.Fatal(err)
			}
			if name != tc.wantName || !reflect.DeepEqual(args, tc.wantArgs) {
				t.Fatalf("openCommand() = %q %#v, want %q %#v", name, args, tc.wantName, tc.wantArgs)
			}
		})
	}
}
