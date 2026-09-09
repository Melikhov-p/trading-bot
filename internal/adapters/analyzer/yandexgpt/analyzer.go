// Package yandexgpt реализует port.NewsAnalyzer поверх YandexGPT Responses API.
package yandexgpt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/ilyakaznacheev/cleanenv"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
	"trading-bot/internal/platform/clock"
)

const (
	responsesEndpoint = "https://ai.api.cloud.yandex.net/v1/responses"
	maxRetries        = 3
	baseRetryDelay    = 500 * time.Millisecond
)

// Config — параметры подключения к YandexGPT Responses API. Секреты
// (APIKey/FolderID) читаются только из окружения через LoadConfig — им нет
// места в YAML-конфиге приложения.
type Config struct {
	APIKey   string        `env:"YANDEX_API_KEY"   env-required:"true"`
	FolderID string        `env:"YANDEX_FOLDER_ID" env-required:"true"`
	PromptID string        `env:"YANDEX_PROMPT_ID" env-default:"fvtpln2v5hgd3etr4hhf"`
	Timeout  time.Duration `env:"YANDEX_TIMEOUT"   env-default:"30s"`
}

// LoadConfig читает Config из переменных окружения (YANDEX_API_KEY,
// YANDEX_FOLDER_ID, YANDEX_PROMPT_ID, YANDEX_TIMEOUT).
func LoadConfig() (Config, error) {
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return Config{}, fmt.Errorf("yandexgpt: read env config: %w", err)
	}

	return cfg, nil
}

// Analyzer — port.NewsAnalyzer поверх YandexGPT Responses API.
type Analyzer struct {
	cfg        Config
	client     *http.Client
	clk        clock.Clock
	endpoint   string        // переопределяется в тестах на httptest.Server URL
	retryDelay time.Duration // переопределяется в тестах, чтобы не ждать реальный backoff
}

// New создаёт Analyzer с заданной конфигурацией, HTTP-клиентом и источником времени.
func New(cfg Config, client *http.Client, clk clock.Clock) *Analyzer {
	return &Analyzer{cfg: cfg, client: client, clk: clk, endpoint: responsesEndpoint, retryDelay: baseRetryDelay}
}

// Name возвращает имя анализатора для логирования и метрик.
func (a *Analyzer) Name() string {
	return "yandexgpt"
}

// Analyze отправляет текст новости в YandexGPT и маппит структурированный
// ответ в news.NewsAnalysis. Сетевые ошибки/5xx/429 ретраятся с backoff;
// невалидный JSON или значение вне доменных enum — нет (see port.ErrInvalidJSON,
// port.ErrSchemaViolation).
func (a *Analyzer) Analyze(ctx context.Context, n news.News) (news.NewsAnalysis, error) {
	outputText, err := a.requestWithRetry(ctx, n.Body)
	if err != nil {
		return news.NewsAnalysis{}, err
	}

	return parseAnalysis(n.ID, a.Name(), outputText, a.clk.Now())
}

// requestWithRetry вызывает callOnce с экспоненциальным backoff, если ошибка
// помечена как retryable (сетевой таймаут, 429, 5xx). Между попытками
// проверяет отмену контекста.
func (a *Analyzer) requestWithRetry(ctx context.Context, newsBody string) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepOrDone(ctx, a.retryDelay*time.Duration(1<<(attempt-1))); err != nil {
				return "", err
			}
		}

		outputText, retryable, err := a.callOnce(ctx, newsBody)
		if err == nil {
			return outputText, nil
		}

		lastErr = err
		if !retryable {
			return "", err
		}
	}

	return "", fmt.Errorf("yandexgpt: exhausted retries: %w", lastErr)
}

func sleepOrDone(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("yandexgpt: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

// callOnce выполняет один HTTP-запрос к YandexGPT Responses API.
// retryable сообщает вызывающему коду, стоит ли повторить попытку.
func (a *Analyzer) callOnce(ctx context.Context, newsBody string) (outputText string, retryable bool, err error) {
	reqBody, err := json.Marshal(responseRequestDTO{
		Prompt: promptDTO{ID: a.cfg.PromptID},
		Input:  newsBody,
	})
	if err != nil {
		return "", false, fmt.Errorf("yandexgpt: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", false, fmt.Errorf("yandexgpt: build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Api-Key "+a.cfg.APIKey)
	req.Header.Set("OpenAI-Project", a.cfg.FolderID)

	resp, err := a.client.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return "", true, fmt.Errorf("yandexgpt: %w: %w", port.ErrUpstreamUnavailable, err)
		}

		return "", false, fmt.Errorf("yandexgpt: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("yandexgpt: read response body: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		var data responseDataDTO
		if err := json.Unmarshal(respBody, &data); err != nil {
			return "", false, fmt.Errorf("yandexgpt: unmarshal response envelope: %w", err)
		}

		return data.OutputText, false, nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError:
		return "", true, fmt.Errorf("yandexgpt: %w: status %d: %s", port.ErrUpstreamUnavailable, resp.StatusCode, respBody)
	default:
		return "", false, fmt.Errorf("yandexgpt: unexpected status %d: %s", resp.StatusCode, respBody)
	}
}

var _ port.NewsAnalyzer = (*Analyzer)(nil)
