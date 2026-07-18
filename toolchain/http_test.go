package toolchain

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestHTTPDownloader(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://example.test/compiler" {
			t.Fatalf("URL = %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("compiler"))}, nil
	})}
	body, err := (HTTPDownloader{Client: client}).Download(context.Background(), "https://example.test/compiler")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := body.Close(); err != nil {
			t.Errorf("close response: %v", err)
		}
	}()
	content, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "compiler" {
		t.Fatalf("content = %q", content)
	}
}

func TestHTTPDownloaderRejectsInsecureURLAndStatus(t *testing.T) {
	if _, err := (HTTPDownloader{}).Download(context.Background(), "http://example.test/compiler"); err == nil {
		t.Fatal("insecure URL accepted")
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Body: io.NopCloser(strings.NewReader("missing"))}, nil
	})}
	if _, err := (HTTPDownloader{Client: client}).Download(context.Background(), "https://example.test/missing"); err == nil {
		t.Fatal("error status accepted")
	}
}

func TestHTTPDownloaderRejectsCredentialsAndDowngrade(t *testing.T) {
	if _, err := (HTTPDownloader{}).Download(context.Background(), "https://user:secret@example.test/compiler"); err == nil {
		t.Fatal("URL credentials accepted")
	}

	insecure := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("redirect downgrade reached the insecure server")
	}))
	defer insecure.Close()

	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, insecure.URL, http.StatusFound)
	}))
	defer secure.Close()

	if _, err := (HTTPDownloader{Client: secure.Client()}).Download(context.Background(), secure.URL); err == nil {
		t.Fatal("HTTP redirect accepted")
	}
}

func TestHTTPDownloaderRejectsOversizedResponse(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Body:          io.NopCloser(strings.NewReader("")),
			ContentLength: maxDownloadBytes + 1,
			Request:       request,
		}, nil
	})}
	if _, err := (HTTPDownloader{Client: client}).Download(context.Background(), "https://example.test/compiler"); err == nil {
		t.Fatal("oversized response accepted")
	}
}
