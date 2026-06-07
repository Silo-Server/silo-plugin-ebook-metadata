package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Silo-Server/silo-plugin-ebook-metadata/metadata"
)

func newGoodreadsFake(t *testing.T) (*httptest.Server, *GoodreadsClient, *int) {
	t.Helper()
	book := loadProviderFixture(t, "goodreads_book.html")
	search := loadProviderFixture(t, "goodreads_search.html")
	requests := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch {
		case strings.HasPrefix(r.URL.Path, "/book/show/54493401"):
			w.Header().Set("Content-Type", "text/html")
			w.Write(book)
		case strings.HasPrefix(r.URL.Path, "/book/show/"):
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == "/search":
			w.Header().Set("Content-Type", "text/html")
			w.Write(search)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	client := NewGoodreadsClientAt(srv.URL, "test-agent")
	client.client = srv.Client()
	return srv, client, &requests
}

func TestGoodreadsFetchByID(t *testing.T) {
	srv, client, _ := newGoodreadsFake(t)
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "54493401")
	if err != nil {
		t.Fatal(err)
	}
	if match == nil {
		t.Fatal("Fetch() returned nil")
	}
	if match.Provider != "goodreads" || match.ProviderID != "54493401" {
		t.Fatalf("provider fields = %#v", match)
	}
	if match.Title != "Project Hail Mary" || match.ISBN != "9780593135204" || match.PublishYear != 2021 {
		t.Fatalf("mapped fields = %#v", match)
	}
	if match.Publisher != "Ballantine Books" || match.PageCount != 476 {
		t.Fatalf("mapped detail fields = %#v", match)
	}
	if len(match.Authors) != 1 || match.Authors[0] != "Andy Weir" {
		t.Fatalf("Authors = %#v, want Andy Weir", match.Authors)
	}
	if match.CoverURL == "" || match.Description == "" {
		t.Fatalf("expected cover and description: %#v", match)
	}
}

func TestGoodreadsFetchMissing(t *testing.T) {
	srv, client, _ := newGoodreadsFake(t)
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "99999999")
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil", err)
	}
	if match != nil {
		t.Fatalf("Fetch() = %#v, want nil", match)
	}
}

func TestGoodreadsSearchByText(t *testing.T) {
	srv, client, _ := newGoodreadsFake(t)
	defer srv.Close()

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "Project Hail Mary", Authors: []string{"Andy Weir"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) < 1 {
		t.Fatalf("Search() returned %d matches, want at least 1", len(matches))
	}
	match := matches[0]
	if match.Provider != "goodreads" || match.ProviderID != "54493401" {
		t.Fatalf("provider fields = %#v", match)
	}
	if match.Title != "Project Hail Mary" {
		t.Fatalf("Title = %q, want Project Hail Mary", match.Title)
	}
	if len(match.Authors) != 1 || match.Authors[0] != "Andy Weir" {
		t.Fatalf("Authors = %#v, want Andy Weir", match.Authors)
	}
}

func TestGoodreadsNonNumericFetchReturnsNilWithoutNetwork(t *testing.T) {
	srv, client, requests := newGoodreadsFake(t)
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "project-hail-mary")
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil", err)
	}
	if match != nil {
		t.Fatalf("Fetch() = %#v, want nil", match)
	}
	if *requests != 0 {
		t.Fatalf("Fetch() made %d requests for nonnumeric ID, want 0", *requests)
	}
}
