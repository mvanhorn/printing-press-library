// Package anylist: custom shopping-list category group mutations (delete +
// reorder).
//
// These operations use the wire contract proven by live probes: a
// PBListOperationList posted as the binary "operations" multipart part
// (application/octet-stream) to POST /data/shopping-lists/update-v2, with the
// operation class ListCategoryGroupOperation (4). The updatedCategoryGroup is
// a copy of the fresh group record with its categories field carrying the
// mutation payload: the full record(s) to remove for "remove-category-ids",
// or identifier-only records in the desired order for
// "set-sorted-category-ids". The v1 /data/shopping-lists/update route, the
// form-urlencoded transport, and the old non-persistent "remove-category"
// handler are never used for category writes; the existing
// sendListOperations / sendListOperationsMultipart helpers target that v1
// route and are intentionally not reused here.
package anylist

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"google.golang.org/protobuf/proto"
)

// DeleteListCategory removes exactly one custom category from its group using
// the remove-category-ids operation (handler "remove-category-ids", operation
// class ListCategoryGroupOperation). The payload copies the fresh category
// group and carries the single selected full category record in the group's
// categories field. Callers must verify the removal by a fresh user-data read
// confirming the category's stable identifier is absent before reporting
// success.
func (c *Client) DeleteListCategory(ctx context.Context, listID string, group *pb.PBListCategoryGroup, category *pb.PBListCategory) error {
	listID = strings.TrimSpace(listID)
	if listID == "" {
		return fmt.Errorf("list ID must not be empty")
	}
	if group == nil || strings.TrimSpace(group.GetIdentifier()) == "" {
		return fmt.Errorf("category group payload must not be nil and must carry a stable ID")
	}
	if category == nil || strings.TrimSpace(category.GetIdentifier()) == "" {
		return fmt.Errorf("category payload must not be nil and must carry a stable ID")
	}
	if category.GetCategoryGroupId() != group.GetIdentifier() {
		return fmt.Errorf("category ID %q belongs to group %q, not %q", category.GetIdentifier(), category.GetCategoryGroupId(), group.GetIdentifier())
	}
	if strings.TrimSpace(category.GetSystemCategory()) != "" {
		return fmt.Errorf("category %q is a system category and cannot be deleted", category.GetIdentifier())
	}
	if category.GetListId() != "" && category.GetListId() != listID {
		return fmt.Errorf("category list ID %q does not match list ID %q", category.GetListId(), listID)
	}
	updatedGroup := proto.Clone(group).(*pb.PBListCategoryGroup)
	updatedGroup.Categories = []*pb.PBListCategory{proto.Clone(category).(*pb.PBListCategory)}
	op := &pb.PBListOperation{
		Metadata: &pb.PBOperationMetadata{
			OperationId:    uuid.NewString(),
			HandlerId:      "remove-category-ids",
			UserId:         c.cfg.UserID,
			OperationClass: int32(pb.PBOperationMetadata_ListCategoryGroupOperation),
		},
		ListId:               listID,
		UpdatedCategoryGroup: updatedGroup,
	}
	return c.sendListOperationsCategoryV2(ctx, &pb.PBListOperationList{Operations: []*pb.PBListOperation{op}})
}

// ReorderListCategories sets the category order of one group using the
// set-sorted-category-ids operation (handler "set-sorted-category-ids",
// operation class ListCategoryGroupOperation). The payload copies the fresh
// category group and carries identifier-only PBListCategory records in the
// desired order; any caller-record fields beyond the identifier are dropped so
// the wire shape cannot drift from the proven one. Callers must verify the
// resulting exact stable-ID order by a fresh user-data read before reporting
// success.
func (c *Client) ReorderListCategories(ctx context.Context, listID string, group *pb.PBListCategoryGroup, orderedCategories []*pb.PBListCategory) error {
	listID = strings.TrimSpace(listID)
	if listID == "" {
		return fmt.Errorf("list ID must not be empty")
	}
	if group == nil || strings.TrimSpace(group.GetIdentifier()) == "" {
		return fmt.Errorf("category group payload must not be nil and must carry a stable ID")
	}
	if len(orderedCategories) == 0 {
		return fmt.Errorf("ordered category list must not be empty")
	}
	records := make([]*pb.PBListCategory, len(orderedCategories))
	for i, record := range orderedCategories {
		if record == nil || strings.TrimSpace(record.GetIdentifier()) == "" {
			return fmt.Errorf("ordered category at position %d must carry a stable ID", i+1)
		}
		records[i] = &pb.PBListCategory{Identifier: strings.TrimSpace(record.GetIdentifier())}
	}
	updatedGroup := proto.Clone(group).(*pb.PBListCategoryGroup)
	updatedGroup.Categories = records
	op := &pb.PBListOperation{
		Metadata: &pb.PBOperationMetadata{
			OperationId:    uuid.NewString(),
			HandlerId:      "set-sorted-category-ids",
			UserId:         c.cfg.UserID,
			OperationClass: int32(pb.PBOperationMetadata_ListCategoryGroupOperation),
		},
		ListId:               listID,
		UpdatedCategoryGroup: updatedGroup,
	}
	return c.sendListOperationsCategoryV2(ctx, &pb.PBListOperationList{Operations: []*pb.PBListOperation{op}})
}
