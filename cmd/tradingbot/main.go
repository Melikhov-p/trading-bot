// Command tradingbot запускает торгового бота на новостях.
// Итерация 2: добавлен ручной прогон одной новости через YandexGPTAnalyzer
// флагом -news-text, для проверки анализатора без полного пайплайна.
// Wiring Orchestrator и остальных адаптеров появится в итерации 4 плана.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"trading-bot/internal/adapters/analyzer/yandexgpt"
	"trading-bot/internal/application/config"
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
	)

	// TODO(итерация 4): собрать Orchestrator из адаптеров (DI-wiring) и вызвать Run(ctx).

	return nil
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
