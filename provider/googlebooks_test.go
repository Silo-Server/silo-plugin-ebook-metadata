package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Silo-Server/silo-plugin-ebook-metadata/metadata"
)

func newGoogleBooksFake(t *testing.T, apiKey string) (*httptest.Server, *GoogleBooksClient, *int) {
	t.Helper()
	volume := loadProviderFixture(t, "googlebooks_volume.json")
	search := loadProviderFixture(t, "googlebooks_search.json")
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch {
		case strings.HasPrefix(r.URL.Path, "/volumes/zyTCAlFPjgYC"):
			w.Write(volume)
		case strings.HasPrefix(r.URL.Path, "/volumes/notfoundXXXX"):
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == "/volumes":
			w.Write(search)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	client := NewGoogleBooksClientAt(srv.URL, apiKey, "test-agent")
	client.client = srv.Client()
	return srv, client, &requests
}

func TestGoogleBooksFetchByID(t *testing.T) {
	srv, client, _ := newGoogleBooksFake(t, "test-key")
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "zyTCAlFPjgYC")
	if err != nil {
		t.Fatal(err)
	}
	if match == nil {
		t.Fatal("Fetch() returned nil")
	}
	if match.Provider != "googlebooks" || match.ProviderID != "zyTCAlFPjgYC" {
		t.Fatalf("provider fields = %#v", match)
	}
	if match.Title != "Project Hail Mary" || match.ISBN != "9780593135204" || match.PublishYear != 2021 {
		t.Fatalf("mapped fields = %#v", match)
	}
	if match.CoverURL == "" || !strings.HasPrefix(match.CoverURL, "https://") {
		t.Fatalf("CoverURL = %q, want https URL", match.CoverURL)
	}
}

func TestGoogleBooksFetchMissing(t *testing.T) {
	srv, client, _ := newGoogleBooksFake(t, "test-key")
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "notfoundXXXX")
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil", err)
	}
	if match != nil {
		t.Fatalf("Fetch() = %#v, want nil", match)
	}
}

func TestGoogleBooksSearchByText(t *testing.T) {
	srv, client, _ := newGoogleBooksFake(t, "test-key")
	defer srv.Close()

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "Project Hail Mary", Authors: []string{"Andy Weir"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("Search() returned %d matches, want 1", len(matches))
	}
	if matches[0].Title != "Project Hail Mary" || matches[0].ProviderID != "zyTCAlFPjgYC" {
		t.Fatalf("Search()[0] = %#v", matches[0])
	}
}

func TestGoogleBooksSearchByISBN(t *testing.T) {
	srv, client, _ := newGoogleBooksFake(t, "test-key")
	defer srv.Close()
	var sawISBNPrefix bool
	transport := client.client.Transport
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("q") == "isbn:9780593135204" {
			sawISBNPrefix = true
		}
		return transport.RoundTrip(req)
	})

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "978-0-593-13520-4"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("Search() returned %d matches, want 1", len(matches))
	}
	if !sawISBNPrefix {
		t.Fatal("Search() did not prefix ISBN query with isbn:")
	}
}

func TestGoogleBooksUnauthenticatedAllowed(t *testing.T) {
	srv, client, requests := newGoogleBooksFake(t, "")
	defer srv.Close()

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "Project Hail Mary"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("Search() returned %d matches, want 1", len(matches))
	}
	if *requests == 0 {
		t.Fatal("Search() made no request without API key")
	}
}

func TestGoogleBooksFetchRejectsInvalidVolumeID(t *testing.T) {
	srv, client, requests := newGoogleBooksFake(t, "test-key")
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "not a volume id")
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil", err)
	}
	if match != nil {
		t.Fatalf("Fetch() = %#v, want nil", match)
	}
	if *requests != 0 {
		t.Fatalf("Fetch() made %d requests for invalid ID, want 0", *requests)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
