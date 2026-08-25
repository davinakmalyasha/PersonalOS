// Package knowledge holds pure domain logic for the Knowledge pillar:
// URL normalization (dedupe) and FTS5 query sanitization.
package knowledge

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode"
)

// trackingParams are stripped during URL normalization so the same page
// reached through different campaigns dedupes to one row.
var trackingParams = map[string]bool{
	"fbclid": true, "gclid": true, "dclid": true, "msclkid": true,
	"mc_cid": true, "mc_eid": true, "igshid": true, "si": true,
}

func isTrackingKey(key string) bool {
	k := strings.ToLower(key)
	if trackingParams[k] {
		return true
	}
	return strings.HasPrefix(k, "utm_")
}

// NormalizeURL canonicalizes a URL for storage/dedupe:
//   - trim whitespace; scheme must be http/https (else error)
//   - lowercase scheme + host; strip default port (80/443)
//   - drop fragment; drop tracking params (utm_*, fbclid, gclid, …)
//   - remaining query keys re-encoded sorted; trailing "/" trimmed
//
// Path and surviving query values are otherwise preserved verbatim.
// Deterministic: equal inputs always yield the canonical form, so the UNIQUE
// index on bookmarks.url provides idempotent creates.
func NormalizeURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("empty url")
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("unparseable url %q", raw)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("url scheme must be http or https, got %q", u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", fmt.Errorf("url host required")
	}
	port := u.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	hostPart := host
	if port != "" {
		hostPart = host + ":" + port
	}

	query := normalizeQuery(u.RawQuery)

	path := u.EscapedPath()
	if path == "/" {
		path = ""
	} else if len(path) > 1 {
		// Trailing slashes are removed once so both variants map to one
		// canonical form ("https://x/blog/" ≡ "https://x/blog").
		path = strings.TrimSuffix(path, "/")
	}
	canonical := scheme + "://" + hostPart + path
	if query != "" {
		canonical += "?" + query
	}
	return canonical, nil
}

// normalizeQuery drops tracking params and re-encodes survivors with sorted
// keys (values keep their original order per key).
func normalizeQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	pairs := strings.Split(rawQuery, "&")
	type kv struct{ k, v string }
	var kept []kv
	for _, p := range pairs {
		if p == "" {
			continue
		}
		key, val, _ := strings.Cut(p, "=")
		if decoded, err := url.QueryUnescape(key); err == nil && isTrackingKey(decoded) {
			continue
		}
		kept = append(kept, kv{key, val})
	}
	if len(kept) == 0 {
		return ""
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].k < kept[j].k })
	var b strings.Builder
	for _, p := range kept {
		if b.Len() > 0 {
			b.WriteByte('&')
		}
		b.WriteString(p.k)
		b.WriteString("=")
		b.WriteString(p.v)
	}
	return b.String()
}

// SanitizeFTSQuery extracts word tokens from free text and returns a safe
// FTS5 MATCH expression: each token double-quoted, implicit AND between them.
// FTS5 operators (AND/OR/NOT/*/:) and punctuation in user input are
// neutralized. Returns "" when nothing searchable remains. Max 24 tokens.
func SanitizeFTSQuery(q string) string {
	var toks []string
	cur := strings.Builder{}
	flush := func() {
		if cur.Len() > 0 {
			toks = append(toks, cur.String())
			cur.Reset()
		}
	}
	for _, r := range strings.ToLower(q) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			cur.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	if len(toks) > 24 {
		toks = toks[:24]
	}
	if len(toks) == 0 {
		return ""
	}
	quoted := make([]string, len(toks))
	for i, t := range toks {
		quoted[i] = `"` + t + `"`
	}
	return strings.Join(quoted, " ")
}
