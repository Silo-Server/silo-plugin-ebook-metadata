package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Silo-Server/silo-plugin-ebook-metadata/metadata"
)

const bbValidID = "b1a2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"

func TestGutenbergFetchAndSearch(t *testing.T) {
	book := loadProviderFixture(t, "gutenberg_book.json")
	search := loadProviderFixture(t, "gutenberg_search.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/books/84":
			w.Write(book)
		case r.URL.Path == "/books":
			w.Write(search)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	client := NewGutenbergClientAt(srv.URL, "test")
	client.http.client = srv.Client()

	match, err := client.Fetch(context.Background(), "84")
	if err != nil {
		t.Fatal(err)
	}
	if match == nil || match.ProviderID != "84" || match.Title != "Frankenstein; Or, The Modern Prometheus" {
		t.Fatalf("Fetch() = %#v", match)
	}
	if len(match.Authors) != 1 || match.Authors[0] != "Shelley, Mary Wollstonecraft" {
		t.Fatalf("Authors = %#v", match.Authors)
	}
	if len(match.Genres) != 5 || match.Language != "en" || match.CoverURL == "" {
		t.Fatalf("mapped fields = %#v", match)
	}

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "frankenstein"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 || matches[0].Provider != gutenbergID || matches[0].ProviderID != "84" {
		t.Fatalf("Search() = %#v", matches)
	}
}

func TestBookBrainzFetchAndSearch(t *testing.T) {
	edition := loadProviderFixture(t, "bookbrainz_edition.json")
	search := loadProviderFixture(t, "bookbrainz_search.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/edition/"+bbValidID:
			w.Write(edition)
		case r.URL.Path == "/search":
			w.Write(search)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	client := NewBookBrainzClientAt(srv.URL, "test")
	client.http.client = srv.Client()

	match, err := client.Fetch(context.Background(), bbValidID)
	if err != nil {
		t.Fatal(err)
	}
	if match == nil || match.ProviderID != bbValidID || match.ISBN != "9780593135204" {
		t.Fatalf("Fetch() = %#v", match)
	}
	if match.Title != "Project Hail Mary" || match.Publisher != "Ballantine Books" || match.PublishYear != 2021 {
		t.Fatalf("mapped fields = %#v", match)
	}

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "project hail mary"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 || matches[0].Provider != bookBrainzID || matches[1].PublishYear != 2021 {
		t.Fatalf("Search() = %#v", matches)
	}
}

func TestBookBrainzSearchSkipsRowsWithoutFetchableID(t *testing.T) {
	html := []byte(`{"results":[{"bbid":"","defaultAlias":{"name":"No ID"}},{"bbid":"b1a2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d","defaultAlias":{"name":"Good ID"}}]}`)

	var resp struct {
		Results []bbEntity `json:"results"`
	}
	if err := json.Unmarshal(html, &resp); err != nil {
		t.Fatal(err)
	}
	var matches []metadata.Match
	for _, entity := range resp.Results {
		match := entity.toMatch()
		if match.Title != "" && catalogUUIDRE.MatchString(match.ProviderID) {
			matches = append(matches, match)
		}
	}
	if len(matches) != 1 || matches[0].ProviderID != bbValidID {
		t.Fatalf("filtered matches = %#v", matches)
	}
}

func TestFantasticFictionSearchOnly(t *testing.T) {
	book := loadProviderFixture(t, "fantasticfiction_book.html")
	search := loadProviderFixture(t, "fantasticfiction_search.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/":
			w.Write(search)
		case "/w/andy-weir/project-hail-mary.htm", "/w/andy-weir/the-martian.htm", "/h/frank-herbert/dune.htm":
			w.Write(book)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	client := NewFantasticFictionClientAt(srv.URL, "test")
	client.http.client = srv.Client()

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "project hail mary"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 || matches[0].ProviderID != "path:/w/andy-weir/project-hail-mary.htm" || matches[0].Title != "Project Hail Mary" || matches[2].SeriesName != "Dune" {
		t.Fatalf("Search() = %#v", matches)
	}
	for _, match := range matches {
		if match.ProviderID == "" {
			t.Fatalf("Search() emitted unfetchable row: %#v", match)
		}
	}
	for _, match := range matches {
		selected, err := client.Fetch(context.Background(), match.ProviderID)
		if err != nil {
			t.Fatal(err)
		}
		if selected == nil || selected.ProviderID != match.ProviderID {
			t.Fatalf("Fetch(%q) = %#v", match.ProviderID, selected)
		}
	}
	selected, err := client.Fetch(context.Background(), matches[0].ProviderID)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.Authors) != 1 || selected.Authors[0] != "Andy Weir" || selected.PublishYear != 2021 {
		t.Fatalf("selected mapped fields = %#v", selected)
	}
}

func TestFantasticFictionSearchSkipsProtocolRelativeLinks(t *testing.T) {
	html := []byte(`<div class="book"><a href="//example.com/book.htm">External</a> by <a>Someone</a> (2020)</div>`)
	if matches := parseFantasticFictionSearchPage(html); len(matches) != 0 {
		t.Fatalf("parseFantasticFictionSearchPage() = %#v, want no unfetchable rows", matches)
	}
}

func TestISFDBFetchAndSearch(t *testing.T) {
	title := loadProviderFixture(t, "isfdb_title.html")
	search := loadProviderFixture(t, "isfdb_search.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/cgi-bin/title.cgi" && r.URL.RawQuery == "1655":
			w.Write(title)
		case r.URL.Path == "/cgi-bin/se.cgi":
			w.Write(search)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	client := NewISFDBClientAt(srv.URL, "test")
	client.http.client = srv.Client()

	match, err := client.Fetch(context.Background(), "1655")
	if err != nil {
		t.Fatal(err)
	}
	if match == nil || match.Title != "Dune" || match.ISBN != "0441172660" || match.PageCount != 517 {
		t.Fatalf("Fetch() = %#v", match)
	}
	if len(match.Authors) != 1 || match.Authors[0] != "Frank Herbert" || match.SeriesPosition != "1" {
		t.Fatalf("mapped fields = %#v", match)
	}

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "dune"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 || matches[2].Title != "Children of Dune" || len(matches[2].Authors) != 2 {
		t.Fatalf("Search() = %#v", matches)
	}
}

func TestLibraryThingFetchAndSearch(t *testing.T) {
	work := loadProviderFixture(t, "librarything_work.html")
	search := loadProviderFixture(t, "librarything_search.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/isbn/9780441172665":
			w.Write(work)
		case r.URL.Path == "/work/1234", r.URL.Path == "/work/5678", r.URL.Path == "/work/9012":
			w.Write(work)
		case r.URL.Path == "/search.php":
			w.Write(search)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	client := NewLibraryThingClientAt(srv.URL, "test")
	client.http.client = srv.Client()

	match, err := client.Fetch(context.Background(), "978-0-441-17266-5")
	if err != nil {
		t.Fatal(err)
	}
	if match == nil || match.Title != "Dune" || match.ISBN != "9780441172665" || match.SeriesName != "Dune" {
		t.Fatalf("Fetch() = %#v", match)
	}
	if !strings.Contains(match.Description, "Paul Atreides") || strings.Contains(match.Description, "<br") {
		t.Fatalf("Description = %q", match.Description)
	}

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "dune"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 || matches[0].ProviderID != "work:1234" || matches[0].PublishYear != 1965 || matches[2].Title != "Children of Dune" {
		t.Fatalf("Search() = %#v", matches)
	}
	for _, match := range matches {
		selected, err := client.Fetch(context.Background(), match.ProviderID)
		if err != nil {
			t.Fatal(err)
		}
		if selected == nil || selected.ProviderID != match.ProviderID {
			t.Fatalf("Fetch(%q) = %#v", match.ProviderID, selected)
		}
	}
}

func TestInternetArchiveFetchAndSearch(t *testing.T) {
	meta := loadProviderFixture(t, "internetarchive_metadata.json")
	search := loadProviderFixture(t, "internetarchive_search.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/metadata/frankenstein00mary":
			w.Write(meta)
		case r.URL.Path == "/advancedsearch.php":
			w.Write(search)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	client := NewInternetArchiveClientAt(srv.URL, "test")
	client.http.client = srv.Client()

	match, err := client.Fetch(context.Background(), "frankenstein00mary")
	if err != nil {
		t.Fatal(err)
	}
	if match == nil || match.ProviderID != "frankenstein00mary" || match.PublishYear != 1818 || match.ISBN != "" {
		t.Fatalf("Fetch() = %#v", match)
	}
	if strings.Contains(match.Description, "<") || !strings.Contains(match.CoverURL, "/services/img/frankenstein00mary") {
		t.Fatalf("mapped fields = %#v", match)
	}

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "frankenstein"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 || matches[1].ProviderID != "projecthailmary00weir" || matches[1].ISBN != "9780593135204" {
		t.Fatalf("Search() = %#v", matches)
	}
}

func TestInternetArchiveSearchSkipsRowsWithoutFetchableID(t *testing.T) {
	body := []byte(`{"response":{"docs":[{"title":"No ID"},{"identifier":"goodid","title":"Good ID"}]}}`)
	var resp struct {
		Response struct {
			Docs []iaItem `json:"docs"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	var matches []metadata.Match
	for _, item := range resp.Response.Docs {
		match := item.toMatch("https://example.test")
		if strings.TrimSpace(match.ProviderID) == "" {
			continue
		}
		matches = append(matches, match)
	}
	if len(matches) != 1 || matches[0].ProviderID != "goodid" {
		t.Fatalf("filtered matches = %#v", matches)
	}
}

func TestWorldCatFetchAndSearch(t *testing.T) {
	record := loadProviderFixture(t, "worldcat_record.html")
	search := loadProviderFixture(t, "worldcat_search.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/isbn/9780441172665":
			w.Write(record)
		case r.URL.Path == "/oclc/rec":
			w.Write(record)
		case r.URL.Path == "/search":
			w.Write(search)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	client := NewWorldCatClientAt(srv.URL, "test")
	client.http.client = srv.Client()

	match, err := client.Fetch(context.Background(), "978-0-441-17266-5")
	if err != nil {
		t.Fatal(err)
	}
	if match == nil || match.Title != "Dune" || match.Publisher != "Ace Books" || match.PublishYear != 1965 {
		t.Fatalf("Fetch() = %#v", match)
	}
	if len(match.Authors) != 2 || !strings.Contains(match.Description, "Arrakis") {
		t.Fatalf("mapped fields = %#v", match)
	}

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "dune"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ProviderID != "path:/oclc/rec" || matches[0].Title != "Dune" {
		t.Fatalf("Search() = %#v", matches)
	}
	selected, err := client.Fetch(context.Background(), matches[0].ProviderID)
	if err != nil {
		t.Fatal(err)
	}
	if selected == nil || selected.ProviderID != "path:/oclc/rec" || selected.Title != "Dune" {
		t.Fatalf("Fetch(search ProviderID) = %#v", selected)
	}
}

func TestWorldCatSearchSkipsProtocolRelativeLinks(t *testing.T) {
	html := []byte(`<div class="result"><div class="result-inner"><a class="title" href="//example.com/record">External</a> by <a>Someone</a> (2020)</div></div>`)
	if matches := parseWorldCatSearchPage(html); len(matches) != 0 {
		t.Fatalf("parseWorldCatSearchPage() = %#v, want no unfetchable rows", matches)
	}
}

func TestDoubanFetchAndSearch(t *testing.T) {
	subject := loadProviderFixture(t, "douban_subject.html")
	search := loadProviderFixture(t, "douban_search.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/subject/2567698"):
			w.Write(subject)
		case r.URL.Path == "/subject_search":
			w.Write(search)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	client := NewDoubanClientAt(srv.URL, "test")
	client.http.client = srv.Client()

	match, err := client.Fetch(context.Background(), "2567698")
	if err != nil {
		t.Fatal(err)
	}
	if match == nil || match.Title != "三体" || match.ISBN != "9787536692930" || match.Language != "zh" {
		t.Fatalf("Fetch() = %#v", match)
	}
	if len(match.Authors) != 1 || match.Publisher != "重庆出版社" || match.SeriesName != "中国科幻基石丛书" {
		t.Fatalf("mapped fields = %#v", match)
	}

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "三体"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 || matches[0].ProviderID != "2567698" || matches[1].Title != "三体II：黑暗森林" {
		t.Fatalf("Search() = %#v", matches)
	}
}

func TestDoubanSearchSkipsRowsWithoutFetchableID(t *testing.T) {
	html := []byte(`<script>window.__DATA__ = {"items":[{"title":"No ID","url":"https://example.test/nope"},{"title":"Good ID","url":"https://book.douban.com/subject/12345/"}]};</script>`)
	matches := parseDoubanSearchPage(html)
	if len(matches) != 1 || matches[0].ProviderID != "12345" {
		t.Fatalf("parseDoubanSearchPage() = %#v", matches)
	}
}
