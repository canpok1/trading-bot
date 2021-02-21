package model

import (
	"fmt"
	"time"
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
type LiquidityType string

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
	Type         OrderType
	Pair         CurrencyPair
	Amount       *float32
	Rate         *float32
	StopLossRate *float32
}

// Order 注文
type Order struct {
	ID           uint64
	Type         OrderType
	Pair         CurrencyPair
	Amount       float32
	Rate         *float32
	StopLossRate *float32
	OpenAt       time.Time
}

// OrderTransaction 注文履歴
type OrderTransaction struct {
	OrderID   uint64
	CreatedAt time.Time
	Canceled  bool
	Contract  *Contract
}

// Contract 約定
type Contract struct {
	ID          uint64
	Rate        float32
	Currency1   CurrencyType
	Fund1       float32
	Currency2   CurrencyType
	Fund2       float32
	FeeCurrency CurrencyType
	Fee         float32
	Liquidity   LiquidityType
	Side        OrderType
}

// Position ポジション
type Position struct {
	OpenerOrder *Order
	CloserOrder *Order
}
