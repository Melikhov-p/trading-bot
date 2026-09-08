// Package pipeline содержит Orchestrator — use case, связывающий все порты
// в единый поток "новость → фильтр → анализ → решение → исполнение".
// Логика ProcessNews/Run реализуется в итерации 4 плана; здесь зафиксирована
// только структура зависимостей.
package pipeline

import (
	"context"
	"fmt"
	"log/slog"

	"trading-bot/internal/application/config"
	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
)

// Orchestrator прогоняет новости через полный торговый пайплайн.
type Orchestrator struct {
	provider     port.NewsProvider
	filter       port.NewsRelevanceFilter
	analyzer     port.NewsAnalyzer
	engine       port.DecisionEngine
	sizer        port.PositionSizer
	broker       port.Broker
	newsRepo     port.NewsRepository
	analysisRepo port.AnalysisRepository
	signalRepo   port.SignalRepository
	orderRepo    port.OrderRepository
	cfg          config.OrchestratorConfig
	logger       *slog.Logger
}

// Deps собирает все зависимости Orchestrator — конструктор с 4+ параметрами
// заменяется структурой опций (см. golang-code-style).
type Deps struct {
	Provider     port.NewsProvider
	Filter       port.NewsRelevanceFilter
	Analyzer     port.NewsAnalyzer
	Engine       port.DecisionEngine
	Sizer        port.PositionSizer
	Broker       port.Broker
	NewsRepo     port.NewsRepository
	AnalysisRepo port.AnalysisRepository
	SignalRepo   port.SignalRepository
	OrderRepo    port.OrderRepository
	Config       config.OrchestratorConfig
	Logger       *slog.Logger
}

// New собирает Orchestrator из готовых зависимостей (ручной DI-wiring в cmd/tradingbot).
func New(deps Deps) *Orchestrator {
	return &Orchestrator{
		provider:     deps.Provider,
		filter:       deps.Filter,
		analyzer:     deps.Analyzer,
		engine:       deps.Engine,
		sizer:        deps.Sizer,
		broker:       deps.Broker,
		newsRepo:     deps.NewsRepo,
		analysisRepo: deps.AnalysisRepo,
		signalRepo:   deps.SignalRepo,
		orderRepo:    deps.OrderRepo,
		cfg:          deps.Config,
		logger:       deps.Logger,
	}
}

// ProcessNews выполняет один прогон пайплайна для одной новости:
// дедуп → сохранение → фильтрация → анализ → решение → идемпотентность → исполнение.
// TODO(итерация 4): реализовать по шагам из плана.
func (o *Orchestrator) ProcessNews(ctx context.Context, n news.News) error {
	return fmt.Errorf("pipeline: ProcessNews: %w", port.ErrNotImplemented)
}

// Run подписывается на NewsProvider и обрабатывает поток новостей пулом воркеров.
// TODO(итерация 4): реализовать пул воркеров размера cfg.WorkerPoolSize.
func (o *Orchestrator) Run(ctx context.Context) error {
	return fmt.Errorf("pipeline: Run: %w", port.ErrNotImplemented)
}
