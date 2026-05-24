// Package anylist provides a client for AnyList's unofficial protobuf API.
// All data endpoints use application/x-www-form-urlencoded with a binary
// protobuf payload in the "operations" field. Auth endpoints return JSON.
package anylist

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

const (
	apiVersion = "3"
	baseURL    = "https://www.anylist.com"
)

type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
}

type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func New(cfg *config.Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// EnsureClientIdentifier generates and saves a client identifier if one doesn't
// exist. The identifier is a 32-char hex string sent on every request as
// X-AnyLeaf-Client-Identifier.
func EnsureClientIdentifier(cfg *config.Config) error {
	if cfg.ClientIdentifier != "" {
		return nil
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("generating client identifier: %w", err)
	}
	return cfg.SaveClientIdentifier(hex.EncodeToString(b))
}

// Login authenticates with email and password, saving tokens to config.
func (c *Client) Login(ctx context.Context, email, password string) error {
	if err := EnsureClientIdentifier(c.cfg); err != nil {
		return err
	}
	data := url.Values{}
	data.Set("email", email)
	data.Set("password", password)

	// Try the newer /auth/token endpoint first
	resp, err := c.postForm(ctx, "/auth/token", data)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		// Fall back to older /data/validate-login endpoint
		resp.Body.Close()
		resp, err = c.postForm(ctx, "/data/validate-login", data)
		if err != nil {
			return fmt.Errorf("login (fallback): %w", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var lr loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return fmt.Errorf("decoding login response: %w", err)
	}
	if lr.AccessToken == "" {
		return fmt.Errorf("login response missing access_token; check email and password")
	}
	return c.cfg.SaveAnyListCredentials(lr.AccessToken, lr.RefreshToken, lr.UserID)
}

// RefreshTokens exchanges the stored refresh token for a new access token.
func (c *Client) RefreshTokens(ctx context.Context) error {
	if c.cfg.RefreshToken == "" {
		return fmt.Errorf("no refresh token stored; run 'auth login' first")
	}
	data := url.Values{}
	data.Set("refresh_token", c.cfg.RefreshToken)

	resp, err := c.postForm(ctx, "/auth/token/refresh", data)
	if err != nil {
		return fmt.Errorf("refreshing tokens: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token refresh failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rr refreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return fmt.Errorf("decoding refresh response: %w", err)
	}
	return c.cfg.SaveAnyListCredentials(rr.AccessToken, rr.RefreshToken, c.cfg.UserID)
}

// GetUserData fetches all user data (lists, recipes, meal plan, etc.) via a
// single protobuf response from /data/user-data/get.
func (c *Client) GetUserData(ctx context.Context) (*pb.PBUserDataResponse, error) {
	resp, err := c.postRaw(ctx, "/data/user-data/get", nil)
	if err != nil {
		return nil, fmt.Errorf("fetching user data: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		// Attempt a token refresh and retry once.
		if refreshErr := c.RefreshTokens(ctx); refreshErr != nil {
			return nil, fmt.Errorf("unauthorized; token refresh failed: %w", refreshErr)
		}
		resp, err = c.postRaw(ctx, "/data/user-data/get", nil)
		if err != nil {
			return nil, fmt.Errorf("fetching user data (retry): %w", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user data fetch failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading user data response: %w", err)
	}

	m := &pb.PBUserDataResponse{}
	if err := proto.Unmarshal(body, m); err != nil {
		return nil, fmt.Errorf("decoding user data protobuf: %w", err)
	}
	return m, nil
}

// AddItem adds an item to a shopping list.
func (c *Client) AddItem(ctx context.Context, listID, itemName, quantity, details, category string) error {
	itemID := strings.ReplaceAll(uuid.NewString(), "-", "")
	op := &pb.PBListOperation{
		Metadata: &pb.PBOperationMetadata{
			OperationId: uuid.NewString(),
			HandlerId:   "add-shopping-list-item",
			UserId:      c.cfg.UserID,
		},
		ListId:     listID,
		ListItemId: itemID,
		ListItem: &pb.ListItem{
			Identifier:      itemID,
			ListId:          listID,
			Name:            itemName,
			Checked:         false,
			CategoryMatchId: "other",
			UserId:          c.cfg.UserID,
		},
	}
	if quantity != "" {
		op.ListItem.Quantity = quantity
	}
	if details != "" {
		op.ListItem.Details = details
	}
	if category != "" {
		op.ListItem.Category = category
	}
	return c.sendListOperation(ctx, op)
}

// SetItemChecked marks an item as checked or unchecked.
func (c *Client) SetItemChecked(ctx context.Context, listID, itemID string, checked bool) error {
	op := &pb.PBListOperation{
		Metadata: &pb.PBOperationMetadata{
			OperationId: uuid.NewString(),
			HandlerId:   "set-list-item-checked",
			UserId:      c.cfg.UserID,
		},
		ListId:     listID,
		ListItemId: itemID,
	}
	if checked {
		op.UpdatedValue = "y"
	} else {
		op.UpdatedValue = "n"
	}
	return c.sendListOperation(ctx, op)
}

// RemoveItem removes an item from a shopping list. AnyList expects the full
// encoded list item on this operation; sending only IDs returns success but is
// ignored by the server.
func (c *Client) RemoveItem(ctx context.Context, listID string, item *pb.ListItem) error {
	op := &pb.PBListOperation{
		Metadata: &pb.PBOperationMetadata{
			OperationId: uuid.NewString(),
			HandlerId:   "remove-shopping-list-item",
			UserId:      c.cfg.UserID,
		},
		ListId:     listID,
		ListItemId: item.GetIdentifier(),
		ListItem:   item,
	}
	return c.sendListOperation(ctx, op)
}

// UpdateItemFields updates one or more scalar fields of an existing list item.
func (c *Client) UpdateItemFields(ctx context.Context, listID, itemID string, fields map[string]string) error {
	handlerIDs := map[string]string{
		"name":              "set-list-item-name",
		"quantity":          "set-list-item-quantity",
		"details":           "set-list-item-details",
		"category_match_id": "set-list-item-category-match-id",
	}
	ops := &pb.PBListOperationList{}
	for field, value := range fields {
		handlerID, ok := handlerIDs[field]
		if !ok {
			return fmt.Errorf("unsupported item update field %q", field)
		}
		ops.Operations = append(ops.Operations, &pb.PBListOperation{
			Metadata: &pb.PBOperationMetadata{
				OperationId: uuid.NewString(),
				HandlerId:   handlerID,
				UserId:      c.cfg.UserID,
			},
			ListId:       listID,
			ListItemId:   itemID,
			UpdatedValue: value,
		})
	}
	if len(ops.Operations) == 0 {
		return nil
	}
	return c.sendListOperations(ctx, ops)
}

// UpdateItem updates fields of an existing list item from a full item value.
func (c *Client) UpdateItem(ctx context.Context, listID string, item *pb.ListItem) error {
	return c.UpdateItemFields(ctx, listID, item.GetIdentifier(), map[string]string{
		"name":              item.GetName(),
		"quantity":          item.GetQuantity(),
		"details":           item.GetDetails(),
		"category_match_id": item.GetCategoryMatchId(),
	})
}

// SaveRecipe creates or updates a recipe in the user's recipe data.
func (c *Client) SaveRecipe(ctx context.Context, recipeDataID string, recipe *pb.PBRecipe, fromWebImport bool) error {
	op := &pb.PBRecipeOperation{
		Metadata: &pb.PBOperationMetadata{
			OperationId: uuid.NewString(),
			HandlerId:   "save-recipe",
			UserId:      c.cfg.UserID,
		},
		RecipeDataId:             recipeDataID,
		Recipe:                   recipe,
		IsNewRecipeFromWebImport: fromWebImport,
	}
	return c.sendRecipeOperation(ctx, op)
}

// RemoveRecipe deletes a recipe from the user's recipe data.
func (c *Client) RemoveRecipe(ctx context.Context, recipeDataID string, recipe *pb.PBRecipe) error {
	op := &pb.PBRecipeOperation{
		Metadata: &pb.PBOperationMetadata{
			OperationId: uuid.NewString(),
			HandlerId:   "remove-recipe",
			UserId:      c.cfg.UserID,
		},
		RecipeDataId: recipeDataID,
		Recipe:       recipe,
		RecipeIds:    []string{recipe.GetIdentifier()},
	}
	return c.sendRecipeOperation(ctx, op)
}

// SaveRecipeCollection creates or updates a recipe collection.
func (c *Client) SaveRecipeCollection(ctx context.Context, recipeDataID string, collection *pb.PBRecipeCollection) error {
	op := &pb.PBRecipeOperation{
		Metadata: &pb.PBOperationMetadata{
			OperationId: uuid.NewString(),
			HandlerId:   "save-recipe-collection",
			UserId:      c.cfg.UserID,
		},
		RecipeDataId:     recipeDataID,
		RecipeCollection: collection,
	}
	return c.sendRecipeOperation(ctx, op)
}

// RemoveRecipeCollection deletes a recipe collection.
func (c *Client) RemoveRecipeCollection(ctx context.Context, recipeDataID string, collection *pb.PBRecipeCollection) error {
	op := &pb.PBRecipeOperation{
		Metadata: &pb.PBOperationMetadata{
			OperationId: uuid.NewString(),
			HandlerId:   "remove-recipe-collection",
			UserId:      c.cfg.UserID,
		},
		RecipeDataId:        recipeDataID,
		RecipeCollection:    collection,
		RecipeCollectionIds: []string{collection.GetIdentifier()},
	}
	return c.sendRecipeOperation(ctx, op)
}

func (c *Client) sendRecipeOperation(ctx context.Context, op *pb.PBRecipeOperation) error {
	req := &pb.PBRecipeOperationList{Operations: []*pb.PBRecipeOperation{op}}
	dat, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling recipe operations: %w", err)
	}
	data := url.Values{}
	data.Set("operations", string(dat))

	resp, err := c.postFormAuthed(ctx, "/data/user-recipe-data/update", data)
	if err != nil {
		return fmt.Errorf("sending recipe operation: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("recipe operation failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// SaveListFolder creates or updates a list folder.
func (c *Client) SaveListFolder(ctx context.Context, listDataID string, folder *pb.PBListFolder, parentFolderID string) error {
	op := &pb.PBListFolderOperation{
		Metadata: &pb.PBOperationMetadata{
			OperationId: uuid.NewString(),
			HandlerId:   "save-list-folder",
			UserId:      c.cfg.UserID,
		},
		ListDataId:            listDataID,
		ListFolder:            folder,
		UpdatedParentFolderId: parentFolderID,
	}
	return c.sendListFolderOperation(ctx, op)
}

// RemoveListFolder deletes a list folder.
func (c *Client) RemoveListFolder(ctx context.Context, listDataID string, folder *pb.PBListFolder) error {
	op := &pb.PBListFolderOperation{
		Metadata: &pb.PBOperationMetadata{
			OperationId: uuid.NewString(),
			HandlerId:   "remove-list-folder",
			UserId:      c.cfg.UserID,
		},
		ListDataId: listDataID,
		ListFolder: folder,
	}
	return c.sendListFolderOperation(ctx, op)
}

func (c *Client) sendListFolderOperation(ctx context.Context, op *pb.PBListFolderOperation) error {
	req := &pb.PBListFolderOperationList{Operations: []*pb.PBListFolderOperation{op}}
	dat, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling list folder operations: %w", err)
	}
	data := url.Values{}
	data.Set("operations", string(dat))

	resp, err := c.postFormAuthed(ctx, "/data/list-folders/update", data)
	if err != nil {
		return fmt.Errorf("sending list folder operation: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("list folder operation failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// sendListOperation sends a single list operation as a protobuf form POST.
func (c *Client) sendListOperation(ctx context.Context, op *pb.PBListOperation) error {
	req := &pb.PBListOperationList{Operations: []*pb.PBListOperation{op}}
	return c.sendListOperations(ctx, req)
}

// sendListOperations sends a batch of list operations.
func (c *Client) sendListOperations(ctx context.Context, ops *pb.PBListOperationList) error {
	dat, err := proto.Marshal(ops)
	if err != nil {
		return fmt.Errorf("marshaling list operations: %w", err)
	}
	data := url.Values{}
	data.Set("operations", string(dat))

	resp, err := c.postFormAuthed(ctx, "/data/shopping-lists/update", data)
	if err != nil {
		return fmt.Errorf("sending list operations: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("list operation failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// postForm sends an unauthenticated form POST (used for login/refresh).
func (c *Client) postForm(ctx context.Context, path string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+path, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-AnyLeaf-API-Version", apiVersion)
	if c.cfg.ClientIdentifier != "" {
		req.Header.Set("X-AnyLeaf-Client-Identifier", c.cfg.ClientIdentifier)
	}
	return c.httpClient.Do(req)
}

// postFormAuthed sends an authenticated form POST.
func (c *Client) postFormAuthed(ctx context.Context, path string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+path, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-AnyLeaf-API-Version", apiVersion)
	if c.cfg.ClientIdentifier != "" {
		req.Header.Set("X-AnyLeaf-Client-Identifier", c.cfg.ClientIdentifier)
	}
	if c.cfg.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)
	}
	return c.httpClient.Do(req)
}

// postRaw sends an authenticated POST with no form body (for user-data/get).
func (c *Client) postRaw(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-AnyLeaf-API-Version", apiVersion)
	if c.cfg.ClientIdentifier != "" {
		req.Header.Set("X-AnyLeaf-Client-Identifier", c.cfg.ClientIdentifier)
	}
	if c.cfg.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)
	}
	return c.httpClient.Do(req)
}
