package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/Silo-Server/silo-plugin-ebook-metadata/metadata"
	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	publicmanifest "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/manifest"
	"github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/runtime"
	"google.golang.org/protobuf/types/known/structpb"
)

// version is set at build time via -ldflags "-X main.version=...".
var version string

const capabilityID = "ebook-metadata"

type runtimeServer struct {
	pluginv1.UnimplementedRuntimeServer

	manifest *pluginv1.PluginManifest
}

//go:embed manifest.json
var manifestJSON []byte

func (s *runtimeServer) GetManifest(context.Context, *pluginv1.GetManifestRequest) (*pluginv1.GetManifestResponse, error) {
	return &pluginv1.GetManifestResponse{Manifest: s.manifest}, nil
}

func (s *runtimeServer) Configure(_ context.Context, _ *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	return &pluginv1.ConfigureResponse{}, nil
}

func providerSearchResultFromMatch(match metadata.Match, itemType string) (*pluginv1.ProviderSearchResult, error) {
	providerIDs, err := stringStruct(metadata.ProviderIDsFromMatch(match))
	if err != nil {
		return nil, err
	}

	return &pluginv1.ProviderSearchResult{
		ProviderId:    primaryProviderID(match),
		Title:         strings.TrimSpace(match.Title),
		OriginalTitle: strings.TrimSpace(match.Title),
		Year:          int32(match.PublishYear),
		Overview:      strings.TrimSpace(match.Description),
		ImageUrl:      strings.TrimSpace(match.CoverURL),
		ItemType:      strings.TrimSpace(itemType),
		ProviderIds:   providerIDs,
	}, nil
}

func metadataItemFromMatch(match metadata.Match, itemType string) (*pluginv1.MetadataItem, error) {
	providerIDs, err := stringStruct(metadata.ProviderIDsFromMatch(match))
	if err != nil {
		return nil, err
	}

	return &pluginv1.MetadataItem{
		ProviderId:    primaryProviderID(match),
		Title:         strings.TrimSpace(match.Title),
		OriginalTitle: strings.TrimSpace(match.Title),
		Year:          int32(match.PublishYear),
		Overview:      strings.TrimSpace(match.Description),
		Genres:        genresFromMatch(match),
		ProviderIds:   providerIDs,
		Metadata:      metadataStruct(match),
		Studios:       publisherStudio(match.Publisher),
		PosterPath:    strings.TrimSpace(match.CoverURL),
		People:        peopleFromMatch(match),
		ItemType:      strings.TrimSpace(itemType),
	}, nil
}

func primaryProviderID(match metadata.Match) string {
	provider := strings.TrimSpace(match.Provider)
	providerID := strings.TrimSpace(match.ProviderID)
	if provider != "" && providerID != "" {
		return provider + ":" + providerID
	}
	return metadata.NormalizeISBN(match.ISBN)
}

func stringStruct(value map[string]string) (*structpb.Struct, error) {
	fields := make(map[string]any)
	for key, val := range value {
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" || val == "" {
			continue
		}
		fields[key] = val
	}
	return structpb.NewStruct(fields)
}

func metadataStruct(match metadata.Match) *structpb.Struct {
	fields := make(map[string]*structpb.Value)
	addString := func(key, value string) {
		if value = strings.TrimSpace(value); value != "" {
			fields[key] = structpb.NewStringValue(value)
		}
	}

	addString("subtitle", match.Subtitle)
	addString("isbn", metadata.NormalizeISBN(match.ISBN))
	addString("publisher", match.Publisher)
	addString("language", match.Language)
	addString("series_name", match.SeriesName)
	addString("series_position", match.SeriesPosition)
	if match.PageCount > 0 {
		fields["page_count"] = structpb.NewNumberValue(float64(match.PageCount))
	}

	return &structpb.Struct{Fields: fields}
}

func publisherStudio(publisher string) []string {
	publisher = strings.TrimSpace(publisher)
	if publisher == "" {
		return nil
	}
	return []string{publisher}
}

func genresFromMatch(match metadata.Match) []string {
	genres := make([]string, 0, len(match.Genres))
	for _, genre := range match.Genres {
		genre = strings.TrimSpace(genre)
		if genre == "" {
			continue
		}
		genres = append(genres, genre)
	}
	if len(genres) == 0 {
		return nil
	}
	return genres
}

func peopleFromMatch(match metadata.Match) []*pluginv1.PersonRecord {
	people := make([]*pluginv1.PersonRecord, 0, len(match.Authors))
	for _, author := range match.Authors {
		author = strings.TrimSpace(author)
		if author == "" {
			continue
		}
		people = append(people, &pluginv1.PersonRecord{
			Name:      author,
			Kind:      "author",
			SortOrder: int32(len(people)),
		})
	}
	return people
}

func main() {
	manifest, err := loadManifest()
	if err != nil {
		panic(err)
	}

	runtime.Serve(runtime.ServeConfig{
		Servers: runtime.CapabilityServers{
			Runtime: &runtimeServer{manifest: manifest},
		},
	})
}

func loadManifest() (*pluginv1.PluginManifest, error) {
	manifest, err := publicmanifest.Load(manifestJSON)
	if err != nil {
		return nil, fmt.Errorf("load embedded manifest: %w", err)
	}

	if version != "" {
		manifest.Version = version
	}

	executablePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}
	binaryData, err := os.ReadFile(executablePath)
	if err != nil {
		return nil, fmt.Errorf("read executable %q: %w", executablePath, err)
	}
	checksum := sha256.Sum256(binaryData)
	manifest.Checksum = hex.EncodeToString(checksum[:])

	return manifest, nil
}
