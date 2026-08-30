// Package anylist: custom shopping-list category mutations (create + rename).
//
// These operations use the wire contract proven by live probes: a
// PBListOperationList posted as the binary "operations" multipart part
// (application/octet-stream) to POST /data/shopping-lists/update-v2, with the
// operation class ListCategoryOperation. The v1 /data/shopping-lists/update
// route and the form-urlencoded transport were proven non-persistent for
// category writes and must never be used for them; the existing
// sendListOperations / sendListOperationsMultipart helpers target that v1
// route and are intentionally not reused here.
package anylist

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/google/uuid"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"google.golang.org/protobuf/proto"
)

// CreateListCategory creates a custom category in the selected list using the
// create-category operation (handler "create-category", operation class
// ListCategoryOperation). The category must already carry a non-conflicting
// identifier, the target list ID, category group ID, name, and initial
// sortIndex. Callers must verify the new category by a fresh user-data read
// on its stable identifier before reporting success.
func (c *Client) CreateListCategory(ctx context.Context, listID string, category *pb.PBListCategory) error {
	listID = strings.TrimSpace(listID)
	if listID == "" {
		return fmt.Errorf("list ID must not be empty")
	}
	if category == nil {
		return fmt.Errorf("category payload must not be nil")
	}
	if strings.TrimSpace(category.GetIdentifier()) == "" {
		return fmt.Errorf("category identifier must not be empty")
	}
	if strings.TrimSpace(category.GetCategoryGroupId()) == "" {
		return fmt.Errorf("category group ID must not be empty")
	}
	if strings.TrimSpace(category.GetName()) == "" {
		return fmt.Errorf("category name must not be empty")
	}
	if category.GetListId() != listID {
		return fmt.Errorf("category list ID %q does not match list ID %q", category.GetListId(), listID)
	}
	op := &pb.PBListOperation{
		Metadata: &pb.PBOperationMetadata{
			OperationId:    uuid.NewString(),
			HandlerId:      "create-category",
			UserId:         c.cfg.UserID,
			OperationClass: int32(pb.PBOperationMetadata_ListCategoryOperation),
		},
		ListId:          listID,
		UpdatedCategory: proto.Clone(category).(*pb.PBListCategory),
	}
	return c.sendListOperationsCategoryV2(ctx, &pb.PBListOperationList{Operations: []*pb.PBListOperation{op}})
}

// RenameListCategory renames one custom category by stable identifier using
// the set-category-name operation (handler "set-category-name", operation
// class ListCategoryOperation). The original and updated records must share
// the same identifier, group, list, and sortIndex; only the name may differ.
// Callers must verify the renamed category by a fresh user-data read on its
// stable identifier before reporting success.
func (c *Client) RenameListCategory(ctx context.Context, listID string, original, updated *pb.PBListCategory) error {
	listID = strings.TrimSpace(listID)
	if listID == "" {
		return fmt.Errorf("list ID must not be empty")
	}
	if original == nil || updated == nil {
		return fmt.Errorf("original and updated category payloads must not be nil")
	}
	if strings.TrimSpace(original.GetIdentifier()) == "" {
		return fmt.Errorf("original category identifier must not be empty")
	}
	if original.GetIdentifier() != updated.GetIdentifier() {
		return fmt.Errorf("original category ID %q does not match updated category ID %q", original.GetIdentifier(), updated.GetIdentifier())
	}
	if original.GetCategoryGroupId() != updated.GetCategoryGroupId() {
		return fmt.Errorf("category rename must preserve category group %q", original.GetCategoryGroupId())
	}
	if original.GetSortIndex() != updated.GetSortIndex() {
		return fmt.Errorf("category rename must preserve sort index %d", original.GetSortIndex())
	}
	if strings.TrimSpace(updated.GetName()) == "" {
		return fmt.Errorf("updated category name must not be empty")
	}
	if strings.EqualFold(original.GetName(), updated.GetName()) {
		return fmt.Errorf("new category name must differ from the current name")
	}
	updatedCopy := proto.Clone(updated).(*pb.PBListCategory)
	updatedCopy.ListId = listID
	op := &pb.PBListOperation{
		Metadata: &pb.PBOperationMetadata{
			OperationId:    uuid.NewString(),
			HandlerId:      "set-category-name",
			UserId:         c.cfg.UserID,
			OperationClass: int32(pb.PBOperationMetadata_ListCategoryOperation),
		},
		ListId:           listID,
		OriginalCategory: proto.Clone(original).(*pb.PBListCategory),
		UpdatedCategory:  updatedCopy,
	}
	return c.sendListOperationsCategoryV2(ctx, &pb.PBListOperationList{Operations: []*pb.PBListOperation{op}})
}

// sendListOperationsCategoryV2 posts a PBListOperationList as the binary
// "operations" multipart part (application/octet-stream) to the proven
// persistent category-mutation route, POST /data/shopping-lists/update-v2.
// It never falls back to the non-persistent v1 route or a form-encoded
// transport.
func (c *Client) sendListOperationsCategoryV2(ctx context.Context, ops *pb.PBListOperationList) error {
	if ops == nil || len(ops.GetOperations()) == 0 {
		return fmt.Errorf("list operation list must not be empty")
	}
	dat, err := proto.Marshal(ops)
	if err != nil {
		return fmt.Errorf("marshaling category operations: %w", err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="operations"`)
	header.Set("Content-Type", "application/octet-stream")
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("creating category operations part: %w", err)
	}
	if _, err := part.Write(dat); err != nil {
		return fmt.Errorf("writing category operations part: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing category operations form: %w", err)
	}
	bodyBytes := body.Bytes()
	contentType := writer.FormDataContentType()
	resp, err := c.doWithRetry(ctx, func(b []byte) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/data/shopping-lists/update-v2", bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("creating category operation request: %w", err)
		}
		c.setAuthHeaders(req)
		req.Header.Set("Content-Type", contentType)
		return req, nil
	}, bodyBytes)
	if err != nil {
		return fmt.Errorf("sending category operations: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("category operation failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return nil
}
