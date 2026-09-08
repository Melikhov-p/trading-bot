// Package filesource реализует port.NewsProvider для локальной отладки
// пайплайна — читает новости из файла вместо реального внешнего источника.
// TODO(итерация 4): реализовать чтение newline-delimited JSON из Path.
package filesource

import (
	"context"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
)

// Provider — заглушка port.NewsProvider, читающая новости из файла.
type Provider struct {
	Path string
}

// New создаёт Provider, читающий новости из файла path.
func New(path string) *Provider {
	return &Provider{Path: path}
}

// Name возвращает имя провайдера для логирования и метрик.
func (p *Provider) Name() string {
	return "filesource"
}

// Subscribe пока не реализован — возвращает закрытые каналы с port.ErrNotImplemented.
func (p *Provider) Subscribe(ctx context.Context) (<-chan news.News, <-chan error) {
	newsCh := make(chan news.News)
	close(newsCh)

	errCh := make(chan error, 1)
	errCh <- port.ErrNotImplemented
	close(errCh)

	return newsCh, errCh
}

var _ port.NewsProvider = (*Provider)(nil)
