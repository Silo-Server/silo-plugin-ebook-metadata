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

func TestProviderIDsFromMatchTrimsAndSkipsEmptyValues(t *testing.T) {
	ids := ProviderIDsFromMatch(Match{
		Provider:   " openlibrary ",
		ProviderID: " OL7353617M ",
		ISBN:       " 978 0 593 13520 4 ",
	})
	if ids["openlibrary"] != "OL7353617M" {
		t.Fatalf("trimmed provider id = %q", ids["openlibrary"])
	}
	if ids[CapabilityID] != "openlibrary:OL7353617M" {
		t.Fatalf("capability id = %q", ids[CapabilityID])
	}
	if ids["isbn"] != "9780593135204" {
		t.Fatalf("isbn = %q", ids["isbn"])
	}

	ids = ProviderIDsFromMatch(Match{Provider: "openlibrary", ISBN: "978-0-593-13520-4"})
	if _, ok := ids["openlibrary"]; ok {
		t.Fatalf("unexpected source id for empty ProviderID: %#v", ids)
	}
	if ids[CapabilityID] != "" {
		t.Fatalf("unexpected capability id for empty ProviderID: %#v", ids)
	}
	if ids["isbn"] != "9780593135204" {
		t.Fatalf("isbn-only id = %#v", ids)
	}
}

func TestParseCapabilityProviderIDRejectsMalformedValues(t *testing.T) {
	cases := []string{
		"",
		"openlibrary",
		":OL7353617M",
		"openlibrary:",
		" : ",
	}
	for _, input := range cases {
		source, id := ParseCapabilityProviderID(input)
		if source != "" || id != "" {
			t.Fatalf("ParseCapabilityProviderID(%q) = %q/%q, want empty", input, source, id)
		}
	}
}

func TestParseCapabilityProviderIDTrimsSourceAndID(t *testing.T) {
	source, id := ParseCapabilityProviderID(" googlebooks : zyTCAlFPjgYC ")
	if source != "googlebooks" || id != "zyTCAlFPjgYC" {
		t.Fatalf("ParseCapabilityProviderID trimmed result = %q/%q", source, id)
	}
}
