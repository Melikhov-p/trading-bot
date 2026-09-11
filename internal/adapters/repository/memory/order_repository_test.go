package memory

import (
	"context"
	"testing"

	"trading-bot/internal/domain/trading"
)

func TestOrderRepository_SaveAndUpdate(t *testing.T) {
	r := NewOrderRepository()
	ctx := context.Background()

	o := trading.Order{ID: "o-1", Status: trading.OrderStatusNew}
	if err := r.Save(ctx, o); err != nil {
		t.Fatalf("Save: %v", err)
	}

	o.Status = trading.OrderStatusFilled
	if err := r.Update(ctx, o); err != nil {
		t.Fatalf("Update: %v", err)
	}
}

func TestOrderRepository_RespectsCancelledContext(t *testing.T) {
	r := NewOrderRepository()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := r.Save(ctx, trading.Order{}); err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
