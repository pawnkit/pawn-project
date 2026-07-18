package toolchain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultHTTPTimeout = 30 * time.Second

// HTTPDownloader fetches HTTPS artifacts with an injected client.
type HTTPDownloader struct {
	Client *http.Client
}

func (d HTTPDownloader) Download(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("toolchain: invalid HTTPS download URL %q", rawURL)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("toolchain: creating download request: %w", err)
	}
	client := d.Client
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	clientCopy := *client
	originalRedirect := client.CheckRedirect
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || req.URL.Host == "" || req.URL.User != nil {
			return fmt.Errorf("toolchain: redirect to invalid HTTPS URL %q", req.URL)
		}
		if originalRedirect != nil {
			return originalRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("toolchain: stopped after 10 redirects")
		}
		return nil
	}
	response, err := clientCopy.Do(request)
	if err != nil {
		return nil, fmt.Errorf("toolchain: downloading artifact: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_ = response.Body.Close()
		return nil, fmt.Errorf("toolchain: download returned %s", response.Status)
	}
	if response.Request != nil && (response.Request.URL == nil || response.Request.URL.Scheme != "https" || response.Request.URL.Host == "") {
		_ = response.Body.Close()
		return nil, errors.New("toolchain: download ended at an invalid HTTPS URL")
	}
	if response.ContentLength > maxDownloadBytes {
		_ = response.Body.Close()
		return nil, fmt.Errorf("toolchain: download exceeds %d byte limit", maxDownloadBytes)
	}
	return response.Body, nil
}
