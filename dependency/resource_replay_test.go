package dependency

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

type countingDownloader struct {
	calls int
	data  []byte
}

func (d *countingDownloader) Download(context.Context, string) (io.ReadCloser, error) {
	d.calls++
	return io.NopCloser(bytes.NewReader(d.data)), nil
}

func TestReplayDownloaderDownloadsOnce(t *testing.T) {
	base := &countingDownloader{data: []byte("asset")}
	replay, err := NewReplayDownloader(base)
	if err != nil {
		t.Fatalf("NewReplayDownloader: %v", err)
	}
	t.Cleanup(func() {
		if err := replay.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	for range 2 {
		reader, err := replay.Download(context.Background(), "https://example.com/asset")
		if err != nil {
			t.Fatalf("Download: %v", err)
		}
		content, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil || string(content) != "asset" {
			t.Fatalf("content = %q, err = %v", content, err)
		}
	}
	if base.calls != 1 {
		t.Fatalf("calls = %d, want 1", base.calls)
	}
}

func TestReplayDownloaderRejectsDownloadsAfterClose(t *testing.T) {
	base := &countingDownloader{data: []byte("asset")}
	replay, err := NewReplayDownloader(base)
	if err != nil {
		t.Fatalf("NewReplayDownloader: %v", err)
	}
	if err := replay.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, err = replay.Download(context.Background(), "https://example.com/asset")
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("Download error = %v", err)
	}
}
