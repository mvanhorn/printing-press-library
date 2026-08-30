// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0.

package anylist

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"google.golang.org/protobuf/proto"
)

func TestRecipeLinkMethodsUseCapturedMultipartContracts(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Path: filepath.Join(t.TempDir(), "config.toml"), AccessToken: "access-token", UserID: "requesting-user"}
	client := New(cfg)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", req.Method)
		}
		if got := req.Header.Get("Content-Type"); len(got) < len("multipart/form-data; boundary=") || got[:len("multipart/form-data; boundary=")] != "multipart/form-data; boundary=" {
			t.Fatalf("Content-Type = %q, want multipart/form-data", got)
		}
		fields := readMultipartFields(t, req)
		var response proto.Message
		switch req.URL.Path {
		case "/data/user-recipe-data/request-recipe-link-v2":
			var request pb.PBRecipeLinkRequest
			if fields["link_request"].ContentType != "application/octet-stream" {
				t.Fatalf("link_request content type = %q, want application/octet-stream", fields["link_request"].ContentType)
			}
			if err := proto.Unmarshal(fields["link_request"].Value, &request); err != nil {
				t.Fatalf("request protobuf: %v", err)
			}
			if request.GetIdentifier() != "request-1" || request.GetRequestingUserId() != "requesting-user" || request.GetConfirmingEmail() != "person@example.com" {
				t.Fatalf("request = %#v", &request)
			}
			response = &pb.PBRecipeLinkRequestResponse{StatusCode: http.StatusOK}
		case "/data/user-recipe-data/cancel-recipe-link-request":
			var request pb.PBRecipeLinkRequest
			if fields["link_request"].ContentType != "application/octet-stream" {
				t.Fatalf("cancel link_request content type = %q, want application/octet-stream", fields["link_request"].ContentType)
			}
			if err := proto.Unmarshal(fields["link_request"].Value, &request); err != nil {
				t.Fatalf("cancel protobuf: %v", err)
			}
			if request.GetIdentifier() != "request-1" {
				t.Fatalf("cancel request ID = %q, want request-1", request.GetIdentifier())
			}
			response = &pb.PBRecipeDataResponse{RecipeDataId: "recipe-data-1"}
		case "/data/user-recipe-data/accept-recipe-link-request":
			if string(fields["link_request_id"].Value) != "request-1" || string(fields["user_id"].Value) != "linked-user" {
				t.Fatalf("accept fields = %#v", fields)
			}
			response = &pb.PBRecipeDataResponse{RecipeDataId: "recipe-data-1"}
		case "/data/user-recipe-data/unlink-recipes":
			if string(fields["user_id"].Value) != "linked-user" {
				t.Fatalf("unlink user ID = %q", fields["user_id"].Value)
			}
			response = &pb.PBRecipeDataResponse{RecipeDataId: "recipe-data-1"}
		default:
			t.Fatalf("unexpected path %q", req.URL.Path)
		}
		body, err := proto.Marshal(response)
		if err != nil {
			t.Fatalf("marshal response: %v", err)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header), Request: req}, nil
	})}

	request := &pb.PBRecipeLinkRequest{Identifier: "request-1", RequestingUserId: "requesting-user", ConfirmingEmail: "person@example.com"}
	if response, err := client.RequestRecipeLink(t.Context(), request); err != nil || response.GetStatusCode() != http.StatusOK {
		t.Fatalf("RequestRecipeLink = %#v, %v", response, err)
	}
	if response, err := client.CancelRecipeLink(t.Context(), request); err != nil || response.GetRecipeDataId() != "recipe-data-1" {
		t.Fatalf("CancelRecipeLink = %#v, %v", response, err)
	}
	if response, err := client.AcceptRecipeLink(t.Context(), "request-1", "linked-user"); err != nil || response.GetRecipeDataId() != "recipe-data-1" {
		t.Fatalf("AcceptRecipeLink = %#v, %v", response, err)
	}
	if response, err := client.UnlinkRecipes(t.Context(), "linked-user"); err != nil || response.GetRecipeDataId() != "recipe-data-1" {
		t.Fatalf("UnlinkRecipes = %#v, %v", response, err)
	}
}

func TestRecipeLinkMethodsRejectMissingIdentifiers(t *testing.T) {
	t.Parallel()

	client := New(&config.Config{UserID: "requesting-user"})
	if _, err := client.RequestRecipeLink(t.Context(), &pb.PBRecipeLinkRequest{ConfirmingEmail: "person@example.com"}); err == nil {
		t.Fatal("RequestRecipeLink accepted a request without identifier/user")
	}
	if _, err := client.CancelRecipeLink(t.Context(), &pb.PBRecipeLinkRequest{}); err == nil {
		t.Fatal("CancelRecipeLink accepted a request without identifier")
	}
	if _, err := client.AcceptRecipeLink(t.Context(), "", "user"); err == nil {
		t.Fatal("AcceptRecipeLink accepted an empty request ID")
	}
	if _, err := client.UnlinkRecipes(t.Context(), " "); err == nil {
		t.Fatal("UnlinkRecipes accepted an empty user ID")
	}
}

type capturedMultipartField struct {
	Value       []byte
	ContentType string
}

func readMultipartFields(t *testing.T, req *http.Request) map[string]capturedMultipartField {
	t.Helper()
	reader, err := req.MultipartReader()
	if err != nil {
		t.Fatalf("MultipartReader returned error: %v", err)
	}
	fields := map[string]capturedMultipartField{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart returned error: %v", err)
		}
		value, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("reading multipart field %q: %v", part.FormName(), err)
		}
		fields[part.FormName()] = capturedMultipartField{Value: value, ContentType: part.Header.Get("Content-Type")}
	}
	return fields
}
