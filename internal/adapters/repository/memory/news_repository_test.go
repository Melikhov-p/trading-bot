package memory

import (
	"context"
	"sync"
	"testing"

	"trading-bot/internal/domain/news"
)

func TestNewsRepository_SaveAndFind(t *testing.T) {
	r := NewNewsRepository()
	ctx := context.Background()

	n := news.News{Source: "interfax", ExternalID: "ext-1", Headline: "test"}
	if err := r.Save(ctx, n); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, found, err := r.FindByExternalID(ctx, "interfax", "ext-1")
	if err != nil {
		t.Fatalf("FindByExternalID: %v", err)
	}
	if !found {
		t.Fatal("expected news to be found")
	}
	if got.Headline != "test" {
		t.Fatalf("Headline = %q, want %q", got.Headline, "test")
	}
}

func TestNewsRepository_FindByExternalID_NotFound(t *testing.T) {
	r := NewNewsRepository()

	_, found, err := r.FindByExternalID(context.Background(), "interfax", "missing")
	if err != nil {
		t.Fatalf("FindByExternalID: %v", err)
	}
	if found {
		t.Fatal("expected news not to be found")
	}
}

func TestNewsRepository_Update_Overwrites(t *testing.T) {
	r := NewNewsRepository()
	ctx := context.Background()

	n := news.News{Source: "interfax", ExternalID: "ext-1", Status: news.StatusReceived}
	if err := r.Save(ctx, n); err != nil {
		t.Fatalf("Save: %v", err)
	}

	n.Status = news.StatusFilteredOut
	if err := r.Update(ctx, n); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _, err := r.FindByExternalID(ctx, "interfax", "ext-1")
	if err != nil {
		t.Fatalf("FindByExternalID: %v", err)
	}
	if got.Status != news.StatusFilteredOut {
		t.Fatalf("Status = %q, want %q", got.Status, news.StatusFilteredOut)
	}
}

func TestNewsRepository_RespectsCancelledContext(t *testing.T) {
	r := NewNewsRepository()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := r.Save(ctx, news.News{}); err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if _, _, err := r.FindByExternalID(ctx, "s", "e"); err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestNewsRepository_ConcurrentAccess(t *testing.T) {
	r := NewNewsRepository()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = r.Save(ctx, news.News{Source: "s", ExternalID: string(rune('a' + i%26))})
			_, _, _ = r.FindByExternalID(ctx, "s", string(rune('a'+i%26)))
		}(i)
	}
	wg.Wait()
}
