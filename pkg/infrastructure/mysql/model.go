package mysql

import "trading-bot/pkg/domain/model"

// Order 注文情報
type Order struct {
	ID           uint64
	OrderType    int
	Pair         string
	Amount       float32
	Rate         *float32
	StopLossRate *float32
	Status       int
}

// NewOrder 生成
func NewOrder(org *model.Order, status model.OrderStatus) *Order {
	var orderType int
	switch org.Type {
	case model.Buy:
		orderType = 0
	case model.Sell:
		orderType = 1
	case model.MarketBuy:
		orderType = 2
	case model.MarketSell:
		orderType = 3
	}

	return &Order{
		ID:           org.ID,
		OrderType:    orderType,
		Pair:         org.Pair.String(),
		Amount:       org.Amount,
		Rate:         org.Rate,
		StopLossRate: org.StopLossRate,
		Status:       int(status),
	}
}

// ToDomainModel ドメインモデルに変換
func (o *Order) ToDomainModel() (*model.Order, error) {
	pair, err := model.ParseToCurrencyPair(o.Pair)
	if err != nil {
		return nil, err
	}

	var orderType model.OrderType
	switch o.OrderType {
	case 0:
		orderType = model.Buy
	case 1:
		orderType = model.Sell
	case 2:
		orderType = model.MarketBuy
	case 3:
		orderType = model.MarketSell
	}

	return &model.Order{
		ID:           o.ID,
		Type:         orderType,
		Pair:         *pair,
		Amount:       o.Amount,
		Rate:         o.Rate,
		StopLossRate: o.StopLossRate,
		Status:       model.OrderStatus(o.Status),
	}, nil
}

// Contract 約定情報
type Contract struct {
	ID               uint64
	OrderID          uint64
	Rate             float32
	Side             int
	IncreaseCurrency string
	IncreaseAmount   float32
	DecreaseCurrency string
	DecreaseAmount   float32
	FeeCurrency      string
	FeeAmount        float32
	Liquidity        int
}

// NewContract 生成
func NewContract(org *model.Contract) *Contract {
	return &Contract{
		ID:               org.ID,
		OrderID:          org.OrderID,
		Rate:             org.Rate,
		Side:             int(org.Side),
		IncreaseCurrency: string(org.IncreaseCurrency),
		IncreaseAmount:   org.IncreaseAmount,
		DecreaseCurrency: string(org.DecreaseCurrency),
		DecreaseAmount:   org.DecreaseAmount,
		FeeCurrency:      string(org.FeeCurrency),
		FeeAmount:        org.Fee,
		Liquidity:        int(org.Liquidity),
	}
}

// Position ポジション
type Position struct {
	ID            uint64
	OpenerOrderID uint64
	CloserOrderID *uint64
}

// Profit 利益
type Profit struct {
	Amount float64
}
