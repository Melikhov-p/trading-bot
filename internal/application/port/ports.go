// Package port объявляет интерфейсы (порты), через которые application-слой
// обращается к внешнему миру. Адаптеры реализуют эти интерфейсы структурно,
// без импорта пакета port — зависимость направлена от adapters к port, а не
// наоборот, что и обеспечивает гексагональную изоляцию домена.
package port

import (
	"context"
	"time"

	"trading-bot/internal/domain/news"
	"trading-bot/internal/domain/trading"
)

// FilterResult — вердикт NewsRelevanceFilter по одной новости.
type FilterResult struct {
	Relevant bool
	Reasons  []string
	Score    float64
	// HintEventType — категория события, угаданная по ключевым словам
	// (news.EventTypeUnknown, если ни одна категория не совпала). Передаётся
	// NewsAnalyzer как ориентир, а не как готовый ответ.
	HintEventType news.EventType
}

// NewsProvider поставляет поток новостей из внешнего источника (Интерфакс и т.п.).
type NewsProvider interface {
	Name() string
	Subscribe(ctx context.Context) (<-chan news.News, <-chan error)
}

// NewsRelevanceFilter отбирает новости, достаточно значимые для дорогого
// прогона через NewsAnalyzer, без анализа каждой новости LLM.
type NewsRelevanceFilter interface {
	IsRelevant(ctx context.Context, n news.News) (FilterResult, error)
}

// NewsAnalyzer извлекает структурированные признаки из новости с помощью LLM.
type NewsAnalyzer interface {
	Name() string
	Analyze(ctx context.Context, n news.News) (news.NewsAnalysis, error)
}

// DecisionEngine детерминированно превращает NewsAnalysis в TradingSignal.
type DecisionEngine interface {
	Name() string
	Decide(ctx context.Context, a news.NewsAnalysis) (trading.TradingSignal, error)
}

// PositionSizer рассчитывает размер позиции для исполнения TradingSignal.
type PositionSizer interface {
	Size(ctx context.Context, s trading.TradingSignal, balance trading.Balance) (qty int64, err error)
}

// Broker исполняет ордера и отдаёт состояние счёта у брокера.
type Broker interface {
	Name() string
	PlaceOrder(ctx context.Context, o trading.Order) (trading.Order, error)
	CancelOrder(ctx context.Context, id trading.OrderID) error
	GetOrderStatus(ctx context.Context, id trading.OrderID) (trading.Order, error)
	GetPositions(ctx context.Context) ([]trading.Position, error)
	GetBalance(ctx context.Context) (trading.Balance, error)
}

// NewsRepository — персистентность новостей: дедупликация по ExternalID и аудит.
type NewsRepository interface {
	Save(ctx context.Context, n news.News) error
	Update(ctx context.Context, n news.News) error
	FindByExternalID(ctx context.Context, source, externalID string) (news.News, bool, error)
}

// AnalysisRepository — персистентность результатов анализа для аудита и бэктестинга.
type AnalysisRepository interface {
	Save(ctx context.Context, a news.NewsAnalysis) error
}

// SignalRepository — персистентность сигналов и поиск недавних для cooldown.
type SignalRepository interface {
	Save(ctx context.Context, s trading.TradingSignal) error
	FindRecentByTicker(ctx context.Context, ticker trading.Ticker, within time.Duration) ([]trading.TradingSignal, error)
}

// OrderRepository — персистентность ордеров для идемпотентности исполнения.
type OrderRepository interface {
	Save(ctx context.Context, o trading.Order) error
	Update(ctx context.Context, o trading.Order) error
}
