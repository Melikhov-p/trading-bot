// Package yandexgpt реализует port.NewsAnalyzer поверх YandexGPT Responses API.
// TODO(итерация 2): перенести и доработать логику текущего корневого main.go —
// HTTP-запрос с ctx и retry/backoff на 5xx/сетевые таймауты/429, defensive-парсинг
// output_text (снятие markdown-обёртки, один repair-retry), маппинг строк JSON
// в доменные enum'ы и Score с валидацией диапазона [0,1].
package yandexgpt

import (
	"context"
	"net/http"
	"time"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
)

// Config — параметры подключения к YandexGPT Responses API.
type Config struct {
	APIKey   string
	FolderID string
	PromptID string
	Timeout  time.Duration
}

// Analyzer — заглушка port.NewsAnalyzer для YandexGPT.
type Analyzer struct {
	cfg    Config
	client *http.Client
}

// New создаёт Analyzer с заданной конфигурацией и HTTP-клиентом.
func New(cfg Config, client *http.Client) *Analyzer {
	return &Analyzer{cfg: cfg, client: client}
}

// Name возвращает имя анализатора для логирования и метрик.
func (a *Analyzer) Name() string {
	return "yandexgpt"
}

// Analyze пока не реализован — см. TODO в комментарии пакета.
func (a *Analyzer) Analyze(ctx context.Context, n news.News) (news.NewsAnalysis, error) {
	return news.NewsAnalysis{}, port.ErrNotImplemented
}

var _ port.NewsAnalyzer = (*Analyzer)(nil)
