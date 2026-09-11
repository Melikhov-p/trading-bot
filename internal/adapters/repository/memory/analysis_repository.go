package memory

import (
	"context"
	"sync"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
)

// AnalysisRepository — потокобезопасный port.AnalysisRepository поверх
// карты, проиндексированной по NewsID.
type AnalysisRepository struct {
	mu     sync.RWMutex
	byNews map[string]news.NewsAnalysis
}

// NewAnalysisRepository создаёт пустой AnalysisRepository.
func NewAnalysisRepository() *AnalysisRepository {
	return &AnalysisRepository{byNews: make(map[string]news.NewsAnalysis)}
}

// Save сохраняет результат анализа новости.
func (r *AnalysisRepository) Save(ctx context.Context, a news.NewsAnalysis) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.byNews[a.NewsID] = a
	return nil
}

var _ port.AnalysisRepository = (*AnalysisRepository)(nil)
