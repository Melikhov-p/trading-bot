// Command tradingbot запускает торгового бота на новостях.
// Итерация 1 (bootstrap): загрузка конфигурации и логирование.
// Wiring Orchestrator и адаптеров появится в итерации 4 плана.
package main

import (
	"flag"
	"fmt"
	"os"

	"trading-bot/internal/application/config"
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
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("tradingbot: %w", err)
	}

	logger := logging.New(cfg.LogLevel)
	logger.Info("trading-bot starting",
		"dry_run", cfg.DryRun,
		"worker_pool_size", cfg.Orchestrator.WorkerPoolSize,
	)

	// TODO(итерация 4): собрать Orchestrator из адаптеров (DI-wiring) и вызвать Run(ctx).

	return nil
}
