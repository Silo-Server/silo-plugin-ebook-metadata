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
	amazonID      = "amazon"
	amazonBaseURL = "https://www.amazon.com"
)

var (
	amazonSourceIDRE = regexp.MustCompile(`^[A-Z0-9]{10}$`)
	amTitleRE        = regexp.MustCompile(`(?is)<span[^>]*\bid="productTitle"[^>]*>([^<]+)</span>`)
	amAuthorsBlockRE = regexp.MustCompile(`(?is)<span[^>]*\bclass="[^"]*\bauthor\b[^"]*"[^>]*>([\s\S]*?)</span>`)
	amAuthorLinkRE   = regexp.MustCompile(`(?is)<a[^>]*\bclass="[^"]*\ba-link-normal\b[^"]*"[^>]*>([^<]+)</a>`)
	amDescNoscriptRE = regexp.MustCompile(`(?is)<div[^>]*\bid="bookDescription_feature_div"[^>]*>[\s\S]*?<noscript>([\s\S]*?)</noscript>`)
	amDescSpanRE     = regexp.MustCompile(`(?is)<div[^>]*\bid="bookDescription_feature_div"[^>]*>[\s\S]*?<span[^>]*>([\s\S]*?)</span>`)
	amCoverREs       = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<img[^>]*\bid="imgBlkFront"[^>]*\bsrc="([^"]+)"`),
		regexp.MustCompile(`(?is)<img[^>]*\bid="ebooksImgBlkFront"[^>]*\bsrc="([^"]+)"`),
		regexp.MustCompile(`(?is)<img[^>]*\bid="main-image"[^>]*\bsrc="([^"]+)"`),
	}
	amDetailLiRE  = regexp.MustCompile(`(?is)<li\b[^>]*>([\s\S]*?)</li>`)
	amPublisherRE = regexp.MustCompile(`(?i)Publisher[:\s]+(.+?)\s*\(`)
	amPubDateRE   = regexp.MustCompile(`\(([^)]+)\)`)
	amLanguageRE  = regexp.MustCompile(`(?i)Language[:\s]+(.+)`)
	amPagesRE     = regexp.MustCompile(`(\d+)\s+pages`)
	amISBN10RE    = regexp.MustCompile(`(?i)ISBN-10[:\s]+(\d{10})`)
	amISBN13RE    = regexp.MustCompile(`(?i)ISBN-13[:\s]+([\d-]+)`)
	amTagStripRE  = regexp.MustCompile(`(?is)<[^>]+>`)
	amWSRE        = regexp.MustCompile(`\s+`)
)

type AmazonClient struct {
	baseURL   string
	client    *http.Client
	userAgent string
}

func NewAmazonClient(userAgent string) *AmazonClient {
	return NewAmazonClientAt(amazonBaseURL, userAgent)
}

func NewAmazonClientAt(baseURL, userAgent string) *AmazonClient {
	return &AmazonClient{
		baseURL:   strings.TrimRight(baseURL, "/"),
		client:    http.DefaultClient,
		userAgent: userAgent,
	}
}

func (c *AmazonClient) ID() string {
	return amazonID
}

func (c *AmazonClient) Fetch(ctx context.Context, id string) (*metadata.Match, error) {
	id = strings.TrimSpace(id)
	if !amazonSourceIDRE.MatchString(id) {
		return nil, nil
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/dp/%s", c.baseURL, url.PathEscape(id)), nil)
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

	match := parseAmazonProductPage(body)
	if match == nil {
		return nil, nil
	}
	match.Provider = amazonID
	match.ProviderID = id
	return match, nil
}

func (c *AmazonClient) Search(ctx context.Context, q metadata.SearchQuery) ([]metadata.Match, error) {
	id := strings.TrimSpace(q.ProviderIDs[amazonID])
	if id == "" {
		sourceID, providerID := metadata.ParseCapabilityProviderID(q.ProviderIDs[metadata.CapabilityID])
		if sourceID == amazonID {
			id = providerID
		}
	}
	if !amazonSourceIDRE.MatchString(id) {
		return nil, nil
	}
	match, err := c.Fetch(ctx, id)
	if err != nil || match == nil {
		return nil, err
	}
	return []metadata.Match{*match}, nil
}

func parseAmazonProductPage(html []byte) *metadata.Match {
	s := string(html)
	title := amStripText(amFirstSubmatch(amTitleRE, s))
	if title == "" {
		return nil
	}
	match := &metadata.Match{Title: title}

	for _, block := range amAuthorsBlockRE.FindAllStringSubmatch(s, -1) {
		if len(block) < 2 {
			continue
		}
		if !strings.Contains(strings.ToLower(amStripText(block[1])), "(author)") {
			continue
		}
		for _, m := range amAuthorLinkRE.FindAllStringSubmatch(block[1], -1) {
			if len(m) < 2 {
				continue
			}
			name := amStripText(m[1])
			if name == "" || strings.Contains(name, "(") {
				continue
			}
			match.Authors = append(match.Authors, name)
		}
	}
	if desc := amStripText(amFirstSubmatch(amDescNoscriptRE, s)); desc != "" {
		match.Description = desc
	} else if desc := amStripText(amFirstSubmatch(amDescSpanRE, s)); desc != "" {
		match.Description = desc
	}
	for _, re := range amCoverREs {
		if cover := strings.TrimSpace(amFirstSubmatch(re, s)); cover != "" {
			match.CoverURL = cover
			break
		}
	}
	for _, m := range amDetailLiRE.FindAllStringSubmatch(s, -1) {
		if len(m) < 2 {
			continue
		}
		text := amStripText(m[1])
		if text == "" {
			continue
		}
		if strings.Contains(text, "Publisher") {
			if match.Publisher == "" {
				if sm := amPublisherRE.FindStringSubmatch(text); len(sm) >= 2 {
					match.Publisher = strings.TrimSpace(sm[1])
				}
			}
			if match.PublishYear == 0 {
				if sm := amPubDateRE.FindStringSubmatch(text); len(sm) >= 2 {
					match.PublishYear = firstYear(sm[1])
				}
			}
		}
		if match.Language == "" && strings.Contains(text, "Language") {
			if sm := amLanguageRE.FindStringSubmatch(text); len(sm) >= 2 {
				match.Language = strings.TrimSpace(sm[1])
			}
		}
		if match.PageCount == 0 && (strings.Contains(text, "Paperback") || strings.Contains(text, "Hardcover") || strings.Contains(text, "pages")) {
			if sm := amPagesRE.FindStringSubmatch(text); len(sm) >= 2 {
				if n, err := strconv.Atoi(sm[1]); err == nil && n > 0 {
					match.PageCount = n
				}
			}
		}
		if strings.Contains(text, "ISBN-13") {
			if sm := amISBN13RE.FindStringSubmatch(text); len(sm) >= 2 {
				match.ISBN = metadata.NormalizeISBN(sm[1])
			}
		} else if match.ISBN == "" && strings.Contains(text, "ISBN-10") {
			if sm := amISBN10RE.FindStringSubmatch(text); len(sm) >= 2 {
				match.ISBN = metadata.NormalizeISBN(sm[1])
			}
		}
	}
	return match
}

func amFirstSubmatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func amStripText(s string) string {
	if s == "" {
		return ""
	}
	s = amTagStripRE.ReplaceAllString(s, " ")
	s = strings.NewReplacer(
		"&nbsp;", " ",
		"&amp;", "&",
		"&quot;", `"`,
		"&apos;", "'",
		"&#39;", "'",
		"&lt;", "<",
		"&gt;", ">",
	).Replace(s)
	s = amWSRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
