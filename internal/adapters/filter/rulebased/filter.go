// Package rulebased реализует port.NewsRelevanceFilter эвристиками без LLM:
// минимальная длина текста, возраст новости, дедупликация по Fingerprint,
// попадание в watchlist и по ключевым словам категорий событий.
package rulebased

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
	"trading-bot/internal/platform/clock"
)

// Config — пороги, веса и справочники правил фильтрации.
type Config struct {
	MinBodyLength int
	MaxAge        time.Duration
	DedupWindow   time.Duration

	// WatchlistWeight/KeywordWeight — вклад каждого сигнала в итоговый score.
	WatchlistWeight float64
	KeywordWeight   float64
	MinScoreToPass  float64

	Watchlist []string
	Keywords  map[string][]string // EventType -> ключевые слова
}

// Filter — эвристический port.NewsRelevanceFilter.
type Filter struct {
	cfg Config
	clk clock.Clock

	mu   sync.Mutex
	seen map[string]time.Time // Fingerprint -> момент истечения DedupWindow
}

// New создаёт Filter с заданной конфигурацией и источником времени.
func New(cfg Config, clk clock.Clock) *Filter {
	return &Filter{cfg: cfg, clk: clk, seen: make(map[string]time.Time)}
}

// IsRelevant отбирает новости, достаточно значимые для прогона через
// NewsAnalyzer: короткие, устаревшие и почти-дублирующие недавние новости
// отсеиваются до вычисления эвристического score.
func (f *Filter) IsRelevant(ctx context.Context, n news.News) (port.FilterResult, error) {
	if err := ctx.Err(); err != nil {
		return port.FilterResult{}, err
	}

	if len(n.Body) < f.cfg.MinBodyLength {
		return port.FilterResult{Reasons: []string{"body too short"}}, nil
	}

	now := f.clk.Now()
	if f.cfg.MaxAge > 0 && now.Sub(n.PublishedAt) > f.cfg.MaxAge {
		return port.FilterResult{Reasons: []string{"news too old"}}, nil
	}

	if f.isDuplicate(n, now) {
		return port.FilterResult{Reasons: []string{"duplicate within dedup window"}}, nil
	}

	watchlistHit := f.watchlistHit(n)
	keywordHit, hintEventType := f.keywordHit(n)

	var (
		score   float64
		reasons []string
	)
	if watchlistHit {
		score += f.cfg.WatchlistWeight
		reasons = append(reasons, "watchlist hit")
	}
	if keywordHit {
		score += f.cfg.KeywordWeight
		reasons = append(reasons, "keyword hit: "+string(hintEventType))
	}

	relevant := score >= f.cfg.MinScoreToPass || (watchlistHit && keywordHit)
	if !relevant {
		reasons = append(reasons, "score below threshold")
	}

	return port.FilterResult{
		Relevant:      relevant,
		Reasons:       reasons,
		Score:         score,
		HintEventType: hintEventType,
	}, nil
}

// isDuplicate проверяет, встречался ли Fingerprint новости в пределах
// DedupWindow, и в любом случае регистрирует его для будущих проверок.
// Попутно вычищает устаревшие записи, чтобы карта не росла бесконечно.
func (f *Filter) isDuplicate(n news.News, now time.Time) bool {
	fp := n.Fingerprint()

	f.mu.Lock()
	defer f.mu.Unlock()

	for k, expiry := range f.seen {
		if now.After(expiry) {
			delete(f.seen, k)
		}
	}

	if expiry, ok := f.seen[fp]; ok && now.Before(expiry) {
		return true
	}

	f.seen[fp] = now.Add(f.cfg.DedupWindow)
	return false
}

// watchlistHit проверяет пересечение тикеров/компаний новости с watchlist.
func (f *Filter) watchlistHit(n news.News) bool {
	for _, w := range f.cfg.Watchlist {
		for _, t := range n.Tickers {
			if string(t) == w {
				return true
			}
		}
		for _, c := range n.Companies {
			if strings.EqualFold(c, w) {
				return true
			}
		}
	}

	return false
}

// keywordHit ищет по заголовку и тексту новости ключевые слова категорий
// событий. Категории перебираются в детерминированном (отсортированном)
// порядке, чтобы при совпадении сразу нескольких категорий hint был
// стабильным между вызовами.
func (f *Filter) keywordHit(n news.News) (bool, news.EventType) {
	if len(f.cfg.Keywords) == 0 {
		return false, news.EventTypeUnknown
	}

	haystack := strings.ToLower(n.Headline + " " + n.Body)

	categories := make([]string, 0, len(f.cfg.Keywords))
	for category := range f.cfg.Keywords {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	for _, category := range categories {
		for _, kw := range f.cfg.Keywords[category] {
			if kw == "" {
				continue
			}
			if strings.Contains(haystack, strings.ToLower(kw)) {
				return true, news.EventType(category)
			}
		}
	}

	return false, news.EventTypeUnknown
}

var _ port.NewsRelevanceFilter = (*Filter)(nil)
