package filesource

import (
	"context"
	"testing"
	"time"
)

func TestProvider_Subscribe_ReadsAllNews(t *testing.T) {
	p := New("testdata/news.ndjson")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	newsCh, errCh := p.Subscribe(ctx)

	var headlines []string
	var errs []error

	newsOpen, errOpen := true, true
	for newsOpen || errOpen {
		select {
		case n, ok := <-newsCh:
			if !ok {
				newsOpen = false
				newsCh = nil
				continue
			}
			headlines = append(headlines, n.Headline)
		case err, ok := <-errCh:
			if !ok {
				errOpen = false
				errCh = nil
				continue
			}
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	want := []string{"First", "Second"}
	if len(headlines) != len(want) {
		t.Fatalf("headlines = %v, want %v", headlines, want)
	}
	for i, h := range want {
		if headlines[i] != h {
			t.Fatalf("headlines[%d] = %q, want %q", i, headlines[i], h)
		}
	}
}

func TestProvider_Subscribe_MissingFile(t *testing.T) {
	p := New("testdata/does-not-exist.ndjson")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	newsCh, errCh := p.Subscribe(ctx)

	if _, ok := <-newsCh; ok {
		t.Fatal("expected newsCh to be closed immediately")
	}

	err, ok := <-errCh
	if !ok || err == nil {
		t.Fatal("expected an error on errCh")
	}
}

func TestProvider_Subscribe_SkipsBadLineButContinues(t *testing.T) {
	p := New("testdata/news_with_bad_line.ndjson")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	newsCh, errCh := p.Subscribe(ctx)

	var headlines []string
	var errs []error

	newsOpen, errOpen := true, true
	for newsOpen || errOpen {
		select {
		case n, ok := <-newsCh:
			if !ok {
				newsOpen = false
				newsCh = nil
				continue
			}
			headlines = append(headlines, n.Headline)
		case err, ok := <-errCh:
			if !ok {
				errOpen = false
				errCh = nil
				continue
			}
			errs = append(errs, err)
		}
	}

	if len(errs) != 1 {
		t.Fatalf("len(errs) = %d, want 1: %v", len(errs), errs)
	}
	if len(headlines) != 2 {
		t.Fatalf("headlines = %v, want 2 entries", headlines)
	}
}

func TestProvider_Subscribe_RespectsCancelledContext(t *testing.T) {
	p := New("testdata/news.ndjson")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	newsCh, errCh := p.Subscribe(ctx)

	timeout := time.After(2 * time.Second)
	newsOpen, errOpen := true, true
	for newsOpen || errOpen {
		select {
		case _, ok := <-newsCh:
			if !ok {
				newsOpen = false
				newsCh = nil
			}
		case _, ok := <-errCh:
			if !ok {
				errOpen = false
				errCh = nil
			}
		case <-timeout:
			t.Fatal("Subscribe did not stop after context cancellation")
		}
	}
}
