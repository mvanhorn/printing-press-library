package engagement

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/commentapi"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/naverurl"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/postparse"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/reactionapi"
)

const (
	sourceReactionAPI  = "reaction-api"
	sourceCbox         = "cbox"
	sourcePostViewHTML = "post-view-html"
)

// Snapshot is the per-post engagement view. Fields default to their
// zero values when individual sources fail: Likes==nil means the
// reaction API didn't return a value, and Comments is 0 when neither
// cbox nor the PostView fallback returned a count. Callers can inspect
// Errors to distinguish "zero engagement" from "source failed".
type Snapshot struct {
	Likes          *int
	Comments       int
	PublishedAtUTC time.Time
	PublishDateStr string

	LikesSource    string
	CommentsSource string
	DateSource     string

	Errors []error
}

// BatchKey identifies a single Naver Blog post for batch engagement lookup.
type BatchKey struct {
	BlogID string
	LogNo  string
}

// Fetch returns the snapshot for a single post. Reaction, cbox, and
// PostView sources are hit in parallel; source failures are non-fatal
// and are reported in Snapshot.Errors.
func Fetch(ctx context.Context, c *client.Client, blogID, logNo string) Snapshot {
	var snap Snapshot
	if c == nil || c.HTTPClient == nil {
		snap.Errors = append(snap.Errors, fmt.Errorf("client: nil client"))
		return snap
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		likes, err := reactionapi.GetReactionsLimited(ctx, c.HTTPClient, c.Limiter(), []reactionapi.PostKey{{BlogID: blogID, LogNo: logNo}})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			snap.Errors = append(snap.Errors, wrapSource(sourceReactionAPI, err))
			return
		}
		if v, ok := likes[blogID+"_"+logNo]; ok {
			vv := v
			snap.Likes = &vv
			snap.LikesSource = sourceReactionAPI
		}
	}()

	go func() {
		defer wg.Done()
		_, total, err := commentapi.GetComments(ctx, c.HTTPClient, blogID, logNo, commentapi.GetOptions{
			PageSize: 1,
			All:      false,
			Limiter:  c.Limiter(),
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			snap.Errors = append(snap.Errors, wrapSource(sourceCbox, err))
			return
		}
		snap.Comments = total
		snap.CommentsSource = sourceCbox
	}()

	go func() {
		defer wg.Done()
		meta, err := fetchPostView(ctx, c, blogID, logNo)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			snap.Errors = append(snap.Errors, wrapSource(sourcePostViewHTML, err))
			return
		}
		applyPostView(&snap, meta)
	}()

	wg.Wait()
	return snap
}

// FetchBatch returns snapshots for keys in input order. Reactions use
// the existing 20-key API batching path; cbox and PostView are fanned
// out at the requested concurrency.
func FetchBatch(ctx context.Context, c *client.Client, keys []BatchKey, concurrency int) []Snapshot {
	snaps := make([]Snapshot, len(keys))
	if len(keys) == 0 {
		return snaps
	}
	if concurrency <= 0 {
		concurrency = 5
	}
	if c == nil || c.HTTPClient == nil {
		err := fmt.Errorf("client: nil client")
		for i := range snaps {
			snaps[i].Errors = append(snaps[i].Errors, err)
		}
		return snaps
	}

	reactionDone := make(chan map[string]int, 1)
	reactionErr := make(chan error, 1)
	go func() {
		reactionKeys := make([]reactionapi.PostKey, 0, len(keys))
		for _, k := range keys {
			reactionKeys = append(reactionKeys, reactionapi.PostKey{BlogID: k.BlogID, LogNo: k.LogNo})
		}
		likes, err := reactionapi.GetReactionsLimited(ctx, c.HTTPClient, c.Limiter(), reactionKeys)
		if err != nil {
			reactionErr <- wrapSource(sourceReactionAPI, err)
			return
		}
		reactionDone <- likes
	}()

	type job struct {
		idx int
		key BatchKey
	}
	jobs := make(chan job)
	var wg sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				fillCommentsAndDate(ctx, c, j.key, &snaps[j.idx])
			}
		}()
	}
	for i, key := range keys {
		select {
		case <-ctx.Done():
			snaps[i].Errors = append(snaps[i].Errors, ctx.Err())
		case jobs <- job{idx: i, key: key}:
		}
	}
	close(jobs)
	wg.Wait()

	select {
	case likes := <-reactionDone:
		for i, key := range keys {
			if v, ok := likes[key.BlogID+"_"+key.LogNo]; ok {
				vv := v
				snaps[i].Likes = &vv
				snaps[i].LikesSource = sourceReactionAPI
			}
		}
	case err := <-reactionErr:
		for i := range snaps {
			snaps[i].Errors = append(snaps[i].Errors, err)
		}
	}
	return snaps
}

func fillCommentsAndDate(ctx context.Context, c *client.Client, key BatchKey, snap *Snapshot) {
	_, total, err := commentapi.GetComments(ctx, c.HTTPClient, key.BlogID, key.LogNo, commentapi.GetOptions{
		PageSize: 1,
		All:      false,
		Limiter:  c.Limiter(),
	})
	if err != nil {
		snap.Errors = append(snap.Errors, wrapSource(sourceCbox, err))
	} else {
		snap.Comments = total
		snap.CommentsSource = sourceCbox
	}

	meta, err := fetchPostView(ctx, c, key.BlogID, key.LogNo)
	if err != nil {
		snap.Errors = append(snap.Errors, wrapSource(sourcePostViewHTML, err))
		return
	}
	applyPostView(snap, meta)
}

// desktopUserAgent forces Naver to serve the static desktop PostView
// HTML. The client's default User-Agent is a mobile Safari string,
// which causes Naver to JS-redirect the desktop PostView URL to
// m.blog.naver.com — and the mobile shape renders its publish date
// via JavaScript, so the date isn't recoverable from that response.
// PostView is the only call site that wants the desktop shape; every
// other endpoint is happy with the mobile UA.
const desktopUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

func fetchPostView(ctx context.Context, c *client.Client, blogID, logNo string) (postparse.PostViewMeta, error) {
	select {
	case <-ctx.Done():
		return postparse.PostViewMeta{}, ctx.Err()
	default:
	}
	headers := map[string]string{
		client.BinaryResponseHeader: "true",
		"User-Agent":                desktopUserAgent,
	}
	raw, err := c.GetWithHeaders(naverurl.PostViewURL(blogID, logNo), nil, headers)
	if err != nil {
		return postparse.PostViewMeta{}, fmt.Errorf("fetching PostView: %w", err)
	}
	meta, err := postparse.ParsePostView([]byte(raw))
	if err != nil {
		return postparse.PostViewMeta{}, fmt.Errorf("parsing PostView: %w", err)
	}
	return meta, nil
}

func applyPostView(snap *Snapshot, meta postparse.PostViewMeta) {
	if snap.CommentsSource == "" {
		n := meta.CommentCount
		if n == 0 && meta.FloatingCommentCount > 0 {
			n = meta.FloatingCommentCount
		}
		if n > 0 {
			snap.Comments = n
			snap.CommentsSource = sourcePostViewHTML
		}
	}
	snap.PublishDateStr = meta.PublishDateStr
	if !meta.PublishedAtUTC.IsZero() {
		snap.PublishedAtUTC = meta.PublishedAtUTC
		snap.DateSource = sourcePostViewHTML
	} else if meta.PublishDateStr != "" {
		snap.DateSource = sourcePostViewHTML
	}
}

func wrapSource(source string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", source, err)
}
