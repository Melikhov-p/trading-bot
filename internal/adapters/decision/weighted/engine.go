// Package weighted реализует port.DecisionEngine взвешенной детерминированной
// формулой: sentiment/surprise → directionalRaw → FinalScore (умноженный на
// impact и confidence как понижающие множители) → Action/Track по порогам Track.
package weighted

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"

	"trading-bot/internal/application/config"
	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
	"trading-bot/internal/domain/trading"
	"trading-bot/internal/platform/clock"
)

// Engine — port.DecisionEngine поверх взвешенной формулы из конфигурации.
type Engine struct {
	cfg config.OrchestratorConfig
	clk clock.Clock
}

// New создаёт Engine с заданной конфигурацией весов/порогов и источником времени.
func New(cfg config.OrchestratorConfig, clk clock.Clock) *Engine {
	return &Engine{cfg: cfg, clk: clk}
}

// Name возвращает имя движка решений для логирования и SignalID.
func (e *Engine) Name() string {
	return "weighted"
}

// Decide детерминированно превращает NewsAnalysis в TradingSignal по формуле
// из конфигурации: см. doc-комментарий пакета и план в CLAUDE.md.
func (e *Engine) Decide(ctx context.Context, a news.NewsAnalysis) (trading.TradingSignal, error) {
	if err := ctx.Err(); err != nil {
		return trading.TradingSignal{}, err
	}

	directionalRaw := e.cfg.Weights.Sentiment*sentimentScore(a, e.cfg.MixedSentimentPenalty) +
		e.surpriseWeightEff(a.Surprise)*surpriseScore(a.Surprise)

	modifier, ok := e.cfg.EventModifiers[string(a.EventType)]
	if !ok {
		modifier = 1
	}
	finalScore := directionalRaw * a.ImpactScore.Float64() * a.Confidence.Float64() * modifier

	track := resolveTrack(a.TimeHorizon)
	thresholds, ok := e.cfg.Thresholds[string(track)]

	var action trading.Action
	switch {
	case !ok || a.Confidence.Float64() < thresholds.MinConfidence || a.ImpactScore.Float64() < thresholds.MinImpact:
		action = trading.ActionHold
	case finalScore >= thresholds.BuyThreshold:
		action = trading.ActionBuy
	case finalScore <= -thresholds.SellThreshold:
		action = trading.ActionSell
	case math.Abs(finalScore) >= thresholds.WatchThreshold:
		action = trading.ActionWatch
	default:
		action = trading.ActionHold
	}

	var ticker trading.Ticker
	if a.Ticker == nil {
		// Оркестратор не шлёт в брокер сигналы без инструмента — такой анализ
		// в лучшем случае достоин наблюдения, а не исполнения.
		action = trading.ActionWatch
	} else {
		ticker = *a.Ticker
	}

	generatedAt := e.clk.Now()

	return trading.TradingSignal{
		ID:          e.signalID(a),
		NewsID:      a.NewsID,
		Ticker:      ticker,
		Action:      action,
		Track:       track,
		FinalScore:  finalScore,
		Strength:    math.Min(1, math.Abs(finalScore)),
		Rationale:   rationale(a, finalScore),
		Status:      trading.SignalStatusPending,
		GeneratedAt: generatedAt,
		ExpiresAt:   generatedAt.Add(thresholds.SignalTTL()),
	}, nil
}

// signalID возвращает детерминированный идентификатор сигнала: одна и та же
// новость с тем же движком и версией конфигурации всегда даёт один SignalID,
// что делает сохранение сигнала идемпотентным.
func (e *Engine) signalID(a news.NewsAnalysis) string {
	h := sha256.Sum256([]byte(a.NewsID + e.Name() + e.cfg.Version))
	return hex.EncodeToString(h[:])
}

// sentimentScore проецирует FundamentalSentiment на [-1, 1]. Для mixed —
// разность числа положительных/отрицательных факторов, ограниченная [-1,1]
// и приглушённая mixedPenalty; при отсутствии факторов — нейтральный 0.
func sentimentScore(a news.NewsAnalysis, mixedPenalty float64) float64 {
	switch a.FundamentalSentiment {
	case news.SentimentPositive:
		return 1
	case news.SentimentNegative:
		return -1
	case news.SentimentMixed:
		pos, neg := len(a.PositiveFactors), len(a.NegativeFactors)
		if pos+neg == 0 {
			return 0
		}
		return clamp(float64(pos-neg)/float64(pos+neg), -1, 1) * mixedPenalty
	default: // neutral, unknown
		return 0
	}
}

// surpriseScore проецирует Surprise на [-1, 1]; neutral/unknown → 0.
func surpriseScore(s news.Surprise) float64 {
	switch s {
	case news.SurprisePositive:
		return 1
	case news.SurpriseNegative:
		return -1
	default:
		return 0
	}
}

// surpriseWeightEff отключает (или приглушает) вес surprise, когда модель не
// смогла сравнить факт с ожиданиями рынка, — так unknown не подменяется
// нейтральным значением по умолчанию, а честно исключается из формулы.
func (e *Engine) surpriseWeightEff(s news.Surprise) float64 {
	if s == news.SurpriseUnknown {
		return e.cfg.Weights.Surprise * e.cfg.UnknownSurprisePenalty
	}
	return e.cfg.Weights.Surprise
}

// resolveTrack выбирает торговый горизонт по TimeHorizon анализа.
func resolveTrack(h news.TimeHorizon) trading.Track {
	switch h {
	case news.HorizonIntraday:
		return trading.TrackIntraday
	case news.Horizon1To3Days, news.Horizon1To2Weeks:
		return trading.TrackSwing
	case news.Horizon1To3Months:
		return trading.TrackPosition
	default:
		return trading.TrackUnknown
	}
}

// rationale собирает человекочитаемое объяснение решения для аудита.
func rationale(a news.NewsAnalysis, finalScore float64) string {
	return fmt.Sprintf(
		"sentiment=%s surprise=%s impact=%.2f confidence=%.2f event=%s final_score=%.3f",
		a.FundamentalSentiment, a.Surprise, a.ImpactScore.Float64(), a.Confidence.Float64(), a.EventType, finalScore,
	)
}

// clamp ограничивает v диапазоном [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

var _ port.DecisionEngine = (*Engine)(nil)
