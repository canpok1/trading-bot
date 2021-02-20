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
