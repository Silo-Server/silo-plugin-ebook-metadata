package main

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/Silo-Server/silo-plugin-ebook-metadata/metadata"
	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
)

func TestRuntimeServerConfigureNoOp(t *testing.T) {
	server := &runtimeServer{}

	_, err := server.Configure(context.Background(), &pluginv1.ConfigureRequest{})
	if err != nil {
		t.Fatalf("Configure() error = %v", err)
	}
}

func TestProviderSearchResultFromMatchMapsEbookIDs(t *testing.T) {
	match := metadata.Match{
		Provider:    "openlibrary",
		ProviderID:  "OL7353617M",
		Title:       "The Name of the Wind",
		Description: "A gifted young man grows into a legend.",
		PublishYear: 2007,
		ISBN:        "978-0-7564-0474-1",
		CoverURL:    "https://covers.openlibrary.org/b/id/123-L.jpg",
	}

	result, err := providerSearchResultFromMatch(match, "ebook")
	if err != nil {
		t.Fatalf("providerSearchResultFromMatch() error = %v", err)
	}

	if result.GetProviderId() != "openlibrary:OL7353617M" {
		t.Fatalf("ProviderId = %q, want openlibrary:OL7353617M", result.GetProviderId())
	}
	if result.GetTitle() != "The Name of the Wind" {
		t.Fatalf("Title = %q, want The Name of the Wind", result.GetTitle())
	}
	if result.GetItemType() != "ebook" {
		t.Fatalf("ItemType = %q, want ebook", result.GetItemType())
	}

	ids := result.GetProviderIds().GetFields()
	if got := ids["openlibrary"].GetStringValue(); got != "OL7353617M" {
		t.Fatalf("ProviderIds.openlibrary = %q, want OL7353617M", got)
	}
	if got := ids["ebook-metadata"].GetStringValue(); got != "openlibrary:OL7353617M" {
		t.Fatalf("ProviderIds.ebook-metadata = %q, want openlibrary:OL7353617M", got)
	}
	if got := ids["isbn"].GetStringValue(); got != "9780756404741" {
		t.Fatalf("ProviderIds.isbn = %q, want 9780756404741", got)
	}
}

func TestMetadataItemFromMatchMapsAuthorsOnly(t *testing.T) {
	match := metadata.Match{
		Provider:       "openlibrary",
		ProviderID:     "OL7353617M",
		Title:          "The Name of the Wind",
		Subtitle:       "The Kingkiller Chronicle: Day One",
		Authors:        []string{"Patrick Rothfuss"},
		Description:    "A gifted young man grows into a legend.",
		Publisher:      "DAW",
		PublishYear:    2007,
		ISBN:           "978-0-7564-0474-1",
		Genres:         []string{"Fantasy"},
		CoverURL:       "https://covers.openlibrary.org/b/id/123-L.jpg",
		Language:       "en",
		PageCount:      662,
		SeriesName:     "The Kingkiller Chronicle",
		SeriesPosition: "1",
	}

	item, err := metadataItemFromMatch(match, "ebook")
	if err != nil {
		t.Fatalf("metadataItemFromMatch() error = %v", err)
	}

	if item.GetProviderId() != "openlibrary:OL7353617M" {
		t.Fatalf("ProviderId = %q, want openlibrary:OL7353617M", item.GetProviderId())
	}
	if item.GetItemType() != "ebook" {
		t.Fatalf("ItemType = %q, want ebook", item.GetItemType())
	}
	if len(item.GetPeople()) != 1 {
		t.Fatalf("People length = %d, want 1", len(item.GetPeople()))
	}
	person := item.GetPeople()[0]
	if person.GetName() != "Patrick Rothfuss" {
		t.Fatalf("People[0].Name = %q, want Patrick Rothfuss", person.GetName())
	}
	if person.GetKind() != "author" {
		t.Fatalf("People[0].Kind = %q, want author", person.GetKind())
	}
	if person.GetSortOrder() != 0 {
		t.Fatalf("People[0].SortOrder = %d, want 0", person.GetSortOrder())
	}
	if got := item.GetMetadata().GetFields()["page_count"].GetNumberValue(); got != 662 {
		t.Fatalf("Metadata.page_count = %v, want 662", got)
	}
	if item.GetPosterPath() != "https://covers.openlibrary.org/b/id/123-L.jpg" {
		t.Fatalf("PosterPath = %q, want cover URL", item.GetPosterPath())
	}
	if len(item.GetStudios()) != 1 || item.GetStudios()[0] != "DAW" {
		t.Fatalf("Studios = %v, want [DAW]", item.GetStudios())
	}
	metadataFields := item.GetMetadata().GetFields()
	if got := metadataFields["series_name"].GetStringValue(); got != "The Kingkiller Chronicle" {
		t.Fatalf("Metadata.series_name = %q, want The Kingkiller Chronicle", got)
	}
	if got := metadataFields["series_position"].GetStringValue(); got != "1" {
		t.Fatalf("Metadata.series_position = %q, want 1", got)
	}
}

func TestLoadManifestParsesEbookCapability(t *testing.T) {
	manifest, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest() error = %v", err)
	}

	if manifest.GetPluginId() != "silo.ebook-metadata" {
		t.Fatalf("PluginId = %q, want silo.ebook-metadata", manifest.GetPluginId())
	}
	if manifest.GetVersion() != "0.1.0" {
		t.Fatalf("Version = %q, want 0.1.0", manifest.GetVersion())
	}
	if manifest.GetSiloApiVersion() != "v1" {
		t.Fatalf("SiloApiVersion = %q, want v1", manifest.GetSiloApiVersion())
	}

	capabilities := manifest.GetCapabilities()
	if len(capabilities) != 1 {
		t.Fatalf("capabilities length = %d, want 1", len(capabilities))
	}

	capability := capabilities[0]
	if capability.GetType() != "metadata_provider.v1" {
		t.Fatalf("capability Type = %q, want metadata_provider.v1", capability.GetType())
	}
	if capability.GetId() != capabilityID {
		t.Fatalf("capability Id = %q, want %s", capability.GetId(), capabilityID)
	}

	defaultPriority := capability.GetMetadata().GetFields()["default_priority"].GetStructValue()
	if defaultPriority == nil {
		t.Fatal("default_priority metadata is missing")
	}
	if got := defaultPriority.GetFields()["ebook"].GetNumberValue(); got != 2 {
		t.Fatalf("default_priority.ebook = %v, want 2", got)
	}
}

func TestLoadManifestPopulatesChecksum(t *testing.T) {
	manifest, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest() error = %v", err)
	}

	checksum := manifest.GetChecksum()
	if checksum == "" || checksum == "__CHECKSUM__" {
		t.Fatalf("Checksum = %q, want populated executable checksum", checksum)
	}
	if len(checksum) != 64 {
		t.Fatalf("Checksum length = %d, want 64", len(checksum))
	}
	if _, err := hex.DecodeString(checksum); err != nil {
		t.Fatalf("Checksum is not hex: %v", err)
	}
}

func TestLoadManifestAppliesBuildTimeVersionOverride(t *testing.T) {
	originalVersion := version
	version = "9.8.7-test"
	t.Cleanup(func() { version = originalVersion })

	manifest, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest() error = %v", err)
	}

	if manifest.GetVersion() != "9.8.7-test" {
		t.Fatalf("Version = %q, want build-time override", manifest.GetVersion())
	}
}

func TestRuntimeServerGetManifestReturnsLoadedManifest(t *testing.T) {
	manifest, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest() error = %v", err)
	}
	server := &runtimeServer{manifest: manifest}

	resp, err := server.GetManifest(context.Background(), &pluginv1.GetManifestRequest{})
	if err != nil {
		t.Fatalf("GetManifest() error = %v", err)
	}
	if resp.GetManifest() != manifest {
		t.Fatalf("GetManifest() returned manifest %p, want %p", resp.GetManifest(), manifest)
	}
}
