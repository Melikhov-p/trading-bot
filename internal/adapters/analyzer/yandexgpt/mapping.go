package yandexgpt

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
	"trading-bot/internal/domain/trading"
)

var eventTypeByValue = map[string]news.EventType{
	"earnings":   news.EventTypeEarnings,
	"guidance":   news.EventTypeGuidance,
	"dividend":   news.EventTypeDividend,
	"m&a":        news.EventTypeMergerAcquisition,
	"management": news.EventTypeManagement,
	"regulation": news.EventTypeRegulation,
	"macro":      news.EventTypeMacro,
	"product":    news.EventTypeProduct,
	"legal":      news.EventTypeLegal,
	"other":      news.EventTypeOther,
}

var sentimentByValue = map[string]news.FundamentalSentiment{
	"positive": news.SentimentPositive,
	"negative": news.SentimentNegative,
	"neutral":  news.SentimentNeutral,
	"mixed":    news.SentimentMixed,
}

var surpriseByValue = map[string]news.Surprise{
	"positive": news.SurprisePositive,
	"negative": news.SurpriseNegative,
	"neutral":  news.SurpriseNeutral,
	"unknown":  news.SurpriseUnknown,
}

var timeHorizonByValue = map[string]news.TimeHorizon{
	"intraday": news.HorizonIntraday,
	"1-3d":     news.Horizon1To3Days,
	"1-2w":     news.Horizon1To2Weeks,
	"1-3m":     news.Horizon1To3Months,
}

// parseAnalysis превращает сырой output_text модели в news.NewsAnalysis:
// снимает возможную markdown-обёртку, при необходимости делает один
// repair-retry извлечения JSON-подстроки, затем валидирует и маппит поля
// в доменные типы.
func parseAnalysis(newsID, modelID, outputText string, analyzedAt time.Time) (news.NewsAnalysis, error) {
	raw, err := extractJSON(outputText)
	if err != nil {
		return news.NewsAnalysis{}, fmt.Errorf("yandexgpt: %w: %w", port.ErrInvalidJSON, err)
	}

	var dto analysisDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		return news.NewsAnalysis{}, fmt.Errorf("yandexgpt: %w: %w", port.ErrInvalidJSON, err)
	}

	return mapAnalysis(newsID, modelID, dto, analyzedAt)
}

// extractJSON снимает markdown-обёртку и, если результат всё ещё не
// валидный JSON, делает один repair-retry — извлекает подстроку между
// первой '{' и последней '}', на случай посторонних символов вокруг объекта.
func extractJSON(text string) ([]byte, error) {
	stripped := stripMarkdownFence(text)
	if json.Valid([]byte(stripped)) {
		return []byte(stripped), nil
	}

	repaired := extractBraces(stripped)
	if repaired == "" || !json.Valid([]byte(repaired)) {
		return nil, fmt.Errorf("output is not valid json after markdown strip and repair")
	}

	return []byte(repaired), nil
}

func stripMarkdownFence(text string) string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}

	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")

	return strings.TrimSpace(trimmed)
}

func extractBraces(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end < start {
		return ""
	}

	return text[start : end+1]
}

func mapAnalysis(newsID, modelID string, dto analysisDTO, analyzedAt time.Time) (news.NewsAnalysis, error) {
	eventType, ok := eventTypeByValue[dto.EventType]
	if !ok {
		return news.NewsAnalysis{}, fmt.Errorf("yandexgpt: %w: event_type %q", port.ErrSchemaViolation, dto.EventType)
	}

	sentiment, ok := sentimentByValue[dto.FundamentalSentiment]
	if !ok {
		return news.NewsAnalysis{}, fmt.Errorf(
			"yandexgpt: %w: fundamental_sentiment %q", port.ErrSchemaViolation, dto.FundamentalSentiment,
		)
	}

	surprise, ok := surpriseByValue[dto.Surprise]
	if !ok {
		return news.NewsAnalysis{}, fmt.Errorf("yandexgpt: %w: surprise %q", port.ErrSchemaViolation, dto.Surprise)
	}

	horizon, ok := timeHorizonByValue[dto.TimeHorizon]
	if !ok {
		return news.NewsAnalysis{}, fmt.Errorf("yandexgpt: %w: time_horizon %q", port.ErrSchemaViolation, dto.TimeHorizon)
	}

	impact, err := news.NewScore(dto.ImpactScore)
	if err != nil {
		return news.NewsAnalysis{}, fmt.Errorf("yandexgpt: %w: impact_score: %w", port.ErrSchemaViolation, err)
	}

	confidence, err := news.NewScore(dto.Confidence)
	if err != nil {
		return news.NewsAnalysis{}, fmt.Errorf("yandexgpt: %w: confidence: %w", port.ErrSchemaViolation, err)
	}

	var ticker *trading.Ticker
	if dto.Ticker != nil && *dto.Ticker != "" {
		t := trading.Ticker(*dto.Ticker)
		ticker = &t
	}

	return news.NewsAnalysis{
		NewsID:               newsID,
		Company:              dto.Company,
		Ticker:               ticker,
		EventType:            eventType,
		FundamentalSentiment: sentiment,
		Surprise:             surprise,
		ImpactScore:          impact,
		Confidence:           confidence,
		TimeHorizon:          horizon,
		PositiveFactors:      dto.PositiveFactors,
		NegativeFactors:      dto.NegativeFactors,
		KeyRisk:              dto.KeyRisk,
		Reason:               dto.Reason,
		ModelID:              modelID,
		AnalyzedAt:           analyzedAt,
	}, nil
}
