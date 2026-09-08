package trading

// OrderID — идентификатор ордера в домене (не путать с BrokerOrderID брокера).
type OrderID string

// OrderSide — направление ордера.
type OrderSide string

const (
	OrderSideUnknown OrderSide = ""
	OrderSideBuy     OrderSide = "buy"
	OrderSideSell    OrderSide = "sell"
)

// OrderKind — тип исполнения ордера.
type OrderKind string

const (
	OrderKindUnknown OrderKind = ""
	OrderKindMarket  OrderKind = "market"
	OrderKindLimit   OrderKind = "limit"
)

// OrderStatus — статус ордера на стороне брокера.
type OrderStatus string

const (
	OrderStatusUnknown   OrderStatus = ""
	OrderStatusNew       OrderStatus = "new"
	OrderStatusFilled    OrderStatus = "filled"
	OrderStatusCancelled OrderStatus = "cancelled"
	OrderStatusRejected  OrderStatus = "rejected"
)

// Order — торговый ордер, сформированный по TradingSignal и отправленный брокеру.
type Order struct {
	ID            OrderID
	SignalID      string
	Ticker        Ticker
	Side          OrderSide
	Kind          OrderKind
	Quantity      int64
	LimitPrice    float64
	Status        OrderStatus
	BrokerOrderID string
}

// Position — открытая позиция по инструменту.
type Position struct {
	Ticker   Ticker
	Quantity int64
	AvgPrice float64
}

// Balance — состояние торгового счёта.
type Balance struct {
	Currency string
	Cash     float64
	Equity   float64
}
