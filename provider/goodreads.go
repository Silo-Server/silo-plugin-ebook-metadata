package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Silo-Server/silo-plugin-ebook-metadata/metadata"
)

const (
	goodreadsID      = "goodreads"
	goodreadsBaseURL = "https://www.goodreads.com"
	grMaxResults     = 10
)

var (
	numericRE     = regexp.MustCompile(`^\d+$`)
	grSearchRowRE = regexp.MustCompile(`(?i)href="/book/show/(\d+)[^"]*"[^>]*>\s*<span[^>]*itemprop="name"[^>]*>([^<]+)</span>`)
	grAuthorRE    = regexp.MustCompile(`(?i)class="authorName"[^>]*>[^<]*<span[^>]*itemprop="name"[^>]*>([^<]+)</span>`)
	grCoverRE     = regexp.MustCompile(`(?i)<img[^>]*class="bookCover"[^>]*src="([^"]+)"`)
	grJSONLDRE    = regexp.MustCompile(`(?i)<script[^>]*type="application/ld\+json"[^>]*>([\s\S]*?)</script>`)
	grNextDataRE  = regexp.MustCompile(`(?i)<script[^>]*id="__NEXT_DATA__"[^>]*>([\s\S]*?)</script>`)
)

type GoodreadsClient struct {
	baseURL   string
	client    *http.Client
	userAgent string
}

func NewGoodreadsClient(userAgent string) *GoodreadsClient {
	return NewGoodreadsClientAt(goodreadsBaseURL, userAgent)
}

func NewGoodreadsClientAt(baseURL, userAgent string) *GoodreadsClient {
	return &GoodreadsClient{
		baseURL:   strings.TrimRight(baseURL, "/"),
		client:    http.DefaultClient,
		userAgent: userAgent,
	}
}

func (c *GoodreadsClient) ID() string {
	return goodreadsID
}

func (c *GoodreadsClient) Fetch(ctx context.Context, id string) (*metadata.Match, error) {
	id = strings.TrimSpace(id)
	if !numericRE.MatchString(id) {
		return nil, nil
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/book/show/%s", c.baseURL, url.PathEscape(id)), nil)
	if err != nil {
		return nil, err
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	body, status, err := httpDoBytes(ctx, c.client, req)
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	match := parseGoodreadsBookPage(body)
	if match == nil {
		return nil, nil
	}
	if match.ProviderID == "" {
		match.ProviderID = id
	}
	match.Provider = goodreadsID
	return match, nil
}

func (c *GoodreadsClient) Search(ctx context.Context, q metadata.SearchQuery) ([]metadata.Match, error) {
	query := strings.TrimSpace(sourceQueryText(q))
	if query == "" {
		return nil, nil
	}

	endpoint := fmt.Sprintf("%s/search?q=%s", c.baseURL, url.QueryEscape(query))
	body, err := httpGetBytes(ctx, c.client, endpoint, c.userAgent)
	if err != nil {
		return nil, err
	}
	matches := parseGoodreadsSearchPage(body)
	for i := range matches {
		matches[i].Provider = goodreadsID
	}
	return matches, nil
}

func parseGoodreadsBookPage(html []byte) *metadata.Match {
	s := string(html)
	if match := parseGoodreadsJSONLD(s); match != nil {
		return match
	}
	return parseGoodreadsNextData(s)
}

func parseGoodreadsJSONLD(html string) *metadata.Match {
	allMatches := grJSONLDRE.FindAllStringSubmatch(html, -1)
	var doc map[string]json.RawMessage
	for _, matches := range allMatches {
		if len(matches) < 2 {
			continue
		}
		var candidate map[string]json.RawMessage
		if err := json.Unmarshal([]byte(matches[1]), &candidate); err != nil {
			continue
		}
		var typ string
		if err := json.Unmarshal(candidate["@type"], &typ); err != nil || typ != "Book" {
			continue
		}
		doc = candidate
		break
	}
	if doc == nil {
		return nil
	}

	match := &metadata.Match{}
	match.Title = grJSONLDString(doc, "name")
	match.Description = grJSONLDString(doc, "description")
	match.ISBN = metadata.NormalizeISBN(grJSONLDString(doc, "isbn"))
	match.CoverURL = grJSONLDString(doc, "image")
	match.PublishYear = firstYear(grJSONLDString(doc, "datePublished"))
	match.Language = grJSONLDString(doc, "inLanguage")
	match.Publisher = grJSONLDPublisher(doc["publisher"])
	match.Authors = grExtractNames(doc["author"])
	if pagesRaw, ok := doc["numberOfPages"]; ok {
		var pages int
		if json.Unmarshal(pagesRaw, &pages) == nil && pages > 0 {
			match.PageCount = pages
		}
	}
	if pageURL := grJSONLDString(doc, "url"); pageURL != "" {
		match.ProviderID = goodreadsIDFromURL(pageURL)
	}
	if match.Title == "" {
		return nil
	}
	return match
}

func grJSONLDString(doc map[string]json.RawMessage, key string) string {
	raw, ok := doc[key]
	if !ok {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func grJSONLDPublisher(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return strings.TrimSpace(value)
	}
	var obj struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return strings.TrimSpace(obj.Name)
	}
	return ""
}

func goodreadsIDFromURL(value string) string {
	parts := strings.Split(strings.TrimSuffix(value, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	if idx := strings.Index(last, "-"); idx >= 0 {
		last = last[:idx]
	}
	if numericRE.MatchString(last) {
		return last
	}
	return ""
}

func parseGoodreadsNextData(html string) *metadata.Match {
	m := grNextDataRE.FindStringSubmatch(html)
	if len(m) < 2 {
		return nil
	}
	var data any
	if err := json.Unmarshal([]byte(m[1]), &data); err != nil {
		return nil
	}
	var out []metadata.Match
	traverseGoodreadsNextData(data, &out, 0)
	if len(out) == 0 {
		return nil
	}
	return &out[0]
}

const maxGoodreadsTraverseDepth = 64

func traverseGoodreadsNextData(v any, out *[]metadata.Match, depth int) {
	if depth > maxGoodreadsTraverseDepth {
		return
	}
	switch val := v.(type) {
	case []any:
		for _, item := range val {
			traverseGoodreadsNextData(item, out, depth+1)
		}
	case map[string]any:
		if match := goodreadsNextDataBookToMatch(val); match != nil {
			*out = append(*out, *match)
			return
		}
		for _, child := range val {
			traverseGoodreadsNextData(child, out, depth+1)
		}
	}
}

func goodreadsNextDataBookToMatch(m map[string]any) *metadata.Match {
	title := grStringField(m, "title")
	if title == "" {
		return nil
	}
	_, hasDesc := m["description"]
	_, hasImage := m["imageUrl"]
	_, hasISBN := m["isbn"]
	_, hasGenres := m["genres"]
	if !hasDesc && !hasImage && !hasISBN && !hasGenres {
		return nil
	}

	match := &metadata.Match{
		Title:       title,
		Description: grStringField(m, "description"),
		ISBN:        metadata.NormalizeISBN(grStringField(m, "isbn")),
		CoverURL:    grStringField(m, "imageUrl"),
		Publisher:   grStringField(m, "publisher"),
		Language:    grStringField(m, "language"),
		PublishYear: firstYear(grStringField(m, "publicationDate")),
	}
	if id, ok := m["legacyId"].(float64); ok {
		match.ProviderID = fmt.Sprintf("%d", int(id))
	}
	if match.ProviderID == "" {
		match.ProviderID = grStringField(m, "id")
	}
	if pages, ok := m["numPages"].(float64); ok && pages > 0 {
		match.PageCount = int(pages)
	}
	if arr, ok := m["authors"].([]any); ok {
		for _, item := range arr {
			if obj, ok := item.(map[string]any); ok {
				if name := grStringField(obj, "name"); name != "" {
					match.Authors = append(match.Authors, name)
				}
			}
		}
	}
	if arr, ok := m["genres"].([]any); ok {
		for _, item := range arr {
			switch value := item.(type) {
			case string:
				if name := strings.TrimSpace(value); name != "" {
					match.Genres = append(match.Genres, name)
				}
			case map[string]any:
				if name := grStringField(value, "name"); name != "" {
					match.Genres = append(match.Genres, name)
				}
			}
		}
	}
	return match
}

func parseGoodreadsSearchPage(html []byte) []metadata.Match {
	s := string(html)
	titleMatches := grSearchRowRE.FindAllStringSubmatch(s, -1)
	if len(titleMatches) == 0 {
		return nil
	}
	if len(titleMatches) > grMaxResults {
		titleMatches = titleMatches[:grMaxResults]
	}
	authorMatches := grAuthorRE.FindAllStringSubmatch(s, -1)
	coverMatches := grCoverRE.FindAllStringSubmatch(s, -1)
	out := make([]metadata.Match, 0, len(titleMatches))
	for i, tm := range titleMatches {
		if len(tm) < 3 {
			continue
		}
		providerID := strings.TrimSpace(tm[1])
		title := strings.TrimSpace(tm[2])
		if providerID == "" || title == "" {
			continue
		}
		match := metadata.Match{ProviderID: providerID, Title: title}
		if i < len(authorMatches) && len(authorMatches[i]) >= 2 {
			match.Authors = single(authorMatches[i][1])
		}
		if i < len(coverMatches) && len(coverMatches[i]) >= 2 {
			match.CoverURL = strings.TrimSpace(coverMatches[i][1])
		}
		out = append(out, match)
	}
	return out
}

func grStringField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func grExtractNames(raw json.RawMessage) []string {
	var arr []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &arr); err == nil {
		names := make([]string, 0, len(arr))
		for _, item := range arr {
			if name := strings.TrimSpace(item.Name); name != "" {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			return names
		}
	}
	var single struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &single); err == nil && strings.TrimSpace(single.Name) != "" {
		return []string{strings.TrimSpace(single.Name)}
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil && strings.TrimSpace(value) != "" {
		return []string{strings.TrimSpace(value)}
	}
	return nil
}
