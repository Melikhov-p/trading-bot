// Package httpclient предоставляет сконфигурированный http.Client для
// исходящих вызовов к внешним API (YandexGPT, Interfax, БКС).
package httpclient

import (
	"net/http"
	"time"
)

// New возвращает http.Client с заданным таймаутом на весь запрос —
// каждый внешний вызов обязан иметь таймаут (см. golang-design-patterns).
func New(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
	}
}
