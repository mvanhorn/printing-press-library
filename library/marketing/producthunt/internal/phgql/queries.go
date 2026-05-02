// Package phgql carries the GraphQL operation strings used to talk to the
// Product Hunt API. Strings are kept verbatim from the operations the
// jaipandya/producthunt-mcp-server confirmed work in production; we extend
// them with a few shapes our own commands need (slim posts query for sync,
// snapshot fields, etc.).
package phgql

const PostQuery = `
query Post($id: ID, $slug: String) {
  post(id: $id, slug: $slug) {
    id
    name
    slug
    tagline
    description
    url
    votesCount
    commentsCount
    createdAt
    featuredAt
    website
    thumbnail { url videoUrl }
    user { id name username headline profileImage url twitterUsername }
    topics { edges { node { id name slug } } }
    media { url videoUrl type }
    makers { id name username profileImage url }
  }
}
`

const PostsQuery = `
query Posts($first: Int, $after: String, $order: PostsOrder, $topic: String, $featured: Boolean, $url: String, $twitterUrl: String, $postedBefore: DateTime, $postedAfter: DateTime) {
  posts(first: $first, after: $after, order: $order, topic: $topic, featured: $featured, url: $url, twitterUrl: $twitterUrl, postedBefore: $postedBefore, postedAfter: $postedAfter) {
    edges {
      node {
        id
        name
        slug
        tagline
        description
        url
        votesCount
        commentsCount
        createdAt
        featuredAt
        website
        thumbnail { url videoUrl }
        user { id name username headline profileImage url twitterUsername }
        topics { edges { node { id name slug } } }
        makers { id name username profileImage url }
      }
    }
    pageInfo { endCursor hasNextPage }
  }
}
`

const PostsSlimQuery = `
query PostsSlim($first: Int, $after: String, $order: PostsOrder, $topic: String, $postedBefore: DateTime, $postedAfter: DateTime) {
  posts(first: $first, after: $after, order: $order, topic: $topic, postedBefore: $postedBefore, postedAfter: $postedAfter) {
    edges {
      node {
        id
        name
        slug
        tagline
        votesCount
        commentsCount
        createdAt
        topics { edges { node { id name slug } } }
      }
    }
    pageInfo { endCursor hasNextPage }
  }
}
`

const CommentQuery = `
query Comment($id: ID!) {
  comment(id: $id) {
    id
    body
    createdAt
    votesCount
    user { id name username headline profileImage }
  }
}
`

const PostCommentsQuery = `
query PostComments($id: ID, $slug: String, $first: Int, $after: String, $order: CommentsOrder) {
  post(id: $id, slug: $slug) {
    id
    name
    comments(first: $first, after: $after, order: $order) {
      edges {
        node {
          id
          body
          createdAt
          votesCount
          user { id name username headline profileImage }
        }
      }
      pageInfo { endCursor hasNextPage }
    }
  }
}
`

const CollectionQuery = `
query Collection($id: ID, $slug: String) {
  collection(id: $id, slug: $slug) {
    id
    name
    description
    tagline
    followersCount
    user { id name username headline profileImage }
    posts {
      edges { node { id name slug tagline votesCount commentsCount } }
    }
  }
}
`

const CollectionsQuery = `
query Collections($first: Int, $after: String, $order: CollectionsOrder, $featured: Boolean, $userId: ID, $postId: ID) {
  collections(first: $first, after: $after, order: $order, featured: $featured, userId: $userId, postId: $postId) {
    edges {
      node {
        id
        name
        description
        tagline
        followersCount
        user { id name username headline profileImage }
      }
    }
    pageInfo { endCursor hasNextPage }
  }
}
`

const TopicQuery = `
query Topic($id: ID, $slug: String) {
  topic(id: $id, slug: $slug) {
    id
    name
    slug
    description
    followersCount
    postsCount
    image
  }
}
`

const TopicsQuery = `
query Topics($first: Int, $after: String, $order: TopicsOrder, $query: String, $followedByUserid: ID) {
  topics(first: $first, after: $after, order: $order, query: $query, followedByUserid: $followedByUserid) {
    edges {
      node { id name slug description followersCount postsCount image }
    }
    pageInfo { endCursor hasNextPage }
  }
}
`

const UserQuery = `
query User($id: ID, $username: String) {
  user(id: $id, username: $username) {
    id
    name
    username
    headline
    createdAt
    twitterUsername
    websiteUrl
    profileImage
    coverImage
    isMaker
    isFollowing
    url
  }
}
`

const UserPostsQuery = `
query UserPosts($id: ID, $username: String, $first: Int, $after: String) {
  user(id: $id, username: $username) {
    id
    madePosts(first: $first, after: $after) {
      edges {
        node {
          id name slug tagline votesCount commentsCount createdAt featuredAt
          thumbnail { url }
        }
      }
      pageInfo { endCursor hasNextPage }
    }
  }
}
`

const UserVotedPostsQuery = `
query UserVotedPosts($id: ID, $username: String, $first: Int, $after: String) {
  user(id: $id, username: $username) {
    id
    votedPosts(first: $first, after: $after) {
      edges {
        node {
          id name slug tagline votesCount commentsCount createdAt featuredAt
          thumbnail { url }
        }
      }
      pageInfo { endCursor hasNextPage }
    }
  }
}
`

const ViewerQuery = `
query {
  viewer {
    user {
      id
      name
      username
      headline
      coverImage
      createdAt
      isFollowing
      isMaker
      isViewer
      madePosts { totalCount }
      profileImage
      twitterUsername
      url
      websiteUrl
      votedPosts { totalCount }
    }
  }
}
`
