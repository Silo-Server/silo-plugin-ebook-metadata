package provider

import (
	"context"
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

func TestFantasticFictionSearchOnly(t *testing.T) {
	search := loadProviderFixture(t, "fantasticfiction_search.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write(search)
	}))
	defer srv.Close()
	client := NewFantasticFictionClientAt(srv.URL, "test")
	client.http.client = srv.Client()

	match, err := client.Fetch(context.Background(), "anything")
	if err != nil || match != nil {
		t.Fatalf("Fetch() = %#v/%v, want nil/nil", match, err)
	}
	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "project hail mary"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 || matches[0].Title != "Project Hail Mary" || matches[2].SeriesName != "Dune" {
		t.Fatalf("Search() = %#v", matches)
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
		case r.URL.Path == "/work/1234":
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
	selected, err := client.Fetch(context.Background(), matches[0].ProviderID)
	if err != nil {
		t.Fatal(err)
	}
	if selected == nil || selected.ProviderID != "work:1234" || selected.Title != "Dune" {
		t.Fatalf("Fetch(search ProviderID) = %#v", selected)
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
	if len(matches) != 2 || matches[0].ProviderID != "path:/oclc/rec" || matches[1].Title != "Dune Messiah" || matches[1].PublishYear != 1969 {
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
