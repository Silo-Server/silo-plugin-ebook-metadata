package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Silo-Server/silo-plugin-ebook-metadata/metadata"
)

func sourceQueryText(q metadata.SearchQuery) string {
	parts := make([]string, 0, 1+len(q.Authors))
	if title := strings.TrimSpace(q.Title); title != "" {
		parts = append(parts, title)
	}
	for _, author := range q.Authors {
		if author = strings.TrimSpace(author); author != "" {
			parts = append(parts, author)
		}
	}
	return strings.Join(parts, " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func firstYear(value string) int {
	for i := 0; i+4 <= len(value); i++ {
		year, err := strconv.Atoi(value[i : i+4])
		if err == nil && year > 0 {
			return year
		}
	}
	return 0
}

func single(value string) []string {
	if value = strings.TrimSpace(value); value == "" {
		return nil
	}
	return []string{value}
}

func httpDoBytes(ctx context.Context, client *http.Client, req *http.Request) ([]byte, int, error) {
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, 0, fmt.Errorf("%s %s: request: %w", req.Method, req.URL.String(), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("%s %s: read body: %w", req.Method, req.URL.String(), err)
	}
	if len(body) > maxResponseBytes {
		return nil, resp.StatusCode, fmt.Errorf("%s %s: response body exceeds %d bytes", req.Method, req.URL.String(), maxResponseBytes)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return body, resp.StatusCode, fmt.Errorf("%s %s: status %d", req.Method, req.URL.String(), resp.StatusCode)
	}
	return body, resp.StatusCode, nil
}
