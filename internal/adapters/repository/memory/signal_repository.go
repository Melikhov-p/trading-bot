package memory

import (
	"context"
	"sync"
	"time"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/trading"
	"trading-bot/internal/platform/clock"
)

// SignalRepository — потокобезопасный port.SignalRepository поверх карты
// "тикер -> сигналы", нужен для cooldown-идемпотентности оркестратора.
type SignalRepository struct {
	clk clock.Clock

	mu       sync.RWMutex
	byTicker map[trading.Ticker][]trading.TradingSignal
}

// NewSignalRepository создаёт пустой SignalRepository с источником времени clk.
func NewSignalRepository(clk clock.Clock) *SignalRepository {
	return &SignalRepository{clk: clk, byTicker: make(map[trading.Ticker][]trading.TradingSignal)}
}

// Save добавляет сигнал в историю по его тикеру.
func (r *SignalRepository) Save(ctx context.Context, s trading.TradingSignal) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.byTicker[s.Ticker] = append(r.byTicker[s.Ticker], s)
	return nil
}

// FindRecentByTicker возвращает сигналы по ticker, сгенерированные не раньше
// чем within назад от текущего момента.
func (r *SignalRepository) FindRecentByTicker(
	ctx context.Context, ticker trading.Ticker, within time.Duration,
) ([]trading.TradingSignal, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	now := r.clk.Now()

	r.mu.RLock()
	defer r.mu.RUnlock()

	var recent []trading.TradingSignal
	for _, s := range r.byTicker[ticker] {
		if now.Sub(s.GeneratedAt) <= within {
			recent = append(recent, s)
		}
	}
	return recent, nil
}

var _ port.SignalRepository = (*SignalRepository)(nil)
