package phgraphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/product-hunt/internal/cliutil"
)

const endpoint = "https://api.producthunt.com/v2/api/graphql"

// Client performs authenticated GraphQL calls against the Product Hunt API v2.
type Client struct {
	token   string
	http    *http.Client
	dryRun  bool
	limiter *cliutil.AdaptiveLimiter
}

// New returns a Client that authenticates every request with token.
// ratePerSec controls pacing; pass 0 to disable rate limiting.
func New(token string, timeout time.Duration, ratePerSec float64) *Client {
	return &Client{
		token:   token,
		http:    &http.Client{Timeout: timeout},
		limiter: cliutil.NewAdaptiveLimiter(ratePerSec),
	}
}

// NewDryRun returns a Client that prints the request and short-circuits without sending.
func NewDryRun(token string) *Client {
	return &Client{token: token, dryRun: true, http: &http.Client{Timeout: 10 * time.Second}}
}

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// Do executes a GraphQL query and returns the data object.
func (c *Client) Do(ctx context.Context, query string, vars map[string]any) (json.RawMessage, error) {
	req := gqlRequest{Query: query, Variables: vars}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling graphql request: %w", err)
	}

	if c.dryRun {
		fmt.Printf("[dry-run] POST %s\n%s\n", endpoint, string(body))
		return json.RawMessage(`{}`), nil
	}

	c.limiter.Wait()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("executing graphql request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("authentication failed (HTTP 401): set PRODUCT_HUNT_TOKEN or run 'product-hunt-pp-cli auth set-token <token>'")
	}
	if resp.StatusCode == 429 {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{
			URL:        endpoint,
			RetryAfter: cliutil.RetryAfter(resp),
			Body:       string(respBody),
		}
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	c.limiter.OnSuccess()

	var gqlResp gqlResponse
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return nil, fmt.Errorf("parsing graphql response: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		msgs := make([]string, len(gqlResp.Errors))
		for i, e := range gqlResp.Errors {
			msgs[i] = e.Message
		}
		return nil, fmt.Errorf("graphql errors: %v", msgs)
	}
	return gqlResp.Data, nil
}

// ---- Posts ----

const postsQuery = `query Posts($first: Int, $after: String, $topic: String, $order: PostsOrder, $featured: Boolean, $postedAfter: DateTime, $postedBefore: DateTime) {
  posts(first: $first, after: $after, topic: $topic, order: $order, featured: $featured, postedAfter: $postedAfter, postedBefore: $postedBefore) {
    edges {
      node {
        id name slug tagline description votesCount commentsCount createdAt featuredAt url
        makers { id name username headline profileImage }
        topics { edges { node { id name slug } } }
        thumbnail { url }
      }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// GetPosts fetches a page of posts with optional filters.
func (c *Client) GetPosts(ctx context.Context, first int, after, topic, order string, featured bool, postedAfter, postedBefore string) (*PostConnection, error) {
	vars := map[string]any{"first": first}
	if after != "" {
		vars["after"] = after
	}
	if topic != "" {
		vars["topic"] = topic
	}
	if order != "" {
		vars["order"] = order
	}
	if featured {
		vars["featured"] = true
	}
	if postedAfter != "" {
		vars["postedAfter"] = postedAfter
	}
	if postedBefore != "" {
		vars["postedBefore"] = postedBefore
	}

	data, err := c.Do(ctx, postsQuery, vars)
	if err != nil {
		return nil, err
	}
	var result struct {
		Posts PostConnection `json:"posts"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing posts response: %w", err)
	}
	return &result.Posts, nil
}

const postQuery = `query Post($id: ID, $slug: String) {
  post(id: $id, slug: $slug) {
    id name slug tagline description votesCount commentsCount createdAt featuredAt url website reviewsRating
    makers { id name username headline profileImage }
    topics { edges { node { id name slug } } }
    thumbnail { url }
  }
}`

// GetPost fetches a single post by ID or slug.
func (c *Client) GetPost(ctx context.Context, idOrSlug string) (*Post, error) {
	vars := map[string]any{}
	// Numeric IDs are passed as id; slugs as slug
	if isNumericID(idOrSlug) {
		vars["id"] = idOrSlug
	} else {
		vars["slug"] = idOrSlug
	}

	data, err := c.Do(ctx, postQuery, vars)
	if err != nil {
		return nil, err
	}
	var result struct {
		Post *Post `json:"post"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing post response: %w", err)
	}
	if result.Post == nil {
		return nil, fmt.Errorf("post %q not found", idOrSlug)
	}
	return result.Post, nil
}

const postCommentsQuery = `query PostComments($id: ID, $slug: String, $first: Int, $after: String, $order: CommentsOrder) {
  post(id: $id, slug: $slug) {
    comments(first: $first, after: $after, order: $order) {
      edges {
        node {
          id body votesCount createdAt
          user { id name username profileImage }
          replies { edges { node { id body votesCount createdAt user { id name username } } } }
        }
      }
      pageInfo { hasNextPage endCursor }
    }
  }
}`

// GetPostComments fetches comments for a post.
func (c *Client) GetPostComments(ctx context.Context, idOrSlug string, first int, after, order string) (*CommentConnection, error) {
	vars := map[string]any{"first": first}
	if isNumericID(idOrSlug) {
		vars["id"] = idOrSlug
	} else {
		vars["slug"] = idOrSlug
	}
	if after != "" {
		vars["after"] = after
	}
	if order != "" {
		vars["order"] = order
	}

	data, err := c.Do(ctx, postCommentsQuery, vars)
	if err != nil {
		return nil, err
	}
	var result struct {
		Post struct {
			Comments CommentConnection `json:"comments"`
		} `json:"post"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing comments response: %w", err)
	}
	return &result.Post.Comments, nil
}

// ---- Topics ----

const topicsQuery = `query Topics($first: Int, $after: String, $query: String, $order: TopicsOrder) {
  topics(first: $first, after: $after, query: $query, order: $order) {
    edges {
      node { id name slug description followersCount postsCount }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// GetTopics fetches topics with optional search query.
func (c *Client) GetTopics(ctx context.Context, first int, after, query, order string) (*TopicConnection, error) {
	vars := map[string]any{"first": first}
	if after != "" {
		vars["after"] = after
	}
	if query != "" {
		vars["query"] = query
	}
	if order != "" {
		vars["order"] = order
	}

	data, err := c.Do(ctx, topicsQuery, vars)
	if err != nil {
		return nil, err
	}
	var result struct {
		Topics TopicConnection `json:"topics"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing topics response: %w", err)
	}
	return &result.Topics, nil
}

const topicQuery = `query Topic($id: ID, $slug: String) {
  topic(id: $id, slug: $slug) {
    id name slug description followersCount postsCount isFollowing
  }
}`

// GetTopic fetches a single topic by ID or slug.
func (c *Client) GetTopic(ctx context.Context, idOrSlug string) (*Topic, error) {
	vars := map[string]any{}
	if isNumericID(idOrSlug) {
		vars["id"] = idOrSlug
	} else {
		vars["slug"] = idOrSlug
	}

	data, err := c.Do(ctx, topicQuery, vars)
	if err != nil {
		return nil, err
	}
	var result struct {
		Topic *Topic `json:"topic"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing topic response: %w", err)
	}
	if result.Topic == nil {
		return nil, fmt.Errorf("topic %q not found", idOrSlug)
	}
	return result.Topic, nil
}

// ---- Users ----

const userQuery = `query User($id: ID, $username: String, $postsFirst: Int) {
  user(id: $id, username: $username) {
    id name username headline profileImage websiteUrl followersCount followingCount isFollowing isMaker createdAt
    posts(first: $postsFirst, order: NEWEST) {
      edges { node { id name slug tagline votesCount commentsCount createdAt featuredAt } }
    }
  }
}`

// GetUser fetches a user by username or numeric ID.
func (c *Client) GetUser(ctx context.Context, usernameOrID string, postsLimit int) (*User, []PostSummary, error) {
	if postsLimit <= 0 {
		postsLimit = 10
	}
	vars := map[string]any{"postsFirst": postsLimit}
	if isNumericID(usernameOrID) {
		vars["id"] = usernameOrID
	} else {
		vars["username"] = usernameOrID
	}

	data, err := c.Do(ctx, userQuery, vars)
	if err != nil {
		return nil, nil, err
	}

	var result struct {
		User *struct {
			User
			Posts struct {
				Edges []struct {
					Node PostSummary `json:"node"`
				} `json:"edges"`
			} `json:"posts"`
		} `json:"user"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, nil, fmt.Errorf("parsing user response: %w", err)
	}
	if result.User == nil {
		return nil, nil, fmt.Errorf("user %q not found", usernameOrID)
	}
	posts := make([]PostSummary, len(result.User.Posts.Edges))
	for i, e := range result.User.Posts.Edges {
		posts[i] = e.Node
	}
	return &result.User.User, posts, nil
}

const viewerQuery = `query Viewer {
  viewer {
    id name username headline profileImage websiteUrl followersCount followingCount createdAt
  }
}`

// GetViewer fetches the authenticated user.
func (c *Client) GetViewer(ctx context.Context) (*User, error) {
	data, err := c.Do(ctx, viewerQuery, nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Viewer *User `json:"viewer"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing viewer response: %w", err)
	}
	return result.Viewer, nil
}

// ---- Collections ----

const collectionsQuery = `query Collections($first: Int, $after: String, $featured: Boolean, $order: CollectionsOrder) {
  collections(first: $first, after: $after, featured: $featured, order: $order) {
    edges {
      node { id name slug tagline description followersCount createdAt featuredAt }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// GetCollections fetches collections.
func (c *Client) GetCollections(ctx context.Context, first int, after string, featured bool, order string) (*CollectionConnection, error) {
	vars := map[string]any{"first": first}
	if after != "" {
		vars["after"] = after
	}
	if featured {
		vars["featured"] = true
	}
	if order != "" {
		vars["order"] = order
	}

	data, err := c.Do(ctx, collectionsQuery, vars)
	if err != nil {
		return nil, err
	}
	var result struct {
		Collections CollectionConnection `json:"collections"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing collections response: %w", err)
	}
	return &result.Collections, nil
}

const collectionQuery = `query Collection($id: ID, $slug: String) {
  collection(id: $id, slug: $slug) {
    id name slug tagline description followersCount createdAt featuredAt
    coverImage { url }
    user { id name username }
    posts(first: 20, order: RANKING) {
      edges { node { id name slug tagline votesCount commentsCount createdAt featuredAt } }
    }
  }
}`

// GetCollection fetches a single collection by ID or slug.
func (c *Client) GetCollection(ctx context.Context, idOrSlug string) (*Collection, error) {
	vars := map[string]any{}
	if isNumericID(idOrSlug) {
		vars["id"] = idOrSlug
	} else {
		vars["slug"] = idOrSlug
	}

	data, err := c.Do(ctx, collectionQuery, vars)
	if err != nil {
		return nil, err
	}

	var result struct {
		Collection *struct {
			Collection
			Posts struct {
				Edges []struct {
					Node PostSummary `json:"node"`
				} `json:"edges"`
			} `json:"posts"`
		} `json:"collection"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing collection response: %w", err)
	}
	if result.Collection == nil {
		return nil, fmt.Errorf("collection %q not found", idOrSlug)
	}
	coll := result.Collection.Collection
	for _, e := range result.Collection.Posts.Edges {
		coll.Posts = append(coll.Posts, Post{
			ID: e.Node.ID, Name: e.Node.Name, Slug: e.Node.Slug,
			Tagline: e.Node.Tagline, VotesCount: e.Node.VotesCount,
			CommentsCount: e.Node.CommentsCount, CreatedAt: e.Node.CreatedAt,
		})
	}
	return &coll, nil
}

// ---- Comments ----

const commentQuery = `query Comment($id: ID!) {
  comment(id: $id) {
    id body votesCount createdAt
    user { id name username profileImage }
    replies { edges { node { id body votesCount createdAt user { id name username } } } }
  }
}`

// GetComment fetches a single comment by ID.
func (c *Client) GetComment(ctx context.Context, id string) (*Comment, error) {
	data, err := c.Do(ctx, commentQuery, map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	var result struct {
		Comment *Comment `json:"comment"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing comment response: %w", err)
	}
	if result.Comment == nil {
		return nil, fmt.Errorf("comment %q not found", id)
	}
	return result.Comment, nil
}

// ---- Mutations ----

const userFollowMutation = `mutation UserFollow($userId: ID!) {
  userFollow(input: { userId: $userId }) {
    user { id name username isFollowing }
  }
}`

const userFollowUndoMutation = `mutation UserFollowUndo($userId: ID!) {
  userFollowUndo(input: { userId: $userId }) {
    user { id name username isFollowing }
  }
}`

// FollowUser follows a user by their numeric ID.
func (c *Client) FollowUser(ctx context.Context, userID string) (*User, error) {
	data, err := c.Do(ctx, userFollowMutation, map[string]any{"userId": userID})
	if err != nil {
		return nil, err
	}
	var result struct {
		UserFollow struct {
			User *User `json:"user"`
		} `json:"userFollow"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing userFollow response: %w", err)
	}
	return result.UserFollow.User, nil
}

// UnfollowUser unfollows a user by their numeric ID.
func (c *Client) UnfollowUser(ctx context.Context, userID string) (*User, error) {
	data, err := c.Do(ctx, userFollowUndoMutation, map[string]any{"userId": userID})
	if err != nil {
		return nil, err
	}
	var result struct {
		UserFollowUndo struct {
			User *User `json:"user"`
		} `json:"userFollowUndo"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing userFollowUndo response: %w", err)
	}
	return result.UserFollowUndo.User, nil
}

// isNumericID returns true if s looks like a numeric ID rather than a slug.
func isNumericID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
