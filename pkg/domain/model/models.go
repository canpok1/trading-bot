package model

type OrderType int

const (
	Sell OrderType = iota
	Buy
)

type CurrencyPair int

const (
	BtcJpy CurrencyPair = iota
)
