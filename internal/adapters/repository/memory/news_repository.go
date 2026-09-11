// Package memory реализует репозитории портов (NewsRepository,
// AnalysisRepository, SignalRepository, OrderRepository) поверх обычных карт
// в памяти процесса — для сборки полного пайплайна на FakeNewsProvider и
// интеграционных тестов, без внешней БД. Персистентность, переживающая
// рестарт процесса, появится в Итерации 5 (SQLite) за теми же интерфейсами.
package memory

import (
	"context"
	"sync"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
)

// NewsRepository — потокобезопасный port.NewsRepository поверх карты,
// проиндексированной по Source+ExternalID (дедупликация входящего потока).
type NewsRepository struct {
	mu  sync.RWMutex
	byE map[string]news.News // ключ: Source + "|" + ExternalID
}

// NewNewsRepository создаёт пустой NewsRepository.
func NewNewsRepository() *NewsRepository {
	return &NewsRepository{byE: make(map[string]news.News)}
}

// Save сохраняет новость (upsert по Source+ExternalID).
func (r *NewsRepository) Save(ctx context.Context, n news.News) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.byE[externalKey(n.Source, n.ExternalID)] = n
	return nil
}

// Update обновляет ранее сохранённую новость (upsert — семантически то же,
// что и Save: для in-memory реализации разделение не несёт дополнительного
// смысла).
func (r *NewsRepository) Update(ctx context.Context, n news.News) error {
	return r.Save(ctx, n)
}

// FindByExternalID ищет новость по источнику и внешнему идентификатору.
func (r *NewsRepository) FindByExternalID(ctx context.Context, source, externalID string) (news.News, bool, error) {
	if err := ctx.Err(); err != nil {
		return news.News{}, false, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	n, ok := r.byE[externalKey(source, externalID)]
	return n, ok, nil
}

func externalKey(source, externalID string) string {
	return source + "|" + externalID
}

var _ port.NewsRepository = (*NewsRepository)(nil)
