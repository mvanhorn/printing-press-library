package evidence

import (
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"
)

var payloadRE = regexp.MustCompile(`(?s)<script\b[^>]*\bid="__NUXT_DATA__"[^>]*>(.*?)</script>`)
var titleRE = regexp.MustCompile(`(?s)<title[^>]*>(.*?)</title>`)
var yearRE = regexp.MustCompile(`(?:【|\[)(20[0-9]{2})(?:】|\])`)
var closedRE = regexp.MustCompile(`(?s)class="close_msg"[^>]*>(.*?)</div>`)
var tagsRE = regexp.MustCompile(`<[^>]+>`)

func clean(s string) string {
	return strings.TrimSpace(html.UnescapeString(tagsRE.ReplaceAllString(s, "")))
}
func Nuxt(body []byte) (map[string]any, error) {
	m := payloadRE.FindSubmatch(body)
	if len(m) != 2 {
		return nil, fail(5, "Weathernews schema changed: missing Nuxt payload")
	}
	var a []any
	if json.Unmarshal(m[1], &a) != nil || len(a) < 4 || len(a) > 100000 {
		return nil, fail(5, "Weathernews schema changed: invalid Nuxt payload")
	}
	memo := map[int]any{}
	active := map[int]bool{}
	budget := 300000
	var decode func(any, int) (any, error)
	decode = func(ref any, depth int) (any, error) {
		budget--
		if budget < 0 || depth > 80 {
			return nil, fail(5, "Weathernews Nuxt complexity limit exceeded")
		}
		f, ok := ref.(float64)
		if !ok || f != float64(int(f)) {
			return nil, fail(5, "Weathernews Nuxt reference invalid")
		}
		i := int(f)
		if i < 0 {
			if i >= -6 {
				return nil, nil
			}
			return nil, fail(5, "Weathernews Nuxt sentinel unknown")
		}
		if i >= len(a) {
			return nil, fail(5, "Weathernews Nuxt reference out of range")
		}
		if v, ok := memo[i]; ok {
			return v, nil
		}
		if active[i] {
			return nil, fail(5, "Weathernews Nuxt cyclic reference")
		}
		active[i] = true
		defer delete(active, i)
		var out any
		switch v := a[i].(type) {
		case map[string]any:
			o := map[string]any{}
			for k, x := range v {
				d, e := decode(x, depth+1)
				if e != nil {
					return nil, e
				}
				o[k] = d
			}
			out = o
		case []any:
			if len(v) > 0 {
				if tag, ok := v[0].(string); ok {
					switch tag {
					case "Reactive", "ShallowReactive", "Ref", "ShallowRef":
						if len(v) != 2 {
							return nil, fail(5, "invalid Nuxt wrapper")
						}
						d, e := decode(v[1], depth+1)
						if e != nil {
							return nil, e
						}
						out = d
					case "Set":
						out = []any{}
					default:
						return nil, fail(5, "unsupported Nuxt wrapper %s", tag)
					}
					break
				}
			}
			o := make([]any, 0, len(v))
			for _, x := range v {
				d, e := decode(x, depth+1)
				if e != nil {
					return nil, e
				}
				o = append(o, d)
			}
			out = o
		default:
			out = v
		}
		memo[i] = out
		return out, nil
	}
	var root any = a[0]
	for n := 0; n < 5; n++ {
		w, ok := root.([]any)
		if !ok {
			break
		}
		if len(w) != 2 {
			return nil, fail(5, "invalid Nuxt root")
		}
		ix, ok := w[1].(float64)
		if !ok || ix < 0 || int(ix) >= len(a) {
			return nil, fail(5, "invalid Nuxt root reference")
		}
		root = a[int(ix)]
	}
	rawRoot := object(root)
	if rawRoot == nil {
		return nil, fail(5, "invalid Nuxt root data")
	}
	decoded, e := decode(rawRoot["data"], 0)
	if e != nil {
		return nil, e
	}
	data := object(decoded)
	if len(data) == 0 {
		return nil, fail(5, "Weathernews schema changed: missing data")
	}
	return data, nil
}
func SeasonState(body []byte, product string) (map[string]any, error) {
	t := titleRE.FindSubmatch(body)
	if len(t) < 2 {
		return nil, fail(5, "season page lacks title")
	}
	y := yearRE.FindSubmatch(t[1])
	if len(y) < 2 {
		return nil, fail(5, "season year is unavailable; refusing inferred current year")
	}
	year, _ := strconv.Atoi(string(y[1]))
	closed := closedRE.FindSubmatch(body)
	ended := false
	var msg any
	if len(closed) == 2 {
		msg = clean(string(closed[1]))
		ended = strings.Contains(str(msg), "終了")
	}
	return map[string]any{"product": product, "year": year, "updates_ended": ended, "ended_message_ja": msg, "timezone": "Asia/Tokyo", "year_basis": "source_page_title"}, nil
}
