// Package filesource реализует port.NewsProvider для локальной отладки
// пайплайна — читает новости из newline-delimited JSON файла вместо
// реального внешнего источника.
package filesource

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
)

// maxLineSize — предельный размер одной строки файла (тело новости может
// быть длинным, дефолтного буфера bufio.Scanner в 64KiB не всегда достаточно).
const maxLineSize = 1 << 20 // 1 MiB

// Provider — port.NewsProvider, читающий новости из файла Path построчно:
// каждая строка — независимый JSON-документ news.News.
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

// Subscribe читает Path построчно и публикует новости в newsCh. Ошибка
// открытия файла — фатальна для всего Subscribe; ошибка парсинга отдельной
// строки уходит в errCh, но не прерывает чтение остальных строк. Оба канала
// закрываются по завершении (конец файла, фатальная ошибка или отмена ctx).
func (p *Provider) Subscribe(ctx context.Context) (<-chan news.News, <-chan error) {
	newsCh := make(chan news.News)
	errCh := make(chan error, 1)

	go func() {
		defer close(newsCh)
		defer close(errCh)

		f, err := os.Open(p.Path)
		if err != nil {
			errCh <- fmt.Errorf("filesource: open %q: %w", p.Path, err)
			return
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}

			var n news.News
			if err := json.Unmarshal(line, &n); err != nil {
				errCh <- fmt.Errorf("filesource: unmarshal line: %w", err)
				continue
			}

			select {
			case newsCh <- n:
			case <-ctx.Done():
				return
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("filesource: scan %q: %w", p.Path, err)
		}
	}()

	return newsCh, errCh
}

var _ port.NewsProvider = (*Provider)(nil)
