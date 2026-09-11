package fixed

import (
	"context"
	"errors"
	"testing"

	"trading-bot/internal/domain/trading"
)

func TestSizer_Size_ReturnsFixedQuantity(t *testing.T) {
	s := New(10)

	qty, err := s.Size(context.Background(), trading.TradingSignal{}, trading.Balance{Equity: 1_000_000})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if qty != 10 {
		t.Fatalf("qty = %d, want 10", qty)
	}
}

func TestSizer_Size_RespectsCancelledContext(t *testing.T) {
	s := New(10)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.Size(ctx, trading.TradingSignal{}, trading.Balance{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
