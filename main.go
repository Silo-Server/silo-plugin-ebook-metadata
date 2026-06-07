package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	publicmanifest "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/manifest"
	"github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/runtime"
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
