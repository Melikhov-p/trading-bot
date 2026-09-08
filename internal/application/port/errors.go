package port

import "errors"

// Сентинел-ошибки NewsAnalyzer.Analyze — используются оркестратором, чтобы
// решить, ретраить вызов или сохранить сырой ответ для ручного review.
var (
	// ErrUpstreamUnavailable — модель/сеть временно недоступны, вызов можно ретраить.
	ErrUpstreamUnavailable = errors.New("port: analyzer upstream unavailable")
	// ErrInvalidJSON — ответ модели не парсится как JSON, ретраить бессмысленно.
	ErrInvalidJSON = errors.New("port: analyzer response is not valid json")
	// ErrSchemaViolation — JSON распарсен, но не соответствует ожидаемой схеме.
	ErrSchemaViolation = errors.New("port: analyzer response violates schema")
)

// ErrNotImplemented возвращают адаптеры-заглушки, чьи протоколы ещё не
// реализованы (ожидают сверки с документацией внешнего API в следующей итерации).
var ErrNotImplemented = errors.New("port: adapter not implemented yet")
