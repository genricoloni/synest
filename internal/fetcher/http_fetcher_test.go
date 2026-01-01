package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestHTTPFetcher_Fetch(t *testing.T) {
	tests := []struct {
		name           string
		contentType    string
		responseBody   []byte
		statusCode     int
		ctxFunc        func() (context.Context, context.CancelFunc)
		expectedError  string
		expectedLength int
	}{
		{
			name:           "Success - Valid Image",
			contentType:    "image/jpeg",
			responseBody:   []byte("fake-image-data"),
			statusCode:     http.StatusOK,
			expectedError:  "",
			expectedLength: 15,
		},
		{
			name:          "Error - 404 Not Found",
			contentType:   "image/jpeg",
			statusCode:    http.StatusNotFound,
			expectedError: "unexpected status code: 404",
		},
		{
			name:          "Error - Invalid Content Type",
			contentType:   "text/plain",
			responseBody:  []byte("not-an-image"),
			statusCode:    http.StatusOK,
			expectedError: "url is not an image",
		},
		{
			name:        "Error - Response Too Large",
			contentType: "image/png",
			// Generate a body exceeding 10MB (simulated for the test with a reduced limit if necessary,
			// but here we use the real fetcher logic)
			responseBody:   []byte(strings.Repeat("a", 11*1024*1024)),
			statusCode:     http.StatusOK,
			expectedError:  "",               // io.ReadAll stops at the limit, does not return error by default but truncated data
			expectedLength: 10 * 1024 * 1024, // Limit enforced in the code
		},
		{
			name: "Error - Context Cancelled",
			ctxFunc: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately
				return ctx, cancel
			},
			expectedError: "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write(tt.responseBody)
			}))
			defer server.Close()

			// Setup context
			var ctx context.Context
			var cancel context.CancelFunc
			if tt.ctxFunc != nil {
				ctx, cancel = tt.ctxFunc()
			} else {
				ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
			}
			defer cancel()

			fetcher := NewHTTPFetcher(zap.NewNop())
			data, err := fetcher.Fetch(ctx, server.URL)

			// Verify error
			if tt.expectedError != "" {
				if err == nil {
					t.Fatalf("expected error containing '%s', got nil", tt.expectedError)
				}
				if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error '%s' to contain '%s'", err.Error(), tt.expectedError)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify data length (for OOM limit test)
			if len(data) != tt.expectedLength {
				t.Errorf("expected data length %d, got %d", tt.expectedLength, len(data))
			}
		})
	}
}
