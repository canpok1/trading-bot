package model

const (
	// Sell 売り注文
	Sell OrderType = "sell"
	// Buy 買い注文
	Buy OrderType = "buy"
)

const (
	// JPY 日本円
	JPY CurrencyType = "jpy"
	// BTC ビットコイン
	BTC CurrencyType = "btc"
	// FCT ファクトム
	FCT CurrencyType = "fct"
	// ETC イーサリアムクラシック
	ETC CurrencyType = "etc"
	// MONA モナコイン
	MONA CurrencyType = "mona"
)

var (
	// BtcJpy BTC/JPY
	BtcJpy CurrencyPair = CurrencyPair{Key: BTC, Settlement: JPY}
	// FctJpy Fct/JPY
	FctJpy CurrencyPair = CurrencyPair{Key: FCT, Settlement: JPY}
	// EtcJpy ETC/JPY
	EtcJpy CurrencyPair = CurrencyPair{Key: ETC, Settlement: JPY}
	// MonaJpy MONA/JPY
	MonaJpy CurrencyPair = CurrencyPair{Key: MONA, Settlement: JPY}
)

const (
	// Taker Taker
	Taker LiquidityType = "T"
	// Maker Maker
	Maker LiquidityType = "M"
)
