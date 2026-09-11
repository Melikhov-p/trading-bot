package memory

import (
	"context"
	"testing"
	"time"

	"trading-bot/internal/domain/trading"
)

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time {
	return c.now
}

var fixedNow = time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

func TestSignalRepository_FindRecentByTicker_WithinWindow(t *testing.T) {
	r := NewSignalRepository(fakeClock{now: fixedNow})
	ctx := context.Background()

	s := trading.TradingSignal{
		Ticker:      "SBER",
		Action:      trading.ActionBuy,
		GeneratedAt: fixedNow.Add(-10 * time.Minute),
	}
	if err := r.Save(ctx, s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	recent, err := r.FindRecentByTicker(ctx, "SBER", 30*time.Minute)
	if err != nil {
		t.Fatalf("FindRecentByTicker: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("len(recent) = %d, want 1", len(recent))
	}
}

func TestSignalRepository_FindRecentByTicker_OutsideWindow(t *testing.T) {
	r := NewSignalRepository(fakeClock{now: fixedNow})
	ctx := context.Background()

	s := trading.TradingSignal{
		Ticker:      "SBER",
		Action:      trading.ActionBuy,
		GeneratedAt: fixedNow.Add(-2 * time.Hour),
	}
	if err := r.Save(ctx, s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	recent, err := r.FindRecentByTicker(ctx, "SBER", 30*time.Minute)
	if err != nil {
		t.Fatalf("FindRecentByTicker: %v", err)
	}
	if len(recent) != 0 {
		t.Fatalf("len(recent) = %d, want 0", len(recent))
	}
}

func TestSignalRepository_FindRecentByTicker_DifferentTicker(t *testing.T) {
	r := NewSignalRepository(fakeClock{now: fixedNow})
	ctx := context.Background()

	s := trading.TradingSignal{Ticker: "GAZP", GeneratedAt: fixedNow}
	if err := r.Save(ctx, s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	recent, err := r.FindRecentByTicker(ctx, "SBER", time.Hour)
	if err != nil {
		t.Fatalf("FindRecentByTicker: %v", err)
	}
	if len(recent) != 0 {
		t.Fatalf("len(recent) = %d, want 0", len(recent))
	}
}

func TestSignalRepository_RespectsCancelledContext(t *testing.T) {
	r := NewSignalRepository(fakeClock{now: fixedNow})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := r.Save(ctx, trading.TradingSignal{}); err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if _, err := r.FindRecentByTicker(ctx, "SBER", time.Hour); err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
