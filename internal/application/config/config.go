// Package config описывает конфигурацию приложения и оркестратора и загружает
// её из YAML. Секреты (API-ключи, токены) в конфиг не входят — они читаются
// адаптерами напрямую из переменных окружения.
package config

import (
	"fmt"
	"math"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

const weightSumEpsilon = 1e-6

// Weights — веса компонентов формулы WeightedFormulaDecisionEngine.
// Инвариант: Sentiment + Surprise == 1.
type Weights struct {
	Sentiment float64 `yaml:"sentiment"`
	Surprise  float64 `yaml:"surprise"`
}

// ThresholdSet — пороги и тайминги, специфичные для одного Track (intraday/swing/position).
// Тайминги хранятся в секундах: yaml.v3 не умеет анмаршалить строки вида "5m"
// в time.Duration без кастомного UnmarshalYAML, а секунды-int избегают этой проблемы.
type ThresholdSet struct {
	MinConfidence    float64 `yaml:"min_confidence"`
	MinImpact        float64 `yaml:"min_impact"`
	BuyThreshold     float64 `yaml:"buy_threshold"`
	SellThreshold    float64 `yaml:"sell_threshold"`
	WatchThreshold   float64 `yaml:"watch_threshold"`
	SignalTTLSeconds int     `yaml:"signal_ttl_seconds"`
	CooldownSeconds  int     `yaml:"cooldown_seconds"`
}

// SignalTTL возвращает время жизни сигнала как time.Duration.
func (t ThresholdSet) SignalTTL() time.Duration {
	return time.Duration(t.SignalTTLSeconds) * time.Second
}

// Cooldown возвращает окно cooldown между сигналами как time.Duration.
func (t ThresholdSet) Cooldown() time.Duration {
	return time.Duration(t.CooldownSeconds) * time.Second
}

// FilterConfig — пороги и веса RuleBasedRelevanceFilter. Тайминги хранятся в
// секундах по той же причине, что и в ThresholdSet — yaml.v3 не анмаршалит
// строки вида "5m" в time.Duration без кастомного UnmarshalYAML.
type FilterConfig struct {
	MinBodyLength      int     `yaml:"min_body_length"`
	MaxAgeSeconds      int     `yaml:"max_age_seconds"`
	DedupWindowSeconds int     `yaml:"dedup_window_seconds"`
	WatchlistWeight    float64 `yaml:"watchlist_weight"`
	KeywordWeight      float64 `yaml:"keyword_weight"`
	MinScoreToPass     float64 `yaml:"min_score_to_pass"`
}

// MaxAge возвращает предельный возраст новости как time.Duration.
func (f FilterConfig) MaxAge() time.Duration {
	return time.Duration(f.MaxAgeSeconds) * time.Second
}

// DedupWindow возвращает окно дедупликации как time.Duration.
func (f FilterConfig) DedupWindow() time.Duration {
	return time.Duration(f.DedupWindowSeconds) * time.Second
}

// OrchestratorConfig — веса/пороги/справочники, управляющие фильтром и движком решений.
type OrchestratorConfig struct {
	// Version участвует в детерминированном SignalID (sha256(NewsID+EngineName+Version)):
	// смена формулы/весов меняет Version, что делает старые SignalID не сравнимыми с новыми.
	Version        string  `yaml:"version"`
	WorkerPoolSize int     `yaml:"worker_pool_size"`
	Weights        Weights `yaml:"weights"`
	// FixedPositionQuantity — размер лота для sizing/fixed.Sizer, пока нет
	// риск-based sizing по реальному балансу/цене (см. broker/bcs).
	FixedPositionQuantity  int64                   `yaml:"fixed_position_quantity"`
	UnknownSurprisePenalty float64                 `yaml:"unknown_surprise_penalty"`
	MixedSentimentPenalty  float64                 `yaml:"mixed_sentiment_penalty"`
	EventModifiers         map[string]float64      `yaml:"event_modifiers"`
	Thresholds             map[string]ThresholdSet `yaml:"thresholds"` // ключ — Track
	Filter                 FilterConfig            `yaml:"filter"`
	Watchlist              []string                `yaml:"watchlist"`
	Keywords               map[string][]string     `yaml:"keywords"` // ключ — EventType
}

// AppConfig — корневая конфигурация приложения. Поля верхнего уровня несут
// env-теги: cleanenv позволяет переопределить их переменной окружения поверх
// YAML — это же понадобится для секретов адаптеров (YANDEX_API_KEY и т.п.),
// когда они появятся в Итерации 2.
type AppConfig struct {
	DryRun       bool               `yaml:"dry_run" env:"DRY_RUN" env-default:"true"`
	LogLevel     string             `yaml:"log_level" env:"LOG_LEVEL" env-default:"info"`
	Orchestrator OrchestratorConfig `yaml:"orchestrator"`
}

// Load читает конфигурацию из YAML-файла по пути path, переопределяет
// значения переменными окружения (см. env-теги AppConfig) и валидирует итог.
func Load(path string) (AppConfig, error) {
	var cfg AppConfig
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return AppConfig{}, fmt.Errorf("config: read %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return AppConfig{}, fmt.Errorf("config: validate %q: %w", path, err)
	}

	return cfg, nil
}

// Validate проверяет инварианты конфигурации, которые не выражаются типами.
func (c AppConfig) Validate() error {
	weightSum := c.Orchestrator.Weights.Sentiment + c.Orchestrator.Weights.Surprise
	if math.Abs(weightSum-1) > weightSumEpsilon {
		return fmt.Errorf("orchestrator.weights: sentiment+surprise must equal 1, got %v", weightSum)
	}

	for track, t := range c.Orchestrator.Thresholds {
		if t.BuyThreshold <= t.WatchThreshold {
			return fmt.Errorf("orchestrator.thresholds[%s]: buy_threshold must be > watch_threshold", track)
		}
		if t.WatchThreshold <= 0 {
			return fmt.Errorf("orchestrator.thresholds[%s]: watch_threshold must be > 0", track)
		}
	}

	return nil
}
