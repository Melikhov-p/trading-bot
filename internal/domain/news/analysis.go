package news

import (
	"time"

	"trading-bot/internal/domain/trading"
)

// EventType — категория события, извлечённая из новости.
type EventType string

const (
	EventTypeUnknown           EventType = ""
	EventTypeEarnings          EventType = "earnings"
	EventTypeGuidance          EventType = "guidance"
	EventTypeDividend          EventType = "dividend"
	EventTypeMergerAcquisition EventType = "m&a"
	EventTypeManagement        EventType = "management"
	EventTypeRegulation        EventType = "regulation"
	EventTypeMacro             EventType = "macro"
	EventTypeProduct           EventType = "product"
	EventTypeLegal             EventType = "legal"
	EventTypeOther             EventType = "other"
)

// FundamentalSentiment — фундаментальная оценка новости для компании.
type FundamentalSentiment string

const (
	SentimentUnknown  FundamentalSentiment = ""
	SentimentPositive FundamentalSentiment = "positive"
	SentimentNegative FundamentalSentiment = "negative"
	SentimentNeutral  FundamentalSentiment = "neutral"
	SentimentMixed    FundamentalSentiment = "mixed"
)

// Surprise — отклонение фактического результата от ожиданий рынка.
type Surprise string

const (
	SurprisePositive Surprise = "positive"
	SurpriseNegative Surprise = "negative"
	SurpriseNeutral  Surprise = "neutral"
	SurpriseUnknown  Surprise = "unknown" // модель явно не смогла сравнить с ожиданиями; отличается от "" (анализ ещё не выполнен)
)

// TimeHorizon — ожидаемый горизонт влияния события на цену.
type TimeHorizon string

const (
	HorizonUnknown    TimeHorizon = ""
	HorizonIntraday   TimeHorizon = "intraday"
	Horizon1To3Days   TimeHorizon = "1-3d"
	Horizon1To2Weeks  TimeHorizon = "1-2w"
	Horizon1To3Months TimeHorizon = "1-3m"
)

// NewsAnalysis — структурированный результат работы NewsAnalyzer,
// маппится 1:1 на JSON-схему из prompts/news_analysis.ru.txt.
type NewsAnalysis struct {
	NewsID               string
	Company              string
	Ticker               *trading.Ticker
	EventType            EventType
	FundamentalSentiment FundamentalSentiment
	Surprise             Surprise
	ImpactScore          Score
	Confidence           Score
	TimeHorizon          TimeHorizon
	PositiveFactors      []string
	NegativeFactors      []string
	KeyRisk              string
	Reason               string
	ModelID              string
	AnalyzedAt           time.Time
}
