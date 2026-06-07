package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Silo-Server/silo-plugin-ebook-metadata/metadata"
)

func newAnnasArchiveFake(t *testing.T) (*httptest.Server, *AnnasArchiveClient, *int) {
	t.Helper()
	book := loadProviderFixture(t, "annasarchive_book.html")
	audio := loadProviderFixture(t, "annasarchive_audiobook.html")
	search := loadProviderFixture(t, "annasarchive_search.html")
	requests := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch {
		case strings.HasPrefix(r.URL.Path, "/md5/a1b2c3d4e5f67890abcdef1234567890"):
			w.Header().Set("Content-Type", "text/html")
			w.Write(book)
		case strings.HasPrefix(r.URL.Path, "/md5/c3d4e5f67890abcdef1234567890abcd"):
			w.Header().Set("Content-Type", "text/html")
			w.Write(audio)
		case strings.HasPrefix(r.URL.Path, "/md5/"):
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == "/search":
			w.Header().Set("Content-Type", "text/html")
			w.Write(search)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	client := NewAnnasArchiveClientAt(srv.URL, "test-agent")
	client.client = srv.Client()
	return srv, client, &requests
}

func TestAnnasArchiveFetchByMD5(t *testing.T) {
	srv, client, _ := newAnnasArchiveFake(t)
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "a1b2c3d4e5f67890abcdef1234567890")
	if err != nil {
		t.Fatal(err)
	}
	if match == nil {
		t.Fatal("Fetch() returned nil")
	}
	if match.Provider != "annasarchive" || match.ProviderID != "a1b2c3d4e5f67890abcdef1234567890" {
		t.Fatalf("provider fields = %#v", match)
	}
	if match.Title != "Project Hail Mary" || match.ISBN != "9780593135204" || match.PublishYear != 2021 {
		t.Fatalf("mapped fields = %#v", match)
	}
	if len(match.Authors) != 1 || match.Authors[0] != "Andy Weir" {
		t.Fatalf("Authors = %#v, want Andy Weir", match.Authors)
	}
	if match.Publisher != "Ballantine Books" || match.Language != "en" || match.PageCount != 476 {
		t.Fatalf("mapped detail fields = %#v", match)
	}
	if match.CoverURL == "" || !strings.HasSuffix(match.CoverURL, "/cover/a1b2c3d4.jpg") {
		t.Fatalf("CoverURL = %q", match.CoverURL)
	}
	if match.Description == "" || !strings.Contains(match.Description, "Ryland Grace") {
		t.Fatalf("Description = %q", match.Description)
	}
	if strings.Contains(match.Description, "&#39;") {
		t.Fatalf("Description still contains raw numeric entity: %q", match.Description)
	}
}

func TestAnnasArchiveFetchMissing(t *testing.T) {
	srv, client, _ := newAnnasArchiveFake(t)
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "ffffffffffffffffffffffffffffffff")
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil", err)
	}
	if match != nil {
		t.Fatalf("Fetch() = %#v, want nil", match)
	}
}

func TestAnnasArchiveNonMD5FetchReturnsNilWithoutNetwork(t *testing.T) {
	srv, client, requests := newAnnasArchiveFake(t)
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "9780593135204")
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil", err)
	}
	if match != nil {
		t.Fatalf("Fetch() = %#v, want nil", match)
	}
	if *requests != 0 {
		t.Fatalf("Fetch() made %d requests for non-MD5 ID, want 0", *requests)
	}
}

func TestAnnasArchiveSearchFiltersAudiobookFormats(t *testing.T) {
	srv, client, _ := newAnnasArchiveFake(t)
	defer srv.Close()

	matches, err := client.Search(context.Background(), metadata.SearchQuery{Title: "Project Hail Mary", Authors: []string{"Andy Weir"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("Search() returned no matches")
	}

	const audioMD5 = "c3d4e5f67890abcdef1234567890abcd"
	for _, match := range matches {
		if match.ProviderID == audioMD5 {
			t.Fatalf("audio format row was not filtered: %#v", match)
		}
	}

	var ebook *metadata.Match
	for i := range matches {
		if matches[i].ProviderID == "a1b2c3d4e5f67890abcdef1234567890" {
			ebook = &matches[i]
			break
		}
	}
	if ebook == nil {
		t.Fatal("expected ebook row in results")
	}
	if ebook.Title != "Project Hail Mary" || ebook.ISBN != "9780593135204" || ebook.PublishYear != 2021 {
		t.Fatalf("ebook row = %#v", ebook)
	}
	if ebook.Publisher != "Ballantine Books" || ebook.Language != "en" {
		t.Fatalf("ebook detail fields = %#v", ebook)
	}
	if len(ebook.Authors) != 1 || ebook.Authors[0] != "Andy Weir" {
		t.Fatalf("Authors = %#v, want Andy Weir", ebook.Authors)
	}

	const unknownMD5 = "d4e5f67890abcdef1234567890abcdef"
	var unknown *metadata.Match
	for i := range matches {
		if matches[i].ProviderID == unknownMD5 {
			unknown = &matches[i]
			break
		}
	}
	if unknown == nil {
		t.Fatal("expected unknown-author ebook row in results")
	}
	if len(unknown.Authors) != 0 {
		t.Fatalf("unknown author row Authors = %#v, want empty", unknown.Authors)
	}
}

func TestAnnasArchiveAudiobookDetailReturnsNil(t *testing.T) {
	srv, client, _ := newAnnasArchiveFake(t)
	defer srv.Close()

	match, err := client.Fetch(context.Background(), "c3d4e5f67890abcdef1234567890abcd")
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil", err)
	}
	if match != nil {
		t.Fatalf("Fetch() = %#v, want nil for audio format", match)
	}
}

func TestAnnasArchiveSearchFiltersAdditionalAudioFormats(t *testing.T) {
	html := []byte(`
		<table>
		<tr>
			<td><a href="/md5/e5f67890abcdef1234567890abcdef12"><span>Audio Format Result</span></a></td>
			<td>2020 [en] .flac 400MB</td>
		</tr>
		<tr>
			<td><a href="/md5/f67890abcdef1234567890abcdef1234"><span>Spoken Word Result</span></a></td>
			<td>2020 [en] spoken word 400MB</td>
		</tr>
		<tr>
			<td><a href="/md5/67890abcdef1234567890abcdef12345"><span>Paper Result</span></a></td>
			<td>2020 [en] .epub 1MB</td>
		</tr>
		</table>
	`)

	matches := parseAnnasArchiveSearchPage(html, "https://example.test")
	if len(matches) != 1 {
		t.Fatalf("parseAnnasArchiveSearchPage() returned %d matches, want 1: %#v", len(matches), matches)
	}
	if matches[0].ProviderID != "67890abcdef1234567890abcdef12345" {
		t.Fatalf("ProviderID = %q, want ebook row", matches[0].ProviderID)
	}
}

func TestAnnasArchiveDetailFiltersAudioIndicatorWithoutExtension(t *testing.T) {
	html := []byte(`
		<html>
		<head><title>Spoken Edition</title></head>
		<body>
			<h1>Spoken Edition</h1>
			<p>Audio book</p>
		</body>
		</html>
	`)

	match, ext := parseAnnasArchiveDetailPage(html, "https://example.test")
	if match != nil || ext != "" {
		t.Fatalf("parseAnnasArchiveDetailPage() = %#v/%q, want nil/empty", match, ext)
	}
}
