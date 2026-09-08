// Package noop реализует port.Broker без реального исполнения ордеров —
// используется вместо реального брокера, когда AppConfig.DryRun == true.
package noop

import (
	"context"
	"log/slog"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/trading"
)

// Broker логирует "would place order" вместо реального исполнения.
type Broker struct {
	logger *slog.Logger
}

// New создаёт Broker, пишущий в logger.
func New(logger *slog.Logger) *Broker {
	return &Broker{logger: logger}
}

// Name возвращает имя брокера для логирования и метрик.
func (b *Broker) Name() string {
	return "noop"
}

// PlaceOrder логирует намерение разместить ордер и возвращает его же с
// присвоенным статусом OrderStatusNew — реального исполнения не происходит.
func (b *Broker) PlaceOrder(ctx context.Context, o trading.Order) (trading.Order, error) {
	b.logger.InfoContext(ctx, "would place order",
		"ticker", o.Ticker,
		"side", o.Side,
		"kind", o.Kind,
		"quantity", o.Quantity,
	)
	o.Status = trading.OrderStatusNew
	o.BrokerOrderID = "noop-" + string(o.ID)
	return o, nil
}

// CancelOrder логирует намерение отменить ордер.
func (b *Broker) CancelOrder(ctx context.Context, id trading.OrderID) error {
	b.logger.InfoContext(ctx, "would cancel order", "order_id", id)
	return nil
}

// GetOrderStatus возвращает пустой ордер с ID и статусом new, поскольку
// noop-брокер не хранит состояние ордеров.
func (b *Broker) GetOrderStatus(ctx context.Context, id trading.OrderID) (trading.Order, error) {
	return trading.Order{ID: id, Status: trading.OrderStatusNew}, nil
}

// GetPositions возвращает пустой список — noop-брокер не держит позиций.
func (b *Broker) GetPositions(ctx context.Context) ([]trading.Position, error) {
	return []trading.Position{}, nil
}

// GetBalance возвращает нулевой баланс — noop-брокер не хранит состояние счёта.
func (b *Broker) GetBalance(ctx context.Context) (trading.Balance, error) {
	return trading.Balance{}, nil
}

var _ port.Broker = (*Broker)(nil)
