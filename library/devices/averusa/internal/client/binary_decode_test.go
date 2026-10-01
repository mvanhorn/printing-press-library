package client

import (
	"bytes"
	"strings"
	"testing"
)

func TestDecodeBinaryResponse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []byte
		wantErr string
	}{
		{name: "plain text", input: "plain document", want: []byte("plain document")},
		{name: "raw binary", input: "%PDF-1.7\x00\xff", want: []byte("%PDF-1.7\x00\xff")},
		{name: "ordinary JSON", input: `{"message":"hello"}`, want: []byte(`{"message":"hello"}`)},
		{name: "base64 envelope", input: `{"_pp_binary":true,"encoding":"base64","bytes":3,"data":"AAH/"}`, want: []byte{0, 1, 255}},
		{name: "unsupported encoding", input: `{"_pp_binary":true,"encoding":"hex","bytes":2,"data":"00"}`, wantErr: "unsupported binary encoding"},
		{name: "invalid base64", input: `{"_pp_binary":true,"encoding":"base64","bytes":1,"data":"%%%"}`, wantErr: "invalid binary payload"},
		{name: "length mismatch", input: `{"_pp_binary":true,"encoding":"base64","bytes":4,"data":"AAH/"}`, wantErr: "binary payload length mismatch"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeBinaryResponse([]byte(tc.input))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("decoded bytes = %q, want %q", got, tc.want)
			}
		})
	}
}
