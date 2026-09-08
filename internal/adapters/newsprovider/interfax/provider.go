// Package interfax реализует port.NewsProvider поверх Interfax API.
//
// TODO(итерация 6): уточнить по документации Interfax API — эндпоинты,
// схема аутентификации, формат пагинации/курсоров, формат самой новости.
// До этого момента адаптер — заглушка, возвращающая port.ErrNotImplemented.
package interfax

import (
	"context"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
)

// Provider — заглушка port.NewsProvider для Interfax.
type Provider struct {
	// TODO: base URL, токен аутентификации, HTTP-клиент — после сверки с документацией.
}

// New создаёт заглушку Provider.
func New() *Provider {
	return &Provider{}
}

// Name возвращает имя провайдера для логирования и метрик.
func (p *Provider) Name() string {
	return "interfax"
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
