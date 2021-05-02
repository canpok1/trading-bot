package model

import (
	"fmt"
	"strings"
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

// NewCurrencyPair 生成
func ParseToCurrencyPair(s string) (*CurrencyPair, error) {
	splited := strings.Split(s, "_")
	if len(splited) != 2 {
		return nil, fmt.Errorf("failed to parse string to CurrencyPair, string: %s", s)
	}

	return &CurrencyPair{
		Key:        CurrencyType(splited[0]),
		Settlement: CurrencyType(splited[1]),
	}, nil
}

// String 文字変換
func (p *CurrencyPair) String() string {
	return fmt.Sprintf("%s_%s", p.Key, p.Settlement)
}

// LiquidityType 流動性種別
type LiquidityType int

// StoreRate 販売所レート
type StoreRate struct {
	Pair CurrencyPair
	Rate float64
}

// OrderRate 注文レート
type OrderRate struct {
	Pair CurrencyPair
	Side OrderSide
	Rate float64
}

// Balance 残高
type Balance struct {
	Currency CurrencyType
	Amount   float64
	Reserved float64
}

// Total 残高合計
func (b *Balance) Total() float64 {
	return b.Amount + b.Reserved
}

// NewOrder 新規注文
type NewOrder struct {
	Type            OrderType
	Pair            CurrencyPair
	Amount          *float64
	Rate            *float64
	MarketBuyAmount *float64
	StopLossRate    *float64
}

// OrderStatus 注文ステータス
type OrderStatus int

// Order 注文
type Order struct {
	ID           uint64
	Type         OrderType
	Pair         CurrencyPair
	Amount       float64
	Rate         *float64
	StopLossRate *float64
	Status       OrderStatus
	OrderedAt    time.Time
}

// String 文字列
func (o *Order) String() string {
	rate := "-"
	if o.Rate != nil {
		rate = fmt.Sprintf("%f", *o.Rate)
	}

	stopLossRate := "-"
	if o.StopLossRate != nil {
		stopLossRate = fmt.Sprintf("%f", *o.StopLossRate)
	}

	status := "-"
	switch o.Status {
	case 0:
		status = "open"
	case 1:
		status = "closed"
	case 2:
		status = "canceled"
	}
	return fmt.Sprintf("order[id:%d %s %s amout:%f rate:%s stop_loss_rate:%s status:%s]", o.ID, o.Type, o.Pair.String(), o.Amount, rate, stopLossRate, status)
}

// OrderSide 注文サイド
type OrderSide int

// Contract 約定
type Contract struct {
	ID               uint64
	OrderID          uint64
	Rate             float64
	IncreaseCurrency CurrencyType
	IncreaseAmount   float64
	DecreaseCurrency CurrencyType
	DecreaseAmount   float64
	FeeCurrency      CurrencyType
	Fee              float64
	Liquidity        LiquidityType
	Side             OrderSide
}

func (c *Contract) String() string {
	liquidity := "-"
	switch c.Liquidity {
	case 0:
		liquidity = "Taker"
	case 1:
		liquidity = "Maker"
	}

	side := "-"
	switch c.Side {
	case 0:
		side = "buy"
	case 1:
		side = "sell"
	}
	return fmt.Sprintf("contract[id:%d order_id:%d rate: %f %s:%f %s:%f fee:%f %s %s]",
		c.ID,
		c.OrderID,
		c.Rate,
		c.IncreaseCurrency,
		c.IncreaseAmount,
		c.DecreaseCurrency,
		c.DecreaseAmount,
		c.Fee,
		liquidity,
		side,
	)
}

// Position ポジション
type Position struct {
	ID          uint64
	OpenerOrder *Order
	CloserOrder *Order
}
