package memory

import (
	"context"
	"sync"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/trading"
)

// OrderRepository — потокобезопасный port.OrderRepository поверх карты,
// проиндексированной по OrderID.
type OrderRepository struct {
	mu   sync.RWMutex
	byID map[trading.OrderID]trading.Order
}

// NewOrderRepository создаёт пустой OrderRepository.
func NewOrderRepository() *OrderRepository {
	return &OrderRepository{byID: make(map[trading.OrderID]trading.Order)}
}

// Save сохраняет ордер (upsert по OrderID).
func (r *OrderRepository) Save(ctx context.Context, o trading.Order) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[o.ID] = o
	return nil
}

// Update обновляет ранее сохранённый ордер (upsert — см. NewsRepository.Update).
func (r *OrderRepository) Update(ctx context.Context, o trading.Order) error {
	return r.Save(ctx, o)
}

var _ port.OrderRepository = (*OrderRepository)(nil)
