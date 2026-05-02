package phgql

import "time"

// User mirrors the GraphQL User type. Many fields come back as the literal
// string "[REDACTED]" for non-self lookups — see Redacted().
type User struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Username        string `json:"username"`
	Headline        string `json:"headline"`
	ProfileImage    string `json:"profileImage"`
	URL             string `json:"url"`
	TwitterUsername string `json:"twitterUsername"`
}

// Redacted reports whether the user record was redacted by Product Hunt's
// global policy. PH returns id "0" plus literal "[REDACTED]" strings; we use
// that triple as the canonical signal.
func (u User) Redacted() bool {
	return u.ID == "0" && (u.Username == "[REDACTED]" || u.Name == "[REDACTED]")
}

type Topic struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Description    string `json:"description"`
	FollowersCount int    `json:"followersCount"`
	PostsCount     int    `json:"postsCount"`
	Image          string `json:"image"`
}

type TopicEdge struct {
	Node Topic `json:"node"`
}

type TopicConnection struct {
	Edges    []TopicEdge `json:"edges"`
	PageInfo PageInfo    `json:"pageInfo"`
}

type Media struct {
	URL      string `json:"url"`
	VideoURL string `json:"videoUrl"`
	Type     string `json:"type"`
}

type Thumbnail struct {
	URL      string `json:"url"`
	VideoURL string `json:"videoUrl"`
}

type Post struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	Tagline       string    `json:"tagline"`
	Description   string    `json:"description"`
	URL           string    `json:"url"`
	VotesCount    int       `json:"votesCount"`
	CommentsCount int       `json:"commentsCount"`
	CreatedAt     time.Time `json:"createdAt"`
	FeaturedAt    time.Time `json:"featuredAt"`
	Website       string    `json:"website"`
	Thumbnail     Thumbnail `json:"thumbnail"`
	User          User      `json:"user"`
	Topics        struct {
		Edges []TopicEdge `json:"edges"`
	} `json:"topics"`
	Media  []Media `json:"media"`
	Makers []User  `json:"makers"`
}

type PostEdge struct {
	Node Post `json:"node"`
}

type PageInfo struct {
	EndCursor   string `json:"endCursor"`
	HasNextPage bool   `json:"hasNextPage"`
}

type PostConnection struct {
	Edges    []PostEdge `json:"edges"`
	PageInfo PageInfo   `json:"pageInfo"`
}

type Comment struct {
	ID         string    `json:"id"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
	VotesCount int       `json:"votesCount"`
	User       User      `json:"user"`
}

type CommentEdge struct {
	Node Comment `json:"node"`
}

type CommentConnection struct {
	Edges    []CommentEdge `json:"edges"`
	PageInfo PageInfo      `json:"pageInfo"`
}

type Collection struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Tagline        string `json:"tagline"`
	FollowersCount int    `json:"followersCount"`
	User           User   `json:"user"`
	Posts          struct {
		Edges []PostEdge `json:"edges"`
	} `json:"posts"`
}

type CollectionEdge struct {
	Node Collection `json:"node"`
}

type CollectionConnection struct {
	Edges    []CollectionEdge `json:"edges"`
	PageInfo PageInfo         `json:"pageInfo"`
}

type ViewerUser struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Username        string    `json:"username"`
	Headline        string    `json:"headline"`
	CoverImage      string    `json:"coverImage"`
	CreatedAt       time.Time `json:"createdAt"`
	IsFollowing     bool      `json:"isFollowing"`
	IsMaker         bool      `json:"isMaker"`
	IsViewer        bool      `json:"isViewer"`
	ProfileImage    string    `json:"profileImage"`
	TwitterUsername string    `json:"twitterUsername"`
	URL             string    `json:"url"`
	WebsiteURL      string    `json:"websiteUrl"`
	MadePosts       struct {
		TotalCount int `json:"totalCount"`
	} `json:"madePosts"`
	VotedPosts struct {
		TotalCount int `json:"totalCount"`
	} `json:"votedPosts"`
}

// PostsResponse / PostResponse / etc. wrap the data field for typed Query calls.
type PostsResponse struct {
	Posts PostConnection `json:"posts"`
}
type PostResponse struct {
	Post Post `json:"post"`
}
type PostCommentsResponse struct {
	Post struct {
		ID       string            `json:"id"`
		Name     string            `json:"name"`
		Comments CommentConnection `json:"comments"`
	} `json:"post"`
}
type CommentResponse struct {
	Comment Comment `json:"comment"`
}
type CollectionResponse struct {
	Collection Collection `json:"collection"`
}
type CollectionsResponse struct {
	Collections CollectionConnection `json:"collections"`
}
type TopicResponse struct {
	Topic Topic `json:"topic"`
}
type TopicsResponse struct {
	Topics TopicConnection `json:"topics"`
}
type UserResponse struct {
	User User `json:"user"`
}
type UserPostsResponse struct {
	User struct {
		ID        string         `json:"id"`
		MadePosts PostConnection `json:"madePosts"`
	} `json:"user"`
}
type UserVotedPostsResponse struct {
	User struct {
		ID         string         `json:"id"`
		VotedPosts PostConnection `json:"votedPosts"`
	} `json:"user"`
}
type ViewerResponse struct {
	Viewer struct {
		User *ViewerUser `json:"user"`
	} `json:"viewer"`
}
