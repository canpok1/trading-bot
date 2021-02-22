package model

const (
	// Buy 指値注文、現物取引、買い
	Buy OrderType = "buy"
	// Sell 指値注文、現物取引、売り
	Sell OrderType = "sell"
	// MarketBuy 成行注文、現物取引、買い
	MarketBuy OrderType = "market_buy"
	// MarketSell 成行注文、現物取引、売り
	MarketSell OrderType = "market_sell"
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
	Taker LiquidityType = iota
	// Maker Maker
	Maker LiquidityType = 1
)

const (
	// Open open
	Open OrderStatus = iota
	// Closed closed
	Closed
	// Canceled canceled
	Canceled
)

const (
	// BuySide 買い
	BuySide ContractSide = iota
	// SellSide 売り
	SellSide
)
