// Package pipeline содержит Orchestrator — use case, связывающий все порты
// в единый поток "новость → фильтр → анализ → решение → исполнение".
package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"trading-bot/internal/application/config"
	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
	"trading-bot/internal/domain/trading"
	"trading-bot/internal/platform/keyedmutex"
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

	// tickerLocks не даёт двум воркерам одновременно отправить ордер по
	// одному и тому же тикеру.
	tickerLocks *keyedmutex.Mutex
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
		tickerLocks:  keyedmutex.New(),
	}
}

// ProcessNews выполняет один прогон пайплайна для одной новости:
// дедуп → сохранение → фильтрация → анализ → решение → идемпотентность → исполнение.
// Бизнес-исходы (новость отфильтрована, анализ не удался, сигнал пропущен по
// cooldown, ордер отклонён брокером) не считаются ошибками пайплайна —
// ProcessNews возвращает nil, залогировав детали. Ошибка возвращается только
// при сбое инфраструктуры (репозиторий, отменённый ctx).
func (o *Orchestrator) ProcessNews(ctx context.Context, n news.News) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	_, found, err := o.newsRepo.FindByExternalID(ctx, n.Source, n.ExternalID)
	if err != nil {
		return fmt.Errorf("pipeline: find news: %w", err)
	}
	if found {
		return nil
	}

	n.Status = news.StatusReceived
	if err := o.newsRepo.Save(ctx, n); err != nil {
		return fmt.Errorf("pipeline: save news: %w", err)
	}

	result, err := o.filter.IsRelevant(ctx, n)
	if err != nil {
		return fmt.Errorf("pipeline: filter: %w", err)
	}
	if !result.Relevant {
		n.Status = news.StatusFilteredOut
		if err := o.newsRepo.Update(ctx, n); err != nil {
			return fmt.Errorf("pipeline: update news: %w", err)
		}
		return nil
	}

	analysis, err := o.analyzer.Analyze(ctx, n)
	if err != nil {
		o.logger.WarnContext(ctx, "news analysis failed", "news_id", n.ID, "error", err)
		n.Status = news.StatusAnalysisFailed
		if err := o.newsRepo.Update(ctx, n); err != nil {
			return fmt.Errorf("pipeline: update news: %w", err)
		}
		return nil
	}

	if err := o.analysisRepo.Save(ctx, analysis); err != nil {
		return fmt.Errorf("pipeline: save analysis: %w", err)
	}
	n.Status = news.StatusAnalyzed
	if err := o.newsRepo.Update(ctx, n); err != nil {
		return fmt.Errorf("pipeline: update news: %w", err)
	}

	signal, err := o.engine.Decide(ctx, analysis)
	if err != nil {
		return fmt.Errorf("pipeline: decide: %w", err)
	}

	if signal.Action != trading.ActionBuy && signal.Action != trading.ActionSell {
		signal.Status = trading.SignalStatusSkipped
		if err := o.signalRepo.Save(ctx, signal); err != nil {
			return fmt.Errorf("pipeline: save signal: %w", err)
		}
		return nil
	}

	cooldown := o.cfg.Thresholds[string(signal.Track)].Cooldown()
	recent, err := o.signalRepo.FindRecentByTicker(ctx, signal.Ticker, cooldown)
	if err != nil {
		return fmt.Errorf("pipeline: find recent signals: %w", err)
	}
	for _, r := range recent {
		if r.Action == signal.Action {
			signal.Status = trading.SignalStatusSkipped
			if err := o.signalRepo.Save(ctx, signal); err != nil {
				return fmt.Errorf("pipeline: save signal: %w", err)
			}
			return nil
		}
	}

	balance, err := o.broker.GetBalance(ctx)
	if err != nil {
		return fmt.Errorf("pipeline: get balance: %w", err)
	}

	qty, err := o.sizer.Size(ctx, signal, balance)
	if err != nil {
		return fmt.Errorf("pipeline: size position: %w", err)
	}

	// SignalID переиспользуется как OrderID: одна и та же новость с тем же
	// движком и версией конфигурации всегда даёт тот же ID, что делает
	// повторную обработку идемпотентной без FindByID/List в OrderRepository.
	order := trading.Order{
		ID:       trading.OrderID(signal.ID),
		SignalID: signal.ID,
		Ticker:   signal.Ticker,
		Side:     orderSide(signal.Action),
		Kind:     trading.OrderKindMarket,
		Quantity: qty,
		Status:   trading.OrderStatusNew,
	}

	unlock := o.tickerLocks.Lock(string(signal.Ticker))
	placed, placeErr := o.broker.PlaceOrder(ctx, order)
	unlock()

	if placeErr != nil {
		o.logger.ErrorContext(ctx, "place order failed", "ticker", signal.Ticker, "error", placeErr)
		order.Status = trading.OrderStatusRejected
		signal.Status = trading.SignalStatusFailed
	} else {
		order = placed
		signal.Status = trading.SignalStatusExecuted
		n.Status = news.StatusExecuted
	}

	if err := o.signalRepo.Save(ctx, signal); err != nil {
		return fmt.Errorf("pipeline: save signal: %w", err)
	}
	if err := o.orderRepo.Save(ctx, order); err != nil {
		return fmt.Errorf("pipeline: save order: %w", err)
	}
	if err := o.newsRepo.Update(ctx, n); err != nil {
		return fmt.Errorf("pipeline: update news: %w", err)
	}

	return nil
}

// orderSide конвертирует торговое действие сигнала в направление ордера.
// Вызывается только для Action ∈ {Buy, Sell} — единственных значений,
// доходящих до этой точки пайплайна.
func orderSide(a trading.Action) trading.OrderSide {
	switch a {
	case trading.ActionBuy:
		return trading.OrderSideBuy
	case trading.ActionSell:
		return trading.OrderSideSell
	default:
		return trading.OrderSideUnknown
	}
}

// Run подписывается на NewsProvider и обрабатывает поток новостей пулом
// воркеров размера cfg.WorkerPoolSize (не меньше 1). Возвращается, когда
// провайдер закрывает свои каналы и все воркеры дорабатывают начатое —
// для конечного источника (FakeNewsProvider) это естественное завершение.
func (o *Orchestrator) Run(ctx context.Context) error {
	newsCh, errCh := o.provider.Subscribe(ctx)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for err := range errCh {
			o.logger.ErrorContext(ctx, "news provider error", "provider", o.provider.Name(), "error", err)
		}
	}()

	workers := max(o.cfg.WorkerPoolSize, 1)

	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for n := range newsCh {
				if err := o.ProcessNews(ctx, n); err != nil {
					o.logger.ErrorContext(ctx, "process news failed", "news_id", n.ID, "error", err)
				}
			}
		}()
	}

	wg.Wait()
	return nil
}
