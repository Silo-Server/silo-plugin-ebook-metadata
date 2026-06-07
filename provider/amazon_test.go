package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Silo-Server/silo-plugin-ebook-metadata/metadata"
)

func newAmazonFake(t *testing.T) (*httptest.Server, *AmazonClient, *int) {
	t.Helper()
	book := loadProviderFixture(t, "amazon_book.html")
	requests := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch {
		case strings.HasPrefix(r.URL.Path, "/dp/B08G9PRS1K"):
			w.Header().Set("Content-Type", "text/html")
			w.Write(book)
		case strings.HasPrefix(r.URL.Path, "/dp/"):
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	client := NewAmazonClientAt(srv.URL, "test-agent")
	client.client = srv.Client()
	return srv, client, &requests
}

func TestAmazonFetchByAmazonID(t *testing.T) {
	srv, client, _ := newAmazonFake(t)
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "B08G9PRS1K")
	if err != nil {
		t.Fatal(err)
	}
	if match == nil {
		t.Fatal("Fetch() returned nil")
	}
	if match.Provider != "amazon" || match.ProviderID != "B08G9PRS1K" {
		t.Fatalf("provider fields = %#v", match)
	}
	if match.Title != "Project Hail Mary" || match.ISBN != "9780593135204" || match.PublishYear != 2021 {
		t.Fatalf("mapped fields = %#v", match)
	}
	wantAuthors := []string{"Andy Weir", "Mary Robinette Kowal"}
	if len(match.Authors) != len(wantAuthors) {
		t.Fatalf("Authors = %#v, want %#v", match.Authors, wantAuthors)
	}
	for i, want := range wantAuthors {
		if match.Authors[i] != want {
			t.Fatalf("Authors[%d] = %q, want %q", i, match.Authors[i], want)
		}
	}
	if match.Publisher != "Ballantine Books" || match.Language != "English" || match.PageCount != 476 {
		t.Fatalf("mapped detail fields = %#v", match)
	}
	if match.CoverURL == "" || match.Description == "" {
		t.Fatalf("expected cover and description: %#v", match)
	}
}

func TestAmazonFetchMissing(t *testing.T) {
	srv, client, _ := newAmazonFake(t)
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "B000000000")
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil", err)
	}
	if match != nil {
		t.Fatalf("Fetch() = %#v, want nil", match)
	}
}

func TestAmazonNonIDFetchReturnsNilWithoutNetwork(t *testing.T) {
	srv, client, requests := newAmazonFake(t)
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "9780593135204")
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil", err)
	}
	if match != nil {
		t.Fatalf("Fetch() = %#v, want nil", match)
	}
	if *requests != 0 {
		t.Fatalf("Fetch() made %d requests for nonmatching ID, want 0", *requests)
	}
}

func TestAmazonTextSearchReturnsNilWithoutNetwork(t *testing.T) {
	srv, client, requests := newAmazonFake(t)
	defer srv.Close()

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "Project Hail Mary", Authors: []string{"Andy Weir"}})
	if err != nil {
		t.Fatalf("Search() error = %v, want nil", err)
	}
	if matches != nil {
		t.Fatalf("Search() = %#v, want nil", matches)
	}
	if *requests != 0 {
		t.Fatalf("Search() made %d requests for text query, want 0", *requests)
	}
}

func TestAmazonSearchIgnoresSourceShapedTextWithoutProviderID(t *testing.T) {
	srv, client, _ := newAmazonFake(t)
	defer srv.Close()

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "B08G9PRS1K"})
	if err != nil {
		t.Fatalf("Search() error = %v, want nil", err)
	}
	if matches != nil {
		t.Fatalf("Search() = %#v, want nil", matches)
	}
}

func TestAmazonSearchDelegatesForExplicitAmazonID(t *testing.T) {
	srv, client, _ := newAmazonFake(t)
	defer srv.Close()

	matches, err := client.Search(context.Background(), metadata.SearchQuery{
		ProviderIDs: map[string]string{"amazon": "B08G9PRS1K"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("Search() returned %d matches, want 1", len(matches))
	}
	if matches[0].ProviderID != "B08G9PRS1K" {
		t.Fatalf("ProviderID = %q, want B08G9PRS1K", matches[0].ProviderID)
	}
}

func TestAmazonSearchDelegatesForCapabilityProviderID(t *testing.T) {
	srv, client, _ := newAmazonFake(t)
	defer srv.Close()

	matches, err := client.Search(context.Background(), metadata.SearchQuery{
		ProviderIDs: map[string]string{metadata.CapabilityID: "amazon:B08G9PRS1K"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("Search() returned %d matches, want 1", len(matches))
	}
	if matches[0].ProviderID != "B08G9PRS1K" {
		t.Fatalf("ProviderID = %q, want B08G9PRS1K", matches[0].ProviderID)
	}
}

func TestAmazonAuthorParsingRequiresAuthorRole(t *testing.T) {
	html := []byte(`
		<span id="productTitle">Role Test</span>
		<span class="author notFaded">
			<a class="a-link-normal" href="/writer">Writer One</a>
			<span class="a-color-secondary">(Author)</span>
		</span>
		<span class="author notFaded">
			<a class="a-link-normal" href="/speaker">Speaker One</a>
			<span class="a-color-secondary">(Performer)</span>
		</span>
	`)

	match := parseAmazonProductPage(html)
	if match == nil {
		t.Fatal("parseAmazonProductPage() returned nil")
	}
	if len(match.Authors) != 1 || match.Authors[0] != "Writer One" {
		t.Fatalf("Authors = %#v, want only Writer One", match.Authors)
	}
}
