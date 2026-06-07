package metadata

import "testing"

func TestProviderIDsFromMatch(t *testing.T) {
	ids := ProviderIDsFromMatch(Match{
		Provider:   "openlibrary",
		ProviderID: "OL7353617M",
		ISBN:       "978-0-593-13520-4",
	})

	if ids["openlibrary"] != "OL7353617M" {
		t.Fatalf("openlibrary id = %q", ids["openlibrary"])
	}
	if ids["ebook-metadata"] != "openlibrary:OL7353617M" {
		t.Fatalf("capability id = %q", ids["ebook-metadata"])
	}
	if ids["isbn"] != "9780593135204" {
		t.Fatalf("isbn = %q", ids["isbn"])
	}
}

func TestParseCapabilityProviderID(t *testing.T) {
	source, id := ParseCapabilityProviderID("googlebooks:zyTCAlFPjgYC")
	if source != "googlebooks" || id != "zyTCAlFPjgYC" {
		t.Fatalf("ParseCapabilityProviderID returned %q/%q", source, id)
	}
}
