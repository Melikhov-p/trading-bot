package trading

import "time"

// Action — торговое действие, которое рекомендует TradingSignal.
type Action string

const (
	ActionUnknown Action = "" // нулевое значение — сигнал ещё не классифицирован
	ActionBuy     Action = "buy"
	ActionSell    Action = "sell"
	ActionHold    Action = "hold"
	ActionWatch   Action = "watch"
)

// Track — торговый горизонт, на котором действует сигнал.
type Track string

const (
	TrackUnknown  Track = ""
	TrackIntraday Track = "intraday"
	TrackSwing    Track = "swing"
	TrackPosition Track = "position"
)

// SignalStatus — статус жизненного цикла сигнала в оркестраторе.
type SignalStatus string

const (
	SignalStatusUnknown  SignalStatus = ""
	SignalStatusPending  SignalStatus = "pending"
	SignalStatusSkipped  SignalStatus = "skipped"
	SignalStatusExecuted SignalStatus = "executed"
	SignalStatusFailed   SignalStatus = "failed"
)

// TradingSignal — результат работы DecisionEngine: рекомендация действия по инструменту.
type TradingSignal struct {
	ID          string
	NewsID      string
	Ticker      Ticker
	Action      Action
	Track       Track
	FinalScore  float64
	Strength    float64
	Rationale   string
	Status      SignalStatus
	GeneratedAt time.Time
	ExpiresAt   time.Time
}
