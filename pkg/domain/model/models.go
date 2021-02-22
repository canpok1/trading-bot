package model

import (
	"fmt"
)

// OrderType 注文種別
type OrderType string

// CurrencyType 通貨種別
type CurrencyType string

// CurrencyPair 通貨ペア
type CurrencyPair struct {
	Key        CurrencyType
	Settlement CurrencyType
}

// String 文字変換
func (p *CurrencyPair) String() string {
	return fmt.Sprintf("%s_%s", p.Key, p.Settlement)
}

// LiquidityType 流動性種別
type LiquidityType int

// OrderRate 注文レート
type OrderRate struct {
	Pair CurrencyPair
	Rate float32
}

// Balance 残高
type Balance struct {
	Jpy float32
	Btc float32
}

// NewOrder 新規注文
type NewOrder struct {
	Type            OrderType
	Pair            CurrencyPair
	Amount          *float32
	Rate            *float32
	MarketBuyAmount *float32
	StopLossRate    *float32
}

// OrderStatus 注文ステータス
type OrderStatus int

// Order 注文
type Order struct {
	ID           uint64
	Type         OrderType
	Pair         CurrencyPair
	Amount       float32
	Rate         *float32
	StopLossRate *float32
	Status       OrderStatus
}

// ContractSide 約定サイド
type ContractSide int

// Contract 約定
type Contract struct {
	ID               uint64
	OrderID          uint64
	Rate             float32
	IncreaseCurrency CurrencyType
	IncreaseAmount   float32
	DecreaseCurrency CurrencyType
	DecreaseAmount   float32
	FeeCurrency      CurrencyType
	Fee              float32
	Liquidity        LiquidityType
	Side             ContractSide
}

// Position ポジション
type Position struct {
	OpenerOrder *Order
	CloserOrder *Order
}
