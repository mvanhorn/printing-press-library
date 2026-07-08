package trust

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	sfclient "github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/client"
)

type fakeOrgClient struct {
	post    func(path string, body any) (json.RawMessage, int, error)
	patch   func(path string, body any) (json.RawMessage, int, error)
	get     func(path string, params map[string]string) (json.RawMessage, error)
	posts   []string
	patches []string
}

func (f *fakeOrgClient) Post(path string, body any) (json.RawMessage, int, error) {
	f.posts = append(f.posts, path)
	if f.post != nil {
		return f.post(path, body)
	}
	return json.RawMessage(`{"id":"0P1TEST","success":true,"errors":[]}`), 201, nil
}

func (f *fakeOrgClient) Patch(path string, body any) (json.RawMessage, int, error) {
	f.patches = append(f.patches, path)
	if f.patch != nil {
		return f.patch(path, body)
	}
	return json.RawMessage(`{"success":true}`), 204, nil
}

func (f *fakeOrgClient) Get(path string, params map[string]string) (json.RawMessage, error) {
	if f.get != nil {
		return f.get(path, params)
	}
	return json.RawMessage(`{"records":[]}`), nil
}

func fixedRegisterOptions() RegisterOptions {
	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	return RegisterOptions{
		OrgAlias:        "prod",
		OrgID:           "00DTEST",
		HostFingerprint: "abcdef123456",
		UserID:          "005TEST",
		Now:             func() time.Time { return now },
	}
}

func TestRegisterCreatesCertificateRecord(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	client := &fakeOrgClient{}

	result, err := RegisterKey(client, fixedRegisterOptions())
	if err != nil {
		t.Fatalf("RegisterKey: %v", err)
	}
	if result.Source != "certificate" {
		t.Fatalf("source = %s, want certificate", result.Source)
	}
	if result.CertificateID != "0P1TEST" {
		t.Fatalf("certificate id = %s", result.CertificateID)
	}

	record, err := LoadKeyRecord(result.KID)
	if err != nil {
		t.Fatalf("LoadKeyRecord: %v", err)
	}
	if record.HostFingerprint != "abcdef123456" || record.IssuerUserID != "005TEST" {
		t.Fatalf("identity not persisted: %#v", record)
	}
	if len(client.posts) != 1 || client.posts[0] != "/services/data/v63.0/tooling/sobjects/Certificate" {
		t.Fatalf("posts = %#v", client.posts)
	}
}

func TestRegisterFallsBackToCMDTWhenCertificateUnavailable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	client := &fakeOrgClient{
		post: func(path string, body any) (json.RawMessage, int, error) {
			if path == "/services/data/v63.0/tooling/sobjects/Certificate" {
				return nil, 400, &sfclient.APIError{Method: "POST", Path: path, StatusCode: 400, Body: `[{"errorCode":"INVALID_TYPE"}]`}
			}
			return json.RawMessage(`{"id":"m00TEST","success":true,"errors":[]}`), 201, nil
		},
	}

	result, err := RegisterKey(client, fixedRegisterOptions())
	if err != nil {
		t.Fatalf("RegisterKey: %v", err)
	}
	if result.Source != "cmdt" {
		t.Fatalf("source = %s, want cmdt", result.Source)
	}
	record, err := LoadKeyRecord(result.KID)
	if err != nil {
		t.Fatalf("LoadKeyRecord: %v", err)
	}
	if record.Receipt == "" {
		t.Fatal("expected CMDT receipt")
	}
	if err := VerifyReceiptChain([]KeyRecord{*record}); err != nil {
		t.Fatalf("VerifyReceiptChain: %v", err)
	}
	if len(client.posts) != 2 {
		t.Fatalf("expected certificate + cmdt posts, got %#v", client.posts)
	}
}

func TestRegisterIsIdempotentForActiveDeviceKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	opts := fixedRegisterOptions()
	first, err := RegisterKey(&fakeOrgClient{}, opts)
	if err != nil {
		t.Fatalf("first RegisterKey: %v", err)
	}
	secondClient := &fakeOrgClient{post: func(path string, body any) (json.RawMessage, int, error) {
		return nil, 0, errors.New("should not post")
	}}
	second, err := RegisterKey(secondClient, opts)
	if err != nil {
		t.Fatalf("second RegisterKey: %v", err)
	}
	if !second.Idempotent || second.KID != first.KID {
		t.Fatalf("expected idempotent same kid: first=%#v second=%#v", first, second)
	}
	if len(secondClient.posts) != 0 {
		t.Fatalf("unexpected posts: %#v", secondClient.posts)
	}
}

// TestRegisterFallsBackToCMDTOnCertificateSchemaDrift locks F-022: a
// Tooling API whose Certificate sobject no longer carries CertificateData
// (observed live at v63.0: HTTP 400 "No such column 'CertificateData' on
// sobject of type Certificate") must fall through to the CMDT path instead
// of failing registration.
func TestRegisterFallsBackToCMDTOnCertificateSchemaDrift(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	client := &fakeOrgClient{
		post: func(path string, body any) (json.RawMessage, int, error) {
			if path == "/services/data/v63.0/tooling/sobjects/Certificate" {
				return nil, 400, &sfclient.APIError{Method: "POST", Path: path, StatusCode: 400, Body: `[{"message":"No such column 'CertificateData' on sobject of type Certificate","errorCode":"INVALID_FIELD"}]`}
			}
			return json.RawMessage(`{"id":"m00TEST","success":true,"errors":[]}`), 201, nil
		},
	}

	result, err := RegisterKey(client, fixedRegisterOptions())
	if err != nil {
		t.Fatalf("RegisterKey: %v", err)
	}
	if result.Source != "cmdt" {
		t.Fatalf("source = %s, want cmdt", result.Source)
	}
}
