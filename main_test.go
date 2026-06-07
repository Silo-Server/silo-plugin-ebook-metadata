package main

import (
	"context"
	"encoding/hex"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
)

func TestRuntimeServerConfigureNoOp(t *testing.T) {
	server := &runtimeServer{}

	_, err := server.Configure(context.Background(), &pluginv1.ConfigureRequest{})
	if err != nil {
		t.Fatalf("Configure() error = %v", err)
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
