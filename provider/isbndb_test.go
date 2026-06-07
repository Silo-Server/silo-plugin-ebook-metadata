package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Silo-Server/silo-plugin-ebook-metadata/metadata"
)

func newISBNdbFake(t *testing.T, apiKey string) (*httptest.Server, *ISBNdbClient, *int) {
	t.Helper()
	book := loadProviderFixture(t, "isbndb_book.json")
	search := loadProviderFixture(t, "isbndb_search.json")
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != apiKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/book/9780201616224"):
			w.Write(book)
		case strings.HasPrefix(r.URL.Path, "/book/9780000000000"):
			w.WriteHeader(http.StatusNotFound)
		case strings.HasPrefix(r.URL.Path, "/books/"):
			w.Write(search)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	client := NewISBNdbClientAt(srv.URL, apiKey, "test-agent")
	client.client = srv.Client()
	return srv, client, &requests
}

func TestISBNdbNoKeySearchFetchReturnNil(t *testing.T) {
	srv, client, requests := newISBNdbFake(t, "")
	defer srv.Close()

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "Pragmatic Programmer"})
	if err != nil || matches != nil {
		t.Fatalf("Search() = %#v, %v; want nil, nil", matches, err)
	}
	match, err := client.Fetch(context.Background(), "9780201616224")
	if err != nil || match != nil {
		t.Fatalf("Fetch() = %#v, %v; want nil, nil", match, err)
	}
	if *requests != 0 {
		t.Fatalf("made %d requests without key, want 0", *requests)
	}
}

func TestISBNdbFetchByISBN(t *testing.T) {
	srv, client, _ := newISBNdbFake(t, "test-key")
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "978-0-201-61622-4")
	if err != nil {
		t.Fatal(err)
	}
	if match == nil {
		t.Fatal("Fetch() returned nil")
	}
	if match.Provider != "isbndb" || match.ProviderID != "9780201616224" {
		t.Fatalf("provider fields = %#v", match)
	}
	if match.Title != "The Pragmatic Programmer" || match.ISBN != "9780201616224" || match.PublishYear != 2019 {
		t.Fatalf("mapped fields = %#v", match)
	}
	if len(match.Authors) != 2 || match.Authors[0] != "David Thomas" {
		t.Fatalf("Authors = %#v", match.Authors)
	}
	if match.PageCount != 352 || match.Description == "" || match.CoverURL == "" {
		t.Fatalf("mapped fields = %#v", match)
	}
}

func TestISBNdbFetchMissing(t *testing.T) {
	srv, client, _ := newISBNdbFake(t, "test-key")
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "9780000000000")
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil", err)
	}
	if match != nil {
		t.Fatalf("Fetch() = %#v, want nil", match)
	}
}

func TestISBNdbSearchByText(t *testing.T) {
	srv, client, _ := newISBNdbFake(t, "test-key")
	defer srv.Close()

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "Pragmatic Programmer"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("Search() returned %d matches, want 2", len(matches))
	}
	if matches[0].Provider != "isbndb" || matches[0].Title != "The Pragmatic Programmer" {
		t.Fatalf("Search()[0] = %#v", matches[0])
	}
}

func TestISBNdbFetchNonISBNReturnsNil(t *testing.T) {
	srv, client, requests := newISBNdbFake(t, "test-key")
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "not-an-isbn")
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil", err)
	}
	if match != nil {
		t.Fatalf("Fetch() = %#v, want nil", match)
	}
	if *requests != 0 {
		t.Fatalf("made %d requests for non-ISBN, want 0", *requests)
	}
}
