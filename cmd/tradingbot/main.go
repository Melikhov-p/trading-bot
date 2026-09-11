// Command tradingbot запускает торгового бота на новостях. Флаг -news-text
// прогоняет один текст через YandexGPTAnalyzer в обход пайплайна (для
// отладки промпта); без него собирается полный Orchestrator на FakeNewsProvider
// (filesource) и noop.Broker (DryRun) и запускается Run до конца файла или
// сигнала остановки.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"trading-bot/internal/adapters/analyzer/yandexgpt"
	"trading-bot/internal/adapters/broker/noop"
	"trading-bot/internal/adapters/decision/weighted"
	"trading-bot/internal/adapters/filter/rulebased"
	"trading-bot/internal/adapters/newsprovider/filesource"
	"trading-bot/internal/adapters/repository/memory"
	"trading-bot/internal/adapters/sizing/fixed"
	"trading-bot/internal/application/config"
	"trading-bot/internal/application/pipeline"
	"trading-bot/internal/domain/news"
	"trading-bot/internal/platform/clock"
	"trading-bot/internal/platform/httpclient"
	"trading-bot/internal/platform/logging"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "configs/config.yaml", "путь к YAML-конфигурации")
	newsText := flag.String(
		"news-text", "", "текст новости для ручного прогона через YandexGPTAnalyzer (без запуска пайплайна)",
	)
	newsFile := flag.String(
		"news-file", "testdata/news_sample.ndjson", "путь к ndjson-файлу новостей для filesource.Provider",
	)
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("tradingbot: %w", err)
	}

	logger := logging.New(cfg.LogLevel)

	if *newsText != "" {
		return runAnalyzeOnce(logger, *newsText)
	}

	logger.Info("trading-bot starting",
		"dry_run", cfg.DryRun,
		"worker_pool_size", cfg.Orchestrator.WorkerPoolSize,
		"news_file", *newsFile,
	)

	orch, err := buildOrchestrator(cfg, *newsFile, logger)
	if err != nil {
		return fmt.Errorf("tradingbot: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := orch.Run(ctx); err != nil {
		return fmt.Errorf("tradingbot: run: %w", err)
	}

	return nil
}

// buildOrchestrator собирает Orchestrator из адаптеров (ручной DI-wiring):
// filesource как NewsProvider, rulebased+weighted как фильтр/движок решений,
// реальный yandexgpt.Analyzer, fixed.Sizer, noop.Broker (пока DryRun всегда
// true — реальный broker/bcs появится в Итерации 7) и 4 memory-репозитория.
func buildOrchestrator(cfg config.AppConfig, newsFile string, logger *slog.Logger) (*pipeline.Orchestrator, error) {
	analyzerCfg, err := yandexgpt.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load analyzer config: %w", err)
	}

	clk := clock.Real{}
	httpClient := httpclient.New(analyzerCfg.Timeout)

	filterCfg := rulebased.Config{
		MinBodyLength:   cfg.Orchestrator.Filter.MinBodyLength,
		MaxAge:          cfg.Orchestrator.Filter.MaxAge(),
		DedupWindow:     cfg.Orchestrator.Filter.DedupWindow(),
		WatchlistWeight: cfg.Orchestrator.Filter.WatchlistWeight,
		KeywordWeight:   cfg.Orchestrator.Filter.KeywordWeight,
		MinScoreToPass:  cfg.Orchestrator.Filter.MinScoreToPass,
		Watchlist:       cfg.Orchestrator.Watchlist,
		Keywords:        cfg.Orchestrator.Keywords,
	}

	deps := pipeline.Deps{
		Provider:     filesource.New(newsFile),
		Filter:       rulebased.New(filterCfg, clk),
		Analyzer:     yandexgpt.New(analyzerCfg, httpClient, clk),
		Engine:       weighted.New(cfg.Orchestrator, clk),
		Sizer:        fixed.New(cfg.Orchestrator.FixedPositionQuantity),
		Broker:       noop.New(logger),
		NewsRepo:     memory.NewNewsRepository(),
		AnalysisRepo: memory.NewAnalysisRepository(),
		SignalRepo:   memory.NewSignalRepository(clk),
		OrderRepo:    memory.NewOrderRepository(),
		Config:       cfg.Orchestrator,
		Logger:       logger,
	}

	return pipeline.New(deps), nil
}

// runAnalyzeOnce прогоняет один текст новости через YandexGPTAnalyzer и
// печатает структурированный результат — без сохранения в репозитории и без
// вызова DecisionEngine/Broker.
func runAnalyzeOnce(logger *slog.Logger, newsText string) error {
	analyzerCfg, err := yandexgpt.LoadConfig()
	if err != nil {
		return fmt.Errorf("tradingbot: %w", err)
	}

	client := httpclient.New(analyzerCfg.Timeout)
	analyzer := yandexgpt.New(analyzerCfg, client, clock.Real{})

	ctx, cancel := context.WithTimeout(context.Background(), analyzerCfg.Timeout+5*time.Second)
	defer cancel()

	n := news.News{
		ID:   "manual-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		Body: newsText,
	}

	analysis, err := analyzer.Analyze(ctx, n)
	if err != nil {
		return fmt.Errorf("tradingbot: analyze: %w", err)
	}

	logger.Info("analysis complete",
		"company", analysis.Company,
		"event_type", analysis.EventType,
		"fundamental_sentiment", analysis.FundamentalSentiment,
		"surprise", analysis.Surprise,
		"impact_score", analysis.ImpactScore.Float64(),
		"confidence", analysis.Confidence.Float64(),
		"time_horizon", analysis.TimeHorizon,
		"key_risk", analysis.KeyRisk,
		"reason", analysis.Reason,
	)

	return nil
}
