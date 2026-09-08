// Package rulebased реализует port.NewsRelevanceFilter эвристиками без LLM:
// минимальная длина текста, возраст новости, дедупликация по Fingerprint,
// попадание в watchlist и по ключевым словам категорий событий.
// TODO(итерация 3): реализовать вычисление FilterResult по правилам из плана.
package rulebased

import (
	"context"
	"time"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
)

// Config — пороги и справочники правил фильтрации.
type Config struct {
	MinBodyLength  int
	MaxAge         time.Duration
	DedupWindow    time.Duration
	MinScoreToPass float64
	Watchlist      []string
	Keywords       map[string][]string // EventType -> ключевые слова
}

// Filter — заглушка port.NewsRelevanceFilter.
type Filter struct {
	cfg Config
}

// New создаёт Filter с заданной конфигурацией.
func New(cfg Config) *Filter {
	return &Filter{cfg: cfg}
}

// IsRelevant пока не реализован — см. TODO в комментарии пакета.
func (f *Filter) IsRelevant(ctx context.Context, n news.News) (port.FilterResult, error) {
	return port.FilterResult{}, port.ErrNotImplemented
}

var _ port.NewsRelevanceFilter = (*Filter)(nil)
