// Package news содержит доменные сущности новостного контура: сырую новость
// и структурированный результат её анализа моделью. Пакет не зависит от
// application/adapters слоёв.
package news

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"trading-bot/internal/domain/trading"
)

// Status — статус обработки новости в пайплайне.
type Status string

const (
	StatusUnknown        Status = ""
	StatusReceived       Status = "received"
	StatusFilteredOut    Status = "filtered_out"
	StatusAnalysisFailed Status = "analysis_failed"
	StatusAnalyzed       Status = "analyzed"
	StatusSkipped        Status = "skipped"
	StatusExecuted       Status = "executed"
)

// News — новость, полученная от NewsProvider, до или после анализа.
type News struct {
	ID          string
	Source      string
	ExternalID  string
	Headline    string
	Body        string
	PublishedAt time.Time
	ReceivedAt  time.Time
	Tickers     []trading.Ticker
	Companies   []string
	URL         string
	Status      Status
	Raw         []byte
}

// Fingerprint возвращает детерминированный хэш содержимого новости,
// используемый фильтром для отсева почти-дублей в TTL-окне.
func (n News) Fingerprint() string {
	h := sha256.Sum256([]byte(n.Source + "|" + n.Headline + "|" + n.Body))
	return hex.EncodeToString(h[:])
}
