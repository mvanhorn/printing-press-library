package ecbo

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/ecbo-cloak/internal/cliutil"
	"io"
	"net/http"
	"strings"
	"time"
)

func (c *Client) resolve(ctx context.Context, id string) (string, error) {
	var limiter *cliutil.AdaptiveLimiter = c.limiter
	if e := limiter.Wait(ctx); e != nil {
		return "", e
	}
	target := "https://cloak.ecbo.io/" + c.Locale + "/space/" + id
	req, e := http.NewRequestWithContext(ctx, "GET", target, nil)
	if e != nil {
		return "", e
	}
	req.Header.Set("User-Agent", "ecbo-cloak-cli/0.1 read-only")
	// One legacy request plus its bounded first-party redirect; count each wire request.
	original := c.HTTP.CheckRedirect
	c.HTTP.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if e := original(r, via); e != nil {
			return e
		}
		c.Meta.Requests++
		return nil
	}
	defer func() { c.HTTP.CheckRedirect = original }()
	c.Meta.Requests++
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return "", &Error{5, "transport", e.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		return "", &cliutil.RateLimitError{URL: target}
	}
	if resp.StatusCode == 404 {
		return "", &Error{3, "not_found", "facility source ID not found"}
	}
	if resp.StatusCode != 200 {
		return "", &Error{5, "upstream", fmt.Sprint("legacy resolution HTTP ", resp.StatusCode)}
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if e != nil || len(b) > maxBody {
		return "", &Error{5, "source_shape", "legacy facility page too large or unreadable"}
	}
	m := nextData.FindSubmatch(b)
	if len(m) != 2 {
		return "", &Error{5, "source_shape", "facility page missing Next.js data"}
	}
	var d map[string]any
	if json.Unmarshal(m[1], &d) != nil {
		return "", &Error{5, "source_shape", "invalid facility page JSON"}
	}
	space := object(object(object(d["props"])["pageProps"])["space"])
	uuid := str(space["space_id"])
	if !uuidPattern.MatchString(uuid) || str(space["encrypted_id"]) != id {
		return "", &Error{5, "source_shape", "legacy facility identity mismatch"}
	}
	if !strings.HasSuffix(resp.Request.URL.Path, "/spaces/"+uuid) {
		return "", &Error{5, "source_shape", "missing canonical facility redirect"}
	}
	c.Meta.Observations = append(c.Meta.Observations, Observation{time.Now().UTC().Format(time.RFC3339), false, resp.Request.URL.String()})
	return uuid, nil
}
