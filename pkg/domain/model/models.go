package model

type OrderType string

const (
	Sell OrderType = "sell"
	Buy  OrderType = "buy"
)

type CurrencyPair string

const (
	BtcJpy CurrencyPair = "BTC/JPY"
)

// 注文レート
type OrderRate struct {
	Pair CurrencyPair
	Rate float32
}

// 残高
type Balance struct {
	Jpy float32
	Btc float32
}

// 注文
type Order struct {
	ID   uint64
	Type OrderType
	Pair CurrencyPair
}

// ポジション
type Position struct {
	OpenerOrder *Order
	CloserOrder *Order
}
