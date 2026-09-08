// Package weighted реализует port.DecisionEngine взвешенной детерминированной
// формулой: sentiment/surprise → directionalRaw → FinalScore (умноженный на
// impact и confidence как понижающие множители) → Action/Track по порогам Track.
// TODO(итерация 3): реализовать формулу и гейты по правилам из плана.
package weighted

import (
	"context"

	"trading-bot/internal/application/config"
	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
	"trading-bot/internal/domain/trading"
)

// Engine — заглушка port.DecisionEngine.
type Engine struct {
	cfg config.OrchestratorConfig
}

// New создаёт Engine с заданной конфигурацией весов и порогов.
func New(cfg config.OrchestratorConfig) *Engine {
	return &Engine{cfg: cfg}
}

// Name возвращает имя движка решений для логирования и SignalID.
func (e *Engine) Name() string {
	return "weighted"
}

// Decide пока не реализован — см. TODO в комментарии пакета.
func (e *Engine) Decide(ctx context.Context, a news.NewsAnalysis) (trading.TradingSignal, error) {
	return trading.TradingSignal{}, port.ErrNotImplemented
}

var _ port.DecisionEngine = (*Engine)(nil)
