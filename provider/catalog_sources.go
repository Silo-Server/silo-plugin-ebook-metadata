package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/Silo-Server/silo-plugin-ebook-metadata/metadata"
)

const (
	gutenbergID        = "gutenberg"
	bookBrainzID       = "bookbrainz"
	fantasticFictionID = "fantasticfiction"
	isfdbID            = "isfdb"
	libraryThingID     = "librarything"
	internetArchiveID  = "internetarchive"
	worldCatID         = "worldcat"
	doubanID           = "douban"
)

var (
	catalogNumericRE = regexp.MustCompile(`^\d+$`)
	catalogUUIDRE    = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	catalogISBNRE    = regexp.MustCompile(`^(?:\d{9}[\dXx]|\d{13})$`)
	catalogTagRE     = regexp.MustCompile(`(?is)<[^>]+>`)
	catalogWSRE      = regexp.MustCompile(`\s+`)
	catalogNumEntRE  = regexp.MustCompile(`&#(\d+);`)
)

type catalogHTTPClient struct {
	baseURL   string
	client    *http.Client
	userAgent string
}

func newCatalogHTTPClient(baseURL, userAgent string) catalogHTTPClient {
	return catalogHTTPClient{baseURL: strings.TrimRight(baseURL, "/"), client: http.DefaultClient, userAgent: userAgent}
}

func (c catalogHTTPClient) get(ctx context.Context, endpoint string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return httpDoBytes(ctx, c.client, req)
}

func htmlText(s string) string {
	s = catalogTagRE.ReplaceAllString(s, " ")
	s = strings.NewReplacer(
		"&nbsp;", " ",
		"&amp;", "&",
		"&quot;", `"`,
		"&apos;", "'",
		"&#39;", "'",
		"&lt;", "<",
		"&gt;", ">",
	).Replace(s)
	s = catalogNumEntRE.ReplaceAllStringFunc(s, func(m string) string {
		sm := catalogNumEntRE.FindStringSubmatch(m)
		if len(sm) < 2 {
			return m
		}
		n, err := strconv.Atoi(sm[1])
		if err != nil || n <= 0 || n > 0x10FFFF {
			return m
		}
		return string(rune(n))
	})
	return strings.TrimSpace(catalogWSRE.ReplaceAllString(s, " "))
}

func firstSubmatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func compactISBN(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "-", "")
}

func fetchableRelativePath(path string) bool {
	return strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//")
}

type GutenbergClient struct {
	http catalogHTTPClient
}

func NewGutenbergClient(userAgent string) *GutenbergClient {
	return NewGutenbergClientAt("https://gutendex.com", userAgent)
}

func NewGutenbergClientAt(baseURL, userAgent string) *GutenbergClient {
	return &GutenbergClient{http: newCatalogHTTPClient(baseURL, userAgent)}
}

func (c *GutenbergClient) ID() string { return gutenbergID }

func (c *GutenbergClient) Search(ctx context.Context, q metadata.SearchQuery) ([]metadata.Match, error) {
	query := strings.TrimSpace(sourceQueryText(q))
	if query == "" {
		return nil, nil
	}
	body, _, err := c.http.get(ctx, fmt.Sprintf("%s/books?search=%s", c.http.baseURL, url.QueryEscape(query)))
	if err != nil {
		return nil, err
	}
	var resp struct {
		Results []gutenbergBook `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	out := make([]metadata.Match, 0, len(resp.Results))
	for _, book := range resp.Results {
		out = append(out, book.toMatch())
	}
	return out, nil
}

func (c *GutenbergClient) Fetch(ctx context.Context, id string) (*metadata.Match, error) {
	id = strings.TrimSpace(id)
	if !catalogNumericRE.MatchString(id) {
		return nil, nil
	}
	body, status, err := c.http.get(ctx, fmt.Sprintf("%s/books/%s", c.http.baseURL, url.PathEscape(id)))
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var book gutenbergBook
	if err := json.Unmarshal(body, &book); err != nil {
		return nil, err
	}
	match := book.toMatch()
	return &match, nil
}

type gutenbergBook struct {
	ID        int                     `json:"id"`
	Title     string                  `json:"title"`
	Authors   []struct{ Name string } `json:"authors"`
	Subjects  []string                `json:"subjects"`
	Languages []string                `json:"languages"`
	Formats   map[string]string       `json:"formats"`
}

func (b gutenbergBook) toMatch() metadata.Match {
	authors := make([]string, 0, len(b.Authors))
	for _, author := range b.Authors {
		if name := strings.TrimSpace(author.Name); name != "" {
			authors = append(authors, name)
		}
	}
	genres := append([]string(nil), b.Subjects...)
	if len(genres) > 5 {
		genres = genres[:5]
	}
	language := ""
	if len(b.Languages) > 0 {
		language = b.Languages[0]
	}
	cover := b.Formats["image/jpeg"]
	if cover == "" {
		cover = b.Formats["image/png"]
	}
	return metadata.Match{Provider: gutenbergID, ProviderID: strconv.Itoa(b.ID), Title: b.Title, Authors: authors, Genres: genres, Language: language, CoverURL: cover}
}

type BookBrainzClient struct {
	http catalogHTTPClient
}

func NewBookBrainzClient(userAgent string) *BookBrainzClient {
	return NewBookBrainzClientAt("https://api.bookbrainz.org/1", userAgent)
}

func NewBookBrainzClientAt(baseURL, userAgent string) *BookBrainzClient {
	return &BookBrainzClient{http: newCatalogHTTPClient(baseURL, userAgent)}
}

func (c *BookBrainzClient) ID() string { return bookBrainzID }

func (c *BookBrainzClient) Search(ctx context.Context, q metadata.SearchQuery) ([]metadata.Match, error) {
	query := strings.TrimSpace(sourceQueryText(q))
	if query == "" {
		return nil, nil
	}
	body, _, err := c.http.get(ctx, fmt.Sprintf("%s/search?q=%s&type=edition&limit=10", c.http.baseURL, url.QueryEscape(query)))
	if err != nil {
		return nil, err
	}
	var resp struct {
		Results []bbEntity `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	out := make([]metadata.Match, 0, len(resp.Results))
	for _, entity := range resp.Results {
		match := entity.toMatch()
		if match.Title != "" && catalogUUIDRE.MatchString(match.ProviderID) {
			out = append(out, match)
		}
	}
	return out, nil
}

func (c *BookBrainzClient) Fetch(ctx context.Context, id string) (*metadata.Match, error) {
	id = strings.TrimSpace(id)
	if !catalogUUIDRE.MatchString(id) {
		return nil, nil
	}
	body, status, err := c.http.get(ctx, fmt.Sprintf("%s/edition/%s", c.http.baseURL, url.PathEscape(id)))
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entity bbEntity
	if err := json.Unmarshal(body, &entity); err != nil {
		return nil, err
	}
	match := entity.toMatch()
	if match.Title == "" {
		return nil, nil
	}
	return &match, nil
}

type bbEntity struct {
	BBID         string `json:"bbid"`
	DefaultAlias *struct {
		Name     string                 `json:"name"`
		Language *struct{ Name string } `json:"language"`
	} `json:"defaultAlias"`
	Disambiguation string                    `json:"disambiguation"`
	Annotation     *struct{ Content string } `json:"annotation"`
	IdentifierSet  *struct {
		Identifiers []struct {
			Type  struct{ Label string } `json:"type"`
			Value string                 `json:"value"`
		} `json:"identifiers"`
	} `json:"identifierSet"`
	AuthorCredit *struct {
		Names []struct {
			Author struct {
				DefaultAlias *struct{ Name string } `json:"defaultAlias"`
			} `json:"author"`
		} `json:"names"`
	} `json:"authorCredit"`
	PublisherSet *struct {
		Publishers []struct {
			DefaultAlias *struct{ Name string } `json:"defaultAlias"`
		} `json:"publishers"`
	} `json:"publisherSet"`
	ReleaseEventSet *struct {
		ReleaseEvents []struct{ Date string } `json:"releaseEvents"`
	} `json:"releaseEventSet"`
}

func (e bbEntity) toMatch() metadata.Match {
	match := metadata.Match{Provider: bookBrainzID, ProviderID: e.BBID}
	if e.DefaultAlias != nil {
		match.Title = e.DefaultAlias.Name
		if e.DefaultAlias.Language != nil {
			match.Language = e.DefaultAlias.Language.Name
		}
	}
	if e.Annotation != nil {
		match.Description = e.Annotation.Content
	}
	if match.Description == "" {
		match.Description = e.Disambiguation
	}
	if e.AuthorCredit != nil {
		for _, name := range e.AuthorCredit.Names {
			if name.Author.DefaultAlias != nil && name.Author.DefaultAlias.Name != "" {
				match.Authors = append(match.Authors, name.Author.DefaultAlias.Name)
			}
		}
	}
	if e.PublisherSet != nil && len(e.PublisherSet.Publishers) > 0 && e.PublisherSet.Publishers[0].DefaultAlias != nil {
		match.Publisher = e.PublisherSet.Publishers[0].DefaultAlias.Name
	}
	if e.ReleaseEventSet != nil && len(e.ReleaseEventSet.ReleaseEvents) > 0 {
		match.PublishYear = firstYear(e.ReleaseEventSet.ReleaseEvents[0].Date)
	}
	var isbn10, isbn13 string
	if e.IdentifierSet != nil {
		for _, id := range e.IdentifierSet.Identifiers {
			label := strings.ToLower(id.Type.Label)
			if strings.Contains(label, "isbn-13") && isbn13 == "" {
				isbn13 = id.Value
			}
			if strings.Contains(label, "isbn-10") && isbn10 == "" {
				isbn10 = id.Value
			}
		}
	}
	match.ISBN = metadata.NormalizeISBN(firstNonEmpty(isbn13, isbn10))
	return match
}

type FantasticFictionClient struct {
	http catalogHTTPClient
}

func NewFantasticFictionClient(userAgent string) *FantasticFictionClient {
	return NewFantasticFictionClientAt("https://www.fantasticfiction.com", userAgent)
}

func NewFantasticFictionClientAt(baseURL, userAgent string) *FantasticFictionClient {
	return &FantasticFictionClient{http: newCatalogHTTPClient(baseURL, userAgent)}
}

func (c *FantasticFictionClient) ID() string { return fantasticFictionID }

func (c *FantasticFictionClient) Fetch(ctx context.Context, id string) (*metadata.Match, error) {
	id = strings.TrimSpace(id)
	path, ok := strings.CutPrefix(id, "path:")
	if !ok || !fetchableRelativePath(path) {
		return nil, nil
	}
	body, status, err := c.http.get(ctx, c.http.baseURL+path)
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	match := parseFantasticFictionBookPage(body)
	if match == nil {
		return nil, nil
	}
	match.Provider = fantasticFictionID
	match.ProviderID = id
	return match, nil
}

func (c *FantasticFictionClient) Search(ctx context.Context, q metadata.SearchQuery) ([]metadata.Match, error) {
	query := strings.TrimSpace(sourceQueryText(q))
	if query == "" {
		return nil, nil
	}
	body, status, err := c.http.get(ctx, fmt.Sprintf("%s/search/?searchfor=book&keywords=%s", c.http.baseURL, url.QueryEscape(query)))
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	matches := parseFantasticFictionSearchPage(body)
	for i := range matches {
		matches[i].Provider = fantasticFictionID
	}
	return matches, nil
}

var (
	ffBookBlockRE = regexp.MustCompile(`(?is)<div[^>]*class="[^"]*book[^"]*"[^>]*>.*?</div>`)
	ffTitleRE     = regexp.MustCompile(`(?is)<a[^>]*href="([^"]+)"[^>]*>([^<]+)</a>`)
	ffAuthorRE    = regexp.MustCompile(`(?is)by\s+<a[^>]*>([^<]+)</a>`)
	ffYearRE      = regexp.MustCompile(`\((\d{4})\)`)
	ffSeriesRE    = regexp.MustCompile(`(?is)Series:\s*<a[^>]*>([^<]+)</a>`)
	ffH1RE        = regexp.MustCompile(`(?is)<h1[^>]*>([^<]+)</h1>`)
	ffTitleTagRE  = regexp.MustCompile(`(?is)<title>\s*([^<]+?)\s*</title>`)
	ffDescRE      = regexp.MustCompile(`(?is)<div[^>]*class="[^"]*blurb[^"]*"[^>]*>(.*?)</div>`)
)

func parseFantasticFictionSearchPage(html []byte) []metadata.Match {
	blocks := ffBookBlockRE.FindAllString(string(html), -1)
	if len(blocks) > 10 {
		blocks = blocks[:10]
	}
	out := make([]metadata.Match, 0, len(blocks))
	for _, block := range blocks {
		tm := ffTitleRE.FindStringSubmatch(block)
		if len(tm) < 3 {
			continue
		}
		title := htmlText(tm[2])
		if title == "" {
			continue
		}
		if !fetchableRelativePath(tm[1]) {
			continue
		}
		match := metadata.Match{ProviderID: "path:" + tm[1], Title: title}
		if author := htmlText(firstSubmatch(ffAuthorRE, block)); author != "" {
			match.Authors = []string{author}
		}
		match.PublishYear = firstYear(firstSubmatch(ffYearRE, block))
		match.SeriesName = htmlText(firstSubmatch(ffSeriesRE, block))
		out = append(out, match)
	}
	return out
}

func parseFantasticFictionBookPage(html []byte) *metadata.Match {
	s := string(html)
	title := htmlText(firstSubmatch(ffH1RE, s))
	if title == "" {
		title = htmlText(firstSubmatch(ffTitleTagRE, s))
		if idx := strings.Index(title, " - "); idx > 0 {
			title = strings.TrimSpace(title[:idx])
		}
	}
	if title == "" {
		return nil
	}
	match := &metadata.Match{Title: title}
	if author := htmlText(firstSubmatch(ffAuthorRE, s)); author != "" {
		match.Authors = []string{author}
	}
	match.PublishYear = firstYear(firstSubmatch(ffYearRE, s))
	match.SeriesName = htmlText(firstSubmatch(ffSeriesRE, s))
	match.Description = htmlText(firstSubmatch(ffDescRE, s))
	return match
}

type ISFDBClient struct {
	http catalogHTTPClient
}

func NewISFDBClient(userAgent string) *ISFDBClient {
	return NewISFDBClientAt("https://www.isfdb.org", userAgent)
}

func NewISFDBClientAt(baseURL, userAgent string) *ISFDBClient {
	return &ISFDBClient{http: newCatalogHTTPClient(baseURL, userAgent)}
}

func (c *ISFDBClient) ID() string { return isfdbID }

func (c *ISFDBClient) Search(ctx context.Context, q metadata.SearchQuery) ([]metadata.Match, error) {
	query := strings.TrimSpace(sourceQueryText(q))
	if query == "" {
		return nil, nil
	}
	body, status, err := c.http.get(ctx, fmt.Sprintf("%s/cgi-bin/se.cgi?arg=%s&type=Title", c.http.baseURL, url.QueryEscape(query)))
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	matches := parseISFDBSearchPage(body)
	for i := range matches {
		matches[i].Provider = isfdbID
	}
	return matches, nil
}

func (c *ISFDBClient) Fetch(ctx context.Context, id string) (*metadata.Match, error) {
	id = strings.TrimSpace(id)
	if !catalogNumericRE.MatchString(id) {
		return nil, nil
	}
	body, status, err := c.http.get(ctx, fmt.Sprintf("%s/cgi-bin/title.cgi?%s", c.http.baseURL, url.QueryEscape(id)))
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	match := parseISFDBTitlePage(body)
	if match == nil {
		return nil, nil
	}
	match.Provider = isfdbID
	match.ProviderID = id
	return match, nil
}

var (
	isRowRE       = regexp.MustCompile(`(?is)<tr[^>]*>.*?</tr>`)
	isTitleLinkRE = regexp.MustCompile(`(?is)<a[^>]*href="/cgi-bin/title\.cgi\?(\d+)"[^>]*>([^<]+)</a>`)
	isAuthorRE    = regexp.MustCompile(`(?is)<a[^>]*href="/cgi-bin/ea\.cgi\?[^"]*"[^>]*>([^<]+)</a>`)
	isYearRE      = regexp.MustCompile(`\b(19|20)\d{2}\b`)
	isH1RE        = regexp.MustCompile(`(?is)<h1[^>]*>([^<]+)</h1>`)
	isPublisherRE = regexp.MustCompile(`(?is)<b>Publisher:</b>\s*(?:<a[^>]*>)?([^<]+)`)
	isISBNRE      = regexp.MustCompile(`(?is)<b>ISBN:</b>\s*([\dXx-]+)`)
	isPagesRE     = regexp.MustCompile(`(?is)<b>Pages:</b>\s*(\d+)`)
	isSeriesRE    = regexp.MustCompile(`(?is)<b>Series:</b>\s*<a[^>]*>([^<]+)</a>`)
	isSeriesPosRE = regexp.MustCompile(`(?is)<b>Series Number:</b>\s*([^<]+)`)
	isCoverRE     = regexp.MustCompile(`(?is)<img[^>]*src="([^"]+)"[^>]*class="[^"]*scan[^"]*"`)
)

func parseISFDBSearchPage(html []byte) []metadata.Match {
	rows := isRowRE.FindAllString(string(html), -1)
	out := make([]metadata.Match, 0, len(rows))
	for _, row := range rows {
		tm := isTitleLinkRE.FindStringSubmatch(row)
		if len(tm) < 3 {
			continue
		}
		match := metadata.Match{ProviderID: tm[1], Title: htmlText(tm[2]), PublishYear: firstYear(isYearRE.FindString(row))}
		for _, am := range isAuthorRE.FindAllStringSubmatch(row, -1) {
			if len(am) >= 2 {
				if name := htmlText(am[1]); name != "" {
					match.Authors = append(match.Authors, name)
				}
			}
		}
		out = append(out, match)
	}
	return out
}

func parseISFDBTitlePage(html []byte) *metadata.Match {
	s := string(html)
	title := htmlText(firstSubmatch(isH1RE, s))
	if title == "" {
		return nil
	}
	match := &metadata.Match{Title: title, PublishYear: firstYear(isYearRE.FindString(s))}
	seen := map[string]bool{}
	for _, am := range isAuthorRE.FindAllStringSubmatch(s, -1) {
		if len(am) < 2 {
			continue
		}
		name := htmlText(am[1])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		match.Authors = append(match.Authors, name)
	}
	match.Publisher = htmlText(firstSubmatch(isPublisherRE, s))
	match.ISBN = metadata.NormalizeISBN(firstSubmatch(isISBNRE, s))
	if pages := firstSubmatch(isPagesRE, s); pages != "" {
		match.PageCount, _ = strconv.Atoi(pages)
	}
	match.SeriesName = htmlText(firstSubmatch(isSeriesRE, s))
	match.SeriesPosition = htmlText(firstSubmatch(isSeriesPosRE, s))
	match.CoverURL = firstSubmatch(isCoverRE, s)
	return match
}

type LibraryThingClient struct {
	http catalogHTTPClient
}

func NewLibraryThingClient(userAgent string) *LibraryThingClient {
	return NewLibraryThingClientAt("https://www.librarything.com", userAgent)
}

func NewLibraryThingClientAt(baseURL, userAgent string) *LibraryThingClient {
	return &LibraryThingClient{http: newCatalogHTTPClient(baseURL, userAgent)}
}

func (c *LibraryThingClient) ID() string { return libraryThingID }

func (c *LibraryThingClient) Search(ctx context.Context, q metadata.SearchQuery) ([]metadata.Match, error) {
	query := strings.TrimSpace(sourceQueryText(q))
	if query == "" {
		return nil, nil
	}
	body, status, err := c.http.get(ctx, fmt.Sprintf("%s/search.php?search=%s&searchtype=newwork_titles", c.http.baseURL, url.QueryEscape(query)))
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	matches := parseLibraryThingSearchPage(body)
	for i := range matches {
		matches[i].Provider = libraryThingID
	}
	return matches, nil
}

func (c *LibraryThingClient) Fetch(ctx context.Context, id string) (*metadata.Match, error) {
	id = compactISBN(id)
	endpoint := ""
	if workID, ok := strings.CutPrefix(id, "work:"); ok && catalogNumericRE.MatchString(workID) {
		endpoint = fmt.Sprintf("%s/work/%s", c.http.baseURL, url.PathEscape(workID))
	} else if catalogISBNRE.MatchString(id) {
		endpoint = fmt.Sprintf("%s/isbn/%s", c.http.baseURL, url.PathEscape(id))
	}
	if endpoint == "" {
		return nil, nil
	}
	body, status, err := c.http.get(ctx, endpoint)
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	match := parseLibraryThingWorkPage(body)
	if match == nil {
		return nil, nil
	}
	match.Provider = libraryThingID
	match.ProviderID = id
	if catalogISBNRE.MatchString(id) {
		match.ISBN = metadata.NormalizeISBN(id)
	}
	return match, nil
}

var (
	ltRowRE     = regexp.MustCompile(`(?is)<tr[^>]*class="[^"]*searchresult[^"]*"[^>]*>.*?</tr>`)
	ltTitleRE   = regexp.MustCompile(`(?is)<a[^>]*href="/work/(\d+)[^"]*"[^>]*>([^<]+)</a>`)
	ltAuthorRE  = regexp.MustCompile(`(?is)<a[^>]*href="/author/[^"]*"[^>]*>([^<]+)</a>`)
	ltYearRE    = regexp.MustCompile(`\((\d{4})\)`)
	ltH1RE      = regexp.MustCompile(`(?is)<h1[^>]*>([^<]+)</h1>`)
	ltOGTitleRE = regexp.MustCompile(`(?i)<meta\s+property="og:title"\s+content="([^"]+)"`)
	ltDescRE    = regexp.MustCompile(`(?is)<div[^>]*id="[^"]*description[^"]*"[^>]*>(.*?)</div>`)
	ltCoverRE   = regexp.MustCompile(`(?is)<img[^>]*class="[^"]*cover[^"]*"[^>]*src="([^"]+)"`)
	ltOGCoverRE = regexp.MustCompile(`(?i)<meta\s+property="og:image"\s+content="([^"]+)"`)
	ltSeriesRE  = regexp.MustCompile(`(?is)Series:\s*<a[^>]*>([^<]+)</a>`)
	ltBRRE      = regexp.MustCompile(`(?i)<br\s*/?>`)
	ltMultiNLRE = regexp.MustCompile(`\n{3,}`)
)

func parseLibraryThingSearchPage(html []byte) []metadata.Match {
	rows := ltRowRE.FindAllString(string(html), -1)
	out := make([]metadata.Match, 0, len(rows))
	for _, row := range rows {
		tm := ltTitleRE.FindStringSubmatch(row)
		if len(tm) < 3 {
			continue
		}
		title := htmlText(tm[2])
		if title == "" {
			continue
		}
		match := metadata.Match{ProviderID: "work:" + tm[1], Title: title, PublishYear: firstYear(firstSubmatch(ltYearRE, row))}
		if author := htmlText(firstSubmatch(ltAuthorRE, row)); author != "" {
			match.Authors = []string{author}
		}
		out = append(out, match)
	}
	return out
}

func parseLibraryThingWorkPage(html []byte) *metadata.Match {
	s := string(html)
	title := htmlText(firstNonEmpty(firstSubmatch(ltH1RE, s), firstSubmatch(ltOGTitleRE, s)))
	if title == "" {
		return nil
	}
	match := &metadata.Match{Title: title}
	seen := map[string]bool{}
	for _, am := range ltAuthorRE.FindAllStringSubmatch(s, -1) {
		if len(am) < 2 {
			continue
		}
		name := htmlText(am[1])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		match.Authors = append(match.Authors, name)
	}
	if desc := firstSubmatch(ltDescRE, s); desc != "" {
		desc = ltBRRE.ReplaceAllString(desc, "\n")
		desc = catalogTagRE.ReplaceAllString(desc, "")
		desc = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'").Replace(desc)
		match.Description = strings.TrimSpace(ltMultiNLRE.ReplaceAllString(desc, "\n\n"))
	}
	match.CoverURL = firstNonEmpty(firstSubmatch(ltCoverRE, s), firstSubmatch(ltOGCoverRE, s))
	match.SeriesName = htmlText(firstSubmatch(ltSeriesRE, s))
	return match
}

type InternetArchiveClient struct {
	http catalogHTTPClient
}

func NewInternetArchiveClient(userAgent string) *InternetArchiveClient {
	return NewInternetArchiveClientAt("https://archive.org", userAgent)
}

func NewInternetArchiveClientAt(baseURL, userAgent string) *InternetArchiveClient {
	return &InternetArchiveClient{http: newCatalogHTTPClient(baseURL, userAgent)}
}

func (c *InternetArchiveClient) ID() string { return internetArchiveID }

func (c *InternetArchiveClient) Search(ctx context.Context, q metadata.SearchQuery) ([]metadata.Match, error) {
	query := strings.TrimSpace(sourceQueryText(q))
	if query == "" {
		return nil, nil
	}
	values := url.Values{}
	values.Set("q", query+" AND mediatype:texts")
	values["fl[]"] = []string{"identifier", "title", "creator", "publisher", "date", "language", "description", "isbn"}
	values.Set("output", "json")
	values.Set("rows", "10")
	body, status, err := c.http.get(ctx, fmt.Sprintf("%s/advancedsearch.php?%s", c.http.baseURL, values.Encode()))
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var resp struct {
		Response struct {
			Docs []iaItem `json:"docs"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	out := make([]metadata.Match, 0, len(resp.Response.Docs))
	for _, item := range resp.Response.Docs {
		match := item.toMatch(c.http.baseURL)
		if strings.TrimSpace(match.ProviderID) == "" {
			continue
		}
		out = append(out, match)
	}
	return out, nil
}

func (c *InternetArchiveClient) Fetch(ctx context.Context, id string) (*metadata.Match, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil
	}
	body, status, err := c.http.get(ctx, fmt.Sprintf("%s/metadata/%s", c.http.baseURL, url.PathEscape(id)))
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var resp struct {
		Metadata json.RawMessage `json:"metadata"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if len(resp.Metadata) == 0 {
		return nil, nil
	}
	var item iaItem
	if err := json.Unmarshal(resp.Metadata, &item); err != nil {
		return nil, err
	}
	if item.Identifier == "" {
		item.Identifier = id
	}
	match := item.toMatch(c.http.baseURL)
	return &match, nil
}

type iaItem struct {
	Identifier  string          `json:"identifier"`
	Title       string          `json:"title"`
	Creator     json.RawMessage `json:"creator"`
	Publisher   json.RawMessage `json:"publisher"`
	Date        string          `json:"date"`
	Language    json.RawMessage `json:"language"`
	Description json.RawMessage `json:"description"`
	ISBN        json.RawMessage `json:"isbn"`
}

func jsonStrings(raw json.RawMessage) []string {
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		seen := map[string]bool{}
		out := make([]string, 0, len(arr))
		for _, value := range arr {
			value = strings.TrimSpace(value)
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			out = append(out, value)
		}
		return out
	}
	var scalar string
	if err := json.Unmarshal(raw, &scalar); err == nil && strings.TrimSpace(scalar) != "" {
		return []string{strings.TrimSpace(scalar)}
	}
	return nil
}

func firstJSONString(raw json.RawMessage) string {
	values := jsonStrings(raw)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (i iaItem) toMatch(baseURL string) metadata.Match {
	desc := ""
	if descriptions := jsonStrings(i.Description); len(descriptions) > 0 {
		desc = htmlText(strings.Join(descriptions, "\n\n"))
	}
	cover := ""
	if i.Identifier != "" {
		cover = fmt.Sprintf("%s/services/img/%s", baseURL, url.PathEscape(i.Identifier))
	}
	return metadata.Match{
		Provider:    internetArchiveID,
		ProviderID:  i.Identifier,
		Title:       i.Title,
		Authors:     jsonStrings(i.Creator),
		Description: desc,
		Publisher:   firstJSONString(i.Publisher),
		PublishYear: firstYear(i.Date),
		ISBN:        metadata.NormalizeISBN(firstJSONString(i.ISBN)),
		Language:    firstJSONString(i.Language),
		CoverURL:    cover,
	}
}

type WorldCatClient struct {
	http catalogHTTPClient
}

func NewWorldCatClient(userAgent string) *WorldCatClient {
	return NewWorldCatClientAt("https://www.worldcat.org", userAgent)
}

func NewWorldCatClientAt(baseURL, userAgent string) *WorldCatClient {
	return &WorldCatClient{http: newCatalogHTTPClient(baseURL, userAgent)}
}

func (c *WorldCatClient) ID() string { return worldCatID }

func (c *WorldCatClient) Search(ctx context.Context, q metadata.SearchQuery) ([]metadata.Match, error) {
	query := strings.TrimSpace(sourceQueryText(q))
	if query == "" {
		return nil, nil
	}
	body, status, err := c.http.get(ctx, fmt.Sprintf("%s/search?q=%s", c.http.baseURL, url.QueryEscape(query)))
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	matches := parseWorldCatSearchPage(body)
	for i := range matches {
		matches[i].Provider = worldCatID
	}
	return matches, nil
}

func (c *WorldCatClient) Fetch(ctx context.Context, id string) (*metadata.Match, error) {
	id = strings.TrimSpace(id)
	endpoint := ""
	if path, ok := strings.CutPrefix(id, "path:"); ok && fetchableRelativePath(path) {
		endpoint = c.http.baseURL + path
	} else {
		id = compactISBN(id)
		if catalogISBNRE.MatchString(id) {
			endpoint = fmt.Sprintf("%s/isbn/%s", c.http.baseURL, url.PathEscape(id))
		}
	}
	if endpoint == "" {
		return nil, nil
	}
	body, status, err := c.http.get(ctx, endpoint)
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	match := parseWorldCatRecordPage(body)
	if match == nil {
		return nil, nil
	}
	match.Provider = worldCatID
	match.ProviderID = id
	if catalogISBNRE.MatchString(id) {
		match.ISBN = metadata.NormalizeISBN(id)
	}
	return match, nil
}

var (
	wcBlockRE       = regexp.MustCompile(`(?is)<div[^>]*class="[^"]*(?:result|record)[^"]*"[^>]*>.*?</div>\s*</div>`)
	wcTitleRE       = regexp.MustCompile(`(?is)<a[^>]*class="[^"]*title[^"]*"[^>]*href="([^"]+)"[^>]*>([^<]+)</a>`)
	wcHeadingRE     = regexp.MustCompile(`(?is)<h[234][^>]*>([^<]+)</h[234]>`)
	wcAuthorByRE    = regexp.MustCompile(`(?is)by\s+<[^>]*>([^<]+)</a>`)
	wcAuthorSpanRE  = regexp.MustCompile(`(?is)<span[^>]*class="[^"]*author[^"]*"[^>]*>([^<]+)</span>`)
	wcYearRE        = regexp.MustCompile(`\b(19|20)\d{2}\b`)
	wcLanguageRE    = regexp.MustCompile(`(?i)Language:\s*([^<]+)`)
	wcH1RE          = regexp.MustCompile(`(?is)<h1[^>]*class="[^"]*title[^"]*"[^>]*>([^<]+)</h1>`)
	wcOGTitleRE     = regexp.MustCompile(`(?i)<meta\s+property="og:title"\s+content="([^"]+)"`)
	wcAuthorRE      = regexp.MustCompile(`(?is)<a[^>]*href="[^"]*/author/[^"]*"[^>]*>([^<]+)</a>`)
	wcPublisherRE   = regexp.MustCompile(`(?i)Publisher:\s*([^<]+)`)
	wcYearLabelRE   = regexp.MustCompile(`(?i)(?:Year|Date):\s*(\d{4})`)
	wcSummaryRE     = regexp.MustCompile(`(?is)<div[^>]*id="[^"]*summary[^"]*"[^>]*>(.*?)</div>`)
	wcDescriptionRE = regexp.MustCompile(`(?is)<div[^>]*class="[^"]*description[^"]*"[^>]*>(.*?)</div>`)
	wcCoverRE       = regexp.MustCompile(`(?is)<img[^>]*class="[^"]*cover[^"]*"[^>]*src="([^"]+)"`)
	wcOGCoverRE     = regexp.MustCompile(`(?i)<meta\s+property="og:image"\s+content="([^"]+)"`)
)

func parseWorldCatSearchPage(html []byte) []metadata.Match {
	blocks := wcBlockRE.FindAllString(string(html), -1)
	out := make([]metadata.Match, 0, len(blocks))
	for _, block := range blocks {
		tm := wcTitleRE.FindStringSubmatch(block)
		title := ""
		providerID := ""
		if len(tm) >= 3 {
			title = htmlText(tm[2])
			if fetchableRelativePath(tm[1]) {
				providerID = "path:" + tm[1]
			}
		}
		if title == "" {
			title = htmlText(firstSubmatch(wcHeadingRE, block))
		}
		if title == "" || providerID == "" {
			continue
		}
		match := metadata.Match{ProviderID: providerID, Title: title, PublishYear: firstYear(wcYearRE.FindString(block))}
		if author := htmlText(firstSubmatch(wcAuthorByRE, block)); author != "" {
			match.Authors = []string{author}
		} else if author := htmlText(firstSubmatch(wcAuthorSpanRE, block)); author != "" {
			match.Authors = []string{author}
		}
		match.Language = htmlText(firstSubmatch(wcLanguageRE, block))
		out = append(out, match)
	}
	return out
}

func parseWorldCatRecordPage(html []byte) *metadata.Match {
	s := string(html)
	title := htmlText(firstNonEmpty(firstSubmatch(wcH1RE, s), firstSubmatch(wcOGTitleRE, s)))
	if title == "" {
		return nil
	}
	match := &metadata.Match{Title: title}
	seen := map[string]bool{}
	for _, am := range wcAuthorRE.FindAllStringSubmatch(s, -1) {
		if len(am) < 2 {
			continue
		}
		name := htmlText(am[1])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		match.Authors = append(match.Authors, name)
		if len(match.Authors) >= 5 {
			break
		}
	}
	match.Publisher = htmlText(firstSubmatch(wcPublisherRE, s))
	if match.PublishYear = firstYear(firstSubmatch(wcYearLabelRE, s)); match.PublishYear == 0 {
		match.PublishYear = firstYear(wcYearRE.FindString(s))
	}
	match.Language = htmlText(firstSubmatch(wcLanguageRE, s))
	match.Description = htmlText(firstNonEmpty(firstSubmatch(wcSummaryRE, s), firstSubmatch(wcDescriptionRE, s)))
	match.CoverURL = firstNonEmpty(firstSubmatch(wcCoverRE, s), firstSubmatch(wcOGCoverRE, s))
	return match
}

type DoubanClient struct {
	http catalogHTTPClient
}

func NewDoubanClient(userAgent string) *DoubanClient {
	return NewDoubanClientAt("https://book.douban.com", userAgent)
}

func NewDoubanClientAt(baseURL, userAgent string) *DoubanClient {
	return &DoubanClient{http: newCatalogHTTPClient(baseURL, userAgent)}
}

func (c *DoubanClient) ID() string { return doubanID }

func (c *DoubanClient) Search(ctx context.Context, q metadata.SearchQuery) ([]metadata.Match, error) {
	query := strings.TrimSpace(sourceQueryText(q))
	if query == "" {
		return nil, nil
	}
	body, status, err := c.http.get(ctx, fmt.Sprintf("%s/subject_search?search_text=%s", c.http.baseURL, url.QueryEscape(query)))
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	matches := parseDoubanSearchPage(body)
	for i := range matches {
		matches[i].Provider = doubanID
		matches[i].Language = "zh"
	}
	return matches, nil
}

func (c *DoubanClient) Fetch(ctx context.Context, id string) (*metadata.Match, error) {
	id = strings.TrimSpace(id)
	if !catalogNumericRE.MatchString(id) {
		return nil, nil
	}
	body, status, err := c.http.get(ctx, fmt.Sprintf("%s/subject/%s/", c.http.baseURL, url.PathEscape(id)))
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	match := parseDoubanSubjectPage(body)
	if match == nil {
		return nil, nil
	}
	match.Provider = doubanID
	match.ProviderID = id
	match.Language = "zh"
	return match, nil
}

var (
	dbDataRE        = regexp.MustCompile(`(?s)window\.__DATA__\s*=\s*(\{.*?\});`)
	dbSubjectRE     = regexp.MustCompile(`/subject/(\d+)/`)
	dbTitleRE       = regexp.MustCompile(`(?is)<span\s+property="v:itemreviewed"[^>]*>([^<]+)</span>`)
	dbTitleFBRE     = regexp.MustCompile(`(?is)<title>\s*([^<]+?)\s*</title>`)
	dbInfoRE        = regexp.MustCompile(`(?is)<div\s+id="info"[^>]*>(.*?)</div>`)
	dbAuthorBlockRE = regexp.MustCompile(`(?is)作者\s*[:：]\s*((?:<a[^>]*>[^<]+</a>\s*/?\s*)+)`)
	dbAnchorRE      = regexp.MustCompile(`(?is)<a[^>]*>([^<]+)</a>`)
	dbPublisherRE   = regexp.MustCompile(`^\s*出版社\s*[:：]\s*(.+?)\s*$`)
	dbYearInfoRE    = regexp.MustCompile(`^\s*出版年\s*[:：]\s*(.+?)\s*$`)
	dbISBNInfoRE    = regexp.MustCompile(`^\s*ISBN\s*[:：]\s*([\dXx-]+)\s*$`)
	dbSeriesInfoRE  = regexp.MustCompile(`^\s*丛书\s*[:：]\s*(.+?)\s*$`)
	dbDescRE        = regexp.MustCompile(`(?is)<span\s+class="all\s+hidden"[^>]*>(.*?)</span>`)
	dbIntroRE       = regexp.MustCompile(`(?is)<div\s+class="intro"[^>]*>(.*?)</div>`)
	dbCoverRE       = regexp.MustCompile(`(?is)<a[^>]+class="nbg"[^>]*>\s*<img[^>]+src="([^"]+)"`)
	dbCoverFBRE     = regexp.MustCompile(`(?is)<div\s+id="mainpic"[^>]*>.*?<img[^>]+src="([^"]+)"`)
	dbBRRE          = regexp.MustCompile(`(?i)<br\s*/?>`)
)

type dbSearchData struct {
	Items []struct {
		Title    string `json:"title"`
		URL      string `json:"url"`
		CoverURL string `json:"cover_url"`
		Abstract string `json:"abstract"`
	} `json:"items"`
}

func parseDoubanSearchPage(html []byte) []metadata.Match {
	m := dbDataRE.FindSubmatch(html)
	if len(m) < 2 {
		return nil
	}
	var data dbSearchData
	if err := json.Unmarshal(m[1], &data); err != nil {
		return nil
	}
	items := data.Items
	if len(items) > 10 {
		items = items[:10]
	}
	out := make([]metadata.Match, 0, len(items))
	for _, item := range items {
		title := htmlText(item.Title)
		if title == "" {
			continue
		}
		match := metadata.Match{Title: title, CoverURL: item.CoverURL}
		if sm := dbSubjectRE.FindStringSubmatch(item.URL); len(sm) >= 2 {
			match.ProviderID = sm[1]
		}
		if match.ProviderID == "" {
			continue
		}
		parts := strings.Split(item.Abstract, " / ")
		cleaned := make([]string, 0, len(parts))
		for _, part := range parts {
			if part = strings.TrimSpace(part); part != "" {
				cleaned = append(cleaned, part)
			}
		}
		if len(cleaned) >= 4 {
			match.Authors = cleaned[:len(cleaned)-3]
			match.Publisher = cleaned[len(cleaned)-3]
			match.PublishYear = firstYear(cleaned[len(cleaned)-2])
		} else if len(cleaned) >= 1 {
			match.Authors = cleaned[:1]
		}
		out = append(out, match)
	}
	return out
}

func parseDoubanSubjectPage(html []byte) *metadata.Match {
	s := string(html)
	title := htmlText(firstSubmatch(dbTitleRE, s))
	if title == "" {
		title = strings.TrimSuffix(htmlText(firstSubmatch(dbTitleFBRE, s)), " (豆瓣)")
	}
	if title == "" {
		return nil
	}
	match := &metadata.Match{Title: title}
	info := firstSubmatch(dbInfoRE, s)
	authorInfo := info
	if idx := strings.Index(authorInfo, "出版社"); idx >= 0 {
		authorInfo = authorInfo[:idx]
	}
	if am := dbAuthorBlockRE.FindStringSubmatch(authorInfo); len(am) >= 2 {
		authorInfo = am[1]
	}
	for _, anchor := range dbAnchorRE.FindAllStringSubmatch(authorInfo, -1) {
		if len(anchor) >= 2 {
			if name := htmlText(anchor[1]); name != "" {
				match.Authors = append(match.Authors, name)
			}
		}
	}
	if len(match.Authors) == 0 {
		for _, anchor := range dbAnchorRE.FindAllStringSubmatch(info, -1) {
			if len(anchor) >= 2 {
				if name := htmlText(anchor[1]); name != "" {
					match.Authors = append(match.Authors, name)
					break
				}
			}
		}
	}
	for _, line := range doubanInfoLines(info) {
		if match.Publisher == "" {
			match.Publisher = htmlText(firstSubmatch(dbPublisherRE, line))
		}
		if match.PublishYear == 0 {
			match.PublishYear = firstYear(firstSubmatch(dbYearInfoRE, line))
		}
		if match.ISBN == "" {
			match.ISBN = metadata.NormalizeISBN(firstSubmatch(dbISBNInfoRE, line))
		}
		if match.SeriesName == "" {
			match.SeriesName = htmlText(firstSubmatch(dbSeriesInfoRE, line))
		}
	}
	match.Description = htmlText(firstNonEmpty(firstSubmatch(dbDescRE, s), firstSubmatch(dbIntroRE, s)))
	match.CoverURL = firstNonEmpty(firstSubmatch(dbCoverRE, s), firstSubmatch(dbCoverFBRE, s))
	return match
}

func doubanInfoLines(info string) []string {
	parts := strings.Split(dbBRRE.ReplaceAllString(info, "\n"), "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if line := htmlText(part); line != "" {
			out = append(out, line)
		}
	}
	return out
}
