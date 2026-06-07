package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestHTTPGetBytesWrapsRequestErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := httpGetBytes(ctx, http.DefaultClient, "https://example.invalid", "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("httpGetBytes() error = %v, want context.Canceled", err)
	}
}

type brokenBody struct{}

func (brokenBody) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func (brokenBody) Close() error {
	return nil
}

type brokenBodyTransport struct{}

func (brokenBodyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       brokenBody{},
	}, nil
}

func TestHTTPGetBytesWrapsBodyReadErrors(t *testing.T) {
	client := &http.Client{Transport: brokenBodyTransport{}}

	_, err := httpGetBytes(context.Background(), client, "https://example.test/book", "")
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("httpGetBytes() error = %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestRateLimiterInvalidRPMDoesNotGrantRequest(t *testing.T) {
	for _, rpm := range []float64{0, -1} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		err := waitForLimiter(ctx, newLimiter(rpm))
		cancel()
		if err == nil {
			t.Fatalf("waitForLimiter(rpm=%v) error = nil, want invalid limiter error", rpm)
		}
	}
}

func TestRateLimiterPositiveRPMAllowsInitialRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := waitForLimiter(ctx, newLimiter(60)); err != nil {
		t.Fatalf("waitForLimiter() error = %v", err)
	}
}
