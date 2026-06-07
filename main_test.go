package main

import (
	"context"
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
