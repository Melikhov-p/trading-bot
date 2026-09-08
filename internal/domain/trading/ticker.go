// Package trading содержит доменные сущности торгового контура: инструменты,
// сигналы, ордера, позиции и баланс. Пакет не зависит от application/adapters слоёв.
package trading

// Ticker — биржевой идентификатор инструмента (например, "SBER").
type Ticker string
