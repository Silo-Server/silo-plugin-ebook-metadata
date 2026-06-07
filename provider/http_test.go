package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPGetBytesLimitsBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(make([]byte, maxResponseBytes+1)); err != nil {
			t.Errorf("server write: %v", err)
		}
	}))
	defer server.Close()

	_, err := httpGetBytes(context.Background(), server.Client(), server.URL, "")
	if err == nil {
		t.Fatal("httpGetBytes() error = nil, want body limit error")
	}
}

func TestHTTPGetBytesSetsUserAgent(t *testing.T) {
	var gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.UserAgent()
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("server write: %v", err)
		}
	}))
	defer server.Close()

	got, err := httpGetBytes(context.Background(), server.Client(), server.URL, "test-agent")
	if err != nil {
		t.Fatalf("httpGetBytes() error = %v", err)
	}
	if string(got) != "ok" {
		t.Fatalf("httpGetBytes() = %q, want ok", string(got))
	}
	if gotUserAgent != "test-agent" {
		t.Fatalf("User-Agent = %q, want test-agent", gotUserAgent)
	}
}

func TestHTTPGetBytesErrorsForNon2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	_, err := httpGetBytes(context.Background(), server.Client(), server.URL, "")
	if err == nil {
		t.Fatal("httpGetBytes() error = nil, want status error")
	}
	if errors.Is(err, context.Canceled) {
		t.Fatalf("httpGetBytes() error = %v, want status error", err)
	}
}
