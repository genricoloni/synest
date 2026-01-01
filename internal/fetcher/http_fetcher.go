package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

const _maxImageSize = 10 * 1024 * 1024 // 10 MB

// HTTPFetcher handles downloading image data from HTTP/HTTPS URLs
type HTTPFetcher struct {
	logger *zap.Logger
	client *http.Client
}

// NewHTTPFetcher creates a new HTTP-based fetcher instance
func NewHTTPFetcher(logger *zap.Logger) *HTTPFetcher {
	return &HTTPFetcher{
		logger: logger,
		client: &http.Client{
			Timeout: 10 * time.Second, // Essential to prevent blocking the daemon
		},
	}
}

// Fetch downloads image data from the given URL
func (f *HTTPFetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	// Future enhancement: validate protocol
	// if !strings.HasPrefix(url, "http") {
	//     return nil, errors.New("unsupported protocol")
	// }

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "synestDaemon/1.0")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Validazione Content-Type
    if !strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") {
        return nil, fmt.Errorf("url is not an image: %s", resp.Header.Get("Content-Type"))
    }

	limitReader := io.LimitReader(resp.Body, _maxImageSize)

	data, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	f.logger.Debug("Image fetched successfully", zap.Int("bytes", len(data)), zap.String("url", url))
	return data, nil
}
