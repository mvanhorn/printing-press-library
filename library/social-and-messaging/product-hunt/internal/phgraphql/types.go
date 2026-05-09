// Package phgraphql provides typed access to the Product Hunt GraphQL API v2.
// All operations POST to https://api.producthunt.com/v2/api/graphql.
package phgraphql

// Post represents a Product Hunt launch/post.
type Post struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Slug          string  `json:"slug"`
	Tagline       string  `json:"tagline"`
	Description   string  `json:"description,omitempty"`
	VotesCount    int     `json:"votesCount"`
	CommentsCount int     `json:"commentsCount"`
	CreatedAt     string  `json:"createdAt"`
	FeaturedAt    string  `json:"featuredAt,omitempty"`
	URL           string  `json:"url"`
	Website       string  `json:"website,omitempty"`
	ReviewsRating float64 `json:"reviewsRating,omitempty"`
	Makers        []User  `json:"makers,omitempty"`
	Topics        struct {
		Edges []struct {
			Node Topic `json:"node"`
		} `json:"edges"`
	} `json:"topics,omitempty"`
	Thumbnail *Media `json:"thumbnail,omitempty"`
}

// PostSummary is a compact view for list operations.
type PostSummary struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Tagline       string `json:"tagline"`
	VotesCount    int    `json:"votesCount"`
	CommentsCount int    `json:"commentsCount"`
	CreatedAt     string `json:"createdAt"`
	FeaturedAt    string `json:"featuredAt,omitempty"`
	URL           string `json:"url"`
}

// User represents a Product Hunt user/maker.
type User struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Username       string `json:"username"`
	Headline       string `json:"headline,omitempty"`
	ProfileImage   string `json:"profileImage,omitempty"`
	WebsiteURL     string `json:"websiteUrl,omitempty"`
	FollowersCount int    `json:"followersCount,omitempty"`
	FollowingCount int    `json:"followingCount,omitempty"`
	IsFollowing    bool   `json:"isFollowing,omitempty"`
	IsMaker        bool   `json:"isMaker,omitempty"`
	CreatedAt      string `json:"createdAt,omitempty"`
}

// Topic represents a Product Hunt topic category.
type Topic struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Description    string `json:"description,omitempty"`
	FollowersCount int    `json:"followersCount,omitempty"`
	PostsCount     int    `json:"postsCount,omitempty"`
	IsFollowing    bool   `json:"isFollowing,omitempty"`
}

// Collection represents a Product Hunt curated collection.
type Collection struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Slug           string  `json:"slug"`
	Tagline        string  `json:"tagline,omitempty"`
	Description    string  `json:"description,omitempty"`
	CoverImage     *Media  `json:"coverImage,omitempty"`
	FollowersCount int     `json:"followersCount,omitempty"`
	CreatedAt      string  `json:"createdAt,omitempty"`
	FeaturedAt     string  `json:"featuredAt,omitempty"`
	User           *User   `json:"user,omitempty"`
	Posts          []Post  `json:"posts,omitempty"`
}

// Comment represents a Product Hunt comment.
type Comment struct {
	ID         string    `json:"id"`
	Body       string    `json:"body"`
	VotesCount int       `json:"votesCount"`
	CreatedAt  string    `json:"createdAt"`
	User       User      `json:"user"`
	Replies    []Comment `json:"replies,omitempty"`
	ParentID   string    `json:"parentId,omitempty"`
}

// Media represents an image/video resource.
type Media struct {
	URL      string `json:"url"`
	VideoURL string `json:"videoUrl,omitempty"`
}

// PageInfo holds cursor-based pagination information.
type PageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

// PostConnection wraps a list of posts with pagination.
type PostConnection struct {
	Edges    []PostEdge `json:"edges"`
	PageInfo PageInfo   `json:"pageInfo"`
}

// PostEdge wraps a post with its cursor.
type PostEdge struct {
	Node   Post   `json:"node"`
	Cursor string `json:"cursor"`
}

// TopicConnection wraps a list of topics with pagination.
type TopicConnection struct {
	Edges    []TopicEdge `json:"edges"`
	PageInfo PageInfo    `json:"pageInfo"`
}

// TopicEdge wraps a topic with its cursor.
type TopicEdge struct {
	Node   Topic  `json:"node"`
	Cursor string `json:"cursor"`
}

// CollectionConnection wraps a list of collections with pagination.
type CollectionConnection struct {
	Edges    []CollectionEdge `json:"edges"`
	PageInfo PageInfo         `json:"pageInfo"`
}

// CollectionEdge wraps a collection with its cursor.
type CollectionEdge struct {
	Node   Collection `json:"node"`
	Cursor string     `json:"cursor"`
}

// CommentConnection wraps a list of comments with pagination.
type CommentConnection struct {
	Edges    []CommentEdge `json:"edges"`
	PageInfo PageInfo      `json:"pageInfo"`
}

// CommentEdge wraps a comment with its cursor.
type CommentEdge struct {
	Node   Comment `json:"node"`
	Cursor string  `json:"cursor"`
}
