// Package fixed реализует port.PositionSizer фиксированным лотом из
// конфигурации — временная заглушка до появления рыночных данных
// (баланс/цена инструмента) от broker/bcs, на которых можно строить
// риск-based sizing.
package fixed

import (
	"context"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/trading"
)

// Sizer — port.PositionSizer, всегда возвращающий один и тот же размер лота.
type Sizer struct {
	quantity int64
}

// New создаёт Sizer, возвращающий quantity для любого сигнала.
func New(quantity int64) *Sizer {
	return &Sizer{quantity: quantity}
}

// Size возвращает фиксированный размер лота, не заглядывая в баланс/цену.
func (s *Sizer) Size(ctx context.Context, _ trading.TradingSignal, _ trading.Balance) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return s.quantity, nil
}

var _ port.PositionSizer = (*Sizer)(nil)
