package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/Silo-Server/silo-plugin-ebook-metadata/metadata"
)

const (
	annasArchiveID      = "annasarchive"
	annasArchiveBaseURL = "https://annas-archive.org"
)

var (
	aaMD5RE             = regexp.MustCompile(`^[a-f0-9]{32}$`)
	aaTrRowRE           = regexp.MustCompile(`(?is)<tr[^>]*>.*?</tr>`)
	aaMD5HrefRE         = regexp.MustCompile(`href="/md5/([a-f0-9]{32})"`)
	aaSearchImgRE       = regexp.MustCompile(`(?is)<img[^>]*src="([^"]+)"`)
	aaSearchTitleRE     = regexp.MustCompile(`(?is)<a[^>]*href="/md5/[^"]*"[^>]*>[\s\S]*?<span[^>]*>([^<]+)</span>`)
	aaSearchTitleAltRE  = regexp.MustCompile(`(?is)class="[^"]*js-vim-focus[^"]*"[^>]*>([^<]+)<`)
	aaSearchAuthorRE    = regexp.MustCompile(`(?is)search\?q=author:[^"]*"[^>]*><span[^>]*>([^<]+)</span>`)
	aaSearchPublisherRE = regexp.MustCompile(`(?is)publisher:[^"]*"[^>]*><span[^>]*>([^<]+)</span>`)
	aaYearRE            = regexp.MustCompile(`\b(19|20)\d{2}\b`)
	aaISBN13RE          = regexp.MustCompile(`\b(97[89]\d{10})\b`)
	aaISBN10RE          = regexp.MustCompile(`\b(\d{9}[\dXx])\b`)
	aaFileFormatRE      = regexp.MustCompile(`(?i)\b(epub|pdf|mobi|azw3|cbr|cbz|fb2|djvu|txt|mp3|m4b|m4a|aac|flac|ogg|opus|wav|wma|aax)\b`)
	aaLangCodeRE        = regexp.MustCompile(`(?i)\[([a-z]{2})\]`)
	aaDetailH1RE        = regexp.MustCompile(`(?is)<h1[^>]*>([^<]+)</h1>`)
	aaDetailTitleTagRE  = regexp.MustCompile(`(?is)<title>([^<]+)</title>`)
	aaDetailCoverRE     = regexp.MustCompile(`(?is)<img[^>]*src="([^"]+)"[^>]*alt="cover"`)
	aaDetailAuthorRE    = regexp.MustCompile(`(?is)Author[s]?:\s*<[^>]*>([^<]+)<`)
	aaDetailPublisherRE = regexp.MustCompile(`(?is)Publisher:\s*<[^>]*>([^<]+)<`)
	aaDetailYearRE      = regexp.MustCompile(`(?i)Year:\s*(\d{4})`)
	aaDetailISBN13RE    = regexp.MustCompile(`(?i)ISBN-13:\s*(\d{13})`)
	aaDetailISBN10RE    = regexp.MustCompile(`(?i)ISBN-10:\s*(\d{9}[\dXx])`)
	aaDetailLanguageRE  = regexp.MustCompile(`(?is)Language:\s*<[^>]*>([^<]+)<`)
	aaDetailPagesRE     = regexp.MustCompile(`(?i)Pages?:\s*(\d+)`)
	aaDetailDescRE      = regexp.MustCompile(`(?is)<div[^>]*class="[^"]*description[^"]*"[^>]*>([\s\S]*?)</div>`)
	aaDetailExtensionRE = regexp.MustCompile(`(?i)Extension:\s*([a-z0-9]+)`)
	aaTagStripRE        = regexp.MustCompile(`(?is)<[^>]+>`)
	aaWSRE              = regexp.MustCompile(`\s+`)
	aaNumEntityRE       = regexp.MustCompile(`&#(\d+);`)
	aaEbookExtensions   = map[string]bool{"epub": true, "pdf": true, "mobi": true, "azw3": true, "cbr": true, "cbz": true, "fb2": true, "djvu": true, "txt": true}
	aaAudioExtensions   = map[string]bool{"mp3": true, "m4b": true, "m4a": true, "aac": true, "flac": true, "ogg": true, "opus": true, "wav": true, "wma": true, "aax": true}
)

type AnnasArchiveClient struct {
	baseURL   string
	client    *http.Client
	userAgent string
}

func NewAnnasArchiveClient(userAgent string) *AnnasArchiveClient {
	return NewAnnasArchiveClientAt(annasArchiveBaseURL, userAgent)
}

func NewAnnasArchiveClientAt(baseURL, userAgent string) *AnnasArchiveClient {
	return &AnnasArchiveClient{
		baseURL:   strings.TrimRight(baseURL, "/"),
		client:    http.DefaultClient,
		userAgent: userAgent,
	}
}

func (c *AnnasArchiveClient) ID() string {
	return annasArchiveID
}

func (c *AnnasArchiveClient) Fetch(ctx context.Context, id string) (*metadata.Match, error) {
	id = strings.TrimSpace(strings.ToLower(id))
	if !aaMD5RE.MatchString(id) {
		return nil, nil
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/md5/%s", c.baseURL, url.PathEscape(id)), nil)
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

	match, ext := parseAnnasArchiveDetailPage(body, c.baseURL)
	if match == nil || aaAudioExtensions[ext] || (ext != "" && !aaEbookExtensions[ext]) {
		return nil, nil
	}
	match.Provider = annasArchiveID
	match.ProviderID = id
	return match, nil
}

func (c *AnnasArchiveClient) Search(ctx context.Context, q metadata.SearchQuery) ([]metadata.Match, error) {
	query := strings.TrimSpace(sourceQueryText(q))
	if query == "" {
		return nil, nil
	}
	endpoint := fmt.Sprintf("%s/search?q=%s&display=table", c.baseURL, url.QueryEscape(query))
	body, err := httpGetBytes(ctx, c.client, endpoint, c.userAgent)
	if err != nil {
		return nil, err
	}
	matches := parseAnnasArchiveSearchPage(body, c.baseURL)
	for i := range matches {
		matches[i].Provider = annasArchiveID
	}
	return matches, nil
}

func parseAnnasArchiveSearchPage(html []byte, base string) []metadata.Match {
	rows := aaTrRowRE.FindAllString(string(html), -1)
	if len(rows) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(rows))
	out := make([]metadata.Match, 0, len(rows))
	for _, row := range rows {
		md5m := aaMD5HrefRE.FindStringSubmatch(row)
		if len(md5m) < 2 {
			continue
		}
		md5 := strings.ToLower(md5m[1])
		if seen[md5] {
			continue
		}
		title := aaStripText(aaFirstSubmatch(aaSearchTitleRE, row))
		if title == "" {
			title = aaStripText(aaFirstSubmatch(aaSearchTitleAltRE, row))
		}
		if len(title) < 2 {
			continue
		}
		if aaHasAudioIndicator(row) {
			continue
		}
		if m := aaFileFormatRE.FindStringSubmatch(row); len(m) >= 2 {
			ext := strings.ToLower(m[1])
			if aaAudioExtensions[ext] || !aaEbookExtensions[ext] {
				continue
			}
		}
		seen[md5] = true

		match := metadata.Match{
			ProviderID: md5,
			Title:      title,
		}
		if author := aaStripText(aaFirstSubmatch(aaSearchAuthorRE, row)); author != "" && !strings.EqualFold(author, "unknown") {
			match.Authors = []string{author}
		}
		match.Publisher = aaStripText(aaFirstSubmatch(aaSearchPublisherRE, row))
		match.PublishYear = firstYear(aaYearRE.FindString(row))
		if m := aaISBN13RE.FindStringSubmatch(row); len(m) >= 2 {
			match.ISBN = metadata.NormalizeISBN(m[1])
		} else if m := aaISBN10RE.FindStringSubmatch(row); len(m) >= 2 {
			match.ISBN = metadata.NormalizeISBN(m[1])
		}
		if m := aaLangCodeRE.FindStringSubmatch(row); len(m) >= 2 {
			match.Language = strings.ToLower(m[1])
		}
		if cover := aaFirstSubmatch(aaSearchImgRE, row); cover != "" {
			match.CoverURL = aaResolveURL(cover, base)
		}
		out = append(out, match)
	}
	return out
}

func parseAnnasArchiveDetailPage(html []byte, base string) (*metadata.Match, string) {
	s := string(html)
	if aaHasAudioIndicator(s) {
		return nil, ""
	}
	title := aaStripText(aaFirstSubmatch(aaDetailH1RE, s))
	if title == "" {
		raw := aaStripText(aaFirstSubmatch(aaDetailTitleTagRE, s))
		if idx := strings.IndexAny(raw, "-|"); idx > 0 {
			raw = strings.TrimSpace(raw[:idx])
		}
		title = raw
	}
	if title == "" {
		return nil, ""
	}

	match := &metadata.Match{Title: title}
	if cover := aaFirstSubmatch(aaDetailCoverRE, s); cover != "" {
		match.CoverURL = aaResolveURL(cover, base)
	}
	if author := aaStripText(aaFirstSubmatch(aaDetailAuthorRE, s)); author != "" && !strings.EqualFold(author, "unknown") {
		match.Authors = []string{author}
	}
	match.Publisher = aaStripText(aaFirstSubmatch(aaDetailPublisherRE, s))
	if m := aaDetailYearRE.FindStringSubmatch(s); len(m) >= 2 {
		match.PublishYear = firstYear(m[1])
	} else {
		match.PublishYear = firstYear(aaYearRE.FindString(s))
	}
	if m := aaDetailISBN13RE.FindStringSubmatch(s); len(m) >= 2 {
		match.ISBN = metadata.NormalizeISBN(m[1])
	} else if m := aaISBN13RE.FindStringSubmatch(s); len(m) >= 2 {
		match.ISBN = metadata.NormalizeISBN(m[1])
	} else if m := aaDetailISBN10RE.FindStringSubmatch(s); len(m) >= 2 {
		match.ISBN = metadata.NormalizeISBN(m[1])
	} else if m := aaISBN10RE.FindStringSubmatch(s); len(m) >= 2 {
		match.ISBN = metadata.NormalizeISBN(m[1])
	}
	if lang := aaStripText(aaFirstSubmatch(aaDetailLanguageRE, s)); lang != "" {
		lang = strings.ToLower(lang)
		if len(lang) > 2 {
			lang = lang[:2]
		}
		match.Language = lang
	} else if m := aaLangCodeRE.FindStringSubmatch(s); len(m) >= 2 {
		match.Language = strings.ToLower(m[1])
	}
	if m := aaDetailPagesRE.FindStringSubmatch(s); len(m) >= 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			match.PageCount = n
		}
	}
	match.Description = aaStripText(aaFirstSubmatch(aaDetailDescRE, s))

	ext := ""
	if m := aaDetailExtensionRE.FindStringSubmatch(s); len(m) >= 2 {
		ext = strings.ToLower(m[1])
	}
	return match, ext
}

func aaHasAudioIndicator(s string) bool {
	text := strings.ToLower(aaStripText(s))
	return strings.Contains(text, "audiobook") ||
		strings.Contains(text, "audio book") ||
		strings.Contains(text, "spoken word") ||
		strings.Contains(text, "audible studios")
}

func aaFirstSubmatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func aaResolveURL(u, base string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	if strings.HasPrefix(u, "//") {
		if strings.HasPrefix(base, "http://") {
			return "http:" + u
		}
		return "https:" + u
	}
	if strings.HasPrefix(u, "/") {
		return strings.TrimRight(base, "/") + u
	}
	return u
}

func aaStripText(s string) string {
	if s == "" {
		return ""
	}
	s = aaTagStripRE.ReplaceAllString(s, " ")
	s = strings.NewReplacer(
		"&nbsp;", " ",
		"&amp;", "&",
		"&quot;", `"`,
		"&apos;", "'",
		"&#39;", "'",
		"&lt;", "<",
		"&gt;", ">",
	).Replace(s)
	s = aaNumEntityRE.ReplaceAllStringFunc(s, func(m string) string {
		sm := aaNumEntityRE.FindStringSubmatch(m)
		if len(sm) < 2 {
			return m
		}
		n, err := strconv.Atoi(sm[1])
		if err != nil || n <= 0 || n > 0x10FFFF {
			return m
		}
		return string(rune(n))
	})
	s = aaWSRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
