// Package bcs реализует port.Broker поверх БКС Trading API.
// TODO(итерация 7): уточнить по документации БКС Trading API — протокол
// (REST/FIX/QUIK), эндпоинты, аутентификация; реализовать сначала в
// sandbox/paper-режиме, если он доступен у брокера.
package bcs

import (
	"context"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/trading"
)

// Broker — заглушка port.Broker для БКС.
type Broker struct {
	// TODO: параметры подключения — после сверки с документацией БКС Trading API.
}

// New создаёт заглушку Broker.
func New() *Broker {
	return &Broker{}
}

// Name возвращает имя брокера для логирования и метрик.
func (b *Broker) Name() string {
	return "bcs"
}

// PlaceOrder пока не реализован — см. TODO в комментарии пакета.
func (b *Broker) PlaceOrder(ctx context.Context, o trading.Order) (trading.Order, error) {
	return trading.Order{}, port.ErrNotImplemented
}

// CancelOrder пока не реализован — см. TODO в комментарии пакета.
func (b *Broker) CancelOrder(ctx context.Context, id trading.OrderID) error {
	return port.ErrNotImplemented
}

// GetOrderStatus пока не реализован — см. TODO в комментарии пакета.
func (b *Broker) GetOrderStatus(ctx context.Context, id trading.OrderID) (trading.Order, error) {
	return trading.Order{}, port.ErrNotImplemented
}

// GetPositions пока не реализован — см. TODO в комментарии пакета.
func (b *Broker) GetPositions(ctx context.Context) ([]trading.Position, error) {
	return nil, port.ErrNotImplemented
}

// GetBalance пока не реализован — см. TODO в комментарии пакета.
func (b *Broker) GetBalance(ctx context.Context) (trading.Balance, error) {
	return trading.Balance{}, port.ErrNotImplemented
}

var _ port.Broker = (*Broker)(nil)
